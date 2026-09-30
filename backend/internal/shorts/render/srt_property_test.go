package render

import (
	"strconv"
	"strings"
	"testing"

	"github.com/rahul/indifferent/backend/internal/models"
	"pgregory.net/rapid"
)

// parsedCue is a caption cue recovered from generated SRT text.
type parsedCue struct {
	start float64
	end   float64
	words []string
}

// parseSRT recovers cues from an SRT document produced by BuildSRT. It is a
// test-only reader used to assert the timing/ordering properties.
func parseSRT(t *rapid.T, srt string) []parsedCue {
	var cues []parsedCue
	blocks := strings.Split(strings.TrimRight(srt, "\n"), "\n\n")
	for _, blk := range blocks {
		if strings.TrimSpace(blk) == "" {
			continue
		}
		lines := strings.Split(blk, "\n")
		if len(lines) < 3 {
			t.Fatalf("malformed SRT block: %q", blk)
		}
		// lines[0] = index, lines[1] = "start --> end", lines[2] = text
		parts := strings.Split(lines[1], " --> ")
		if len(parts) != 2 {
			t.Fatalf("malformed timestamp line: %q", lines[1])
		}
		cues = append(cues, parsedCue{
			start: parseTimestamp(t, parts[0]),
			end:   parseTimestamp(t, parts[1]),
			words: strings.Fields(strings.Join(lines[2:], " ")),
		})
	}
	return cues
}

func parseTimestamp(t *rapid.T, ts string) float64 {
	// HH:MM:SS,mmm
	main := strings.Split(ts, ",")
	if len(main) != 2 {
		t.Fatalf("bad timestamp %q", ts)
	}
	hms := strings.Split(main[0], ":")
	if len(hms) != 3 {
		t.Fatalf("bad timestamp %q", ts)
	}
	h, _ := strconv.Atoi(hms[0])
	m, _ := strconv.Atoi(hms[1])
	s, _ := strconv.Atoi(hms[2])
	ms, _ := strconv.Atoi(main[1])
	return float64(h*3600+m*60+s) + float64(ms)/1000.0
}

// genWords produces a word list with monotonically increasing, non-overlapping
// timings, plus a segmentStart that may fall before, inside, or after them.
func genWords(t *rapid.T) ([]models.Word, float64, float64) {
	n := rapid.IntRange(0, 40).Draw(t, "wordCount")
	words := make([]models.Word, 0, n)
	cursor := rapid.Float64Range(0, 100).Draw(t, "firstStart")
	maxEnd := cursor
	for i := 0; i < n; i++ {
		gap := rapid.Float64Range(0, 1.5).Draw(t, "gap")
		dur := rapid.Float64Range(0.05, 1.2).Draw(t, "dur")
		start := cursor + gap
		end := start + dur
		words = append(words, models.Word{
			Text:  "w" + strconv.Itoa(i),
			Start: start,
			End:   end,
		})
		cursor = end
		maxEnd = end
	}
	segmentStart := rapid.Float64Range(0, 120).Draw(t, "segmentStart")
	return words, segmentStart, maxEnd
}

// Feature: ai-shorts-generator, Property 7
// Caption cues stay within the clip and preserve order: times are
// non-negative, non-overlapping, monotonically increasing, bounded by the
// clip duration, with words in transcript order.
// Validates: Requirements 4.4
func TestProperty7_CaptionCueTimingAndOrder(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		words, segmentStart, maxEnd := genWords(t)
		srt := BuildSRT(words, segmentStart)
		cues := parseSRT(t, srt)

		// Clip duration upper bound: latest word end shifted into clip space.
		clipDuration := maxEnd - segmentStart
		if clipDuration < 0 {
			clipDuration = 0
		}
		// Allow 1ms of rounding slack from timestamp formatting.
		const slack = 0.0011

		// Reconstruct the flattened word order from cues and compare to input.
		var flat []string
		var prevEnd float64
		for ci, c := range cues {
			if c.start < 0 {
				t.Fatalf("cue %d start negative: %v", ci, c.start)
			}
			if c.start > c.end+slack {
				t.Fatalf("cue %d start %v after end %v", ci, c.start, c.end)
			}
			if ci > 0 && c.start < prevEnd-slack {
				t.Fatalf("cue %d overlaps previous: start %v < prevEnd %v", ci, c.start, prevEnd)
			}
			if c.end > clipDuration+slack {
				t.Fatalf("cue %d end %v exceeds clip duration %v", ci, c.end, clipDuration)
			}
			prevEnd = c.end
			flat = append(flat, c.words...)
		}

		if len(flat) != len(words) {
			t.Fatalf("word count mismatch: cues have %d, input %d", len(flat), len(words))
		}
		for i := range words {
			if flat[i] != words[i].Text {
				t.Fatalf("word order mismatch at %d: got %q want %q", i, flat[i], words[i].Text)
			}
		}
	})
}
