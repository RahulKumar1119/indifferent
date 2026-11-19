package render

// Reusable caption styles for AI Shorts: one preset selected per job (stored
// on the ShortsJob record), honored by the render worker when burning
// captions. Sizes are libass script units on the SRT default play
// resolution (PlayResY 288); ASS output scales them to its own PlayRes.

import (
	"fmt"
	"math"
	"path/filepath"
	"strings"

	"github.com/rahul/indifferent/backend/internal/models"
)

// Caption presets (mirror models.CaptionStyle preset strings).
const (
	CaptionMinimal = "minimal"
	CaptionBold    = "bold"
	CaptionKaraoke = "karaoke"
	CaptionClean   = "clean"
)

// resolvedCaption fills defaults: preset (legacy bold), size, colors,
// position.
func resolvedCaption(s models.CaptionStyle) models.CaptionStyle {
	out := s
	if out.Preset == "" {
		out.Preset = CaptionBold
	}
	defSize := map[string]int{
		CaptionMinimal: 14,
		CaptionBold:    18,
		CaptionKaraoke: 18,
		CaptionClean:   16,
	}[out.Preset]
	if out.FontSize == 0 {
		out.FontSize = defSize
	}
	if out.Color == "" {
		out.Color = "#FFFFFF"
	}
	if out.Highlight == "" {
		out.Highlight = "#FFD60A"
	}
	if out.Position == "" {
		out.Position = "bottom"
	}
	return out
}

func captionAlignment(s models.CaptionStyle) int {
	switch s.Position {
	case "top":
		return 8
	case "middle":
		return 5
	default:
		return 2
	}
}

// hexToASS converts #RRGGBB to libass &HAABBGGRR (opaque).
func hexToASS(hex string) string {
	if len(hex) != 7 || hex[0] != '#' {
		return "&H00FFFFFF"
	}
	return "&H00" + hex[5:7] + hex[3:5] + hex[1:3]
}

// ResolveForceStyle renders the SRT force_style value for non-karaoke presets.
// The zero style resolves byte-identical to the historical burn.
func ResolveForceStyle(s models.CaptionStyle) string {
	r := resolvedCaption(s)
	font := ""
	if f, ok := fontFamilies[r.Font]; ok {
		font = ",FontName=" + f
	}
	switch r.Preset {
	case CaptionMinimal:
		return fmt.Sprintf("Alignment=%d,FontSize=%d,PrimaryColour=%s,BorderStyle=1,Outline=1,Shadow=0,MarginV=%d%s",
			captionAlignment(r), r.FontSize, hexToASS(r.Color), forceMarginV(r, 45), font)
	case CaptionClean:
		return fmt.Sprintf("Alignment=%d,FontSize=%d,PrimaryColour=%s,BorderStyle=1,Outline=1,Shadow=0,MarginV=%d%s",
			captionAlignment(r), r.FontSize, hexToASS(r.Color), forceMarginV(r, 50), font)
	default: // bold: byte-identical to the historical burn unless a margin or font is asked for
		margin := ""
		if r.AboveControls && captionAlignment(r) == 2 {
			margin = fmt.Sprintf(",MarginV=%d", forceMarginV(r, 10))
		}
		return fmt.Sprintf("Alignment=%d,FontSize=%d,PrimaryColour=%s,BorderStyle=3,Outline=2%s%s",
			captionAlignment(r), r.FontSize, hexToASS(r.Color), margin, font)
	}
}

// forceMarginV is the MarginV of an SRT preset, in the 288 pixel scale libass reads SRT in. legacy
// is what the preset has always used. Captions above the controls are never lowered: a preset
// that already sits higher keeps its own margin.
func forceMarginV(s models.CaptionStyle, legacy int) int {
	if s.AboveControls && captionAlignment(s) == 2 {
		px := float64(aboveControlsMarginPx)
		if want := int(math.Ceil(px / assScale)); want > legacy {
			return want
		}
	}
	return legacy
}

// IsKaraoke reports whether the style burns word-by-word ASS captions rather
// than SRT.
func IsKaraoke(s models.CaptionStyle) bool { return isKaraokeStyle(s) }

func isKaraokeStyle(s models.CaptionStyle) bool {
	return resolvedCaption(s).Preset == CaptionKaraoke
}

// Captions builds the caption document for words in the style, and the file extension it needs:
// ".ass" for karaoke, and ".srt" for the other presets unless a cue is too wide to fit the frame.
// SRT cannot make one cue smaller, so then the preset is written as ASS. A caption file that
// fits is written exactly as it always was. Times are shifted by -segmentStart. breakBefore, when
// given, forces a new cue at a word (see BuildSRTBreaks).
//
// Whatever is returned, the burn filter is chosen by the extension: see SubtitlesFilter.
func Captions(words []models.Word, segmentStart float64, style models.CaptionStyle, breakBefore []bool) (doc, ext string) {
	if isKaraokeStyle(style) {
		return buildStyledASS(words, segmentStart, style, breakBefore, true), ".ass"
	}
	if captionsNeedShrink(words, style, breakBefore) {
		return buildStyledASS(words, segmentStart, style, breakBefore, false), ".ass"
	}
	return buildSRT(words, segmentStart, breakBefore), ".srt"
}

// captionsNeedShrink reports whether any cue is too wide for the frame even on two lines at the
// size asked for. libass wraps SRT text by itself, so a cue that fits on two lines needs nothing.
func captionsNeedShrink(words []models.Word, style models.CaptionStyle, breakBefore []bool) bool {
	r := resolvedCaption(style)
	ps, ok := presetStyles[r.Preset]
	if !ok {
		ps = presetStyles[CaptionBold]
	}
	sizePx := int(float64(r.FontSize)*assScale + 0.5)
	outlinePx := presetOutlinePx(ps)
	font := measureFont(r.Font)
	for _, c := range groupCues(words, breakBefore) {
		texts := make([]string, 0, c.to-c.from)
		for k := c.from; k < c.to; k++ {
			texts = append(texts, words[k].Text)
		}
		if layoutCue(texts, font, sizePx, outlinePx).shrunk {
			return true
		}
	}
	return false
}

// BuildCaptions is Captions for a story video, whose words are already relative to the start of
// the video.
func BuildCaptions(words []models.Word, style models.CaptionStyle, breakBefore []bool) (doc, ext string) {
	return Captions(words, 0, style, breakBefore)
}

// SubtitlesFilter is the FFmpeg filter that burns the caption file at path. An ASS file carries
// its own style; an SRT file takes the preset's force_style. path is put inside the filter
// expression, so it must already be safe there (see storyvideo.escapeFilterPath).
func SubtitlesFilter(path string, style models.CaptionStyle) string {
	if isKaraokeStyle(style) || isASSPath(path) {
		return fmt.Sprintf("subtitles=%s", path)
	}
	return fmt.Sprintf("subtitles=%s:force_style='%s'", path, ResolveForceStyle(style))
}

// isASSPath reports whether a caption file is ASS, by its extension.
func isASSPath(path string) bool {
	return strings.EqualFold(filepath.Ext(path), ".ass")
}
