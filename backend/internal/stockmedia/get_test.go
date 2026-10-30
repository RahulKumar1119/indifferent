package stockmedia

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"pgregory.net/rapid"
)

// --- id validation ---------------------------------------------------------

func TestValidateAssetID(t *testing.T) {
	for id, want := range map[string]bool{
		"1": true, "2499611": true, "195893": true, strings.Repeat("9", 18): true,
		"": false, strings.Repeat("9", 19): false,
		"abc": false, "12 34": false, "12;34": false, "-5": false, "+5": false, "1.5": false,
		"0x10": false, "१२३": false, // Devanagari digits are not ASCII digits
		"12/34": false, "12?x=1": false, "..": false, "12\n": false, " 12": false, "1e5": false,
	} {
		if got := ValidAssetID(id); got != want {
			t.Errorf("ValidAssetID(%q) = %v, want %v", id, got, want)
		}
	}
}

func TestAssetRefValid(t *testing.T) {
	good := AssetRef{ProviderPixabay, MediaVideo, "125"}
	if !good.Valid() {
		t.Fatal("a well formed ref was rejected")
	}
	for name, r := range map[string]AssetRef{
		"unknown provider": {"unsplash", MediaVideo, "1"},
		"unknown type":     {ProviderPexels, "audio", "1"},
		"bad id":           {ProviderPexels, MediaPhoto, "x"},
		"empty":            {},
	} {
		if r.Valid() {
			t.Errorf("%s was accepted", name)
		}
	}
}

// --- Pexels ---------------------------------------------------------------

func TestPexels_GetPhoto(t *testing.T) {
	rec := newRecorder(t, func(w http.ResponseWriter, r *http.Request, _ int) {
		respondJSON(w, `{"id": 2014422, "width": 3024, "height": 3024,
		  "url": "https://www.pexels.com/photo/brown-rocks-2014422/",
		  "photographer": "Joey Farina", "photographer_url": "https://www.pexels.com/@joey",
		  "alt": "Brown rocks",
		  "src": {"original": "https://images.pexels.com/photos/2014422/pexels-photo-2014422.jpeg",
		          "medium": "https://images.pexels.com/photos/2014422/pexels-photo-2014422.jpeg?h=350"}}`)
	})
	a, err := newTestPexels(t, rec).Get(context.Background(), MediaPhoto, "2014422")
	if err != nil {
		t.Fatal(err)
	}
	req := rec.request(0)
	if req.URL.Path != "/v1/photos/2014422" || req.Header.Get("Authorization") != testPexelsKey {
		t.Errorf("request = %s, auth %q", req.URL.Path, req.Header.Get("Authorization"))
	}
	if a.ID != "2014422" || a.Type != MediaPhoto || a.Author != "Joey Farina" || a.Orientation != OrientationSquare || a.PreviewURL == "" {
		t.Errorf("asset = %+v", a)
	}
}

func TestPexels_GetVideoUsesTheDocumentedRepeatedPath(t *testing.T) {
	rec := newRecorder(t, func(w http.ResponseWriter, r *http.Request, _ int) {
		respondJSON(w, `{"id": 2499611, "width": 1080, "height": 1920, "url": "https://www.pexels.com/video/2499611/",
		  "image": "https://images.pexels.com/videos/2499611/free-video-2499611.jpg", "duration": 22,
		  "user": {"name": "Joey Farina", "url": "https://www.pexels.com/@joey"},
		  "video_files": [
		   {"file_type": "video/mp4", "width": 1080, "height": 1920, "link": "https://player.vimeo.com/external/1.hd.mp4"},
		   {"file_type": "video/mp4", "width": 540, "height": 960, "link": "https://player.vimeo.com/external/1.sd.mp4"},
		   {"file_type": "video/mp4", "width": 240, "height": 426, "link": "https://player.vimeo.com/external/1.tiny.mp4"}]}`)
	})
	a, err := newTestPexels(t, rec).Get(context.Background(), MediaVideo, "2499611")
	if err != nil {
		t.Fatal(err)
	}
	if got := rec.request(0).URL.Path; got != "/v1/videos/videos/2499611" {
		t.Errorf("path = %q, want /v1/videos/videos/:id as documented", got)
	}
	if !strings.HasSuffix(a.MediaURL, "1.hd.mp4") {
		t.Errorf("render file = %s", a.MediaURL)
	}
	// Preview: the smallest file whose longer side is at least 640, not the
	// 240px one and not the 1080p one.
	if !strings.HasSuffix(a.PlayURL, "1.hd.mp4") && !strings.HasSuffix(a.PlayURL, "1.sd.mp4") {
		t.Errorf("PlayURL = %s", a.PlayURL)
	}
	if !strings.HasSuffix(a.PlayURL, "1.sd.mp4") {
		t.Errorf("PlayURL = %s, want the 540x960 file (longer side 960 >= 640, smallest such)", a.PlayURL)
	}
}

func TestPexels_GetNotFoundAndErrors(t *testing.T) {
	for _, tc := range []struct {
		status int
		is     error
		not    error
	}{
		{404, ErrNotFound, ErrBadRequest},
		{401, ErrAuth, ErrNotFound},
		{429, ErrRateLimited, ErrNotFound},
		{500, ErrUnavailable, ErrNotFound},
	} {
		rec := newRecorder(t, func(w http.ResponseWriter, r *http.Request, _ int) { http.Error(w, "x", tc.status) })
		_, err := newTestPexels(t, rec).Get(context.Background(), MediaPhoto, "1")
		if !errors.Is(err, tc.is) || errors.Is(err, tc.not) {
			t.Errorf("status %d: err = %v", tc.status, err)
		}
	}
}

func TestPexels_GetRejectsBadInputWithoutARequest(t *testing.T) {
	rec := newRecorder(t, func(w http.ResponseWriter, r *http.Request, _ int) { respondJSON(w, `{}`) })
	p := newTestPexels(t, rec)
	for _, id := range []string{"", "abc", "1/../../x", "1?a=b", strings.Repeat("1", 30)} {
		if _, err := p.Get(context.Background(), MediaPhoto, id); !errors.Is(err, ErrInvalidParams) {
			t.Errorf("id %q: err = %v", id, err)
		}
	}
	if _, err := p.Get(context.Background(), "audio", "1"); !errors.Is(err, ErrInvalidParams) {
		t.Errorf("bad type: err = %v", err)
	}
	if rec.count() != 0 {
		t.Errorf("%d requests were made for invalid input", rec.count())
	}
}

func TestPexels_GetUnreadableAsset(t *testing.T) {
	rec := newRecorder(t, func(w http.ResponseWriter, r *http.Request, _ int) { respondJSON(w, `{"id": 0}`) })
	_, err := newTestPexels(t, rec).Get(context.Background(), MediaPhoto, "5")
	if !errors.Is(err, ErrBadResponse) {
		t.Errorf("err = %v, want ErrBadResponse", err)
	}
}

// --- Pixabay --------------------------------------------------------------

func TestPixabay_GetImageAndVideo(t *testing.T) {
	rec := newRecorder(t, func(w http.ResponseWriter, r *http.Request, _ int) {
		if strings.HasPrefix(r.URL.Path, "/api/videos") {
			respondJSON(w, pixabayVideosFixture)
		} else {
			respondJSON(w, pixabayImagesFixture)
		}
	})
	p := newTestPixabay(t, rec)
	img, err := p.Get(context.Background(), MediaPhoto, "195893")
	if err != nil || img.ID != "195893" {
		t.Fatalf("image: %+v, %v", img, err)
	}
	vid, err := p.Get(context.Background(), MediaVideo, "125")
	if err != nil || vid.ID != "125" || vid.PlayURL == "" {
		t.Fatalf("video: %+v, %v", vid, err)
	}
	for i, wantPath := range []string{"/api/", "/api/videos/"} {
		q := rec.request(i).URL.Query()
		if rec.request(i).URL.Path != wantPath || q.Get("id") == "" || q.Get("key") != testPixabayKey {
			t.Errorf("request %d = %s", i, rec.request(i).URL.String())
		}
		if q.Has("q") || q.Has("per_page") {
			t.Errorf("a lookup must not carry search parameters: %v", q)
		}
	}
}

// Observed against the live API on 2026-10-30: a missing video is HTTP 404, a
// missing image is HTTP 200 with an empty list, and a malformed id is HTTP 400.
func TestPixabay_GetMissingAssets(t *testing.T) {
	t.Run("video 404", func(t *testing.T) {
		rec := newRecorder(t, func(w http.ResponseWriter, r *http.Request, _ int) {
			http.Error(w, "[ERROR 404] Video #1 does not exist.", 404)
		})
		if _, err := newTestPixabay(t, rec).Get(context.Background(), MediaVideo, "1"); !errors.Is(err, ErrNotFound) {
			t.Errorf("err = %v", err)
		}
	})
	t.Run("image 200 with no hits", func(t *testing.T) {
		rec := newRecorder(t, func(w http.ResponseWriter, r *http.Request, _ int) {
			respondJSON(w, `{"total":0,"totalHits":0,"hits":[]}`)
		})
		if _, err := newTestPixabay(t, rec).Get(context.Background(), MediaPhoto, "1"); !errors.Is(err, ErrNotFound) {
			t.Errorf("err = %v", err)
		}
	})
	t.Run("a bad key stays an auth error, not a missing asset", func(t *testing.T) {
		rec := newRecorder(t, func(w http.ResponseWriter, r *http.Request, _ int) {
			http.Error(w, "[ERROR 400] Invalid or missing API key (https://pixabay.com/api/docs/).", 400)
		})
		_, err := newTestPixabay(t, rec).Get(context.Background(), MediaPhoto, "1")
		if !errors.Is(err, ErrAuth) || errors.Is(err, ErrNotFound) {
			t.Errorf("err = %v", err)
		}
	})
	t.Run("rate limited", func(t *testing.T) {
		rec := newRecorder(t, func(w http.ResponseWriter, r *http.Request, _ int) { http.Error(w, "API rate limit exceeded", 429) })
		if _, err := newTestPixabay(t, rec).Get(context.Background(), MediaPhoto, "1"); !errors.Is(err, ErrRateLimited) {
			t.Errorf("err = %v", err)
		}
	})
}

func TestPixabay_GetNeverLeaksTheKey(t *testing.T) {
	rec := newRecorder(t, func(w http.ResponseWriter, r *http.Request, _ int) {
		http.Error(w, "echo "+r.URL.String(), http.StatusInternalServerError)
	})
	_, err := newTestPixabay(t, rec).Get(context.Background(), MediaPhoto, "1")
	if err != nil && strings.Contains(err.Error(), testPixabayKey) {
		t.Errorf("the key leaked: %v", err)
	}
	rec2 := newRecorder(t, func(w http.ResponseWriter, r *http.Request, _ int) { http.Error(w, "echo "+r.URL.String(), 404) })
	_, err = newTestPixabay(t, rec2).Get(context.Background(), MediaVideo, "1")
	if err == nil || strings.Contains(err.Error(), testPixabayKey) {
		t.Errorf("err = %v", err)
	}
}

func TestPixabay_GetRejectsBadInputWithoutARequest(t *testing.T) {
	rec := newRecorder(t, func(w http.ResponseWriter, r *http.Request, _ int) { respondJSON(w, `{}`) })
	p := newTestPixabay(t, rec)
	for _, id := range []string{"", "abc", "1&q=x", "1;2", strings.Repeat("1", 30)} {
		if _, err := p.Get(context.Background(), MediaPhoto, id); !errors.Is(err, ErrInvalidParams) {
			t.Errorf("id %q: err = %v", id, err)
		}
	}
	if _, err := p.Get(context.Background(), "gif", "1"); !errors.Is(err, ErrInvalidParams) {
		t.Errorf("bad type: err = %v", err)
	}
	if rec.count() != 0 {
		t.Errorf("%d requests for invalid input", rec.count())
	}
}

func TestPixabay_GetUnreadableAsset(t *testing.T) {
	rec := newRecorder(t, func(w http.ResponseWriter, r *http.Request, _ int) { respondJSON(w, `{"hits":[{"id":0}]}`) })
	if _, err := newTestPixabay(t, rec).Get(context.Background(), MediaPhoto, "5"); !errors.Is(err, ErrBadResponse) {
		t.Errorf("err = %v", err)
	}
}

// --- preview rendition ----------------------------------------------------

func TestPickPreviewRendition(t *testing.T) {
	mk := func(w, h int) rendition {
		return rendition{URL: "https://x.test/" + string(rune('a'+w%26)), Format: "video/mp4", Width: w, Height: h}
	}
	for _, tc := range []struct {
		name string
		in   []rendition
		want int // expected width of the choice, 0 for none
	}{
		{"smallest at or above 640", []rendition{mk(240, 426), mk(540, 960), mk(1080, 1920)}, 540},
		{"exactly 640 qualifies", []rendition{mk(360, 640), mk(1080, 1920)}, 360},
		{"landscape uses its longer side", []rendition{mk(426, 240), mk(640, 360), mk(1920, 1080)}, 640},
		{"all small: the largest", []rendition{mk(160, 240), mk(240, 426)}, 240},
		{"only huge: the smallest of them", []rendition{mk(2160, 3840), mk(1080, 1920)}, 1080},
		{"single", []rendition{mk(1080, 1920)}, 1080},
		{"unusable ones are skipped", []rendition{{URL: "", Format: "video/mp4", Width: 800, Height: 800}, {URL: "x", Format: "video/x-flv", Width: 800, Height: 800}, mk(0, 0), mk(540, 960)}, 540},
		{"none", nil, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := pickPreviewRendition(tc.in)
			if ok != (tc.want != 0) || got.Width != tc.want {
				t.Errorf("got (%d, %v), want %d", got.Width, ok, tc.want)
			}
		})
	}
}

func TestProperty_PreviewRenditionIsAUsableMember(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		n := rapid.IntRange(0, 6).Draw(t, "n")
		var rs []rendition
		for i := 0; i < n; i++ {
			rs = append(rs, rendition{
				URL:    rapid.SampledFrom([]string{"", "https://x.test/v.mp4"}).Draw(t, "url"),
				Format: rapid.SampledFrom([]string{"video/mp4", "video/x-flv", ""}).Draw(t, "fmt"),
				Width:  rapid.IntRange(-3, 4000).Draw(t, "w"),
				Height: rapid.IntRange(-3, 4000).Draw(t, "h"),
			})
		}
		got, ok := pickPreviewRendition(rs)
		var usable []rendition
		for _, r := range rs {
			if r.URL != "" && r.Width > 0 && r.Height > 0 && allowedFormats[r.Format] {
				usable = append(usable, r)
			}
		}
		if ok != (len(usable) > 0) {
			t.Fatalf("ok=%v with %d usable", ok, len(usable))
		}
		if !ok {
			return
		}
		var floor []rendition
		for _, r := range usable {
			if r.longSide() >= previewMinLongSide {
				floor = append(floor, r)
			}
		}
		if len(floor) > 0 {
			if got.longSide() < previewMinLongSide {
				t.Fatalf("picked %+v below the floor though %d meet it", got, len(floor))
			}
			for _, r := range floor {
				if r.pixels() < got.pixels() {
					t.Fatalf("picked %+v but %+v meets the floor and is smaller", got, r)
				}
			}
		} else {
			for _, r := range usable {
				if r.longSide() > got.longSide() {
					t.Fatalf("all below the floor, picked %+v but %+v is larger", got, r)
				}
			}
		}
	})
}

// --- Service.Get ----------------------------------------------------------

func assetStub(name ProviderName, calls *int) *stubProvider {
	return &stubProvider{name: name, getFn: func(typ MediaType, id string, _ int) (Asset, error) {
		return goodAsset(name, id, typ, 2000, 3000), nil
	}}
}

func TestServiceGet_ValidatesBeforeAnyCall(t *testing.T) {
	prov := assetStub(ProviderPexels, nil)
	s := testService(t, prov)
	ctx := context.Background()
	if _, err := s.Get(ctx, ProviderPixabay, MediaPhoto, "1"); !errors.Is(err, ErrNotConfigured) {
		t.Errorf("unregistered provider: %v", err)
	}
	if _, err := s.Get(ctx, ProviderPexels, "audio", "1"); !errors.Is(err, ErrInvalidParams) {
		t.Errorf("bad type: %v", err)
	}
	for _, id := range []string{"", "x", "1/2", strings.Repeat("9", 40)} {
		if _, err := s.Get(ctx, ProviderPexels, MediaPhoto, id); !errors.Is(err, ErrInvalidParams) {
			t.Errorf("id %q: %v", id, err)
		}
	}
	cctx, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := s.Get(cctx, ProviderPexels, MediaPhoto, "1"); !errors.Is(err, context.Canceled) {
		t.Errorf("cancelled: %v", err)
	}
	if prov.getCount() != 0 {
		t.Errorf("provider called %d times for invalid input", prov.getCount())
	}
}

func TestServiceGet_CachesPerProviderTypeAndId(t *testing.T) {
	pexels, pixabay := assetStub(ProviderPexels, nil), assetStub(ProviderPixabay, nil)
	s := testService(t, pexels, pixabay)
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		a, err := s.Get(ctx, ProviderPexels, MediaVideo, "42")
		if err != nil || a.ID != "42" {
			t.Fatal(err)
		}
	}
	if pexels.getCount() != 1 {
		t.Errorf("3 identical lookups made %d provider calls, want 1", pexels.getCount())
	}
	s.Get(ctx, ProviderPexels, MediaPhoto, "42")  // other type
	s.Get(ctx, ProviderPexels, MediaVideo, "43")  // other id
	s.Get(ctx, ProviderPixabay, MediaVideo, "42") // other provider
	if pexels.getCount() != 3 || pixabay.getCount() != 1 {
		t.Errorf("pexels=%d pixabay=%d, want 3 and 1: each (provider, type, id) is its own entry", pexels.getCount(), pixabay.getCount())
	}
}

func TestServiceGet_ErrorsAreNotCachedAndNotFoundIsTyped(t *testing.T) {
	prov := &stubProvider{name: ProviderPexels, getFn: func(typ MediaType, id string, n int) (Asset, error) {
		if n == 1 {
			return Asset{}, &ProviderError{Provider: ProviderPexels, Kind: KindUnavailable, Status: 503}
		}
		return goodAsset(ProviderPexels, id, typ, 2000, 3000), nil
	}}
	s := testService(t, prov)
	if _, err := s.Get(context.Background(), ProviderPexels, MediaPhoto, "7"); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("first: %v", err)
	}
	if _, err := s.Get(context.Background(), ProviderPexels, MediaPhoto, "7"); err != nil {
		t.Errorf("a failure was cached: %v", err)
	}
	gone := &stubProvider{name: ProviderPixabay}
	if _, err := testService(t, gone).Get(context.Background(), ProviderPixabay, MediaPhoto, "7"); !errors.Is(err, ErrNotFound) {
		t.Errorf("not found: %v", err)
	}
}

func TestServiceGet_AnAssetFailingValidationReadsAsNotFound(t *testing.T) {
	tooSmall := &stubProvider{name: ProviderPexels, getFn: func(typ MediaType, id string, _ int) (Asset, error) {
		return goodAsset(ProviderPexels, id, typ, 400, 600), nil
	}}
	var logs []string
	var mu sync.Mutex
	s, _ := NewService([]Provider{tooSmall}, WithURLPolicy(allowAnyHost), WithLogger(func(f string, a ...any) {
		mu.Lock()
		logs = append(logs, sprintf(f, a...))
		mu.Unlock()
	}))
	_, err := s.Get(context.Background(), ProviderPexels, MediaPhoto, "9")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
	if joined := strings.Join(logs, "\n"); !strings.Contains(joined, "stockmedia_get_rejected") || !strings.Contains(joined, "reason=resolution") {
		t.Errorf("the rejection should be logged with its reason:\n%s", joined)
	}
	// And a host off the allowlist is refused with the real policy.
	wrongHost := &stubProvider{name: ProviderPexels, getFn: func(typ MediaType, id string, _ int) (Asset, error) {
		a := goodAsset(ProviderPexels, id, typ, 3000, 4000)
		a.MediaURL = "https://evil.example.com/x.jpg"
		return a, nil
	}}
	real, _ := NewService([]Provider{wrongHost}, WithLogger(func(string, ...any) {}))
	if _, err := real.Get(context.Background(), ProviderPexels, MediaPhoto, "9"); !errors.Is(err, ErrNotFound) {
		t.Errorf("an off-allowlist host was returned: %v", err)
	}
}

func TestServiceGet_RespectsTheRateLimitCooldown(t *testing.T) {
	clock := newFakeClock()
	prov := &stubProvider{name: ProviderPixabay, getFn: func(MediaType, string, int) (Asset, error) {
		return Asset{}, rateLimitErr(ProviderPixabay, 30*time.Second)
	}}
	s := serviceWithClock(t, clock, prov)
	ctx := context.Background()
	if _, err := s.Get(ctx, ProviderPixabay, MediaPhoto, "1"); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("first: %v", err)
	}
	if _, err := s.Get(ctx, ProviderPixabay, MediaPhoto, "2"); !errors.Is(err, ErrCoolingDown) {
		t.Errorf("during the cooldown: %v", err)
	}
	if prov.getCount() != 1 {
		t.Errorf("provider called %d times during a cooldown", prov.getCount())
	}
}

func TestServiceGetMany(t *testing.T) {
	var mu sync.Mutex
	active, peak := 0, 0
	prov := &stubProvider{name: ProviderPexels, getFn: func(typ MediaType, id string, _ int) (Asset, error) {
		mu.Lock()
		active++
		peak = max(peak, active)
		mu.Unlock()
		time.Sleep(15 * time.Millisecond)
		mu.Lock()
		active--
		mu.Unlock()
		if id == "404" {
			return Asset{}, &ProviderError{Provider: ProviderPexels, Kind: KindNotFound, Status: 404}
		}
		return goodAsset(ProviderPexels, id, typ, 2000, 3000), nil
	}}
	s := testService(t, prov)
	refs := []AssetRef{
		{ProviderPexels, MediaPhoto, "1"},
		{ProviderPexels, MediaPhoto, "404"},
		{ProviderPexels, MediaVideo, "bad id"},
		{"unsplash", MediaPhoto, "1"},
		{ProviderPexels, MediaPhoto, "2"},
		{ProviderPexels, MediaVideo, "3"}, {ProviderPexels, MediaVideo, "4"}, {ProviderPexels, MediaVideo, "5"}, {ProviderPexels, MediaVideo, "6"},
	}
	out := s.GetMany(context.Background(), refs)
	if len(out) != len(refs) {
		t.Fatalf("%d results for %d refs", len(out), len(refs))
	}
	for i, r := range out {
		if r.Ref != refs[i] {
			t.Errorf("result %d is for %+v, want %+v: order must be kept", i, r.Ref, refs[i])
		}
	}
	if out[0].Asset == nil || out[0].Asset.ID != "1" || out[4].Asset == nil || out[8].Asset == nil {
		t.Errorf("good lookups failed: %+v", out)
	}
	if !errors.Is(out[1].Err, ErrNotFound) || out[1].Asset != nil {
		t.Errorf("a missing asset must not fail the others: %+v", out[1])
	}
	if !errors.Is(out[2].Err, ErrInvalidParams) || !errors.Is(out[3].Err, ErrInvalidParams) {
		t.Errorf("malformed refs: %v / %v", out[2].Err, out[3].Err)
	}
	if peak > maxConcurrentGets {
		t.Errorf("%d lookups ran at once, the bound is %d", peak, maxConcurrentGets)
	}
	if got := s.GetMany(context.Background(), nil); len(got) != 0 {
		t.Errorf("no refs gave %d results", len(got))
	}
}

func TestServiceGetMany_ContextCancelled(t *testing.T) {
	prov := assetStub(ProviderPexels, nil)
	s := testService(t, prov)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	refs := make([]AssetRef, 8)
	for i := range refs {
		refs[i] = AssetRef{ProviderPexels, MediaPhoto, "1"}
	}
	for i, r := range s.GetMany(ctx, refs) {
		if r.Err == nil {
			t.Errorf("result %d succeeded on a cancelled context", i)
		}
	}
}

func TestSearchSeedsTheLookupCache(t *testing.T) {
	a := goodAsset(ProviderPexels, "777", MediaVideo, 1080, 1920)
	prov := okProvider(ProviderPexels, a)
	prov.getFn = func(MediaType, string, int) (Asset, error) {
		t.Error("the provider was asked for an asset the search had just returned")
		return Asset{}, &ProviderError{Provider: ProviderPexels, Kind: KindUnavailable}
	}
	s := testService(t, prov)
	ctx := context.Background()
	if _, err := s.Search(ctx, ProviderPexels, SearchParams{Query: "x", Type: MediaVideo}); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get(ctx, ProviderPexels, MediaVideo, "777")
	if err != nil || got.ID != "777" || got.Width != 1080 {
		t.Fatalf("Get after Search: %+v, %v", got, err)
	}
	if prov.getCount() != 0 {
		t.Errorf("%d lookups reached the provider", prov.getCount())
	}
	// Only validated assets are seeded: one that failed validation must not be
	// served by Get just because a search saw it.
	bad := goodAsset(ProviderPexels, "888", MediaPhoto, 300, 400) // too small
	prov2 := okProvider(ProviderPexels, bad)
	s2 := testService(t, prov2)
	if _, err := s2.Search(ctx, ProviderPexels, SearchParams{Query: "x", Type: MediaPhoto, Orientation: OrientationAny}); err != nil {
		t.Fatal(err)
	}
	if _, err := s2.Get(ctx, ProviderPexels, MediaPhoto, "888"); !errors.Is(err, ErrNotFound) {
		t.Errorf("an asset that failed validation was served by Get: %v", err)
	}
}

func TestSearchSeedingIsKeyedByProviderTypeAndId(t *testing.T) {
	a := goodAsset(ProviderPexels, "5", MediaPhoto, 2000, 3000)
	prov := okProvider(ProviderPexels, a)
	asked := 0
	prov.getFn = func(typ MediaType, id string, _ int) (Asset, error) {
		asked++
		return goodAsset(ProviderPexels, id, typ, 2000, 3000), nil
	}
	s := testService(t, prov)
	s.Search(context.Background(), ProviderPexels, SearchParams{Query: "x", Type: MediaPhoto})
	s.Get(context.Background(), ProviderPexels, MediaPhoto, "5") // seeded
	s.Get(context.Background(), ProviderPexels, MediaVideo, "5") // other type: must ask
	s.Get(context.Background(), ProviderPexels, MediaPhoto, "6") // other id: must ask
	if asked != 2 {
		t.Errorf("provider asked %d times, want 2 (only the unseeded ones)", asked)
	}
}
