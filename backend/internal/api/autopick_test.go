package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"log"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rahul/indifferent/backend/internal/stockmedia"
	"github.com/rahul/indifferent/backend/internal/storymetrics"
	"github.com/rahul/indifferent/backend/internal/storyplan"
)

// ctxStock is a StockMedia whose search sees the context, for budget and
// concurrency tests. Lookups return the asset they are asked for.
type ctxStock struct {
	mu        sync.Mutex
	search    func(ctx context.Context, p stockmedia.SearchParams) (stockmedia.SearchResult, error)
	providers []stockmedia.ProviderName
	searches  []stockmedia.SearchParams
	reserves  []bool // whether each search carried a quota reserve
}

func (f *ctxStock) Providers() []stockmedia.ProviderName { return f.providers }

func (f *ctxStock) Search(ctx context.Context, name stockmedia.ProviderName, p stockmedia.SearchParams) (stockmedia.SearchResult, error) {
	if name != stockmedia.ProviderPixabay {
		return stockmedia.SearchResult{}, fmt.Errorf("auto-pick asked %s", name)
	}
	f.mu.Lock()
	f.searches = append(f.searches, p)
	f.mu.Unlock()
	return f.search(ctx, p)
}

func (f *ctxStock) SearchAll(context.Context, stockmedia.SearchParams) (stockmedia.MultiResult, error) {
	return stockmedia.MultiResult{}, errors.New("auto-pick must not search every provider")
}

func (f *ctxStock) GetMany(_ context.Context, refs []stockmedia.AssetRef) []stockmedia.AssetResult {
	out := make([]stockmedia.AssetResult, len(refs))
	for i, r := range refs {
		a := stockAsset(r.Provider, r.Type, r.ID, 1080, 1920)
		out[i] = stockmedia.AssetResult{Ref: r, Asset: &a}
	}
	return out
}

func (f *ctxStock) count() int { f.mu.Lock(); defer f.mu.Unlock(); return len(f.searches) }

// byQuery answers each search with assets whose ids start with the query, so
// every scene gets its own.
func byQuery(n int) func(context.Context, stockmedia.SearchParams) (stockmedia.SearchResult, error) {
	return func(_ context.Context, p stockmedia.SearchParams) (stockmedia.SearchResult, error) {
		var as []stockmedia.Asset
		for i := 0; i < n; i++ {
			id := idFor(p.Query, i)
			as = append(as, stockAsset(stockmedia.ProviderPixabay, p.Type, id, 1080, 1920))
		}
		return stockmedia.SearchResult{Provider: stockmedia.ProviderPixabay, Type: p.Type, Assets: as}, nil
	}
}

// idFor is the numeric id (as Pixabay's are) of result i for a query.
func idFor(q string, i int) string {
	h := fnv.New32a()
	h.Write([]byte(q))
	return fmt.Sprintf("%d%02d", h.Sum32(), i)
}

func scenesPlan(n int) *storyplan.Plan {
	p := &storyplan.Plan{Language: "en"}
	for i := 0; i < n; i++ {
		typ := storyplan.MediaPhoto
		if i%2 == 0 {
			typ = storyplan.MediaVideo
		}
		p.Scenes = append(p.Scenes, storyplan.Scene{
			Narration: "a line", Keywords: []string{fmt.Sprintf("scene%d", i)}, MediaType: typ, Duration: 5,
		})
	}
	return p
}

func autoHandler(fs *ctxStock) (*APIHandler, *bytes.Buffer) {
	var metrics bytes.Buffer
	return &APIHandler{AutoPick: true, StockMedia: fs, Metrics: storymetrics.New(&metrics)}, &metrics
}

func pixabayOnly(search func(context.Context, stockmedia.SearchParams) (stockmedia.SearchResult, error)) *ctxStock {
	return &ctxStock{providers: []stockmedia.ProviderName{stockmedia.ProviderPixabay}, search: search}
}

func TestAutoPick_OffOrUnconfiguredSearchesNothing(t *testing.T) {
	fs := pixabayOnly(byQuery(3))
	for name, h := range map[string]*APIHandler{
		"flag off":        {StockMedia: fs},
		"no stock":        {AutoPick: true},
		"pexels only":     {AutoPick: true, StockMedia: &ctxStock{providers: []stockmedia.ProviderName{stockmedia.ProviderPexels}, search: byQuery(3)}},
		"no stock at all": {},
	} {
		plan := scenesPlan(3)
		if s := h.autoPick(context.Background(), plan, time.Now()); s != nil {
			t.Errorf("%s: summary = %+v, want nil", name, s)
		}
		for i, sc := range plan.Scenes {
			if sc.Media != nil || sc.MediaSource != "" {
				t.Errorf("%s: scene %d = %+v", name, i+1, sc)
			}
		}
	}
	if fs.count() != 0 {
		t.Errorf("searched %d times", fs.count())
	}
}

func TestAutoPick_PicksEveryScene_WithThePickersOpeningSearch(t *testing.T) {
	fs := pixabayOnly(byQuery(3))
	h, metrics := autoHandler(fs)
	plan := scenesPlan(4)
	s := h.autoPick(context.Background(), plan, time.Now())
	if s == nil || s.Picked != 4 || s.Empty != 0 {
		t.Fatalf("summary = %+v", s)
	}
	for i, sc := range plan.Scenes {
		if sc.Media == nil || sc.MediaSource != storyplan.MediaSourceAuto {
			t.Fatalf("scene %d = %+v", i+1, sc)
		}
		if sc.Media.Provider != "pixabay" || sc.Media.Type != sc.MediaType || sc.Media.ID != idFor(fmt.Sprintf("scene%d", i), 0) {
			t.Errorf("scene %d media = %+v", i+1, sc.Media)
		}
	}
	for _, p := range fs.searches {
		if p.Page != 1 || p.PerPage != 12 || p.Orientation != stockmedia.OrientationPortrait {
			t.Errorf("not the picker's opening search: %+v", p)
		}
	}
	if !strings.Contains(metrics.String(), `"PicturesAutoPicked":4`) {
		t.Errorf("metrics = %s", metrics.String())
	}
}

// Requirement 3 and 6: a scene whose search failed, found nothing, or was held
// back for people stays empty and unmarked, and the rest are still picked.
func TestAutoPick_FailuresLeaveOnlyThoseScenesEmpty(t *testing.T) {
	fs := pixabayOnly(func(ctx context.Context, p stockmedia.SearchParams) (stockmedia.SearchResult, error) {
		switch p.Query {
		case "scene0":
			return stockmedia.SearchResult{}, &stockmedia.ProviderError{Provider: stockmedia.ProviderPixabay, Kind: stockmedia.KindUnavailable}
		case "scene1":
			return stockmedia.SearchResult{Provider: stockmedia.ProviderPixabay}, nil
		case "scene2":
			return stockmedia.SearchResult{}, fmt.Errorf("x: %w", stockmedia.ErrQuotaReserved)
		case "scene3":
			return stockmedia.SearchResult{}, &stockmedia.ProviderError{Provider: stockmedia.ProviderPixabay, Kind: stockmedia.KindRateLimited}
		case "scene4":
			return stockmedia.SearchResult{}, fmt.Errorf("x: %w", stockmedia.ErrCoolingDown)
		}
		return byQuery(2)(ctx, p)
	})
	h, metrics := autoHandler(fs)
	plan := scenesPlan(6)
	s := h.autoPick(context.Background(), plan, time.Now())
	if s.Picked != 1 || s.Empty != 5 {
		t.Fatalf("summary = %+v", s)
	}
	want := map[string]int{"error": 1, "no_results": 1, "quota": 3}
	for k, v := range want {
		if s.Reasons[k] != v {
			t.Errorf("reasons = %v, want %v", s.Reasons, want)
			break
		}
	}
	for i := 0; i < 5; i++ {
		if sc := plan.Scenes[i]; sc.Media != nil || sc.MediaSource != "" {
			t.Errorf("scene %d should be empty and unmarked: %+v", i+1, sc)
		}
	}
	if sc := plan.Scenes[5]; sc.Media == nil || sc.MediaSource != storyplan.MediaSourceAuto {
		t.Errorf("scene 6 = %+v", sc)
	}
	if !strings.Contains(metrics.String(), `"AutoPickScenesEmpty":3`) || !strings.Contains(metrics.String(), `"Code":"quota"`) {
		t.Errorf("metrics = %s", metrics.String())
	}
}

func TestAutoPick_NoPictureIsUsedTwiceAndTheChoiceIsInSceneOrder(t *testing.T) {
	same := []stockmedia.Asset{
		stockAsset(stockmedia.ProviderPixabay, stockmedia.MediaPhoto, "1", 1080, 1920),
		stockAsset(stockmedia.ProviderPixabay, stockmedia.MediaPhoto, "2", 1080, 1920),
	}
	fs := pixabayOnly(func(_ context.Context, p stockmedia.SearchParams) (stockmedia.SearchResult, error) {
		return stockmedia.SearchResult{Provider: stockmedia.ProviderPixabay, Assets: same}, nil
	})
	h, _ := autoHandler(fs)
	plan := &storyplan.Plan{Scenes: []storyplan.Scene{
		{Keywords: []string{"a"}, MediaType: "photo", Duration: 5},
		{Keywords: []string{"b"}, MediaType: "photo", Duration: 5},
		{Keywords: []string{"c"}, MediaType: "photo", Duration: 5},
	}}
	s := h.autoPick(context.Background(), plan, time.Now())
	if plan.Scenes[0].Media.ID != "1" || plan.Scenes[1].Media.ID != "2" || plan.Scenes[2].Media != nil {
		t.Errorf("scenes = %+v", plan.Scenes)
	}
	if s.Picked != 2 || s.Reasons["no_results"] != 1 {
		t.Errorf("summary = %+v", s)
	}
}

func TestAutoPick_AVideoSceneGetsAClipLongEnoughWhenThereIsOne(t *testing.T) {
	clip := func(id string, secs float64) stockmedia.Asset {
		a := stockAsset(stockmedia.ProviderPixabay, stockmedia.MediaVideo, id, 1080, 1920)
		a.Duration = secs
		return a
	}
	fs := pixabayOnly(func(_ context.Context, p stockmedia.SearchParams) (stockmedia.SearchResult, error) {
		if p.Query == "short only" {
			return stockmedia.SearchResult{Assets: []stockmedia.Asset{clip("31", 2), clip("32", 3)}}, nil
		}
		return stockmedia.SearchResult{Assets: []stockmedia.Asset{clip("41", 3), clip("42", 9)}}, nil
	})
	h, _ := autoHandler(fs)
	plan := &storyplan.Plan{Scenes: []storyplan.Scene{
		{Keywords: []string{"sea"}, MediaType: "video", Duration: 6},
		{Keywords: []string{"short only"}, MediaType: "video", Duration: 6},
	}}
	h.autoPick(context.Background(), plan, time.Now())
	if plan.Scenes[0].Media.ID != "42" {
		t.Errorf("scene 1 got %s, want the clip long enough", plan.Scenes[0].Media.ID)
	}
	if plan.Scenes[1].Media == nil || plan.Scenes[1].Media.ID != "31" {
		t.Errorf("scene 2 = %+v, want the first clip when none is long enough", plan.Scenes[1].Media)
	}
}

// Requirement 2: the budget is 4 seconds, and less when the request is close to
// its 26 second deadline. A search still running then is given up.
func TestAutoPick_StopsAtTheDeadline(t *testing.T) {
	fs := pixabayOnly(func(ctx context.Context, p stockmedia.SearchParams) (stockmedia.SearchResult, error) {
		if p.Query == "scene0" {
			return byQuery(1)(ctx, p)
		}
		<-ctx.Done() // a provider that never answers
		return stockmedia.SearchResult{}, ctx.Err()
	})
	h, _ := autoHandler(fs)
	plan := scenesPlan(5)
	// The request began 25.8 s ago: 200 ms are left of the 26.
	began := time.Now()
	s := h.autoPick(context.Background(), plan, began.Add(-autoPickDeadline+200*time.Millisecond))
	took := time.Since(began)
	if took < 150*time.Millisecond || took > time.Second {
		t.Errorf("took %v, want about 200ms", took)
	}
	if s.Picked != 1 || s.Reasons["budget"] != 4 {
		t.Errorf("summary = %+v", s)
	}
	for i := 1; i < 5; i++ {
		if plan.Scenes[i].Media != nil || plan.Scenes[i].MediaSource != "" {
			t.Errorf("scene %d = %+v", i+1, plan.Scenes[i])
		}
	}
}

func TestAutoPick_PastTheDeadlineSearchesNothing(t *testing.T) {
	fs := pixabayOnly(byQuery(1))
	h, _ := autoHandler(fs)
	plan := scenesPlan(3)
	s := h.autoPick(context.Background(), plan, time.Now().Add(-27*time.Second))
	if fs.count() != 0 || s.Empty != 3 || s.Reasons["budget"] != 3 {
		t.Errorf("searches = %d, summary = %+v", fs.count(), s)
	}
}

// A person closing the tab does not cut the searches short: their results are
// cached for the picker, and the budget bounds them anyway.
func TestAutoPick_ACancelledRequestStillFinishesWithinTheBudget(t *testing.T) {
	fs := pixabayOnly(func(ctx context.Context, p stockmedia.SearchParams) (stockmedia.SearchResult, error) {
		time.Sleep(20 * time.Millisecond)
		if ctx.Err() != nil {
			return stockmedia.SearchResult{}, ctx.Err()
		}
		return byQuery(1)(ctx, p)
	})
	h, _ := autoHandler(fs)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s := h.autoPick(ctx, scenesPlan(2), time.Now())
	if s.Picked != 2 {
		t.Errorf("summary = %+v", s)
	}
}

// Requirement 1 and 7: at most 3 searches at once for one plan, with several
// plans made at the same time.
func TestAutoPick_AtMostThreeSearchesAtOncePerPlan_UnderConcurrentPlans(t *testing.T) {
	const plans = 4
	var mu sync.Mutex
	inFlight := map[string]int{} // plan -> searches running now
	maxSeen := map[string]int{}
	var total, maxTotal atomic.Int64
	fs := pixabayOnly(func(ctx context.Context, p stockmedia.SearchParams) (stockmedia.SearchResult, error) {
		plan := strings.SplitN(p.Query, " ", 2)[0]
		mu.Lock()
		inFlight[plan]++
		if inFlight[plan] > maxSeen[plan] {
			maxSeen[plan] = inFlight[plan]
		}
		mu.Unlock()
		if n := total.Add(1); n > maxTotal.Load() {
			maxTotal.Store(n)
		}
		time.Sleep(30 * time.Millisecond)
		total.Add(-1)
		mu.Lock()
		inFlight[plan]--
		mu.Unlock()
		return byQuery(1)(ctx, p)
	})
	h, _ := autoHandler(fs)
	var wg sync.WaitGroup
	results := make([]*autoPickSummary, plans)
	for k := 0; k < plans; k++ {
		wg.Add(1)
		go func(k int) {
			defer wg.Done()
			plan := &storyplan.Plan{}
			for i := 0; i < 10; i++ {
				plan.Scenes = append(plan.Scenes, storyplan.Scene{
					Keywords: []string{fmt.Sprintf("plan%d scene%d", k, i)}, MediaType: "photo", Duration: 5,
				})
			}
			results[k] = h.autoPick(context.Background(), plan, time.Now())
		}(k)
	}
	wg.Wait()
	for k, s := range results {
		if s.Picked != 10 {
			t.Errorf("plan %d: %+v", k, s)
		}
	}
	for plan, n := range maxSeen {
		if n > autoPickConcurrency {
			t.Errorf("%s ran %d searches at once, more than %d", plan, n, autoPickConcurrency)
		}
	}
	if maxTotal.Load() <= autoPickConcurrency {
		t.Logf("plans did not overlap (max %d at once); the per-plan bound was still checked", maxTotal.Load())
	}
	if fs.count() != plans*10 {
		t.Errorf("searches = %d", fs.count())
	}
}

// Requirement 5: the log line has counts and reason codes, never search words.
func TestAutoPick_TheLogHasNoSearchWords(t *testing.T) {
	var logs bytes.Buffer
	log.SetOutput(&logs)
	defer log.SetOutput(os.Stderr)
	fs := pixabayOnly(func(ctx context.Context, p stockmedia.SearchParams) (stockmedia.SearchResult, error) {
		if p.Query == "secretword" {
			return stockmedia.SearchResult{}, errors.New("provider said: secretword failed")
		}
		return byQuery(1)(ctx, p)
	})
	h, _ := autoHandler(fs)
	plan := &storyplan.Plan{Scenes: []storyplan.Scene{
		{Keywords: []string{"secretword"}, MediaType: "photo", Duration: 5},
		{Keywords: []string{"lighthouse"}, MediaType: "photo", Duration: 5},
	}}
	h.autoPick(context.Background(), plan, time.Now())
	out := logs.String()
	if !strings.Contains(out, "storyplan_autopick scenes=2 picked=1 empty=1 reasons=error:1") {
		t.Errorf("log = %q", out)
	}
	for _, word := range []string{"secretword", "lighthouse"} {
		if strings.Contains(out, word) {
			t.Errorf("the log has the search word %q: %q", word, out)
		}
	}
}

// End to end through POST /story-plans: the plan comes back with its pictures
// marked and shown, and the record keeps the counts.
func TestCreatePlan_AutoPickEndToEnd(t *testing.T) {
	e := newSPEnv(t)
	fs := pixabayOnly(byQuery(2))
	e.h.StockMedia = fs
	e.h.AutoPick = true
	resp := e.do(t, "POST", "/story-plans", storyBody(spEnglish), "u1")
	if resp.StatusCode != 201 {
		t.Fatalf("status %d: %s", resp.StatusCode, resp.Body)
	}
	var got StoryPlanResponse
	if err := json.Unmarshal([]byte(resp.Body), &got); err != nil {
		t.Fatal(err)
	}
	for i, sc := range got.Plan.Scenes {
		if sc.Media == nil || sc.MediaSource != "auto" {
			t.Errorf("scene %d = %+v", i+1, sc)
		}
	}
	if len(got.Media) != len(got.Plan.Scenes) || got.Media[0] == nil || got.Media[0].Asset == nil {
		t.Errorf("media not attached: %+v", got.Media)
	}
	if strings.Contains(resp.Body, "SECRET-ORIGINAL") {
		t.Error("the response has a full-resolution URL")
	}
	rec := e.storedPlan(t, "u1", got.PlanID)
	if rec.AutoPick == nil || rec.AutoPick.Picked != 2 {
		t.Errorf("stored autoPick = %+v", rec.AutoPick)
	}
}

// Requirement 6: auto-pick failing does not fail the plan.
func TestCreatePlan_AutoPickFailingStillMakesThePlan(t *testing.T) {
	e := newSPEnv(t)
	e.h.StockMedia = pixabayOnly(func(context.Context, stockmedia.SearchParams) (stockmedia.SearchResult, error) {
		return stockmedia.SearchResult{}, fmt.Errorf("x: %w", stockmedia.ErrQuotaReserved)
	})
	e.h.AutoPick = true
	resp := e.do(t, "POST", "/story-plans", storyBody(spEnglish), "u1")
	if resp.StatusCode != 201 {
		t.Fatalf("status %d: %s", resp.StatusCode, resp.Body)
	}
	var got StoryPlanResponse
	json.Unmarshal([]byte(resp.Body), &got)
	for i, sc := range got.Plan.Scenes {
		if sc.Media != nil || sc.MediaSource != "" {
			t.Errorf("scene %d = %+v", i+1, sc)
		}
	}
	if got.Media != nil {
		t.Errorf("media = %+v", got.Media)
	}
	rec := e.storedPlan(t, "u1", got.PlanID)
	if rec.AutoPick == nil || rec.AutoPick.Empty != 2 || rec.AutoPick.Reasons["quota"] != 2 {
		t.Errorf("stored autoPick = %+v", rec.AutoPick)
	}
}

// Requirement 4: swapping or removing an auto-picked picture clears the mark,
// keeping it unchanged keeps it, and the counts survive the save.
func TestUpdatePlan_AutoPickMarkIsClearedOnReplaceOrRemove(t *testing.T) {
	e := newSPEnv(t)
	plan := samplePlan("en")
	plan.Scenes = append(plan.Scenes, storyplan.Scene{
		Narration: plan.Scenes[0].Narration, Description: "x", Keywords: []string{"net"}, MediaType: "photo", Duration: 4,
	})
	plan.TotalSeconds = 15
	e.planner.res.Plan = plan
	fs := pixabayOnly(byQuery(2))
	e.h.StockMedia = fs
	e.h.AutoPick = true
	var metrics bytes.Buffer
	e.h.Metrics = storymetrics.New(&metrics)
	resp := e.do(t, "POST", "/story-plans", storyBody(spEnglish), "u1")
	var created StoryPlanResponse
	json.Unmarshal([]byte(resp.Body), &created)
	if len(created.Plan.Scenes) != 3 {
		t.Fatalf("created = %s", resp.Body)
	}
	keep := *created.Plan.Scenes[0].Media
	swap := storyplan.MediaRef{Provider: "pixabay", Type: "photo", ID: "999"}
	body, _ := json.Marshal(UpdateStoryPlanRequest{Revision: 1, Scenes: []storyplan.SceneEdit{
		{Media: &keep},
		{Media: &swap},
		{ClearMedia: true},
	}})
	metrics.Reset()
	resp = e.do(t, "PUT", "/story-plans/"+created.PlanID, string(body), "u1")
	if resp.StatusCode != 200 {
		t.Fatalf("status %d: %s", resp.StatusCode, resp.Body)
	}
	var saved StoryPlanResponse
	json.Unmarshal([]byte(resp.Body), &saved)
	sc := saved.Plan.Scenes
	if sc[0].MediaSource != "auto" || *sc[0].Media != keep {
		t.Errorf("unchanged pick lost its mark: %+v", sc[0])
	}
	if sc[1].MediaSource != "" || *sc[1].Media != swap {
		t.Errorf("swapped pick still marked: %+v", sc[1])
	}
	if sc[2].Media != nil || sc[2].MediaSource != "" {
		t.Errorf("removed pick still there: %+v", sc[2])
	}
	m := metrics.String()
	for _, want := range []string{`"AutoPicksReplaced":1`, `"AutoPicksRemoved":1`, `"PicturesChosen":1`} {
		if !strings.Contains(m, want) {
			t.Errorf("metrics lack %s: %s", want, m)
		}
	}
	for _, unwanted := range []string{`"PicturesReplaced"`, `"PicturesRemoved"`} {
		if strings.Contains(m, unwanted) {
			t.Errorf("an auto-picked picture counted as the user's: %s", m)
		}
	}
	rec := e.storedPlan(t, "u1", created.PlanID)
	if rec.AutoPick == nil || rec.AutoPick.Picked != 3 {
		t.Errorf("counts lost in the save: %+v", rec.AutoPick)
	}
	// A client cannot mark its own choice as auto-picked: the field is not
	// part of an edit, and a stored mark only survives an unchanged pick.
	if rec.Plan.Scenes[1].MediaSource != "" {
		t.Errorf("stored scene 2 = %+v", rec.Plan.Scenes[1])
	}
}

func TestStoryPlanRecord_AutoPickRoundTrips(t *testing.T) {
	in := storyPlanRecord{UserID: "u", PlanID: "p", Status: "ready", AutoPick: &autoPickSummary{
		Picked: 5, Empty: 2, Reasons: map[string]int{"quota": 1, "budget": 1}, Ms: 1234,
	}}
	item, err := storyPlanToItem(in)
	if err != nil {
		t.Fatal(err)
	}
	out := itemToStoryPlan(item)
	if out.AutoPick == nil || out.AutoPick.Picked != 5 || out.AutoPick.Empty != 2 || out.AutoPick.Reasons["budget"] != 1 || out.AutoPick.Ms != 1234 {
		t.Errorf("round trip = %+v", out.AutoPick)
	}
	none, _ := storyPlanToItem(storyPlanRecord{UserID: "u", PlanID: "p"})
	if _, has := none["autoPick"]; has {
		t.Error("a plan made without auto-pick has an autoPick attribute")
	}
}

// Requirement 7, against the real stock service: once Pixabay reports fewer
// than 30 requests left, auto-pick stops calling it while a person's picker
// search still goes through.
func TestAutoPick_YieldsToThePickerWhenQuotaIsLow(t *testing.T) {
	prov := &quotaProvider{remaining: 29}
	svc, err := stockmedia.NewService([]stockmedia.Provider{prov},
		stockmedia.WithLogger(func(string, ...any) {}),
		stockmedia.WithURLPolicy(func(stockmedia.ProviderName, *url.URL) error { return nil }))
	if err != nil {
		t.Fatal(err)
	}
	e := newSPEnv(t)
	e.h.StockMedia = svc
	e.h.AutoPick = true

	// A person searches first: the provider reports 29 left.
	r := e.get(t, "/stock-media/search", "u1", map[string]string{"q": "harbour", "type": "photo", "provider": "pixabay"})
	if r.StatusCode != 200 {
		t.Fatalf("picker search: %d %s", r.StatusCode, r.Body)
	}
	before := prov.calls.Load()

	// A plan is made: auto-pick holds back.
	resp := e.do(t, "POST", "/story-plans", storyBody(spEnglish), "u2")
	if resp.StatusCode != 201 {
		t.Fatalf("plan: %d %s", resp.StatusCode, resp.Body)
	}
	if n := prov.calls.Load() - before; n != 0 {
		t.Errorf("auto-pick called Pixabay %d times under the reserve", n)
	}
	var got StoryPlanResponse
	json.Unmarshal([]byte(resp.Body), &got)
	for i, sc := range got.Plan.Scenes {
		if sc.Media != nil {
			t.Errorf("scene %d was picked under the reserve: %+v", i+1, sc.Media)
		}
	}

	// And the picker still gets through.
	r = e.get(t, "/stock-media/search", "u1", map[string]string{"q": "boat", "type": "photo", "provider": "pixabay"})
	if r.StatusCode != 200 || prov.calls.Load() != before+1 {
		t.Errorf("picker search after the plan: %d, calls %d", r.StatusCode, prov.calls.Load()-before)
	}
}

// quotaProvider is a Pixabay that always reports remaining requests left.
type quotaProvider struct {
	remaining int
	calls     atomic.Int64
}

func (p *quotaProvider) Name() stockmedia.ProviderName { return stockmedia.ProviderPixabay }

func (p *quotaProvider) Search(_ context.Context, sp stockmedia.SearchParams) (stockmedia.SearchResult, error) {
	p.calls.Add(1)
	a := stockAsset(stockmedia.ProviderPixabay, sp.Type, fmt.Sprintf("%d", 5000+p.calls.Load()), 1080, 1920)
	return stockmedia.SearchResult{
		Provider: stockmedia.ProviderPixabay, Type: sp.Type, Assets: []stockmedia.Asset{a}, Total: 1,
		Quota: &stockmedia.Quota{Remaining: p.remaining, ResetAt: time.Now().Add(time.Minute)},
	}, nil
}

func (p *quotaProvider) Get(_ context.Context, typ stockmedia.MediaType, id string) (stockmedia.Asset, error) {
	p.calls.Add(1)
	return stockAsset(stockmedia.ProviderPixabay, typ, id, 1080, 1920), nil
}

// storedPlan reads a plan record straight from the test table.
func (e *spEnv) storedPlan(t *testing.T, user, planID string) storyPlanRecord {
	t.Helper()
	e.mu.Lock()
	defer e.mu.Unlock()
	item, ok := e.items["USER#"+user+"|"+storyPlanPrefix+planID]
	if !ok {
		t.Fatalf("no stored plan %s", planID)
	}
	return itemToStoryPlan(item)
}

func TestLooksSensitive(t *testing.T) {
	for _, q := range []string{
		"contact ravi@example.com",
		"follow @ravi_k on instagram",
		"visit https://shop.example",
		"www.myshop",
		"myshop.com sale",
		"mybakery.in cakes",
		"call 98765 43210",
		"+91 98765-43210",
		"(022) 2345 6789",
		"phone 555.123.4567",
		"order 1234567",
		"account 123456",
		"card 4111 1111 1111 1111",
		"aadhaar 1234 5678 9012",
		"नंबर ९८७६५४३२१०",
	} {
		if !looksSensitive(q) {
			t.Errorf("%q was not flagged", q)
		}
	}
	for _, q := range []string{
		"fishing boat sunrise",
		"1920s street",
		"2 dogs running",
		"city at night",
		"year 2024 festival",
		"top 10 beaches",
		"4k ocean waves",
		"3 friends 2 cats",
		"मछुआरा नाव",
		"केरल बैकवाटर",
		"mr. sharma at home",
		"st. mary church",
		"",
	} {
		if looksSensitive(q) {
			t.Errorf("%q was flagged", q)
		}
	}
}

// The comment on PR #307: a scene whose words look like personal data is not
// sent to Pixabay; the others still are, and the log says only why.
func TestAutoPick_ASensitiveSceneIsNotSearched(t *testing.T) {
	var logs bytes.Buffer
	log.SetOutput(&logs)
	defer log.SetOutput(os.Stderr)
	fs := pixabayOnly(byQuery(1))
	h, metrics := autoHandler(fs)
	plan := &storyplan.Plan{Scenes: []storyplan.Scene{
		{Keywords: []string{"harbour"}, MediaType: "photo", Duration: 5},
		{Keywords: []string{"call ravi", "98765 43210"}, MediaType: "photo", Duration: 5},
		{Keywords: []string{"ravi@example.com"}, MediaType: "photo", Duration: 5},
		{Keywords: []string{"lantern"}, MediaType: "photo", Duration: 5},
	}}
	s := h.autoPick(context.Background(), plan, time.Now())
	if s.Picked != 2 || s.Empty != 2 || s.Reasons["skipped_sensitive"] != 2 {
		t.Errorf("summary = %+v", s)
	}
	for _, p := range fs.searches {
		if strings.Contains(p.Query, "ravi") || strings.Contains(p.Query, "98765") {
			t.Errorf("searched for %q", p.Query)
		}
	}
	if fs.count() != 2 {
		t.Errorf("searches = %d, want 2", fs.count())
	}
	for _, i := range []int{1, 2} {
		if sc := plan.Scenes[i]; sc.Media != nil || sc.MediaSource != "" {
			t.Errorf("scene %d = %+v", i+1, sc)
		}
	}
	out := logs.String()
	if !strings.Contains(out, "skipped_sensitive:2") {
		t.Errorf("log = %q", out)
	}
	for _, word := range []string{"ravi", "98765", "example.com", "harbour", "lantern"} {
		if strings.Contains(out, word) {
			t.Errorf("the log has %q: %q", word, out)
		}
	}
	if !strings.Contains(metrics.String(), `"Code":"skipped_sensitive"`) {
		t.Errorf("metrics = %s", metrics.String())
	}
}

// A sensitive scene keeps its own reason even when time runs out first.
func TestAutoPick_ASensitiveSceneIsReportedAsSuchPastTheDeadline(t *testing.T) {
	fs := pixabayOnly(byQuery(1))
	h, _ := autoHandler(fs)
	plan := &storyplan.Plan{Scenes: []storyplan.Scene{
		{Keywords: []string{"harbour"}, MediaType: "photo", Duration: 5},
		{Keywords: []string{"www.example"}, MediaType: "photo", Duration: 5},
	}}
	s := h.autoPick(context.Background(), plan, time.Now().Add(-autoPickDeadline+50*time.Millisecond))
	if s.Reasons["skipped_sensitive"] != 1 {
		t.Errorf("summary = %+v", s)
	}
}
