package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/rahul/indifferent/backend/internal/models"
)

// RotatedSessionTTL is the time a rotated session is kept for reuse detection.
const RotatedSessionTTL = 5 * time.Minute

var (
	// ErrInvalidToken is returned when a JWT token is malformed or has an invalid signature.
	ErrInvalidToken = errors.New("invalid token")
	// ErrExpiredToken is returned when a JWT token has expired.
	ErrExpiredToken = errors.New("token expired")
	// ErrSessionNotFound is returned when a refresh token session does not exist in DynamoDB.
	ErrSessionNotFound = errors.New("session not found")
	// ErrSessionExpired is returned when a refresh token session has expired.
	ErrSessionExpired = errors.New("session expired")
	// ErrTokenReuse is returned when a rotated refresh token is reused (possible theft).
	ErrTokenReuse = errors.New("refresh token reuse detected")
)

// JWTService provides JWT token management including validation, refresh, and logout.
type JWTService struct {
	Secret       string
	DB           DynamoDBClient
	SessionTable string
	UsersTable   string
}

// ValidateToken parses and validates a JWT token string.
// It verifies the HS256 signature and checks that the token has not expired.
// Returns the decoded claims on success, or an error on failure.
func (s *JWTService) ValidateToken(tokenString string) (*models.JWTClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(s.Secret), nil
	}, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	}

	if !token.Valid {
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, ErrInvalidToken
	}

	userID, _ := claims["userId"].(string)
	email, _ := claims["email"].(string)

	var exp int64
	if expVal, ok := claims["exp"].(float64); ok {
		exp = int64(expVal)
	}

	var iat int64
	if iatVal, ok := claims["iat"].(float64); ok {
		iat = int64(iatVal)
	}

	if userID == "" {
		return nil, ErrInvalidToken
	}

	return &models.JWTClaims{
		UserID: userID,
		Email:  email,
		Exp:    exp,
		Iat:    iat,
	}, nil
}

// ExtractUserID validates the token and returns just the userId claim.
func (s *JWTService) ExtractUserID(tokenString string) (string, error) {
	claims, err := s.ValidateToken(tokenString)
	if err != nil {
		return "", err
	}
	return claims.UserID, nil
}

// RefreshToken validates a refresh token and issues new access and refresh tokens.
// It implements token family reuse detection: if a rotated token is presented again,
// all sessions in that family are invalidated (theft detection).
func (s *JWTService) RefreshToken(ctx context.Context, refreshToken string) (*models.AuthTokens, error) {
	tokenHash := hashToken(refreshToken)

	// Look up session in DynamoDB.
	result, err := s.DB.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(s.SessionTable),
		Key: map[string]dbtypes.AttributeValue{
			"PK": &dbtypes.AttributeValueMemberS{Value: "SESSION#" + tokenHash},
			"SK": &dbtypes.AttributeValueMemberS{Value: "SESSION"},
		},
		ConsistentRead: aws.Bool(true),
	})
	if err != nil {
		return nil, fmt.Errorf("get session: %w", err)
	}

	if result.Item == nil {
		return nil, ErrSessionNotFound
	}

	// Check if this token was already rotated (reuse detection)
	if rotatedAttr, ok := result.Item["rotated"]; ok {
		if boolVal, ok := rotatedAttr.(*dbtypes.AttributeValueMemberBOOL); ok && boolVal.Value {
			// THEFT DETECTED! Invalidate all sessions in this family
			familyID := extractStringAttr(result.Item, "familyId")
			if familyID != "" {
				s.invalidateFamily(ctx, familyID)
			}
			return nil, ErrTokenReuse
		}
	}

	// Extract userId from the session item
	userIDAttr, ok := result.Item["userId"]
	if !ok {
		return nil, ErrSessionNotFound
	}
	userIDVal, ok := userIDAttr.(*dbtypes.AttributeValueMemberS)
	if !ok {
		return nil, ErrSessionNotFound
	}
	userID := userIDVal.Value

	// Check expiration
	if expiresAtAttr, ok := result.Item["expiresAt"]; ok {
		if expiresAtVal, ok := expiresAtAttr.(*dbtypes.AttributeValueMemberN); ok {
			expiresAt, err := strconv.ParseInt(expiresAtVal.Value, 10, 64)
			if err == nil && time.Now().Unix() > expiresAt {
				return nil, ErrSessionExpired
			}
		}
	}

	// Extract email and familyId from session
	email := extractStringAttr(result.Item, "email")
	familyID := extractStringAttr(result.Item, "familyId")

	// If no familyId exists (legacy session), generate one for forward compatibility
	if familyID == "" {
		familyID = uuid.New().String()
	}

	// Mark old session as rotated (keep for reuse detection window)
	s.markSessionRotated(ctx, tokenHash)

	// Generate new access token
	accessToken, err := s.GenerateAccessToken(userID, email)
	if err != nil {
		return nil, fmt.Errorf("generate access token: %w", err)
	}

	// Generate new refresh token and store new session with same familyId
	newRefreshToken, err := s.createSessionWithFamily(ctx, userID, email, familyID)
	if err != nil {
		return nil, fmt.Errorf("create new session: %w", err)
	}

	return &models.AuthTokens{
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
		ExpiresIn:    int64(AccessTokenTTL.Seconds()),
	}, nil
}

// Logout invalidates a session by deleting it from DynamoDB.
// If the session doesn't exist, this is a no-op (returns nil).
func (s *JWTService) Logout(ctx context.Context, refreshToken string) error {
	tokenHash := hashToken(refreshToken)

	_, err := s.DB.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(s.SessionTable),
		Key: map[string]dbtypes.AttributeValue{
			"PK": &dbtypes.AttributeValueMemberS{Value: "SESSION#" + tokenHash},
			"SK": &dbtypes.AttributeValueMemberS{Value: "SESSION"},
		},
	})
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}

	return nil
}

// GenerateAccessToken creates a signed JWT access token with the given user info.
func (s *JWTService) GenerateAccessToken(userID, email string) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"userId": userID,
		"email":  email,
		"iat":    now.Unix(),
		"exp":    now.Add(AccessTokenTTL).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(s.Secret))
	if err != nil {
		return "", fmt.Errorf("sign jwt: %w", err)
	}

	return signed, nil
}

// createSession generates a new refresh token, hashes it, and stores the session in DynamoDB
// with a new familyId (used on initial login).
func (s *JWTService) createSession(ctx context.Context, userID, email string) (string, error) {
	familyID := uuid.New().String()
	return s.createSessionWithFamily(ctx, userID, email, familyID)
}

// createSessionWithFamily generates a new refresh token and stores the session with
// the given familyId. Used during token rotation to preserve the family chain.
func (s *JWTService) createSessionWithFamily(ctx context.Context, userID, email, familyID string) (string, error) {
	tokenBytes := make([]byte, RefreshTokenBytes)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", fmt.Errorf("generate random token: %w", err)
	}
	refreshToken := hex.EncodeToString(tokenBytes)

	tokenHash := hashToken(refreshToken)
	now := time.Now()
	expiresAt := now.Add(RefreshTokenTTL).Unix()

	item := map[string]dbtypes.AttributeValue{
		"PK":        &dbtypes.AttributeValueMemberS{Value: "SESSION#" + tokenHash},
		"SK":        &dbtypes.AttributeValueMemberS{Value: "SESSION"},
		"userId":    &dbtypes.AttributeValueMemberS{Value: userID},
		"email":     &dbtypes.AttributeValueMemberS{Value: email},
		"familyId":  &dbtypes.AttributeValueMemberS{Value: familyID},
		"expiresAt": &dbtypes.AttributeValueMemberN{Value: fmt.Sprintf("%d", expiresAt)},
		"createdAt": &dbtypes.AttributeValueMemberS{Value: now.UTC().Format(time.RFC3339)},
	}

	_, err := s.DB.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(s.SessionTable),
		Item:      item,
	})
	if err != nil {
		return "", fmt.Errorf("store session: %w", err)
	}

	return refreshToken, nil
}

// markSessionRotated marks a session as already used. The session is kept
// for a short window (RotatedSessionTTL) so that reuse can be detected.
func (s *JWTService) markSessionRotated(ctx context.Context, tokenHash string) {
	rotatedExpiresAt := time.Now().Add(RotatedSessionTTL).Unix()

	_, _ = s.DB.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(s.SessionTable),
		Key: map[string]dbtypes.AttributeValue{
			"PK": &dbtypes.AttributeValueMemberS{Value: "SESSION#" + tokenHash},
			"SK": &dbtypes.AttributeValueMemberS{Value: "SESSION"},
		},
		UpdateExpression: aws.String("SET rotated = :r, expiresAt = :e"),
		ExpressionAttributeValues: map[string]dbtypes.AttributeValue{
			":r": &dbtypes.AttributeValueMemberBOOL{Value: true},
			":e": &dbtypes.AttributeValueMemberN{Value: fmt.Sprintf("%d", rotatedExpiresAt)},
		},
	})
}

// invalidateFamily deletes all sessions belonging to the given token family.
// This is called when token reuse is detected to protect the user.
func (s *JWTService) invalidateFamily(ctx context.Context, familyID string) {
	// Scan for all sessions with matching familyId
	scanOutput, err := s.DB.Scan(ctx, &dynamodb.ScanInput{
		TableName:        aws.String(s.SessionTable),
		FilterExpression: aws.String("familyId = :fid"),
		ExpressionAttributeValues: map[string]dbtypes.AttributeValue{
			":fid": &dbtypes.AttributeValueMemberS{Value: familyID},
		},
	})
	if err != nil {
		return
	}

	// Delete each session in the family
	for _, item := range scanOutput.Items {
		pkAttr, pkOk := item["PK"].(*dbtypes.AttributeValueMemberS)
		skAttr, skOk := item["SK"].(*dbtypes.AttributeValueMemberS)
		if !pkOk || !skOk {
			continue
		}
		_, _ = s.DB.DeleteItem(ctx, &dynamodb.DeleteItemInput{
			TableName: aws.String(s.SessionTable),
			Key: map[string]dbtypes.AttributeValue{
				"PK": &dbtypes.AttributeValueMemberS{Value: pkAttr.Value},
				"SK": &dbtypes.AttributeValueMemberS{Value: skAttr.Value},
			},
		})
	}
}

// extractStringAttr extracts a string attribute from a DynamoDB item.
func extractStringAttr(item map[string]dbtypes.AttributeValue, key string) string {
	if attr, ok := item[key]; ok {
		if val, ok := attr.(*dbtypes.AttributeValueMemberS); ok {
			return val.Value
		}
	}
	return ""
}
