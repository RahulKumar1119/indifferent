package stockmedia

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

// reserveRig is a Pixabay stand-in that reports remaining as its quota, with
// the window resetting in 60 seconds.
func reserveRig(t *testing.T, remaining *atomic.Int64, status int) (*Service, *recorder, *fakeClock) {
	t.Helper()
	rec := newRecorder(t, func(w http.ResponseWriter, r *http.Request, _ int) {
		w.Header().Set("X-RateLimit-Remaining", strconv.FormatInt(remaining.Load(), 10))
		w.Header().Set("X-RateLimit-Reset", "60")
		if status != http.StatusOK {
			w.WriteHeader(status)
			return
		}
		respondJSON(w, `{"totalHits":0,"hits":[]}`)
	})
	p := newTestPixabay(t, rec)
	clock := newFakeClock()
	p.now = clock.Now
	return serviceWithClock(t, clock, p), rec, clock
}

// photos is unfiltered, so an empty answer is not asked again without the
// orientation filter: one search is one call.
func photos(q string) SearchParams {
	return SearchParams{Query: q, Type: MediaPhoto, Orientation: OrientationAny}
}

// #302: an automatic search leaves the last requests of the window to people.
func TestQuotaReserve_RefusesBelowTheReserveAndPeopleStillGetThrough(t *testing.T) {
	var remaining atomic.Int64
	remaining.Store(29)
	s, rec, _ := reserveRig(t, &remaining, http.StatusOK)
	ctx := context.Background()
	auto := WithQuotaReserve(ctx, 30)

	// No report yet (a fresh container): the call goes ahead.
	if _, err := s.Search(auto, ProviderPixabay, photos("first")); err != nil {
		t.Fatalf("first auto search: %v", err)
	}
	// The provider said 29 left: below the reserve.
	_, err := s.Search(auto, ProviderPixabay, photos("second"))
	if !errors.Is(err, ErrQuotaReserved) {
		t.Fatalf("err = %v, want ErrQuotaReserved", err)
	}
	if rec.count() != 1 {
		t.Errorf("a reserved search reached the provider: %d calls", rec.count())
	}
	// A person's search has no reserve and goes through.
	if _, err := s.Search(ctx, ProviderPixabay, photos("second")); err != nil {
		t.Fatalf("a person's search was refused: %v", err)
	}
	if rec.count() != 2 {
		t.Errorf("calls = %d, want 2", rec.count())
	}
	// A cached answer costs no quota, so it is still served to auto-pick.
	if res, err := s.Search(auto, ProviderPixabay, photos("first")); err != nil || !res.Cached {
		t.Errorf("cached search under the reserve: cached=%v err=%v", res.Cached, err)
	}
}

func TestQuotaReserve_AtTheReserveIsAllowed(t *testing.T) {
	var remaining atomic.Int64
	remaining.Store(30)
	s, rec, _ := reserveRig(t, &remaining, http.StatusOK)
	auto := WithQuotaReserve(context.Background(), 30)
	for _, q := range []string{"a", "b"} {
		if _, err := s.Search(auto, ProviderPixabay, photos(q)); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	if rec.count() != 2 {
		t.Errorf("calls = %d, want 2", rec.count())
	}
}

func TestQuotaReserve_ANewWindowLiftsIt(t *testing.T) {
	var remaining atomic.Int64
	remaining.Store(5)
	s, rec, clock := reserveRig(t, &remaining, http.StatusOK)
	auto := WithQuotaReserve(context.Background(), 30)
	s.Search(auto, ProviderPixabay, photos("a"))
	if _, err := s.Search(auto, ProviderPixabay, photos("b")); !errors.Is(err, ErrQuotaReserved) {
		t.Fatalf("err = %v, want ErrQuotaReserved", err)
	}
	clock.Advance(61 * time.Second) // past the reported reset
	remaining.Store(100)
	if _, err := s.Search(auto, ProviderPixabay, photos("b")); err != nil {
		t.Fatalf("after the reset: %v", err)
	}
	if rec.count() != 2 {
		t.Errorf("calls = %d, want 2", rec.count())
	}
	// And the fresh report is what counts from then on.
	if _, err := s.Search(auto, ProviderPixabay, photos("c")); err != nil {
		t.Errorf("with 100 left: %v", err)
	}
}

// A retry is one more call against the same shared limit, so a search that
// yields to people is tried once.
func TestQuotaReserve_NoRetries(t *testing.T) {
	var remaining atomic.Int64
	remaining.Store(90)
	s, rec, _ := reserveRig(t, &remaining, http.StatusServiceUnavailable)
	s.sleep = func(ctx context.Context, d time.Duration) error { return nil }
	auto := WithQuotaReserve(context.Background(), 30)
	if _, err := s.Search(auto, ProviderPixabay, photos("a")); err == nil {
		t.Fatal("a 503 was not an error")
	}
	if rec.count() != 1 {
		t.Errorf("calls = %d, want 1 (no retry)", rec.count())
	}
	// The control: a person's search is retried.
	s.Search(context.Background(), ProviderPixabay, photos("b"))
	if rec.count() <= 2 {
		t.Errorf("a person's search was not retried: %d calls in all", rec.count())
	}
}

// The cooldown after a refusal still applies to automatic searches.
func TestQuotaReserve_TheCooldownStillApplies(t *testing.T) {
	var remaining atomic.Int64
	remaining.Store(0) // used up: a cooldown starts
	s, rec, _ := reserveRig(t, &remaining, http.StatusOK)
	ctx := context.Background()
	s.Search(ctx, ProviderPixabay, photos("a"))
	_, err := s.Search(WithQuotaReserve(ctx, 30), ProviderPixabay, photos("b"))
	if !errors.Is(err, ErrCoolingDown) {
		t.Errorf("err = %v, want ErrCoolingDown", err)
	}
	if rec.count() != 1 {
		t.Errorf("calls = %d, want 1", rec.count())
	}
}
