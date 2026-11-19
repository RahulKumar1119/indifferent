package render

import (
	"encoding/json"
	"os"
	"testing"
)

// TestLegacyCaptionsUnchanged pins what #327 promised: a caption that already fits is written exactly
// as it was. testdata/legacy_caption_golden.json holds a hash of the output of the code from before
// caption fitting (commit 4a64c35, the parent of the change) for 600 fixed inputs: random words in
// English and Hindi, with and without very long words, in every preset and size, plus animated
// captions. For each input, the output must be identical, unless the input has a cue that is too wide
// for the frame, which is the one case the change is meant to alter.
func TestLegacyCaptionsUnchanged(t *testing.T) {
	raw, err := os.ReadFile("testdata/legacy_caption_golden.json")
	if err != nil {
		t.Fatal(err)
	}
	var golden map[string]string
	if err := json.Unmarshal(raw, &golden); err != nil {
		t.Fatal(err)
	}
	inputs := legacyInputs()
	if len(golden) != len(inputs) {
		t.Fatalf("golden has %d inputs, generator has %d", len(golden), len(inputs))
	}
	same, changed := 0, 0
	for _, in := range inputs {
		want, ok := golden[in.ID]
		if !ok {
			t.Fatalf("%s is not in the golden file", in.ID)
		}
		got := legacyOutput(in)
		if got == want {
			same++
			continue
		}
		changed++
		if !legacyAffected(in) {
			t.Errorf("%s (%+v, %d words) changed, but no cue is too wide for the frame", in.ID, in.Style, len(in.Words))
		}
	}
	// Both halves of the claim are exercised: plenty unchanged, and some corrected.
	if same < 300 || changed < 30 {
		t.Errorf("the inputs should cover both cases: %d unchanged, %d corrected", same, changed)
	}
	t.Logf("%d of %d captions are byte for byte what they were; %d had a cue too wide for the frame and were corrected", same, len(inputs), changed)
}

// legacyAffected reports whether the input has a cue the fitting changes: for an ASS caption (karaoke
// and animated), one that has to wrap or shrink; for an SRT caption, one that has to shrink, because
// libass already wraps SRT text itself.
func legacyAffected(in legacyInput) bool {
	cues := groupCues(in.Words, nil)
	for _, c := range cues {
		texts := make([]string, 0, c.to-c.from)
		for k := c.from; k < c.to; k++ {
			texts = append(texts, in.Words[k].Text)
		}
		if in.Animated != nil {
			px := int(float64(in.Animated.FontSize)*assScale + 0.5)
			if l := layoutCue(texts, measureFont(in.Animated.Font), px, in.Animated.Outline); l.shrunk || l.breakAt > 0 {
				return true
			}
			continue
		}
		r := resolvedCaption(in.Style)
		px := int(float64(r.FontSize)*assScale + 0.5)
		outline := 3
		if !isKaraokeStyle(in.Style) {
			outline = presetOutlinePx(presetStyles[r.Preset])
		}
		l := layoutCue(texts, measureFont(r.Font), px, outline)
		if l.shrunk || (isKaraokeStyle(in.Style) && l.breakAt > 0) {
			return true
		}
	}
	return false
}
