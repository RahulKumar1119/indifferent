package auth

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func fixedLoginClock(t *testing.T, at time.Time) {
	t.Helper()
	old := loginNow
	loginNow = func() time.Time { return at }
	t.Cleanup(func() { loginNow = old })
}

func TestLoginLockoutAfterTooManyAttempts(t *testing.T) {
	fixedLoginClock(t, time.Unix(1_800_000_000, 0))
	ctx := context.Background()
	svc := testPasswordService(newMemoryDB())
	if _, err := svc.SignUp(ctx, "victim@example.com", "V", "correct-horse"); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < LoginMaxAttempts; i++ {
		_, err := svc.Login(ctx, "victim@example.com", "wrong-horse")
		if !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("attempt %d: want ErrInvalidCredentials, got %v", i+1, err)
		}
	}

	// The next attempt is refused, and even the right password is refused:
	// the check happens before the password is looked at.
	_, err := svc.Login(ctx, "victim@example.com", "correct-horse")
	var tooMany *TooManyAttemptsError
	if !errors.As(err, &tooMany) {
		t.Fatalf("want TooManyAttemptsError, got %v", err)
	}
	if tooMany.RetryAfter <= 0 || tooMany.RetryAfter > LoginWindow {
		t.Errorf("RetryAfter = %v, want within (0, %v]", tooMany.RetryAfter, LoginWindow)
	}
}

func TestLoginLimitCountsUnknownAddresses(t *testing.T) {
	fixedLoginClock(t, time.Unix(1_800_000_000, 0))
	ctx := context.Background()
	svc := testPasswordService(newMemoryDB())
	for i := 0; i < LoginMaxAttempts; i++ {
		_, _ = svc.Login(ctx, "ghost@example.com", "whatever12")
	}
	var tooMany *TooManyAttemptsError
	if _, err := svc.Login(ctx, "ghost@example.com", "whatever12"); !errors.As(err, &tooMany) {
		t.Fatalf("an address with no account must be limited the same way, got %v", err)
	}
}

func TestLoginLimitIsPerAddressAndPerWindow(t *testing.T) {
	start := time.Unix(1_800_000_000, 0)
	fixedLoginClock(t, start)
	ctx := context.Background()
	svc := testPasswordService(newMemoryDB())
	if _, err := svc.SignUp(ctx, "a@example.com", "A", "correct-horse"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SignUp(ctx, "b@example.com", "B", "correct-horse"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i <= LoginMaxAttempts; i++ {
		_, _ = svc.Login(ctx, "a@example.com", "wrong-horse")
	}

	// Another address is unaffected, and the case/space variant of a is not.
	if _, err := svc.Login(ctx, "b@example.com", "correct-horse"); err != nil {
		t.Errorf("b should not be limited: %v", err)
	}
	var tooMany *TooManyAttemptsError
	if _, err := svc.Login(ctx, "  A@Example.com ", "correct-horse"); !errors.As(err, &tooMany) {
		t.Errorf("a variant spelling must share the counter, got %v", err)
	}

	// A new window starts clean.
	fixedLoginClock(t, start.Add(LoginWindow))
	if _, err := svc.Login(ctx, "a@example.com", "correct-horse"); err != nil {
		t.Errorf("next window should allow login: %v", err)
	}
}

func TestSuccessfulLoginResetsCounter(t *testing.T) {
	fixedLoginClock(t, time.Unix(1_800_000_000, 0))
	ctx := context.Background()
	svc := testPasswordService(newMemoryDB())
	if _, err := svc.SignUp(ctx, "ok@example.com", "O", "correct-horse"); err != nil {
		t.Fatal(err)
	}
	for round := 0; round < 2; round++ {
		for i := 0; i < LoginMaxAttempts-2; i++ {
			_, _ = svc.Login(ctx, "ok@example.com", "wrong-horse")
		}
		if _, err := svc.Login(ctx, "ok@example.com", "correct-horse"); err != nil {
			t.Fatalf("round %d: login should succeed, got %v", round, err)
		}
	}
}

func TestLoginAttemptKeyHoldsNoPlaintextEmail(t *testing.T) {
	store := newMemoryDB()
	fixedLoginClock(t, time.Unix(1_800_000_000, 0))
	svc := testPasswordService(store)
	_, _ = svc.Login(context.Background(), "secret.person@example.com", "whatever12")
	found := false
	for key := range store.items {
		if strings.HasPrefix(key, "LOGINATTEMPT#") {
			found = true
			if strings.Contains(key, "secret") || strings.Contains(key, "example.com") {
				t.Errorf("counter key leaks the address: %s", key)
			}
		}
	}
	if !found {
		t.Error("no counter item written")
	}
}
