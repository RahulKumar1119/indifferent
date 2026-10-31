package stockcache

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/rahul/indifferent/backend/internal/stockmedia"
)

// pixabayStub stands in for the Pixabay API and counts how often it is asked.
// hd switches what it returns from 1280 px photos to 1920 px ones, which is what
// the real API does once the key has full access.
type pixabayStub struct {
	mu       sync.Mutex
	searches int
	gets     int
	hd       bool
}

func (p *pixabayStub) Name() stockmedia.ProviderName { return stockmedia.ProviderPixabay }

func (p *pixabayStub) asset(id string) stockmedia.Asset {
	size, long := "1280", 1280
	if p.hd {
		size, long = "1920", 1920
	}
	return stockmedia.Asset{
		Provider: stockmedia.ProviderPixabay, ID: id, Type: stockmedia.MediaPhoto,
		PageURL: "https://pixabay.com/photos/x-" + id + "/", PreviewURL: "https://pixabay.com/get/p" + id + "_640.jpg",
		MediaURL: "https://pixabay.com/get/m" + id + "_" + size + ".jpg", Format: "image/jpeg",
		Width: long * 9 / 16, Height: long, Orientation: stockmedia.OrientationPortrait,
		Author: "Jo", Credit: "Image by Jo on Pixabay",
	}
}

func (p *pixabayStub) Search(_ context.Context, sp stockmedia.SearchParams) (stockmedia.SearchResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.searches++
	return stockmedia.SearchResult{
		Provider: stockmedia.ProviderPixabay, Type: sp.Type, Total: 2, Page: sp.Page, PerPage: sp.PerPage,
		Assets:      []stockmedia.Asset{p.asset("1"), p.asset("2")},
		Attribution: stockmedia.Attribution{Text: "Images provided by Pixabay", URL: "https://pixabay.com"},
	}, nil
}

func (p *pixabayStub) Get(_ context.Context, _ stockmedia.MediaType, id string) (stockmedia.Asset, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.gets++
	return p.asset(id), nil
}

func (p *pixabayStub) calls() (searches, gets int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.searches, p.gets
}

func (p *pixabayStub) setHD(v bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.hd = v
}

// lambda is one container: its own Service and its own in-memory tier, over the
// shared table and the one provider.
func lambda(t *testing.T, db *fakeDB, clock *fakeClock, prov *pixabayStub) *stockmedia.Service {
	t.Helper()
	c, _ := container(t, db, clock)
	s, err := stockmedia.NewService([]stockmedia.Provider{prov}, stockmedia.WithCache(c),
		stockmedia.WithLogger(func(string, ...any) {}), stockmedia.WithRetry(stockmedia.RetryPolicy{}))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

var photos = stockmedia.SearchParams{Query: "Lighthouse at night", Type: stockmedia.MediaPhoto, PerPage: 20}

func TestTheProviderIsAskedOnceWhateverTheContainer(t *testing.T) {
	db, clock, prov := newFakeDB(), newClock(), &pixabayStub{}
	a, b, c := lambda(t, db, clock, prov), lambda(t, db, clock, prov), lambda(t, db, clock, prov)

	first, err := a.Search(bg, stockmedia.ProviderPixabay, photos)
	if err != nil || first.Cached || len(first.Assets) != 2 {
		t.Fatalf("first search: cached=%v assets=%d err=%v", first.Cached, len(first.Assets), err)
	}
	for name, s := range map[string]*stockmedia.Service{"second container": b, "third container": c} {
		got, err := s.Search(bg, stockmedia.ProviderPixabay, stockmedia.SearchParams{Query: "lighthouse AT night", Type: stockmedia.MediaPhoto, PerPage: 20})
		if err != nil || !got.Cached || len(got.Assets) != 2 || got.Assets[0] != first.Assets[0] {
			t.Errorf("%s: cached=%v assets=%d err=%v; want the first container's answer", name, got.Cached, len(got.Assets), err)
		}
	}
	if searches, _ := prov.calls(); searches != 1 {
		t.Errorf("provider searches = %d, want 1: Pixabay asks for requests to be cached across the whole service", searches)
	}
}

// The scenario in the comment on #194: the key is upgraded, but a 24 hour cache
// keeps serving 1280 px links, so the approval looks like a refusal for a day.
func TestAFlushMakesAnApprovedKeyShowUpAtOnceInsteadOfADayLater(t *testing.T) {
	db, clock, prov := newFakeDB(), newClock(), &pixabayStub{}
	a, b := lambda(t, db, clock, prov), lambda(t, db, clock, prov)
	res, _ := a.Search(bg, stockmedia.ProviderPixabay, photos)
	if res.Assets[0].Width != 720 {
		t.Fatalf("setup: expected 1280 px photos, got width %d", res.Assets[0].Width)
	}

	prov.setHD(true) // Pixabay approves the key
	clock.Advance(2 * time.Hour)
	if got, _ := b.Search(bg, stockmedia.ProviderPixabay, photos); got.Assets[0].Width != 720 || !got.Cached {
		t.Fatalf("setup: without a flush the old photos should still be served, got %+v", got.Assets[0])
	}

	db.setEpoch("2026-11-01T09:00:00Z") // deploy/flush-stockmedia-cache.sh
	clock.Advance(epochEvery)
	// The first container to search after the flush goes to Pixabay and gets the
	// 1920 px photos. Every other container, the one that served the old results
	// included, then gets those same fresh results from the shared cache.
	c := lambda(t, db, clock, prov)
	for i, step := range []struct {
		who        string
		s          *stockmedia.Service
		wantCached bool
	}{
		{"the container that served the old results", a, false},
		{"another container", b, true},
		{"a new container", c, true},
	} {
		got, err := step.s.Search(bg, stockmedia.ProviderPixabay, photos)
		if err != nil || got.Assets[0].Width != 1080 {
			t.Fatalf("%s: width=%d err=%v; want the fresh 1920 px results (width 1080 at 9:16)", step.who, got.Assets[0].Width, err)
		}
		if got.Cached != step.wantCached {
			t.Errorf("step %d, %s: cached=%v, want %v", i+1, step.who, got.Cached, step.wantCached)
		}
	}
	if searches, _ := prov.calls(); searches != 2 {
		t.Errorf("provider searches = %d, want 2: one before the flush, one after, shared by all three", searches)
	}
}

func TestAnEntryIsServedFor24HoursAndNoLonger(t *testing.T) {
	db, clock, prov := newFakeDB(), newClock(), &pixabayStub{}
	lambda(t, db, clock, prov).Search(bg, stockmedia.ProviderPixabay, photos)

	clock.Advance(24*time.Hour - time.Second)
	if got, _ := lambda(t, db, clock, prov).Search(bg, stockmedia.ProviderPixabay, photos); !got.Cached {
		t.Error("an entry 23 h 59 min 59 s old was not served")
	}
	clock.Advance(time.Second)
	if got, _ := lambda(t, db, clock, prov).Search(bg, stockmedia.ProviderPixabay, photos); got.Cached {
		t.Error("an entry 24 hours old was served again")
	}
	if searches, _ := prov.calls(); searches != 2 {
		t.Errorf("provider searches = %d, want 2", searches)
	}
}

func TestAssetsSeededBySearchStayInTheContainerThatSearched(t *testing.T) {
	db, clock, prov := newFakeDB(), newClock(), &pixabayStub{}
	a, b, c := lambda(t, db, clock, prov), lambda(t, db, clock, prov), lambda(t, db, clock, prov)
	a.Search(bg, stockmedia.ProviderPixabay, photos)

	if _, err := a.Get(bg, stockmedia.ProviderPixabay, stockmedia.MediaPhoto, "1"); err != nil {
		t.Fatal(err)
	}
	if _, gets := prov.calls(); gets != 0 {
		t.Errorf("picking an asset just seen cost %d provider lookups, want 0", gets)
	}
	// Another container has to ask once; what it learns is then shared.
	if _, err := b.Get(bg, stockmedia.ProviderPixabay, stockmedia.MediaPhoto, "1"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Get(bg, stockmedia.ProviderPixabay, stockmedia.MediaPhoto, "1"); err != nil {
		t.Fatal(err)
	}
	if _, gets := prov.calls(); gets != 1 {
		t.Errorf("provider lookups = %d for the same asset in three containers, want 1", gets)
	}
	// Seeds cost no table writes: one search entry, one lookup entry.
	if _, _, puts := db.counts(); puts != 2 {
		t.Errorf("table writes = %d, want 2 (the search, and the one lookup b made)", puts)
	}
}

// The per-asset entries a search seeds hold the same old links as the search did.
// If a flush left them, picking an asset would still give a 1280 px link for up
// to a day after Pixabay approved the key.
func TestAFlushAlsoDropsTheAssetsSeededBySearches(t *testing.T) {
	db, clock, prov := newFakeDB(), newClock(), &pixabayStub{}
	a := lambda(t, db, clock, prov)
	a.Search(bg, stockmedia.ProviderPixabay, photos)
	if got, _ := a.Get(bg, stockmedia.ProviderPixabay, stockmedia.MediaPhoto, "1"); got.Width != 720 {
		t.Fatalf("setup: expected the 1280 px asset, got width %d", got.Width)
	}

	prov.setHD(true)
	db.setEpoch("flushed")
	clock.Advance(epochEvery)

	got, err := a.Get(bg, stockmedia.ProviderPixabay, stockmedia.MediaPhoto, "1")
	if err != nil || got.Width != 1080 {
		t.Errorf("after the flush: width=%d err=%v; want the fresh 1920 px asset (width 1080), not the seeded one", got.Width, err)
	}
	if _, gets := prov.calls(); gets != 1 {
		t.Errorf("provider lookups = %d, want 1 (the flush made the lookup go to the provider)", gets)
	}
}

func TestABrokenTableLeavesSearchWorkingAsIfThereWereNoSharedCache(t *testing.T) {
	db, clock, prov := newFakeDB(), newClock(), &pixabayStub{}
	db.getErr, db.putErr = errors.New("AccessDeniedException"), errors.New("AccessDeniedException")
	a := lambda(t, db, clock, prov)
	for i := 0; i < 3; i++ {
		got, err := a.Search(bg, stockmedia.ProviderPixabay, photos)
		if err != nil || len(got.Assets) != 2 {
			t.Fatalf("search %d with a broken table: assets=%d err=%v", i, len(got.Assets), err)
		}
	}
	if searches, _ := prov.calls(); searches != 1 {
		t.Errorf("provider searches = %d, want 1: memory should still absorb repeats", searches)
	}
}

// A day of traffic: five containers, a few hundred searches over ten distinct
// phrases. With a shared cache the provider answers each phrase once; with one
// cache per container it would be asked up to five times as often.
func TestRealisticLoadCostsOneProviderCallPerDistinctSearch(t *testing.T) {
	const containers, searches, phrases = 5, 400, 10
	rng := rand.New(rand.NewSource(7))

	run := func(newService func(prov *pixabayStub) *stockmedia.Service) int {
		prov := &pixabayStub{}
		pool := make([]*stockmedia.Service, containers)
		for i := range pool {
			pool[i] = newService(prov)
		}
		for i := 0; i < searches; i++ {
			p := stockmedia.SearchParams{Query: fmt.Sprint("phrase ", rng.Intn(phrases)), Type: stockmedia.MediaPhoto, PerPage: 20}
			if _, err := pool[rng.Intn(containers)].Search(bg, stockmedia.ProviderPixabay, p); err != nil {
				t.Fatal(err)
			}
		}
		n, _ := prov.calls()
		return n
	}

	db, clock := newFakeDB(), newClock()
	shared := run(func(prov *pixabayStub) *stockmedia.Service { return lambda(t, db, clock, prov) })
	perProcess := run(func(prov *pixabayStub) *stockmedia.Service {
		s, err := stockmedia.NewService([]stockmedia.Provider{prov}, stockmedia.WithLogger(func(string, ...any) {}), stockmedia.WithRetry(stockmedia.RetryPolicy{}))
		if err != nil {
			t.Fatal(err)
		}
		return s
	})
	t.Logf("%d searches over %d phrases in %d containers: %d provider calls with the shared cache, %d with one cache per container", searches, phrases, containers, shared, perProcess)
	if shared != phrases {
		t.Errorf("shared cache: %d provider calls, want %d (one per distinct phrase)", shared, phrases)
	}
	if perProcess <= shared {
		t.Errorf("control: per-container caches made %d calls, no more than the shared cache's %d, so this test shows nothing", perProcess, shared)
	}
}
