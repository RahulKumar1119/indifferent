package thumbnail

import (
	"image/color"
	"math"
)

// FontKind chooses one of the typefaces built into the binary. Using embedded
// fonts keeps rendering identical everywhere and adds nothing to the image.
type FontKind string

const (
	FontBold   FontKind = "bold"   // Go Bold
	FontMedium FontKind = "medium" // Go Medium
	FontMono   FontKind = "mono"   // Go Mono Bold
)

// Look is everything a style preset decides: type, colours and the grade
// applied to the video frame.
type Look struct {
	Style     Style
	Font      FontKind
	Headline  color.RGBA // headline fill
	Stroke    color.RGBA // outline and shadow colour
	StrokeEm  float64    // outline width as a share of the font size; 0 means none
	Accent    color.RGBA // tag background, rules, card border
	AccentInk color.RGBA // text drawn on the accent colour
	Shade     color.RGBA // scrim laid behind text
	ShadeA    float64    // peak scrim opacity, 0 to 1
	Sat       float64    // saturation multiplier
	Contrast  float64    // contrast multiplier
	Bright    float64    // brightness multiplier
	Vignette  float64    // edge darkening, 0 to 1
	Letterbox bool       // cinematic bars top and bottom
}

func rgb(hex uint32) color.RGBA {
	return color.RGBA{R: uint8(hex >> 16), G: uint8(hex >> 8), B: uint8(hex), A: 255}
}

// LookFor returns the look for a style. An unknown style gets Viral.
func LookFor(s Style) Look {
	switch s {
	case StyleTechnology:
		return Look{Style: s, Font: FontMono,
			Headline: rgb(0xFFFFFF), Stroke: rgb(0x020617), StrokeEm: 0.05,
			Accent: rgb(0x22D3EE), AccentInk: rgb(0x042F3A),
			Shade: rgb(0x050B1F), ShadeA: 0.86, Sat: 1.12, Contrast: 1.08, Bright: 0.96, Vignette: 0.35}
	case StyleEducational:
		return Look{Style: s, Font: FontBold,
			Headline: rgb(0xFFFFFF), Stroke: rgb(0x0B1B3A), StrokeEm: 0.055,
			Accent: rgb(0xFBBF24), AccentInk: rgb(0x1F1300),
			Shade: rgb(0x0B1B3A), ShadeA: 0.84, Sat: 1.05, Contrast: 1.04, Bright: 1.0, Vignette: 0.2}
	case StyleNews:
		return Look{Style: s, Font: FontBold,
			Headline: rgb(0xFFFFFF), Stroke: rgb(0x000000), StrokeEm: 0.03,
			Accent: rgb(0xDC2626), AccentInk: rgb(0xFFFFFF),
			Shade: rgb(0x0A0A0A), ShadeA: 0.88, Sat: 0.96, Contrast: 1.06, Bright: 0.98, Vignette: 0.25}
	case StylePodcast:
		return Look{Style: s, Font: FontBold,
			Headline: rgb(0xFFFFFF), Stroke: rgb(0x1E0B33), StrokeEm: 0.05,
			Accent: rgb(0xF472B6), AccentInk: rgb(0x2A0719),
			Shade: rgb(0x1E0B33), ShadeA: 0.85, Sat: 1.1, Contrast: 1.05, Bright: 0.97, Vignette: 0.3}
	case StyleCinematic:
		return Look{Style: s, Font: FontMedium,
			Headline: rgb(0xF5F1E8), Stroke: rgb(0x000000), StrokeEm: 0.02,
			Accent: rgb(0xE8B04B), AccentInk: rgb(0x120C00),
			Shade: rgb(0x000000), ShadeA: 0.82, Sat: 0.88, Contrast: 1.16, Bright: 0.92, Vignette: 0.55, Letterbox: true}
	case StyleMinimal:
		return Look{Style: s, Font: FontMedium,
			Headline: rgb(0xFFFFFF), Stroke: rgb(0x000000), StrokeEm: 0,
			Accent: rgb(0xFFFFFF), AccentInk: rgb(0x111111),
			Shade: rgb(0x000000), ShadeA: 0.78, Sat: 0.95, Contrast: 1.0, Bright: 0.96, Vignette: 0.1}
	default: // viral
		return Look{Style: StyleViral, Font: FontBold,
			Headline: rgb(0xFFE600), Stroke: rgb(0x000000), StrokeEm: 0.085,
			Accent: rgb(0xD61F1F), AccentInk: rgb(0xFFFFFF),
			Shade: rgb(0x000000), ShadeA: 0.82, Sat: 1.28, Contrast: 1.12, Bright: 1.02, Vignette: 0.28}
	}
}

// ResolveStyle picks the style to render with: the user's choice if they made
// one, else the model's suggestion, else Viral.
func ResolveStyle(requested, suggested Style) Style {
	if requested != "" {
		return requested
	}
	if suggested != "" {
		return suggested
	}
	return StyleViral
}

// contrastRatio is the WCAG contrast ratio between two colours. Used by tests
// to make sure text stays readable over its scrim.
func contrastRatio(a, b color.RGBA) float64 {
	la, lb := luminance(a), luminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

func luminance(c color.RGBA) float64 {
	ch := func(v uint8) float64 {
		f := float64(v) / 255
		if f <= 0.03928 {
			return f / 12.92
		}
		return math.Pow((f+0.055)/1.055, 2.4)
	}
	return 0.2126*ch(c.R) + 0.7152*ch(c.G) + 0.0722*ch(c.B)
}
