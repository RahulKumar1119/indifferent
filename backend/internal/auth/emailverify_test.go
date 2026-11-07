package auth

import (
	"context"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	dbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// recordingVerificationMailer remembers what it was asked to send.
type recordingVerificationMailer struct {
	to    []string
	links []string
	err   error
}

func (r *recordingVerificationMailer) SendVerification(ctx context.Context, to, link string) error {
	if r.err != nil {
		return r.err
	}
	r.to = append(r.to, to)
	r.links = append(r.links, link)
	return nil
}

func tokenFromLink(t *testing.T, link string) string {
	t.Helper()
	u, err := url.Parse(link)
	if err != nil || u.Query().Get("token") == "" {
		t.Fatalf("link %q has no token", link)
	}
	return u.Query().Get("token")
}

// passwordServiceWithVerifier is testPasswordService with confirmation on.
func passwordServiceWithVerifier(store *memoryDB, mailer *recordingVerificationMailer) *PasswordAuthService {
	svc := testPasswordService(store)
	svc.Verifier = &EmailVerifier{DB: store, UsersTable: "Users", TokenTable: "Sessions", BaseURL: "https://indifferent.fun/", Mailer: mailer}
	return svc
}

func profileOf(t *testing.T, store *memoryDB, email string) map[string]dbtypes.AttributeValue {
	t.Helper()
	idx := store.items[itemKey("EMAIL#"+email, "PROFILE")]
	if idx == nil {
		t.Fatalf("no email index for %s", email)
	}
	p := store.items[itemKey("USER#"+strAttr(idx, "userId"), "PROFILE")]
	if p == nil {
		t.Fatalf("no profile for %s", email)
	}
	return p
}

func boolOf(item map[string]dbtypes.AttributeValue, key string) (value, present bool) {
	if v, ok := item[key].(*dbtypes.AttributeValueMemberBOOL); ok {
		return v.Value, true
	}
	return false, false
}

func TestSignUp_SendsAConfirmationLinkAndMarksTheAccountUnconfirmed(t *testing.T) {
	store := newMemoryDB()
	mailer := &recordingVerificationMailer{}
	svc := passwordServiceWithVerifier(store, mailer)
	if _, err := svc.SignUp(context.Background(), "New@Example.com", "N", "correct-horse"); err != nil {
		t.Fatal(err)
	}

	if v, present := boolOf(profileOf(t, store, "new@example.com"), "emailVerified"); !present || v {
		t.Errorf("emailVerified = %v (present %v), want an explicit false", v, present)
	}
	if len(mailer.links) != 1 || mailer.to[0] != "new@example.com" {
		t.Fatalf("mail sent to %v, want one mail to the normalised address", mailer.to)
	}
	if !strings.HasPrefix(mailer.links[0], "https://indifferent.fun/verify-email?token=") {
		t.Errorf("link = %q", mailer.links[0])
	}

	// Only a hash is stored, and the token lives for a day.
	token := tokenFromLink(t, mailer.links[0])
	found := false
	for k, item := range store.items {
		if !strings.HasPrefix(k, verifyKeyPrefix) {
			continue
		}
		found = true
		if strings.Contains(k, token) {
			t.Error("the raw token is stored; only its hash may be")
		}
		exp, _ := strconv.ParseInt(item["expiresAt"].(*dbtypes.AttributeValueMemberN).Value, 10, 64)
		if d := time.Until(time.Unix(exp, 0)); d < 23*time.Hour || d > 25*time.Hour {
			t.Errorf("token lives %v, want about 24h", d)
		}
	}
	if !found {
		t.Error("no token stored")
	}
}

func TestSignUp_WithoutAVerifierSendsNothingAndHoldsNothingBack(t *testing.T) {
	store := newMemoryDB()
	svc := testPasswordService(store)
	if _, err := svc.SignUp(context.Background(), "plain@example.com", "P", "correct-horse"); err != nil {
		t.Fatal(err)
	}
	if _, present := boolOf(profileOf(t, store, "plain@example.com"), "emailVerified"); present {
		t.Error("an account that cannot be confirmed must not be marked unconfirmed")
	}
}

func TestSignUp_StillWorksWhenTheMailCannotBeSent(t *testing.T) {
	store := newMemoryDB()
	svc := passwordServiceWithVerifier(store, &recordingVerificationMailer{err: errors.New("ses down")})
	tokens, err := svc.SignUp(context.Background(), "x@example.com", "X", "correct-horse")
	if err != nil || tokens.AccessToken == "" {
		t.Fatalf("sign-up must succeed without the mail: %v", err)
	}
	for k := range store.items {
		if strings.HasPrefix(k, verifyKeyPrefix) {
			t.Error("a token nobody received was left valid")
		}
	}
}

func TestVerify_ConfirmsOnceOnly(t *testing.T) {
	store := newMemoryDB()
	mailer := &recordingVerificationMailer{}
	svc := passwordServiceWithVerifier(store, mailer)
	if _, err := svc.SignUp(context.Background(), "once@example.com", "O", "correct-horse"); err != nil {
		t.Fatal(err)
	}
	token := tokenFromLink(t, mailer.links[0])

	if err := svc.Verifier.Verify(context.Background(), token); err != nil {
		t.Fatal(err)
	}
	p := profileOf(t, store, "once@example.com")
	if v, _ := boolOf(p, "emailVerified"); !v {
		t.Error("the account is not marked confirmed")
	}
	if extractStringAttr(p, "emailVerifiedAt") == "" {
		t.Error("no confirmation time recorded")
	}
	if extractStringAttr(p, "passwordHash") == "" {
		t.Error("confirming must not touch the password")
	}
	if err := svc.Verifier.Verify(context.Background(), token); !errors.Is(err, ErrInvalidVerification) {
		t.Errorf("second use: %v, want ErrInvalidVerification", err)
	}
}

func TestVerify_RefusesBadExpiredAndStrayTokens(t *testing.T) {
	store := newMemoryDB()
	mailer := &recordingVerificationMailer{}
	svc := passwordServiceWithVerifier(store, mailer)
	if _, err := svc.SignUp(context.Background(), "tok@example.com", "T", "correct-horse"); err != nil {
		t.Fatal(err)
	}
	good := tokenFromLink(t, mailer.links[0])
	ctx := context.Background()

	for name, token := range map[string]string{
		"empty":     "",
		"too short": "abcd",
		"not hex":   strings.Repeat("zz", 32),
		"too long":  good + "00",
		"unknown":   strings.Repeat("ab", 32),
	} {
		if err := svc.Verifier.Verify(ctx, token); !errors.Is(err, ErrInvalidVerification) {
			t.Errorf("%s: %v, want ErrInvalidVerification", name, err)
		}
	}

	// An expired token that TTL has not removed yet is still refused.
	for k, item := range store.items {
		if strings.HasPrefix(k, verifyKeyPrefix) {
			item["expiresAt"] = &dbtypes.AttributeValueMemberN{Value: strconv.FormatInt(time.Now().Add(-time.Minute).Unix(), 10)}
		}
	}
	if err := svc.Verifier.Verify(ctx, good); !errors.Is(err, ErrInvalidVerification) {
		t.Errorf("expired: %v, want ErrInvalidVerification", err)
	}
	if v, _ := boolOf(profileOf(t, store, "tok@example.com"), "emailVerified"); v {
		t.Error("an expired link confirmed the account")
	}
}

func TestVerify_RefusesWhenTheAddressOnTheAccountChanged(t *testing.T) {
	store := newMemoryDB()
	mailer := &recordingVerificationMailer{}
	svc := passwordServiceWithVerifier(store, mailer)
	if _, err := svc.SignUp(context.Background(), "old@example.com", "O", "correct-horse"); err != nil {
		t.Fatal(err)
	}
	profileOf(t, store, "old@example.com")["email"] = &dbtypes.AttributeValueMemberS{Value: "different@example.com"}
	if err := svc.Verifier.Verify(context.Background(), tokenFromLink(t, mailer.links[0])); !errors.Is(err, ErrInvalidVerification) {
		t.Errorf("got %v, want ErrInvalidVerification", err)
	}
}

func TestVerify_SendBuildsLinkWithoutDoubleSlash(t *testing.T) {
	store := newMemoryDB()
	mailer := &recordingVerificationMailer{}
	v := &EmailVerifier{DB: store, UsersTable: "Users", TokenTable: "Sessions", BaseURL: "https://indifferent.fun///", Mailer: mailer}
	if err := v.Send(context.Background(), "u1", "a@example.com"); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(mailer.links[0], "https://indifferent.fun/verify-email?token=") {
		t.Errorf("link = %q", mailer.links[0])
	}
}
