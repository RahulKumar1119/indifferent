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
	"github.com/rahul/indifferent/backend/internal/models"
)

var (
	// ErrInvalidToken is returned when a JWT token is malformed or has an invalid signature.
	ErrInvalidToken = errors.New("invalid token")
	// ErrExpiredToken is returned when a JWT token has expired.
	ErrExpiredToken = errors.New("token expired")
	// ErrSessionNotFound is returned when a refresh token session does not exist in DynamoDB.
	ErrSessionNotFound = errors.New("session not found")
	// ErrSessionExpired is returned when a refresh token session has expired.
	ErrSessionExpired = errors.New("session expired")
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
// It hashes the provided refresh token, looks up the session in DynamoDB,
// verifies it hasn't expired, deletes the old session, and creates a new one.
func (s *JWTService) RefreshToken(ctx context.Context, refreshToken string) (*models.AuthTokens, error) {
	tokenHash := hashToken(refreshToken)

	// Look up session in DynamoDB.
	// Sessions created by JWTService use PK=SESSION#<hash>, SK=SESSION.
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

	// Extract email from session
	var email string
	if emailAttr, ok := result.Item["email"]; ok {
		if emailVal, ok := emailAttr.(*dbtypes.AttributeValueMemberS); ok {
			email = emailVal.Value
		}
	}

	// Delete old session
	_, _ = s.DB.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(s.SessionTable),
		Key: map[string]dbtypes.AttributeValue{
			"PK": &dbtypes.AttributeValueMemberS{Value: "SESSION#" + tokenHash},
			"SK": &dbtypes.AttributeValueMemberS{Value: "SESSION"},
		},
	})

	// Generate new access token
	accessToken, err := s.GenerateAccessToken(userID, email)
	if err != nil {
		return nil, fmt.Errorf("generate access token: %w", err)
	}

	// Generate new refresh token and store new session
	newRefreshToken, err := s.createSession(ctx, userID, email)
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

// createSession generates a new refresh token, hashes it, and stores the session in DynamoDB.
func (s *JWTService) createSession(ctx context.Context, userID, email string) (string, error) {
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
