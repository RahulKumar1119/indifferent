package auth

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	dbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/golang-jwt/jwt/v5"
)

// googleSignIn runs one Google sign-in for (googleID, email) against store.
func googleSignIn(t *testing.T, store *memoryDB, googleID, email string, verified bool) (userID string, refreshToken string, err error) {
	t.Helper()
	tokenURL, userURL := googleOAuthServersVerified(t, googleID, email, verified)
	svc := newGoogleAuthServiceWithURLs(testConfig(), store, &testHTTPClient{client: http.DefaultClient}, tokenURL, userURL)
	tokens, err := svc.Authenticate(context.Background(), "code", "")
	if err != nil {
		return "", "", err
	}
	parsed, _, perr := new(jwt.Parser).ParseUnverified(tokens.AccessToken, jwt.MapClaims{})
	if perr != nil {
		t.Fatal(perr)
	}
	return parsed.Claims.(jwt.MapClaims)["userId"].(string), tokens.RefreshToken, nil
}

func ownerIDOf(t *testing.T, store *memoryDB, email string) string {
	t.Helper()
	return strAttr(store.items[itemKey("EMAIL#"+email, "PROFILE")], "userId")
}

// A stranger signs up with someone else's address; the real owner then signs in
// with Google. The owner must get in (SEC-08), and the stranger must be out.
func TestGoogleLink_HandsAnUnconfirmedAccountToTheVerifiedGoogleOwner(t *testing.T) {
	ctx := context.Background()
	store := newMemoryDB()
	svc := passwordServiceWithVerifier(store, &recordingVerificationMailer{})
	squatter, err := svc.SignUp(ctx, "victim@example.com", "Squatter", "squatters-password")
	if err != nil {
		t.Fatal(err)
	}
	owner := ownerIDOf(t, store, "victim@example.com")

	userID, googleRefresh, err := googleSignIn(t, store, "google-victim-1", "victim@example.com", true)
	if err != nil {
		t.Fatalf("the real owner must be able to sign in with Google: %v", err)
	}
	if userID != owner {
		t.Errorf("signed in as %q, want the existing account %q", userID, owner)
	}

	p := profileOf(t, store, "victim@example.com")
	if _, ok := p["passwordHash"]; ok {
		t.Error("the squatter's password is still on the account")
	}
	if v, _ := boolOf(p, "emailVerified"); !v {
		t.Error("the account is not marked confirmed after Google verified the address")
	}
	if _, ok := p["sessionsRevokedBeforeMs"]; !ok {
		t.Error("the squatter's sessions were not revoked")
	}

	if _, err := svc.Login(ctx, "victim@example.com", "squatters-password"); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("squatter can still log in with the password: %v", err)
	}
	if _, err := svc.JWT.RefreshToken(ctx, squatter.RefreshToken); !errors.Is(err, ErrSessionExpired) {
		t.Errorf("squatter's refresh token still works: %v", err)
	}
	if _, err := svc.JWT.RefreshToken(ctx, googleRefresh); err != nil {
		t.Errorf("the owner's own new session must refresh: %v", err)
	}
}

func TestGoogleLink_TheNextGoogleSignInKeepsWorking(t *testing.T) {
	store := newMemoryDB()
	svc := passwordServiceWithVerifier(store, &recordingVerificationMailer{})
	if _, err := svc.SignUp(context.Background(), "again@example.com", "S", "squatters-password"); err != nil {
		t.Fatal(err)
	}
	owner := ownerIDOf(t, store, "again@example.com")
	for i := 0; i < 3; i++ {
		userID, refresh, err := googleSignIn(t, store, "google-again-1", "again@example.com", true)
		if err != nil || userID != owner {
			t.Fatalf("sign-in %d: user %q err %v, want %q", i+1, userID, err, owner)
		}
		if _, err := svc.JWT.RefreshToken(context.Background(), refresh); err != nil {
			t.Fatalf("sign-in %d: its session cannot refresh: %v", i+1, err)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

func TestGoogleLink_WithoutGooglesVerificationTheAddressStaysTaken(t *testing.T) {
	store := newMemoryDB()
	svc := passwordServiceWithVerifier(store, &recordingVerificationMailer{})
	if _, err := svc.SignUp(context.Background(), "unproven@example.com", "S", "squatters-password"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := googleSignIn(t, store, "google-x", "unproven@example.com", false); !errors.Is(err, ErrEmailTaken) {
		t.Fatalf("got %v, want ErrEmailTaken", err)
	}
	p := profileOf(t, store, "unproven@example.com")
	if extractStringAttr(p, "passwordHash") == "" {
		t.Error("an unverified Google address must not strip the password")
	}
}

func TestGoogleLink_ALegacyPasswordAccountCountsAsUnconfirmed(t *testing.T) {
	// Accounts created before confirmation existed have no emailVerified mark.
	ctx := context.Background()
	store := newMemoryDB()
	svc := testPasswordService(store)
	old, err := svc.SignUp(ctx, "legacy@example.com", "L", "correct-horse-battery")
	if err != nil {
		t.Fatal(err)
	}
	owner := ownerIDOf(t, store, "legacy@example.com")
	if _, present := boolOf(profileOf(t, store, "legacy@example.com"), "emailVerified"); present {
		t.Fatal("test setup: the legacy account should have no mark")
	}
	userID, _, err := googleSignIn(t, store, "google-legacy", "legacy@example.com", true)
	if err != nil || userID != owner {
		t.Fatalf("user %q err %v, want %q", userID, err, owner)
	}
	if _, err := svc.JWT.RefreshToken(ctx, old.RefreshToken); !errors.Is(err, ErrSessionExpired) {
		t.Errorf("the password-era session survived the hand-over: %v", err)
	}
}

func TestGoogleLink_AConfirmedPasswordAccountIsOpenedNotStripped(t *testing.T) {
	ctx := context.Background()
	store := newMemoryDB()
	mailer := &recordingVerificationMailer{}
	svc := passwordServiceWithVerifier(store, mailer)
	pw, err := svc.SignUp(ctx, "both@example.com", "B", "correct-horse")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Verifier.Verify(ctx, tokenFromLink(t, mailer.links[0])); err != nil {
		t.Fatal(err)
	}
	owner := ownerIDOf(t, store, "both@example.com")

	userID, _, err := googleSignIn(t, store, "google-both", "both@example.com", true)
	if err != nil || userID != owner {
		t.Fatalf("user %q err %v, want %q", userID, err, owner)
	}
	p := profileOf(t, store, "both@example.com")
	if extractStringAttr(p, "passwordHash") == "" {
		t.Error("a confirmed account lost its password")
	}
	if _, ok := p["sessionsRevokedBeforeMs"]; ok {
		t.Error("a confirmed account had its sessions revoked")
	}
	if _, err := svc.JWT.RefreshToken(ctx, pw.RefreshToken); err != nil {
		t.Errorf("the owner's password session was ended: %v", err)
	}
	if _, err := svc.Login(ctx, "both@example.com", "correct-horse"); err != nil {
		t.Errorf("the owner can no longer use the password: %v", err)
	}
}

func TestGoogleLink_NeverMergesTwoGoogleAccounts(t *testing.T) {
	// The address was reassigned to a different Google account: not the same
	// person, so the old account's data must not open to the new one.
	store := newMemoryDB()
	seedUserItem(store, "google-old", "shared@example.com", "Old")
	if _, _, err := googleSignIn(t, store, "google-new", "shared@example.com", true); !errors.Is(err, ErrEmailTaken) {
		t.Fatalf("got %v, want ErrEmailTaken", err)
	}
}

func TestGoogleLink_RefusesWhenTheProfileAddressNoLongerMatches(t *testing.T) {
	store := newMemoryDB()
	svc := testPasswordService(store)
	if _, err := svc.SignUp(context.Background(), "match@example.com", "M", "correct-horse"); err != nil {
		t.Fatal(err)
	}
	profileOf(t, store, "match@example.com")["email"] = &dbtypes.AttributeValueMemberS{Value: "other@example.com"}
	if _, _, err := googleSignIn(t, store, "google-m", "match@example.com", true); !errors.Is(err, ErrEmailTaken) {
		t.Fatalf("got %v, want ErrEmailTaken", err)
	}
}

func TestGoogleSignIn_MarksANewGoogleAccountConfirmedOnlyWhenGoogleVerified(t *testing.T) {
	for _, verified := range []bool{true, false} {
		store := newMemoryDB()
		if _, _, err := googleSignIn(t, store, "google-new-1", "fresh@example.com", verified); err != nil {
			t.Fatal(err)
		}
		v, present := boolOf(profileOf(t, store, "fresh@example.com"), "emailVerified")
		if verified && (!present || !v) {
			t.Errorf("verified Google address: emailVerified = %v/%v, want true", v, present)
		}
		if !verified && present {
			t.Errorf("unverified Google address was marked %v; it must be left unmarked", v)
		}
	}
}

// Before this change a Google re-login rewrote the profile from scratch,
// dropping every attribute it did not know about: notification choices and, since
// SEC-14, the "sign out of all devices" mark.
func TestGoogleSignIn_KeepsEveryOtherProfileAttribute(t *testing.T) {
	store := newMemoryDB()
	if _, _, err := googleSignIn(t, store, "google-keep", "keep@example.com", true); err != nil {
		t.Fatal(err)
	}
	p := profileOf(t, store, "keep@example.com")
	p["notifyCompleted"] = &dbtypes.AttributeValueMemberBOOL{Value: false}
	p["sessionsRevokedBeforeMs"] = &dbtypes.AttributeValueMemberN{Value: "123"}
	created := extractStringAttr(p, "createdAt")

	if _, _, err := googleSignIn(t, store, "google-keep", "keep@example.com", true); err != nil {
		t.Fatal(err)
	}
	p = profileOf(t, store, "keep@example.com")
	if v, _ := boolOf(p, "notifyCompleted"); v {
		t.Error("notification choice was reset by a re-login")
	}
	if n, ok := p["sessionsRevokedBeforeMs"].(*dbtypes.AttributeValueMemberN); !ok || n.Value != "123" {
		t.Error("the sign-out-everywhere mark was erased by a re-login")
	}
	if extractStringAttr(p, "createdAt") != created {
		t.Error("createdAt changed")
	}
}

func TestGoogleLink_AHandedOverAccountBelongsToThatGoogleAccountOnly(t *testing.T) {
	store := newMemoryDB()
	svc := passwordServiceWithVerifier(store, &recordingVerificationMailer{})
	if _, err := svc.SignUp(context.Background(), "mine@example.com", "S", "squatters-password"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := googleSignIn(t, store, "google-first", "mine@example.com", true); err != nil {
		t.Fatal(err)
	}
	if got := extractStringAttr(profileOf(t, store, "mine@example.com"), "googleId"); got != "google-first" {
		t.Fatalf("googleId = %q, want the Google account that took it over", got)
	}
	// Another Google account later holding the same address is not let in.
	if _, _, err := googleSignIn(t, store, "google-second", "mine@example.com", true); !errors.Is(err, ErrEmailTaken) {
		t.Fatalf("a second Google account got in: %v", err)
	}
}

// Google profiles written by this version are marked confirmed, so the only
// thing that keeps a second Google account out of one is that it is not a
// password account.
func TestGoogleLink_NeverMergesTwoGoogleAccounts_EvenWhenTheFirstIsMarkedConfirmed(t *testing.T) {
	store := newMemoryDB()
	if _, _, err := googleSignIn(t, store, "google-old", "reassigned@example.com", true); err != nil {
		t.Fatal(err)
	}
	if _, _, err := googleSignIn(t, store, "google-new", "reassigned@example.com", true); !errors.Is(err, ErrEmailTaken) {
		t.Fatalf("a second Google account got into the first one's account: %v", err)
	}
}
