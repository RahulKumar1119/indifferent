package render

// Reusable caption styles for AI Shorts: one preset selected per job (stored
// on the ShortsJob record), honored by the render worker when burning
// captions. Sizes are libass script units on the SRT default play
// resolution (PlayResY 288); ASS output scales them to its own PlayRes.

import (
	"fmt"

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
	switch r.Preset {
	case CaptionMinimal:
		return fmt.Sprintf("Alignment=%d,FontSize=%d,PrimaryColour=%s,BorderStyle=1,Outline=1,Shadow=0,MarginV=45",
			captionAlignment(r), r.FontSize, hexToASS(r.Color))
	case CaptionClean:
		return fmt.Sprintf("Alignment=%d,FontSize=%d,PrimaryColour=%s,BorderStyle=1,Outline=1,Shadow=0,MarginV=50",
			captionAlignment(r), r.FontSize, hexToASS(r.Color))
	default: // bold: byte-identical to the historical burn
		return fmt.Sprintf("Alignment=%d,FontSize=%d,PrimaryColour=%s,BorderStyle=3,Outline=2",
			captionAlignment(r), r.FontSize, hexToASS(r.Color))
	}
}

// IsKaraoke reports whether captions must be built as ASS (word sweep)
// instead of SRT + force_style.
func isKaraokeStyle(s models.CaptionStyle) bool {
	return resolvedCaption(s).Preset == CaptionKaraoke
}
