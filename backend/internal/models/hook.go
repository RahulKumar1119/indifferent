package models

import (
	"fmt"
	"math"
	"strings"
	"unicode"
)

// ClipHook is the hook line burned into the first seconds of one clip (#329).
// It is off by default: a clip with no hook, or with Enabled false, renders
// exactly as it did before hooks existed.
type ClipHook struct {
	Enabled bool `json:"enabled"`
	// Text is what the hook says, in the clip's language. At most
	// MaxHookChars user-perceived characters (see HookTextLength), drawn on at
	// most two lines.
	Text string `json:"text"`
	// Style is box (bold on a dark box), outline (white with a dark outline) or
	// band (dark text on an accent band).
	Style string `json:"style"`
	// Position is top or middle. Bottom is where captions sit.
	Position string `json:"position"`
	// Seconds is how long the hook shows from the start of the clip.
	Seconds   float64 `json:"seconds"`
	UpdatedAt string  `json:"updatedAt,omitempty"`
}

// Hook limits and choices.
const (
	MaxHookChars       = 80
	MinHookSeconds     = 1.0
	MaxHookSeconds     = 5.0
	DefaultHookSeconds = 2.5

	HookStyleBox     = "box"
	HookStyleOutline = "outline"
	HookStyleBand    = "band"

	HookTop    = "top"
	HookMiddle = "middle"
)

// Active reports whether the hook should be drawn.
func (h *ClipHook) Active() bool {
	return h != nil && h.Enabled && strings.TrimSpace(h.Text) != ""
}

// Normalized fills the defaults a request may leave out: the box style, the
// top position and 2.5 seconds. The text is trimmed.
func (h ClipHook) Normalized() ClipHook {
	h.Text = strings.TrimSpace(h.Text)
	if h.Style == "" {
		h.Style = HookStyleBox
	}
	if h.Position == "" {
		h.Position = HookTop
	}
	if h.Seconds == 0 {
		h.Seconds = DefaultHookSeconds
	}
	return h
}

// Validate checks a normalized hook. A hook that is switched off may have no
// text, so it can be saved half written; one that is on needs some.
func (h ClipHook) Validate() error {
	switch h.Style {
	case HookStyleBox, HookStyleOutline, HookStyleBand:
	default:
		return fmt.Errorf("unknown hook style %q: must be box, outline or band", h.Style)
	}
	switch h.Position {
	case HookTop, HookMiddle:
	default:
		return fmt.Errorf("unknown hook position %q: must be top or middle", h.Position)
	}
	if math.IsNaN(h.Seconds) || math.IsInf(h.Seconds, 0) || h.Seconds < MinHookSeconds || h.Seconds > MaxHookSeconds {
		return fmt.Errorf("the hook shows for %g to %g seconds", MinHookSeconds, MaxHookSeconds)
	}
	if strings.IndexFunc(h.Text, func(r rune) bool { return unicode.IsControl(r) || r == '\u2028' || r == '\u2029' }) >= 0 {
		return fmt.Errorf("the hook text cannot contain a line break; it is split into lines for you")
	}
	if n := HookTextLength(h.Text); n > MaxHookChars {
		return fmt.Errorf("the hook text is %d characters; the limit is %d", n, MaxHookChars)
	}
	if h.Enabled && strings.TrimSpace(h.Text) == "" {
		return fmt.Errorf("write the hook text, or switch the hook off")
	}
	return nil
}

// HookTextLength counts the characters a reader sees. A Hindi vowel sign, a
// nasal mark or a virama belongs to the letter before it, and a conjunct (a
// consonant, a virama and another consonant) is one character, so "क्षमा" is
// three. Combining marks, joiners, variation selectors, emoji skin tones and
// the second flag letter join the character before them too. This follows
// Unicode's extended grapheme clusters for the scripts the product supports.
func HookTextLength(s string) int {
	n := 0
	var prev rune
	afterZWJ := false
	regionalOpen := false
	for i, r := range s {
		joins := false
		switch {
		case i == 0:
		case unicode.In(r, unicode.Mn, unicode.Mc, unicode.Me):
			joins = true
		case r == '\u200c' || r == '\u200d':
			joins = true
		case r >= 0xFE00 && r <= 0xFE0F, r >= 0xE0100 && r <= 0xE01EF, r >= 0x1F3FB && r <= 0x1F3FF:
			joins = true
		case afterZWJ:
			joins = true
		case prev == '\u094d' && isDevanagariConsonant(r):
			joins = true
		case r == '\n' && prev == '\r':
			joins = true
		case isRegionalIndicator(r) && regionalOpen:
			joins = true
		}
		if isRegionalIndicator(r) {
			regionalOpen = !joins
		} else {
			regionalOpen = false
		}
		if !joins {
			n++
		}
		afterZWJ = r == '\u200d'
		prev = r
	}
	return n
}

func isDevanagariConsonant(r rune) bool {
	return (r >= 0x0915 && r <= 0x0939) || (r >= 0x0958 && r <= 0x095F) || (r >= 0x0978 && r <= 0x097F)
}

func isRegionalIndicator(r rune) bool { return r >= 0x1F1E6 && r <= 0x1F1FF }

// Single-clip render states.
const (
	ClipRenderRendering = "rendering"
	ClipRenderDone      = "done"
	ClipRenderFailed    = "failed"
)

// ClipRenderView is what the page is told about a clip's latest single-clip
// render. Keys, tokens and request ids stay on the server.
type ClipRenderView struct {
	State       string `json:"state"`
	RequestedAt string `json:"requestedAt,omitempty"`
	RenderedAt  string `json:"renderedAt,omitempty"`
	Error       string `json:"error,omitempty"`
	// Renders counts the single-clip renders that finished, so the page can
	// tell a new file from the one it is already playing.
	Renders int `json:"renders,omitempty"`
}
