package api

import (
	"encoding/json"
	"testing"

	"github.com/rahul/indifferent/backend/internal/storyplan"
)

func putOps(t *testing.T, e *spEnv, user, id string, rev int, ops []storyplan.SceneOp, scenes []storyplan.SceneEdit) (int, string) {
	t.Helper()
	b, _ := json.Marshal(UpdateStoryPlanRequest{Revision: rev, Ops: ops, Scenes: scenes})
	resp := e.do(t, "PUT", "/story-plans/"+id, string(b), user)
	return resp.StatusCode, resp.Body
}

func TestUpdatePlan_SplitMergeMoveRoundTrip(t *testing.T) {
	e, _, id := newReadyPlan(t)
	first := decodePlan(t, e.do(t, "GET", "/story-plans/"+id, "", "user1"))
	orig := first.Plan.Scenes

	a, b := "The boat leaves the shore before the sun is up", "and the fisherman watches the water go gold"
	code, body := putOps(t, e, "user1", id, 1,
		[]storyplan.SceneOp{{Op: storyplan.OpSplit, Scene: 1}},
		[]storyplan.SceneEdit{{Narration: &a}, {Narration: &b}, {}})
	if code != 200 {
		t.Fatalf("split: %d %s", code, body)
	}
	var got StoryPlanResponse
	_ = json.Unmarshal([]byte(body), &got)
	if got.Revision != 2 || len(got.Plan.Scenes) != 3 {
		t.Fatalf("revision %d, %d scenes", got.Revision, len(got.Plan.Scenes))
	}
	if got.Plan.Scenes[0].Narration != a || got.Plan.Scenes[1].Narration != b || got.Plan.Scenes[2].Narration != orig[1].Narration {
		t.Errorf("narrations = %+v", got.Plan.Scenes)
	}
	if got.Plan.Scenes[1].Description != orig[0].Description {
		t.Errorf("the second half lost its description")
	}

	// Move the last scene to the front, then merge the first two.
	code, body = putOps(t, e, "user1", id, 2,
		[]storyplan.SceneOp{{Op: storyplan.OpMove, From: 3, To: 1}, {Op: storyplan.OpMerge, Scene: 1}},
		[]storyplan.SceneEdit{{}, {}})
	if code != 200 {
		t.Fatalf("move+merge: %d %s", code, body)
	}
	_ = json.Unmarshal([]byte(body), &got)
	if got.Revision != 3 || len(got.Plan.Scenes) != 2 || got.Plan.Scenes[0].Narration != orig[1].Narration+" "+a {
		t.Errorf("after move+merge: rev %d scenes %+v", got.Revision, got.Plan.Scenes)
	}
	read := decodePlan(t, e.do(t, "GET", "/story-plans/"+id, "", "user1"))
	if len(read.Plan.Scenes) != 2 || read.Revision != 3 {
		t.Errorf("stored plan: %d scenes rev %d", len(read.Plan.Scenes), read.Revision)
	}
}

func TestUpdatePlan_OpsRejections(t *testing.T) {
	e, _, id := newReadyPlan(t)
	for name, c := range map[string]struct {
		ops    []storyplan.SceneOp
		scenes []storyplan.SceneEdit
	}{
		"move out of range":      {[]storyplan.SceneOp{{Op: storyplan.OpMove, From: 1, To: 9}}, make([]storyplan.SceneEdit, 2)},
		"unknown op":             {[]storyplan.SceneOp{{Op: "drop", Scene: 1}}, make([]storyplan.SceneEdit, 2)},
		"merge last":             {[]storyplan.SceneOp{{Op: storyplan.OpMerge, Scene: 2}}, make([]storyplan.SceneEdit, 1)},
		"merge below minimum":    {[]storyplan.SceneOp{{Op: storyplan.OpMerge, Scene: 1}}, make([]storyplan.SceneEdit, 1)},
		"split without halves":   {[]storyplan.SceneOp{{Op: storyplan.OpSplit, Scene: 1}}, make([]storyplan.SceneEdit, 3)},
		"wrong edit count":       {[]storyplan.SceneOp{{Op: storyplan.OpMove, From: 1, To: 2}}, make([]storyplan.SceneEdit, 3)},
		"too many ops":           {make([]storyplan.SceneOp, storyplan.MaxOps+1), make([]storyplan.SceneEdit, 2)},
		"edits ignoring the ops": {[]storyplan.SceneOp{{Op: storyplan.OpSplit, Scene: 1}}, make([]storyplan.SceneEdit, 2)},
	} {
		code, body := putOps(t, e, "user1", id, 1, c.ops, c.scenes)
		if code != 400 {
			t.Errorf("%s: status %d %s", name, code, body)
		}
	}
	read := decodePlan(t, e.do(t, "GET", "/story-plans/"+id, "", "user1"))
	if read.Revision != 1 || len(read.Plan.Scenes) != 2 {
		t.Errorf("a rejected change saved something: rev %d, %d scenes", read.Revision, len(read.Plan.Scenes))
	}
}

func TestUpdatePlan_OpsRevisionConflictAndOwnership(t *testing.T) {
	e, _, id := newReadyPlan(t)
	narr := "A first edit that should win, said clearly enough to be a scene"
	if resp := putPlan(t, e, "user1", id, 1, []storyplan.SceneEdit{{Narration: &narr}, {}}); resp.StatusCode != 200 {
		t.Fatal(resp.Body)
	}
	mv := []storyplan.SceneOp{{Op: storyplan.OpMove, From: 1, To: 2}}
	if code, body := putOps(t, e, "user1", id, 1, mv, make([]storyplan.SceneEdit, 2)); code != 409 {
		t.Errorf("stale revision with ops: %d %s", code, body)
	}
	if code, _ := putOps(t, e, "user2", id, 2, mv, make([]storyplan.SceneEdit, 2)); code != 404 {
		t.Errorf("another user's plan: %d", code)
	}
	got := decodePlan(t, e.do(t, "GET", "/story-plans/"+id, "", "user1"))
	if got.Plan.Scenes[0].Narration != narr {
		t.Errorf("a refused move changed the plan")
	}
}

// A client that has never heard of ops keeps working unchanged.
func TestUpdatePlan_WithoutOpsIsUnchanged(t *testing.T) {
	e, _, id := newReadyPlan(t)
	narr := "An ordinary edit from a page that sends no operations at all"
	if resp := putPlan(t, e, "user1", id, 1, []storyplan.SceneEdit{{Narration: &narr}, {}}); resp.StatusCode != 200 {
		t.Fatalf("%d %s", resp.StatusCode, resp.Body)
	}
	if resp := putPlan(t, e, "user1", id, 2, []storyplan.SceneEdit{{}}); resp.StatusCode != 400 {
		t.Errorf("a wrong edit count without ops must still be refused: %d", resp.StatusCode)
	}
}
