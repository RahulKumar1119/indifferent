package auth

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/golang-jwt/jwt/v5"
)

const jwtTestSecret = "test-secret-key-for-jwt-signing"

func newTestJWTService(db DynamoDBClient) *JWTService {
	return &JWTService{
		Secret:       jwtTestSecret,
		DB:           db,
		SessionTable: "TestSessions",
		UsersTable:   "TestUsers",
	}
}

func generateTestToken(userID, email string, expiration time.Time, secret string) string {
	claims := jwt.MapClaims{
		"userId": userID,
		"email":  email,
		"iat":    time.Now().Unix(),
		"exp":    expiration.Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, _ := token.SignedString([]byte(secret))
	return signed
}

func TestValidateToken_ValidToken(t *testing.T) {
	svc := newTestJWTService(&mockDynamoDBClient{})

	tokenStr := generateTestToken("user-123", "test@example.com", time.Now().Add(15*time.Minute), jwtTestSecret)

	claims, err := svc.ValidateToken(tokenStr)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if claims.UserID != "user-123" {
		t.Errorf("expected userID 'user-123', got '%s'", claims.UserID)
	}
	if claims.Email != "test@example.com" {
		t.Errorf("expected email 'test@example.com', got '%s'", claims.Email)
	}
	if claims.Exp == 0 {
		t.Error("expected non-zero exp claim")
	}
	if claims.Iat == 0 {
		t.Error("expected non-zero iat claim")
	}
}

func TestValidateToken_ExpiredToken(t *testing.T) {
	svc := newTestJWTService(&mockDynamoDBClient{})

	tokenStr := generateTestToken("user-123", "test@example.com", time.Now().Add(-1*time.Hour), jwtTestSecret)

	_, err := svc.ValidateToken(tokenStr)
	if err == nil {
		t.Fatal("expected error for expired token, got nil")
	}
	if err != ErrExpiredToken {
		t.Errorf("expected ErrExpiredToken, got: %v", err)
	}
}

func TestValidateToken_InvalidSignature(t *testing.T) {
	svc := newTestJWTService(&mockDynamoDBClient{})

	tokenStr := generateTestToken("user-123", "test@example.com", time.Now().Add(15*time.Minute), "wrong-secret")

	_, err := svc.ValidateToken(tokenStr)
	if err == nil {
		t.Fatal("expected error for invalid signature, got nil")
	}
	if err != ErrInvalidToken {
		t.Errorf("expected ErrInvalidToken, got: %v", err)
	}
}

func TestValidateToken_MalformedToken(t *testing.T) {
	svc := newTestJWTService(&mockDynamoDBClient{})

	_, err := svc.ValidateToken("not-a-jwt-token")
	if err == nil {
		t.Fatal("expected error for malformed token, got nil")
	}
	if err != ErrInvalidToken {
		t.Errorf("expected ErrInvalidToken, got: %v", err)
	}
}

func TestValidateToken_MissingUserID(t *testing.T) {
	svc := newTestJWTService(&mockDynamoDBClient{})

	// Token without userId claim
	claims := jwt.MapClaims{
		"email": "test@example.com",
		"iat":   time.Now().Unix(),
		"exp":   time.Now().Add(15 * time.Minute).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, _ := token.SignedString([]byte(jwtTestSecret))

	_, err := svc.ValidateToken(tokenStr)
	if err == nil {
		t.Fatal("expected error for missing userId, got nil")
	}
	if err != ErrInvalidToken {
		t.Errorf("expected ErrInvalidToken, got: %v", err)
	}
}

func TestExtractUserID_ValidToken(t *testing.T) {
	svc := newTestJWTService(&mockDynamoDBClient{})

	tokenStr := generateTestToken("user-456", "user@example.com", time.Now().Add(15*time.Minute), jwtTestSecret)

	userID, err := svc.ExtractUserID(tokenStr)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if userID != "user-456" {
		t.Errorf("expected 'user-456', got '%s'", userID)
	}
}

func TestExtractUserID_InvalidToken(t *testing.T) {
	svc := newTestJWTService(&mockDynamoDBClient{})

	_, err := svc.ExtractUserID("invalid-token")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestRefreshToken_ValidRefreshToken(t *testing.T) {
	refreshToken := "valid-refresh-token-hex-string"
	tokenHash := hashToken(refreshToken)

	mock := &mockDynamoDBClient{
		getItemFunc: func(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
			pk := params.Key["PK"].(*dbtypes.AttributeValueMemberS).Value
			expectedPK := "SESSION#" + tokenHash
			if pk != expectedPK {
				t.Errorf("expected PK '%s', got '%s'", expectedPK, pk)
			}

			return &dynamodb.GetItemOutput{
				Item: map[string]dbtypes.AttributeValue{
					"PK":        &dbtypes.AttributeValueMemberS{Value: "SESSION#" + tokenHash},
					"SK":        &dbtypes.AttributeValueMemberS{Value: "SESSION"},
					"userId":    &dbtypes.AttributeValueMemberS{Value: "user-789"},
					"email":     &dbtypes.AttributeValueMemberS{Value: "refresh@example.com"},
					"expiresAt": &dbtypes.AttributeValueMemberN{Value: fmt.Sprintf("%d", time.Now().Add(7*24*time.Hour).Unix())},
					"createdAt": &dbtypes.AttributeValueMemberS{Value: time.Now().UTC().Format(time.RFC3339)},
				},
			}, nil
		},
		deleteItemFunc: func(ctx context.Context, params *dynamodb.DeleteItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error) {
			return &dynamodb.DeleteItemOutput{}, nil
		},
		putItemFunc: func(ctx context.Context, params *dynamodb.PutItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
			if params.TableName == nil || *params.TableName != "TestSessions" {
				t.Errorf("expected table 'TestSessions', got '%v'", params.TableName)
			}
			return &dynamodb.PutItemOutput{}, nil
		},
	}

	svc := newTestJWTService(mock)

	tokens, err := svc.RefreshToken(context.Background(), refreshToken)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if tokens.AccessToken == "" {
		t.Error("expected non-empty access token")
	}
	if tokens.RefreshToken == "" {
		t.Error("expected non-empty refresh token")
	}
	if tokens.ExpiresIn != int64(AccessTokenTTL.Seconds()) {
		t.Errorf("expected ExpiresIn %d, got %d", int64(AccessTokenTTL.Seconds()), tokens.ExpiresIn)
	}

	// Verify the new access token is valid
	claims, err := svc.ValidateToken(tokens.AccessToken)
	if err != nil {
		t.Fatalf("new access token should be valid: %v", err)
	}
	if claims.UserID != "user-789" {
		t.Errorf("expected userId 'user-789', got '%s'", claims.UserID)
	}
	if claims.Email != "refresh@example.com" {
		t.Errorf("expected email 'refresh@example.com', got '%s'", claims.Email)
	}
}

func TestRefreshToken_SessionNotFound(t *testing.T) {
	mock := &mockDynamoDBClient{
		getItemFunc: func(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
			return &dynamodb.GetItemOutput{Item: nil}, nil
		},
	}

	svc := newTestJWTService(mock)

	_, err := svc.RefreshToken(context.Background(), "unknown-refresh-token")
	if err == nil {
		t.Fatal("expected error for unknown refresh token, got nil")
	}
	if err != ErrSessionNotFound {
		t.Errorf("expected ErrSessionNotFound, got: %v", err)
	}
}

func TestRefreshToken_SessionExpired(t *testing.T) {
	refreshToken := "expired-session-token"
	tokenHash := hashToken(refreshToken)

	mock := &mockDynamoDBClient{
		getItemFunc: func(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
			return &dynamodb.GetItemOutput{
				Item: map[string]dbtypes.AttributeValue{
					"PK":        &dbtypes.AttributeValueMemberS{Value: "SESSION#" + tokenHash},
					"SK":        &dbtypes.AttributeValueMemberS{Value: "SESSION"},
					"userId":    &dbtypes.AttributeValueMemberS{Value: "user-expired"},
					"email":     &dbtypes.AttributeValueMemberS{Value: "expired@example.com"},
					"expiresAt": &dbtypes.AttributeValueMemberN{Value: fmt.Sprintf("%d", time.Now().Add(-1*time.Hour).Unix())},
					"createdAt": &dbtypes.AttributeValueMemberS{Value: time.Now().UTC().Format(time.RFC3339)},
				},
			}, nil
		},
	}

	svc := newTestJWTService(mock)

	_, err := svc.RefreshToken(context.Background(), refreshToken)
	if err == nil {
		t.Fatal("expected error for expired session, got nil")
	}
	if err != ErrSessionExpired {
		t.Errorf("expected ErrSessionExpired, got: %v", err)
	}
}

func TestLogout_DeletesSession(t *testing.T) {
	refreshToken := "logout-refresh-token"
	tokenHash := hashToken(refreshToken)
	deleteCalled := false

	mock := &mockDynamoDBClient{
		deleteItemFunc: func(ctx context.Context, params *dynamodb.DeleteItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error) {
			deleteCalled = true
			pk := params.Key["PK"].(*dbtypes.AttributeValueMemberS).Value
			expectedPK := "SESSION#" + tokenHash
			if pk != expectedPK {
				t.Errorf("expected PK '%s', got '%s'", expectedPK, pk)
			}
			sk := params.Key["SK"].(*dbtypes.AttributeValueMemberS).Value
			if sk != "SESSION" {
				t.Errorf("expected SK 'SESSION', got '%s'", sk)
			}
			if *params.TableName != "TestSessions" {
				t.Errorf("expected table 'TestSessions', got '%s'", *params.TableName)
			}
			return &dynamodb.DeleteItemOutput{}, nil
		},
	}

	svc := newTestJWTService(mock)

	err := svc.Logout(context.Background(), refreshToken)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !deleteCalled {
		t.Error("expected DeleteItem to be called")
	}
}

func TestLogout_NonExistentSession(t *testing.T) {
	mock := &mockDynamoDBClient{
		deleteItemFunc: func(ctx context.Context, params *dynamodb.DeleteItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error) {
			return &dynamodb.DeleteItemOutput{}, nil
		},
	}

	svc := newTestJWTService(mock)

	err := svc.Logout(context.Background(), "non-existent-token")
	if err != nil {
		t.Fatalf("expected no error for non-existent session, got: %v", err)
	}
}

func TestGenerateAccessToken(t *testing.T) {
	svc := newTestJWTService(&mockDynamoDBClient{})

	tokenStr, err := svc.GenerateAccessToken("user-gen", "gen@example.com")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if tokenStr == "" {
		t.Fatal("expected non-empty token")
	}

	// Validate the generated token
	claims, err := svc.ValidateToken(tokenStr)
	if err != nil {
		t.Fatalf("generated token should be valid: %v", err)
	}
	if claims.UserID != "user-gen" {
		t.Errorf("expected userId 'user-gen', got '%s'", claims.UserID)
	}
	if claims.Email != "gen@example.com" {
		t.Errorf("expected email 'gen@example.com', got '%s'", claims.Email)
	}

	// Verify expiration is approximately AccessTokenTTL from now
	expectedExp := time.Now().Add(AccessTokenTTL).Unix()
	if claims.Exp < expectedExp-2 || claims.Exp > expectedExp+2 {
		t.Errorf("expected exp around %d, got %d", expectedExp, claims.Exp)
	}
}

func TestRefreshToken_ReuseDetection(t *testing.T) {
	// Simulate a rotated token being reused - should return ErrTokenReuse
	refreshToken := "already-rotated-token"
	tokenHash := hashToken(refreshToken)
	familyID := "family-abc-123"

	scanCalled := false
	deletedKeys := []string{}

	mock := &mockDynamoDBClient{
		getItemFunc: func(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
			return &dynamodb.GetItemOutput{
				Item: map[string]dbtypes.AttributeValue{
					"PK":        &dbtypes.AttributeValueMemberS{Value: "SESSION#" + tokenHash},
					"SK":        &dbtypes.AttributeValueMemberS{Value: "SESSION"},
					"userId":    &dbtypes.AttributeValueMemberS{Value: "user-victim"},
					"email":     &dbtypes.AttributeValueMemberS{Value: "victim@example.com"},
					"familyId":  &dbtypes.AttributeValueMemberS{Value: familyID},
					"rotated":   &dbtypes.AttributeValueMemberBOOL{Value: true},
					"expiresAt": &dbtypes.AttributeValueMemberN{Value: fmt.Sprintf("%d", time.Now().Add(5*time.Minute).Unix())},
				},
			}, nil
		},
		scanFunc: func(ctx context.Context, params *dynamodb.ScanInput, optFns ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error) {
			scanCalled = true
			// Return two sessions in the same family
			return &dynamodb.ScanOutput{
				Items: []map[string]dbtypes.AttributeValue{
					{
						"PK": &dbtypes.AttributeValueMemberS{Value: "SESSION#hash1"},
						"SK": &dbtypes.AttributeValueMemberS{Value: "SESSION"},
					},
					{
						"PK": &dbtypes.AttributeValueMemberS{Value: "SESSION#hash2"},
						"SK": &dbtypes.AttributeValueMemberS{Value: "SESSION"},
					},
				},
			}, nil
		},
		deleteItemFunc: func(ctx context.Context, params *dynamodb.DeleteItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error) {
			pk := params.Key["PK"].(*dbtypes.AttributeValueMemberS).Value
			deletedKeys = append(deletedKeys, pk)
			return &dynamodb.DeleteItemOutput{}, nil
		},
	}

	svc := newTestJWTService(mock)

	_, err := svc.RefreshToken(context.Background(), refreshToken)
	if err == nil {
		t.Fatal("expected error for reused rotated token, got nil")
	}
	if err != ErrTokenReuse {
		t.Errorf("expected ErrTokenReuse, got: %v", err)
	}
	if !scanCalled {
		t.Error("expected Scan to be called for family invalidation")
	}
	if len(deletedKeys) != 2 {
		t.Errorf("expected 2 sessions deleted, got %d", len(deletedKeys))
	}
}

func TestRefreshToken_RotationIncludesFamilyId(t *testing.T) {
	// Normal rotation should mark old session as rotated and create new session with same familyId
	refreshToken := "valid-token-for-rotation"
	tokenHash := hashToken(refreshToken)
	familyID := "family-xyz-789"

	updateCalled := false
	var createdFamilyId string

	mock := &mockDynamoDBClient{
		getItemFunc: func(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
			return &dynamodb.GetItemOutput{
				Item: map[string]dbtypes.AttributeValue{
					"PK":        &dbtypes.AttributeValueMemberS{Value: "SESSION#" + tokenHash},
					"SK":        &dbtypes.AttributeValueMemberS{Value: "SESSION"},
					"userId":    &dbtypes.AttributeValueMemberS{Value: "user-normal"},
					"email":     &dbtypes.AttributeValueMemberS{Value: "normal@example.com"},
					"familyId":  &dbtypes.AttributeValueMemberS{Value: familyID},
					"expiresAt": &dbtypes.AttributeValueMemberN{Value: fmt.Sprintf("%d", time.Now().Add(7*24*time.Hour).Unix())},
				},
			}, nil
		},
		updateItemFunc: func(ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
			updateCalled = true
			// Verify it's marking the session as rotated
			pk := params.Key["PK"].(*dbtypes.AttributeValueMemberS).Value
			if pk != "SESSION#"+tokenHash {
				t.Errorf("expected update on SESSION#%s, got %s", tokenHash, pk)
			}
			return &dynamodb.UpdateItemOutput{}, nil
		},
		putItemFunc: func(ctx context.Context, params *dynamodb.PutItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
			// Capture the familyId from the new session
			if fid, ok := params.Item["familyId"]; ok {
				if fidVal, ok := fid.(*dbtypes.AttributeValueMemberS); ok {
					createdFamilyId = fidVal.Value
				}
			}
			return &dynamodb.PutItemOutput{}, nil
		},
	}

	svc := newTestJWTService(mock)

	tokens, err := svc.RefreshToken(context.Background(), refreshToken)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if tokens.AccessToken == "" {
		t.Error("expected non-empty access token")
	}
	if tokens.RefreshToken == "" {
		t.Error("expected non-empty refresh token")
	}
	if !updateCalled {
		t.Error("expected UpdateItem to be called to mark old session as rotated")
	}
	if createdFamilyId != familyID {
		t.Errorf("expected new session familyId '%s', got '%s'", familyID, createdFamilyId)
	}
}

func TestRefreshToken_LegacySessionWithoutFamilyId(t *testing.T) {
	// Sessions without familyId (legacy) should still work - a new familyId is generated
	refreshToken := "legacy-token-no-family"
	tokenHash := hashToken(refreshToken)

	var createdFamilyId string

	mock := &mockDynamoDBClient{
		getItemFunc: func(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
			return &dynamodb.GetItemOutput{
				Item: map[string]dbtypes.AttributeValue{
					"PK":        &dbtypes.AttributeValueMemberS{Value: "SESSION#" + tokenHash},
					"SK":        &dbtypes.AttributeValueMemberS{Value: "SESSION"},
					"userId":    &dbtypes.AttributeValueMemberS{Value: "user-legacy"},
					"email":     &dbtypes.AttributeValueMemberS{Value: "legacy@example.com"},
					"expiresAt": &dbtypes.AttributeValueMemberN{Value: fmt.Sprintf("%d", time.Now().Add(7*24*time.Hour).Unix())},
				},
			}, nil
		},
		updateItemFunc: func(ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
			return &dynamodb.UpdateItemOutput{}, nil
		},
		putItemFunc: func(ctx context.Context, params *dynamodb.PutItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
			if fid, ok := params.Item["familyId"]; ok {
				if fidVal, ok := fid.(*dbtypes.AttributeValueMemberS); ok {
					createdFamilyId = fidVal.Value
				}
			}
			return &dynamodb.PutItemOutput{}, nil
		},
	}

	svc := newTestJWTService(mock)

	tokens, err := svc.RefreshToken(context.Background(), refreshToken)
	if err != nil {
		t.Fatalf("expected no error for legacy session, got: %v", err)
	}
	if tokens.AccessToken == "" {
		t.Error("expected non-empty access token")
	}
	if createdFamilyId == "" {
		t.Error("expected a familyId to be generated for legacy session")
	}
}

func TestRefreshToken_NonRotatedSessionWorks(t *testing.T) {
	// A session with rotated=false should work normally
	refreshToken := "non-rotated-token"
	tokenHash := hashToken(refreshToken)

	mock := &mockDynamoDBClient{
		getItemFunc: func(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
			return &dynamodb.GetItemOutput{
				Item: map[string]dbtypes.AttributeValue{
					"PK":        &dbtypes.AttributeValueMemberS{Value: "SESSION#" + tokenHash},
					"SK":        &dbtypes.AttributeValueMemberS{Value: "SESSION"},
					"userId":    &dbtypes.AttributeValueMemberS{Value: "user-ok"},
					"email":     &dbtypes.AttributeValueMemberS{Value: "ok@example.com"},
					"familyId":  &dbtypes.AttributeValueMemberS{Value: "family-ok"},
					"rotated":   &dbtypes.AttributeValueMemberBOOL{Value: false},
					"expiresAt": &dbtypes.AttributeValueMemberN{Value: fmt.Sprintf("%d", time.Now().Add(7*24*time.Hour).Unix())},
				},
			}, nil
		},
		updateItemFunc: func(ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
			return &dynamodb.UpdateItemOutput{}, nil
		},
		putItemFunc: func(ctx context.Context, params *dynamodb.PutItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
			return &dynamodb.PutItemOutput{}, nil
		},
	}

	svc := newTestJWTService(mock)

	tokens, err := svc.RefreshToken(context.Background(), refreshToken)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if tokens.AccessToken == "" {
		t.Error("expected non-empty access token")
	}
}
