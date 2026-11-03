package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	dbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/rahul/indifferent/backend/internal/stockmedia"
	"github.com/rahul/indifferent/backend/internal/storyjob"
	"github.com/rahul/indifferent/backend/internal/storymetrics"
	"github.com/rahul/indifferent/backend/internal/storyplan"
)

// metricsSink collects the lines an Emitter writes.
type metricsSink struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (m *metricsSink) Write(p []byte) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.buf.Write(p)
}

// recorded is what the metrics said, as name -> total, and the Code of each failure.
type recorded struct {
	totals map[string]float64
	codes  []string
	lines  int
}

func (m *metricsSink) read(t *testing.T) recorded {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()
	r := recorded{totals: map[string]float64{}}
	for _, l := range strings.Split(strings.TrimSpace(m.buf.String()), "\n") {
		if l == "" {
			continue
		}
		r.lines++
		var root map[string]any
		if err := json.Unmarshal([]byte(l), &root); err != nil {
			t.Fatalf("a metric line is not JSON: %q", l)
		}
		dir := root["_aws"].(map[string]any)["CloudWatchMetrics"].([]any)[0].(map[string]any)
		for _, md := range dir["Metrics"].([]any) {
			name := md.(map[string]any)["Name"].(string)
			r.totals[name] += root[name].(float64)
		}
		if c, ok := root[storymetrics.CodeDimension].(string); ok {
			r.codes = append(r.codes, c)
		}
	}
	return r
}

func (m *metricsSink) equal(t *testing.T, want map[string]float64) {
	t.Helper()
	got := m.read(t).totals
	if len(got) != len(want) {
		t.Errorf("metrics = %v, want %v", got, want)
		return
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("metrics = %v, want %v", got, want)
			return
		}
	}
}

func withMetrics(h *APIHandler) *metricsSink {
	s := &metricsSink{}
	h.Metrics = storymetrics.New(s)
	return s
}

// ---------------------------------------------------------------- plans

func TestMetrics_AReadyPlanIsCountedOnceAndOnlyWhenItIsSaved(t *testing.T) {
	e := newSPEnv(t)
	m := withMetrics(e.h)
	if resp := e.do(t, "POST", "/story-plans", storyBody(spEnglish), "user1"); resp.StatusCode != 201 {
		t.Fatalf("%d %s", resp.StatusCode, resp.Body)
	}
	m.equal(t, map[string]float64{storymetrics.PlansReady: 1})
}

func TestMetrics_APlanThatCouldNotBeSavedIsNotCountedReady(t *testing.T) {
	e := newSPEnv(t)
	m := withMetrics(e.h)
	e.putErr = func(n int) error { // the first write records the attempt; the second saves the plan
		if n == 2 {
			return errors.New("dynamodb down")
		}
		return nil
	}
	if resp := e.do(t, "POST", "/story-plans", storyBody(spEnglish), "user1"); resp.StatusCode != 500 {
		t.Fatalf("%d %s", resp.StatusCode, resp.Body)
	}
	m.equal(t, map[string]float64{})
}

func TestMetrics_APlannerFailureIsCountedButARefusedStoryIsNot(t *testing.T) {
	for name, err := range map[string]error{
		"timeout":       storyplan.ErrModelTimeout,
		"model failed":  storyplan.ErrModelFailed,
		"unusable plan": storyplan.ErrInvalidPlan,
		"other":         errors.New("boom"),
	} {
		t.Run(name, func(t *testing.T) {
			e := newSPEnv(t)
			m := withMetrics(e.h)
			e.planner.err = err
			e.do(t, "POST", "/story-plans", storyBody(spEnglish), "user1")
			m.equal(t, map[string]float64{storymetrics.PlansFailed: 1})
		})
	}

	t.Run("a story too short to plan never reached the planner", func(t *testing.T) {
		e := newSPEnv(t)
		m := withMetrics(e.h)
		if resp := e.do(t, "POST", "/story-plans", storyBody("too short"), "user1"); resp.StatusCode != 400 {
			t.Fatalf("%d", resp.StatusCode)
		}
		m.equal(t, map[string]float64{})
		if e.planner.calls() != 0 {
			t.Error("setup: the planner was called")
		}
	})

	t.Run("a person who left is not a planner failure", func(t *testing.T) {
		e := newSPEnv(t)
		m := withMetrics(e.h)
		e.planner.err = context.Canceled
		e.do(t, "POST", "/story-plans", storyBody(spEnglish), "user1")
		m.equal(t, map[string]float64{})
	})

	t.Run("a story over the hourly allowance never became an attempt", func(t *testing.T) {
		e := newSPEnv(t)
		e.h.StoryPlansPerHour = 1
		m := withMetrics(e.h)
		e.do(t, "POST", "/story-plans", storyBody(spEnglish), "user1")
		before := m.read(t)
		if resp := e.do(t, "POST", "/story-plans", storyBody(spEnglish), "user1"); resp.StatusCode != 429 {
			t.Fatalf("%d", resp.StatusCode)
		}
		if after := m.read(t); after.lines != before.lines {
			t.Errorf("a refused story wrote %d more metric lines", after.lines-before.lines)
		}
	})
}

// ---------------------------------------------------------------- edits

func savedEdits(t *testing.T, edits []storyplan.SceneEdit, prepare func(e *spEnv, fs *fakeStock, id string)) (*metricsSink, int) {
	t.Helper()
	e, fs, id := newReadyPlan(t)
	if prepare != nil {
		prepare(e, fs, id)
	}
	m := withMetrics(e.h)
	rev := decodePlan(t, e.do(t, "GET", "/story-plans/"+id, "", "user1")).Revision
	resp := putPlan(t, e, "user1", id, rev, edits)
	return m, resp.StatusCode
}

func pick(provider, id string) *storyplan.MediaRef {
	return &storyplan.MediaRef{Provider: provider, Type: "photo", ID: id}
}

func TestMetrics_PicturesChosenReplacedAndRemovedAreCountedFromWhatTheSaveChanged(t *testing.T) {
	// Scene 1 has pixabay 7 and scene 2 has pexels 8 before the save.
	setup := func(e *spEnv, fs *fakeStock, id string) {
		if resp := putPlan(t, e, "user1", id, 1, []storyplan.SceneEdit{{Media: pick("pixabay", "7")}, {Media: pick("pexels", "8")}}); resp.StatusCode != 200 {
			t.Fatalf("setup: %d %s", resp.StatusCode, resp.Body)
		}
	}
	for _, tc := range []struct {
		name  string
		edits []storyplan.SceneEdit
		want  map[string]float64
	}{
		{"one replaced and one removed",
			[]storyplan.SceneEdit{{Media: pick("pixabay", "99")}, {ClearMedia: true}},
			map[string]float64{storymetrics.PicturesReplaced: 1, storymetrics.PicturesRemoved: 1}},
		{"nothing changed", []storyplan.SceneEdit{{}, {}}, map[string]float64{}},
		{"the same picture sent again is not a replacement",
			[]storyplan.SceneEdit{{Media: pick("pixabay", "7")}, {Media: pick("pexels", "8")}}, map[string]float64{}},
		{"same id from the other provider is a different picture",
			[]storyplan.SceneEdit{{Media: pick("pexels", "7")}, {}}, map[string]float64{storymetrics.PicturesReplaced: 1}},
		{"same id as another type is a different picture",
			[]storyplan.SceneEdit{{Media: &storyplan.MediaRef{Provider: "pixabay", Type: "video", ID: "7"}}, {}}, map[string]float64{storymetrics.PicturesReplaced: 1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, fs, id := newReadyPlan(t)
			setup(e, fs, id)
			m := withMetrics(e.h)
			resp := putPlan(t, e, "user1", id, 2, tc.edits)
			if resp.StatusCode != 200 {
				t.Fatalf("%d %s", resp.StatusCode, resp.Body)
			}
			m.equal(t, tc.want)
		})
	}

	t.Run("two first picks", func(t *testing.T) {
		m, status := savedEdits(t, []storyplan.SceneEdit{{Media: pick("pixabay", "7")}, {Media: pick("pexels", "8")}}, nil)
		if status != 200 {
			t.Fatalf("%d", status)
		}
		m.equal(t, map[string]float64{storymetrics.PicturesChosen: 2})
	})
	t.Run("a picture put back after being removed is chosen again", func(t *testing.T) {
		e, fs, id := newReadyPlan(t)
		setup(e, fs, id)
		putPlan(t, e, "user1", id, 2, []storyplan.SceneEdit{{ClearMedia: true}, {}})
		m := withMetrics(e.h)
		putPlan(t, e, "user1", id, 3, []storyplan.SceneEdit{{Media: pick("pixabay", "7")}, {}})
		m.equal(t, map[string]float64{storymetrics.PicturesChosen: 1})
	})
}

func TestMetrics_NarrationEditsAreCountedPerSceneActuallyChanged(t *testing.T) {
	a, b := "A new first line, spoken a little differently", "A new second line, spoken a little differently too"
	m, status := savedEdits(t, []storyplan.SceneEdit{{Narration: &a}, {Narration: &b}}, nil)
	if status != 200 {
		t.Fatalf("%d", status)
	}
	m.equal(t, map[string]float64{storymetrics.NarrationsEdited: 2})

	// Sending a scene's narration back unchanged is not an edit.
	e, _, id := newReadyPlan(t)
	cur := decodePlan(t, e.do(t, "GET", "/story-plans/"+id, "", "user1"))
	same := cur.Plan.Scenes[0].Narration
	m2 := withMetrics(e.h)
	if resp := putPlan(t, e, "user1", id, cur.Revision, []storyplan.SceneEdit{{Narration: &same}, {}}); resp.StatusCode != 200 {
		t.Fatalf("%d %s", resp.StatusCode, resp.Body)
	}
	m2.equal(t, map[string]float64{})
}

func TestMetrics_ASaveIsRecordedOnlyOnceTheWriteHasCommitted(t *testing.T) {
	setup := func() (*spEnv, string) {
		e, _, id := newReadyPlan(t)
		return e, id
	}
	t.Run("a stale revision", func(t *testing.T) {
		e, id := setup()
		m := withMetrics(e.h)
		if resp := putPlan(t, e, "user1", id, 99, []storyplan.SceneEdit{{Media: pick("pixabay", "7")}, {}}); resp.StatusCode != 409 {
			t.Fatalf("%d", resp.StatusCode)
		}
		m.equal(t, map[string]float64{})
	})
	t.Run("a write that loses the race", func(t *testing.T) {
		e, id := setup()
		m := withMetrics(e.h)
		e.putErr = func(int) error {
			return &dbtypes.ConditionalCheckFailedException{Message: aws.String("The conditional request failed")}
		}
		if resp := putPlan(t, e, "user1", id, 1, []storyplan.SceneEdit{{Media: pick("pixabay", "7")}, {}}); resp.StatusCode != 409 {
			t.Fatalf("%d", resp.StatusCode)
		}
		m.equal(t, map[string]float64{})
	})
	t.Run("a database error", func(t *testing.T) {
		e, id := setup()
		m := withMetrics(e.h)
		e.putErr = func(int) error { return errors.New("down") }
		if resp := putPlan(t, e, "user1", id, 1, []storyplan.SceneEdit{{Media: pick("pixabay", "7")}, {}}); resp.StatusCode != 500 {
			t.Fatalf("%d", resp.StatusCode)
		}
		m.equal(t, map[string]float64{})
	})
	t.Run("a picture the library no longer has", func(t *testing.T) {
		e, id := setup()
		e.h.StockMedia.(*fakeStock).get = nil // every lookup is "not found"
		m := withMetrics(e.h)
		if resp := putPlan(t, e, "user1", id, 1, []storyplan.SceneEdit{{Media: pick("pixabay", "7")}, {}}); resp.StatusCode != 400 {
			t.Fatalf("%d", resp.StatusCode)
		}
		m.equal(t, map[string]float64{})
	})
	t.Run("an invalid edit", func(t *testing.T) {
		e, id := setup()
		m := withMetrics(e.h)
		if resp := putPlan(t, e, "user1", id, 1, []storyplan.SceneEdit{{}}); resp.StatusCode != 400 { // one edit for two scenes
			t.Fatalf("%d", resp.StatusCode)
		}
		m.equal(t, map[string]float64{})
	})
}

func TestPlanEditsComparesScenesByPosition(t *testing.T) {
	scene := func(narr string, m *storyplan.MediaRef) storyplan.Scene {
		return storyplan.Scene{Narration: narr, Media: m}
	}
	before := storyplan.Plan{Scenes: []storyplan.Scene{scene("a", nil), scene("b", pick("pixabay", "1")), scene("c", pick("pixabay", "2")), scene("d", pick("pixabay", "3"))}}
	after := storyplan.Plan{Scenes: []storyplan.Scene{scene("a2", pick("pixabay", "9")), scene("b", nil), scene("c", pick("pexels", "2")), scene("d", pick("pixabay", "3"))}}
	got := planEdits(before, after)
	want := storymetrics.Edits{Chosen: 1, Removed: 1, Replaced: 1, Narration: 1}
	if got != want {
		t.Errorf("planEdits = %+v, want %+v", got, want)
	}
	// Plans of different length cannot happen after ApplyEdits; this must still not panic.
	_ = planEdits(storyplan.Plan{Scenes: []storyplan.Scene{scene("a", nil)}}, after)
	_ = planEdits(before, storyplan.Plan{})
}

// ---------------------------------------------------------------- videos

func TestMetrics_AVideoIsCountedStartedOnceItsJobExists(t *testing.T) {
	e := newSVEnv(t)
	m := withMetrics(e.h)
	plan := planWithPictures("en")
	e.seedPlan(svGoogleUser, svPlanA, 3, storyPlanReady, &plan)
	if resp := e.do("POST", "/story-videos", createBody(svPlanA, 3, ""), svGoogleUser); resp.StatusCode != 202 {
		t.Fatalf("%d %s", resp.StatusCode, resp.Body)
	}
	m.equal(t, map[string]float64{storymetrics.VideosStarted: 1})
}

func TestMetrics_ARefusedVideoIsNotCountedStarted(t *testing.T) {
	for name, setup := range map[string]func(e *svEnv){
		"a plan with a missing picture": func(e *svEnv) {
			plan := planWithPictures("en")
			plan.Scenes[1].Media = nil
			e.seedPlan(svGoogleUser, svPlanA, 1, storyPlanReady, &plan)
		},
		"a stale revision": func(e *svEnv) {
			plan := planWithPictures("en")
			e.seedPlan(svGoogleUser, svPlanA, 5, storyPlanReady, &plan)
		},
		"no such plan": func(e *svEnv) {},
		"a database error": func(e *svEnv) {
			plan := planWithPictures("en")
			e.seedPlan(svGoogleUser, svPlanA, 1, storyPlanReady, &plan)
			e.putErr = errors.New("down")
		},
		"already running": func(e *svEnv) {
			plan := planWithPictures("en")
			e.seedPlan(svGoogleUser, svPlanA, 1, storyPlanReady, &plan)
			e.seedJob(svGoogleUser, svPlanA, "rendering", e.now.Add(-time.Minute))
		},
	} {
		t.Run(name, func(t *testing.T) {
			e := newSVEnv(t)
			m := withMetrics(e.h)
			setup(e)
			e.do("POST", "/story-videos", createBody(svPlanA, 1, ""), svGoogleUser)
			if got := m.read(t).totals[storymetrics.VideosStarted]; got != 0 {
				t.Errorf("VideosStarted = %v for a request that made no job", got)
			}
		})
	}
}

func TestMetrics_AStartThatFailsIsCountedStartedAndFailedWithItsCode(t *testing.T) {
	e := newSVEnv(t)
	m := withMetrics(e.h)
	plan := planWithPictures("en")
	e.seedPlan(svGoogleUser, svPlanA, 1, storyPlanReady, &plan)
	e.startErr = errors.New("states: throttled")
	if resp := e.do("POST", "/story-videos", createBody(svPlanA, 1, ""), svGoogleUser); resp.StatusCode != 500 {
		t.Fatalf("%d", resp.StatusCode)
	}
	m.equal(t, map[string]float64{storymetrics.VideosStarted: 1, storymetrics.VideosFailed: 1})
	if got := m.read(t).codes; len(got) != 1 || got[0] != storyjob.CodeStartFailed {
		t.Errorf("codes = %v, want [START_FAILED]", got)
	}
}

func TestMetrics_AStartFailureThatCouldNotBeRecordedOnTheJobIsNotCountedAsAFailure(t *testing.T) {
	e := newSVEnv(t)
	m := withMetrics(e.h)
	plan := planWithPictures("en")
	e.seedPlan(svGoogleUser, svPlanA, 1, storyPlanReady, &plan)
	e.startErr = errors.New("states: throttled")
	e.updErr = errors.New("dynamodb down") // the job cannot be marked failed
	e.do("POST", "/story-videos", createBody(svPlanA, 1, ""), svGoogleUser)
	m.equal(t, map[string]float64{storymetrics.VideosStarted: 1})
}

func TestMetrics_ACancelIsCountedOnceAndOnlyWhenItEndedTheVideo(t *testing.T) {
	t.Run("a running video", func(t *testing.T) {
		e := newSVEnv(t)
		m := withMetrics(e.h)
		id := e.seedJob(svGoogleUser, svPlanA, "rendering", e.now.Add(-time.Minute))
		e.do("POST", "/story-videos/"+id+"/cancel", "", svGoogleUser)
		m.equal(t, map[string]float64{storymetrics.VideosCancelled: 1})
	})
	t.Run("cancelling twice", func(t *testing.T) {
		e := newSVEnv(t)
		m := withMetrics(e.h)
		id := e.seedJob(svGoogleUser, svPlanA, "narrating", e.now.Add(-time.Minute))
		e.do("POST", "/story-videos/"+id+"/cancel", "", svGoogleUser)
		e.do("POST", "/story-videos/"+id+"/cancel", "", svGoogleUser)
		m.equal(t, map[string]float64{storymetrics.VideosCancelled: 1})
	})
	for _, st := range []string{"completed", "failed"} {
		t.Run("a video that has already "+st, func(t *testing.T) {
			e := newSVEnv(t)
			m := withMetrics(e.h)
			id := e.seedJob(svGoogleUser, svPlanA, st, e.now.Add(-time.Hour))
			e.do("POST", "/story-videos/"+id+"/cancel", "", svGoogleUser)
			m.equal(t, map[string]float64{})
		})
	}
	t.Run("a video that finishes first", func(t *testing.T) {
		e := newSVEnv(t)
		m := withMetrics(e.h)
		id := e.seedJob(svGoogleUser, svPlanA, "rendering", e.now.Add(-time.Minute))
		e.beforeUpdate = func() {
			e.mu.Lock()
			defer e.mu.Unlock()
			e.items["USER#"+svGoogleUser+"|STORYVIDEO#"+id]["status"] = &dbtypes.AttributeValueMemberS{Value: "completed"}
		}
		e.do("POST", "/story-videos/"+id+"/cancel", "", svGoogleUser)
		m.equal(t, map[string]float64{})
	})
	t.Run("someone else's video", func(t *testing.T) {
		e := newSVEnv(t)
		m := withMetrics(e.h)
		id := e.seedJob(svGoogleUser, svPlanA, "rendering", e.now.Add(-time.Minute))
		e.do("POST", "/story-videos/"+id+"/cancel", "", "another-user")
		m.equal(t, map[string]float64{})
	})
}

func TestMetrics_ReadingAndDeletingVideosRecordsNothing(t *testing.T) {
	e := newSVEnv(t)
	m := withMetrics(e.h)
	id := e.seedJob(svGoogleUser, svPlanA, "completed", e.now.Add(-time.Hour))
	e.do("GET", "/story-videos/"+id, "", svGoogleUser)
	e.do("GET", "/story-videos", "", svGoogleUser)
	e.do("DELETE", "/story-videos/"+id, "", svGoogleUser)
	m.equal(t, map[string]float64{})
}

// ---------------------------------------------------------------- searches

func searchWith(t *testing.T, fs func(*fakeStock), params map[string]string) (*metricsSink, int) {
	t.Helper()
	e, f := newStockEnv(t)
	if fs != nil {
		fs(f)
	}
	m := withMetrics(e.h)
	resp := e.get(t, "/stock-media/search", "user1", params)
	return m, resp.StatusCode
}

func TestMetrics_ASearchThatFoundSomethingIsCountedOnce(t *testing.T) {
	m, status := searchWith(t, nil, map[string]string{"q": "lighthouse", "type": "photo"})
	if status != 200 {
		t.Fatalf("%d", status)
	}
	m.equal(t, map[string]float64{storymetrics.StockSearches: 1})
}

func TestMetrics_ASearchThatFoundNothingIsCountedEmpty(t *testing.T) {
	empty := func(f *fakeStock) {
		f.all = func(p stockmedia.SearchParams) (stockmedia.MultiResult, error) {
			return stockmedia.MultiResult{Results: []stockmedia.SearchResult{{Provider: stockmedia.ProviderPixabay, Type: p.Type}}}, nil
		}
		f.search = okSearch()
	}
	for name, params := range map[string]map[string]string{
		"both providers": {"q": "zzqx", "type": "photo"},
		"one named":      {"q": "zzqx", "type": "photo", "provider": "pixabay"},
		"a video search": {"q": "zzqx", "type": "video"},
	} {
		t.Run(name, func(t *testing.T) {
			m, status := searchWith(t, empty, params)
			if status != 200 {
				t.Fatalf("%d", status)
			}
			m.equal(t, map[string]float64{storymetrics.StockSearches: 1, storymetrics.StockSearchesEmpty: 1})
		})
	}
}

func TestMetrics_OnlyTheFirstPageOfASearchCounts(t *testing.T) {
	for page, wantCounted := range map[string]bool{"": true, "1": true, "2": false, "5": false} {
		params := map[string]string{"q": "lighthouse", "type": "photo"}
		if page != "" {
			params["page"] = page
		}
		m, _ := searchWith(t, nil, params)
		if got := m.read(t).totals[storymetrics.StockSearches] == 1; got != wantCounted {
			t.Errorf("page %q counted = %v, want %v", page, got, wantCounted)
		}
	}
}

func TestMetrics_ASearchWhereAProviderFailedSaysNothingAboutCoverage(t *testing.T) {
	// Pexels is down and Pixabay has nothing: that is not evidence that nothing exists.
	m, status := searchWith(t, func(f *fakeStock) {
		f.all = func(p stockmedia.SearchParams) (stockmedia.MultiResult, error) {
			return stockmedia.MultiResult{
				Results:  []stockmedia.SearchResult{{Provider: stockmedia.ProviderPixabay, Type: p.Type}},
				Failures: []stockmedia.ProviderFailure{{Provider: stockmedia.ProviderPexels, Err: &stockmedia.ProviderError{Provider: stockmedia.ProviderPexels, Kind: stockmedia.KindUnavailable}}},
			}, nil
		}
	}, map[string]string{"q": "lighthouse", "type": "photo"})
	if status != 200 {
		t.Fatalf("%d", status)
	}
	m.equal(t, map[string]float64{})
}

func TestMetrics_AFailedOrRefusedSearchIsNotCounted(t *testing.T) {
	t.Run("every provider failed", func(t *testing.T) {
		m, status := searchWith(t, func(f *fakeStock) {
			f.all = func(stockmedia.SearchParams) (stockmedia.MultiResult, error) {
				return stockmedia.MultiResult{}, stockmedia.ErrAllProvidersFailed
			}
		}, map[string]string{"q": "lighthouse", "type": "photo"})
		if status == 200 {
			t.Fatalf("setup: %d", status)
		}
		m.equal(t, map[string]float64{})
	})
	t.Run("a named provider failed", func(t *testing.T) {
		m, _ := searchWith(t, func(f *fakeStock) {
			f.search = func(stockmedia.ProviderName, stockmedia.SearchParams) (stockmedia.SearchResult, error) {
				return stockmedia.SearchResult{}, &stockmedia.ProviderError{Kind: stockmedia.KindUnavailable}
			}
		}, map[string]string{"q": "lighthouse", "type": "photo", "provider": "pixabay"})
		m.equal(t, map[string]float64{})
	})
	t.Run("a malformed search", func(t *testing.T) {
		m, status := searchWith(t, nil, map[string]string{"q": "", "type": "photo"})
		if status != 400 {
			t.Fatalf("%d", status)
		}
		m.equal(t, map[string]float64{})
	})
}

func TestMetrics_AHandlerWithNoEmitterWorksAsBefore(t *testing.T) {
	e, _ := newStockEnv(t)
	if e.h.Metrics != nil {
		t.Fatal("setup: the default handler should have no emitter")
	}
	if resp := e.get(t, "/stock-media/search", "user1", map[string]string{"q": "x", "type": "photo"}); resp.StatusCode != 200 {
		t.Errorf("%d", resp.StatusCode)
	}
	if resp := e.do(t, "POST", "/story-plans", storyBody(spEnglish), "user1"); resp.StatusCode != 201 {
		t.Errorf("%d", resp.StatusCode)
	}
}

func TestMetrics_NothingInAMetricLineIdentifiesAUserOrAStory(t *testing.T) {
	e, _, id := newReadyPlan(t)
	m := withMetrics(e.h)
	a := "A secret sentence about grandmother's house by the sea shore"
	putPlan(t, e, "user1", id, 1, []storyplan.SceneEdit{{Narration: &a, Media: pick("pixabay", "7")}, {}})
	e.get(t, "/stock-media/search", "user1", map[string]string{"q": "grandmother", "type": "photo"})
	m.mu.Lock()
	all := m.buf.String()
	m.mu.Unlock()
	if all == "" {
		t.Fatal("setup: no metric lines")
	}
	for _, leak := range []string{"user1", id, "grandmother", "secret", "pixabay", "7\""} {
		if strings.Contains(all, leak) {
			t.Errorf("a metric line contains %q:\n%s", leak, all)
		}
	}
}
