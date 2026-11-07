package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"golang.org/x/crypto/bcrypt"
)

// SEC-11: a failed login must cost the same whether or not the address has an
// account. These tests count the work instead of timing it, which would be
// flaky: every failed attempt does two reads of the users table and exactly one
// bcrypt compare.

// readCountingDB counts GetItem calls on the users table.
type readCountingDB struct {
	*memoryDB
	userReads int
}

func (c *readCountingDB) GetItem(ctx context.Context, p *dynamodb.GetItemInput, o ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
	if *p.TableName == "Users" {
		c.userReads++
	}
	return c.memoryDB.GetItem(ctx, p, o...)
}

type compareCall struct{ hash string }

// recordCompares swaps in a bcrypt compare that records the hash it was given.
func recordCompares(t *testing.T) *[]compareCall {
	t.Helper()
	var calls []compareCall
	orig := comparePasswordHash
	comparePasswordHash = func(hashed, password []byte) error {
		calls = append(calls, compareCall{hash: string(hashed)})
		return orig(hashed, password)
	}
	t.Cleanup(func() { comparePasswordHash = orig })
	return &calls
}

func TestLogin_EveryFailureDoesTheSameWork(t *testing.T) {
	ctx := context.Background()

	store := newMemoryDB()
	db := &readCountingDB{memoryDB: store}
	svc := testPasswordService(db)
	if _, err := svc.SignUp(ctx, "real@example.com", "R", "correct-horse"); err != nil {
		t.Fatal(err)
	}
	// A Google-only account has a profile and an address but no password hash.
	seedUserItem(store, "google-only-1", "googleonly@example.com", "G")
	// A crashed sign-up can leave the address index without a profile.
	store.items[itemKey("EMAIL#orphan@example.com", "PROFILE")] = map[string]dbtypes.AttributeValue{
		"PK": &dbtypes.AttributeValueMemberS{Value: "EMAIL#orphan@example.com"}, "SK": &dbtypes.AttributeValueMemberS{Value: "PROFILE"},
		"userId": &dbtypes.AttributeValueMemberS{Value: "ghost-user"},
	}
	// An index entry with no user id in it.
	store.items[itemKey("EMAIL#blank@example.com", "PROFILE")] = map[string]dbtypes.AttributeValue{
		"PK": &dbtypes.AttributeValueMemberS{Value: "EMAIL#blank@example.com"}, "SK": &dbtypes.AttributeValueMemberS{Value: "PROFILE"},
	}

	calls := recordCompares(t)
	for _, tc := range []struct {
		name, email, password string
	}{
		{"unknown address", "nobody@example.com", "whatever-password"},
		{"known address, wrong password", "real@example.com", "wrong-password"},
		{"google-only account", "googleonly@example.com", "whatever-password"},
		{"address index with no profile", "orphan@example.com", "whatever-password"},
		{"address index with no user id", "blank@example.com", "whatever-password"},
	} {
		*calls = nil
		db.userReads = 0
		_, err := svc.Login(ctx, tc.email, tc.password)
		if !errors.Is(err, ErrInvalidCredentials) {
			t.Errorf("%s: err = %v, want ErrInvalidCredentials", tc.name, err)
		}
		if db.userReads != 2 {
			t.Errorf("%s: %d reads of the users table, want 2", tc.name, db.userReads)
		}
		if len(*calls) != 1 {
			t.Errorf("%s: %d bcrypt compares, want exactly 1", tc.name, len(*calls))
		}
	}

	// The success path does the same reads and one compare, and still works.
	*calls = nil
	db.userReads = 0
	if _, err := svc.Login(ctx, "real@example.com", "correct-horse"); err != nil {
		t.Fatalf("correct password: %v", err)
	}
	if db.userReads != 2 || len(*calls) != 1 {
		t.Errorf("success: %d reads, %d compares; want 2 and 1", db.userReads, len(*calls))
	}
}

func TestLogin_WithNothingRealToCompareItUsesTheDummyHash(t *testing.T) {
	svc := testPasswordService(newMemoryDB())
	calls := recordCompares(t)
	_, _ = svc.Login(context.Background(), "nobody@example.com", "whatever-password")
	if len(*calls) != 1 || (*calls)[0].hash != dummyPasswordHash {
		t.Fatalf("compare calls = %+v, want one against the dummy hash", *calls)
	}
}

func TestLogin_ARealAccountIsComparedAgainstItsOwnHash(t *testing.T) {
	ctx := context.Background()
	store := newMemoryDB()
	svc := testPasswordService(store)
	if _, err := svc.SignUp(ctx, "own@example.com", "O", "correct-horse"); err != nil {
		t.Fatal(err)
	}
	calls := recordCompares(t)
	_, _ = svc.Login(ctx, "own@example.com", "wrong-password")
	own := strAttr(profileOf(t, store, "own@example.com"), "passwordHash")
	if len(*calls) != 1 || (*calls)[0].hash != own || own == dummyPasswordHash {
		t.Fatalf("compare calls = %+v, want one against the account's own hash", *calls)
	}
}

// The dummy compare only hides anything if it costs what a real one does.
func TestDummyPasswordHash_IsAsExpensiveAsARealOne(t *testing.T) {
	cost, err := bcrypt.Cost([]byte(dummyPasswordHash))
	if err != nil {
		t.Fatalf("dummy hash is not a bcrypt hash: %v", err)
	}
	if cost != bcrypt.DefaultCost {
		t.Errorf("dummy hash cost = %d; sign-up hashes at %d, so a real compare would take a different time", cost, bcrypt.DefaultCost)
	}

	// And it is the cost sign-up really writes.
	store := newMemoryDB()
	if _, err := testPasswordService(store).SignUp(context.Background(), "cost@example.com", "C", "correct-horse"); err != nil {
		t.Fatal(err)
	}
	real, err := bcrypt.Cost([]byte(strAttr(profileOf(t, store, "cost@example.com"), "passwordHash")))
	if err != nil || real != cost {
		t.Errorf("sign-up hash cost = %d (err %v), dummy = %d", real, err, cost)
	}

	// Nobody can sign in with it: it matches none of the obvious guesses.
	for _, guess := range []string{"", "password", "dummy", "dummy password", "correct-horse", "no-such-user"} {
		if bcrypt.CompareHashAndPassword([]byte(dummyPasswordHash), []byte(guess)) == nil {
			t.Errorf("the dummy hash matches %q", guess)
		}
	}
}

// A guard against the compare being skipped some other way: a failed login for
// an address with no account must really take bcrypt time.
func TestLogin_UnknownAddressReallyTakesBcryptTime(t *testing.T) {
	svc := testPasswordService(newMemoryDB())
	start := time.Now()
	_, _ = svc.Login(context.Background(), "nobody@example.com", "whatever-password")
	if took := time.Since(start); took < 10*time.Millisecond {
		t.Errorf("an unknown address was refused in %v; a bcrypt compare at cost %d takes longer", took, bcrypt.DefaultCost)
	}
}
