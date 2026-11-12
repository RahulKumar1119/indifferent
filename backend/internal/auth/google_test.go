package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
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
	queryFunc      func(ctx context.Context, params *dynamodb.QueryInput, optFns ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error)
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

func (m *mockDynamoDBClient) Query(ctx context.Context, params *dynamodb.QueryInput, optFns ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error) {
	if m.queryFunc != nil {
		return m.queryFunc(ctx, params, optFns...)
	}
	return &dynamodb.QueryOutput{Items: []map[string]dbtypes.AttributeValue{}}, nil
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

	tokens, err := svc.Authenticate(context.Background(), "valid-auth-code", "")
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

	// Verify DynamoDB was called: user upsert + email reservation + session
	if mockDB.putItemCalls != 3 {
		t.Errorf("expected 3 DynamoDB PutItem calls, got %d", mockDB.putItemCalls)
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
	key := itemKey(strAttr(params.Item, "PK"), strAttr(params.Item, "SK"))
	if params.ConditionExpression != nil && *params.ConditionExpression == "attribute_not_exists(PK)" {
		if _, exists := m.items[key]; exists {
			return nil, &dbtypes.ConditionalCheckFailedException{Message: aws.String("condition failed")}
		}
	}
	m.items[key] = params.Item
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
	k := itemKey(strAttr(params.Key, "PK"), strAttr(params.Key, "SK"))
	old := m.items[k]
	if params.ConditionExpression != nil && !conditionHolds(old, *params.ConditionExpression, params.ExpressionAttributeValues) {
		return &dynamodb.DeleteItemOutput{}, &dbtypes.ConditionalCheckFailedException{Message: aws.String("condition failed")}
	}
	delete(m.items, k)
	out := &dynamodb.DeleteItemOutput{}
	if params.ReturnValues == dbtypes.ReturnValueAllOld {
		out.Attributes = old
	}
	return out, nil
}

func (m *memoryDB) UpdateItem(ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
	if params.UpdateExpression != nil && strings.Contains(*params.UpdateExpression, "emailVerified") {
		// Email confirmation and Google account hand-over: SET a = :x, ... [REMOVE c].
		k := itemKey(strAttr(params.Key, "PK"), strAttr(params.Key, "SK"))
		item := m.items[k]
		if params.ConditionExpression != nil && !conditionHolds(item, *params.ConditionExpression, params.ExpressionAttributeValues) {
			return &dynamodb.UpdateItemOutput{}, &dbtypes.ConditionalCheckFailedException{Message: aws.String("condition failed")}
		}
		applySetRemove(item, *params.UpdateExpression, params.ExpressionAttributeValues)
		return &dynamodb.UpdateItemOutput{}, nil
	}
	if params.UpdateExpression != nil && strings.Contains(*params.UpdateExpression, "sessionsRevokedBeforeMs") {
		// LogoutAll: SET sessionsRevokedBeforeMs = :now on the user's profile.
		k := itemKey(strAttr(params.Key, "PK"), strAttr(params.Key, "SK"))
		item, ok := m.items[k]
		if !ok {
			return &dynamodb.UpdateItemOutput{}, &dbtypes.ConditionalCheckFailedException{Message: aws.String("condition failed")}
		}
		item["sessionsRevokedBeforeMs"] = params.ExpressionAttributeValues[":now"]
		return &dynamodb.UpdateItemOutput{}, nil
	}
	if _, ok := params.ExpressionAttributeValues[":one"]; ok {
		// "ADD attempts :one": atomic counter upsert (login attempt limit).
		k := itemKey(strAttr(params.Key, "PK"), strAttr(params.Key, "SK"))
		item, ok := m.items[k]
		if !ok {
			item = map[string]dbtypes.AttributeValue{"PK": params.Key["PK"], "SK": params.Key["SK"]}
			m.items[k] = item
		}
		// The sign-up counter (#297) names its attribute through #n.
		attr := "attempts"
		if name, ok := params.ExpressionAttributeNames["#n"]; ok {
			attr = name
		}
		n := 0
		if cur, ok := item[attr].(*dbtypes.AttributeValueMemberN); ok {
			n, _ = strconv.Atoi(cur.Value)
		}
		item[attr] = &dbtypes.AttributeValueMemberN{Value: strconv.Itoa(n + 1)}
		return &dynamodb.UpdateItemOutput{Attributes: map[string]dbtypes.AttributeValue{attr: item[attr]}}, nil
	}
	item, ok := m.items[itemKey(strAttr(params.Key, "PK"), strAttr(params.Key, "SK"))]
	if !ok {
		// attribute_exists(PK) fails: no such session.
		return &dynamodb.UpdateItemOutput{}, &dbtypes.ConditionalCheckFailedException{Message: aws.String("condition failed")}
	}
	if v, ok := item["rotated"].(*dbtypes.AttributeValueMemberBOOL); ok && v.Value {
		// Already claimed: the rotation guard fails.
		return &dynamodb.UpdateItemOutput{}, &dbtypes.ConditionalCheckFailedException{Message: aws.String("condition failed")}
	}
	// ReturnValues ALL_OLD: snapshot before applying the claim.
	old := make(map[string]dbtypes.AttributeValue, len(item))
	for k, v := range item {
		old[k] = v
	}
	item["rotated"] = &dbtypes.AttributeValueMemberBOOL{Value: true}
	for k, v := range params.ExpressionAttributeValues {
		switch k {
		case ":r":
			if b, ok := v.(*dbtypes.AttributeValueMemberBOOL); ok {
				item["rotated"] = &dbtypes.AttributeValueMemberBOOL{Value: b.Value}
			}
		case ":e", ":exp":
			if n, ok := v.(*dbtypes.AttributeValueMemberN); ok {
				item["expiresAt"] = &dbtypes.AttributeValueMemberN{Value: n.Value}
			}
		}
	}
	return &dynamodb.UpdateItemOutput{Attributes: old}, nil
}

func (m *memoryDB) Query(ctx context.Context, params *dynamodb.QueryInput, optFns ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error) {
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
	return &dynamodb.QueryOutput{Items: out}, nil
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

	tokens, err := loginSvc.Authenticate(context.Background(), "valid-auth-code", "")
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

	_, err := svc.Authenticate(context.Background(), "expired-code", "")
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

	_, err := svc.Authenticate(context.Background(), "valid-auth-code", "")
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

	_, err := svc.Authenticate(context.Background(), "valid-auth-code", "")
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
			// First calls (email reservation, user upsert) succeed
			if callCount <= 2 {
				return &dynamodb.PutItemOutput{}, nil
			}
			// Third call (session creation) fails
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

	_, err := svc.Authenticate(context.Background(), "valid-auth-code", "")
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

func googleOAuthServers(t *testing.T, googleID, email string) (string, string) {
	t.Helper()
	return googleOAuthServersVerified(t, googleID, email, false)
}

// googleOAuthServersVerified is googleOAuthServers where Google also states
// whether it verified the address (verified_email).
func googleOAuthServersVerified(t *testing.T, googleID, email string, verified bool) (string, string) {
	t.Helper()
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(GoogleTokenResponse{
			AccessToken: "google-access-token",
			TokenType:   "Bearer",
			ExpiresIn:   3600,
			IDToken:     "google-id-token",
		})
	}))
	t.Cleanup(tokenServer.Close)
	userInfoServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(GoogleUserInfo{
			ID:        googleID,
			Email:     email,
			Name:      "G User",
			AvatarURL: "https://lh3.google.com/photo.jpg",

			VerifiedEmail: verified,
		})
	}))
	t.Cleanup(userInfoServer.Close)
	return tokenServer.URL, userInfoServer.URL
}

func seedUserItem(store *memoryDB, userID, email, name string) {
	store.items[itemKey("USER#"+userID, "PROFILE")] = map[string]dbtypes.AttributeValue{
		"PK":        &dbtypes.AttributeValueMemberS{Value: "USER#" + userID},
		"SK":        &dbtypes.AttributeValueMemberS{Value: "PROFILE"},
		"email":     &dbtypes.AttributeValueMemberS{Value: email},
		"name":      &dbtypes.AttributeValueMemberS{Value: name},
		"createdAt": &dbtypes.AttributeValueMemberS{Value: "2024-01-01T00:00:00Z"},
	}
	store.items[itemKey("EMAIL#"+email, "PROFILE")] = map[string]dbtypes.AttributeValue{
		"PK":     &dbtypes.AttributeValueMemberS{Value: "EMAIL#" + email},
		"SK":     &dbtypes.AttributeValueMemberS{Value: "PROFILE"},
		"userId": &dbtypes.AttributeValueMemberS{Value: userID},
	}
}

func TestAuthenticate_EmailTakenByPasswordUser(t *testing.T) {
	// A password account owns taken@example.com; a Google login for the same
	// address must fail taken instead of creating a detached second identity.
	store := newMemoryDB()
	seedUserItem(store, "password-user-1", "taken@example.com", "PW User")
	tokenURL, userURL := googleOAuthServers(t, "google-user-999", "taken@example.com")
	svc := newGoogleAuthServiceWithURLs(testConfig(), store, &testHTTPClient{client: http.DefaultClient}, tokenURL, userURL)

	if _, err := svc.Authenticate(context.Background(), "code", ""); !errors.Is(err, ErrEmailTaken) {
		t.Fatalf("expected ErrEmailTaken, got: %v", err)
	}
}

func TestAuthenticate_SameGoogleUserRelogin(t *testing.T) {
	// Re-login keeps working: the reservation already maps to the same user.
	store := newMemoryDB()
	seedUserItem(store, "google-user-123", "test@example.com", "Test User")
	tokenURL, userURL := googleOAuthServers(t, "google-user-123", "test@example.com")
	svc := newGoogleAuthServiceWithURLs(testConfig(), store, &testHTTPClient{client: http.DefaultClient}, tokenURL, userURL)

	tokens, err := svc.Authenticate(context.Background(), "code", "")
	if err != nil {
		t.Fatalf("re-login failed: %v", err)
	}
	if tokens.AccessToken == "" || tokens.RefreshToken == "" {
		t.Fatal("expected a full token pair on re-login")
	}
}

// ---- PKCE (SEC-07) ---------------------------------------------------------

// exchangeBody runs one sign-in against a fake Google and returns the form the
// token endpoint received.
func exchangeBody(t *testing.T, codeVerifier string) url.Values {
	t.Helper()
	var received url.Values
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		received, _ = url.ParseQuery(string(raw))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(GoogleTokenResponse{AccessToken: "google-access-token", TokenType: "Bearer", ExpiresIn: 3600})
	}))
	defer tokenServer.Close()
	userInfoServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(GoogleUserInfo{ID: "google-user-123", Email: "test@example.com", Name: "Test User"})
	}))
	defer userInfoServer.Close()
	svc := newGoogleAuthServiceWithURLs(testConfig(), &mockDynamoDBClient{}, &testHTTPClient{client: http.DefaultClient}, tokenServer.URL, userInfoServer.URL)
	if _, err := svc.Authenticate(context.Background(), "the-code", codeVerifier); err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	return received
}

func TestAuthenticate_SendsThePKCEVerifierToGoogleWhenThereIsOne(t *testing.T) {
	verifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk" // the RFC 7636 example
	form := exchangeBody(t, verifier)
	if got := form.Get("code_verifier"); got != verifier {
		t.Errorf("code_verifier = %q, want %q (Google can only bind the code to the browser if it gets this)", got, verifier)
	}
	if form.Get("code") != "the-code" || form.Get("grant_type") != "authorization_code" {
		t.Errorf("the rest of the exchange changed: %v", form)
	}
}

func TestAuthenticate_SendsNoVerifierWhenThePageDidNotMakeOne(t *testing.T) {
	form := exchangeBody(t, "")
	if _, present := form["code_verifier"]; present {
		t.Errorf("code_verifier was sent as %q although there was none; a page loaded before PKCE must still sign in", form.Get("code_verifier"))
	}
}

func TestValidCodeVerifier(t *testing.T) {
	forty3 := strings.Repeat("a", 43)
	cases := []struct {
		name string
		v    string
		want bool
	}{
		{"the RFC example", "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk", true},
		{"shortest allowed", forty3, true},
		{"longest allowed", strings.Repeat("a", 128), true},
		{"every allowed punctuation", strings.Repeat("-._~", 11), true},
		{"one too short", forty3[:42], false},
		{"one too long", strings.Repeat("a", 129), false},
		{"empty", "", false},
		{"a space", forty3[:20] + " " + forty3[:22], false},
		{"a plus sign", forty3[:20] + "+" + forty3[:22], false},
		{"a slash", forty3[:20] + "/" + forty3[:22], false},
		{"an equals sign (base64 padding)", forty3[:42] + "=", false},
		{"a newline", forty3[:42] + "\n", false},
		{"non-ASCII", forty3[:42] + "é", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ValidCodeVerifier(c.v); got != c.want {
				t.Errorf("ValidCodeVerifier(%q) = %v, want %v", c.v, got, c.want)
			}
		})
	}
}

// conditionHolds evaluates the few condition forms the auth code uses, joined
// by AND: attribute_exists(X), "attr = :v" and "attr > :v". A nil item fails
// every attribute_exists, like a missing DynamoDB item.
func conditionHolds(item map[string]dbtypes.AttributeValue, cond string, vals map[string]dbtypes.AttributeValue) bool {
	for _, clause := range strings.Split(cond, " AND ") {
		clause = strings.TrimSpace(clause)
		switch {
		case strings.HasPrefix(clause, "attribute_exists("):
			name := strings.TrimSuffix(strings.TrimPrefix(clause, "attribute_exists("), ")")
			if item == nil || item[name] == nil {
				return false
			}
		case strings.Contains(clause, " > "):
			parts := strings.SplitN(clause, " > ", 2)
			cur, ok := item[strings.TrimSpace(parts[0])].(*dbtypes.AttributeValueMemberN)
			want, ok2 := vals[strings.TrimSpace(parts[1])].(*dbtypes.AttributeValueMemberN)
			if item == nil || !ok || !ok2 {
				return false
			}
			a, _ := strconv.ParseInt(cur.Value, 10, 64)
			b, _ := strconv.ParseInt(want.Value, 10, 64)
			if a <= b {
				return false
			}
		case strings.Contains(clause, " = "):
			parts := strings.SplitN(clause, " = ", 2)
			if item == nil || strAttr(item, strings.TrimSpace(parts[0])) != strAttr(map[string]dbtypes.AttributeValue{"v": vals[strings.TrimSpace(parts[1])]}, "v") {
				return false
			}
		default:
			panic("test fake does not understand condition clause: " + clause)
		}
	}
	return true
}

// applySetRemove applies "SET a = :x, b = :y REMOVE c, d" to item.
func applySetRemove(item map[string]dbtypes.AttributeValue, expr string, vals map[string]dbtypes.AttributeValue) {
	set, remove := expr, ""
	if i := strings.Index(expr, " REMOVE "); i >= 0 {
		set, remove = expr[:i], expr[i+len(" REMOVE "):]
	}
	set = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(set), "SET"))
	for _, assign := range strings.Split(set, ",") {
		parts := strings.SplitN(assign, "=", 2)
		if len(parts) != 2 {
			continue
		}
		item[strings.TrimSpace(parts[0])] = vals[strings.TrimSpace(parts[1])]
	}
	for _, name := range strings.Split(remove, ",") {
		if name = strings.TrimSpace(name); name != "" {
			delete(item, name)
		}
	}
}
