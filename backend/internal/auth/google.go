package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/rahul/indifferent/backend/internal/models"
)

const (
	// AccessTokenTTL is the lifetime of an access token.
	AccessTokenTTL = 15 * time.Minute
	// RefreshTokenTTL is the lifetime of a refresh token.
	RefreshTokenTTL = 7 * 24 * time.Hour
	// RefreshTokenBytes is the number of random bytes for a refresh token.
	RefreshTokenBytes = 32
)

// GoogleUserInfo represents the user profile returned by Google's userinfo endpoint.
type GoogleUserInfo struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	AvatarURL string `json:"picture"`
	// VerifiedEmail is Google's own statement that the account holder proved
	// they control Email. Without it the address is only a claim.
	VerifiedEmail bool `json:"verified_email"`
}

// GoogleTokenResponse represents the token endpoint response from Google.
type GoogleTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
	IDToken     string `json:"id_token"`
}

// DynamoDBClient defines the DynamoDB operations needed by the auth service.
type DynamoDBClient interface {
	PutItem(ctx context.Context, params *dynamodb.PutItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error)
	GetItem(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error)
	DeleteItem(ctx context.Context, params *dynamodb.DeleteItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error)
	UpdateItem(ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error)
	Query(ctx context.Context, params *dynamodb.QueryInput, optFns ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error)
}

// HTTPClient defines the HTTP operations needed by the auth service.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// GoogleAuthService provides Google OAuth authentication and token management.
type GoogleAuthService interface {
	// Authenticate exchanges a Google authorization code for user info,
	// creates or updates the user in DynamoDB, and returns auth tokens.
	//
	// codeVerifier is the PKCE verifier (RFC 7636) the browser made before it
	// sent the user to Google. When it is not empty it goes to Google with the
	// code, and Google refuses the code unless it matches the challenge the
	// sign-in started with. That ties the code to the browser that began the
	// flow, so a code obtained elsewhere cannot be redeemed here.
	Authenticate(ctx context.Context, authCode, codeVerifier string) (*models.AuthTokens, error)
}

// ValidCodeVerifier reports whether v is a well-formed PKCE code verifier:
// 43 to 128 characters from the unreserved set (RFC 7636, section 4.1).
func ValidCodeVerifier(v string) bool {
	if len(v) < 43 || len(v) > 128 {
		return false
	}
	for i := 0; i < len(v); i++ {
		c := v[i]
		switch {
		case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c >= '0' && c <= '9',
			c == '-', c == '.', c == '_', c == '~':
		default:
			return false
		}
	}
	return true
}

// GoogleAuthConfig holds the configuration for Google OAuth.
type GoogleAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
	JWTSecret    string
	UsersTable   string
	SessionTable string
	// Signups tells the site owner about each new account (#297). Nil does nothing.
	Signups *SignupAlerter
}

// googleAuthService is the concrete implementation of GoogleAuthService.
type googleAuthService struct {
	config   GoogleAuthConfig
	db       DynamoDBClient
	http     HTTPClient
	tokenURL string
	userURL  string
}

// NewGoogleAuthService creates a new GoogleAuthService instance.
func NewGoogleAuthService(cfg GoogleAuthConfig, db DynamoDBClient, httpClient HTTPClient) GoogleAuthService {
	return &googleAuthService{
		config:   cfg,
		db:       db,
		http:     httpClient,
		tokenURL: "https://oauth2.googleapis.com/token",
		userURL:  "https://www.googleapis.com/oauth2/v2/userinfo",
	}
}

// newGoogleAuthServiceWithURLs is used internally for testing with custom endpoints.
func newGoogleAuthServiceWithURLs(cfg GoogleAuthConfig, db DynamoDBClient, httpClient HTTPClient, tokenURL, userURL string) *googleAuthService {
	return &googleAuthService{
		config:   cfg,
		db:       db,
		http:     httpClient,
		tokenURL: tokenURL,
		userURL:  userURL,
	}
}

// Authenticate exchanges a Google authorization code for user info,
// creates or updates the user in DynamoDB, and returns auth tokens.
func (s *googleAuthService) Authenticate(ctx context.Context, authCode, codeVerifier string) (*models.AuthTokens, error) {
	// Step 1: Exchange authorization code for Google access token
	googleToken, err := s.exchangeCode(ctx, authCode, codeVerifier)
	if err != nil {
		return nil, fmt.Errorf("exchange auth code: %w", err)
	}

	// Step 2: Fetch user info from Google
	userInfo, err := s.fetchUserInfo(ctx, googleToken.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("fetch user info: %w", err)
	}

	// Step 3: Create or update user in DynamoDB Users table. The account that
	// results is usually the Google one, but a verified Google address that
	// already belongs to an unconfirmed password account signs in to that
	// account (see linkToPasswordAccount).
	userID, linkedAtMs, err := s.upsertUser(ctx, userInfo)
	if err != nil {
		return nil, fmt.Errorf("upsert user: %w", err)
	}

	// Step 4: Generate JWT access token
	now := time.Now()
	if linkedAtMs > 0 && now.UnixMilli() <= linkedAtMs {
		// The sessions of the password account were revoked at linkedAtMs; the
		// session made now must start after that moment to survive it.
		now = time.UnixMilli(linkedAtMs + 1)
	}
	accessToken, err := s.generateAccessToken(userID, userInfo.Email, now)
	if err != nil {
		return nil, fmt.Errorf("generate access token: %w", err)
	}

	// Step 5: Generate refresh token and store session
	refreshToken, err := s.createSession(ctx, userID, userInfo.Email, now)
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	return &models.AuthTokens{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(AccessTokenTTL.Seconds()),
	}, nil
}

// exchangeCode exchanges the authorization code with Google's token endpoint.
// A non-empty codeVerifier is sent as code_verifier (PKCE).
func (s *googleAuthService) exchangeCode(ctx context.Context, code, codeVerifier string) (*GoogleTokenResponse, error) {
	data := url.Values{
		"code":          {code},
		"client_id":     {s.config.ClientID},
		"client_secret": {s.config.ClientSecret},
		"redirect_uri":  {s.config.RedirectURI},
		"grant_type":    {"authorization_code"},
	}
	if codeVerifier != "" {
		data.Set("code_verifier", codeVerifier)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send token request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("google token endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp GoogleTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("decode token response: %w", err)
	}

	return &tokenResp, nil
}

// fetchUserInfo calls Google's userinfo endpoint with the access token.
func (s *googleAuthService) fetchUserInfo(ctx context.Context, accessToken string) (*GoogleUserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.userURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create userinfo request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := s.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send userinfo request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("google userinfo endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	var userInfo GoogleUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return nil, fmt.Errorf("decode userinfo response: %w", err)
	}

	return &userInfo, nil
}

// upsertUser creates or updates the user in the DynamoDB Users table and
// returns the id of the account the person signs in to. linkedAtMs is non-zero
// when that account is an older password account that this sign-in took over
// (the moment its other sessions were revoked).
//
// createdAt, theme and every other attribute on an existing profile are kept on
// re-login: the item is read and written back with the Google fields updated.
func (s *googleAuthService) upsertUser(ctx context.Context, user *GoogleUserInfo) (string, int64, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	item := map[string]dbtypes.AttributeValue{}
	// isNew is true only when the read worked and found no profile: a failed
	// read says nothing about whether the account exists, so it is never
	// reported as a sign-up.
	isNew := false
	if existing, err := s.db.GetItem(ctx, &dynamodb.GetItemInput{
		TableName:      aws.String(s.config.UsersTable),
		Key:            userKey(user.ID),
		ConsistentRead: aws.Bool(true),
	}); err == nil {
		isNew = existing.Item == nil
		for k, v := range existing.Item {
			item[k] = v
		}
	}
	if extractStringAttr(item, "createdAt") == "" {
		item["createdAt"] = &dbtypes.AttributeValueMemberS{Value: now}
	}
	if extractStringAttr(item, "theme") == "" {
		item["theme"] = &dbtypes.AttributeValueMemberS{Value: "light"}
	}

	// Claim the email address so the same address cannot also register a
	// detached password account. A mapping to a DIFFERENT user means the
	// address already has an account; same-user or absent proceeds.
	emailIdxKey := emailKey(user.Email)
	if _, err := s.db.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:           aws.String(s.config.UsersTable),
		Item:                map[string]dbtypes.AttributeValue{"PK": emailIdxKey["PK"], "SK": emailIdxKey["SK"], "userId": &dbtypes.AttributeValueMemberS{Value: user.ID}},
		ConditionExpression: aws.String("attribute_not_exists(PK)"),
	}); err != nil {
		var condFailed *dbtypes.ConditionalCheckFailedException
		if !errors.As(err, &condFailed) {
			return "", 0, fmt.Errorf("reserve email: %w", err)
		}
		idx, ierr := s.db.GetItem(ctx, &dynamodb.GetItemInput{
			TableName:      aws.String(s.config.UsersTable),
			Key:            emailIdxKey,
			ConsistentRead: aws.Bool(true),
		})
		if ierr != nil {
			return "", 0, fmt.Errorf("lookup email: %w", ierr)
		}
		if idx.Item == nil {
			return "", 0, ErrEmailTaken
		}
		if owner := extractStringAttr(idx.Item, "userId"); owner != user.ID {
			return s.linkToPasswordAccount(ctx, owner, user)
		}
	}

	item["PK"] = userKey(user.ID)["PK"]
	item["SK"] = userKey(user.ID)["SK"]
	item["email"] = &dbtypes.AttributeValueMemberS{Value: user.Email}
	item["name"] = &dbtypes.AttributeValueMemberS{Value: user.Name}
	item["avatarUrl"] = &dbtypes.AttributeValueMemberS{Value: user.AvatarURL}
	if user.VerifiedEmail {
		item["emailVerified"] = &dbtypes.AttributeValueMemberBOOL{Value: true}
	}
	if _, err := s.db.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(s.config.UsersTable),
		Item:      item,
	}); err != nil {
		return "", 0, fmt.Errorf("put user item: %w", err)
	}
	if isNew {
		s.config.Signups.newAccount(ctx, user.Name, user.Email, "google")
	}
	return user.ID, 0, nil
}

// linkToPasswordAccount handles a Google sign-in whose address already belongs
// to another account. Only Google's statement that the address is verified can
// justify sharing an account, so without it the sign-in is refused as before.
//
// The owner must be a password account. A different Google account that once
// had the same address (a reassigned work address, say) is never merged in.
//
//   - If the password account never confirmed its address (the usual case is
//     someone who typed another person's address), the Google sign-in proves who
//     the address really belongs to. The account is handed over: its password is
//     removed and every earlier session is revoked, so whoever created it can no
//     longer get in. The Google id is stored on the account (googleId) so this
//     same Google account keeps signing in to it. This is what stops a stranger
//     squatting an address.
//   - If the address was confirmed, both sign-ins belong to the same person and
//     the Google sign-in simply opens that account. Nothing is removed.
func (s *googleAuthService) linkToPasswordAccount(ctx context.Context, ownerID string, user *GoogleUserInfo) (string, int64, error) {
	if !user.VerifiedEmail {
		return "", 0, ErrEmailTaken
	}
	profile, err := s.db.GetItem(ctx, &dynamodb.GetItemInput{
		TableName:      aws.String(s.config.UsersTable),
		Key:            userKey(ownerID),
		ConsistentRead: aws.Bool(true),
	})
	if err != nil {
		return "", 0, fmt.Errorf("lookup account: %w", err)
	}
	if profile.Item == nil {
		return "", 0, ErrEmailTaken
	}
	// This very Google account was handed the account earlier (the password was
	// removed then, so the check below would no longer recognise it).
	if linked := extractStringAttr(profile.Item, "googleId"); linked != "" {
		if linked == user.ID {
			return ownerID, 0, nil
		}
		return "", 0, ErrEmailTaken
	}
	if extractStringAttr(profile.Item, "passwordHash") == "" {
		return "", 0, ErrEmailTaken
	}
	// The address on the profile must still be the one the index points at.
	if !strings.EqualFold(extractStringAttr(profile.Item, "email"), user.Email) {
		return "", 0, ErrEmailTaken
	}
	if v, ok := profile.Item["emailVerified"].(*dbtypes.AttributeValueMemberBOOL); ok && v.Value {
		return ownerID, 0, nil
	}

	nowMs := time.Now().UnixMilli()
	_, err = s.db.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:           aws.String(s.config.UsersTable),
		Key:                 userKey(ownerID),
		UpdateExpression:    aws.String("SET emailVerified = :yes, emailVerifiedAt = :at, sessionsRevokedBeforeMs = :ms, googleId = :gid REMOVE passwordHash"),
		ConditionExpression: aws.String("attribute_exists(PK) AND attribute_exists(passwordHash)"),
		ExpressionAttributeValues: map[string]dbtypes.AttributeValue{
			":yes": &dbtypes.AttributeValueMemberBOOL{Value: true},
			":at":  &dbtypes.AttributeValueMemberS{Value: time.Now().UTC().Format(time.RFC3339)},
			":ms":  &dbtypes.AttributeValueMemberN{Value: strconv.FormatInt(nowMs, 10)},
			":gid": &dbtypes.AttributeValueMemberS{Value: user.ID},
		},
	})
	if err != nil {
		var cond *dbtypes.ConditionalCheckFailedException
		if errors.As(err, &cond) {
			// Someone else changed the account at the same moment; the person can
			// simply sign in again.
			return "", 0, ErrEmailTaken
		}
		return "", 0, fmt.Errorf("take over account: %w", err)
	}
	return ownerID, nowMs, nil
}

// generateAccessToken creates a signed JWT access token with the given claims.
func (s *googleAuthService) generateAccessToken(userID, email string, now time.Time) (string, error) {
	claims := jwt.MapClaims{
		"userId": userID,
		"email":  email,
		"iss":    TokenIssuer,
		"aud":    TokenAudience,
		"iat":    now.Unix(),
		"exp":    now.Add(AccessTokenTTL).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(s.config.JWTSecret))
	if err != nil {
		return "", fmt.Errorf("sign jwt: %w", err)
	}

	return signed, nil
}

// createSession generates a refresh token, hashes it, and stores the session
// in DynamoDB. The item key (PK=SESSION#hash, SK=SESSION) and attributes must
// match what JWTService.RefreshToken reads; a mismatch makes every refresh
// fail with "session not found" and logs users out.
func (s *googleAuthService) createSession(ctx context.Context, userID, email string, now time.Time) (string, error) {
	// Generate random refresh token
	tokenBytes := make([]byte, RefreshTokenBytes)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", fmt.Errorf("generate random token: %w", err)
	}
	refreshToken := hex.EncodeToString(tokenBytes)

	// Hash the refresh token for storage
	tokenHash := hashToken(refreshToken)

	// Store session in DynamoDB with TTL
	expiresAt := now.Add(RefreshTokenTTL).Unix()

	// Generate a new familyId for this login session
	familyId := uuid.New().String()

	item := map[string]dbtypes.AttributeValue{
		"PK":            &dbtypes.AttributeValueMemberS{Value: "SESSION#" + tokenHash},
		"SK":            &dbtypes.AttributeValueMemberS{Value: "SESSION"},
		"refreshToken":  &dbtypes.AttributeValueMemberS{Value: tokenHash},
		"userId":        &dbtypes.AttributeValueMemberS{Value: userID},
		"email":         &dbtypes.AttributeValueMemberS{Value: email},
		"familyId":      &dbtypes.AttributeValueMemberS{Value: familyId},
		"expiresAt":     &dbtypes.AttributeValueMemberN{Value: fmt.Sprintf("%d", expiresAt)},
		"createdAt":     &dbtypes.AttributeValueMemberS{Value: now.UTC().Format(time.RFC3339)},
		"familyStartMs": &dbtypes.AttributeValueMemberN{Value: strconv.FormatInt(now.UnixMilli(), 10)},
	}

	_, err := s.db.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(s.config.SessionTable),
		Item:      item,
	})
	if err != nil {
		return "", fmt.Errorf("store session: %w", err)
	}

	return refreshToken, nil
}

// hashToken computes a SHA-256 hash of the given token string.
func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}
