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
	var b strings.Builder
	index := 1

	i := 0
	for i < len(words) {
		cueStart := words[i].Start
		cueEnd := words[i].End
		texts := []string{words[i].Text}
		j := i + 1
		for j < len(words) && len(texts) < maxWordsPerCue {
			// Stop this cue if adding the next word would push its span past
			// the per-cue duration cap.
			if words[j].End-cueStart > maxCueDuration {
				break
			}
			texts = append(texts, words[j].Text)
			cueEnd = words[j].End
			j++
		}

		start := shift(cueStart, segmentStart)
		end := shift(cueEnd, segmentStart)

		fmt.Fprintf(&b, "%d\n%s --> %s\n%s\n\n", index, formatTimestamp(start), formatTimestamp(end), strings.Join(texts, " "))

		index++
		i = j
	}

	return b.String()
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
