package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/golang-jwt/jwt/v5"
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
}

// HTTPClient defines the HTTP operations needed by the auth service.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// GoogleAuthService provides Google OAuth authentication and token management.
type GoogleAuthService interface {
	// Authenticate exchanges a Google authorization code for user info,
	// creates or updates the user in DynamoDB, and returns auth tokens.
	Authenticate(ctx context.Context, authCode string) (*models.AuthTokens, error)
}

// GoogleAuthConfig holds the configuration for Google OAuth.
type GoogleAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
	JWTSecret    string
	UsersTable   string
	SessionTable string
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
func (s *googleAuthService) Authenticate(ctx context.Context, authCode string) (*models.AuthTokens, error) {
	// Step 1: Exchange authorization code for Google access token
	googleToken, err := s.exchangeCode(ctx, authCode)
	if err != nil {
		return nil, fmt.Errorf("exchange auth code: %w", err)
	}

	// Step 2: Fetch user info from Google
	userInfo, err := s.fetchUserInfo(ctx, googleToken.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("fetch user info: %w", err)
	}

	// Step 3: Create or update user in DynamoDB Users table
	if err := s.upsertUser(ctx, userInfo); err != nil {
		return nil, fmt.Errorf("upsert user: %w", err)
	}

	// Step 4: Generate JWT access token
	now := time.Now()
	accessToken, err := s.generateAccessToken(userInfo.ID, userInfo.Email, now)
	if err != nil {
		return nil, fmt.Errorf("generate access token: %w", err)
	}

	// Step 5: Generate refresh token and store session
	refreshToken, err := s.createSession(ctx, userInfo.ID, now)
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
func (s *googleAuthService) exchangeCode(ctx context.Context, code string) (*GoogleTokenResponse, error) {
	data := url.Values{
		"code":          {code},
		"client_id":     {s.config.ClientID},
		"client_secret": {s.config.ClientSecret},
		"redirect_uri":  {s.config.RedirectURI},
		"grant_type":    {"authorization_code"},
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

// upsertUser creates or updates the user in the DynamoDB Users table.
func (s *googleAuthService) upsertUser(ctx context.Context, user *GoogleUserInfo) error {
	now := time.Now().UTC().Format(time.RFC3339)

	item := map[string]dbtypes.AttributeValue{
		"PK":        &dbtypes.AttributeValueMemberS{Value: "USER#" + user.ID},
		"SK":        &dbtypes.AttributeValueMemberS{Value: "PROFILE"},
		"email":     &dbtypes.AttributeValueMemberS{Value: user.Email},
		"name":      &dbtypes.AttributeValueMemberS{Value: user.Name},
		"avatarUrl": &dbtypes.AttributeValueMemberS{Value: user.AvatarURL},
		"createdAt": &dbtypes.AttributeValueMemberS{Value: now},
		"theme":     &dbtypes.AttributeValueMemberS{Value: "light"},
	}

	_, err := s.db.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(s.config.UsersTable),
		Item:      item,
	})
	if err != nil {
		return fmt.Errorf("put user item: %w", err)
	}

	return nil
}

// generateAccessToken creates a signed JWT access token with the given claims.
func (s *googleAuthService) generateAccessToken(userID, email string, now time.Time) (string, error) {
	claims := jwt.MapClaims{
		"userId": userID,
		"email":  email,
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

// createSession generates a refresh token, hashes it, and stores the session in DynamoDB.
func (s *googleAuthService) createSession(ctx context.Context, userID string, now time.Time) (string, error) {
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

	item := map[string]dbtypes.AttributeValue{
		"PK":           &dbtypes.AttributeValueMemberS{Value: "SESSION#" + tokenHash},
		"SK":           &dbtypes.AttributeValueMemberS{Value: "USER#" + userID},
		"refreshToken": &dbtypes.AttributeValueMemberS{Value: tokenHash},
		"expiresAt":    &dbtypes.AttributeValueMemberN{Value: fmt.Sprintf("%d", expiresAt)},
		"createdAt":    &dbtypes.AttributeValueMemberS{Value: now.UTC().Format(time.RFC3339)},
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
