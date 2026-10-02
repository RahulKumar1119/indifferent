package render

import (
	"strings"
	"testing"

	"github.com/rahul/indifferent/backend/internal/models"
)

func TestBuildSRT_EmptyWords(t *testing.T) {
	if got := BuildSRT(nil, 0); got != "" {
		t.Fatalf("expected empty output for no words, got %q", got)
	}
}

func TestBuildSRT_BlockFormatting(t *testing.T) {
	words := []models.Word{
		{Text: "hello", Start: 0.12, End: 0.5},
		{Text: "world", Start: 0.5, End: 1.0},
	}
	got := BuildSRT(words, 0)
	want := "1\n00:00:00,120 --> 00:00:01,000\nhello world\n\n"
	if got != want {
		t.Fatalf("SRT formatting mismatch:\n got=%q\nwant=%q", got, want)
	}
}

func TestBuildSRT_TimeShiftRelativeToSegmentStart(t *testing.T) {
	words := []models.Word{
		{Text: "one", Start: 10.0, End: 10.4},
		{Text: "two", Start: 10.4, End: 11.0},
	}
	got := BuildSRT(words, 10.0)
	// Shifted by -10s: 0.0 --> 1.0
	if !strings.Contains(got, "00:00:00,000 --> 00:00:01,000") {
		t.Fatalf("expected shifted timestamps, got %q", got)
	}
}

func TestBuildSRT_ClampsNegativeToZero(t *testing.T) {
	words := []models.Word{
		{Text: "before", Start: 2.0, End: 3.0},
	}
	// segmentStart after the word -> times clamp to zero.
	got := BuildSRT(words, 5.0)
	if !strings.HasPrefix(got, "1\n00:00:00,000 --> 00:00:00,000\nbefore\n\n") {
		t.Fatalf("expected clamped-to-zero timestamps, got %q", got)
	}
}

func TestBuildSRT_GroupsByWordCount(t *testing.T) {
	// 8 words with tiny durations should split at the 4-word boundary
	// (short lines fit the 9:16 window without edge clipping).
	words := make([]models.Word, 8)
	for i := 0; i < 8; i++ {
		start := float64(i) * 0.1
		words[i] = models.Word{Text: "w", Start: start, End: start + 0.05}
	}
	got := BuildSRT(words, 0)
	blocks := splitBlocks(got)
	if len(blocks) != 2 {
		t.Fatalf("expected 2 cues (4-word boundary), got %d:\n%s", len(blocks), got)
	}
	// First cue has 4 words, second has 4.
	if n := len(strings.Fields(blocks[0].text)); n != 4 {
		t.Errorf("first cue has %d words, want 4", n)
	}
	if n := len(strings.Fields(blocks[1].text)); n != 4 {
		t.Errorf("second cue has %d words, want 4", n)
	}
}

func TestBuildSRT_GroupsByDuration(t *testing.T) {
	// 4 words spanning >2.5s should split on the duration boundary before
	// reaching the 4-word cap.
	words := []models.Word{
		{Text: "a", Start: 0.0, End: 1.0},
		{Text: "b", Start: 1.0, End: 2.0},
		{Text: "c", Start: 2.0, End: 3.0}, // end 3.0 > 2.5 cap -> new cue
		{Text: "d", Start: 3.0, End: 4.0},
	}
	got := BuildSRT(words, 0)
	blocks := splitBlocks(got)
	if len(blocks) < 2 {
		t.Fatalf("expected cue split on duration cap, got %d cues:\n%s", len(blocks), got)
	}
}

type block struct {
	timing string
	text   string
}

func splitBlocks(srt string) []block {
	var out []block
	for _, blk := range strings.Split(strings.TrimRight(srt, "\n"), "\n\n") {
		if strings.TrimSpace(blk) == "" {
			continue
		}
		lines := strings.Split(blk, "\n")
		if len(lines) < 3 {
			continue
		}
		out = append(out, block{timing: lines[1], text: strings.Join(lines[2:], " ")})
	}
	return out
}
