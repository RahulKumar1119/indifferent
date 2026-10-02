package auth

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

func testPasswordService(store DynamoDBClient) *PasswordAuthService {
	jwtSvc := &JWTService{Secret: testConfig().JWTSecret, DB: store, SessionTable: "Sessions", UsersTable: "Users"}
	return NewPasswordAuthService(store, jwtSvc, "Users", "Sessions")
}

func TestPasswordSignupAndLogin(t *testing.T) {
	ctx := context.Background()
	svc := testPasswordService(newMemoryDB())

	tokens, err := svc.SignUp(ctx, "User@Example.com", "Test User", "s3cret-pass")
	if err != nil {
		t.Fatalf("signup failed: %v", err)
	}
	if tokens.AccessToken == "" || tokens.RefreshToken == "" {
		t.Fatal("expected a full token pair from signup")
	}

	// Login is case-insensitive on the address.
	logged, err := svc.Login(ctx, "user@example.com", "s3cret-pass")
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	if logged.AccessToken == "" {
		t.Fatal("expected access token from login")
	}

	// The password session refreshes through the shared rotation path.
	jwtSvc := &JWTService{Secret: testConfig().JWTSecret, DB: svc.DB, SessionTable: "Sessions", UsersTable: "Users"}
	if _, err := jwtSvc.RefreshToken(ctx, logged.RefreshToken); err != nil {
		t.Fatalf("refresh of password session failed: %v", err)
	}
}

func TestPasswordSignupDuplicate(t *testing.T) {
	ctx := context.Background()
	svc := testPasswordService(newMemoryDB())

	if _, err := svc.SignUp(ctx, "dup@example.com", "One", "password-1"); err != nil {
		t.Fatalf("first signup failed: %v", err)
	}
	if _, err := svc.SignUp(ctx, "DUP@example.com", "Two", "password-2"); !errors.Is(err, ErrEmailTaken) {
		t.Fatalf("expected ErrEmailTaken for duplicate (case-insensitive), got: %v", err)
	}
}

func TestPasswordLoginFailuresAreGeneric(t *testing.T) {
	ctx := context.Background()
	svc := testPasswordService(newMemoryDB())

	if _, err := svc.SignUp(ctx, "real@example.com", "Real", "correct-horse"); err != nil {
		t.Fatalf("signup failed: %v", err)
	}

	wrongPass, err := svc.Login(ctx, "real@example.com", "wrong-horse")
	unknown, err2 := svc.Login(ctx, "ghost@example.com", "anything12")
	if err == nil || err2 == nil {
		t.Fatalf("expected both logins to fail (got %v / %v)", wrongPass, unknown)
	}
	if err.Error() != err2.Error() {
		t.Errorf("login failures must be indistinguishable, got %q vs %q", err, err2)
	}
}

func TestPasswordSignupValidation(t *testing.T) {
	ctx := context.Background()
	svc := testPasswordService(newMemoryDB())

	for _, tc := range []struct {
		name     string
		email    string
		password string
	}{
		{"bad email", "not-an-email", "valid-pass-1"},
		{"short password", "ok@example.com", "short"},
		{"empty password", "ok@example.com", ""},
	} {
		if _, err := svc.SignUp(ctx, tc.email, "N", tc.password); err == nil {
			t.Errorf("%s: expected validation error", tc.name)
		}
	}
}

func TestPasswordNeverStoredPlaintext(t *testing.T) {
	ctx := context.Background()
	store := newMemoryDB()
	svc := testPasswordService(store)

	if _, err := svc.SignUp(ctx, "hash@example.com", "H", "plaintext-secret"); err != nil {
		t.Fatalf("signup failed: %v", err)
	}
	for key, item := range store.items {
		if !strings.HasPrefix(key, "USER#") {
			continue
		}
		hashAttr, ok := item["passwordHash"].(*dbtypes.AttributeValueMemberS)
		if !ok || hashAttr.Value == "" {
			t.Fatalf("profile %q missing password hash", key)
		}
		if strings.Contains(hashAttr.Value, "plaintext-secret") {
			t.Errorf("password stored in plaintext in %q", key)
		}
		if !strings.HasPrefix(hashAttr.Value, "$2a$") {
			t.Errorf("expected bcrypt hash in %q, got %q", key, hashAttr.Value[:10])
		}
	}
}

func TestPasswordLoginOAuthOnlyAccount(t *testing.T) {
	// A Gmail-created profile has no passwordHash: password login must fail
	// generically rather than panic or succeed.
	ctx := context.Background()
	store := newMemoryDB()
	svc := testPasswordService(store)

	seed := func(item map[string]dbtypes.AttributeValue) {
		t.Helper()
		if _, err := store.PutItem(ctx, &dynamodb.PutItemInput{TableName: strPtr("Users"), Item: item}); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	seed(map[string]dbtypes.AttributeValue{
		"PK":     &dbtypes.AttributeValueMemberS{Value: "EMAIL#oauth@example.com"},
		"SK":     &dbtypes.AttributeValueMemberS{Value: "PROFILE"},
		"userId": &dbtypes.AttributeValueMemberS{Value: "google-user-1"},
	})
	seed(map[string]dbtypes.AttributeValue{
		"PK":     &dbtypes.AttributeValueMemberS{Value: "USER#google-user-1"},
		"SK":     &dbtypes.AttributeValueMemberS{Value: "PROFILE"},
		"email":  &dbtypes.AttributeValueMemberS{Value: "oauth@example.com"},
		"userId": &dbtypes.AttributeValueMemberS{Value: "google-user-1"},
	})

	if _, err := svc.Login(ctx, "oauth@example.com", "whatever-12"); err == nil {
		t.Fatal("expected login to fail for OAuth-only account")
	}
}

func strPtr(s string) *string { return &s }
