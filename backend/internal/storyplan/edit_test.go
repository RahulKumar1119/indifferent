package storyplan

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"pgregory.net/rapid"
)

func ptr[T any](v T) *T { return &v }

// basePlan returns a valid plan of n English scenes of about 5 seconds each.
func basePlan(t testing.TB, n int) Plan {
	t.Helper()
	p, err := parseAndValidate(mustJSON(t, englishPlan(n)), ScriptLatin)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func noEdits(n int) []SceneEdit { return make([]SceneEdit, n) }

func TestMediaRefValid(t *testing.T) {
	for ref, want := range map[MediaRef]bool{
		{"pixabay", "video", "125"}:                       true,
		{"pexels", "photo", "2014422"}:                    true,
		{"pexels", "photo", strings.Repeat("9", 18)}:      true,
		{"pexels", "photo", strings.Repeat("9", 19)}:      false,
		{"unsplash", "photo", "1"}:                        false,
		{"Pexels", "photo", "1"}:                          false, // case sensitive: the API lower-cases providers itself
		{"pexels", "gif", "1"}:                            false,
		{"pexels", "photo", ""}:                           false,
		{"pexels", "photo", "abc"}:                        false,
		{"pexels", "photo", "1/../x"}:                     false,
		{"pexels", "photo", "1?a=b"}:                      false,
		{"pexels", "photo", "-1"}:                         false,
		{"pexels", "photo", "१२३"}:                        false,
		{"pexels", "photo", "12\n"}:                       false,
		{"", "", ""}:                                      false,
		{"pexels", "photo", "https://evil.example/x.mp4"}: false,
	} {
		if got := ref.Valid(); got != want {
			t.Errorf("%+v.Valid() = %v, want %v", ref, got, want)
		}
	}
}

func TestApplyEdits_NoChangeIsIdentity(t *testing.T) {
	p := basePlan(t, 4)
	out, err := ApplyEdits(p, noEdits(4), ScriptLatin)
	if err != nil {
		t.Fatal(err)
	}
	p.Warnings = nil
	if !reflect.DeepEqual(out, p) {
		t.Errorf("an empty edit changed the plan:\n%+v\n%+v", out, p)
	}
}

func TestApplyEdits_NarrationIsCleanedAndReEstimated(t *testing.T) {
	p := basePlan(t, 4)
	edits := noEdits(4)
	edits[1].Narration = ptr(`<b>The</b> boat <break time="9s"/> drifted slowly past the old stone jetty at sunrise today`)
	out, err := ApplyEdits(p, edits, ScriptLatin)
	if err != nil {
		t.Fatal(err)
	}
	got := out.Scenes[1]
	if strings.ContainsAny(got.Narration, "<>") || !strings.HasPrefix(got.Narration, "The boat") {
		t.Errorf("narration = %q", got.Narration)
	}
	// The estimate follows the new words: speaking time plus the pad, rounded to
	// a tenth. It must differ from the old scene's 5.0 only because the words did.
	want := round1(clamp(float64(wordCount(got.Narration))/WordsPerSecond+scenePad, MinSceneSeconds, MaxSceneSeconds))
	if got.Duration != want {
		t.Errorf("duration = %v, want %v for %d words", got.Duration, want, wordCount(got.Narration))
	}
	if wordCount(got.Narration) != 12 {
		t.Errorf("the tag stripping changed the word count to %d, want 12", wordCount(got.Narration))
	}
	// And a longer text gets a longer scene, which is the point of re-estimating.
	longer := noEdits(4)
	longer[1].Narration = ptr(words("w", 20))
	l, err := ApplyEdits(p, longer, ScriptLatin)
	if err != nil || l.Scenes[1].Duration <= got.Duration {
		t.Errorf("20 words gave %v, not longer than 12 words (%v): %v", l.Scenes[1].Duration, got.Duration, err)
	}
	// The other scenes are untouched.
	for _, i := range []int{0, 2, 3} {
		if !reflect.DeepEqual(out.Scenes[i], p.Scenes[i]) {
			t.Errorf("scene %d changed", i+1)
		}
	}
	if sum := sumDurations(out.Scenes); out.TotalSeconds < sum-0.15 || out.TotalSeconds > sum+0.15 {
		t.Errorf("TotalSeconds %v does not match the scenes (%v)", out.TotalSeconds, sum)
	}
	// The input plan is not modified.
	if p.Scenes[1].Narration == got.Narration {
		t.Error("ApplyEdits modified its input")
	}
}

func TestApplyEdits_PickingAndClearingMedia(t *testing.T) {
	p := basePlan(t, 3)
	pick := &MediaRef{"pixabay", "video", "125"}
	edits := noEdits(3)
	edits[0].Media = pick
	out, err := ApplyEdits(p, edits, ScriptLatin)
	if err != nil {
		t.Fatal(err)
	}
	if out.Scenes[0].Media == nil || *out.Scenes[0].Media != *pick {
		t.Fatalf("media = %+v", out.Scenes[0].Media)
	}
	if out.Scenes[0].Media == pick {
		t.Error("the stored reference must be a copy, not the caller's pointer")
	}
	// The pick survives an edit of another scene, and is cleared on request.
	e2 := noEdits(3)
	e2[1].Narration = ptr("A different line for the second scene, said out loud here")
	again, err := ApplyEdits(out, e2, ScriptLatin)
	if err != nil || again.Scenes[0].Media == nil {
		t.Fatalf("a pick was lost: %v %+v", err, again.Scenes[0].Media)
	}
	e3 := noEdits(3)
	e3[0].ClearMedia = true
	cleared, err := ApplyEdits(again, e3, ScriptLatin)
	if err != nil || cleared.Scenes[0].Media != nil {
		t.Errorf("not cleared: %v %+v", err, cleared.Scenes[0].Media)
	}
}

func TestApplyEdits_Rejections(t *testing.T) {
	p := basePlan(t, 4)
	long := strings.Repeat("word ", 400)
	for name, tc := range map[string]struct {
		edits []SceneEdit
		in    string // part of the user-facing reason
	}{
		"too few edits": {noEdits(3), "has 3 scenes but the plan has 4"},
		"too many":      {noEdits(5), "has 5 scenes but the plan has 4"},
		"none":          {nil, "has 0 scenes"},
		"empty narration": {func() []SceneEdit {
			e := noEdits(4)
			e[2].Narration = ptr("   ")
			return e
		}(), "scene 3: the narration is empty"},
		"only markup":  {func() []SceneEdit { e := noEdits(4); e[0].Narration = ptr("<b></b>"); return e }(), "scene 1: the narration is empty"},
		"far too long": {func() []SceneEdit { e := noEdits(4); e[0].Narration = ptr(long); return e }(), "too long"},
		"too long for one scene": {func() []SceneEdit {
			e := noEdits(4)
			e[0].Narration = ptr(words("w", 80))
			return e
		}(), "split it in two"},
		"wrong language": {func() []SceneEdit {
			e := noEdits(4)
			e[3].Narration = ptr("यह दृश्य हिंदी में लिखा गया है और कहानी अंग्रेज़ी में है")
			return e
		}(), "scene 4: the narration should be in the same language"},
		"media: unknown provider": {func() []SceneEdit { e := noEdits(4); e[0].Media = &MediaRef{"unsplash", "photo", "1"}; return e }(), "scene 1: that media choice is not valid"},
		"media: url as id": {func() []SceneEdit {
			e := noEdits(4)
			e[1].Media = &MediaRef{"pexels", "photo", "https://evil.example/x"}
			return e
		}(), "scene 2: that media choice is not valid"},
		"media: pick and clear": {func() []SceneEdit {
			e := noEdits(4)
			e[0].Media = &MediaRef{"pexels", "photo", "1"}
			e[0].ClearMedia = true
			return e
		}(), "not both"},
	} {
		t.Run(name, func(t *testing.T) {
			out, err := ApplyEdits(p, tc.edits, ScriptLatin)
			if !errors.Is(err, ErrInvalidEdit) {
				t.Fatalf("err = %v, want ErrInvalidEdit", err)
			}
			var ee *EditError
			if !errors.As(err, &ee) || !strings.Contains(ee.Reason, tc.in) {
				t.Errorf("reason = %q, want it to contain %q", ee.Reason, tc.in)
			}
			if len(out.Scenes) != 0 {
				t.Error("a rejected edit must return no plan")
			}
		})
	}
}

// All or nothing: a bad edit in a later scene must not leave an earlier one
// applied.
func TestApplyEdits_IsAllOrNothing(t *testing.T) {
	p := basePlan(t, 4)
	edits := noEdits(4)
	edits[0].Narration = ptr("A perfectly fine replacement line for the very first scene")
	edits[1].Media = &MediaRef{"pexels", "photo", "7"}
	edits[3].Narration = ptr("") // invalid
	before := fmt.Sprintf("%+v", p)
	if _, err := ApplyEdits(p, edits, ScriptLatin); !errors.Is(err, ErrInvalidEdit) {
		t.Fatalf("err = %v", err)
	}
	if after := fmt.Sprintf("%+v", p); after != before {
		t.Error("the input plan was modified by a rejected edit")
	}
}

func TestApplyEdits_HindiNarrationStaysInScript(t *testing.T) {
	p, err := parseAndValidate(mustJSON(t, hindiPlan(4)), ScriptDevanagari)
	if err != nil {
		t.Fatal(err)
	}
	edits := noEdits(4)
	edits[0].Narration = ptr("मछुआरा सुबह के अँधेरे में चुपचाप अपनी पुरानी नाव खेता है")
	if _, err := ApplyEdits(p, edits, ScriptDevanagari); err != nil {
		t.Errorf("Hindi narration on a Hindi story was refused: %v", err)
	}
	edits[0].Narration = ptr("The fisherman rows his old boat quietly through the dark")
	if _, err := ApplyEdits(p, edits, ScriptDevanagari); !errors.Is(err, ErrInvalidEdit) {
		t.Errorf("English narration on a Hindi story was accepted: %v", err)
	}
	// Mixed text with English names is fine.
	edits[0].Narration = ptr("मीरा पुणे के Railway Station के बाहर चाय बेचती है और Sachin उसकी मदद करता है")
	if _, err := ApplyEdits(p, edits, ScriptDevanagari); err != nil {
		t.Errorf("mixed narration was refused: %v", err)
	}
}

func TestApplyEdits_TotalLengthIsEnforcedWithoutDroppingScenes(t *testing.T) {
	p := basePlan(t, 10) // about 5 s each
	edits := noEdits(10)
	for i := range edits {
		edits[i].Narration = ptr(words("w", 34)) // 13.6 s of speech each
	}
	_, err := ApplyEdits(p, edits, ScriptLatin)
	var ee *EditError
	if !errors.As(err, &ee) || !strings.Contains(ee.Reason, "over the 90 second limit") {
		t.Errorf("err = %v: a user's edit must be refused, not trimmed from the end", err)
	}
}

func TestApplyEdits_UnknownScriptDoesNotBlockEdits(t *testing.T) {
	p := basePlan(t, 3)
	for _, sc := range []Script{ScriptNone, ScriptOther} {
		edits := noEdits(3)
		edits[0].Narration = ptr("Anything at all can be said here in any language today")
		if _, err := ApplyEdits(p, edits, sc); err != nil {
			t.Errorf("script %q: %v", sc, err)
		}
	}
}

// Whatever a user sends, the result is either ErrInvalidEdit or a plan that
// still meets the plan invariants.
func TestProperty_ApplyEditsNeverBreaksAPlan(t *testing.T) {
	text := rapid.OneOf(
		rapid.StringMatching(`[a-zA-Z ,.]{0,80}`),
		rapid.StringMatching(`[\x{0900}-\x{097F} ]{0,80}`),
		rapid.StringMatching(`[a-zA-Z<>/="' \x{200B}\x{202E}]{0,100}`),
		rapid.String(),
	)
	ref := rapid.Custom(func(t *rapid.T) *MediaRef {
		if rapid.Bool().Draw(t, "none") {
			return nil
		}
		return &MediaRef{
			Provider: rapid.SampledFrom([]string{"pexels", "pixabay", "other", ""}).Draw(t, "prov"),
			Type:     rapid.SampledFrom([]string{"photo", "video", "gif", ""}).Draw(t, "typ"),
			ID:       rapid.OneOf(rapid.StringMatching(`[0-9]{1,20}`), rapid.String()).Draw(t, "id"),
		}
	})
	rapid.Check(t, func(t *rapid.T) {
		story := rapid.SampledFrom([]Script{ScriptLatin, ScriptDevanagari, ScriptNone}).Draw(t, "story")
		n := rapid.IntRange(MinScenes, 8).Draw(t, "n")
		var base Plan
		var err error
		if story == ScriptDevanagari {
			base, err = parseAndValidate(mustJSON(t, hindiPlan(n)), story)
		} else {
			base, err = parseAndValidate(mustJSON(t, englishPlan(n)), ScriptLatin)
		}
		if err != nil {
			t.Fatalf("base plan: %v", err)
		}
		edits := make([]SceneEdit, len(base.Scenes)+rapid.IntRange(-1, 1).Draw(t, "extra"))
		for i := range edits {
			if rapid.Bool().Draw(t, "hasNarr") {
				s := text.Draw(t, "narr")
				edits[i].Narration = &s
			}
			edits[i].Media = ref.Draw(t, "media")
			edits[i].ClearMedia = rapid.Bool().Draw(t, "clear")
		}
		out, err := ApplyEdits(base, edits, story)
		if err != nil {
			if !errors.Is(err, ErrInvalidEdit) {
				t.Fatalf("unexpected error type: %v", err)
			}
			return
		}
		if len(out.Scenes) != len(base.Scenes) {
			t.Fatalf("scene count changed: %d -> %d", len(base.Scenes), len(out.Scenes))
		}
		assertInvariants(t, out, story)
		for i, sc := range out.Scenes {
			if sc.Media != nil && !sc.Media.Valid() {
				t.Fatalf("scene %d holds an invalid media ref %+v", i+1, sc.Media)
			}
		}
	})
}
