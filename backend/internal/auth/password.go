package auth

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/rahul/indifferent/backend/internal/models"
)

// minPasswordLength is the minimum accepted password size. bcrypt silently
// truncates past maxBcryptPasswordBytes, so longer input is rejected instead.
const (
	minPasswordLength      = 8
	maxBcryptPasswordBytes = 72
)

// ErrEmailTaken is returned on signup when the address is already registered.
var ErrEmailTaken = fmt.Errorf("email already registered")

// ErrInvalidCredentials is the single error used for every login failure
// (unknown email, bad password) so callers cannot enumerate accounts.
var ErrInvalidCredentials = fmt.Errorf("invalid email or password")

// PasswordAuthService provides email/password signup and login backed by the
// same users table and session model as Google OAuth: sessions it creates are
// refreshable through JWTService.RefreshToken without further changes.
type PasswordAuthService struct {
	DB           DynamoDBClient
	JWT          *JWTService
	UsersTable   string
	SessionTable string
}

// NewPasswordAuthService builds a PasswordAuthService sharing the JWT service
// used for token issuance and rotation.
func NewPasswordAuthService(db DynamoDBClient, jwt *JWTService, usersTable, sessionTable string) *PasswordAuthService {
	return &PasswordAuthService{DB: db, JWT: jwt, UsersTable: usersTable, SessionTable: sessionTable}
}

// normalizeEmail trims and lowercases an address so lookup and uniqueness are
// case-insensitive.
func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// validateCredentials enforces email shape and password bounds before any I/O.
func validateCredentials(email, password string) error {
	if _, err := mail.ParseAddress(normalizeEmail(email)); err != nil {
		return fmt.Errorf("invalid email address")
	}
	if len(password) < minPasswordLength {
		return fmt.Errorf("password must be at least %d characters", minPasswordLength)
	}
	if len(password) > maxBcryptPasswordBytes {
		return fmt.Errorf("password must be at most %d characters", maxBcryptPasswordBytes)
	}
	return nil
}

// userKey returns the primary key of a user profile item.
func userKey(userID string) map[string]dbtypes.AttributeValue {
	return map[string]dbtypes.AttributeValue{
		"PK": &dbtypes.AttributeValueMemberS{Value: "USER#" + userID},
		"SK": &dbtypes.AttributeValueMemberS{Value: "PROFILE"},
	}
}

// emailKey returns the primary key of the email→userId index item.
func emailKey(email string) map[string]dbtypes.AttributeValue {
	return map[string]dbtypes.AttributeValue{
		"PK": &dbtypes.AttributeValueMemberS{Value: "EMAIL#" + normalizeEmail(email)},
		"SK": &dbtypes.AttributeValueMemberS{Value: "PROFILE"},
	}
}

// SignUp registers a new email/password user, creates a session family, and
// returns the initial token pair (same shape as Google login).
func (s *PasswordAuthService) SignUp(ctx context.Context, email, name, password string) (*models.AuthTokens, error) {
	email = normalizeEmail(email)
	if err := validateCredentials(email, password); err != nil {
		return nil, err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	userID := uuid.New().String()
	now := time.Now().UTC().Format(time.RFC3339)

	// Reserve the address first so a duplicate signup fails fast even if the
	// profile write below has not happened yet.
	idxKey := emailKey(email)
	if _, err := s.DB.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:           aws.String(s.UsersTable),
		Item:                map[string]dbtypes.AttributeValue{"PK": idxKey["PK"], "SK": idxKey["SK"], "userId": &dbtypes.AttributeValueMemberS{Value: userID}},
		ConditionExpression: aws.String("attribute_not_exists(PK)"),
	}); err != nil {
		var condFailed *dbtypes.ConditionalCheckFailedException
		if errors.As(err, &condFailed) {
			return nil, ErrEmailTaken
		}
		return nil, fmt.Errorf("reserve email: %w", err)
	}

	if _, err := s.DB.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(s.UsersTable),
		Item: map[string]dbtypes.AttributeValue{
			"PK":           userKey(userID)["PK"],
			"SK":           userKey(userID)["SK"],
			"email":        &dbtypes.AttributeValueMemberS{Value: email},
			"name":         &dbtypes.AttributeValueMemberS{Value: name},
			"passwordHash": &dbtypes.AttributeValueMemberS{Value: string(hash)},
			"createdAt":    &dbtypes.AttributeValueMemberS{Value: now},
		},
	}); err != nil {
		// Best effort rollback of the reservation; the next signup retry
		// surfaces ErrEmailTaken only if this delete also failed.
		_, _ = s.DB.DeleteItem(ctx, &dynamodb.DeleteItemInput{
			TableName: aws.String(s.UsersTable),
			Key:       emailKey(email),
		})
		return nil, fmt.Errorf("store user: %w", err)
	}

	return s.issueTokens(ctx, userID, email)
}

// Login verifies an email/password pair and returns a fresh token pair. The
// error is identical for unknown addresses and wrong passwords.
func (s *PasswordAuthService) Login(ctx context.Context, email, password string) (*models.AuthTokens, error) {
	email = normalizeEmail(email)
	if strings.TrimSpace(email) == "" || password == "" {
		return nil, ErrInvalidCredentials
	}

	idx, err := s.DB.GetItem(ctx, &dynamodb.GetItemInput{
		TableName:      aws.String(s.UsersTable),
		Key:            emailKey(email),
		ConsistentRead: aws.Bool(true),
	})
	if err != nil {
		return nil, fmt.Errorf("lookup user: %w", err)
	}
	if idx.Item == nil {
		return nil, ErrInvalidCredentials
	}
	userID := extractStringAttr(idx.Item, "userId")
	if userID == "" {
		return nil, ErrInvalidCredentials
	}

	profile, err := s.DB.GetItem(ctx, &dynamodb.GetItemInput{
		TableName:      aws.String(s.UsersTable),
		Key:            userKey(userID),
		ConsistentRead: aws.Bool(true),
	})
	if err != nil {
		return nil, fmt.Errorf("lookup user: %w", err)
	}
	if profile.Item == nil {
		return nil, ErrInvalidCredentials
	}
	stored, ok := profile.Item["passwordHash"].(*dbtypes.AttributeValueMemberS)
	if !ok || stored.Value == "" {
		// OAuth-only account (e.g. Gmail signup): no password to check.
		return nil, ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(stored.Value), []byte(password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	return s.issueTokens(ctx, userID, email)
}

// issueTokens mints an access token plus a fresh session family for the user.
func (s *PasswordAuthService) issueTokens(ctx context.Context, userID, email string) (*models.AuthTokens, error) {
	accessToken, err := s.JWT.GenerateAccessToken(userID, email)
	if err != nil {
		return nil, fmt.Errorf("generate access token: %w", err)
	}
	refreshToken, err := s.JWT.createSessionWithFamily(ctx, userID, email, uuid.New().String())
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}
	return &models.AuthTokens{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(AccessTokenTTL.Seconds()),
	}, nil
}
