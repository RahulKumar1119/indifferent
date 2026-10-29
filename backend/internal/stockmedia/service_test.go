package stockmedia

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

// okProvider returns a provider that always answers with the given assets.
func okProvider(name ProviderName, assets ...Asset) *stubProvider {
	return &stubProvider{name: name, fn: func(p SearchParams, _ int) (SearchResult, error) {
		return SearchResult{Provider: name, Type: p.Type, Assets: append([]Asset(nil), assets...), Total: len(assets), Page: p.Page, PerPage: p.PerPage,
			Attribution: Attribution{Text: string(name) + " credit", URL: "https://" + string(name) + ".test"}}, nil
	}}
}

// failProvider returns a provider that always fails with the given error.
func failProvider(name ProviderName, err error) *stubProvider {
	return &stubProvider{name: name, fn: func(SearchParams, int) (SearchResult, error) { return SearchResult{}, err }}
}

func rateLimitErr(name ProviderName, retry time.Duration) error {
	return &ProviderError{Provider: name, Kind: KindRateLimited, Status: 429, RetryAfter: retry}
}

func serviceWithClock(t *testing.T, clock *fakeClock, providers ...Provider) *Service {
	t.Helper()
	s, err := NewService(providers, WithLogger(func(string, ...any) {}), WithURLPolicy(allowAnyHost), WithClock(clock.Now))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestNewService(t *testing.T) {
	if _, err := NewService(nil); !errors.Is(err, ErrNotConfigured) {
		t.Errorf("no providers: err = %v, want ErrNotConfigured", err)
	}
	if _, err := NewService([]Provider{nil}); !errors.Is(err, ErrNotConfigured) {
		t.Errorf("nil provider: err = %v, want ErrNotConfigured", err)
	}
	a, b := okProvider(ProviderPexels), okProvider(ProviderPexels)
	if _, err := NewService([]Provider{a, b}); err == nil {
		t.Error("registering a provider twice must be an error")
	}
	s := testService(t, okProvider(ProviderPixabay), okProvider(ProviderPexels))
	if got := fmt.Sprint(s.Providers()); got != "[pixabay pexels]" {
		t.Errorf("Providers() = %s, want registration order", got)
	}
}

func TestSearch_UnknownProvider(t *testing.T) {
	s := testService(t, okProvider(ProviderPexels))
	_, err := s.Search(context.Background(), ProviderPixabay, SearchParams{Query: "x", Type: MediaPhoto})
	if !errors.Is(err, ErrNotConfigured) {
		t.Errorf("err = %v, want ErrNotConfigured", err)
	}
}

func TestSearch_ValidatesParametersBeforeAnyCall(t *testing.T) {
	prov := okProvider(ProviderPexels)
	s := testService(t, prov)
	long := strings.Repeat("a", MaxQueryRunes+1)
	for name, p := range map[string]SearchParams{
		"empty query":       {Query: "", Type: MediaPhoto},
		"blank query":       {Query: " \t\n ", Type: MediaPhoto},
		"query too long":    {Query: long, Type: MediaPhoto},
		"no type":           {Query: "x"},
		"bad type":          {Query: "x", Type: "audio"},
		"bad orientation":   {Query: "x", Type: MediaPhoto, Orientation: "diagonal"},
		"negative page":     {Query: "x", Type: MediaPhoto, Page: -1},
		"page too high":     {Query: "x", Type: MediaPhoto, Page: maxPage + 1},
		"per-page too low":  {Query: "x", Type: MediaPhoto, PerPage: MinPerPage - 1},
		"per-page too high": {Query: "x", Type: MediaPhoto, PerPage: MaxPerPage + 1},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := s.Search(context.Background(), ProviderPexels, p); !errors.Is(err, ErrInvalidParams) {
				t.Errorf("Search err = %v, want ErrInvalidParams", err)
			}
			if _, err := s.SearchAll(context.Background(), p); !errors.Is(err, ErrInvalidParams) {
				t.Errorf("SearchAll err = %v, want ErrInvalidParams", err)
			}
		})
	}
	if prov.callCount() != 0 {
		t.Errorf("provider called %d times for invalid input, want 0", prov.callCount())
	}
}

func TestSearch_DefaultsToPortraitAndFirstPage(t *testing.T) {
	prov := okProvider(ProviderPexels, goodAsset(ProviderPexels, "1", MediaPhoto, 3000, 4000))
	s := testService(t, prov)
	if _, err := s.Search(context.Background(), ProviderPexels, SearchParams{Query: "x", Type: MediaPhoto}); err != nil {
		t.Fatal(err)
	}
	got := prov.call(0)
	if got.Orientation != OrientationPortrait || got.Page != 1 || got.PerPage != DefaultPerPage {
		t.Errorf("provider saw %+v, want portrait page 1 per-page %d", got, DefaultPerPage)
	}
}

func TestSearch_QueryIsNormalizedBeforeCaching(t *testing.T) {
	prov := okProvider(ProviderPexels, goodAsset(ProviderPexels, "1", MediaPhoto, 3000, 4000))
	s := testService(t, prov)
	ctx := context.Background()
	for _, q := range []string{"Forest Walk", "  forest   walk ", "FOREST\twalk"} {
		if _, err := s.Search(ctx, ProviderPexels, SearchParams{Query: q, Type: MediaPhoto}); err != nil {
			t.Fatal(err)
		}
	}
	if prov.callCount() != 1 {
		t.Errorf("provider called %d times, want 1 (same query up to case and spacing)", prov.callCount())
	}
	if got := prov.call(0).Query; got != "Forest Walk" {
		t.Errorf("provider saw %q, want the first spelling with spacing cleaned", got)
	}
}

func TestSearch_CacheHitMissAndKeys(t *testing.T) {
	prov := okProvider(ProviderPexels, goodAsset(ProviderPexels, "1", MediaPhoto, 3000, 4000))
	s := testService(t, prov)
	ctx := context.Background()
	base := SearchParams{Query: "city", Type: MediaPhoto}

	first, err := s.Search(ctx, ProviderPexels, base)
	if err != nil || first.Cached {
		t.Fatalf("first: err=%v cached=%v", err, first.Cached)
	}
	second, err := s.Search(ctx, ProviderPexels, base)
	if err != nil || !second.Cached {
		t.Fatalf("second should be a cache hit: err=%v cached=%v", err, second.Cached)
	}
	if prov.callCount() != 1 {
		t.Fatalf("calls = %d, want 1", prov.callCount())
	}

	for name, p := range map[string]SearchParams{
		"other page":        {Query: "city", Type: MediaPhoto, Page: 2},
		"other per-page":    {Query: "city", Type: MediaPhoto, PerPage: 30},
		"other orientation": {Query: "city", Type: MediaPhoto, Orientation: OrientationLandscape},
		"other type":        {Query: "city", Type: MediaVideo},
		"other query":       {Query: "town", Type: MediaPhoto},
	} {
		before := prov.callCount()
		if _, err := s.Search(ctx, ProviderPexels, p); err != nil {
			t.Fatal(err)
		}
		if prov.callCount() != before+1 {
			t.Errorf("%s: should not share a cache entry", name)
		}
	}
}

func TestSearch_CacheEntriesAreNotShared(t *testing.T) {
	prov := okProvider(ProviderPexels,
		goodAsset(ProviderPexels, "1", MediaPhoto, 3000, 4000),
		goodAsset(ProviderPexels, "2", MediaPhoto, 3000, 4000))
	s := testService(t, prov)
	p := SearchParams{Query: "x", Type: MediaPhoto}
	a, _ := s.Search(context.Background(), ProviderPexels, p)
	a.Assets[0].Author = "tampered"
	a.Assets[0], a.Assets[1] = a.Assets[1], a.Assets[0]
	b, _ := s.Search(context.Background(), ProviderPexels, p)
	if ids(b.Assets) != "pexels:1,pexels:2" || b.Assets[0].Author == "tampered" {
		t.Errorf("a caller's edits reached the cache: %s %q", ids(b.Assets), b.Assets[0].Author)
	}
}

func TestSearch_CacheExpiry(t *testing.T) {
	clock := newFakeClock()
	cache := NewMemoryCache(10)
	cache.now = clock.Now
	pexels := okProvider(ProviderPexels, goodAsset(ProviderPexels, "1", MediaPhoto, 3000, 4000))
	pixabay := okProvider(ProviderPixabay, goodAsset(ProviderPixabay, "1", MediaPhoto, 3000, 4000))
	s, err := NewService([]Provider{pexels, pixabay}, WithLogger(func(string, ...any) {}), WithURLPolicy(allowAnyHost), WithClock(clock.Now), WithCache(cache))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	p := SearchParams{Query: "x", Type: MediaPhoto}
	for _, name := range []ProviderName{ProviderPexels, ProviderPixabay} {
		if _, err := s.Search(ctx, name, p); err != nil {
			t.Fatal(err)
		}
	}

	// Pixabay requires caching for 24 hours; Pexels is kept for an hour.
	clock.Advance(59 * time.Minute)
	s.Search(ctx, ProviderPexels, p)
	s.Search(ctx, ProviderPixabay, p)
	if pexels.callCount() != 1 || pixabay.callCount() != 1 {
		t.Fatalf("within the hour both should be cached: pexels=%d pixabay=%d", pexels.callCount(), pixabay.callCount())
	}
	clock.Advance(2 * time.Minute) // 61 minutes
	s.Search(ctx, ProviderPexels, p)
	s.Search(ctx, ProviderPixabay, p)
	if pexels.callCount() != 2 {
		t.Errorf("pexels should have expired after an hour, calls = %d", pexels.callCount())
	}
	if pixabay.callCount() != 1 {
		t.Errorf("pixabay must stay cached for 24 hours (API terms), calls = %d", pixabay.callCount())
	}
	clock.Advance(23 * time.Hour) // 24h01m after the first Pixabay call
	s.Search(ctx, ProviderPixabay, p)
	if pixabay.callCount() != 2 {
		t.Errorf("pixabay should refetch after 24 hours, calls = %d", pixabay.callCount())
	}
}

func TestSearch_ErrorsAreNotCached(t *testing.T) {
	boom := &ProviderError{Provider: ProviderPexels, Kind: KindUnavailable, Status: 503}
	prov := &stubProvider{name: ProviderPexels, fn: func(p SearchParams, n int) (SearchResult, error) {
		if n == 1 {
			return SearchResult{}, boom
		}
		return SearchResult{Provider: ProviderPexels, Type: p.Type, Assets: []Asset{goodAsset(ProviderPexels, "1", MediaPhoto, 3000, 4000)}}, nil
	}}
	s := testService(t, prov)
	p := SearchParams{Query: "x", Type: MediaPhoto}
	if _, err := s.Search(context.Background(), ProviderPexels, p); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("first err = %v", err)
	}
	res, err := s.Search(context.Background(), ProviderPexels, p)
	if err != nil || len(res.Assets) != 1 {
		t.Errorf("a failure must not be cached; second call: res=%v err=%v", ids(res.Assets), err)
	}
}

func TestSearch_EmptyResultIsCachedAndNotAnError(t *testing.T) {
	prov := okProvider(ProviderPexels)
	s := testService(t, prov)
	p := SearchParams{Query: "zzzz", Type: MediaPhoto, Orientation: OrientationAny}
	for i := 0; i < 3; i++ {
		res, err := s.Search(context.Background(), ProviderPexels, p)
		if err != nil || len(res.Assets) != 0 {
			t.Fatalf("call %d: res=%v err=%v", i, ids(res.Assets), err)
		}
	}
	if prov.callCount() != 1 {
		t.Errorf("calls = %d, want 1: an empty result is still an answer", prov.callCount())
	}
}

func TestSearch_DropsInvalidAssetsAndCountsWhy(t *testing.T) {
	good := goodAsset(ProviderPexels, "1", MediaPhoto, 3000, 4000)
	tiny := goodAsset(ProviderPexels, "2", MediaPhoto, 400, 600)
	noURL := goodAsset(ProviderPexels, "3", MediaPhoto, 3000, 4000)
	noURL.MediaURL = ""
	httpURL := goodAsset(ProviderPexels, "4", MediaPhoto, 3000, 4000)
	httpURL.MediaURL = "http://pexels.test/plain"
	noDur := goodAsset(ProviderPexels, "5", MediaVideo, 1080, 1920)
	noDur.Duration = 0
	prov := okProvider(ProviderPexels, tiny, good, noURL, httpURL, noDur)
	prov.fn = wrapWithSkipped(prov.fn, 2)
	s := testService(t, prov)

	res, err := s.Search(context.Background(), ProviderPexels, SearchParams{Query: "x", Type: MediaPhoto, Orientation: OrientationAny})
	if err != nil {
		t.Fatal(err)
	}
	if ids(res.Assets) != "pexels:1" {
		t.Errorf("assets = %s, want only pexels:1", ids(res.Assets))
	}
	if res.Skipped != 2+4 {
		t.Errorf("Skipped = %d, want 2 from the provider plus 4 invalid", res.Skipped)
	}
	for reason, want := range map[string]int{"resolution": 1, "url": 2, "duration": 1} {
		if res.SkipReasons[reason] != want {
			t.Errorf("SkipReasons[%s] = %d, want %d (all: %v)", reason, res.SkipReasons[reason], want, res.SkipReasons)
		}
	}
}

// wrapWithSkipped makes a provider report n assets it already dropped.
func wrapWithSkipped(fn func(SearchParams, int) (SearchResult, error), n int) func(SearchParams, int) (SearchResult, error) {
	return func(p SearchParams, c int) (SearchResult, error) {
		r, err := fn(p, c)
		r.Skipped = n
		return r, err
	}
}

func TestSearch_OrdersByOrientationKeepingProviderOrderWithin(t *testing.T) {
	land1 := goodAsset(ProviderPexels, "L1", MediaVideo, 1920, 1080)
	port1 := goodAsset(ProviderPexels, "P1", MediaVideo, 1080, 1920)
	sq := goodAsset(ProviderPexels, "S1", MediaVideo, 1500, 1500)
	land2 := goodAsset(ProviderPexels, "L2", MediaVideo, 1920, 1080)
	port2 := goodAsset(ProviderPexels, "P2", MediaVideo, 1080, 1920)
	prov := okProvider(ProviderPexels, land1, port1, sq, land2, port2)
	s := testService(t, prov)
	ctx := context.Background()

	res, _ := s.Search(ctx, ProviderPexels, SearchParams{Query: "x", Type: MediaVideo}) // portrait by default
	if got := ids(res.Assets); got != "pexels:P1,pexels:P2,pexels:S1,pexels:L1,pexels:L2" {
		t.Errorf("portrait order = %s", got)
	}
	res, _ = s.Search(ctx, ProviderPexels, SearchParams{Query: "x", Type: MediaVideo, Orientation: OrientationLandscape})
	if got := ids(res.Assets); got != "pexels:L1,pexels:L2,pexels:S1,pexels:P1,pexels:P2" {
		t.Errorf("landscape order = %s", got)
	}
	res, _ = s.Search(ctx, ProviderPexels, SearchParams{Query: "x", Type: MediaVideo, Orientation: OrientationAny})
	if got := ids(res.Assets); got != "pexels:L1,pexels:P1,pexels:S1,pexels:L2,pexels:P2" {
		t.Errorf("any order must keep the provider's order, got %s", got)
	}
}

func TestSearch_RelaxesOrientationWhenFilteredSearchIsEmpty(t *testing.T) {
	land := goodAsset(ProviderPexels, "L1", MediaVideo, 1920, 1080)
	prov := &stubProvider{name: ProviderPexels, fn: func(p SearchParams, _ int) (SearchResult, error) {
		if p.Orientation != OrientationAny {
			return SearchResult{Provider: ProviderPexels, Type: p.Type, filtered: true}, nil // nothing matches the filter
		}
		return SearchResult{Provider: ProviderPexels, Type: p.Type, Assets: []Asset{land}, Total: 1}, nil
	}}
	s := testService(t, prov)
	p := SearchParams{Query: "rare", Type: MediaVideo}

	res, err := s.Search(context.Background(), ProviderPexels, p)
	if err != nil {
		t.Fatal(err)
	}
	if ids(res.Assets) != "pexels:L1" {
		t.Fatalf("assets = %s, want the landscape clip as a fallback", ids(res.Assets))
	}
	if prov.callCount() != 2 || prov.call(1).Orientation != OrientationAny {
		t.Errorf("want one filtered call then one unfiltered, got %d calls", prov.callCount())
	}
	// The relaxed answer is cached under the original request.
	if _, err := s.Search(context.Background(), ProviderPexels, p); err != nil {
		t.Fatal(err)
	}
	if prov.callCount() != 2 {
		t.Errorf("repeating the search should hit the cache, calls = %d", prov.callCount())
	}
}

func TestSearch_NoRelaxWhenNotFilteredOrOnLaterPages(t *testing.T) {
	empty := func(filtered bool) *stubProvider {
		return &stubProvider{name: ProviderPexels, fn: func(p SearchParams, _ int) (SearchResult, error) {
			return SearchResult{Provider: ProviderPexels, Type: p.Type, filtered: filtered}, nil
		}}
	}
	ctx := context.Background()

	unfiltered := empty(false)
	testService(t, unfiltered).Search(ctx, ProviderPexels, SearchParams{Query: "x", Type: MediaVideo})
	if unfiltered.callCount() != 1 {
		t.Errorf("an unfiltered empty result is a real empty result; calls = %d", unfiltered.callCount())
	}

	page2 := empty(true)
	testService(t, page2).Search(ctx, ProviderPexels, SearchParams{Query: "x", Type: MediaVideo, Page: 2})
	if page2.callCount() != 1 {
		t.Errorf("running past the end of the filtered results must not trigger a retry; calls = %d", page2.callCount())
	}
}

func TestSearch_RelaxFailureStillReturnsTheEmptyResult(t *testing.T) {
	prov := &stubProvider{name: ProviderPexels, fn: func(p SearchParams, n int) (SearchResult, error) {
		if n == 1 {
			return SearchResult{Provider: ProviderPexels, Type: p.Type, filtered: true}, nil
		}
		return SearchResult{}, &ProviderError{Provider: ProviderPexels, Kind: KindUnavailable, Status: 503}
	}}
	res, err := testService(t, prov).Search(context.Background(), ProviderPexels, SearchParams{Query: "x", Type: MediaVideo})
	if err != nil || len(res.Assets) != 0 {
		t.Errorf("res=%v err=%v; the retry failing should leave the first answer, not turn it into an error", ids(res.Assets), err)
	}
}

func TestRateLimit_StartsCooldownAndStopsCalling(t *testing.T) {
	clock := newFakeClock()
	prov := failProvider(ProviderPixabay, rateLimitErr(ProviderPixabay, 37*time.Second))
	s := serviceWithClock(t, clock, prov)
	p := SearchParams{Query: "x", Type: MediaPhoto}
	ctx := context.Background()

	_, err := s.Search(ctx, ProviderPixabay, p)
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("first err = %v", err)
	}
	if prov.callCount() != 1 {
		t.Fatalf("calls = %d", prov.callCount())
	}

	clock.Advance(10 * time.Second)
	_, err = s.Search(ctx, ProviderPixabay, p)
	var pe *ProviderError
	if !errors.As(err, &pe) || !errors.Is(err, ErrRateLimited) || !errors.Is(err, ErrCoolingDown) {
		t.Fatalf("during cooldown err = %v, want rate-limited and cooling down", err)
	}
	if prov.callCount() != 1 {
		t.Errorf("the provider was called during its cooldown (%d calls)", prov.callCount())
	}
	if pe.RetryAfter != 27*time.Second {
		t.Errorf("RetryAfter = %v, want the 27s that remain", pe.RetryAfter)
	}
	// A search for a different query is blocked too: the limit is per key.
	if _, err := s.Search(ctx, ProviderPixabay, SearchParams{Query: "other", Type: MediaPhoto}); !errors.Is(err, ErrCoolingDown) {
		t.Errorf("another query during cooldown: err = %v", err)
	}

	clock.Advance(28 * time.Second) // past the 37s
	if _, err := s.Search(ctx, ProviderPixabay, p); !errors.Is(err, ErrRateLimited) || errors.Is(err, ErrCoolingDown) {
		t.Fatalf("after cooldown the provider should be called again (and may refuse again): %v", err)
	}
	if prov.callCount() != 2 {
		t.Errorf("calls = %d, want 2", prov.callCount())
	}
}

func TestRateLimit_DefaultAndCappedCooldowns(t *testing.T) {
	for _, tc := range []struct {
		name      string
		provider  ProviderName
		retry     time.Duration
		wantUntil time.Duration
	}{
		{"pixabay default", ProviderPixabay, 0, pixabayCooldown},
		{"pexels default", ProviderPexels, 0, pexelsCooldown},
		{"provider asks for more than the cap", ProviderPexels, 72 * time.Hour, maxCooldown},
	} {
		t.Run(tc.name, func(t *testing.T) {
			clock := newFakeClock()
			prov := failProvider(tc.provider, rateLimitErr(tc.provider, tc.retry))
			s := serviceWithClock(t, clock, prov)
			p := SearchParams{Query: "x", Type: MediaPhoto}
			s.Search(context.Background(), tc.provider, p)

			clock.Advance(tc.wantUntil - time.Second)
			if _, err := s.Search(context.Background(), tc.provider, p); !errors.Is(err, ErrCoolingDown) {
				t.Errorf("1s before the cooldown ends: err = %v, want still cooling down", err)
			}
			clock.Advance(2 * time.Second)
			if _, err := s.Search(context.Background(), tc.provider, p); errors.Is(err, ErrCoolingDown) {
				t.Errorf("1s after the cooldown ends the provider should be called again")
			}
		})
	}
}

func TestRateLimit_OtherErrorsDoNotCoolDown(t *testing.T) {
	prov := failProvider(ProviderPexels, &ProviderError{Provider: ProviderPexels, Kind: KindUnavailable, Status: 503})
	s := testService(t, prov)
	p := SearchParams{Query: "x", Type: MediaPhoto}
	for i := 0; i < 3; i++ {
		s.Search(context.Background(), ProviderPexels, p)
	}
	if prov.callCount() != 3 {
		t.Errorf("calls = %d; an outage is not a rate limit and must not start a cooldown", prov.callCount())
	}
}

func TestRateLimit_ExhaustedQuotaStartsCooldownBeforeA429(t *testing.T) {
	clock := newFakeClock()
	reset := clock.Now().Add(45 * time.Second)
	prov := &stubProvider{name: ProviderPixabay, fn: func(p SearchParams, _ int) (SearchResult, error) {
		return SearchResult{Provider: ProviderPixabay, Type: p.Type, Quota: &Quota{Remaining: 0, ResetAt: reset}}, nil
	}}
	s := serviceWithClock(t, clock, prov)
	ctx := context.Background()
	if _, err := s.Search(ctx, ProviderPixabay, SearchParams{Query: "a", Type: MediaPhoto}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Search(ctx, ProviderPixabay, SearchParams{Query: "b", Type: MediaPhoto}); !errors.Is(err, ErrCoolingDown) {
		t.Errorf("with no quota left the next call should wait, err = %v", err)
	}
	if prov.callCount() != 1 {
		t.Errorf("calls = %d, want 1", prov.callCount())
	}
	// Cached answers are still served while cooling down.
	if res, err := s.Search(ctx, ProviderPixabay, SearchParams{Query: "a", Type: MediaPhoto}); err != nil || !res.Cached {
		t.Errorf("cache must still answer during a cooldown: err=%v cached=%v", err, res.Cached)
	}
}

func TestSearchAll_OneProviderFailing(t *testing.T) {
	pexels := okProvider(ProviderPexels, goodAsset(ProviderPexels, "1", MediaVideo, 1080, 1920))
	pixabay := failProvider(ProviderPixabay, &ProviderError{Provider: ProviderPixabay, Kind: KindUnavailable, Status: 502})
	s := testService(t, pexels, pixabay)

	m, err := s.SearchAll(context.Background(), SearchParams{Query: "x", Type: MediaVideo})
	if err != nil {
		t.Fatalf("one working provider is enough: %v", err)
	}
	if ids(m.Assets()) != "pexels:1" || len(m.Results) != 1 {
		t.Errorf("assets = %s, results = %d", ids(m.Assets()), len(m.Results))
	}
	if len(m.Failures) != 1 || m.Failures[0].Provider != ProviderPixabay || !errors.Is(m.Failures[0].Err, ErrUnavailable) {
		t.Errorf("failures = %+v", m.Failures)
	}
	if got := m.Attributions(); len(got) != 1 || got[0].Text != "pexels credit" {
		t.Errorf("only providers that answered are credited: %+v", got)
	}
}

func TestSearchAll_FallsBackWhenOneIsRateLimited(t *testing.T) {
	clock := newFakeClock()
	pexels := failProvider(ProviderPexels, rateLimitErr(ProviderPexels, 0))
	pixabay := okProvider(ProviderPixabay, goodAsset(ProviderPixabay, "9", MediaPhoto, 2000, 3000))
	s := serviceWithClock(t, clock, pexels, pixabay)
	p := SearchParams{Query: "x", Type: MediaPhoto}

	m, err := s.SearchAll(context.Background(), p)
	if err != nil || ids(m.Assets()) != "pixabay:9" {
		t.Fatalf("res=%s err=%v", ids(m.Assets()), err)
	}
	// While Pexels cools down it is not called, and Pixabay keeps answering.
	other := SearchParams{Query: "y", Type: MediaPhoto}
	if _, err := s.SearchAll(context.Background(), other); err != nil {
		t.Fatal(err)
	}
	if pexels.callCount() != 1 {
		t.Errorf("pexels called %d times, want 1 (cooldown)", pexels.callCount())
	}
}

func TestSearchAll_AllProvidersFail(t *testing.T) {
	s := testService(t,
		failProvider(ProviderPexels, rateLimitErr(ProviderPexels, 0)),
		failProvider(ProviderPixabay, &ProviderError{Provider: ProviderPixabay, Kind: KindAuth, Status: 401}))
	m, err := s.SearchAll(context.Background(), SearchParams{Query: "x", Type: MediaPhoto})
	if !errors.Is(err, ErrAllProvidersFailed) || !errors.Is(err, ErrRateLimited) || !errors.Is(err, ErrAuth) {
		t.Errorf("err = %v, want ErrAllProvidersFailed wrapping both causes", err)
	}
	if len(m.Failures) != 2 || len(m.Results) != 0 {
		t.Errorf("failures=%d results=%d", len(m.Failures), len(m.Results))
	}
}

func TestSearchAll_EmptyResultsAreNotFailure(t *testing.T) {
	s := testService(t, okProvider(ProviderPexels), okProvider(ProviderPixabay))
	m, err := s.SearchAll(context.Background(), SearchParams{Query: "x", Type: MediaPhoto, Orientation: OrientationAny})
	if err != nil || len(m.Assets()) != 0 || len(m.Failures) != 0 {
		t.Errorf("err=%v assets=%d failures=%d", err, len(m.Assets()), len(m.Failures))
	}
}

func TestSearchAll_InterleavesProviders(t *testing.T) {
	a := func(p ProviderName, id string) Asset { return goodAsset(p, id, MediaPhoto, 2000, 3000) }
	s := testService(t,
		okProvider(ProviderPexels, a(ProviderPexels, "a1"), a(ProviderPexels, "a2"), a(ProviderPexels, "a3")),
		okProvider(ProviderPixabay, a(ProviderPixabay, "b1")))
	m, err := s.SearchAll(context.Background(), SearchParams{Query: "x", Type: MediaPhoto})
	if err != nil {
		t.Fatal(err)
	}
	if got := ids(m.Assets()); got != "pexels:a1,pixabay:b1,pexels:a2,pexels:a3" {
		t.Errorf("order = %s", got)
	}
}

func TestSearchAll_QueriesProvidersConcurrently(t *testing.T) {
	var mu sync.Mutex
	inFlight, peak := 0, 0
	slow := func(name ProviderName) *stubProvider {
		return &stubProvider{name: name, fn: func(p SearchParams, _ int) (SearchResult, error) {
			mu.Lock()
			inFlight++
			peak = max(peak, inFlight)
			mu.Unlock()
			time.Sleep(60 * time.Millisecond)
			mu.Lock()
			inFlight--
			mu.Unlock()
			return SearchResult{Provider: name, Type: p.Type}, nil
		}}
	}
	s := testService(t, slow(ProviderPexels), slow(ProviderPixabay))
	if _, err := s.SearchAll(context.Background(), SearchParams{Query: "x", Type: MediaPhoto, Orientation: OrientationAny}); err != nil {
		t.Fatal(err)
	}
	if peak < 2 {
		t.Errorf("peak concurrency = %d, want both providers in flight at once", peak)
	}
}

func TestSearchAll_ContextCancelled(t *testing.T) {
	s := testService(t, okProvider(ProviderPexels), okProvider(ProviderPixabay))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := s.SearchAll(ctx, SearchParams{Query: "x", Type: MediaPhoto})
	if !errors.Is(err, context.Canceled) || errors.Is(err, ErrAllProvidersFailed) {
		t.Errorf("err = %v, want context.Canceled and not a provider failure", err)
	}
}

func TestService_RaceFree(t *testing.T) {
	// Run with -race: many goroutines searching, hitting the cache, and
	// triggering cooldowns on the same service.
	clock := newFakeClock()
	var n int
	var mu sync.Mutex
	prov := &stubProvider{name: ProviderPexels, fn: func(p SearchParams, _ int) (SearchResult, error) {
		mu.Lock()
		n++
		k := n
		mu.Unlock()
		if k%7 == 0 {
			return SearchResult{}, rateLimitErr(ProviderPexels, time.Second)
		}
		return SearchResult{Provider: ProviderPexels, Type: p.Type, Assets: []Asset{goodAsset(ProviderPexels, "1", MediaPhoto, 2000, 3000)}}, nil
	}}
	s := serviceWithClock(t, clock, prov, okProvider(ProviderPixabay, goodAsset(ProviderPixabay, "1", MediaPhoto, 2000, 3000)))
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				s.SearchAll(context.Background(), SearchParams{Query: fmt.Sprintf("q%d", (i+j)%5), Type: MediaPhoto})
				clock.Advance(100 * time.Millisecond)
			}
		}()
	}
	wg.Wait()
}
