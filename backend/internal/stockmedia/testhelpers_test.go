package stockmedia

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	testPexelsKey  = "pexels-test-key-AAAA1111"
	testPixabayKey = "pixabay-test-key-BBBB2222"
)

// allowAnyHost is a URLPolicy that accepts every URL, for tests whose fixtures
// point at example hosts. Tests of the real policy use defaultURLPolicy.
func allowAnyHost(ProviderName, *url.URL) error { return nil }

// testService builds a Service over the given providers with logging off, the
// host policy relaxed and retries off, so a test that counts provider calls
// counts attempts. Tests of retrying use retryService.
func testService(t *testing.T, providers ...Provider) *Service {
	t.Helper()
	s, err := NewService(providers, WithLogger(func(string, ...any) {}), WithURLPolicy(allowAnyHost), WithRetry(RetryPolicy{}))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// recorder is an HTTP test server that records requests and serves
// programmable responses.
type recorder struct {
	*httptest.Server
	mu       sync.Mutex
	requests []*http.Request
	bodies   []string
	handler  func(w http.ResponseWriter, r *http.Request, n int)
}

func newRecorder(t *testing.T, h func(w http.ResponseWriter, r *http.Request, n int)) *recorder {
	t.Helper()
	rec := &recorder{handler: h}
	rec.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.mu.Lock()
		rec.requests = append(rec.requests, r.Clone(r.Context()))
		n := len(rec.requests)
		rec.mu.Unlock()
		rec.handler(w, r, n)
	}))
	t.Cleanup(rec.Close)
	return rec
}

func (r *recorder) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.requests)
}

func (r *recorder) request(i int) *http.Request {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.requests[i]
}

// respondJSON writes body with status 200.
func respondJSON(w http.ResponseWriter, body string) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = io.WriteString(w, body)
}

// jsonBody is a shorthand for building fixtures from Go values.
func jsonBody(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// fakeClock is a manually advanced clock.
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func newFakeClock() *fakeClock { return &fakeClock{t: time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)} }

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

// stubProvider is a Provider whose answers are set by the test.
type stubProvider struct {
	name  ProviderName
	mu    sync.Mutex
	calls []SearchParams
	fn    func(p SearchParams, n int) (SearchResult, error)

	// getFn answers Get; nil means every lookup is ErrNotFound.
	getFn    func(typ MediaType, id string, n int) (Asset, error)
	getCalls int
}

func (s *stubProvider) Get(_ context.Context, typ MediaType, id string) (Asset, error) {
	s.mu.Lock()
	s.getCalls++
	n, fn := s.getCalls, s.getFn
	s.mu.Unlock()
	if fn == nil {
		return Asset{}, &ProviderError{Provider: s.name, Kind: KindNotFound, Status: 404}
	}
	return fn(typ, id, n)
}

func (s *stubProvider) getCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.getCalls
}

func (s *stubProvider) Name() ProviderName { return s.name }

func (s *stubProvider) Search(_ context.Context, p SearchParams) (SearchResult, error) {
	s.mu.Lock()
	s.calls = append(s.calls, p)
	n := len(s.calls)
	s.mu.Unlock()
	return s.fn(p, n)
}

func (s *stubProvider) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.calls)
}

func (s *stubProvider) call(i int) SearchParams {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls[i]
}

// goodAsset returns an asset that passes validation under allowAnyHost.
func goodAsset(p ProviderName, id string, typ MediaType, w, h int) Asset {
	a := Asset{
		Provider:    p,
		ID:          id,
		Type:        typ,
		PageURL:     fmt.Sprintf("https://%s.test/page/%s", p, id),
		MediaURL:    fmt.Sprintf("https://%s.test/file/%s", p, id),
		Width:       w,
		Height:      h,
		Orientation: orientationOf(w, h),
		Author:      "Author " + id,
		Credit:      "Credit " + id,
	}
	if typ == MediaVideo {
		a.Format, a.Duration = "video/mp4", 10
	} else {
		a.Format = "image/jpeg"
	}
	return a
}

func jsonMarshal(v any) ([]byte, error) { return json.Marshal(v) }

func sprintf(format string, args ...any) string { return fmt.Sprintf(format, args...) }

func ids(assets []Asset) string {
	out := make([]string, len(assets))
	for i, a := range assets {
		out[i] = string(a.Provider) + ":" + a.ID
	}
	return strings.Join(out, ",")
}
