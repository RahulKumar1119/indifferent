package stockmedia

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

// A provider that starts serving files from a new CDN must not just make
// results vanish: the drop is counted by reason and the host is logged, so the
// cause is one grep away. This runs with the production URL policy.
func TestService_CDNChangeIsVisible(t *testing.T) {
	onNewCDN := goodAsset(ProviderPexels, "1", MediaVideo, 1080, 1920)
	onNewCDN.PageURL = "https://www.pexels.com/video/1/"
	onNewCDN.MediaURL = "https://new-cdn.example.net/v/1.mp4?token=SECRET-TOKEN"
	onKnownCDN := goodAsset(ProviderPexels, "2", MediaVideo, 1080, 1920)
	onKnownCDN.PageURL = "https://www.pexels.com/video/2/"
	onKnownCDN.MediaURL = "https://player.vimeo.com/external/2.hd.mp4"

	var mu sync.Mutex
	var logs []string
	logf := func(format string, args ...any) {
		mu.Lock()
		defer mu.Unlock()
		logs = append(logs, strings.TrimSpace(sprintf(format, args...)))
	}
	s, err := NewService([]Provider{okProvider(ProviderPexels, onNewCDN, onKnownCDN)}, WithLogger(logf)) // default policy
	if err != nil {
		t.Fatal(err)
	}
	res, err := s.Search(context.Background(), ProviderPexels, SearchParams{Query: "x", Type: MediaVideo})
	if err != nil {
		t.Fatal(err)
	}
	if ids(res.Assets) != "pexels:2" || res.SkipReasons["host"] != 1 || res.Skipped != 1 {
		t.Errorf("assets=%s skipped=%d reasons=%v", ids(res.Assets), res.Skipped, res.SkipReasons)
	}
	joined := strings.Join(logs, "\n")
	if !strings.Contains(joined, "reason=host") || !strings.Contains(joined, "new-cdn.example.net") {
		t.Errorf("the rejected host should be logged:\n%s", joined)
	}
	if strings.Contains(joined, "SECRET-TOKEN") || strings.Contains(joined, "/v/1.mp4") {
		t.Errorf("only the host may be logged, never the path or query:\n%s", joined)
	}
}

func TestService_LogsCarryNoQueryText(t *testing.T) {
	var logs []string
	var mu sync.Mutex
	s, _ := NewService([]Provider{okProvider(ProviderPexels)}, WithURLPolicy(allowAnyHost),
		WithLogger(func(f string, a ...any) { mu.Lock(); logs = append(logs, sprintf(f, a...)); mu.Unlock() }))
	// Users' stories become queries; they are not logged, only their length.
	if _, err := s.Search(context.Background(), ProviderPexels, SearchParams{Query: "my private diary entry", Type: MediaPhoto}); err != nil {
		t.Fatal(err)
	}
	if joined := strings.Join(logs, "\n"); strings.Contains(joined, "private") || strings.Contains(joined, "diary") {
		t.Errorf("query text reached the logs:\n%s", joined)
	}
}

func TestQuota_ExhaustedWithoutResetUsesDefaultCooldown(t *testing.T) {
	clock := newFakeClock()
	for _, q := range []*Quota{{Remaining: 0}, {Remaining: 0, ResetAt: clock.Now().Add(-time.Minute)}} {
		prov := &stubProvider{name: ProviderPexels, fn: func(p SearchParams, _ int) (SearchResult, error) {
			return SearchResult{Provider: ProviderPexels, Type: p.Type, Quota: q}, nil
		}}
		s := serviceWithClock(t, clock, prov)
		s.Search(context.Background(), ProviderPexels, SearchParams{Query: "a", Type: MediaPhoto})
		clock.Advance(pexelsCooldown - time.Second)
		if _, err := s.Search(context.Background(), ProviderPexels, SearchParams{Query: "b", Type: MediaPhoto}); !isCoolingDown(err) {
			t.Errorf("quota %+v: want the default cooldown, err = %v", q, err)
		}
		clock.Advance(2 * time.Second)
		if _, err := s.Search(context.Background(), ProviderPexels, SearchParams{Query: "c", Type: MediaPhoto}); isCoolingDown(err) {
			t.Errorf("quota %+v: cooldown should have ended", q)
		}
	}
	// Remaining quota starts nothing.
	prov := &stubProvider{name: ProviderPexels, fn: func(p SearchParams, _ int) (SearchResult, error) {
		return SearchResult{Provider: ProviderPexels, Type: p.Type, Quota: &Quota{Remaining: 5}}, nil
	}}
	s := serviceWithClock(t, clock, prov)
	for _, q := range []string{"a", "b", "c"} {
		if _, err := s.Search(context.Background(), ProviderPexels, SearchParams{Query: q, Type: MediaPhoto}); err != nil {
			t.Fatalf("with quota left, no search should wait: %v", err)
		}
	}
}

func isCoolingDown(err error) bool {
	return err != nil && strings.Contains(err.Error(), ErrCoolingDown.Error())
}

func TestPexels_VideoWithNoUsableRenditionIsSkipped(t *testing.T) {
	body := `{"videos": [
	  {"id": 1, "duration": 5, "user": {"name": "A"}, "video_files": []},
	  {"id": 2, "duration": 5, "user": {"name": "A"}, "video_files": [{"file_type": "video/x-flv", "width": 1080, "height": 1920, "link": "https://player.vimeo.com/a.flv"}]},
	  {"id": 3, "duration": 5, "user": {"name": "A"}, "video_files": [{"file_type": "video/mp4", "width": 0, "height": 0, "link": "https://player.vimeo.com/b.mp4"}]},
	  {"id": 4, "duration": 5, "user": {"name": ""}, "video_files": [{"file_type": "video/mp4", "width": 1080, "height": 1920, "link": "https://player.vimeo.com/c.mp4"}]}
	]}`
	rec := newRecorder(t, func(w http.ResponseWriter, r *http.Request, _ int) { respondJSON(w, body) })
	res, err := newTestPexels(t, rec).Search(context.Background(), SearchParams{Query: "x", Type: MediaVideo})
	if err != nil {
		t.Fatal(err)
	}
	if ids(res.Assets) != "pexels:4" || res.Skipped != 3 {
		t.Errorf("assets=%s skipped=%d", ids(res.Assets), res.Skipped)
	}
	if res.Assets[0].Credit != "Video on Pexels" {
		t.Errorf("credit without an author = %q", res.Assets[0].Credit)
	}
}

func TestPixabay_VideoWithNoUsableRenditionIsSkipped(t *testing.T) {
	body := `{"totalHits": 2, "hits": [
	  {"id": 1, "duration": 5, "videos": {"large": {"url": "", "width": 0}, "medium": {"url": ""}, "small": {}, "tiny": {}}},
	  {"id": 2, "duration": 5, "user": "", "user_id": 0, "pageURL": "https://pixabay.com/videos/id-2/",
	   "videos": {"tiny": {"url": "https://cdn.pixabay.com/video/2_tiny.mp4", "width": 960, "height": 540, "thumbnail": "https://cdn.pixabay.com/video/2_tiny.jpg"}}}
	]}`
	rec := newRecorder(t, func(w http.ResponseWriter, r *http.Request, _ int) { respondJSON(w, body) })
	res, err := newTestPixabay(t, rec).Search(context.Background(), SearchParams{Query: "x", Type: MediaVideo})
	if err != nil {
		t.Fatal(err)
	}
	if ids(res.Assets) != "pixabay:2" || res.Skipped != 1 {
		t.Fatalf("assets=%s skipped=%d", ids(res.Assets), res.Skipped)
	}
	a := res.Assets[0]
	if a.AuthorURL != "" || a.Credit != "Video on Pixabay" {
		t.Errorf("an unknown contributor must not produce a profile URL or a name: %+v", a)
	}
}

func TestPixabay_PhotoWithoutImageSizeFallsBackToPreviewShape(t *testing.T) {
	body := `{"totalHits": 1, "hits": [
	  {"id": 7, "pageURL": "https://pixabay.com/en/p-7/", "largeImageURL": "https://pixabay.com/get/p_1280.jpg",
	   "webformatWidth": 360, "webformatHeight": 640, "user": "U", "user_id": 3}
	]}`
	rec := newRecorder(t, func(w http.ResponseWriter, r *http.Request, _ int) { respondJSON(w, body) })
	res, err := newTestPixabay(t, rec).Search(context.Background(), SearchParams{Query: "x", Type: MediaPhoto})
	if err != nil || len(res.Assets) != 1 {
		t.Fatalf("res=%v err=%v", ids(res.Assets), err)
	}
	a := res.Assets[0]
	// 360x640 has the 9:16 shape; scaled so the longer side is 1280.
	if a.Width != 720 || a.Height != 1280 {
		t.Errorf("size = %dx%d, want 720x1280", a.Width, a.Height)
	}
}

func TestSizeHelpers(t *testing.T) {
	if got := orientationOf(0, 100); got != OrientationAny {
		t.Errorf("unknown size = %q, want any", got)
	}
	if w, h := scaleToLongSide(0, 5, 100); w != 0 || h != 0 {
		t.Errorf("invalid input should give 0x0, got %dx%d", w, h)
	}
	for _, tc := range []struct{ w, h, limit, wantW, wantH int }{
		{1000, 500, 1280, 1000, 500},   // already within the limit: unchanged
		{4000, 2250, 1920, 1920, 1080}, // landscape shrinks
		{2250, 4000, 1280, 720, 1280},  // portrait shrinks
		{3000, 3000, 1500, 1500, 1500}, // square shrinks
	} {
		if w, h := capLongSide(tc.w, tc.h, tc.limit); w != tc.wantW || h != tc.wantH {
			t.Errorf("capLongSide(%d,%d,%d) = %dx%d, want %dx%d", tc.w, tc.h, tc.limit, w, h, tc.wantW, tc.wantH)
		}
	}
}

func TestOptions(t *testing.T) {
	small := goodAsset(ProviderPexels, "1", MediaPhoto, 600, 800)
	prov := okProvider(ProviderPexels, small)
	strict, _ := NewService([]Provider{prov}, WithLogger(func(string, ...any) {}), WithURLPolicy(allowAnyHost))
	lenient, _ := NewService([]Provider{prov}, WithLogger(func(string, ...any) {}), WithURLPolicy(allowAnyHost), WithMinLongSide(500))
	p := SearchParams{Query: "x", Type: MediaPhoto}
	if res, _ := strict.Search(context.Background(), ProviderPexels, p); len(res.Assets) != 0 {
		t.Error("the default minimum size should drop an 800px asset")
	}
	if res, _ := lenient.Search(context.Background(), ProviderPexels, p); len(res.Assets) != 1 {
		t.Error("WithMinLongSide(500) should keep it")
	}

	custom := &fakeDoer{}
	s, _ := NewService([]Provider{prov}, WithLogger(func(string, ...any) {}), WithAssetClient(custom))
	_ = s.CheckAvailable(context.Background(), realAsset())
	if custom.calls == 0 {
		t.Error("WithAssetClient was ignored")
	}
}

type fakeDoer struct{ calls int }

func (f *fakeDoer) Do(req *http.Request) (*http.Response, error) {
	f.calls++
	return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"video/mp4"}}, Body: http.NoBody, Request: req}, nil
}

func TestNewServiceFromEnv_ReadsProcessEnvironment(t *testing.T) {
	t.Setenv(EnvPexelsKey, "")
	t.Setenv(EnvPixabayKey, "")
	if _, err := NewServiceFromEnv(); err == nil {
		t.Error("with no keys set the service must not start")
	}
	t.Setenv(EnvPexelsKey, "k")
	s, err := NewServiceFromEnv(WithLogger(func(string, ...any) {}))
	if err != nil || len(s.Providers()) != 1 {
		t.Errorf("err=%v providers=%v", err, s)
	}
}
