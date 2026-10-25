package render

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/rahul/indifferent/backend/internal/models"
)

// animatedFonts maps the animated caption font choice to an installed DejaVu
// family.
var animatedFonts = map[string]string{
	"sans":      "DejaVu Sans",
	"serif":     "DejaVu Serif",
	"condensed": "DejaVu Sans Condensed",
}

// BuildAnimatedASS builds the ASS document for animated captions. Cues group
// words the way BuildSRT does. Each word carries a \kf sweep: spoken words turn
// to the highlight color, and unspoken words show the text color dimmed by the
// intensity. A word's sweep starts where the previous word ended, so silence
// between words falls inside the next sweep and the highlight tracks speech.
// Times are relative to segmentStart.
func BuildAnimatedASS(words []models.Word, segmentStart float64, c models.AnimatedCaptions) string {
	var b strings.Builder
	fmt.Fprintf(&b, "[Script Info]\nScriptType: v4.00+\nPlayResX: %d\nPlayResY: %d\nWrapStyle: 2\nScaledBorderAndShadow: yes\n\n",
		assPlayResX, assPlayResY)
	b.WriteString("[V4+ Styles]\nFormat: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding\n")
	outlineColour, backColour, borderStyle := backgroundColors(c.Background)
	alignment := captionAlignment(models.CaptionStyle{Position: c.Position})
	fmt.Fprintf(&b, "Style: Default,%s,%d,%s,%s,%s,%s,0,0,0,0,100,100,0,0,%d,%d,0,%d,40,40,90,1\n\n",
		animatedFonts[c.Font], int(float64(c.FontSize)*assScale+0.5),
		hexToASS(c.Highlight), dimmedASS(c.Color, c.Intensity),
		outlineColour, backColour, borderStyle, c.Outline, alignment)
	b.WriteString("[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n")

	i := 0
	for i < len(words) {
		cueStart := words[i].Start
		cueEnd := words[i].End
		j := i + 1
		for j < len(words) && j-i < maxWordsPerCue {
			if words[j].End-cueStart > maxCueDuration {
				break
			}
			cueEnd = words[j].End
			j++
		}
		parts := make([]string, 0, j-i)
		prevEnd := cueStart
		for k := i; k < j; k++ {
			cs := int((words[k].End-prevEnd)*100 + 0.5)
			if cs < 1 {
				cs = 1
			}
			parts = append(parts, fmt.Sprintf(`{\kf%d}%s`, cs, assText(words[k].Text)))
			prevEnd = words[k].End
		}
		fmt.Fprintf(&b, "Dialogue: 0,%s,%s,Default,,0,0,0,,%s\n",
			formatASSTime(shift(cueStart, segmentStart)), formatASSTime(shift(cueEnd, segmentStart)),
			strings.Join(parts, " "))
		i = j
	}
	return b.String()
}

// backgroundColors returns the outline colour, back colour and border style for
// the caption background. A box takes the same colour in both fields, because
// libass and VSFilter read the box colour from different fields.
func backgroundColors(background string) (outline, back string, border int) {
	switch background {
	case "dark":
		return "&H99000000", "&H99000000", 3
	case "light":
		return "&H99FFFFFF", "&H99FFFFFF", 3
	default: // none: black outline, no box
		return "&H00000000", "&HFF000000", 1
	}
}

// dimmedASS returns the unspoken-word colour: the text colour with an alpha set
// by the intensity. Intensity 0 keeps the text solid, and 100 makes it fully
// transparent.
func dimmedASS(hex string, intensity int) string {
	alpha := int(float64(intensity)*255/100 + 0.5)
	return fmt.Sprintf("&H%02X%s", alpha, hexToASS(hex)[4:])
}

// assText makes user text safe inside an ASS event. ASS reads a backslash as the
// start of a tag and braces as an override block, so a backslash becomes a
// slash and braces become parentheses. Other control characters become spaces.
func assText(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '\\':
			b.WriteRune('/')
		case r == '{':
			b.WriteRune('(')
		case r == '}':
			b.WriteRune(')')
		case unicode.IsControl(r):
			b.WriteRune(' ')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
