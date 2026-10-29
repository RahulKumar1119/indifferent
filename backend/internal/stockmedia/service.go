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
	if err := s.checkCooldown(name); err != nil {
		s.logf("stockmedia_search_skipped provider=%s reason=cooldown", name)
		return SearchResult{}, err
	}

	res, err := prov.Search(ctx, p)
	if res.Quota != nil {
		s.noteQuota(name, res.Quota)
	}
	if err != nil {
		s.noteError(name, err)
		return SearchResult{}, err
	}

	s.finish(&res, p)
	s.logf("stockmedia_search provider=%s type=%s page=%d queryLen=%d orientation=%s results=%d skipped=%d cached=false",
		name, p.Type, p.Page, len([]rune(p.Query)), p.Orientation, len(res.Assets), res.Skipped)

	if allowRelax && res.filtered && len(res.Assets) == 0 && p.Page == 1 {
		relaxed := p
		relaxed.Orientation = OrientationAny
		if alt, err := s.searchOne(ctx, prov, relaxed, false); err == nil && len(alt.Assets) > 0 {
			sortByOrientation(alt.Assets, p.Orientation)
			alt.Cached = false
			s.cache.Set(key, alt, s.ttl[name])
			return alt, nil
		} else if err != nil && ctx.Err() != nil {
			return SearchResult{}, ctx.Err()
		}
	}

	s.cache.Set(key, res, s.ttl[name])
	return res, nil
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

// noteError starts a cooldown after a rate-limit response. Other errors do
// not: an outage is retried by the caller, and the provider is not being
// asked to slow down.
func (s *Service) noteError(name ProviderName, err error) {
	var pe *ProviderError
	if !errors.As(err, &pe) {
		return
	}
	s.logf("stockmedia_provider_error provider=%s kind=%s status=%d", name, pe.Kind, pe.Status)
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
