package rank

import (
	"fmt"
	"strings"
	"testing"

	"github.com/rahul/indifferent/backend/internal/models"
	"github.com/rahul/indifferent/backend/internal/shorts"
)

// transcriptOf builds a transcript of `words` words, one per second.
func transcriptOf(words int) models.Transcript {
	tr := models.Transcript{}
	for i := 0; i < words; i++ {
		tr.Words = append(tr.Words, models.Word{
			Text:  fmt.Sprintf("w%d", i),
			Start: float64(i),
			End:   float64(i) + 0.5,
		})
	}
	return tr
}

// The prompt must stay small enough for a one-hour source: one line per
// block, not one line per word.
func TestBuildPrompt_CompactsLongTranscripts(t *testing.T) {
	tenMinutes := transcriptOf(10 * 60)
	oneHour := transcriptOf(60 * 60)

	tenPrompt := buildPrompt(tenMinutes, 600)
	hourPrompt := buildPrompt(oneHour, 3600)

	hourLines := strings.Count(hourPrompt, "\n")
	if !strings.Contains(tenPrompt, "w0") {
		t.Error("ten-minute prompt lost its first word")
	}

	// A block is 10 seconds, so an hour is about 360 blocks, not 3600 words.
	if hourLines > 400 {
		t.Errorf("one-hour prompt has %d lines, want under 400", hourLines)
	}
	// It must still be far below one line per word.
	wordLines := 0
	for _, w := range oneHour.Words {
		_ = w
		wordLines++
	}
	if hourLines >= wordLines {
		t.Errorf("compaction did nothing: %d lines vs %d word lines", hourLines, wordLines)
	}
	// Every word still has to reach the model, or a moment could be missed.
	if !strings.Contains(hourPrompt, "w3599") {
		t.Error("last word missing from the prompt")
	}
	if !strings.Contains(hourPrompt, "w0") {
		t.Error("first word missing from the prompt")
	}
}

// A short transcript is unchanged in substance: the words are all there.
func TestBuildPrompt_KeepsAllWords(t *testing.T) {
	tr := models.Transcript{Words: []models.Word{
		{Text: "hello", Start: 12, End: 12.5},
		{Text: "world", Start: 35, End: 35.5},
	}}
	prompt := buildPrompt(tr, 60)
	if !strings.Contains(prompt, "hello") || !strings.Contains(prompt, "world") {
		t.Errorf("prompt lost words:\n%s", prompt)
	}
}

// Snapping must move boundaries onto word edges.
func TestSnapSegmentsToWords(t *testing.T) {
	words := []models.Word{
		{Text: "a", Start: 0, End: 0.4},
		{Text: "b", Start: 30, End: 30.4},
		{Text: "c", Start: 60, End: 60.4},
	}
	// 30.2 to 60.2 is exactly the minimum clip length, so it stays valid.
	raw := []rawSegment{{Start: 30.2, End: 60.2, Score: 1, HookText: "h"}}

	out := snapSegmentsToWords(raw, words)
	if out[0].Start != 30 || out[0].End != 60.4 {
		t.Errorf("snapped = %v-%v, want 30-60.4", out[0].Start, out[0].End)
	}
}

// Snapping must never turn a valid segment into an invalid one: a segment
// sitting exactly on the minimum duration keeps the model's own numbers.
func TestSnapSegmentsToWords_KeepsBoundsValid(t *testing.T) {
	words := []models.Word{
		{Text: "a", Start: 0, End: 0.4},
		{Text: "b", Start: 30, End: 30.4},
	}
	// Exactly the minimum clip length, with a start that would snap forward.
	raw := []rawSegment{{Start: 0.1, End: 30.1, Score: 1, HookText: "h"}}

	out := snapSegmentsToWords(raw, words)
	d := out[0].End - out[0].Start
	if d < shorts.MinClipDuration || d > shorts.MaxClipDuration {
		t.Errorf("snapped duration %v is out of bounds", d)
	}
}

// A boundary before the first word or after the last one is left alone.
func TestSnapSegmentsToWords_OutOfRange(t *testing.T) {
	words := []models.Word{{Text: "a", Start: 100, End: 100.4}}
	raw := []rawSegment{{Start: 0, End: 40, Score: 1, HookText: "h"}}
	out := snapSegmentsToWords(raw, words)
	if out[0].Start != 0 || out[0].End != 40 {
		t.Errorf("out-of-range boundaries were moved: %v-%v", out[0].Start, out[0].End)
	}
}
