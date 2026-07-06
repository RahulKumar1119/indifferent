package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/golang-jwt/jwt/v5"
)

// mockDynamoDBClient implements DynamoDBClient for testing.
type mockDynamoDBClient struct {
	putItemFunc    func(ctx context.Context, params *dynamodb.PutItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error)
	getItemFunc    func(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error)
	deleteItemFunc func(ctx context.Context, params *dynamodb.DeleteItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error)
	putItemCalls   int
}

func (m *mockDynamoDBClient) PutItem(ctx context.Context, params *dynamodb.PutItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
	m.putItemCalls++
	if m.putItemFunc != nil {
		return m.putItemFunc(ctx, params, optFns...)
	}
	return &dynamodb.PutItemOutput{}, nil
}

func (m *mockDynamoDBClient) GetItem(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
	if m.getItemFunc != nil {
		return m.getItemFunc(ctx, params, optFns...)
	}
	return &dynamodb.GetItemOutput{}, nil
}

func (m *mockDynamoDBClient) DeleteItem(ctx context.Context, params *dynamodb.DeleteItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error) {
	if m.deleteItemFunc != nil {
		return m.deleteItemFunc(ctx, params, optFns...)
	}
	return &dynamodb.DeleteItemOutput{}, nil
}

// testHTTPClient wraps an http.Client for use in tests.
type testHTTPClient struct {
	client *http.Client
}

func (t *testHTTPClient) Do(req *http.Request) (*http.Response, error) {
	return t.client.Do(req)
}

func testConfig() GoogleAuthConfig {
	return GoogleAuthConfig{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		RedirectURI:  "http://localhost:3000/callback",
		JWTSecret:    "test-jwt-secret-key-for-signing",
		UsersTable:   "Users",
		SessionTable: "Sessions",
	}
}

func TestAuthenticate_Success(t *testing.T) {
	// Set up mock Google token endpoint
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), "code=valid-auth-code") {
			http.Error(w, "invalid code", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(GoogleTokenResponse{
			AccessToken: "google-access-token",
			TokenType:   "Bearer",
			ExpiresIn:   3600,
			IDToken:     "google-id-token",
		})
	}))
	defer tokenServer.Close()

	// Set up mock Google userinfo endpoint
	userInfoServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader != "Bearer google-access-token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(GoogleUserInfo{
			ID:        "google-user-123",
			Email:     "test@example.com",
			Name:      "Test User",
			AvatarURL: "https://lh3.google.com/photo.jpg",
		})
	}))
	defer userInfoServer.Close()

	mockDB := &mockDynamoDBClient{}
	httpClient := &testHTTPClient{client: http.DefaultClient}

	svc := newGoogleAuthServiceWithURLs(
		testConfig(),
		mockDB,
		httpClient,
		tokenServer.URL,
		userInfoServer.URL,
	)

	tokens, err := svc.Authenticate(context.Background(), "valid-auth-code")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify tokens are returned
	if tokens.AccessToken == "" {
		t.Error("expected non-empty access token")
	}
	if tokens.RefreshToken == "" {
		t.Error("expected non-empty refresh token")
	}
	if tokens.ExpiresIn != int64(AccessTokenTTL.Seconds()) {
		t.Errorf("expected ExpiresIn=%d, got %d", int64(AccessTokenTTL.Seconds()), tokens.ExpiresIn)
	}

	// Verify JWT claims
	parsedToken, err := jwt.Parse(tokens.AccessToken, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(testConfig().JWTSecret), nil
	})
	if err != nil {
		t.Fatalf("failed to parse JWT: %v", err)
	}

	claims, ok := parsedToken.Claims.(jwt.MapClaims)
	if !ok {
		t.Fatal("failed to extract JWT claims")
	}
	if claims["userId"] != "google-user-123" {
		t.Errorf("expected userId 'google-user-123', got '%v'", claims["userId"])
	}
	if claims["email"] != "test@example.com" {
		t.Errorf("expected email 'test@example.com', got '%v'", claims["email"])
	}

	// Verify DynamoDB was called: once for user upsert, once for session
	if mockDB.putItemCalls != 2 {
		t.Errorf("expected 2 DynamoDB PutItem calls, got %d", mockDB.putItemCalls)
	}
}

func TestAuthenticate_GoogleTokenExchangeFailure(t *testing.T) {
	// Token endpoint returns an error
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"invalid_grant","error_description":"code expired"}`, http.StatusBadRequest)
	}))
	defer tokenServer.Close()

	userInfoServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("userinfo endpoint should not be called on token exchange failure")
		http.Error(w, "should not be reached", http.StatusInternalServerError)
	}))
	defer userInfoServer.Close()

	mockDB := &mockDynamoDBClient{}
	httpClient := &testHTTPClient{client: http.DefaultClient}

	svc := newGoogleAuthServiceWithURLs(
		testConfig(),
		mockDB,
		httpClient,
		tokenServer.URL,
		userInfoServer.URL,
	)

	_, err := svc.Authenticate(context.Background(), "expired-code")
	if err == nil {
		t.Fatal("expected error for token exchange failure, got nil")
	}

	if !strings.Contains(err.Error(), "exchange auth code") {
		t.Errorf("expected error to contain 'exchange auth code', got: %v", err)
	}

	// DynamoDB should not have been called
	if mockDB.putItemCalls != 0 {
		t.Errorf("expected 0 DynamoDB PutItem calls, got %d", mockDB.putItemCalls)
	}
}

func TestAuthenticate_GoogleUserInfoFetchFailure(t *testing.T) {
	// Token endpoint succeeds
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(GoogleTokenResponse{
			AccessToken: "google-access-token",
			TokenType:   "Bearer",
			ExpiresIn:   3600,
		})
	}))
	defer tokenServer.Close()

	// Userinfo endpoint returns an error
	userInfoServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer userInfoServer.Close()

	mockDB := &mockDynamoDBClient{}
	httpClient := &testHTTPClient{client: http.DefaultClient}

	svc := newGoogleAuthServiceWithURLs(
		testConfig(),
		mockDB,
		httpClient,
		tokenServer.URL,
		userInfoServer.URL,
	)

	_, err := svc.Authenticate(context.Background(), "valid-auth-code")
	if err == nil {
		t.Fatal("expected error for userinfo fetch failure, got nil")
	}

	if !strings.Contains(err.Error(), "fetch user info") {
		t.Errorf("expected error to contain 'fetch user info', got: %v", err)
	}

	// DynamoDB should not have been called
	if mockDB.putItemCalls != 0 {
		t.Errorf("expected 0 DynamoDB PutItem calls, got %d", mockDB.putItemCalls)
	}
}

func TestAuthenticate_DynamoDBUserUpsertFailure(t *testing.T) {
	// Token endpoint succeeds
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(GoogleTokenResponse{
			AccessToken: "google-access-token",
			TokenType:   "Bearer",
			ExpiresIn:   3600,
		})
	}))
	defer tokenServer.Close()

	// Userinfo endpoint succeeds
	userInfoServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(GoogleUserInfo{
			ID:        "google-user-456",
			Email:     "user@example.com",
			Name:      "Another User",
			AvatarURL: "https://lh3.google.com/avatar.jpg",
		})
	}))
	defer userInfoServer.Close()

	mockDB := &mockDynamoDBClient{
		putItemFunc: func(ctx context.Context, params *dynamodb.PutItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
			// Fail on first call (user upsert)
			return nil, fmt.Errorf("dynamodb connection error")
		},
	}
	httpClient := &testHTTPClient{client: http.DefaultClient}

	svc := newGoogleAuthServiceWithURLs(
		testConfig(),
		mockDB,
		httpClient,
		tokenServer.URL,
		userInfoServer.URL,
	)

	_, err := svc.Authenticate(context.Background(), "valid-auth-code")
	if err == nil {
		t.Fatal("expected error for DynamoDB user upsert failure, got nil")
	}

	if !strings.Contains(err.Error(), "upsert user") {
		t.Errorf("expected error to contain 'upsert user', got: %v", err)
	}
}

func TestAuthenticate_SessionCreationFailure(t *testing.T) {
	// Token endpoint succeeds
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(GoogleTokenResponse{
			AccessToken: "google-access-token",
			TokenType:   "Bearer",
			ExpiresIn:   3600,
		})
	}))
	defer tokenServer.Close()

	// Userinfo endpoint succeeds
	userInfoServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(GoogleUserInfo{
			ID:        "google-user-789",
			Email:     "session@example.com",
			Name:      "Session User",
			AvatarURL: "https://lh3.google.com/pic.jpg",
		})
	}))
	defer userInfoServer.Close()

	callCount := 0
	mockDB := &mockDynamoDBClient{
		putItemFunc: func(ctx context.Context, params *dynamodb.PutItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
			callCount++
			// First call (user upsert) succeeds
			if callCount == 1 {
				return &dynamodb.PutItemOutput{}, nil
			}
			// Second call (session creation) fails
			return nil, fmt.Errorf("dynamodb throttling exception")
		},
	}
	httpClient := &testHTTPClient{client: http.DefaultClient}

	svc := newGoogleAuthServiceWithURLs(
		testConfig(),
		mockDB,
		httpClient,
		tokenServer.URL,
		userInfoServer.URL,
	)

	_, err := svc.Authenticate(context.Background(), "valid-auth-code")
	if err == nil {
		t.Fatal("expected error for session creation failure, got nil")
	}

	if !strings.Contains(err.Error(), "create session") {
		t.Errorf("expected error to contain 'create session', got: %v", err)
	}
}

func TestHashToken(t *testing.T) {
	token := "test-refresh-token"
	hash1 := hashToken(token)
	hash2 := hashToken(token)

	// Same input produces same hash
	if hash1 != hash2 {
		t.Errorf("hashToken not deterministic: %s != %s", hash1, hash2)
	}

	// Different input produces different hash
	hash3 := hashToken("different-token")
	if hash1 == hash3 {
		t.Error("expected different hashes for different inputs")
	}

	// Hash is 64 hex characters (SHA-256 = 32 bytes = 64 hex chars)
	if len(hash1) != 64 {
		t.Errorf("expected hash length 64, got %d", len(hash1))
	}
}
