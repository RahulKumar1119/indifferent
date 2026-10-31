package render

import (
	"strings"
	"testing"

	"github.com/rahul/indifferent/backend/internal/models"
)

func wordsAt(texts ...string) []models.Word {
	var out []models.Word
	for i, t := range texts {
		out = append(out, models.Word{Text: t, Start: float64(i) * 0.3, End: float64(i)*0.3 + 0.25})
	}
	return out
}

func TestBuildSRTBreaks_ANewSceneStartsANewCue(t *testing.T) {
	words := wordsAt("a", "b", "c", "d", "e", "f") // would be one cue of 4 then one of 2
	plain := BuildSRT(words, 0)
	if strings.Count(plain, " --> ") != 2 {
		t.Fatalf("setup: expected 2 cues without breaks, got\n%s", plain)
	}
	// Scene change before word 2: cues are [a b] [c d e f].
	got := BuildSRTBreaks(words, 0, []bool{false, false, true, false, false, false})
	if !strings.Contains(got, "a b\n") || !strings.Contains(got, "c d e f\n") {
		t.Errorf("the cue ran across the scene change:\n%s", got)
	}
}

func TestBuildSRTBreaks_NoBreaksIsTheHistoricalOutput(t *testing.T) {
	words := wordsAt("a", "b", "c", "d", "e", "f", "g", "h", "i")
	for _, breaks := range [][]bool{nil, {}, make([]bool, len(words))} {
		if BuildSRTBreaks(words, 0, breaks) != BuildSRT(words, 0) {
			t.Errorf("breaks %v changed the output", breaks)
		}
	}
}

func TestBuildSRTBreaks_ABreakOnTheFirstWordOrPastTheEndChangesNothing(t *testing.T) {
	words := wordsAt("a", "b", "c")
	want := BuildSRT(words, 0)
	if got := BuildSRTBreaks(words, 0, []bool{true, false, false}); got != want {
		t.Errorf("a break on word 0 changed the output:\n%s", got)
	}
	if got := BuildSRTBreaks(words, 0, []bool{false, false, false, true, true}); got != want {
		t.Errorf("breaks past the last word changed the output:\n%s", got)
	}
}

func TestBuildASSBreaks_ANewSceneStartsANewCue(t *testing.T) {
	words := wordsAt("a", "b", "c", "d")
	style := models.CaptionStyle{Preset: "karaoke"}
	got := BuildASSBreaks(words, 0, style, []bool{false, true, false, false})
	if n := strings.Count(got, "Dialogue:"); n != 2 {
		t.Fatalf("want 2 dialogue lines, got %d:\n%s", n, got)
	}
	if !strings.Contains(got, `{\kf25}a`) || strings.Contains(got, `a {\kf`) {
		t.Errorf("first cue should hold only the first word:\n%s", got)
	}
	if BuildASSBreaks(words, 0, style, nil) != BuildASS(words, 0, style) {
		t.Error("no breaks must equal BuildASS")
	}
}

func TestGroupCues_EveryWordIsInExactlyOneCueInOrder(t *testing.T) {
	words := wordsAt("a", "b", "c", "d", "e", "f", "g", "h", "i", "j")
	cues := groupCues(words, []bool{false, false, true, false, false, false, false, true, false, false})
	next := 0
	for _, c := range cues {
		if c.from != next || c.to <= c.from || c.to-c.from > maxWordsPerCue {
			t.Fatalf("bad cue %+v after word %d", c, next)
		}
		next = c.to
	}
	if next != len(words) {
		t.Errorf("cues cover %d of %d words", next, len(words))
	}
}
