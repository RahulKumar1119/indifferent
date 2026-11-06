package auth

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	dbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// signedUp creates a user and returns the service, the store and the refresh token.
func signedUp(t *testing.T) (*PasswordAuthService, *memoryDB, string) {
	t.Helper()
	store := newMemoryDB()
	svc := testPasswordService(store)
	tokens, err := svc.SignUp(context.Background(), "person@example.com", "P", "correct-horse")
	if err != nil {
		t.Fatal(err)
	}
	return svc, store, tokens.RefreshToken
}

func sessionItem(store *memoryDB, refreshToken string) map[string]dbtypes.AttributeValue {
	return store.items[itemKey("SESSION#"+hashToken(refreshToken), "SESSION")]
}

func setFamilyStart(store *memoryDB, refreshToken string, at time.Time) {
	sessionItem(store, refreshToken)["familyStartMs"] = &dbtypes.AttributeValueMemberN{Value: strconv.FormatInt(at.UnixMilli(), 10)}
}

func familyStartOf(t *testing.T, store *memoryDB, refreshToken string) int64 {
	t.Helper()
	item := sessionItem(store, refreshToken)
	if item == nil {
		t.Fatal("session not stored")
	}
	n, ok := item["familyStartMs"].(*dbtypes.AttributeValueMemberN)
	if !ok {
		t.Fatal("session has no familyStartMs")
	}
	v, _ := strconv.ParseInt(n.Value, 10, 64)
	return v
}

func TestSession_AbsoluteAgeStopsRefresh(t *testing.T) {
	svc, store, rt := signedUp(t)
	setFamilyStart(store, rt, time.Now().Add(-MaxSessionAge-time.Hour))
	if _, err := svc.JWT.RefreshToken(context.Background(), rt); !errors.Is(err, ErrSessionExpired) {
		t.Fatalf("a 30-day-old sign-in must not refresh, got %v", err)
	}
	// Refused without burning the session: a retry is not read as token theft.
	if _, err := svc.JWT.RefreshToken(context.Background(), rt); !errors.Is(err, ErrSessionExpired) {
		t.Fatalf("retry should fail the same way, got %v", err)
	}
}

func TestSession_RotationKeepsTheOriginalSignInTime(t *testing.T) {
	svc, store, rt := signedUp(t)
	start := time.Now().Add(-29 * 24 * time.Hour)
	setFamilyStart(store, rt, start)
	for i := 0; i < 3; i++ {
		tokens, err := svc.JWT.RefreshToken(context.Background(), rt)
		if err != nil {
			t.Fatalf("refresh %d: %v", i+1, err)
		}
		rt = tokens.RefreshToken
		if got := familyStartOf(t, store, rt); got != start.UnixMilli() {
			t.Fatalf("refresh %d reset the sign-in time: %d, want %d", i+1, got, start.UnixMilli())
		}
	}
}

func TestSession_LegacySessionFallsBackToCreatedAt(t *testing.T) {
	svc, store, rt := signedUp(t)
	item := sessionItem(store, rt)
	delete(item, "familyStartMs")
	item["createdAt"] = &dbtypes.AttributeValueMemberS{Value: time.Now().Add(-40 * 24 * time.Hour).UTC().Format(time.RFC3339)}
	if _, err := svc.JWT.RefreshToken(context.Background(), rt); !errors.Is(err, ErrSessionExpired) {
		t.Fatalf("an old legacy session must expire, got %v", err)
	}

	svc, store, rt = signedUp(t)
	item = sessionItem(store, rt)
	delete(item, "familyStartMs")
	tokens, err := svc.JWT.RefreshToken(context.Background(), rt)
	if err != nil {
		t.Fatalf("a recent legacy session must still refresh: %v", err)
	}
	if familyStartOf(t, store, tokens.RefreshToken) == 0 {
		t.Error("the rotated session must record the sign-in time")
	}
}

func TestSession_LogoutAllRevokesEarlierSignInsOnly(t *testing.T) {
	ctx := context.Background()
	svc, store, phone := signedUp(t)
	laptop, err := svc.Login(ctx, "person@example.com", "correct-horse")
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(3 * time.Millisecond)

	userID := ""
	for _, it := range store.items {
		if strAttr(it, "SK") == "SESSION" {
			userID = strAttr(it, "userId")
		}
	}
	if err := svc.JWT.LogoutAll(ctx, userID); err != nil {
		t.Fatal(err)
	}

	for name, rt := range map[string]string{"phone": phone, "laptop": laptop.RefreshToken} {
		if _, err := svc.JWT.RefreshToken(ctx, rt); !errors.Is(err, ErrSessionExpired) {
			t.Errorf("%s: want ErrSessionExpired after sign out everywhere, got %v", name, err)
		}
	}

	time.Sleep(3 * time.Millisecond)
	fresh, err := svc.Login(ctx, "person@example.com", "correct-horse")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.JWT.RefreshToken(ctx, fresh.RefreshToken); err != nil {
		t.Errorf("a sign-in made after the revoke must work: %v", err)
	}
}

func TestSession_LogoutAllForUnknownUserFails(t *testing.T) {
	svc := testPasswordService(newMemoryDB())
	if err := svc.JWT.LogoutAll(context.Background(), "nobody"); err == nil {
		t.Error("revoking sessions of a user that does not exist should fail, not silently succeed")
	}
}

func claimsToken(t *testing.T, claims jwt.MapClaims) string {
	t.Helper()
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(jwtTestSecret))
	if err != nil {
		t.Fatal(err)
	}
	return signed
}

func TestAccessToken_RequiresExpIssuerAndAudience(t *testing.T) {
	svc := newTestJWTService(&mockDynamoDBClient{})
	exp := time.Now().Add(10 * time.Minute).Unix()
	base := func() jwt.MapClaims {
		return jwt.MapClaims{"userId": "u1", "email": "a@b.co", "iss": TokenIssuer, "aud": TokenAudience, "iat": time.Now().Unix(), "exp": exp}
	}
	without := func(key string) jwt.MapClaims { c := base(); delete(c, key); return c }
	with := func(key string, v any) jwt.MapClaims { c := base(); c[key] = v; return c }

	if _, err := svc.ValidateToken(claimsToken(t, base())); err != nil {
		t.Fatalf("a complete token must be accepted: %v", err)
	}
	for name, c := range map[string]jwt.MapClaims{
		"no exp":                     without("exp"),
		"no iss":                     without("iss"),
		"no aud":                     without("aud"),
		"wrong iss":                  with("iss", "someone-else"),
		"wrong aud":                  with("aud", "another-service"),
		"aud is a list without ours": with("aud", []string{"another-service"}),
	} {
		if _, err := svc.ValidateToken(claimsToken(t, c)); err == nil {
			t.Errorf("%s: token was accepted", name)
		}
	}
}

func TestAccessTokens_WeMintCarryIssuerAndAudience(t *testing.T) {
	svc := newTestJWTService(&mockDynamoDBClient{})
	check := func(name, token string) {
		t.Helper()
		parsed, _, err := new(jwt.Parser).ParseUnverified(token, jwt.MapClaims{})
		if err != nil {
			t.Fatal(err)
		}
		c := parsed.Claims.(jwt.MapClaims)
		if c["iss"] != TokenIssuer {
			t.Errorf("%s: iss = %v", name, c["iss"])
		}
		if aud, _ := c.GetAudience(); len(aud) != 1 || aud[0] != TokenAudience {
			t.Errorf("%s: aud = %v", name, aud)
		}
		if _, err := svc.ValidateToken(token); err != nil {
			t.Errorf("%s: our own token fails our own validation: %v", name, err)
		}
	}
	tok, err := svc.GenerateAccessToken("u1", "a@b.co")
	if err != nil {
		t.Fatal(err)
	}
	check("JWTService.GenerateAccessToken", tok)

	g := &googleAuthService{config: GoogleAuthConfig{JWTSecret: jwtTestSecret}}
	gtok, err := g.generateAccessToken("u1", "a@b.co", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	check("googleAuthService.generateAccessToken", gtok)
}

func TestPasswordStrength(t *testing.T) {
	ctx := context.Background()
	svc := testPasswordService(newMemoryDB())
	for _, tc := range []struct{ email, password string }{
		{"a@example.com", "password123"},
		{"a@example.com", "PASSWORD123"},
		{"a@example.com", "12345678"},
		{"a@example.com", "qwertyuiop"},
		{"a@example.com", "aaaaaaaaaa"},
		{"rahul.k@example.com", "rahul.k-2026!"},
		{"sachin@example.com", "SACHIN-forever9"},
	} {
		_, err := svc.SignUp(ctx, tc.email, "N", tc.password)
		if err == nil || !strings.HasPrefix(err.Error(), "password must be") {
			t.Errorf("%q / %q: want a 'password must be' refusal, got %v", tc.email, tc.password, err)
		}
	}
	for i, pw := range []string{"correct-horse", "t7!kQ9#vLm2", "blue-lantern-42", "My dog Rex eats 3 bones"} {
		if _, err := svc.SignUp(ctx, "user"+strconv.Itoa(i)+"@example.com", "N", pw); err != nil {
			t.Errorf("%q should be accepted: %v", pw, err)
		}
	}
}

func TestPasswordStrength_DoesNotApplyToLogin(t *testing.T) {
	// An existing account whose password is on the list must still sign in.
	ctx := context.Background()
	store := newMemoryDB()
	svc := testPasswordService(store)
	if _, err := svc.SignUp(ctx, "old@example.com", "O", "correct-horse"); err != nil {
		t.Fatal(err)
	}
	for k, it := range store.items {
		if strings.HasPrefix(k, "USER#") {
			hash, _ := bcryptHash("password123")
			it["passwordHash"] = &dbtypes.AttributeValueMemberS{Value: hash}
		}
	}
	if _, err := svc.Login(ctx, "old@example.com", "password123"); err != nil {
		t.Errorf("login with a listed password must keep working for existing accounts: %v", err)
	}
}

func bcryptHash(p string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(p), bcrypt.MinCost)
	return string(b), err
}
