package render

// Karaoke caption builder: ASS dialogue lines with per-word {\kf} sweep
// timings so the highlight fills word-by-word as it is spoken. The render
// worker writes this instead of SRT when the job's style is karaoke.

import (
	"fmt"
	"strings"
	"unicode"

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
	return buildStyledASS(words, segmentStart, style, nil, true)
}

// BuildASSBreaks is BuildASS with hard cue breaks, like BuildSRTBreaks.
func BuildASSBreaks(words []models.Word, segmentStart float64, style models.CaptionStyle, breakBefore []bool) string {
	return buildStyledASS(words, segmentStart, style, breakBefore, true)
}

// aboveControlsMarginPx is the bottom margin of captions placed above the buttons and text that
// Shorts, Reels and TikTok draw over a video: 15% of the 1920 pixel height.
const aboveControlsMarginPx = 288

// legacyKaraokeMarginPx is the bottom margin karaoke captions have always had.
const legacyKaraokeMarginPx = 90

// srtOutlineScale is how many pixels one unit of Outline is in an SRT caption. libass reads SRT in
// a 384 by 288 script and scales borders by the width, so Outline=2 is a border of 5.6 pixels on a
// 1080 pixel frame (font sizes and margins scale by the height, 1920 / 288). Measured against SRT
// renders in alpine:3.21: boxes and outlines are this wide.
const srtOutlineScale = float64(assPlayResX) / 384

// presetOutlinePx is a preset's outline or box padding across the frame, in pixels. Up and down it
// is wider, because libass scales a border by the width sideways and by the height vertically
// (see presetOutlineYPx).
func presetOutlinePx(ps presetASS) int {
	return int(float64(ps.outline)*srtOutlineScale + 0.5)
}

// presetOutlineYPx is the same border measured up and down.
func presetOutlineYPx(ps presetASS) int {
	return int(float64(ps.outline)*assScale + 0.5)
}

// presetASS is how the SRT presets are drawn when a cue is too wide and the caption file has to be
// ASS (SRT cannot change the size of one cue). It repeats what ResolveForceStyle asks libass for,
// in the same units, so a cue that fits looks the same either way.
type presetASS struct {
	borderStyle int
	outline     int // in the 288 pixel scale of ResolveForceStyle
	marginV     int // in the 288 pixel scale; 10 is what libass uses when none is given
}

var presetStyles = map[string]presetASS{
	CaptionMinimal: {borderStyle: 1, outline: 1, marginV: 45},
	CaptionClean:   {borderStyle: 1, outline: 1, marginV: 50},
	CaptionBold:    {borderStyle: 3, outline: 2, marginV: 10},
}

// fontName is the family a style asks for. Empty keeps Arial, which the render images answer
// with DejaVu Sans, so a style saved before the font choice existed is drawn as before.
func fontName(s models.CaptionStyle) string {
	if f, ok := fontFamilies[s.Font]; ok {
		return f
	}
	return "Arial"
}

// bottomMarginPx is the margin from the bottom edge for bottom captions, in pixels.
func bottomMarginPx(s models.CaptionStyle, legacyPx int) int {
	if s.AboveControls && legacyPx < aboveControlsMarginPx {
		return aboveControlsMarginPx
	}
	return legacyPx
}

func buildStyledASS(words []models.Word, segmentStart float64, style models.CaptionStyle, breakBefore []bool, karaoke bool) string {
	r := resolvedCaption(style)
	sizePx := int(float64(r.FontSize)*assScale + 0.5)
	outlinePx := 3
	borderTags := ""
	marginV := legacyKaraokeMarginPx
	var b strings.Builder
	fmt.Fprintf(&b, "[Script Info]\nScriptType: v4.00+\nPlayResX: %d\nPlayResY: %d\nWrapStyle: 2\nScaledBorderAndShadow: yes\n\n",
		assPlayResX, assPlayResY)
	fmt.Fprintf(&b, "[V4+ Styles]\nFormat: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding\n")
	if karaoke {
		if captionAlignment(r) == 2 {
			marginV = bottomMarginPx(r, legacyKaraokeMarginPx)
		}
		fmt.Fprintf(&b, "Style: Default,%s,%d,%s,%s,&HFF000000,&H80000000,0,0,0,0,100,100,0,0,1,3,0,%d,40,40,%d,1\n\n",
			fontName(r), sizePx, hexToASS(r.Highlight), hexToASS("#9AA0A6"), captionAlignment(r), marginV)
	} else {
		ps := presetStyles[r.Preset]
		if ps.outline == 0 {
			ps = presetStyles[CaptionBold]
		}
		outlinePx = presetOutlinePx(ps)
		// An SRT border is wider up and down than sideways; one Outline value cannot say that, so
		// each line carries both.
		borderTags = fmt.Sprintf(`{\xbord%d\ybord%d}`, outlinePx, presetOutlineYPx(ps))
		// The same margin, in the same units, as the SRT path asks libass for, so the two agree.
		marginV = int(float64(forceMarginV(r, ps.marginV))*assScale + 0.5)
		// libass draws a box in the outline colour, VSFilter in the back colour: give both.
		fmt.Fprintf(&b, "Style: Default,%s,%d,%s,&H00FFFFFF,&H00000000,&H00000000,0,0,0,0,100,100,0,0,%d,%d,0,%d,28,28,%d,1\n\n",
			fontName(r), sizePx, hexToASS(r.Color), ps.borderStyle, presetOutlineYPx(ps), captionAlignment(r), marginV)
	}
	b.WriteString("[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n")
	font := measureFont(r.Font)
	for _, c := range groupCues(words, breakBefore) {
		texts := make([]string, 0, c.to-c.from)
		for k := c.from; k < c.to; k++ {
			texts = append(texts, words[k].Text)
		}
		lay := layoutCue(texts, font, sizePx, outlinePx)
		parts := make([]string, 0, len(texts))
		for k := c.from; k < c.to; k++ {
			if !karaoke {
				parts = append(parts, assEscape(words[k].Text))
				continue
			}
			cs := int((words[k].End-words[k].Start)*100 + 0.5)
			if cs < 1 {
				cs = 1
			}
			parts = append(parts, fmt.Sprintf(`{\kf%d}%s`, cs, assEscape(words[k].Text)))
		}
		text := borderTags + joinCue(parts, lay.breakAt)
		if lay.shrunk {
			text = fmt.Sprintf(`{\fs%d}`, lay.sizePx) + text
		}
		start := shift(words[c.from].Start, segmentStart)
		end := shift(words[c.to-1].End, segmentStart)
		fmt.Fprintf(&b, "Dialogue: 0,%s,%s,Default,,0,0,0,,%s\n",
			formatASSTime(start), formatASSTime(end), text)
	}
	return b.String()
}

// assEscape escapes ASS special characters in caption text. A Dialogue event is
// one line of the file, so a line break (or any control character) in the text
// would end the event and let the rest be read as a new line of the subtitle
// file, such as another event. They become plain spaces instead.
func assEscape(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || r == '\u2028' || r == '\u2029' {
			return ' '
		}
		return r
	}, s)
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
