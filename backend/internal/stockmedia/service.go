package stockmedia

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Cache lifetimes. Pixabay's API terms require requests to be cached for 24
// hours, so that is a floor, not a tuning knob. Pexels sets no cache rule, so
// its results are kept for an hour: long enough to absorb a user paging and
// swapping assets while a plan is edited, short enough that its limit of 200
// requests an hour is not the only thing standing between us and stale data.
const (
	pixabayCacheTTL = 24 * time.Hour
	pexelsCacheTTL  = time.Hour
)

// Cooldowns applied after a rate-limit response that did not say how long to
// wait. Pixabay's window is 60 seconds; Pexels counts hourly and monthly, so
// it is given longer. maxCooldown caps what a provider can ask for, because a
// monthly quota can be exhausted for weeks and a service that stays blocked
// for weeks cannot notice the quota coming back.
const (
	pixabayCooldown = time.Minute
	pexelsCooldown  = 10 * time.Minute
	maxCooldown     = time.Hour
)

// Service searches one or more stock providers. It is safe for concurrent use.
type Service struct {
	providers   []Provider
	byName      map[ProviderName]Provider
	cache       Cache
	ttl         map[ProviderName]time.Duration
	policy      URLPolicy
	assetClient HTTPDoer
	// assetTransport is the transport the default asset client uses; tests set
	// it to reach a TLS test server.
	assetTransport http.RoundTripper
	minLongSide    int
	logf           func(format string, args ...any)
	now            func() time.Time
	retry          RetryPolicy
	// sleep waits between retries. Nil means really wait; tests replace it to
	// move a fake clock instead.
	sleep func(context.Context, time.Duration) error

	mu       sync.Mutex
	cooldown map[ProviderName]time.Time
}

// Option customizes a Service.
type Option func(*Service)

// WithCache replaces the default in-memory cache.
func WithCache(c Cache) Option { return func(s *Service) { s.cache = c } }

// WithURLPolicy replaces the host allowlist applied to asset URLs. Tests use
// it to allow a local server; production code should not.
func WithURLPolicy(p URLPolicy) Option { return func(s *Service) { s.policy = p } }

// WithMinLongSide replaces MinLongSide, the smallest longer side an asset may
// have.
func WithMinLongSide(px int) Option { return func(s *Service) { s.minLongSide = px } }

// WithLogger replaces log.Printf as the destination for service logs.
func WithLogger(f func(format string, args ...any)) Option { return func(s *Service) { s.logf = f } }

// WithClock replaces time.Now, for tests.
func WithClock(now func() time.Time) Option { return func(s *Service) { s.now = now } }

// WithRetry replaces DefaultRetryPolicy, which applies to Search and Get. Use
// BatchRetryPolicy for background work. The zero RetryPolicy turns retries off.
func WithRetry(p RetryPolicy) Option { return func(s *Service) { s.retry = p } }

// NewService returns a Service over the given providers. At least one is
// required; the order given is the order results from several providers are
// interleaved in.
func NewService(providers []Provider, opts ...Option) (*Service, error) {
	if len(providers) == 0 {
		return nil, fmt.Errorf("%w: no providers", ErrNotConfigured)
	}
	s := &Service{
		byName:      make(map[ProviderName]Provider, len(providers)),
		cache:       NewMemoryCache(DefaultCacheEntries),
		ttl:         map[ProviderName]time.Duration{ProviderPixabay: pixabayCacheTTL, ProviderPexels: pexelsCacheTTL},
		policy:      defaultURLPolicy,
		minLongSide: MinLongSide,
		logf:        log.Printf,
		now:         time.Now,
		retry:       DefaultRetryPolicy(),
		cooldown:    make(map[ProviderName]time.Time),
	}
	for _, p := range providers {
		if p == nil {
			return nil, fmt.Errorf("%w: nil provider", ErrNotConfigured)
		}
		if _, dup := s.byName[p.Name()]; dup {
			return nil, fmt.Errorf("stockmedia: provider %q registered twice", p.Name())
		}
		s.byName[p.Name()] = p
		s.providers = append(s.providers, p)
	}
	for _, o := range opts {
		o(s)
	}
	// Built after the options so it sees a replaced URL policy.
	if s.assetClient == nil {
		s.assetClient = s.assetHTTPClient()
	}
	return s, nil
}

// Providers lists the registered providers in registration order.
func (s *Service) Providers() []ProviderName {
	names := make([]ProviderName, len(s.providers))
	for i, p := range s.providers {
		names[i] = p.Name()
	}
	return names
}

// Search queries one provider.
//
// Parameters are validated first, so a bad query fails the same way whether
// the provider is cached, cooling down or reachable. The result holds only
// assets that passed validation. They are ordered best orientation match
// first; a provider's own relevance order is kept within one orientation.
//
// When a provider is asked for a particular orientation and has nothing
// matching on the first page, the search is repeated once without the filter
// and the results are ordered by closeness instead, so a scene gets a
// landscape clip to crop before it gets no media at all.
func (s *Service) Search(ctx context.Context, name ProviderName, p SearchParams) (SearchResult, error) {
	prov, ok := s.byName[name]
	if !ok {
		return SearchResult{}, fmt.Errorf("%w: %q", ErrNotConfigured, name)
	}
	p, err := normalizeParams(p)
	if err != nil {
		return SearchResult{}, err
	}
	return s.searchOne(ctx, prov, p, true)
}

func (s *Service) searchOne(ctx context.Context, prov Provider, p SearchParams, allowRelax bool) (SearchResult, error) {
	if err := ctx.Err(); err != nil {
		return SearchResult{}, err
	}
	name := prov.Name()
	key := cacheKey(name, p)
	if hit, ok := s.cache.Get(key); ok {
		hit.Cached = true
		s.logf("stockmedia_search provider=%s type=%s page=%d results=%d cached=true", name, p.Type, p.Page, len(hit.Assets))
		return hit, nil
	}
	var res SearchResult
	err := s.callProvider(ctx, name, "search", func(ctx context.Context) error {
		var err error
		res, err = prov.Search(ctx, p)
		if res.Quota != nil {
			s.noteQuota(name, res.Quota)
		}
		return err
	})
	if err != nil {
		return SearchResult{}, err
	}

	s.finish(&res, p)
	s.logf("stockmedia_search provider=%s type=%s page=%d queryLen=%d orientation=%s results=%d skipped=%d cached=false%s",
		name, p.Type, p.Page, len([]rune(p.Query)), p.Orientation, len(res.Assets), res.Skipped, s.quotaFields(res.Quota))

	if allowRelax && res.filtered && len(res.Assets) == 0 && p.Page == 1 {
		relaxed := p
		relaxed.Orientation = OrientationAny
		if alt, err := s.searchOne(ctx, prov, relaxed, false); err == nil && len(alt.Assets) > 0 {
			sortByOrientation(alt.Assets, p.Orientation)
			alt.Cached = false
			s.cache.Set(key, alt, s.ttl[name])
			s.seedAssets(name, alt.Assets)
			return alt, nil
		} else if err != nil && ctx.Err() != nil {
			return SearchResult{}, ctx.Err()
		}
	}

	s.cache.Set(key, res, s.ttl[name])
	s.seedAssets(name, res.Assets)
	return res, nil
}

// seedAssets stores each validated search result as a lookup result too. A
// search and a lookup decode the same fields with the same code, so what a
// search returned is what Get would return. Seeding means a user who picks an
// asset they have just seen costs no provider call, which matters because the
// providers' limits are shared by every user of the site.
func (s *Service) seedAssets(name ProviderName, assets []Asset) {
	for _, a := range assets {
		s.cache.Set(assetCacheKey(name, a.Type, a.ID), SearchResult{Assets: []Asset{a}}, assetCacheTTL(name))
	}
}

func assetCacheKey(name ProviderName, typ MediaType, id string) string {
	return fmt.Sprintf("get|%s|%s|%s", name, typ, id)
}

// finish validates and orders a provider's assets in place.
func (s *Service) finish(res *SearchResult, p SearchParams) {
	valid := res.Assets[:0:0]
	for _, a := range res.Assets {
		if err := validateAsset(a, s.policy, s.minLongSide); err != nil {
			res.Skipped++
			var ia *InvalidAssetError
			reason := "invalid"
			if errors.As(err, &ia) {
				reason = ia.Reason
			}
			if reason == "host" {
				// Log the host so a provider moving to a new CDN is easy to
				// spot and fix in providerHosts. It stays out of the error.
				s.logf("stockmedia_asset_rejected provider=%s asset=%s reason=host host=%q", a.Provider, a.ID, hostOf(a.MediaURL))
			}
			if res.SkipReasons == nil {
				res.SkipReasons = make(map[string]int)
			}
			res.SkipReasons[reason]++
			continue
		}
		valid = append(valid, a)
	}
	res.Assets = valid
	sortByOrientation(res.Assets, p.Orientation)
}

// hostOf returns the hostname of a URL, or "" when it cannot be parsed.
func hostOf(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return u.Hostname()
}

func cacheKey(name ProviderName, p SearchParams) string {
	return fmt.Sprintf("%s|%s|%s|%d|%d|%s", name, p.Type, p.Orientation, p.Page, p.PerPage, strings.ToLower(p.Query))
}

// MultiResult is the outcome of searching several providers.
type MultiResult struct {
	// Results holds one entry per provider that answered, in registration order.
	Results []SearchResult
	// Failures holds one entry per provider that did not.
	Failures []ProviderFailure
}

// Assets returns every asset from every provider, interleaved one provider at
// a time so the first results mix sources instead of listing one provider's
// matches ahead of the other's.
func (m MultiResult) Assets() []Asset {
	var out []Asset
	for i := 0; ; i++ {
		added := false
		for _, r := range m.Results {
			if i < len(r.Assets) {
				out = append(out, r.Assets[i])
				added = true
			}
		}
		if !added {
			return out
		}
	}
}

// Attributions lists the credit to show, one per provider that answered.
func (m MultiResult) Attributions() []Attribution {
	out := make([]Attribution, 0, len(m.Results))
	for _, r := range m.Results {
		out = append(out, r.Attribution)
	}
	return out
}

// SearchAll queries every registered provider concurrently.
//
// A provider that fails does not fail the search: its error is recorded in
// Failures and the other provider's results are returned. Only when every
// provider fails is an error returned, wrapping ErrAllProvidersFailed along
// with each provider's own error.
func (s *Service) SearchAll(ctx context.Context, p SearchParams) (MultiResult, error) {
	p, err := normalizeParams(p)
	if err != nil {
		return MultiResult{}, err
	}

	type outcome struct {
		res SearchResult
		err error
	}
	out := make([]outcome, len(s.providers))
	var wg sync.WaitGroup
	for i, prov := range s.providers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, err := s.searchOne(ctx, prov, p, true)
			out[i] = outcome{res, err}
		}()
	}
	wg.Wait()

	if err := ctx.Err(); err != nil {
		return MultiResult{}, err
	}

	var m MultiResult
	var errs []error
	for i, o := range out {
		if o.err != nil {
			m.Failures = append(m.Failures, ProviderFailure{Provider: s.providers[i].Name(), Err: o.err})
			errs = append(errs, o.err)
			continue
		}
		m.Results = append(m.Results, o.res)
	}
	if len(m.Results) == 0 {
		return m, fmt.Errorf("%w: %w", ErrAllProvidersFailed, errors.Join(errs...))
	}
	return m, nil
}

// checkCooldown returns an error when the provider must not be called yet.
func (s *Service) checkCooldown(name ProviderName) error {
	s.mu.Lock()
	until := s.cooldown[name]
	s.mu.Unlock()
	if now := s.now(); now.Before(until) {
		return &ProviderError{
			Provider:   name,
			Kind:       KindRateLimited,
			RetryAfter: until.Sub(now),
			Err:        ErrCoolingDown,
		}
	}
	return nil
}

// callProvider makes one provider call, trying it again under the retry policy
// when that is safe. op names the call ("search" or "get") in logs.
//
// Each attempt first checks the provider is not cooling down, and afterwards
// records a rate-limit response or a failure. A call that fails after more than
// one attempt, or succeeds after a failure, says so in the logs, because a
// provider that needs retries is one to watch even when no user saw an error.
func (s *Service) callProvider(ctx context.Context, name ProviderName, op string, call func(ctx context.Context) error) error {
	policy := s.retry
	policy.now, policy.sleep = s.now, s.sleep
	started := s.now()
	attempts := 0
	err := policy.Do(ctx, func(actx context.Context, attempt int) error {
		attempts = attempt
		if err := s.checkCooldown(name); err != nil {
			s.logf("stockmedia_%s_skipped provider=%s reason=cooldown attempt=%d", op, name, attempt)
			return err
		}
		err := call(actx)
		if err != nil {
			// A retry is cut off by what is left of the call's time budget. That
			// is the provider being slow, not the caller giving up.
			if actx.Err() != nil && ctx.Err() == nil && errors.Is(err, actx.Err()) {
				err = &ProviderError{Provider: name, Kind: KindUnavailable, Err: errors.New("request timed out")}
			}
			s.noteError(name, op, attempt, err)
		}
		return err
	}, func(attempt int, wait time.Duration, err error) {
		s.logf("stockmedia_retry provider=%s op=%s attempt=%d wait_ms=%d%s", name, op, attempt, wait.Milliseconds(), errorFields(err))
	})
	switch {
	case err == nil && attempts > 1:
		s.logf("stockmedia_recovered provider=%s op=%s attempts=%d elapsed_ms=%d", name, op, attempts, s.now().Sub(started).Milliseconds())
	case err != nil && attempts > 1 && ctx.Err() == nil:
		s.logf("stockmedia_gave_up provider=%s op=%s attempts=%d elapsed_ms=%d%s", name, op, attempts, s.now().Sub(started).Milliseconds(), errorFields(err))
	}
	return err
}

// errorFields describes a provider failure for a log line: its kind, HTTP
// status, and how long the provider asked callers to wait. A ProviderError's
// text never holds a URL or key, and is quoted here so a provider's own words
// cannot add fields or lines to the log.
func errorFields(err error) string {
	var pe *ProviderError
	if !errors.As(err, &pe) {
		return fmt.Sprintf(" kind=other err=%q", err.Error())
	}
	return fmt.Sprintf(" kind=%s status=%d retry_after_s=%d err=%q", pe.Kind, pe.Status, int(pe.RetryAfter.Seconds()), pe.Error())
}

// quotaFields is what a provider said about its rate limit, as log fields, or
// nothing when it said nothing. Watching Remaining fall is how a quota running
// out is noticed before the first refusal.
func (s *Service) quotaFields(q *Quota) string {
	if q == nil {
		return ""
	}
	out := fmt.Sprintf(" quota_remaining=%d", q.Remaining)
	if !q.ResetAt.IsZero() {
		out += fmt.Sprintf(" quota_reset_s=%d", max(0, int(q.ResetAt.Sub(s.now()).Seconds())))
	}
	return out
}

// noteError logs a provider failure and starts a cooldown after a rate-limit
// response. Other errors start none: the provider is not asking us to slow
// down, and whether to try again is the retry policy's call.
func (s *Service) noteError(name ProviderName, op string, attempt int, err error) {
	var pe *ProviderError
	if !errors.As(err, &pe) {
		return
	}
	s.logf("stockmedia_provider_error provider=%s op=%s attempt=%d%s", name, op, attempt, errorFields(err))
	if pe.Kind != KindRateLimited {
		return
	}
	wait := pe.RetryAfter
	if wait <= 0 {
		wait = defaultCooldown(name)
	}
	s.startCooldown(name, wait)
}

// noteQuota starts a cooldown when a provider reports its quota is used up,
// so the next call is not wasted on a certain 429.
func (s *Service) noteQuota(name ProviderName, q *Quota) {
	if q.Remaining > 0 {
		return
	}
	wait := q.ResetAt.Sub(s.now())
	if wait <= 0 {
		wait = defaultCooldown(name)
	}
	s.startCooldown(name, wait)
}

func (s *Service) startCooldown(name ProviderName, wait time.Duration) {
	wait = min(wait, maxCooldown)
	until := s.now().Add(wait)
	s.mu.Lock()
	if until.After(s.cooldown[name]) {
		s.cooldown[name] = until
	}
	s.mu.Unlock()
	s.logf("stockmedia_cooldown provider=%s seconds=%d", name, int(wait.Seconds()))
}

func defaultCooldown(name ProviderName) time.Duration {
	if name == ProviderPexels {
		return pexelsCooldown
	}
	return pixabayCooldown
}

// assetCacheTTL is how long a looked-up asset is kept. A user editing a plan
// opens the same few assets again and again, and Pixabay's terms require
// requests to be cached for 24 hours. An asset the provider removes is
// therefore still served for up to a day; the render phase checks the file is
// really there (CheckAvailable) before it copies it.
func assetCacheTTL(name ProviderName) time.Duration {
	if name == ProviderPexels {
		return pexelsCacheTTL
	}
	return pixabayCacheTTL
}

// Get fetches one asset by provider and id, for the places a plan refers to an
// asset it chose earlier. A plan stores only the provider, the type and the id,
// never a URL, because Pixabay forbids permanent hotlinking and Pexels video
// links are signed; Get returns fresh URLs.
//
// The asset is validated like a search result. One that fails validation is
// reported as ErrNotFound, because to the caller it is not usable, and the
// reason is logged. Lookups share the search cache, so repeated reads of the
// same asset cost no provider calls, and a provider in a rate-limit cooldown is
// not called.
func (s *Service) Get(ctx context.Context, name ProviderName, typ MediaType, id string) (Asset, error) {
	prov, ok := s.byName[name]
	if !ok {
		return Asset{}, fmt.Errorf("%w: %q", ErrNotConfigured, name)
	}
	if typ != MediaPhoto && typ != MediaVideo {
		return Asset{}, fmt.Errorf("%w: type must be %q or %q", ErrInvalidParams, MediaPhoto, MediaVideo)
	}
	if err := validateAssetID(id); err != nil {
		return Asset{}, err
	}
	if err := ctx.Err(); err != nil {
		return Asset{}, err
	}

	key := assetCacheKey(name, typ, id)
	if hit, ok := s.cache.Get(key); ok && len(hit.Assets) == 1 {
		return hit.Assets[0], nil
	}
	var a Asset
	err := s.callProvider(ctx, name, "get", func(ctx context.Context) error {
		var err error
		a, err = prov.Get(ctx, typ, id)
		return err
	})
	if err != nil {
		return Asset{}, err
	}
	if verr := validateAsset(a, s.policy, s.minLongSide); verr != nil {
		var ia *InvalidAssetError
		reason := "invalid"
		if errors.As(verr, &ia) {
			reason = ia.Reason
		}
		s.logf("stockmedia_get_rejected provider=%s type=%s asset=%s reason=%s", name, typ, id, reason)
		return Asset{}, &ProviderError{Provider: name, Kind: KindNotFound, Err: fmt.Errorf("the asset cannot be used (%s)", reason)}
	}
	s.logf("stockmedia_get provider=%s type=%s cached=false", name, typ)
	s.cache.Set(key, SearchResult{Assets: []Asset{a}}, assetCacheTTL(name))
	return a, nil
}

// AssetRef names one asset without any URL: all a plan stores about a choice.
type AssetRef struct {
	Provider ProviderName `json:"provider"`
	Type     MediaType    `json:"type"`
	ID       string       `json:"id"`
}

// Valid reports whether the reference is well formed.
func (r AssetRef) Valid() bool {
	return (r.Provider == ProviderPexels || r.Provider == ProviderPixabay) &&
		(r.Type == MediaPhoto || r.Type == MediaVideo) && ValidAssetID(r.ID)
}

// AssetResult is the outcome of looking up one reference.
type AssetResult struct {
	Ref   AssetRef
	Asset *Asset // set on success
	Err   error  // set on failure; Asset is nil
}

// GetMany looks up several references concurrently and returns the outcomes in
// the order given. One failing lookup does not affect the others, so a plan with
// one removed asset still opens. At most maxConcurrentGets run at once.
func (s *Service) GetMany(ctx context.Context, refs []AssetRef) []AssetResult {
	out := make([]AssetResult, len(refs))
	sem := make(chan struct{}, maxConcurrentGets)
	var wg sync.WaitGroup
	for i, ref := range refs {
		out[i].Ref = ref
		if !ref.Valid() {
			out[i].Err = fmt.Errorf("%w: malformed asset reference", ErrInvalidParams)
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				out[i].Err = ctx.Err()
				return
			}
			a, err := s.Get(ctx, ref.Provider, ref.Type, ref.ID)
			if err != nil {
				out[i].Err = err
				return
			}
			out[i].Asset = &a
		}()
	}
	wg.Wait()
	return out
}

// maxConcurrentGets bounds parallel lookups. A plan has at most ten scenes, and
// the providers' limits are per key, so a few at a time is plenty.
const maxConcurrentGets = 4
