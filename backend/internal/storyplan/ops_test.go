package storyplan

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// opsPlan is a plan whose scenes are told apart by their narration and picture.
func opsPlan(t testing.TB, n int) Plan {
	t.Helper()
	p := basePlan(t, n)
	for i := range p.Scenes {
		p.Scenes[i].Narration = fmt.Sprintf("Scene number %c has some words to say here.", 'A'+i)
		p.Scenes[i].Media = &MediaRef{Provider: "pexels", Type: MediaPhoto, ID: fmt.Sprint(100 + i)}
		p.Scenes[i].MediaSource = MediaSourceAuto
	}
	return p
}

func narrations(p Plan) string {
	var s []string
	for _, sc := range p.Scenes {
		s = append(s, strings.TrimSuffix(strings.TrimPrefix(sc.Narration, "Scene number "), " has some words to say here."))
	}
	return strings.Join(s, "")
}

func TestApplyPlanChange_NoOpsIsApplyEdits(t *testing.T) {
	p := opsPlan(t, 3)
	_, got, err := ApplyPlanChange(p, nil, noEdits(3), ScriptLatin)
	if err != nil {
		t.Fatal(err)
	}
	want, _ := ApplyEdits(p, noEdits(3), ScriptLatin)
	if narrations(got) != narrations(want) || got.TotalSeconds != want.TotalSeconds {
		t.Errorf("no ops changed the result")
	}
	if _, _, err := ApplyPlanChange(p, nil, noEdits(2), ScriptLatin); !errors.Is(err, ErrInvalidEdit) {
		t.Errorf("a wrong edit count without ops must still be refused, got %v", err)
	}
}

func TestApplyPlanChange_Move(t *testing.T) {
	for _, c := range []struct {
		from, to int
		want     string
	}{
		{1, 4, "BCDA"}, {4, 1, "DABC"}, {2, 3, "ACBD"}, {3, 2, "ACBD"}, {1, 2, "BACD"},
	} {
		p := opsPlan(t, 4)
		_, got, err := ApplyPlanChange(p, []SceneOp{{Op: OpMove, From: c.from, To: c.to}}, noEdits(4), ScriptLatin)
		if err != nil {
			t.Fatalf("move %d->%d: %v", c.from, c.to, err)
		}
		if narrations(got) != c.want {
			t.Errorf("move %d->%d = %s, want %s", c.from, c.to, narrations(got), c.want)
		}
		if got.TotalSeconds != p.TotalSeconds {
			t.Errorf("a move changed the total: %v vs %v", got.TotalSeconds, p.TotalSeconds)
		}
		// Each picture travels with its scene.
		for _, sc := range got.Scenes {
			letter := int(sc.Narration[len("Scene number ")] - 'A')
			if sc.Media == nil || sc.Media.ID != fmt.Sprint(100+letter) {
				t.Errorf("move %d->%d: picture did not travel with its scene: %+v", c.from, c.to, sc)
			}
			if sc.MediaSource != MediaSourceAuto {
				t.Errorf("a move lost the auto-pick mark")
			}
		}
	}
}

func TestApplyPlanChange_ComposedMovesReorder(t *testing.T) {
	p := opsPlan(t, 4)
	ops := []SceneOp{{Op: OpMove, From: 4, To: 1}, {Op: OpMove, From: 4, To: 2}} // DABC then DCAB? check: DABC -> move 4(C)->2
	_, got, err := ApplyPlanChange(p, ops, noEdits(4), ScriptLatin)
	if err != nil {
		t.Fatal(err)
	}
	if narrations(got) != "DCAB" {
		t.Errorf("got %s", narrations(got))
	}
}

func TestApplyPlanChange_Split(t *testing.T) {
	p := opsPlan(t, 3)
	edits := []SceneEdit{{}, {Narration: ptr("First half of the second scene.")}, {Narration: ptr("Second half of the second scene.")}, {}}
	re, got, err := ApplyPlanChange(p, []SceneOp{{Op: OpSplit, Scene: 2}}, edits, ScriptLatin)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Scenes) != 4 || len(re.Scenes) != 4 {
		t.Fatalf("scenes = %d", len(got.Scenes))
	}
	a, b := got.Scenes[1], got.Scenes[2]
	if a.Narration != "First half of the second scene." || b.Narration != "Second half of the second scene." {
		t.Errorf("narrations: %q / %q", a.Narration, b.Narration)
	}
	if a.Media == nil || b.Media == nil || *a.Media != *b.Media || *a.Media != *p.Scenes[1].Media {
		t.Errorf("the second half must inherit the picture: %+v %+v", a.Media, b.Media)
	}
	if a.Media == b.Media {
		t.Errorf("the halves share one MediaRef pointer")
	}
	if b.MediaSource != p.Scenes[1].MediaSource || b.Description != p.Scenes[1].Description || b.MediaType != p.Scenes[1].MediaType {
		t.Errorf("the second half lost its settings: %+v", b)
	}
	if len(b.Keywords) == 0 || &b.Keywords[0] == &a.Keywords[0] {
		t.Errorf("keywords missing or shared")
	}
	if a.Duration <= 0 || b.Duration <= 0 {
		t.Errorf("durations were not estimated: %v %v", a.Duration, b.Duration)
	}
	var sum float64
	for _, sc := range got.Scenes {
		sum += sc.Duration
	}
	if got.TotalSeconds != round1(sum) {
		t.Errorf("total %v != sum %v", got.TotalSeconds, sum)
	}
	if p.Scenes[1].Narration != "Scene number B has some words to say here." {
		t.Errorf("the saved plan was mutated")
	}
}

func TestApplyPlanChange_SplitNeedsBothHalves(t *testing.T) {
	p := opsPlan(t, 3)
	for name, edits := range map[string][]SceneEdit{
		"none":      noEdits(4),
		"first":     {{}, {Narration: ptr("Only the first half.")}, {}, {}},
		"second":    {{}, {}, {Narration: ptr("Only the second half.")}, {}},
		"empty one": {{}, {Narration: ptr("   ")}, {Narration: ptr("Fine here.")}, {}},
	} {
		_, _, err := ApplyPlanChange(p, []SceneOp{{Op: OpSplit, Scene: 2}}, edits, ScriptLatin)
		if !errors.Is(err, ErrInvalidEdit) {
			t.Errorf("%s: want ErrInvalidEdit, got %v", name, err)
		}
	}
}

func TestApplyPlanChange_Merge(t *testing.T) {
	p := opsPlan(t, 4)
	p.Scenes[1].MediaSource = ""
	re, got, err := ApplyPlanChange(p, []SceneOp{{Op: OpMerge, Scene: 1}}, noEdits(3), ScriptLatin)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Scenes) != 3 {
		t.Fatalf("scenes = %d", len(got.Scenes))
	}
	m := got.Scenes[0]
	if m.Narration != p.Scenes[0].Narration+" "+p.Scenes[1].Narration {
		t.Errorf("merged narration = %q", m.Narration)
	}
	if *m.Media != *p.Scenes[0].Media || m.MediaSource != MediaSourceAuto || m.Description != p.Scenes[0].Description {
		t.Errorf("the merged scene must keep the first scene's picture and settings: %+v", m)
	}
	if m.Duration <= p.Scenes[0].Duration {
		t.Errorf("duration was not recomputed for the longer text: %v", m.Duration)
	}
	if len(re.Scenes) != 3 || re.Scenes[0].Narration != m.Narration {
		t.Errorf("restructured plan does not hold the merge: %d scenes", len(re.Scenes))
	}
	// An edit may override the joined text.
	_, got, err = ApplyPlanChange(p, []SceneOp{{Op: OpMerge, Scene: 1}}, []SceneEdit{{Narration: ptr("Shortened.")}, {}, {}}, ScriptLatin)
	if err != nil || got.Scenes[0].Narration != "Shortened." {
		t.Errorf("override failed: %v %q", err, got.Scenes[0].Narration)
	}
}

func TestApplyPlanChange_MergeTooLongNeedsNarration(t *testing.T) {
	p := opsPlan(t, 3)
	long := strings.Repeat("word ", 70)
	p.Scenes[0].Narration, p.Scenes[1].Narration = long, long
	if _, _, err := ApplyPlanChange(p, []SceneOp{{Op: OpMerge, Scene: 1}}, noEdits(2), ScriptLatin); !errors.Is(err, ErrInvalidEdit) {
		t.Errorf("an over-long merge without a shortened text must be refused, got %v", err)
	}
	if _, _, err := ApplyPlanChange(p, []SceneOp{{Op: OpMerge, Scene: 1}}, []SceneEdit{{Narration: ptr("Short.")}, {}}, ScriptLatin); err != nil {
		t.Errorf("with a shortened text: %v", err)
	}
}

func TestApplyPlanChange_SplitThenMergeAndMove(t *testing.T) {
	p := opsPlan(t, 3)
	ops := []SceneOp{{Op: OpSplit, Scene: 1}, {Op: OpMove, From: 2, To: 4}, {Op: OpMerge, Scene: 3}}
	// A,A',B,C -> A,B,C,A' -> A,B,(C+A')
	edits := []SceneEdit{{Narration: ptr("Opening.")}, {}, {}}
	re, _, err := ApplyPlanChange(p, ops, edits, ScriptLatin)
	if err == nil {
		t.Fatalf("A' was split without both halves being given; expected an error, restructured=%s", narrations(re))
	}
	// Merging gives the split half's text away, so the flag only matters for survivors.
	edits = []SceneEdit{{Narration: ptr("Opening.")}, {}, {Narration: ptr("Last scene together.")}}
	_, got, err := ApplyPlanChange(p, ops, edits, ScriptLatin)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Scenes) != 3 || got.Scenes[0].Narration != "Opening." || got.Scenes[2].Narration != "Last scene together." {
		t.Errorf("got %+v", got.Scenes)
	}
}

func TestApplyPlanChange_InvalidOps(t *testing.T) {
	p := opsPlan(t, 3)
	many := make([]SceneOp, MaxOps+1)
	for i := range many {
		many[i] = SceneOp{Op: OpMove, From: 1, To: 2}
	}
	for name, ops := range map[string][]SceneOp{
		"unknown op":          {{Op: "delete", Scene: 1}},
		"empty op":            {{}},
		"move from 0":         {{Op: OpMove, From: 0, To: 2}},
		"move from too high":  {{Op: OpMove, From: 4, To: 1}},
		"move to too high":    {{Op: OpMove, From: 1, To: 4}},
		"move negative":       {{Op: OpMove, From: -1, To: 2}},
		"move in place":       {{Op: OpMove, From: 2, To: 2}},
		"split 0":             {{Op: OpSplit, Scene: 0}},
		"split too high":      {{Op: OpSplit, Scene: 4}},
		"merge last scene":    {{Op: OpMerge, Scene: 3}},
		"merge 0":             {{Op: OpMerge, Scene: 0}},
		"merge below minimum": {{Op: OpMerge, Scene: 1}, {Op: OpMerge, Scene: 1}},
		"too many ops":        many,
	} {
		_, _, err := ApplyPlanChange(p, ops, noEdits(3), ScriptLatin)
		if !errors.Is(err, ErrInvalidEdit) {
			t.Errorf("%s: want ErrInvalidEdit, got %v", name, err)
		}
	}
}

func TestApplyPlanChange_SceneCountLimits(t *testing.T) {
	full := opsPlan(t, MaxScenes)
	_, _, err := ApplyPlanChange(full, []SceneOp{{Op: OpSplit, Scene: 1}}, make([]SceneEdit, MaxScenes+1), ScriptLatin)
	if !errors.Is(err, ErrInvalidEdit) {
		t.Errorf("splitting a full plan must be refused, got %v", err)
	}
	two := opsPlan(t, MinScenes)
	if _, _, err := ApplyPlanChange(two, []SceneOp{{Op: OpMerge, Scene: 1}}, noEdits(1), ScriptLatin); !errors.Is(err, ErrInvalidEdit) {
		t.Errorf("merging down to one scene must be refused, got %v", err)
	}
	// Merging first makes room for a split.
	_, got, err := ApplyPlanChange(full, []SceneOp{{Op: OpMerge, Scene: 1}, {Op: OpSplit, Scene: 1}}, append(
		[]SceneEdit{{Narration: ptr("Part one.")}, {Narration: ptr("Part two.")}}, make([]SceneEdit, MaxScenes-2)...), ScriptLatin)
	if err != nil || len(got.Scenes) != MaxScenes {
		t.Errorf("merge then split: %v %d", err, len(got.Scenes))
	}
}

func TestApplyPlanChange_EditCountMustMatchTheResult(t *testing.T) {
	p := opsPlan(t, 3)
	for _, n := range []int{0, 2, 4} {
		_, _, err := ApplyPlanChange(p, []SceneOp{{Op: OpMove, From: 1, To: 3}}, make([]SceneEdit, n), ScriptLatin)
		if !errors.Is(err, ErrInvalidEdit) {
			t.Errorf("%d edits: %v", n, err)
		}
	}
}

func TestApplyPlanChange_TotalLengthStillEnforced(t *testing.T) {
	p := opsPlan(t, MaxScenes)
	long := strings.Repeat("a long sentence goes on and on ", 5)
	edits := make([]SceneEdit, MaxScenes)
	for i := range edits {
		edits[i].Narration = ptr(long)
	}
	if _, _, err := ApplyPlanChange(p, []SceneOp{{Op: OpMove, From: 1, To: 2}}, edits, ScriptLatin); !errors.Is(err, ErrInvalidEdit) {
		t.Errorf("a video over the limit must be refused after a move, got %v", err)
	}
}

func TestApplyPlanChange_IsAllOrNothing(t *testing.T) {
	p := opsPlan(t, 3)
	before := narrations(p)
	_, _, _ = ApplyPlanChange(p, []SceneOp{{Op: OpMove, From: 1, To: 3}, {Op: OpMerge, Scene: 3}}, noEdits(3), ScriptLatin)
	if narrations(p) != before || p.Scenes[0].Media.ID != "100" {
		t.Errorf("the input plan was changed")
	}
}
