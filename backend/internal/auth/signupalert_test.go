package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

// Issue #297: one alert per new account, numbered, and never in the way of
// the sign-in.

type recordingNotifier struct {
	mu    sync.Mutex
	sent  []Signup
	err   error
	block bool // wait for the deadline, like a hung mail service
}

func (r *recordingNotifier) NotifySignup(ctx context.Context, s Signup) error {
	if r.block {
		<-ctx.Done()
		return ctx.Err()
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sent = append(r.sent, s)
	return r.err
}

func (r *recordingNotifier) all() []Signup {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Signup(nil), r.sent...)
}

func alerter(store DynamoDBClient, n SignupNotifier) *SignupAlerter {
	return &SignupAlerter{DB: store, UsersTable: "Users", Notifier: n,
		Now: func() time.Time { return time.Date(2026, 11, 12, 16, 30, 0, 0, time.UTC) }}
}

func googleSignInAlerting(t *testing.T, store *memoryDB, a *SignupAlerter, googleID, email string) error {
	t.Helper()
	tokenURL, userURL := googleOAuthServersVerified(t, googleID, email, true)
	cfg := testConfig()
	cfg.Signups = a
	svc := newGoogleAuthServiceWithURLs(cfg, store, &testHTTPClient{client: http.DefaultClient}, tokenURL, userURL)
	_, err := svc.Authenticate(context.Background(), "code", "")
	return err
}

func TestSignupAlert_OnePerNewGoogleAccountAndNoneOnReturn(t *testing.T) {
	store, n := newMemoryDB(), &recordingNotifier{}
	a := alerter(store, n)
	if err := googleSignInAlerting(t, store, a, "g-1", "first@gmail.com"); err != nil {
		t.Fatal(err)
	}
	if err := googleSignInAlerting(t, store, a, "g-1", "first@gmail.com"); err != nil { // returning
		t.Fatal(err)
	}
	if err := googleSignInAlerting(t, store, a, "g-2", "second@gmail.com"); err != nil {
		t.Fatal(err)
	}
	got := n.all()
	if len(got) != 2 {
		t.Fatalf("alerts = %+v, want one per new account", got)
	}
	if got[0].Email != "first@gmail.com" || got[0].Method != "google" || got[0].Number != 1 {
		t.Errorf("first alert = %+v", got[0])
	}
	if got[1].Email != "second@gmail.com" || got[1].Number != 2 {
		t.Errorf("second alert = %+v", got[1])
	}
	if !got[0].At.Equal(time.Date(2026, 11, 12, 16, 30, 0, 0, time.UTC)) {
		t.Errorf("At = %v", got[0].At)
	}
}

func TestSignupAlert_OnePerNewPasswordAccountAndNoneForARefusedOne(t *testing.T) {
	store, n := newMemoryDB(), &recordingNotifier{}
	svc := testPasswordService(store)
	svc.Signups = alerter(store, n)
	ctx := context.Background()
	if _, err := svc.SignUp(ctx, "new.person@gmail.com", "New Person", "s3cret-pass-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SignUp(ctx, "new.person@gmail.com", "Someone Else", "s3cret-pass-2"); !errors.Is(err, ErrEmailTaken) {
		t.Fatalf("second sign-up: %v, want ErrEmailTaken", err)
	}
	if _, err := svc.SignUp(ctx, "x@gmail.com", "Weak", "short"); err == nil {
		t.Fatal("a weak password was accepted")
	}
	if _, err := svc.Login(ctx, "new.person@gmail.com", "s3cret-pass-1"); err != nil {
		t.Fatal(err)
	}
	got := n.all()
	if len(got) != 1 || got[0].Email != "new.person@gmail.com" || got[0].Name != "New Person" || got[0].Method != "email" || got[0].Number != 1 {
		t.Fatalf("alerts = %+v, want exactly the one new account", got)
	}
}

func TestSignupAlert_NoneWhenAGoogleSignInTakesOverAPasswordAccount(t *testing.T) {
	store, n := newMemoryDB(), &recordingNotifier{}
	pw := testPasswordService(store)
	pw.Signups = alerter(store, n)
	if _, err := pw.SignUp(context.Background(), "owner@gmail.com", "Owner", "s3cret-pass-1"); err != nil {
		t.Fatal(err)
	}
	// The verified Google owner of the address signs in to that account: it is
	// not a new account.
	if err := googleSignInAlerting(t, store, alerter(store, n), "g-owner", "owner@gmail.com"); err != nil {
		t.Fatal(err)
	}
	if got := n.all(); len(got) != 1 {
		t.Errorf("alerts = %+v, want only the password sign-up", got)
	}
}

// A failed profile read says nothing about whether the account exists.
func TestSignupAlert_NoneWhenTheProfileReadFails(t *testing.T) {
	store, n := &failingGetDB{memoryDB: newMemoryDB()}, &recordingNotifier{}
	tokenURL, userURL := googleOAuthServersVerified(t, "g-3", "third@gmail.com", true)
	cfg := testConfig()
	cfg.Signups = alerter(store, n)
	svc := newGoogleAuthServiceWithURLs(cfg, store, &testHTTPClient{client: http.DefaultClient}, tokenURL, userURL)
	svc.Authenticate(context.Background(), "code", "")
	if got := n.all(); len(got) != 0 {
		t.Errorf("alerts = %+v, want none when the profile could not be read", got)
	}
}

type failingGetDB struct{ *memoryDB }

func (f *failingGetDB) GetItem(ctx context.Context, in *dynamodb.GetItemInput, o ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
	if strAttr(in.Key, "PK") == "USER#g-3" {
		return nil, errors.New("throttled")
	}
	return f.memoryDB.GetItem(ctx, in, o...)
}

func TestSignupAlert_TestAccountsAreNeitherCountedNorReported(t *testing.T) {
	store, n := newMemoryDB(), &recordingNotifier{}
	svc := testPasswordService(store)
	svc.Signups = alerter(store, n)
	if _, err := svc.SignUp(context.Background(), "e2e-1@example.com", "E2E", "s3cret-pass-1"); err != nil {
		t.Fatal(err)
	}
	if len(n.all()) != 0 {
		t.Error("a test account was reported")
	}
	if _, ok := store.items[itemKey(signupCounterPK, signupCounterSK)]; ok {
		t.Error("a test account was counted")
	}
}

func TestSignupAlert_ASlowOrFailingSendNeverStopsTheSignIn(t *testing.T) {
	for name, n := range map[string]*recordingNotifier{
		"fails": {err: errors.New("ses down")},
		"hangs": {block: true},
	} {
		t.Run(name, func(t *testing.T) {
			store := newMemoryDB()
			svc := testPasswordService(store)
			svc.Signups = alerter(store, n)
			start := time.Now()
			tokens, err := svc.SignUp(context.Background(), name+"@gmail.com", "Person", "s3cret-pass-1")
			if err != nil || tokens.AccessToken == "" {
				t.Fatalf("sign-up failed with the alert %s: %v", name, err)
			}
			if took := time.Since(start); took > signupAlertTimeout+2*time.Second {
				t.Errorf("sign-up took %v, the alert must give up after %v", took, signupAlertTimeout)
			}
		})
	}
}

func TestSignupAlert_ACounterFailureStillSendsWithoutANumber(t *testing.T) {
	n := &recordingNotifier{}
	store := &failingUpdateDB{memoryDB: newMemoryDB()}
	svc := testPasswordService(store)
	svc.Signups = alerter(store, n)
	if _, err := svc.SignUp(context.Background(), "p@gmail.com", "P", "s3cret-pass-1"); err != nil {
		t.Fatal(err)
	}
	if got := n.all(); len(got) != 1 || got[0].Number != 0 {
		t.Errorf("alerts = %+v, want one without a number", got)
	}
}

type failingUpdateDB struct{ *memoryDB }

func (f *failingUpdateDB) UpdateItem(ctx context.Context, in *dynamodb.UpdateItemInput, o ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
	if strAttr(in.Key, "PK") == signupCounterPK {
		return nil, errors.New("throttled")
	}
	return f.memoryDB.UpdateItem(ctx, in, o...)
}

// scripts/ops/user_counts.py seeds and shows this counter; its key must match.
func TestSignupAlert_TheOpsScriptUsesTheSameCounterKey(t *testing.T) {
	src, err := os.ReadFile("../../../scripts/ops/user_counts.py")
	if err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf("COUNTER_PK, COUNTER_SK = %q, %q", signupCounterPK, signupCounterSK)
	if !strings.Contains(string(src), want) || !strings.Contains(string(src), `"n": {"N"`) {
		t.Errorf("user_counts.py does not use %s with attribute %q", want, signupCounterAttr)
	}
}

func TestSignupAlert_NilAlerterDoesNothing(t *testing.T) {
	var a *SignupAlerter
	a.newAccount(context.Background(), "n", "e@gmail.com", "email") // must not panic
	(&SignupAlerter{}).newAccount(context.Background(), "n", "e@gmail.com", "email")
	if !strings.HasPrefix(signupCounterPK, "COUNTER#") {
		t.Error("the counter key must not look like a user or an email index row")
	}
}
