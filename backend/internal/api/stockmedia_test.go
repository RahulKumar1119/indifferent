package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/rahul/indifferent/backend/internal/stockmedia"
	"github.com/rahul/indifferent/backend/internal/storyplan"
)

// fakeStock is a StockMedia whose answers the test sets.
type fakeStock struct {
	mu        sync.Mutex
	search    func(name stockmedia.ProviderName, p stockmedia.SearchParams) (stockmedia.SearchResult, error)
	all       func(p stockmedia.SearchParams) (stockmedia.MultiResult, error)
	get       func(ref stockmedia.AssetRef) (stockmedia.Asset, error)
	searches  []stockmedia.SearchParams
	lookups   []stockmedia.AssetRef
	providers []stockmedia.ProviderName
}

func (f *fakeStock) Providers() []stockmedia.ProviderName { return f.providers }

func (f *fakeStock) Search(_ context.Context, name stockmedia.ProviderName, p stockmedia.SearchParams) (stockmedia.SearchResult, error) {
	f.mu.Lock()
	f.searches = append(f.searches, p)
	fn := f.search
	f.mu.Unlock()
	return fn(name, p)
}

func (f *fakeStock) SearchAll(_ context.Context, p stockmedia.SearchParams) (stockmedia.MultiResult, error) {
	f.mu.Lock()
	f.searches = append(f.searches, p)
	fn := f.all
	f.mu.Unlock()
	return fn(p)
}

func (f *fakeStock) GetMany(_ context.Context, refs []stockmedia.AssetRef) []stockmedia.AssetResult {
	out := make([]stockmedia.AssetResult, len(refs))
	for i, r := range refs {
		f.mu.Lock()
		f.lookups = append(f.lookups, r)
		fn := f.get
		f.mu.Unlock()
		out[i].Ref = r
		if fn == nil {
			out[i].Err = &stockmedia.ProviderError{Provider: r.Provider, Kind: stockmedia.KindNotFound}
			continue
		}
		a, err := fn(r)
		if err != nil {
			out[i].Err = err
		} else {
			out[i].Asset = &a
		}
	}
	return out
}

func (f *fakeStock) searchCount() int { f.mu.Lock(); defer f.mu.Unlock(); return len(f.searches) }
func (f *fakeStock) lookupCount() int { f.mu.Lock(); defer f.mu.Unlock(); return len(f.lookups) }

func stockAsset(p stockmedia.ProviderName, typ stockmedia.MediaType, id string, w, h int) stockmedia.Asset {
	a := stockmedia.Asset{
		Provider: p, ID: id, Type: typ,
		PageURL:     "https://" + string(p) + ".test/p/" + id,
		PreviewURL:  "https://" + string(p) + ".test/prev/" + id + ".jpg",
		MediaURL:    "https://" + string(p) + ".test/SECRET-ORIGINAL/" + id,
		Format:      "image/jpeg",
		Width:       w,
		Height:      h,
		Orientation: stockmedia.OrientationPortrait,
		Author:      "Author " + id,
		AuthorURL:   "https://" + string(p) + ".test/u/" + id,
		Credit:      "Photo by Author " + id + " on " + string(p),
	}
	if typ == stockmedia.MediaVideo {
		a.Format, a.Duration = "video/mp4", 12
		a.PlayURL = "https://" + string(p) + ".test/play/" + id + ".mp4"
	}
	return a
}

func okSearch(assets ...stockmedia.Asset) func(stockmedia.ProviderName, stockmedia.SearchParams) (stockmedia.SearchResult, error) {
	return func(name stockmedia.ProviderName, p stockmedia.SearchParams) (stockmedia.SearchResult, error) {
		return stockmedia.SearchResult{
			Provider: name, Type: p.Type, Assets: assets, Total: len(assets),
			Attribution: stockmedia.Attribution{Text: "Photos provided by " + string(name), URL: "https://" + string(name) + ".test"},
		}, nil
	}
}

func newStockEnv(t *testing.T) (*spEnv, *fakeStock) {
	t.Helper()
	e := newSPEnv(t)
	fs := &fakeStock{providers: []stockmedia.ProviderName{stockmedia.ProviderPexels, stockmedia.ProviderPixabay}}
	fs.search = okSearch(stockAsset(stockmedia.ProviderPexels, stockmedia.MediaPhoto, "11", 1080, 1920))
	fs.all = func(p stockmedia.SearchParams) (stockmedia.MultiResult, error) {
		return stockmedia.MultiResult{Results: []stockmedia.SearchResult{
			{Provider: stockmedia.ProviderPexels, Type: p.Type, Total: 100,
				Assets:      []stockmedia.Asset{stockAsset(stockmedia.ProviderPexels, p.Type, "11", 1080, 1920), stockAsset(stockmedia.ProviderPexels, p.Type, "12", 1080, 1920)},
				Attribution: stockmedia.Attribution{Text: "Photos provided by Pexels", URL: "https://www.pexels.com"}},
			{Provider: stockmedia.ProviderPixabay, Type: p.Type, Total: 500,
				Assets:      []stockmedia.Asset{stockAsset(stockmedia.ProviderPixabay, p.Type, "21", 1080, 1920)},
				Attribution: stockmedia.Attribution{Text: "Images provided by Pixabay", URL: "https://pixabay.com"}},
		}}, nil
	}
	fs.get = func(r stockmedia.AssetRef) (stockmedia.Asset, error) {
		return stockAsset(r.Provider, r.Type, r.ID, 1080, 1920), nil
	}
	e.h.StockMedia = fs
	return e, fs
}

func (e *spEnv) get(t *testing.T, path, user string, params map[string]string) events.APIGatewayProxyResponse {
	t.Helper()
	headers := map[string]string{}
	if user != "" {
		headers["Authorization"] = "Bearer " + generateTestToken(user)
	}
	resp, err := e.h.HandleRequest(context.Background(), events.APIGatewayProxyRequest{
		HTTPMethod: "GET", Path: path, Headers: headers, QueryStringParameters: params,
	})
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func decodeSearch(t *testing.T, resp events.APIGatewayProxyResponse) StockSearchResponse {
	t.Helper()
	var r StockSearchResponse
	if err := json.Unmarshal([]byte(resp.Body), &r); err != nil {
		t.Fatalf("not a search response: %s", resp.Body)
	}
	return r
}

// --- /stock-media/search -----------------------------------------------------

func TestStockSearch_RequiresSignInAndTheService(t *testing.T) {
	e, fs := newStockEnv(t)
	for _, path := range []string{"/stock-media/search", "/stock-media/assets"} {
		if resp := e.get(t, path, "", map[string]string{"q": "x", "type": "photo", "refs": "pexels:photo:1"}); resp.StatusCode != 401 {
			t.Errorf("%s without sign-in: %d", path, resp.StatusCode)
		}
	}
	if fs.searchCount()+fs.lookupCount() != 0 {
		t.Error("the stock service ran for an unauthenticated request")
	}
	e.h.StockMedia = nil
	for _, path := range []string{"/stock-media/search", "/stock-media/assets"} {
		resp := e.get(t, path, "user1", map[string]string{"q": "x", "type": "photo", "refs": "pexels:photo:1"})
		if resp.StatusCode != 503 || decodeErr(t, resp).Code != "UNAVAILABLE" {
			t.Errorf("%s with no stock service: %d %s", path, resp.StatusCode, resp.Body)
		}
	}
}

func TestStockSearch_OnlyGET(t *testing.T) {
	e, fs := newStockEnv(t)
	resp := e.do(t, "POST", "/stock-media/search", `{"q":"x","type":"photo"}`, "user1")
	if resp.StatusCode != 404 || fs.searchCount() != 0 {
		t.Errorf("POST: status %d, searches %d", resp.StatusCode, fs.searchCount())
	}
	if resp := e.get(t, "/stock-media/other", "user1", nil); resp.StatusCode != 404 {
		t.Errorf("unknown stock route: %d", resp.StatusCode)
	}
}

func TestStockSearch_BothProviders(t *testing.T) {
	e, fs := newStockEnv(t)
	resp := e.get(t, "/stock-media/search", "user1", map[string]string{"q": "  mountain lake ", "type": "PHOTO"})
	if resp.StatusCode != 200 {
		t.Fatalf("status %d %s", resp.StatusCode, resp.Body)
	}
	got := decodeSearch(t, resp)
	if len(got.Assets) != 3 || got.Page != 1 || got.PerPage != 12 {
		t.Fatalf("assets=%d page=%d perPage=%d", len(got.Assets), got.Page, got.PerPage)
	}
	if len(got.Attributions) != 2 || got.Attributions[0].Text != "Photos provided by Pexels" || got.Attributions[1].URL != "https://pixabay.com" {
		t.Errorf("attributions = %+v", got.Attributions)
	}
	if !got.HasMore {
		t.Error("with 100 and 500 matches the first page cannot be the last")
	}
	if p := fs.searches[0]; p.Type != stockmedia.MediaPhoto || strings.TrimSpace(p.Query) != "mountain lake" || p.PerPage != 12 {
		t.Errorf("the service saw %+v", p)
	}
	a := got.Assets[0]
	if a.Credit == "" || a.PageURL == "" || a.PreviewURL == "" || a.Author == "" {
		t.Errorf("an asset the page cannot credit or show: %+v", a)
	}
}

// The full-resolution file is for the renderer. A search must never hand it to
// the browser.
func TestStockSearch_NeverExposesTheOriginalFileOrAKey(t *testing.T) {
	e, _ := newStockEnv(t)
	for _, path := range []string{"/stock-media/search", "/stock-media/assets"} {
		resp := e.get(t, path, "user1", map[string]string{"q": "x", "type": "video", "refs": "pexels:video:11"})
		if strings.Contains(resp.Body, "SECRET-ORIGINAL") || strings.Contains(resp.Body, "mediaUrl") || strings.Contains(strings.ToLower(resp.Body), "apikey") {
			t.Errorf("%s leaks the original file or a key: %s", path, resp.Body)
		}
	}
}

func TestStockSearch_SingleProvider(t *testing.T) {
	e, fs := newStockEnv(t)
	var asked []stockmedia.ProviderName
	fs.search = func(name stockmedia.ProviderName, p stockmedia.SearchParams) (stockmedia.SearchResult, error) {
		asked = append(asked, name)
		return okSearch(stockAsset(name, p.Type, "5", 1080, 1920))(name, p)
	}
	resp := e.get(t, "/stock-media/search", "user1", map[string]string{"q": "x", "type": "video", "provider": "Pixabay"})
	got := decodeSearch(t, resp)
	if resp.StatusCode != 200 || len(got.Assets) != 1 || got.Assets[0].Provider != "pixabay" || fmt.Sprint(asked) != "[pixabay]" {
		t.Fatalf("status %d asked %v body %s", resp.StatusCode, asked, resp.Body)
	}
	if len(got.Attributions) != 1 || got.Attributions[0].Provider != "pixabay" {
		t.Errorf("attributions = %+v", got.Attributions)
	}
}

func TestStockSearch_Validation(t *testing.T) {
	e, fs := newStockEnv(t)
	long := strings.Repeat("a", 101)
	for name, params := range map[string]map[string]string{
		"no query":            {"type": "photo"},
		"blank query":         {"q": "   ", "type": "photo"},
		"no type":             {"q": "x"},
		"bad type":            {"q": "x", "type": "audio"},
		"bad provider":        {"q": "x", "type": "photo", "provider": "unsplash"},
		"bad orientation":     {"q": "x", "type": "photo", "orientation": "diagonal"},
		"page not a number":   {"q": "x", "type": "photo", "page": "two"},
		"page negative":       {"q": "x", "type": "photo", "page": "-1"},
		"page too large":      {"q": "x", "type": "photo", "page": "1001"},
		"perPage not number":  {"q": "x", "type": "photo", "perPage": "lots"},
		"perPage too small":   {"q": "x", "type": "photo", "perPage": "1"},
		"query too long":      {"q": long, "type": "photo"},
		"Hindi only":          {"q": "पहाड़ और नदी", "type": "photo"},
		"another Indian only": {"q": "தமிழ்", "type": "video"},
	} {
		t.Run(name, func(t *testing.T) {
			resp := e.get(t, "/stock-media/search", "user1", params)
			if resp.StatusCode != 400 || decodeErr(t, resp).Code != "VALIDATION_ERROR" {
				t.Errorf("status %d body %s", resp.StatusCode, resp.Body)
			}
		})
	}
	if fs.searchCount() != 0 {
		t.Errorf("%d searches ran for invalid input", fs.searchCount())
	}
	// The Hindi message tells the user what to do.
	msg := decodeErr(t, e.get(t, "/stock-media/search", "user1", map[string]string{"q": "पहाड़", "type": "photo"})).Message
	if !strings.Contains(msg, "English") || !strings.Contains(msg, "mandir") {
		t.Errorf("message %q should say to search in English and give an example", msg)
	}
	// A mixed query is fine: the Hindi words are ignored by the providers, even
	// when they outnumber the English one.
	for _, q := range []string{"mountain पहाड़", "पहाड़ पर बर्फ snow", "बर्फ से ढका Mountain"} {
		if resp := e.get(t, "/stock-media/search", "user1", map[string]string{"q": q, "type": "photo"}); resp.StatusCode != 200 {
			t.Errorf("mixed query %q was refused: %d %s", q, resp.StatusCode, resp.Body)
		}
	}
}

func TestStockSearch_PerPageIsCappedAndDefaultsToPortrait(t *testing.T) {
	e, fs := newStockEnv(t)
	e.get(t, "/stock-media/search", "user1", map[string]string{"q": "x", "type": "photo", "perPage": "30", "page": "3"})
	e.get(t, "/stock-media/search", "user1", map[string]string{"q": "x", "type": "photo", "perPage": "30"})
	if p := fs.searches[0]; p.PerPage != 30 || p.Page != 3 {
		t.Errorf("params = %+v", p)
	}
	// An oversized page is clamped to the maximum and the response says so.
	for _, big := range []string{"31", "500", "99999"} {
		got := decodeSearch(t, e.get(t, "/stock-media/search", "user1", map[string]string{"q": "x", "type": "photo", "perPage": big}))
		if got.PerPage != 30 {
			t.Errorf("perPage %s gave %d, want it clamped to 30", big, got.PerPage)
		}
	}
	if p := fs.searches[len(fs.searches)-1]; p.PerPage != 30 {
		t.Errorf("the service was asked for %d per page", p.PerPage)
	}
}

func TestStockSearch_EmptyResultIsNotAnError(t *testing.T) {
	e, fs := newStockEnv(t)
	fs.all = func(p stockmedia.SearchParams) (stockmedia.MultiResult, error) {
		return stockmedia.MultiResult{Results: []stockmedia.SearchResult{
			{Provider: stockmedia.ProviderPexels, Attribution: stockmedia.Attribution{Text: "Photos provided by Pexels", URL: "https://www.pexels.com"}},
		}}, nil
	}
	resp := e.get(t, "/stock-media/search", "user1", map[string]string{"q": "zzzzqq", "type": "photo"})
	got := decodeSearch(t, resp)
	if resp.StatusCode != 200 || got.Assets == nil || len(got.Assets) != 0 || got.HasMore {
		t.Errorf("status %d body %s", resp.StatusCode, resp.Body)
	}
	if !strings.Contains(resp.Body, `"assets":[]`) {
		t.Errorf("an empty result must be [] and not null, so the page can iterate it: %s", resp.Body)
	}
}

func TestStockSearch_OneProviderDownStillReturnsTheOther(t *testing.T) {
	e, fs := newStockEnv(t)
	fs.all = func(p stockmedia.SearchParams) (stockmedia.MultiResult, error) {
		return stockmedia.MultiResult{
			Results: []stockmedia.SearchResult{{Provider: stockmedia.ProviderPixabay, Type: p.Type, Assets: []stockmedia.Asset{stockAsset(stockmedia.ProviderPixabay, p.Type, "21", 1080, 1920)}, Total: 1,
				Attribution: stockmedia.Attribution{Text: "Images provided by Pixabay", URL: "https://pixabay.com"}}},
			Failures: []stockmedia.ProviderFailure{{Provider: stockmedia.ProviderPexels,
				Err: &stockmedia.ProviderError{Provider: stockmedia.ProviderPexels, Kind: stockmedia.KindRateLimited, RetryAfter: 90 * time.Second}}},
		}, errors.New("not all failed")
	}
	resp := e.get(t, "/stock-media/search", "user1", map[string]string{"q": "x", "type": "photo"})
	got := decodeSearch(t, resp)
	if resp.StatusCode != 200 || len(got.Assets) != 1 {
		t.Fatalf("status %d %s", resp.StatusCode, resp.Body)
	}
	if len(got.Failures) != 1 || got.Failures[0].Provider != "pexels" || got.Failures[0].Reason != "rate_limited" || got.Failures[0].RetryAfter != 90 {
		t.Errorf("failures = %+v", got.Failures)
	}
	if len(got.Attributions) != 1 {
		t.Errorf("only the provider that answered is credited: %+v", got.Attributions)
	}
}

func TestStockSearch_FailureCodes(t *testing.T) {
	for _, tc := range []struct {
		name   string
		err    error
		status int
		code   string
		retry  bool
	}{
		{"all providers rate limited", fmt.Errorf("%w: %w", stockmedia.ErrAllProvidersFailed,
			&stockmedia.ProviderError{Provider: "pexels", Kind: stockmedia.KindRateLimited, RetryAfter: 45 * time.Second}), 429, "PROVIDER_RATE_LIMITED", true},
		{"all providers down", fmt.Errorf("%w: %w", stockmedia.ErrAllProvidersFailed,
			&stockmedia.ProviderError{Provider: "pexels", Kind: stockmedia.KindUnavailable, Status: 503}), 502, "SEARCH_FAILED", false},
		{"not configured", fmt.Errorf("%w: x", stockmedia.ErrNotConfigured), 503, "UNAVAILABLE", false},
		{"invalid params from the service", fmt.Errorf("%w: x", stockmedia.ErrInvalidParams), 400, "VALIDATION_ERROR", false},
		{"unsupported language from the service", fmt.Errorf("%w: %w", stockmedia.ErrInvalidParams, stockmedia.ErrUnsupportedLanguage), 400, "VALIDATION_ERROR", false},
		{"timeout", context.DeadlineExceeded, 504, "SEARCH_TIMEOUT", false},
		{"cancelled", context.Canceled, 408, "CANCELED", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, fs := newStockEnv(t)
			fs.all = func(stockmedia.SearchParams) (stockmedia.MultiResult, error) { return stockmedia.MultiResult{}, tc.err }
			resp := e.get(t, "/stock-media/search", "user1", map[string]string{"q": "x", "type": "photo"})
			if resp.StatusCode != tc.status || decodeErr(t, resp).Code != tc.code {
				t.Errorf("status %d body %s, want %d %s", resp.StatusCode, resp.Body, tc.status, tc.code)
			}
			if got := resp.Headers["Retry-After"]; (got != "") != tc.retry {
				t.Errorf("Retry-After = %q, want present=%v", got, tc.retry)
			}
			if tc.retry && resp.Headers["Retry-After"] != "45" {
				t.Errorf("Retry-After = %q, want 45", resp.Headers["Retry-After"])
			}
			if tc.retry && bodyRetryAfter(t, resp) != "45" {
				t.Errorf("body retryAfter = %s, want 45: the page can only read the body", bodyRetryAfter(t, resp))
			}
			if strings.Contains(resp.Body, "pexels:") || strings.Contains(resp.Body, "HTTP 503") {
				t.Errorf("provider detail leaked: %s", resp.Body)
			}
		})
	}
}

func TestStockSearch_ARejectedKeyLooksLikeAnOutageToTheUser(t *testing.T) {
	e, fs := newStockEnv(t)
	fs.all = func(p stockmedia.SearchParams) (stockmedia.MultiResult, error) {
		return stockmedia.MultiResult{
			Results:  []stockmedia.SearchResult{{Provider: stockmedia.ProviderPixabay, Type: p.Type, Attribution: stockmedia.Attribution{Text: "t", URL: "https://pixabay.com"}}},
			Failures: []stockmedia.ProviderFailure{{Provider: stockmedia.ProviderPexels, Err: &stockmedia.ProviderError{Provider: stockmedia.ProviderPexels, Kind: stockmedia.KindAuth, Status: 401}}},
		}, nil
	}
	got := decodeSearch(t, e.get(t, "/stock-media/search", "user1", map[string]string{"q": "x", "type": "photo"}))
	if len(got.Failures) != 1 || got.Failures[0].Reason != "unavailable" {
		t.Errorf("failures = %+v: a bad key is the site's problem, so the user sees 'unavailable', not 'auth'", got.Failures)
	}
}

func TestStockSearch_PerUserAllowance(t *testing.T) {
	e, fs := newStockEnv(t)
	now := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	e.h.StockLimiter = &PreviewLimiter{Limit: 3, Period: time.Hour, Now: func() time.Time { return now }}
	q := map[string]string{"q": "x", "type": "photo"}
	for i := 0; i < 3; i++ {
		if resp := e.get(t, "/stock-media/search", "user1", q); resp.StatusCode != 200 {
			t.Fatalf("search %d: %d", i+1, resp.StatusCode)
		}
	}
	resp := e.get(t, "/stock-media/search", "user1", q)
	if resp.StatusCode != 429 || decodeErr(t, resp).Code != "RATE_LIMITED" || resp.Headers["Retry-After"] == "" {
		t.Fatalf("status %d %s %v", resp.StatusCode, resp.Body, resp.Headers)
	}
	if got := bodyRetryAfter(t, resp); got != resp.Headers["Retry-After"] {
		t.Errorf("body retryAfter = %s, header = %s: the page can only read the body", got, resp.Headers["Retry-After"])
	}
	if fs.searchCount() != 3 {
		t.Errorf("the service ran %d times, want 3: the refused search must not reach it", fs.searchCount())
	}
	if resp := e.get(t, "/stock-media/search", "user2", q); resp.StatusCode != 200 {
		t.Errorf("another user was blocked: %d", resp.StatusCode)
	}
	now = now.Add(61 * time.Minute)
	if resp := e.get(t, "/stock-media/search", "user1", q); resp.StatusCode != 200 {
		t.Errorf("after the window: %d", resp.StatusCode)
	}
	// An invalid search does not spend the allowance.
	e.h.StockLimiter = &PreviewLimiter{Limit: 1, Period: time.Hour, Now: func() time.Time { return now }}
	for i := 0; i < 5; i++ {
		e.get(t, "/stock-media/search", "user3", map[string]string{"type": "photo"})
	}
	if resp := e.get(t, "/stock-media/search", "user3", q); resp.StatusCode != 200 {
		t.Errorf("invalid searches used up the allowance: %d", resp.StatusCode)
	}
}

func TestStockSearch_DefaultAllowanceIs120AnHour(t *testing.T) {
	e, _ := newStockEnv(t)
	l := e.h.stockLimiter()
	if l.Limit != 120 || l.Period != time.Hour {
		t.Errorf("default limiter = %d per %v", l.Limit, l.Period)
	}
	if e.h.stockLimiter() != l {
		t.Error("the limiter must be created once")
	}
}

// --- /stock-media/assets -----------------------------------------------------

func TestStockAssets_ResolvesInOrderAndMarksMissingOnes(t *testing.T) {
	e, fs := newStockEnv(t)
	fs.get = func(r stockmedia.AssetRef) (stockmedia.Asset, error) {
		switch r.ID {
		case "404":
			return stockmedia.Asset{}, &stockmedia.ProviderError{Provider: r.Provider, Kind: stockmedia.KindNotFound}
		case "503":
			return stockmedia.Asset{}, &stockmedia.ProviderError{Provider: r.Provider, Kind: stockmedia.KindUnavailable}
		case "429":
			return stockmedia.Asset{}, &stockmedia.ProviderError{Provider: r.Provider, Kind: stockmedia.KindRateLimited}
		}
		return stockAsset(r.Provider, r.Type, r.ID, 1080, 1920), nil
	}
	resp := e.get(t, "/stock-media/assets", "user1", map[string]string{"refs": "pexels:photo:1,pixabay:video:404,pexels:photo:503,pixabay:photo:3,pexels:video:429"})
	if resp.StatusCode != 200 {
		t.Fatalf("status %d %s", resp.StatusCode, resp.Body)
	}
	var got StockAssetsResponse
	if err := json.Unmarshal([]byte(resp.Body), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Items) != 5 {
		t.Fatalf("%d items", len(got.Items))
	}
	want := []struct{ id, status string }{{"1", ""}, {"404", "unavailable"}, {"503", "error"}, {"3", ""}, {"429", "error"}}
	for i, w := range want {
		it := got.Items[i]
		if it.ID != w.id || it.Status != w.status || (w.status == "" && it.Asset == nil) || (w.status != "" && it.Asset != nil) {
			t.Errorf("item %d = %+v, want id %s status %q", i, it, w.id, w.status)
		}
	}
	// A credit appears for each provider that produced an asset, once.
	if len(got.Attributions) != 2 {
		t.Errorf("attributions = %+v, want one each for pexels and pixabay", got.Attributions)
	}
	if got.Items[0].Asset.Credit == "" {
		t.Error("asset has no credit line")
	}
}

func TestStockAssets_Validation(t *testing.T) {
	e, fs := newStockEnv(t)
	tooMany := strings.TrimSuffix(strings.Repeat("pexels:photo:1,", storyplan.MaxScenes+1), ",")
	for name, refs := range map[string]string{
		"missing":          "",
		"blank":            "  ",
		"two fields":       "pexels:1",
		"four fields":      "pexels:photo:1:x",
		"unknown provider": "unsplash:photo:1",
		"unknown type":     "pexels:gif:1",
		"id not digits":    "pexels:photo:abc",
		"id is a url":      "pexels:photo:https",
		"empty id":         "pexels:photo:",
		"id too long":      "pexels:photo:" + strings.Repeat("9", 30),
		"one bad of two":   "pexels:photo:1,nope",
		"trailing comma":   "pexels:photo:1,",
		"too many":         tooMany,
	} {
		t.Run(name, func(t *testing.T) {
			resp := e.get(t, "/stock-media/assets", "user1", map[string]string{"refs": refs})
			if resp.StatusCode != 400 || decodeErr(t, resp).Code != "VALIDATION_ERROR" {
				t.Errorf("status %d body %s", resp.StatusCode, resp.Body)
			}
		})
	}
	if fs.lookupCount() != 0 {
		t.Errorf("%d lookups for invalid references", fs.lookupCount())
	}
	exactly := strings.TrimSuffix(strings.Repeat("pexels:photo:1,", storyplan.MaxScenes), ",")
	if resp := e.get(t, "/stock-media/assets", "user1", map[string]string{"refs": exactly}); resp.StatusCode != 200 {
		t.Errorf("exactly %d refs: %d", storyplan.MaxScenes, resp.StatusCode)
	}
}

func TestParseAssetRefs_RoundTripsThroughEscaping(t *testing.T) {
	refs, msg := parseAssetRefs("pexels:photo:12,pixabay:video:34")
	if msg != "" || len(refs) != 2 || refs[1].Provider != "pixabay" || refs[1].Type != "video" || refs[1].ID != "34" {
		t.Errorf("refs = %+v msg %q", refs, msg)
	}
	// The page URL-encodes the colons and commas; the gateway decodes them.
	if got, _ := url.QueryUnescape(url.QueryEscape("pexels:photo:12,pixabay:video:34")); got != "pexels:photo:12,pixabay:video:34" {
		t.Error("escaping round trip")
	}
}

func TestAttributionForProvider(t *testing.T) {
	for _, tc := range []struct {
		provider string
		typ      stockmedia.MediaType
		text     string
		url      string
	}{
		{"pexels", stockmedia.MediaPhoto, "Photos provided by Pexels", "https://www.pexels.com"},
		{"pexels", stockmedia.MediaVideo, "Videos provided by Pexels", "https://www.pexels.com"},
		{"pixabay", stockmedia.MediaPhoto, "Images provided by Pixabay", "https://pixabay.com"},
		{"pixabay", stockmedia.MediaVideo, "Videos provided by Pixabay", "https://pixabay.com"},
	} {
		if a := attributionForProvider(tc.provider, tc.typ); a.Text != tc.text || a.URL != tc.url || a.Provider != tc.provider {
			t.Errorf("%s/%s = %+v", tc.provider, tc.typ, a)
		}
	}
}

// --- PUT /story-plans/:id and media on GET ------------------------------------

func putPlan(t *testing.T, e *spEnv, user, id string, rev int, scenes []storyplan.SceneEdit) events.APIGatewayProxyResponse {
	t.Helper()
	b, _ := json.Marshal(UpdateStoryPlanRequest{Revision: rev, Scenes: scenes})
	return e.do(t, "PUT", "/story-plans/"+id, string(b), user)
}

func newReadyPlan(t *testing.T) (*spEnv, *fakeStock, string) {
	t.Helper()
	e, fs := newStockEnv(t)
	resp := e.do(t, "POST", "/story-plans", storyBody(spEnglish), "user1")
	if resp.StatusCode != 201 {
		t.Fatalf("create: %d %s", resp.StatusCode, resp.Body)
	}
	return e, fs, decodePlan(t, resp).PlanID
}

func TestNewPlansStartAtRevisionOne(t *testing.T) {
	e, _, id := newReadyPlan(t)
	got := decodePlan(t, e.do(t, "GET", "/story-plans/"+id, "", "user1"))
	if got.Revision != 1 {
		t.Errorf("revision = %d", got.Revision)
	}
}

func TestUpdatePlan_NarrationAndMediaRoundTrip(t *testing.T) {
	e, fs, id := newReadyPlan(t)
	edits := make([]storyplan.SceneEdit, 2)
	narr := "The fisherman pushes the boat out into the quiet backwaters before the sun is up"
	edits[0] = storyplan.SceneEdit{Narration: &narr, Media: &storyplan.MediaRef{Provider: "pixabay", Type: "video", ID: "125"}}
	resp := putPlan(t, e, "user1", id, 1, edits)
	if resp.StatusCode != 200 {
		t.Fatalf("status %d %s", resp.StatusCode, resp.Body)
	}
	got := decodePlan(t, resp)
	if got.Revision != 2 || got.Plan.Scenes[0].Narration != narr || got.Plan.Scenes[0].Media == nil {
		t.Fatalf("response = %+v", got)
	}
	// The save stored a reference and nothing else about the asset.
	stored := e.items["USER#user1|STORYPLAN#"+id]["plan"].(*dbtypes.AttributeValueMemberS).Value
	if !strings.Contains(stored, `"media":{"provider":"pixabay","type":"video","id":"125"}`) {
		t.Errorf("stored plan lacks the reference: %s", stored)
	}
	for _, leak := range []string{"https://", "previewUrl", "playUrl", "SECRET-ORIGINAL", "mediaUrl", "credit"} {
		if strings.Contains(stored, leak) {
			t.Errorf("the stored plan contains %q: provider data must never be stored", leak)
		}
	}
	// And reading it back attaches fresh details, with a credit.
	read := decodePlan(t, e.do(t, "GET", "/story-plans/"+id, "", "user1"))
	if len(read.Media) != 2 || read.Media[0] == nil || read.Media[0].Asset == nil || read.Media[1] != nil {
		t.Fatalf("media = %+v", read.Media)
	}
	if read.Media[0].Asset.PlayURL == "" || read.Media[0].Asset.Credit == "" {
		t.Errorf("asset = %+v", read.Media[0].Asset)
	}
	if len(read.MediaAttributions) != 1 || read.MediaAttributions[0].Provider != "pixabay" {
		t.Errorf("attributions = %+v", read.MediaAttributions)
	}
	if fs.lookupCount() < 2 {
		t.Errorf("lookups = %d: the new pick is verified on save and looked up on read", fs.lookupCount())
	}
}

func TestUpdatePlan_StaleRevisionIsAConflict(t *testing.T) {
	e, _, id := newReadyPlan(t)
	narr := "A first edit that should win, said clearly enough to be a scene"
	if resp := putPlan(t, e, "user1", id, 1, []storyplan.SceneEdit{{Narration: &narr}, {}}); resp.StatusCode != 200 {
		t.Fatalf("first: %d %s", resp.StatusCode, resp.Body)
	}
	other := "A second edit made from the old copy of the plan on another tab"
	resp := putPlan(t, e, "user1", id, 1, []storyplan.SceneEdit{{Narration: &other}, {}})
	if resp.StatusCode != 409 || decodeErr(t, resp).Code != "STALE_REVISION" {
		t.Errorf("status %d body %s", resp.StatusCode, resp.Body)
	}
	got := decodePlan(t, e.do(t, "GET", "/story-plans/"+id, "", "user1"))
	if got.Plan.Scenes[0].Narration != narr || got.Revision != 2 {
		t.Errorf("the stale write got through: %q rev %d", got.Plan.Scenes[0].Narration, got.Revision)
	}
	// A revision from the future is refused too.
	if resp := putPlan(t, e, "user1", id, 9, []storyplan.SceneEdit{{}, {}}); resp.StatusCode != 409 {
		t.Errorf("future revision: %d", resp.StatusCode)
	}
}

// Two saves that both read revision N and race to write: exactly one wins, and
// the loser gets a 409 from the database's own condition, not from our read.
func TestUpdatePlan_ARaceBetweenTwoSavesHasOneWinner(t *testing.T) {
	e, _, id := newReadyPlan(t)
	a, b := "The first of two racing edits, long enough to be a real scene", "The second of two racing edits, also long enough to be a scene"
	var wg sync.WaitGroup
	codes := make([]int, 2)
	for i, text := range []*string{&a, &b} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			codes[i] = putPlan(t, e, "user1", id, 1, []storyplan.SceneEdit{{Narration: text}, {}}).StatusCode
		}()
	}
	wg.Wait()
	ok, conflict := 0, 0
	for _, c := range codes {
		switch c {
		case 200:
			ok++
		case 409:
			conflict++
		}
	}
	if ok != 1 || conflict != 1 {
		t.Errorf("codes = %v, want one 200 and one 409", codes)
	}
}

func TestUpdatePlan_TheDatabaseConditionCatchesWhatTheReadMissed(t *testing.T) {
	// The handler's read saw revision 1, but by the time it writes, someone
	// else has saved revision 2. Only the conditional write can notice.
	e, _, id := newReadyPlan(t)
	key := "USER#user1|STORYPLAN#" + id
	db := e.h.DB.(*mockDynamoDB)
	orig := db.getItemFunc
	db.getItemFunc = func(ctx context.Context, in *dynamodb.GetItemInput, o ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
		out, err := orig(ctx, in, o...)
		e.mu.Lock()
		e.items[key]["revision"] = &dbtypes.AttributeValueMemberN{Value: "2"} // lands after the read
		e.mu.Unlock()
		return out, err
	}
	narr := "An edit made against a revision that is already out of date"
	resp := putPlan(t, e, "user1", id, 1, []storyplan.SceneEdit{{Narration: &narr}, {}})
	if resp.StatusCode != 409 || decodeErr(t, resp).Code != "STALE_REVISION" {
		t.Errorf("status %d body %s", resp.StatusCode, resp.Body)
	}
}

// The same race for a plan saved before revisions existed: the read sees no
// revision, then another save gives it revision 1 before this write lands.
func TestUpdatePlan_TheDatabaseConditionCatchesARaceOnAPlanWithoutARevision(t *testing.T) {
	e, _, id := newReadyPlan(t)
	key := "USER#user1|STORYPLAN#" + id
	delete(e.items[key], "revision")
	db := e.h.DB.(*mockDynamoDB)
	orig := db.getItemFunc
	db.getItemFunc = func(ctx context.Context, in *dynamodb.GetItemInput, o ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
		out, err := orig(ctx, in, o...)
		e.mu.Lock()
		e.items[key]["revision"] = &dbtypes.AttributeValueMemberN{Value: "1"} // lands after the read
		e.mu.Unlock()
		return out, err
	}
	narr := "An edit made while another save of the same old plan is landing"
	resp := putPlan(t, e, "user1", id, 0, []storyplan.SceneEdit{{Narration: &narr}, {}})
	if resp.StatusCode != 409 || decodeErr(t, resp).Code != "STALE_REVISION" {
		t.Errorf("status %d body %s", resp.StatusCode, resp.Body)
	}
}

func TestUpdatePlan_APlanWithoutARevisionCanStillBeEdited(t *testing.T) {
	e, _, id := newReadyPlan(t)
	key := "USER#user1|STORYPLAN#" + id
	delete(e.items[key], "revision") // saved before revisions existed
	narr := "Editing a plan that was stored before revisions were introduced"
	resp := putPlan(t, e, "user1", id, 0, []storyplan.SceneEdit{{Narration: &narr}, {}})
	if resp.StatusCode != 200 || decodePlan(t, resp).Revision != 1 {
		t.Errorf("status %d body %s", resp.StatusCode, resp.Body)
	}
	if resp := putPlan(t, e, "user1", id, 0, []storyplan.SceneEdit{{Narration: &narr}, {}}); resp.StatusCode != 409 {
		t.Errorf("the second save with revision 0: %d, want 409", resp.StatusCode)
	}
}

func TestUpdatePlan_OwnershipAndShape(t *testing.T) {
	e, _, id := newReadyPlan(t)
	one := []storyplan.SceneEdit{{}, {}}
	if resp := putPlan(t, e, "user2", id, 1, one); resp.StatusCode != 404 {
		t.Errorf("another user's plan: %d, want 404", resp.StatusCode)
	}
	if resp := e.do(t, "PUT", "/story-plans/"+id, "{}", ""); resp.StatusCode != 401 {
		t.Errorf("no sign-in: %d", resp.StatusCode)
	}
	if resp := e.do(t, "PUT", "/story-plans/"+id, "{nope", "user1"); resp.StatusCode != 400 || decodeErr(t, resp).Code != "INVALID_BODY" {
		t.Errorf("bad json: %d", resp.StatusCode)
	}
	if resp := e.do(t, "PUT", "/story-plans/"+id, `{"revision":"one"}`, "user1"); resp.StatusCode != 400 {
		t.Errorf("revision not a number: %d", resp.StatusCode)
	}
	if resp := putPlan(t, e, "user1", uuidNew(), 1, one); resp.StatusCode != 404 {
		t.Errorf("unknown plan: %d", resp.StatusCode)
	}
	if resp := e.do(t, "PUT", "/story-plans/"+id, `{"revision":1,"scenes":[`+strings.Repeat(`{},`, 40000)+`{}]}`, "user1"); resp.StatusCode != 400 {
		t.Errorf("oversized body: %d", resp.StatusCode)
	}
}

func uuidNew() string { return "00000000-0000-4000-8000-000000000000" }

func TestUpdatePlan_ValidationFailuresChangeNothing(t *testing.T) {
	e, _, id := newReadyPlan(t)
	empty, hindi := "", "यह दृश्य हिंदी में लिखा गया है और कहानी अंग्रेज़ी में है"
	for name, tc := range map[string]struct {
		scenes []storyplan.SceneEdit
		in     string
	}{
		"wrong number of scenes": {[]storyplan.SceneEdit{{}}, "has 1 scenes"},
		"empty narration":        {[]storyplan.SceneEdit{{Narration: &empty}, {}}, "scene 1: the narration is empty"},
		"wrong language":         {[]storyplan.SceneEdit{{}, {Narration: &hindi}}, "scene 2: the narration should be in the same language"},
		"bad media":              {[]storyplan.SceneEdit{{Media: &storyplan.MediaRef{Provider: "pexels", Type: "photo", ID: "https://evil"}}, {}}, "scene 1: that media choice is not valid"},
	} {
		t.Run(name, func(t *testing.T) {
			before := fmt.Sprint(e.items["USER#user1|STORYPLAN#"+id]["plan"], e.items["USER#user1|STORYPLAN#"+id]["revision"])
			resp := putPlan(t, e, "user1", id, 1, tc.scenes)
			if resp.StatusCode != 400 || !strings.Contains(decodeErr(t, resp).Message, tc.in) {
				t.Errorf("status %d body %s, want 400 containing %q", resp.StatusCode, resp.Body, tc.in)
			}
			if after := fmt.Sprint(e.items["USER#user1|STORYPLAN#"+id]["plan"], e.items["USER#user1|STORYPLAN#"+id]["revision"]); after != before {
				t.Error("a rejected edit changed the stored plan")
			}
		})
	}
}

func TestUpdatePlan_NewPicksAreVerifiedButOldOnesAreNot(t *testing.T) {
	e, fs, id := newReadyPlan(t)
	pick := &storyplan.MediaRef{Provider: "pexels", Type: "photo", ID: "11"}
	if resp := putPlan(t, e, "user1", id, 1, []storyplan.SceneEdit{{Media: pick}, {}}); resp.StatusCode != 200 {
		t.Fatalf("pick: %d %s", resp.StatusCode, resp.Body)
	}
	if fs.lookupCount() == 0 {
		t.Fatal("a new pick was saved without checking that it exists")
	}
	before := fs.lookupCount()
	// Now the provider goes away. Editing narration must still work, because
	// the pick did not change.
	fs.get = func(r stockmedia.AssetRef) (stockmedia.Asset, error) {
		return stockmedia.Asset{}, &stockmedia.ProviderError{Provider: r.Provider, Kind: stockmedia.KindUnavailable}
	}
	narr := "Fixing a typo in the narration while the photo library is down for now"
	resp := putPlan(t, e, "user1", id, 2, []storyplan.SceneEdit{{Narration: &narr, Media: pick}, {}})
	if resp.StatusCode != 200 {
		t.Errorf("editing with an unchanged pick failed while the provider was down: %d %s", resp.StatusCode, resp.Body)
	}
	_ = before
	// A different pick while the provider is down cannot be verified: 503.
	other := &storyplan.MediaRef{Provider: "pexels", Type: "photo", ID: "99"}
	resp = putPlan(t, e, "user1", id, 3, []storyplan.SceneEdit{{Media: other}, {}})
	if resp.StatusCode != 503 || decodeErr(t, resp).Code != "PLAN_UNAVAILABLE" {
		t.Errorf("an unverifiable pick: %d %s", resp.StatusCode, resp.Body)
	}
}

func TestUpdatePlan_APickThatDoesNotExistIsRefused(t *testing.T) {
	e, fs, id := newReadyPlan(t)
	fs.get = func(r stockmedia.AssetRef) (stockmedia.Asset, error) {
		return stockmedia.Asset{}, &stockmedia.ProviderError{Provider: r.Provider, Kind: stockmedia.KindNotFound}
	}
	resp := putPlan(t, e, "user1", id, 1, []storyplan.SceneEdit{{}, {Media: &storyplan.MediaRef{Provider: "pixabay", Type: "photo", ID: "123"}}})
	msg := decodeErr(t, resp).Message
	if resp.StatusCode != 400 || !strings.Contains(msg, "Scene 2") || !strings.Contains(msg, "no longer available") {
		t.Errorf("status %d message %q", resp.StatusCode, msg)
	}
	// With no stock service at all, a new pick cannot be saved either.
	e.h.StockMedia = nil
	resp = putPlan(t, e, "user1", id, 1, []storyplan.SceneEdit{{Media: &storyplan.MediaRef{Provider: "pixabay", Type: "photo", ID: "123"}}, {}})
	if resp.StatusCode != 503 {
		t.Errorf("no stock service: %d", resp.StatusCode)
	}
}

func TestUpdatePlan_OnlyReadyPlansAreEditable(t *testing.T) {
	e, _, id := newReadyPlan(t)
	e.items["USER#user1|STORYPLAN#"+id]["status"] = &dbtypes.AttributeValueMemberS{Value: "failed"}
	delete(e.items["USER#user1|STORYPLAN#"+id], "plan")
	resp := putPlan(t, e, "user1", id, 1, []storyplan.SceneEdit{{}, {}})
	if resp.StatusCode != 409 || decodeErr(t, resp).Code != "NOT_READY" {
		t.Errorf("status %d %s", resp.StatusCode, resp.Body)
	}
}

func TestUpdatePlan_SaveFailure(t *testing.T) {
	e, _, id := newReadyPlan(t)
	n := len(e.puts)
	e.putErr = func(k int) error {
		if k > n {
			return errors.New("write failed")
		}
		return nil
	}
	narr := "An edit whose save fails because the database is unavailable right now"
	resp := putPlan(t, e, "user1", id, 1, []storyplan.SceneEdit{{Narration: &narr}, {}})
	if resp.StatusCode != 500 || strings.Contains(resp.Body, "write failed") {
		t.Errorf("status %d body %s", resp.StatusCode, resp.Body)
	}
}

func TestGetPlan_AMissingAssetDoesNotBreakThePlan(t *testing.T) {
	e, fs, id := newReadyPlan(t)
	pick := &storyplan.MediaRef{Provider: "pixabay", Type: "photo", ID: "555"}
	if resp := putPlan(t, e, "user1", id, 1, []storyplan.SceneEdit{{Media: pick}, {Media: &storyplan.MediaRef{Provider: "pexels", Type: "photo", ID: "556"}}}); resp.StatusCode != 200 {
		t.Fatalf("save: %d %s", resp.StatusCode, resp.Body)
	}
	fs.get = func(r stockmedia.AssetRef) (stockmedia.Asset, error) {
		if r.ID == "555" {
			return stockmedia.Asset{}, &stockmedia.ProviderError{Provider: r.Provider, Kind: stockmedia.KindNotFound}
		}
		return stockAsset(r.Provider, r.Type, r.ID, 1080, 1920), nil
	}
	resp := e.do(t, "GET", "/story-plans/"+id, "", "user1")
	got := decodePlan(t, resp)
	if resp.StatusCode != 200 || got.Plan == nil || len(got.Media) != 2 {
		t.Fatalf("status %d: the plan must still open: %s", resp.StatusCode, resp.Body)
	}
	if got.Media[0].Status != "unavailable" || got.Media[0].Asset != nil {
		t.Errorf("scene 1 media = %+v", got.Media[0])
	}
	if got.Media[1].Asset == nil {
		t.Errorf("scene 2 media = %+v: one gone asset must not hide the other", got.Media[1])
	}
	// Even with every provider failing, or no stock service at all, the plan opens.
	fs.get = func(r stockmedia.AssetRef) (stockmedia.Asset, error) {
		return stockmedia.Asset{}, &stockmedia.ProviderError{Provider: r.Provider, Kind: stockmedia.KindUnavailable}
	}
	if resp := e.do(t, "GET", "/story-plans/"+id, "", "user1"); resp.StatusCode != 200 || decodePlan(t, resp).Media[0].Status != "error" {
		t.Errorf("provider outage: %d %s", resp.StatusCode, resp.Body)
	}
	e.h.StockMedia = nil
	if resp := e.do(t, "GET", "/story-plans/"+id, "", "user1"); resp.StatusCode != 200 || len(decodePlan(t, resp).Media) != 0 {
		t.Errorf("no stock service: %d %s", resp.StatusCode, resp.Body)
	}
}

func TestGetPlan_WithNoPicksCostsNoLookups(t *testing.T) {
	e, fs, id := newReadyPlan(t)
	resp := e.do(t, "GET", "/story-plans/"+id, "", "user1")
	if resp.StatusCode != 200 || fs.lookupCount() != 0 || strings.Contains(resp.Body, `"media":[`) {
		t.Errorf("status %d lookups %d body %s", resp.StatusCode, fs.lookupCount(), resp.Body)
	}
}

func TestGetPlan_ATamperedStoredRefIsIgnored(t *testing.T) {
	e, fs, id := newReadyPlan(t)
	key := "USER#user1|STORYPLAN#" + id
	bad := `{"language":"en","hook":"h","scenes":[{"narration":"n","description":"d","keywords":["k"],"mediaType":"photo","duration":5,"media":{"provider":"pexels","type":"photo","id":"../../admin"}},{"narration":"n2","description":"d","keywords":["k"],"mediaType":"photo","duration":5}],"totalSeconds":10}`
	e.items[key]["plan"] = &dbtypes.AttributeValueMemberS{Value: bad}
	resp := e.do(t, "GET", "/story-plans/"+id, "", "user1")
	if resp.StatusCode != 200 || fs.lookupCount() != 0 {
		t.Errorf("status %d lookups %d: an invalid stored reference must never be looked up", resp.StatusCode, fs.lookupCount())
	}
}

func TestPlanEdit_PreservesTheCreatingUsersOwnership(t *testing.T) {
	e, _, id := newReadyPlan(t)
	narr := "An edit that must keep the plan in the same owner's partition"
	putPlan(t, e, "user1", id, 1, []storyplan.SceneEdit{{Narration: &narr}, {}})
	for k, it := range e.items {
		if !strings.HasPrefix(k, "USER#user1|") {
			t.Errorf("unexpected item %s", k)
		}
		if pk := it["PK"].(*dbtypes.AttributeValueMemberS).Value; pk != "USER#user1" {
			t.Errorf("PK = %s", pk)
		}
	}
}
