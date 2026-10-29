package stockmedia

import (
	"context"
	"math"
	"net/http"
	"strconv"
	"testing"
	"time"

	"pgregory.net/rapid"
)

// Provider headers are untrusted. CodeQL flagged int(int64) conversions of
// them (go/incorrect-integer-conversion); these tests pin the bounded behaviour
// that replaced them.
func TestHeaderInt(t *testing.T) {
	huge := strconv.FormatInt(math.MaxInt64, 10)
	for _, tc := range []struct {
		name  string
		value string
		want  int
		ok    bool
	}{
		{"normal", "19684", 19684, true},
		{"zero", "0", 0, true},
		{"padded", "  42  ", 42, true},
		{"negative is used up", "-5", 0, true},
		{"just over the cap", "1000001", maxHeaderCount, true},
		{"max int64", huge, maxHeaderCount, true},
		{"beyond int64", "99999999999999999999999", maxHeaderCount, true},
		{"beyond negative int64", "-99999999999999999999999", 0, true},
		{"empty", "", 0, false},
		{"blank", "   ", 0, false},
		{"text", "soon", 0, false},
		{"float", "3.5", 0, false},
		{"hex", "0x10", 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := http.Header{}
			if tc.value != "" || tc.name == "empty" {
				h.Set("X-Test", tc.value)
			}
			got, ok := headerInt(h, "X-Test")
			if got != tc.want || ok != tc.ok {
				t.Errorf("headerInt(%q) = (%d, %v), want (%d, %v)", tc.value, got, ok, tc.want, tc.ok)
			}
		})
	}
	if _, ok := headerInt(http.Header{}, "Missing"); ok {
		t.Error("an absent header must report ok=false")
	}
}

func TestHeaderInt_AlwaysWithinBounds(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		v := rapid.OneOf(
			rapid.Custom(func(t *rapid.T) string { return strconv.FormatInt(rapid.Int64().Draw(t, "n"), 10) }),
			rapid.StringMatching(`-?[0-9]{1,40}`),
			rapid.String(),
		).Draw(t, "value")
		h := http.Header{}
		h.Set("X-Test", v)
		n, ok := headerInt(h, "X-Test")
		if !ok {
			return
		}
		if n < 0 || n > maxHeaderCount {
			t.Fatalf("headerInt(%q) = %d, outside [0, %d]", v, n, maxHeaderCount)
		}
		// Safe to use as a Duration multiplier, and small enough for a 32-bit int.
		if d := time.Duration(n) * time.Second; d < 0 {
			t.Fatalf("%d seconds overflowed a Duration", n)
		}
		if n > math.MaxInt32 {
			t.Fatalf("%d does not fit in 32 bits", n)
		}
	})
}

func TestHeaderUnix(t *testing.T) {
	good := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC).Unix()
	for _, tc := range []struct {
		name, value string
		want        time.Time
	}{
		{"valid", strconv.FormatInt(good, 10), time.Unix(good, 0)},
		{"zero", "0", time.Time{}},
		{"negative", "-1", time.Time{}},
		{"year 9999", "253402300799", time.Unix(253402300799, 0)},
		{"past year 9999", "253402300800", time.Time{}},
		{"max int64", strconv.FormatInt(math.MaxInt64, 10), time.Time{}},
		{"beyond int64", "99999999999999999999", time.Time{}},
		{"text", "tomorrow", time.Time{}},
		{"empty", "", time.Time{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := http.Header{}
			h.Set("X-Reset", tc.value)
			if got := headerUnix(h, "X-Reset"); !got.Equal(tc.want) {
				t.Errorf("headerUnix(%q) = %v, want %v", tc.value, got, tc.want)
			}
		})
	}
}

func TestPexelsQuota_HostileHeaders(t *testing.T) {
	for _, tc := range []struct {
		name            string
		remaining       string
		reset           string
		wantRemaining   int
		wantResetZero   bool
		wantQuotaNotNil bool
	}{
		{"normal", "19684", "1793000000", 19684, false, true},
		{"negative remaining reads as used up", "-3", "1793000000", 0, false, true},
		{"enormous remaining is clamped", "9223372036854775807", "1793000000", maxHeaderCount, false, true},
		{"nonsense reset is ignored", "10", "9223372036854775807", 10, true, true},
		{"no remaining header means no quota", "", "1793000000", 0, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := http.Header{}
			if tc.remaining != "" {
				h.Set("X-Ratelimit-Remaining", tc.remaining)
			}
			h.Set("X-Ratelimit-Reset", tc.reset)
			q := pexelsQuota(h)
			if (q != nil) != tc.wantQuotaNotNil {
				t.Fatalf("quota = %+v", q)
			}
			if q == nil {
				return
			}
			if q.Remaining != tc.wantRemaining || q.ResetAt.IsZero() != tc.wantResetZero {
				t.Errorf("quota = %+v, want remaining %d and zero-reset=%v", q, tc.wantRemaining, tc.wantResetZero)
			}
		})
	}
}

func TestPixabayQuota_HostileHeaders(t *testing.T) {
	clock := newFakeClock()
	p := &Pixabay{now: clock.Now}
	for _, tc := range []struct {
		name, remaining, reset string
		wantRemaining          int
		wantReset              time.Duration
	}{
		{"normal", "97", "42", 97, 42 * time.Second},
		{"negative remaining reads as used up", "-1", "42", 0, 42 * time.Second},
		{"negative reset reads as now", "5", "-100", 5, 0},
		{"enormous reset is clamped, not wrapped", "5", "9223372036854775807", 5, maxHeaderCount * time.Second},
		{"enormous remaining is clamped", "9223372036854775807", "1", maxHeaderCount, time.Second},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := http.Header{}
			h.Set("X-RateLimit-Remaining", tc.remaining)
			h.Set("X-RateLimit-Reset", tc.reset)
			q := p.quota(h)
			if q == nil || q.Remaining != tc.wantRemaining {
				t.Fatalf("quota = %+v, want remaining %d", q, tc.wantRemaining)
			}
			if got := q.ResetAt.Sub(clock.Now()); got != tc.wantReset {
				t.Errorf("reset in %v, want %v", got, tc.wantReset)
			}
			if q.ResetAt.Before(clock.Now()) {
				t.Errorf("a hostile header moved the reset into the past: %v", q.ResetAt)
			}
		})
	}
}

// End to end: a hostile header must never make the service behave badly. A
// quota header reading "huge" means plenty left, and "negative" starts a
// cooldown that is capped like any other.
func TestService_HostileQuotaHeadersAreSafe(t *testing.T) {
	for _, tc := range []struct {
		name, remaining string
		wantCoolingDown bool
	}{
		{"huge remaining means no cooldown", "9223372036854775807", false},
		{"negative remaining starts a cooldown", "-50", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := newRecorder(t, func(w http.ResponseWriter, r *http.Request, _ int) {
				w.Header().Set("X-RateLimit-Remaining", tc.remaining)
				w.Header().Set("X-RateLimit-Reset", "9223372036854775807")
				respondJSON(w, `{"totalHits":0,"hits":[]}`)
			})
			p := newTestPixabay(t, rec)
			clock := newFakeClock()
			p.now = clock.Now
			s := serviceWithClock(t, clock, p)
			ctx := context.Background()
			if _, err := s.Search(ctx, ProviderPixabay, SearchParams{Query: "a", Type: MediaPhoto}); err != nil {
				t.Fatal(err)
			}
			_, err := s.Search(ctx, ProviderPixabay, SearchParams{Query: "b", Type: MediaPhoto})
			if got := isCoolingDown(err); got != tc.wantCoolingDown {
				t.Errorf("cooling down = %v, want %v (err %v)", got, tc.wantCoolingDown, err)
			}
			if tc.wantCoolingDown {
				// The reset header is hostile, but the wait stays within the cap.
				clock.Advance(maxCooldown + time.Second)
				if _, err := s.Search(ctx, ProviderPixabay, SearchParams{Query: "c", Type: MediaPhoto}); isCoolingDown(err) {
					t.Errorf("a hostile reset header kept the provider blocked past maxCooldown")
				}
			}
		})
	}
}
