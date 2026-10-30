//go:build live

package stockmedia

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

// Live checks against the real Pexels and Pixabay APIs. They are excluded from
// normal builds and from CI (no network, no keys, and they spend rate limit):
//
//	PEXELS_API_KEY=... PIXABAY_API_KEY=... go test -tags live -run Live -v ./internal/stockmedia/
//
// The unit tests use fixtures copied from the providers' documentation. These
// confirm the documentation still matches what the services return, which is
// the first thing to check when a provider seems to have changed.

func liveService(t *testing.T) *Service {
	t.Helper()
	if os.Getenv(EnvPexelsKey) == "" && os.Getenv(EnvPixabayKey) == "" {
		t.Skip("set PEXELS_API_KEY and/or PIXABAY_API_KEY to run live checks")
	}
	s, err := NewServiceFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestLive_SearchEveryProviderAndType(t *testing.T) {
	s := liveService(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	for _, name := range s.Providers() {
		for _, typ := range []MediaType{MediaPhoto, MediaVideo} {
			t.Run(string(name)+"/"+string(typ), func(t *testing.T) {
				res, err := s.Search(ctx, name, SearchParams{Query: "city night", Type: typ, PerPage: 10})
				if err != nil {
					t.Fatal(err)
				}
				t.Logf("total=%d returned=%d skipped=%d reasons=%v quota=%+v", res.Total, len(res.Assets), res.Skipped, res.SkipReasons, res.Quota)
				mix := map[Orientation]int{}
				for _, a := range res.Assets {
					mix[a.Orientation]++
				}
				t.Logf("orientation mix on this page: %v", mix)
				if len(res.Assets) == 0 {
					t.Fatalf("no usable assets: check SkipReasons above (a new CDN host shows as reason=host)")
				}
				for _, a := range res.Assets {
					if err := ValidateAsset(a); err != nil {
						t.Errorf("%s: %v", a.describe(), err)
					}
				}
				first := res.Assets[0]
				if err := s.CheckAvailable(ctx, first); err != nil {
					t.Errorf("first asset not available: %v", err)
				}
				t.Logf("first: %s %dx%d %s credit=%q", first.describe(), first.Width, first.Height, first.Orientation, first.Credit)
			})
		}
	}
}

func TestLive_InvalidKeyIsAnAuthError(t *testing.T) {
	liveService(t)
	pex, _ := NewPexels(ProviderConfig{APIKey: "not-a-real-key"})
	if _, err := pex.Search(context.Background(), SearchParams{Query: "x", Type: MediaPhoto}); !errors.Is(err, ErrAuth) {
		t.Errorf("pexels with a bogus key: err = %v, want ErrAuth", err)
	}
	// Pixabay answers a bad key with HTTP 400, which must still read as auth.
	pix, _ := NewPixabay(ProviderConfig{APIKey: "not-a-real-key"})
	if _, err := pix.Search(context.Background(), SearchParams{Query: "x", Type: MediaPhoto}); !errors.Is(err, ErrAuth) {
		t.Errorf("pixabay with a bogus key: err = %v, want ErrAuth", err)
	}
}

// TestLive_GetByIDRoundTrip looks up, by id, assets a search just returned, and
// checks the lookup agrees with the search. This is how a saved plan reopens its
// chosen media: it keeps only {provider, type, id}.
func TestLive_GetByIDRoundTrip(t *testing.T) {
	s := liveService(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	for _, name := range s.Providers() {
		for _, typ := range []MediaType{MediaPhoto, MediaVideo} {
			t.Run(string(name)+"/"+string(typ), func(t *testing.T) {
				res, err := s.Search(ctx, name, SearchParams{Query: "mountain lake", Type: typ, PerPage: 5})
				if err != nil || len(res.Assets) == 0 {
					t.Fatalf("search: %v (%d assets)", err, len(res.Assets))
				}
				want := res.Assets[0]
				got, err := s.Get(ctx, name, typ, want.ID)
				if err != nil {
					t.Fatalf("Get(%s): %v", want.ID, err)
				}
				if got.ID != want.ID || got.Type != typ || got.Width != want.Width || got.Height != want.Height {
					t.Errorf("lookup disagrees with search:\n got  %s %dx%d\n want %s %dx%d", got.ID, got.Width, got.Height, want.ID, want.Width, want.Height)
				}
				if got.Credit == "" || got.PageURL == "" || got.MediaURL == "" {
					t.Errorf("incomplete asset: %+v", got)
				}
				if typ == MediaVideo && got.PlayURL == "" {
					t.Error("a video needs a preview file")
				}
				if err := ValidateAsset(got); err != nil {
					t.Errorf("the looked-up asset fails validation: %v", err)
				}
				t.Logf("%s %s %dx%d play=%v", got.describe(), got.Orientation, got.Width, got.Height, got.PlayURL != "")
			})
		}
	}
	// A missing asset is a typed error, not a failure of the service.
	for _, name := range s.Providers() {
		if _, err := s.Get(ctx, name, MediaVideo, "1"); !errors.Is(err, ErrNotFound) {
			t.Errorf("%s: video 1: err = %v, want ErrNotFound", name, err)
		}
	}
}
