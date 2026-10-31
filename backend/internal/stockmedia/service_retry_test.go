package stockmedia

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

// retryRig is a Service with the default retry policy, a clock that only moves
// when the service waits, and a record of every wait and log line.
type retryRig struct {
	*Service
	clock *fakeClock

	mu    sync.Mutex
	waits []time.Duration
	logs  []string
}

func newRetryRig(t *testing.T, tweak func(*RetryPolicy), providers ...Provider) *retryRig {
	t.Helper()
	r := &retryRig{clock: newFakeClock()}
	policy := DefaultRetryPolicy()
	policy.jitter = func() float64 { return 0.5 } // waits are exactly BaseDelay * 2^n
	if tweak != nil {
		tweak(&policy)
	}
	s, err := NewService(providers,
		WithURLPolicy(allowAnyHost), WithClock(r.clock.Now), WithRetry(policy),
		WithLogger(func(format string, args ...any) {
			r.mu.Lock()
			defer r.mu.Unlock()
			r.logs = append(r.logs, sprintf(format, args...))
		}))
	if err != nil {
		t.Fatal(err)
	}
	s.sleep = func(ctx context.Context, d time.Duration) error {
		r.mu.Lock()
		r.waits = append(r.waits, d)
		r.mu.Unlock()
		r.clock.Advance(d)
		return ctx.Err()
	}
	r.Service = s
	return r
}

func (r *retryRig) waited() []time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]time.Duration(nil), r.waits...)
}

func (r *retryRig) logged(substr string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, l := range r.logs {
		if strings.Contains(l, substr) {
			return true
		}
	}
	return false
}

func searchPhotos(q string) SearchParams { return SearchParams{Query: q, Type: MediaPhoto} }

// flaky answers the first failures calls with err, then with one good asset.
func flaky(name ProviderName, failures int, err error) *stubProvider {
	good := okProvider(name, goodAsset(name, "1", MediaPhoto, 1280, 1920))
	return &stubProvider{name: name, fn: func(p SearchParams, n int) (SearchResult, error) {
		if n <= failures {
			return SearchResult{}, err
		}
		return good.fn(p, n)
	}}
}

func sameDurations(got, want []time.Duration) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func TestRetry_SearchRecoversFromAnOutage(t *testing.T) {
	prov := flaky(ProviderPexels, 1, &ProviderError{Provider: ProviderPexels, Kind: KindUnavailable, Status: 503})
	r := newRetryRig(t, nil, prov)

	res, err := r.Search(context.Background(), ProviderPexels, searchPhotos("lighthouse"))
	if err != nil || len(res.Assets) != 1 {
		t.Fatalf("err = %v, assets = %d; want the second try's answer", err, len(res.Assets))
	}
	if prov.callCount() != 2 {
		t.Errorf("provider calls = %d, want 2", prov.callCount())
	}
	if w := r.waited(); !sameDurations(w, []time.Duration{250 * time.Millisecond}) {
		t.Errorf("waits = %v, want [250ms]", w)
	}
	for _, want := range []string{"stockmedia_retry provider=pexels op=search attempt=1 wait_ms=250", "kind=unavailable status=503", "stockmedia_recovered provider=pexels op=search attempts=2"} {
		if !r.logged(want) {
			t.Errorf("no log line containing %q in %q", want, r.logs)
		}
	}
	// The recovered answer is cached like any other.
	if _, err := r.Search(context.Background(), ProviderPexels, searchPhotos("lighthouse")); err != nil || prov.callCount() != 2 {
		t.Errorf("second search: err = %v, calls = %d; want a cache hit", err, prov.callCount())
	}
}

func TestRetry_ACallThatWorksFirstTimeLeavesNoRetryNoise(t *testing.T) {
	prov := okProvider(ProviderPexels, goodAsset(ProviderPexels, "1", MediaPhoto, 1280, 1920))
	r := newRetryRig(t, nil, prov)
	if _, err := r.Search(context.Background(), ProviderPexels, searchPhotos("a")); err != nil {
		t.Fatal(err)
	}
	for _, noise := range []string{"stockmedia_retry", "stockmedia_recovered", "stockmedia_gave_up", "stockmedia_provider_error"} {
		if r.logged(noise) {
			t.Errorf("a clean call logged %q: %q", noise, r.logs)
		}
	}
	if len(r.waited()) != 0 {
		t.Errorf("waited %v on a clean call", r.waited())
	}
}

func TestRetry_TheSearchLogShowsHowMuchQuotaIsLeft(t *testing.T) {
	good := goodAsset(ProviderPexels, "1", MediaPhoto, 1280, 1920)
	var r *retryRig
	prov := &stubProvider{name: ProviderPexels, fn: func(p SearchParams, _ int) (SearchResult, error) {
		return SearchResult{Provider: ProviderPexels, Type: p.Type, Assets: []Asset{good},
			Quota: &Quota{Remaining: 17, ResetAt: r.clock.Now().Add(30 * time.Second)}}, nil
	}}
	r = newRetryRig(t, nil, prov)
	if _, err := r.Search(context.Background(), ProviderPexels, searchPhotos("a")); err != nil {
		t.Fatal(err)
	}
	if !r.logged("quota_remaining=17 quota_reset_s=30") {
		t.Errorf("the quota was not logged: %q", r.logs)
	}

	// A provider that says nothing about its quota adds no fields.
	quiet := newRetryRig(t, nil, okProvider(ProviderPexels, good))
	if _, err := quiet.Search(context.Background(), ProviderPexels, searchPhotos("a")); err != nil {
		t.Fatal(err)
	}
	if quiet.logged("quota_") {
		t.Errorf("quota fields appeared with no quota: %q", quiet.logs)
	}
}

func TestRetry_ServicesRetryByDefault(t *testing.T) {
	prov := flaky(ProviderPexels, 1, &ProviderError{Provider: ProviderPexels, Kind: KindUnavailable, Status: 503})
	// No WithRetry: this is how the API builds its service. It really waits, for
	// about the first backoff (250ms, varied by a quarter).
	s, err := NewService([]Provider{prov}, WithLogger(func(string, ...any) {}), WithURLPolicy(allowAnyHost))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Search(context.Background(), ProviderPexels, searchPhotos("a")); err != nil || prov.callCount() != 2 {
		t.Errorf("err = %v, provider calls = %d; want one retry to rescue the search", err, prov.callCount())
	}
}

func TestRetry_ACancelledCallIsNotLoggedAsGivingUp(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	prov := &ctxProvider{name: ProviderPexels, fn: func(ctx context.Context, n int) (SearchResult, error) {
		if n == 1 {
			return SearchResult{}, &ProviderError{Provider: ProviderPexels, Kind: KindUnavailable, Status: 503}
		}
		cancel() // the caller leaves during the retry
		return SearchResult{}, ctx.Err()
	}}
	r := newRetryRig(t, nil, prov)
	_, err := r.Search(ctx, ProviderPexels, searchPhotos("a"))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if prov.calls() != 2 {
		t.Fatalf("provider calls = %d, want 2 (the test needs a retry first)", prov.calls())
	}
	if r.logged("stockmedia_gave_up") {
		t.Errorf("a caller leaving was logged as the provider failing: %q", r.logs)
	}
}

func TestRetry_AQuotaWhoseResetHasPassedLogsZeroSecondsNotANegativeNumber(t *testing.T) {
	good := goodAsset(ProviderPexels, "1", MediaPhoto, 1280, 1920)
	var r *retryRig
	prov := &stubProvider{name: ProviderPexels, fn: func(p SearchParams, _ int) (SearchResult, error) {
		return SearchResult{Provider: ProviderPexels, Type: p.Type, Assets: []Asset{good},
			Quota: &Quota{Remaining: 9, ResetAt: r.clock.Now().Add(-5 * time.Second)}}, nil
	}}
	r = newRetryRig(t, nil, prov)
	if _, err := r.Search(context.Background(), ProviderPexels, searchPhotos("a")); err != nil {
		t.Fatal(err)
	}
	if !r.logged("quota_remaining=9 quota_reset_s=0") {
		t.Errorf("log = %q", r.logs)
	}
}

func TestRetry_AQuotaWithNoResetTimeLogsOnlyWhatIsRemaining(t *testing.T) {
	good := goodAsset(ProviderPexels, "1", MediaPhoto, 1280, 1920)
	prov := &stubProvider{name: ProviderPexels, fn: func(p SearchParams, _ int) (SearchResult, error) {
		return SearchResult{Provider: ProviderPexels, Type: p.Type, Assets: []Asset{good}, Quota: &Quota{Remaining: 5}}, nil
	}}
	r := newRetryRig(t, nil, prov)
	if _, err := r.Search(context.Background(), ProviderPexels, searchPhotos("a")); err != nil {
		t.Fatal(err)
	}
	if !r.logged("quota_remaining=5") || r.logged("quota_reset_s") {
		t.Errorf("log = %q", r.logs)
	}
}

func TestRetry_SearchGivesUpAfterTheLastAttempt(t *testing.T) {
	prov := failProvider(ProviderPexels, &ProviderError{Provider: ProviderPexels, Kind: KindUnavailable, Status: 502})
	r := newRetryRig(t, nil, prov)

	_, err := r.Search(context.Background(), ProviderPexels, searchPhotos("lighthouse"))
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("err = %v, want ErrUnavailable", err)
	}
	if prov.callCount() != 3 {
		t.Errorf("provider calls = %d, want 3 (DefaultRetryPolicy)", prov.callCount())
	}
	if w := r.waited(); !sameDurations(w, []time.Duration{250 * time.Millisecond, 500 * time.Millisecond}) {
		t.Errorf("waits = %v, want [250ms 500ms]", w)
	}
	if !r.logged("stockmedia_gave_up provider=pexels op=search attempts=3") {
		t.Errorf("giving up was not logged: %q", r.logs)
	}
	// An outage is not the provider asking us to slow down.
	if err := r.checkCooldown(ProviderPexels); err != nil {
		t.Errorf("an outage started a cooldown: %v", err)
	}
}

func TestRetry_AShortRateLimitIsWaitedOutInFull(t *testing.T) {
	var calledAt []time.Time
	var r *retryRig
	prov := flaky(ProviderPixabay, 1, rateLimitErr(ProviderPixabay, time.Second))
	inner := prov.fn
	prov.fn = func(p SearchParams, n int) (SearchResult, error) {
		calledAt = append(calledAt, r.clock.Now())
		return inner(p, n)
	}
	r = newRetryRig(t, nil, prov)

	if _, err := r.Search(context.Background(), ProviderPixabay, searchPhotos("lighthouse")); err != nil {
		t.Fatal(err)
	}
	if len(calledAt) != 2 {
		t.Fatalf("provider calls = %d, want 2", len(calledAt))
	}
	if gap := calledAt[1].Sub(calledAt[0]); gap < time.Second {
		t.Errorf("retried after %v, but the provider asked for 1s", gap)
	}
	if w := r.waited(); !sameDurations(w, []time.Duration{time.Second}) {
		t.Errorf("waits = %v, want [1s]", w)
	}
	if !r.logged("kind=rate_limited status=429 retry_after_s=1") {
		t.Errorf("the rate-limit hit was not logged with its wait: %q", r.logs)
	}
	if err := r.checkCooldown(ProviderPixabay); err != nil {
		t.Errorf("still cooling down after the wait: %v", err)
	}
}

func TestRetry_ARateLimitWithNoWaitGivenIsNotRetriedAndCoolsDown(t *testing.T) {
	// Pexels sends no headers on a 429, so the wait is unknown.
	prov := failProvider(ProviderPexels, rateLimitErr(ProviderPexels, 0))
	r := newRetryRig(t, nil, prov)

	_, err := r.Search(context.Background(), ProviderPexels, searchPhotos("a"))
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("err = %v, want ErrRateLimited", err)
	}
	if prov.callCount() != 1 || len(r.waited()) != 0 {
		t.Errorf("calls = %d, waits = %v; want one call and no wait", prov.callCount(), r.waited())
	}
	// The cooldown holds the provider off, and nothing waits for it.
	_, err = r.Search(context.Background(), ProviderPexels, searchPhotos("b"))
	if !errors.Is(err, ErrCoolingDown) || prov.callCount() != 1 || len(r.waited()) != 0 {
		t.Errorf("err = %v, calls = %d, waits = %v; want a refusal with no call and no wait", err, prov.callCount(), r.waited())
	}
}

func TestRetry_ALongRateLimitFailsAtOnceSoTheCallerCanUseAnotherProvider(t *testing.T) {
	prov := failProvider(ProviderPixabay, rateLimitErr(ProviderPixabay, 30*time.Second))
	r := newRetryRig(t, nil, prov)

	start := time.Now()
	_, err := r.Search(context.Background(), ProviderPixabay, searchPhotos("a"))
	if !errors.Is(err, ErrRateLimited) || prov.callCount() != 1 || len(r.waited()) != 0 {
		t.Fatalf("err = %v, calls = %d, waits = %v; want one call, no wait", err, prov.callCount(), r.waited())
	}
	if time.Since(start) > time.Second {
		t.Error("a 30 second rate limit held the caller up")
	}
	var pe *ProviderError
	_ = errors.As(r.checkCooldown(ProviderPixabay), &pe)
	if pe == nil || pe.RetryAfter != 30*time.Second {
		t.Errorf("cooldown = %+v, want the 30s the provider asked for", pe)
	}
}

func TestRetry_AProviderInCooldownIsNotWaitedFor(t *testing.T) {
	prov := okProvider(ProviderPixabay, goodAsset(ProviderPixabay, "1", MediaPhoto, 1280, 1920))
	r := newRetryRig(t, nil, prov)
	r.startCooldown(ProviderPixabay, time.Second) // well inside MaxRateLimitWait

	_, err := r.Search(context.Background(), ProviderPixabay, searchPhotos("a"))
	if !errors.Is(err, ErrCoolingDown) || prov.callCount() != 0 || len(r.waited()) != 0 {
		t.Errorf("err = %v, calls = %d, waits = %v; want an immediate refusal", err, prov.callCount(), r.waited())
	}
	if !r.logged("stockmedia_search_skipped provider=pixabay reason=cooldown") {
		t.Errorf("skipped call not logged: %q", r.logs)
	}
}

func TestRetry_ARetryingProviderDoesNotHoldBackTheOtherInASearchOfBoth(t *testing.T) {
	pexels := okProvider(ProviderPexels, goodAsset(ProviderPexels, "1", MediaPhoto, 1280, 1920))
	pixabay := failProvider(ProviderPixabay, &ProviderError{Provider: ProviderPixabay, Kind: KindUnavailable, Status: 503})
	r := newRetryRig(t, nil, pexels, pixabay)

	m, err := r.SearchAll(context.Background(), searchPhotos("lighthouse"))
	if err != nil {
		t.Fatalf("err = %v: one working provider must be enough", err)
	}
	if ids(m.Assets()) != "pexels:1" || len(m.Failures) != 1 || m.Failures[0].Provider != ProviderPixabay || !errors.Is(m.Failures[0].Err, ErrUnavailable) {
		t.Errorf("assets = %s, failures = %+v", ids(m.Assets()), m.Failures)
	}
	if pexels.callCount() != 1 || pixabay.callCount() != 3 {
		t.Errorf("calls: pexels %d, pixabay %d; want 1 and 3", pexels.callCount(), pixabay.callCount())
	}
}

func TestRetry_ErrorsThatTryingAgainCannotFixAreReturnedAtOnce(t *testing.T) {
	for name, err := range map[string]error{
		"bad key":      &ProviderError{Provider: ProviderPexels, Kind: KindAuth, Status: 401},
		"bad request":  &ProviderError{Provider: ProviderPexels, Kind: KindBadRequest, Status: 400},
		"not found":    &ProviderError{Provider: ProviderPexels, Kind: KindNotFound, Status: 404},
		"bad response": &ProviderError{Provider: ProviderPexels, Kind: KindBadResponse, Status: 200},
	} {
		t.Run(name, func(t *testing.T) {
			prov := failProvider(ProviderPexels, err)
			r := newRetryRig(t, nil, prov)
			if _, got := r.Search(context.Background(), ProviderPexels, searchPhotos("a")); got == nil {
				t.Fatal("no error")
			}
			if prov.callCount() != 1 || len(r.waited()) != 0 {
				t.Errorf("calls = %d, waits = %v; want one call, no wait", prov.callCount(), r.waited())
			}
		})
	}
}

func TestRetry_GetRecoversFromAnOutage(t *testing.T) {
	prov := &stubProvider{name: ProviderPixabay}
	prov.getFn = func(typ MediaType, id string, n int) (Asset, error) {
		if n == 1 {
			return Asset{}, &ProviderError{Provider: ProviderPixabay, Kind: KindUnavailable}
		}
		return goodAsset(ProviderPixabay, id, typ, 1280, 1920), nil
	}
	r := newRetryRig(t, nil, prov)

	a, err := r.Get(context.Background(), ProviderPixabay, MediaPhoto, "42")
	if err != nil || a.ID != "42" {
		t.Fatalf("asset = %+v, err = %v", a, err)
	}
	if prov.getCount() != 2 || !r.logged("stockmedia_recovered provider=pixabay op=get attempts=2") {
		t.Errorf("gets = %d, logs = %q", prov.getCount(), r.logs)
	}
	if _, err := r.Get(context.Background(), ProviderPixabay, MediaPhoto, "42"); err != nil || prov.getCount() != 2 {
		t.Errorf("second Get: err = %v, gets = %d; want a cache hit", err, prov.getCount())
	}
}

func TestRetry_AMissingAssetIsNotRetried(t *testing.T) {
	prov := &stubProvider{name: ProviderPixabay} // no getFn: every lookup is ErrNotFound
	r := newRetryRig(t, nil, prov)
	if _, err := r.Get(context.Background(), ProviderPixabay, MediaPhoto, "1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
	if prov.getCount() != 1 || len(r.waited()) != 0 {
		t.Errorf("gets = %d, waits = %v; want one lookup, no wait", prov.getCount(), r.waited())
	}
}

// ctxProvider is a provider whose search can be held until its context ends.
type ctxProvider struct {
	name ProviderName
	mu   sync.Mutex
	n    int
	fn   func(ctx context.Context, n int) (SearchResult, error)
}

func (c *ctxProvider) Name() ProviderName { return c.name }
func (c *ctxProvider) Search(ctx context.Context, _ SearchParams) (SearchResult, error) {
	c.mu.Lock()
	c.n++
	n := c.n
	c.mu.Unlock()
	return c.fn(ctx, n)
}
func (c *ctxProvider) Get(context.Context, MediaType, string) (Asset, error) {
	return Asset{}, ErrNotFound
}
func (c *ctxProvider) calls() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.n
}

func TestRetry_ARetryCutOffByTheBudgetIsAnOutageNotACancellation(t *testing.T) {
	var r *retryRig
	prov := &ctxProvider{name: ProviderPexels, fn: func(ctx context.Context, n int) (SearchResult, error) {
		if n == 1 {
			return SearchResult{}, &ProviderError{Provider: ProviderPexels, Kind: KindUnavailable, Status: 503}
		}
		<-ctx.Done() // a provider that never answers; the budget ends the wait
		// The deadline is real time and the rig's clock is fake: tell the clock
		// what the wait cost, so the service sees the budget spent.
		r.clock.Advance(80 * time.Millisecond)
		return SearchResult{}, ctx.Err()
	}}
	r = newRetryRig(t, func(p *RetryPolicy) {
		p.BaseDelay, p.MaxDelay = time.Millisecond, time.Millisecond
		p.Budget, p.minAttempt = 80*time.Millisecond, 10*time.Millisecond
	}, prov)

	_, err := r.Search(context.Background(), ProviderPexels, searchPhotos("a"))
	if errors.Is(err, context.DeadlineExceeded) || !errors.Is(err, ErrUnavailable) {
		t.Fatalf("err = %v, want a provider outage (ErrUnavailable), not a context error", err)
	}
	if prov.calls() != 2 {
		t.Errorf("provider calls = %d, want 2", prov.calls())
	}
}

func TestRetry_ACallerWhoCancelsGetsTheCancellationNotAnOutage(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	prov := &ctxProvider{name: ProviderPexels, fn: func(ctx context.Context, _ int) (SearchResult, error) {
		cancel()
		<-ctx.Done()
		return SearchResult{}, ctx.Err()
	}}
	r := newRetryRig(t, nil, prov)

	_, err := r.Search(ctx, ProviderPexels, searchPhotos("a"))
	if !errors.Is(err, context.Canceled) || errors.Is(err, ErrUnavailable) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
	if prov.calls() != 1 || len(r.waited()) != 0 {
		t.Errorf("calls = %d, waits = %v; want one call, no wait", prov.calls(), r.waited())
	}
}

// The real Pixabay client over HTTP: an outage on the wire is retried, and
// nothing about the retries puts the key or the address in the logs.
func TestRetry_PixabayOverHTTPRecoversAndLogsNoSecrets(t *testing.T) {
	rec := newRecorder(t, func(w http.ResponseWriter, r *http.Request, n int) {
		if n <= 2 {
			http.Error(w, "upstream says: key="+testPixabayKey, http.StatusServiceUnavailable)
			return
		}
		respondJSON(w, pixabayImagesFixture)
	})
	r := newRetryRig(t, nil, newTestPixabay(t, rec))

	res, err := r.Search(context.Background(), ProviderPixabay, searchPhotos("flower"))
	if err != nil || len(res.Assets) == 0 {
		t.Fatalf("err = %v, assets = %d", err, len(res.Assets))
	}
	if rec.count() != 3 {
		t.Errorf("HTTP requests = %d, want 3", rec.count())
	}
	r.mu.Lock()
	all := strings.Join(r.logs, "\n")
	r.mu.Unlock()
	for _, secret := range []string{testPixabayKey, rec.URL, "127.0.0.1"} {
		if strings.Contains(all, secret) {
			t.Errorf("a log line holds %q:\n%s", secret, all)
		}
	}
}

func TestRetry_PixabayOverHTTPWaitsOutTheWindowItNames(t *testing.T) {
	rec := newRecorder(t, func(w http.ResponseWriter, r *http.Request, n int) {
		if n == 1 {
			w.Header().Set("Retry-After", "1")
			http.Error(w, "API rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		respondJSON(w, pixabayImagesFixture)
	})
	r := newRetryRig(t, nil, newTestPixabay(t, rec))

	if _, err := r.Search(context.Background(), ProviderPixabay, searchPhotos("flower")); err != nil {
		t.Fatal(err)
	}
	if rec.count() != 2 || !sameDurations(r.waited(), []time.Duration{time.Second}) {
		t.Errorf("requests = %d, waits = %v; want 2 requests with a 1s wait", rec.count(), r.waited())
	}
}
