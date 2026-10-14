package render

// Karaoke caption builder: ASS dialogue lines with per-word {\kf} sweep
// timings so the highlight fills word-by-word as it is spoken. The render
// worker writes this instead of SRT when the job's style is karaoke.

import (
	"fmt"
	"strings"

	"github.com/rahul/indifferent/backend/internal/models"
)

// assPlayRes matches the output frame so sizes/margins map 1:1 to pixels.
// SRT force_style sizes assume the libass default PlayResY of 288, so ASS
// sizes scale by assScale.
const (
	assPlayResX = 1080
	assPlayResY = 1920
	assScale    = float64(assPlayResY) / 288
)

// BuildASS groups words exactly like BuildSRT (same cue boundaries) but
// emits ASS Dialogue lines whose per-word {\kf<centiseconds>} tags sweep the
// highlight color across the cue. Times are relative to segmentStart.
func BuildASS(words []models.Word, segmentStart float64, style models.CaptionStyle) string {
	r := resolvedCaption(style)
	var b strings.Builder
	fmt.Fprintf(&b, "[Script Info]\nScriptType: v4.00+\nPlayResX: %d\nPlayResY: %d\nWrapStyle: 2\nScaledBorderAndShadow: yes\n\n",
		assPlayResX, assPlayResY)
	fmt.Fprintf(&b, "[V4+ Styles]\nFormat: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding\n")
	fmt.Fprintf(&b, "Style: Default,Arial,%d,%s,%s,&HFF000000,&H80000000,0,0,0,0,100,100,0,0,1,3,0,%d,40,40,90,1\n\n",
		int(float64(r.FontSize)*assScale+0.5), hexToASS(r.Highlight), hexToASS("#9AA0A6"), captionAlignment(r))
	b.WriteString("[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n")

	i := 0
	for i < len(words) {
		cueStart := words[i].Start
		cueEnd := words[i].End
		texts := []string{words[i].Text}
		j := i + 1
		for j < len(words) && len(texts) < maxWordsPerCue {
			if words[j].End-cueStart > maxCueDuration {
				break
			}
			texts = append(texts, words[j].Text)
			cueEnd = words[j].End
			j++
		}
		var parts []string
		for k := i; k < j; k++ {
			cs := int((words[k].End-words[k].Start)*100 + 0.5)
			if cs < 1 {
				cs = 1
			}
			parts = append(parts, fmt.Sprintf(`{\kf%d}%s`, cs, assEscape(words[k].Text)))
		}
		start := shift(cueStart, segmentStart)
		end := shift(cueEnd, segmentStart)
		fmt.Fprintf(&b, "Dialogue: 0,%s,%s,Default,,0,0,0,,%s\n",
			formatASSTime(start), formatASSTime(end), strings.Join(parts, " "))
		i = j
	}
	return b.String()
}

// assEscape escapes ASS special characters in caption text.
func assEscape(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, "{", `\{`)
	s = strings.ReplaceAll(s, "}", `\}`)
	return s
}

// formatASSTime renders seconds as "H:MM:SS.cc".
func formatASSTime(seconds float64) string {
	if seconds < 0 {
		seconds = 0
	}
	totalCs := int64(seconds*100 + 0.5)
	cs := totalCs % 100
	totalSecs := totalCs / 100
	s := totalSecs % 60
	m := (totalSecs / 60) % 60
	h := totalSecs / 3600
	return fmt.Sprintf("%d:%02d:%02d.%02d", h, m, s, cs)
}
