package stockmedia

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Fixtures follow the response shapes in the Pixabay API documentation.
const pixabayImagesFixture = `{
  "total": 4692, "totalHits": 500,
  "hits": [
    {"id": 195893, "pageURL": "https://pixabay.com/en/blossom-bloom-flower-195893/",
     "type": "photo", "tags": "blossom, bloom, flower",
     "previewURL": "https://cdn.pixabay.com/photo/2013/10/15/09/12/flower-195893_150.jpg",
     "webformatURL": "https://pixabay.com/get/35bbf209e13e39d2_640.jpg", "webformatWidth": 360, "webformatHeight": 640,
     "largeImageURL": "https://pixabay.com/get/ed6a99fd0a76647_1280.jpg",
     "imageWidth": 2250, "imageHeight": 4000,
     "user_id": 48777, "user": "Josch13"},
    {"id": 73424, "pageURL": "https://pixabay.com/en/wide-73424/", "tags": "wide",
     "webformatURL": "https://pixabay.com/get/aaaa_640.jpg", "webformatWidth": 640, "webformatHeight": 360,
     "largeImageURL": "https://pixabay.com/get/bbbb_1280.jpg",
     "fullHDURL": "https://pixabay.com/get/cccc_1920.jpg",
     "imageWidth": 4000, "imageHeight": 2250,
     "user_id": 5, "user": "Wide Person"}
  ]
}`

const pixabayVideosFixture = `{
  "total": 42, "totalHits": 42,
  "hits": [
    {"id": 125, "pageURL": "https://pixabay.com/videos/id-125/", "type": "film", "tags": "flowers, yellow, blossom",
     "duration": 12,
     "videos": {
       "large":  {"url": "", "width": 0, "height": 0, "size": 0, "thumbnail": ""},
       "medium": {"url": "https://cdn.pixabay.com/video/2015/08/08/125-135736646_medium.mp4", "width": 1920, "height": 1080, "size": 3562083, "thumbnail": "https://cdn.pixabay.com/video/2015/08/08/125-135736646_medium.jpg"},
       "small":  {"url": "https://cdn.pixabay.com/video/2015/08/08/125-135736646_small.mp4", "width": 1280, "height": 720, "size": 1030736, "thumbnail": "https://cdn.pixabay.com/video/2015/08/08/125-135736646_small.jpg"},
       "tiny":   {"url": "https://cdn.pixabay.com/video/2015/08/08/125-135736646_tiny.mp4", "width": 960, "height": 540, "size": 426799, "thumbnail": "https://cdn.pixabay.com/video/2015/08/08/125-135736646_tiny.jpg"}
     },
     "user_id": 1281706, "user": "Coverr-Free-Footage"},
    {"id": 473, "pageURL": "https://pixabay.com/videos/id-473/", "tags": "tall", "duration": 8,
     "videos": {
       "large":  {"url": "https://cdn.pixabay.com/video/473_large.mp4", "width": 2160, "height": 3840, "thumbnail": "https://cdn.pixabay.com/video/473_large.jpg"},
       "medium": {"url": "https://cdn.pixabay.com/video/473_medium.mp4", "width": 1080, "height": 1920, "thumbnail": "https://cdn.pixabay.com/video/473_medium.jpg"},
       "small":  {"url": "https://cdn.pixabay.com/video/473_small.mp4", "width": 720, "height": 1280, "thumbnail": "https://cdn.pixabay.com/video/473_small.jpg"},
       "tiny":   {"url": "https://cdn.pixabay.com/video/473_tiny.mp4", "width": 540, "height": 960, "thumbnail": "https://cdn.pixabay.com/video/473_tiny.jpg"}
     },
     "user_id": 9, "user": "Tall Filmer"}
  ]
}`

func newTestPixabay(t *testing.T, rec *recorder) *Pixabay {
	t.Helper()
	p, err := NewPixabay(ProviderConfig{APIKey: testPixabayKey, BaseURL: rec.URL})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestPixabay_ImageSearchRequest(t *testing.T) {
	rec := newRecorder(t, func(w http.ResponseWriter, r *http.Request, _ int) { respondJSON(w, pixabayImagesFixture) })
	_, err := newTestPixabay(t, rec).Search(context.Background(), SearchParams{Query: "yellow flowers", Type: MediaPhoto, Page: 3, PerPage: 20})
	if err != nil {
		t.Fatal(err)
	}
	req := rec.request(0)
	if req.URL.Path != "/api/" {
		t.Errorf("path = %q, want /api/", req.URL.Path)
	}
	q := req.URL.Query()
	want := map[string]string{
		"key": testPixabayKey, "q": "yellow flowers", "image_type": "photo",
		"orientation": "vertical", "safesearch": "true", "page": "3", "per_page": "20",
	}
	for k, v := range want {
		if got := q.Get(k); got != v {
			t.Errorf("param %s = %q, want %q", k, got, v)
		}
	}
	if req.Header.Get("Authorization") != "" {
		t.Error("Pixabay takes its key as a query parameter, not a header")
	}
}

func TestPixabay_OrientationParamMapping(t *testing.T) {
	for _, tc := range []struct {
		in      Orientation
		want    string
		present bool
	}{
		{OrientationPortrait, "vertical", true},
		{OrientationLandscape, "horizontal", true},
		{OrientationSquare, "", false}, // the API has no square filter
		{OrientationAny, "", false},
	} {
		t.Run(string(tc.in), func(t *testing.T) {
			rec := newRecorder(t, func(w http.ResponseWriter, r *http.Request, _ int) { respondJSON(w, pixabayImagesFixture) })
			if _, err := newTestPixabay(t, rec).Search(context.Background(), SearchParams{Query: "x", Type: MediaPhoto, Orientation: tc.in}); err != nil {
				t.Fatal(err)
			}
			q := rec.request(0).URL.Query()
			if q.Has("orientation") != tc.present || q.Get("orientation") != tc.want {
				t.Errorf("orientation = %q (present=%v), want %q (present=%v)", q.Get("orientation"), q.Has("orientation"), tc.want, tc.present)
			}
		})
	}
}

func TestPixabay_VideoSearchHasNoOrientationParam(t *testing.T) {
	rec := newRecorder(t, func(w http.ResponseWriter, r *http.Request, _ int) { respondJSON(w, pixabayVideosFixture) })
	if _, err := newTestPixabay(t, rec).Search(context.Background(), SearchParams{Query: "x", Type: MediaVideo, Orientation: OrientationPortrait}); err != nil {
		t.Fatal(err)
	}
	req := rec.request(0)
	if req.URL.Path != "/api/videos/" {
		t.Errorf("path = %q, want /api/videos/", req.URL.Path)
	}
	q := req.URL.Query()
	if q.Has("orientation") || q.Has("image_type") {
		t.Errorf("the video endpoint takes neither orientation nor image_type: %v", q)
	}
	if q.Get("safesearch") != "true" {
		t.Error("safesearch must stay on for video too")
	}
}

func TestPixabay_DecodesImages(t *testing.T) {
	rec := newRecorder(t, func(w http.ResponseWriter, r *http.Request, _ int) { respondJSON(w, pixabayImagesFixture) })
	res, err := newTestPixabay(t, rec).Search(context.Background(), SearchParams{Query: "flower", Type: MediaPhoto})
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 500 || len(res.Assets) != 2 {
		t.Fatalf("total=%d assets=%d", res.Total, len(res.Assets))
	}
	a := res.Assets[0]
	// largeImageURL is at most 1280px on its longer side: 2250x4000 shrinks to
	// 720x1280.
	want := Asset{
		Provider: ProviderPixabay, ID: "195893", Type: MediaPhoto,
		PageURL:     "https://pixabay.com/en/blossom-bloom-flower-195893/",
		PreviewURL:  "https://pixabay.com/get/35bbf209e13e39d2_640.jpg",
		MediaURL:    "https://pixabay.com/get/ed6a99fd0a76647_1280.jpg",
		Format:      "image/jpeg",
		Width:       720,
		Height:      1280,
		Orientation: OrientationPortrait,
		Alt:         "blossom, bloom, flower",
		Author:      "Josch13",
		AuthorURL:   "https://pixabay.com/users/Josch13-48777/",
		Credit:      "Image by Josch13 on Pixabay",
	}
	if a != want {
		t.Errorf("asset =\n%+v\nwant\n%+v", a, want)
	}
	// The second hit has full API access: fullHDURL is used, up to 1920px.
	b := res.Assets[1]
	if b.MediaURL != "https://pixabay.com/get/cccc_1920.jpg" || b.Width != 1920 || b.Height != 1080 {
		t.Errorf("full-HD asset = %s %dx%d", b.MediaURL, b.Width, b.Height)
	}
	if res.Attribution.Text != "Images provided by Pixabay" || res.Attribution.URL != "https://pixabay.com" {
		t.Errorf("attribution = %+v", res.Attribution)
	}
}

func TestPixabay_VideoRenditionsIgnoreEmptyLargeAndPickBestFit(t *testing.T) {
	rec := newRecorder(t, func(w http.ResponseWriter, r *http.Request, _ int) { respondJSON(w, pixabayVideosFixture) })
	res, err := newTestPixabay(t, rec).Search(context.Background(), SearchParams{Query: "x", Type: MediaVideo})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Assets) != 2 {
		t.Fatalf("got %d assets", len(res.Assets))
	}
	// First hit: "large" is empty (no 4K), so medium 1920x1080 is the best fit.
	first := res.Assets[0]
	if !strings.HasSuffix(first.MediaURL, "_medium.mp4") || first.Width != 1920 || first.Height != 1080 {
		t.Errorf("first picked %s %dx%d, want the 1920x1080 medium file", first.MediaURL, first.Width, first.Height)
	}
	if first.Duration != 12 || first.Format != "video/mp4" || first.PreviewURL == "" {
		t.Errorf("first fields wrong: %+v", first)
	}
	// Second hit: large is 2160x3840 (over the frame), so medium 1080x1920.
	second := res.Assets[1]
	if !strings.HasSuffix(second.MediaURL, "473_medium.mp4") || second.Orientation != OrientationPortrait {
		t.Errorf("second picked %s (%s), want the portrait medium file", second.MediaURL, second.Orientation)
	}
}

func TestPixabay_SkipsBrokenHits(t *testing.T) {
	body := `{"totalHits": 4, "hits": [
	  {"id": 1, "pageURL": "https://pixabay.com/en/a-1/", "largeImageURL": "https://pixabay.com/get/a_1280.jpg", "imageWidth": 3000, "imageHeight": 4000, "user": "A", "user_id": 1},
	  {"id": 2, "largeImageURL": "https://pixabay.com/get/noext", "imageWidth": 3000, "imageHeight": 4000},
	  {"id": 0, "largeImageURL": "https://pixabay.com/get/z_1280.jpg"},
	  {"id": "x"}
	]}`
	rec := newRecorder(t, func(w http.ResponseWriter, r *http.Request, _ int) { respondJSON(w, body) })
	res, err := newTestPixabay(t, rec).Search(context.Background(), SearchParams{Query: "x", Type: MediaPhoto})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Assets) != 1 || res.Assets[0].ID != "1" || res.Skipped != 3 {
		t.Errorf("assets=%s skipped=%d", ids(res.Assets), res.Skipped)
	}
}

func TestPixabay_ReadsQuotaHeaders(t *testing.T) {
	clock := newFakeClock()
	rec := newRecorder(t, func(w http.ResponseWriter, r *http.Request, _ int) {
		w.Header().Set("X-RateLimit-Limit", "100")
		w.Header().Set("X-RateLimit-Remaining", "97")
		w.Header().Set("X-RateLimit-Reset", "42")
		respondJSON(w, pixabayImagesFixture)
	})
	p := newTestPixabay(t, rec)
	p.now = clock.Now
	res, err := p.Search(context.Background(), SearchParams{Query: "x", Type: MediaPhoto})
	if err != nil {
		t.Fatal(err)
	}
	if res.Quota == nil || res.Quota.Remaining != 97 || !res.Quota.ResetAt.Equal(clock.Now().Add(42*time.Second)) {
		t.Errorf("quota = %+v", res.Quota)
	}
}

func TestPixabay_RateLimitedUsesWindowReset(t *testing.T) {
	clock := newFakeClock()
	rec := newRecorder(t, func(w http.ResponseWriter, r *http.Request, _ int) {
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.Header().Set("X-RateLimit-Reset", "37")
		http.Error(w, "API rate limit exceeded", http.StatusTooManyRequests)
	})
	p := newTestPixabay(t, rec)
	p.now = clock.Now
	res, err := p.Search(context.Background(), SearchParams{Query: "x", Type: MediaPhoto})
	var pe *ProviderError
	if !errors.As(err, &pe) || pe.Kind != KindRateLimited {
		t.Fatalf("err = %v, want a rate-limit ProviderError", err)
	}
	if pe.RetryAfter != 37*time.Second {
		t.Errorf("RetryAfter = %v, want 37s from the window reset header", pe.RetryAfter)
	}
	if res.Quota == nil || res.Quota.Remaining != 0 {
		t.Errorf("quota should be reported on a 429 too: %+v", res.Quota)
	}
	if !strings.Contains(err.Error(), "API rate limit exceeded") {
		t.Errorf("the provider's message should be kept for diagnostics: %v", err)
	}
}

func TestPixabay_ErrorMapping(t *testing.T) {
	tests := []struct {
		status int
		kind   ErrorKind
		is     error
	}{
		{http.StatusTooManyRequests, KindRateLimited, ErrRateLimited},
		{http.StatusBadRequest, KindBadRequest, ErrBadRequest},
		{http.StatusUnauthorized, KindAuth, ErrAuth},
		{http.StatusInternalServerError, KindUnavailable, ErrUnavailable},
		{http.StatusGatewayTimeout, KindUnavailable, ErrUnavailable},
	}
	for _, tc := range tests {
		t.Run(strconv.Itoa(tc.status), func(t *testing.T) {
			rec := newRecorder(t, func(w http.ResponseWriter, r *http.Request, _ int) { http.Error(w, "[ERROR 400] bad", tc.status) })
			_, err := newTestPixabay(t, rec).Search(context.Background(), SearchParams{Query: "x", Type: MediaPhoto})
			var pe *ProviderError
			if !errors.As(err, &pe) || pe.Kind != tc.kind || pe.Status != tc.status || !errors.Is(err, tc.is) {
				t.Errorf("err = %v (%+v), want kind %s", err, pe, tc.kind)
			}
		})
	}
}

// Observed against the live API: an invalid or missing key is HTTP 400 with a
// plain-text body, not 401. It must still surface as an auth error, while an
// ordinary 400 stays a bad request.
func TestPixabay_InvalidKeyIs400ButMapsToAuth(t *testing.T) {
	for name, tc := range map[string]struct {
		status int
		body   string
		auth   bool
	}{
		"live message":        {http.StatusBadRequest, "[ERROR 400] Invalid or missing API key (https://pixabay.com/api/docs/).", true},
		"different casing":    {http.StatusBadRequest, "invalid API KEY", true},
		"other 400":           {http.StatusBadRequest, `[ERROR 400] "per_page" is out of valid range.`, false},
		"api key text on 500": {http.StatusInternalServerError, "API key service down", false},
	} {
		t.Run(name, func(t *testing.T) {
			rec := newRecorder(t, func(w http.ResponseWriter, r *http.Request, _ int) { http.Error(w, tc.body, tc.status) })
			_, err := newTestPixabay(t, rec).Search(context.Background(), SearchParams{Query: "x", Type: MediaPhoto})
			if got := errors.Is(err, ErrAuth); got != tc.auth {
				t.Errorf("errors.Is(err, ErrAuth) = %v, want %v (%v)", got, tc.auth, err)
			}
			if tc.auth && errors.Is(err, ErrBadRequest) {
				t.Error("a rejected key must not also read as a bad request")
			}
		})
	}
}

// Pixabay's key is in the URL. These tests make sure nothing a caller can
// print, log or serialize ever contains it.
func TestPixabay_KeyNeverLeaks(t *testing.T) {
	t.Run("connection refused", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
		base := srv.URL
		srv.Close() // nothing listens any more
		p, _ := NewPixabay(ProviderConfig{APIKey: testPixabayKey, BaseURL: base})
		_, err := p.Search(context.Background(), SearchParams{Query: "secret-query", Type: MediaPhoto})
		assertNoKey(t, err)
		if !errors.Is(err, ErrUnavailable) {
			t.Errorf("err = %v, want ErrUnavailable", err)
		}
	})

	t.Run("timeout", func(t *testing.T) {
		rec := newRecorder(t, func(w http.ResponseWriter, r *http.Request, _ int) { time.Sleep(300 * time.Millisecond) })
		p, _ := NewPixabay(ProviderConfig{APIKey: testPixabayKey, BaseURL: rec.URL, HTTPClient: &http.Client{Timeout: 50 * time.Millisecond}})
		_, err := p.Search(context.Background(), SearchParams{Query: "x", Type: MediaPhoto})
		assertNoKey(t, err)
		if !errors.Is(err, ErrUnavailable) {
			t.Errorf("err = %v, want ErrUnavailable", err)
		}
	})

	t.Run("error body that echoes the request", func(t *testing.T) {
		rec := newRecorder(t, func(w http.ResponseWriter, r *http.Request, _ int) {
			http.Error(w, "bad request: "+r.URL.String(), http.StatusBadRequest)
		})
		_, err := newTestPixabay(t, rec).Search(context.Background(), SearchParams{Query: "x", Type: MediaPhoto})
		assertNoKey(t, err)
		if err == nil || !strings.Contains(err.Error(), "[REDACTED]") {
			t.Errorf("the key should be replaced by a marker, got: %v", err)
		}
	})

	t.Run("key with characters that get escaped", func(t *testing.T) {
		const odd = "ab+c/d=e&f"
		rec := newRecorder(t, func(w http.ResponseWriter, r *http.Request, _ int) {
			http.Error(w, "echo: "+r.URL.RawQuery, http.StatusBadRequest)
		})
		p, _ := NewPixabay(ProviderConfig{APIKey: odd, BaseURL: rec.URL})
		_, err := p.Search(context.Background(), SearchParams{Query: "x", Type: MediaPhoto})
		if err == nil || strings.Contains(err.Error(), "ab%2Bc") || strings.Contains(err.Error(), odd) {
			t.Errorf("escaped or raw key leaked: %v", err)
		}
	})

	t.Run("formatting a secret and a config", func(t *testing.T) {
		cfg := ProviderConfig{APIKey: testPixabayKey}
		for _, s := range []string{
			fmt.Sprint(cfg.APIKey), fmt.Sprintf("%v", cfg), fmt.Sprintf("%+v", cfg),
			fmt.Sprintf("%#v", cfg), fmt.Sprintf("%s", cfg.APIKey), fmt.Sprintf("%q", cfg.APIKey),
		} {
			if strings.Contains(s, testPixabayKey) {
				t.Errorf("secret leaked through formatting: %s", s)
			}
		}
		b, _ := jsonMarshal(cfg)
		if strings.Contains(string(b), testPixabayKey) {
			t.Errorf("secret leaked through JSON: %s", b)
		}
	})
}

func assertNoKey(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected an error")
	}
	for _, s := range []string{err.Error(), fmt.Sprintf("%v", err), fmt.Sprintf("%+v", err), fmt.Sprintf("%#v", err)} {
		if strings.Contains(s, testPixabayKey) {
			t.Errorf("error leaks the key: %s", s)
		}
		// A "key=" parameter is fine only if its value was redacted.
		if i := strings.Index(s, "key="); i >= 0 && !strings.HasPrefix(s[i:], "key=[REDACTED]") {
			t.Errorf("error shows a key parameter that was not redacted: %s", s)
		}
		// net/http puts the full URL in transport errors; none may get through.
		if strings.Contains(s, "http://") || strings.Contains(s, "https://") {
			t.Errorf("error contains a URL: %s", s)
		}
	}
	for e := err; e != nil; e = errors.Unwrap(e) {
		if strings.Contains(e.Error(), testPixabayKey) {
			t.Errorf("wrapped error leaks the key: %s", e.Error())
		}
	}
}

func TestNewPixabay_RequiresKey(t *testing.T) {
	if _, err := NewPixabay(ProviderConfig{}); !errors.Is(err, ErrNotConfigured) {
		t.Errorf("err = %v, want ErrNotConfigured", err)
	}
}
