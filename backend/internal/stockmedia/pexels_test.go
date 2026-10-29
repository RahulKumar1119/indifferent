package stockmedia

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Fixtures follow the response shapes in the Pexels API documentation.
const pexelsPhotosFixture = `{
  "total_results": 10000, "page": 1, "per_page": 3,
  "photos": [
    {"id": 3573351, "width": 3066, "height": 3968,
     "url": "https://www.pexels.com/photo/trees-during-day-3573351/",
     "photographer": "Lukas Rodriguez", "photographer_url": "https://www.pexels.com/@lukas-rodriguez-1845331",
     "alt": "Trees during day",
     "src": {"original": "https://images.pexels.com/photos/3573351/pexels-photo-3573351.png",
             "large": "https://images.pexels.com/photos/3573351/pexels-photo-3573351.png?h=650&w=940",
             "medium": "https://images.pexels.com/photos/3573351/pexels-photo-3573351.png?h=350"}},
    {"id": 2014422, "width": 3024, "height": 3024,
     "url": "https://www.pexels.com/photo/brown-rocks-2014422/",
     "photographer": "Joey Farina", "photographer_url": "https://www.pexels.com/@joey",
     "alt": "Brown rocks",
     "src": {"original": "https://images.pexels.com/photos/2014422/pexels-photo-2014422.jpeg",
             "medium": "https://images.pexels.com/photos/2014422/pexels-photo-2014422.jpeg?h=350"}},
    {"id": 99, "width": 4000, "height": 2250,
     "url": "https://www.pexels.com/photo/wide-99/", "photographer": "Wide Person",
     "src": {"original": "https://images.pexels.com/photos/99/wide.jpg"}}
  ]
}`

const pexelsVideosFixture = `{
  "page": 1, "per_page": 2, "total_results": 20475, "url": "https://www.pexels.com/videos/",
  "videos": [
    {"id": 2499611, "width": 1080, "height": 1920,
     "url": "https://www.pexels.com/video/2499611/",
     "image": "https://images.pexels.com/videos/2499611/free-video-2499611.jpg",
     "duration": 22, "user": {"id": 680589, "name": "Joey Farina", "url": "https://www.pexels.com/@joey"},
     "video_files": [
       {"id": 125004, "quality": "hd", "file_type": "video/mp4", "width": 1080, "height": 1920, "link": "https://player.vimeo.com/external/342571552.hd.mp4?s=abc"},
       {"id": 125005, "quality": "sd", "file_type": "video/mp4", "width": 540, "height": 960, "link": "https://player.vimeo.com/external/342571552.sd.mp4?s=def"},
       {"id": 125006, "quality": "uhd", "file_type": "video/mp4", "width": 2160, "height": 3840, "link": "https://player.vimeo.com/external/342571552.uhd.mp4?s=ghi"}
     ]},
    {"id": 1448735, "width": 4096, "height": 2160,
     "url": "https://www.pexels.com/video/video-of-forest-1448735/",
     "image": "https://images.pexels.com/videos/1448735/free-video-1448735.jpg",
     "duration": 32, "user": {"id": 574687, "name": "Ruvim Miksanskiy", "url": "https://www.pexels.com/@digitech"},
     "video_files": [
       {"id": 58649, "quality": "sd", "file_type": "video/mp4", "width": 640, "height": 338, "link": "https://player.vimeo.com/external/x.sd.mp4"},
       {"id": 58650, "quality": "hd", "file_type": "video/mp4", "width": 1920, "height": 1080, "link": "https://player.vimeo.com/external/x.hd.mp4"},
       {"id": 58651, "quality": "uhd", "file_type": "video/mp4", "width": 4096, "height": 2160, "link": "https://player.vimeo.com/external/x.uhd.mp4"}
     ]}
  ]
}`

func newTestPexels(t *testing.T, rec *recorder) *Pexels {
	t.Helper()
	p, err := NewPexels(ProviderConfig{APIKey: testPexelsKey, BaseURL: rec.URL})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestPexels_PhotoSearchRequest(t *testing.T) {
	rec := newRecorder(t, func(w http.ResponseWriter, r *http.Request, _ int) { respondJSON(w, pexelsPhotosFixture) })
	p := newTestPexels(t, rec)

	_, err := p.Search(context.Background(), SearchParams{Query: "  forest   walk ", Type: MediaPhoto, Page: 2, PerPage: 40})
	if err != nil {
		t.Fatal(err)
	}
	req := rec.request(0)
	if req.URL.Path != "/v1/search" {
		t.Errorf("path = %q, want /v1/search", req.URL.Path)
	}
	// The key is the whole Authorization value, with no scheme prefix.
	if got := req.Header.Get("Authorization"); got != testPexelsKey {
		t.Errorf("Authorization = %q, want the bare key", got)
	}
	if req.URL.Query().Get("key") != "" || strings.Contains(req.URL.String(), testPexelsKey) {
		t.Errorf("Pexels key must travel in the header, not the URL: %s", req.URL)
	}
	q := req.URL.Query()
	for k, want := range map[string]string{"query": "forest walk", "orientation": "portrait", "page": "2", "per_page": "40"} {
		if got := q.Get(k); got != want {
			t.Errorf("param %s = %q, want %q", k, got, want)
		}
	}
}

func TestPexels_VideoSearchUsesVideoEndpoint(t *testing.T) {
	rec := newRecorder(t, func(w http.ResponseWriter, r *http.Request, _ int) { respondJSON(w, pexelsVideosFixture) })
	p := newTestPexels(t, rec)
	if _, err := p.Search(context.Background(), SearchParams{Query: "ocean", Type: MediaVideo}); err != nil {
		t.Fatal(err)
	}
	if got := rec.request(0).URL.Path; got != "/v1/videos/search" {
		t.Errorf("path = %q, want the /v1/videos/ path the docs say replaces /videos/", got)
	}
}

func TestPexels_OrientationAnyOmitsFilter(t *testing.T) {
	rec := newRecorder(t, func(w http.ResponseWriter, r *http.Request, _ int) { respondJSON(w, pexelsPhotosFixture) })
	p := newTestPexels(t, rec)
	if _, err := p.Search(context.Background(), SearchParams{Query: "x", Type: MediaPhoto, Orientation: OrientationAny}); err != nil {
		t.Fatal(err)
	}
	if rec.request(0).URL.Query().Has("orientation") {
		t.Error("orientation=any must not send an orientation filter")
	}
}

func TestPexels_DecodesPhotos(t *testing.T) {
	rec := newRecorder(t, func(w http.ResponseWriter, r *http.Request, _ int) { respondJSON(w, pexelsPhotosFixture) })
	res, err := newTestPexels(t, rec).Search(context.Background(), SearchParams{Query: "trees", Type: MediaPhoto})
	if err != nil {
		t.Fatal(err)
	}
	if res.Provider != ProviderPexels || res.Type != MediaPhoto || res.Total != 10000 || len(res.Assets) != 3 {
		t.Fatalf("unexpected result header: %+v", res)
	}
	a := res.Assets[0]
	want := Asset{
		Provider: ProviderPexels, ID: "3573351", Type: MediaPhoto,
		PageURL:     "https://www.pexels.com/photo/trees-during-day-3573351/",
		PreviewURL:  "https://images.pexels.com/photos/3573351/pexels-photo-3573351.png?h=350",
		MediaURL:    "https://images.pexels.com/photos/3573351/pexels-photo-3573351.png",
		Format:      "image/png",
		Width:       3066,
		Height:      3968,
		Orientation: OrientationPortrait,
		Alt:         "Trees during day",
		Author:      "Lukas Rodriguez",
		AuthorURL:   "https://www.pexels.com/@lukas-rodriguez-1845331",
		Credit:      "Photo by Lukas Rodriguez on Pexels",
	}
	if a != want {
		t.Errorf("asset =\n%+v\nwant\n%+v", a, want)
	}
	if res.Assets[1].Orientation != OrientationSquare || res.Assets[2].Orientation != OrientationLandscape {
		t.Errorf("orientations = %s, %s; want square, landscape", res.Assets[1].Orientation, res.Assets[2].Orientation)
	}
	if res.Attribution.Text != "Photos provided by Pexels" || res.Attribution.URL != "https://www.pexels.com" {
		t.Errorf("attribution = %+v", res.Attribution)
	}
}

func TestPexels_PicksLargestRenditionThatFitsTheFrame(t *testing.T) {
	rec := newRecorder(t, func(w http.ResponseWriter, r *http.Request, _ int) { respondJSON(w, pexelsVideosFixture) })
	res, err := newTestPexels(t, rec).Search(context.Background(), SearchParams{Query: "forest", Type: MediaVideo})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Assets) != 2 {
		t.Fatalf("got %d assets", len(res.Assets))
	}
	portrait, landscape := res.Assets[0], res.Assets[1]

	// 1080x1920 fits in 1920 on the longer side; 2160x3840 (uhd) does not.
	if portrait.Width != 1080 || portrait.Height != 1920 || !strings.HasSuffix(strings.Split(portrait.MediaURL, "?")[0], ".hd.mp4") {
		t.Errorf("portrait picked %dx%d %s, want the 1080x1920 hd file", portrait.Width, portrait.Height, portrait.MediaURL)
	}
	// 1920x1080 is the largest within limit; 4096x2160 is skipped.
	if landscape.Width != 1920 || landscape.Height != 1080 {
		t.Errorf("landscape picked %dx%d, want 1920x1080", landscape.Width, landscape.Height)
	}
	if portrait.Duration != 22 || portrait.Format != "video/mp4" || portrait.Orientation != OrientationPortrait {
		t.Errorf("portrait fields wrong: %+v", portrait)
	}
	if portrait.Credit != "Video by Joey Farina on Pexels" {
		t.Errorf("credit = %q", portrait.Credit)
	}
	if res.Attribution.Text != "Videos provided by Pexels" {
		t.Errorf("attribution = %q", res.Attribution.Text)
	}
}

func TestPexels_SkipsUndecodableItemsAndKeepsTheRest(t *testing.T) {
	body := `{"total_results": 3, "photos": [
	  {"id": 1, "width": 3000, "height": 4000, "url": "https://www.pexels.com/photo/1/", "photographer": "A",
	   "src": {"original": "https://images.pexels.com/photos/1/a.jpg"}},
	  {"id": "not-a-number"},
	  {"id": 0, "src": {"original": "https://images.pexels.com/photos/0/zero.jpg"}},
	  {"id": 4, "width": 3000, "height": 4000, "src": {"original": "https://images.pexels.com/photos/4/noext"}},
	  "garbage"
	]}`
	rec := newRecorder(t, func(w http.ResponseWriter, r *http.Request, _ int) { respondJSON(w, body) })
	res, err := newTestPexels(t, rec).Search(context.Background(), SearchParams{Query: "x", Type: MediaPhoto})
	if err != nil {
		t.Fatalf("one bad item must not fail the page: %v", err)
	}
	if len(res.Assets) != 1 || res.Assets[0].ID != "1" {
		t.Errorf("assets = %s, want only pexels id 1", ids(res.Assets))
	}
	if res.Skipped != 4 {
		t.Errorf("Skipped = %d, want 4", res.Skipped)
	}
}

func TestPexels_ReadsQuotaHeaders(t *testing.T) {
	reset := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC).Unix()
	rec := newRecorder(t, func(w http.ResponseWriter, r *http.Request, _ int) {
		w.Header().Set("X-Ratelimit-Limit", "20000")
		w.Header().Set("X-Ratelimit-Remaining", "19684")
		w.Header().Set("X-Ratelimit-Reset", strconv.FormatInt(reset, 10))
		respondJSON(w, pexelsPhotosFixture)
	})
	res, err := newTestPexels(t, rec).Search(context.Background(), SearchParams{Query: "x", Type: MediaPhoto})
	if err != nil {
		t.Fatal(err)
	}
	if res.Quota == nil || res.Quota.Remaining != 19684 || res.Quota.ResetAt.Unix() != reset {
		t.Errorf("quota = %+v", res.Quota)
	}
}

func TestPexels_ErrorMapping(t *testing.T) {
	tests := []struct {
		status int
		kind   ErrorKind
		is     error
	}{
		{http.StatusTooManyRequests, KindRateLimited, ErrRateLimited},
		{http.StatusUnauthorized, KindAuth, ErrAuth},
		{http.StatusForbidden, KindAuth, ErrAuth},
		{http.StatusBadRequest, KindBadRequest, ErrBadRequest},
		{http.StatusNotFound, KindBadRequest, ErrBadRequest},
		{http.StatusInternalServerError, KindUnavailable, ErrUnavailable},
		{http.StatusBadGateway, KindUnavailable, ErrUnavailable},
		{http.StatusServiceUnavailable, KindUnavailable, ErrUnavailable},
	}
	for _, tc := range tests {
		t.Run(strconv.Itoa(tc.status), func(t *testing.T) {
			rec := newRecorder(t, func(w http.ResponseWriter, r *http.Request, _ int) {
				w.Header().Set("Retry-After", "30")
				http.Error(w, "nope", tc.status)
			})
			_, err := newTestPexels(t, rec).Search(context.Background(), SearchParams{Query: "x", Type: MediaPhoto})
			var pe *ProviderError
			if !errors.As(err, &pe) {
				t.Fatalf("err = %v, want *ProviderError", err)
			}
			if pe.Kind != tc.kind || pe.Status != tc.status || pe.Provider != ProviderPexels {
				t.Errorf("got %+v", pe)
			}
			if !errors.Is(err, tc.is) {
				t.Errorf("errors.Is(err, %v) = false", tc.is)
			}
			if tc.kind == KindRateLimited && pe.RetryAfter != 30*time.Second {
				t.Errorf("RetryAfter = %v, want 30s", pe.RetryAfter)
			}
			if pe.Retryable() != (tc.kind == KindUnavailable) {
				t.Errorf("Retryable = %v for %s", pe.Retryable(), tc.kind)
			}
		})
	}
}

func TestPexels_MalformedResponses(t *testing.T) {
	for name, body := range map[string]string{
		"not json":   `<html>maintenance</html>`,
		"wrong type": `{"photos": "oops"}`,
		"empty":      ``,
	} {
		t.Run(name, func(t *testing.T) {
			rec := newRecorder(t, func(w http.ResponseWriter, r *http.Request, _ int) { respondJSON(w, body) })
			_, err := newTestPexels(t, rec).Search(context.Background(), SearchParams{Query: "x", Type: MediaPhoto})
			if !errors.Is(err, ErrBadResponse) {
				t.Errorf("err = %v, want ErrBadResponse", err)
			}
		})
	}
}

func TestPexels_ZeroResultsIsNotAnError(t *testing.T) {
	rec := newRecorder(t, func(w http.ResponseWriter, r *http.Request, _ int) {
		respondJSON(w, `{"total_results": 0, "page": 1, "per_page": 15, "photos": []}`)
	})
	res, err := newTestPexels(t, rec).Search(context.Background(), SearchParams{Query: "zzzz", Type: MediaPhoto})
	if err != nil || len(res.Assets) != 0 || res.Total != 0 {
		t.Errorf("got res=%+v err=%v, want an empty result and no error", res, err)
	}
}

func TestPexels_ContextCancellationIsReturnedUnchanged(t *testing.T) {
	rec := newRecorder(t, func(w http.ResponseWriter, r *http.Request, _ int) {
		<-r.Context().Done()
	})
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := newTestPexels(t, rec).Search(ctx, SearchParams{Query: "x", Type: MediaPhoto})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v, want context.DeadlineExceeded", err)
	}
}

func TestNewPexels_RequiresKey(t *testing.T) {
	if _, err := NewPexels(ProviderConfig{}); !errors.Is(err, ErrNotConfigured) {
		t.Errorf("err = %v, want ErrNotConfigured", err)
	}
	if _, err := NewPexels(ProviderConfig{APIKey: "k", BaseURL: "not a url"}); err == nil {
		t.Error("an invalid base URL must be rejected at construction")
	}
}
