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
	dbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/golang-jwt/jwt/v5"
)

// mockDynamoDBClient implements DynamoDBClient for testing.
type mockDynamoDBClient struct {
	putItemFunc    func(ctx context.Context, params *dynamodb.PutItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error)
	getItemFunc    func(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error)
	deleteItemFunc func(ctx context.Context, params *dynamodb.DeleteItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error)
	updateItemFunc func(ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error)
	scanFunc       func(ctx context.Context, params *dynamodb.ScanInput, optFns ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error)
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

func (m *mockDynamoDBClient) UpdateItem(ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
	if m.updateItemFunc != nil {
		return m.updateItemFunc(ctx, params, optFns...)
	}
	return &dynamodb.UpdateItemOutput{}, nil
}

func (m *mockDynamoDBClient) Scan(ctx context.Context, params *dynamodb.ScanInput, optFns ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error) {
	if m.scanFunc != nil {
		return m.scanFunc(ctx, params, optFns...)
	}
	return &dynamodb.ScanOutput{Items: []map[string]dbtypes.AttributeValue{}}, nil
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

// memoryDB is a minimal in-memory DynamoDBClient shared by the login and
// refresh services in the round-trip test below.
type memoryDB struct {
	items map[string]map[string]dbtypes.AttributeValue
}

func newMemoryDB() *memoryDB {
	return &memoryDB{items: map[string]map[string]dbtypes.AttributeValue{}}
}

func itemKey(pk, sk string) string { return pk + "|" + sk }

func strAttr(item map[string]dbtypes.AttributeValue, key string) string {
	if v, ok := item[key].(*dbtypes.AttributeValueMemberS); ok {
		return v.Value
	}
	return ""
}

func (m *memoryDB) PutItem(ctx context.Context, params *dynamodb.PutItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
	m.items[itemKey(strAttr(params.Item, "PK"), strAttr(params.Item, "SK"))] = params.Item
	return &dynamodb.PutItemOutput{}, nil
}

func (m *memoryDB) GetItem(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
	item, ok := m.items[itemKey(strAttr(params.Key, "PK"), strAttr(params.Key, "SK"))]
	if !ok {
		return &dynamodb.GetItemOutput{}, nil
	}
	return &dynamodb.GetItemOutput{Item: item}, nil
}

func (m *memoryDB) DeleteItem(ctx context.Context, params *dynamodb.DeleteItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error) {
	delete(m.items, itemKey(strAttr(params.Key, "PK"), strAttr(params.Key, "SK")))
	return &dynamodb.DeleteItemOutput{}, nil
}

func (m *memoryDB) UpdateItem(ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
	item, ok := m.items[itemKey(strAttr(params.Key, "PK"), strAttr(params.Key, "SK"))]
	if !ok {
		return &dynamodb.UpdateItemOutput{}, nil
	}
	for k, v := range params.ExpressionAttributeValues {
		switch k {
		case ":r":
			if b, ok := v.(*dbtypes.AttributeValueMemberBOOL); ok {
				item["rotated"] = &dbtypes.AttributeValueMemberBOOL{Value: b.Value}
			}
		case ":e":
			if n, ok := v.(*dbtypes.AttributeValueMemberN); ok {
				item["expiresAt"] = &dbtypes.AttributeValueMemberN{Value: n.Value}
			}
		}
	}
	return &dynamodb.UpdateItemOutput{}, nil
}

func (m *memoryDB) Scan(ctx context.Context, params *dynamodb.ScanInput, optFns ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error) {
	fid := ""
	if v, ok := params.ExpressionAttributeValues[":fid"].(*dbtypes.AttributeValueMemberS); ok {
		fid = v.Value
	}
	var out []map[string]dbtypes.AttributeValue
	for _, item := range m.items {
		if fid == "" || strAttr(item, "familyId") == fid {
			out = append(out, item)
		}
	}
	return &dynamodb.ScanOutput{Items: out}, nil
}

// TestLoginSessionRefreshRoundTrip is the regression test for the production
// incident where every /auth/refresh failed: the login writer stored sessions
// under SK=USER#<id> while JWTService.RefreshToken reads PK=SESSION#hash +
// SK=SESSION. Authenticating and then refreshing against the same store must
// succeed, and the stored session must carry SK=SESSION.
func TestLoginSessionRefreshRoundTrip(t *testing.T) {
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(GoogleTokenResponse{
			AccessToken: "google-access-token",
			TokenType:   "Bearer",
			ExpiresIn:   3600,
			IDToken:     "google-id-token",
		})
	}))
	defer tokenServer.Close()

	userInfoServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(GoogleUserInfo{
			ID:        "google-user-123",
			Email:     "test@example.com",
			Name:      "Test User",
			AvatarURL: "https://lh3.google.com/photo.jpg",
		})
	}))
	defer userInfoServer.Close()

	store := newMemoryDB()
	httpClient := &testHTTPClient{client: http.DefaultClient}
	loginSvc := newGoogleAuthServiceWithURLs(testConfig(), store, httpClient, tokenServer.URL, userInfoServer.URL)

	tokens, err := loginSvc.Authenticate(context.Background(), "valid-auth-code")
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	// Exactly one session item, keyed the way the refresh reader looks it up
	// (the other stored item is the USER#PROFILE user record).
	var sessions []map[string]dbtypes.AttributeValue
	for key, item := range store.items {
		if strings.HasPrefix(key, "SESSION#") {
			sessions = append(sessions, item)
		}
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 stored session, got %d", len(sessions))
	}
	for _, item := range sessions {
		key := itemKey(strAttr(item, "PK"), strAttr(item, "SK"))
		if strAttr(item, "SK") != "SESSION" {
			t.Fatalf("session %q stored with SK=%q, want SK=SESSION (refresh would 401)", key, strAttr(item, "SK"))
		}
		if strAttr(item, "email") == "" {
			t.Errorf("session %q missing email (refreshed access tokens would lose it)", key)
		}
	}

	jwtSvc := &JWTService{Secret: testConfig().JWTSecret, DB: store, SessionTable: "Sessions", UsersTable: "Users"}
	refreshed, err := jwtSvc.RefreshToken(context.Background(), tokens.RefreshToken)
	if err != nil {
		t.Fatalf("refresh of a just-created login session failed: %v", err)
	}
	if refreshed.AccessToken == "" || refreshed.RefreshToken == "" {
		t.Error("expected fresh token pair from refresh")
	}
	if refreshed.RefreshToken == tokens.RefreshToken {
		t.Error("expected rotation to issue a new refresh token")
	}

	// The rotated token must be rejected on reuse (theft detection intact).
	if _, err := jwtSvc.RefreshToken(context.Background(), tokens.RefreshToken); err == nil {
		t.Error("expected rotated refresh token to be rejected")
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
