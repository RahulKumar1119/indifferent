// Package render contains the clip-rendering logic for the AI Shorts
// pipeline: the pure SRT caption generator and the FFmpeg argument builder.
package render

import (
	"fmt"
	"strings"

	"github.com/rahul/indifferent/backend/internal/models"
)

const (
	// maxWordsPerCue caps how many words appear in a single caption cue.
	// Four words fit on one 1080px-wide vertical line at caption size;
	// longer cues overflow the 9:16 window and get chopped at the edges.
	maxWordsPerCue = 4
	// maxCueDuration caps a cue's on-screen span in seconds; a new cue starts
	// once the accumulated span would exceed this.
	maxCueDuration = 2.5
)

// BuildSRT groups word-level timings into short caption cues and formats them
// as an SRT document whose times are relative to the clip start.
//
// Words are grouped into cues of up to maxWordsPerCue words or ~maxCueDuration
// seconds (whichever comes first). Each cue's timestamps are shifted by
// -segmentStart and clamped to be non-negative so captions align after the
// clip is trimmed to [segmentStart, ...]. Transcript word order is preserved.
// (Requirement 4.4)
func BuildSRT(words []models.Word, segmentStart float64) string {
	return buildSRT(words, segmentStart, nil)
}

// BuildSRTBreaks is BuildSRT with hard cue breaks: breakBefore[i] set means
// word i starts a new cue even when the previous one has room. A story video
// uses it so no cue runs across a change of scene.
func BuildSRTBreaks(words []models.Word, segmentStart float64, breakBefore []bool) string {
	return buildSRT(words, segmentStart, breakBefore)
}

func buildSRT(words []models.Word, segmentStart float64, breakBefore []bool) string {
	var b strings.Builder
	for n, c := range groupCues(words, breakBefore) {
		start := shift(words[c.from].Start, segmentStart)
		end := shift(words[c.to-1].End, segmentStart)
		texts := make([]string, 0, c.to-c.from)
		for k := c.from; k < c.to; k++ {
			texts = append(texts, words[k].Text)
		}
		fmt.Fprintf(&b, "%d\n%s --> %s\n%s\n\n", n+1, formatTimestamp(start), formatTimestamp(end), strings.Join(texts, " "))
	}
	return b.String()
}

// cue is the half-open range [from, to) of words shown together.
type cue struct{ from, to int }

// groupCues splits words into caption cues of up to maxWordsPerCue words and
// about maxCueDuration seconds. A word with breakBefore set always starts a
// new cue. With no breaks the grouping is exactly the historical one.
func groupCues(words []models.Word, breakBefore []bool) []cue {
	var cues []cue
	i := 0
	for i < len(words) {
		cueStart := words[i].Start
		j := i + 1
		for j < len(words) && j-i < maxWordsPerCue {
			if j < len(breakBefore) && breakBefore[j] {
				break
			}
			// Stop this cue if adding the next word would push its span past
			// the per-cue duration cap.
			if words[j].End-cueStart > maxCueDuration {
				break
			}
			j++
		}
		cues = append(cues, cue{i, j})
		i = j
	}
	return cues
}

// shift moves an absolute time into clip-relative time, clamped to >= 0.
func shift(t, segmentStart float64) float64 {
	s := t - segmentStart
	if s < 0 {
		return 0
	}
	return s
}

// formatTimestamp renders seconds as an SRT timestamp "HH:MM:SS,mmm".
func formatTimestamp(seconds float64) string {
	if seconds < 0 {
		seconds = 0
	}
	totalMillis := int64(seconds*1000 + 0.5)
	ms := totalMillis % 1000
	totalSecs := totalMillis / 1000
	s := totalSecs % 60
	totalMins := totalSecs / 60
	m := totalMins % 60
	h := totalMins / 60
	return fmt.Sprintf("%02d:%02d:%02d,%03d", h, m, s, ms)
}
