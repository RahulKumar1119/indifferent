package models

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Phase 1 video effect limits. Times are seconds from the start of a clip.
// effectMaxSeconds repeats shorts.MaxClipDuration (90s). The models package
// cannot import the shorts package without risking a cycle.
const (
	effectMaxSeconds   = 90.0
	maxPunchIns        = 10
	punchInMinDuration = 0.5
	punchInMaxDuration = 6.0
	maxFreezes         = 5
	freezeMinDuration  = 0.2
	freezeMaxDuration  = 3.0
	freezeMaxTotal     = 10.0
	freezeMinGap       = 0.1
	maxImpactTexts     = 5
	impactMinDuration  = 0.3
	impactMaxDuration  = 4.0
	impactMaxChars     = 16
	maxCaptionOutline  = 12
	fadeMaxDuration    = 2.0
)

// VideoEffects is the phase 1 effect configuration stored on a shorts job.
// A nil effect was never configured. Each effect has its own enabled flag, so
// a disabled effect keeps its saved values. A nil *VideoEffects means no
// effects, and renders exactly as it did before effects existed.
type VideoEffects struct {
	Captions *AnimatedCaptions `json:"captions,omitempty"`
	Zoom     *ZoomEffect       `json:"zoom,omitempty"`
	Grade    *GradeEffect      `json:"grade,omitempty"`
	Freeze   *FreezeEffect     `json:"freeze,omitempty"`
	Fade     *FadeEffect       `json:"fade,omitempty"`
}

// AnimatedCaptions draws burned-in captions with a word-by-word highlight.
// When enabled it replaces the job's caption style for that render.
type AnimatedCaptions struct {
	Enabled    bool   `json:"enabled"`
	Intensity  int    `json:"intensity"`  // 0-100, how far unspoken words fade
	Font       string `json:"font"`       // sans|serif|condensed
	FontSize   int    `json:"fontSize"`   // 8-72, same scale as the caption style
	Color      string `json:"color"`      // #RRGGBB text color
	Highlight  string `json:"highlight"`  // #RRGGBB color of the spoken word
	Position   string `json:"position"`   // top|middle|bottom
	Outline    int    `json:"outline"`    // 0-12 pixels at 1080x1920
	Background string `json:"background"` // none|dark|light
}

// ZoomEffect pushes in on the frame at the chosen punch-in times.
type ZoomEffect struct {
	Enabled   bool      `json:"enabled"`
	Intensity int       `json:"intensity"` // 0-100, peak zoom 1.0x to 1.5x
	PunchIns  []PunchIn `json:"punchIns"`
}

// PunchIn is one zoom moment: it starts at At and lasts Duration seconds.
type PunchIn struct {
	At       float64 `json:"at"`
	Duration float64 `json:"duration"`
}

// GradeEffect applies a color look scaled by Intensity.
type GradeEffect struct {
	Enabled   bool   `json:"enabled"`
	Look      string `json:"look"`      // cinematic|warm|cool|high-contrast
	Intensity int    `json:"intensity"` // 0-100
}

// FreezeEffect holds still frames and shows impact text. Intensity sets the
// size of the impact text.
type FreezeEffect struct {
	Enabled     bool         `json:"enabled"`
	Intensity   int          `json:"intensity"`
	Freezes     []FreezeHold `json:"freezes"`
	ImpactTexts []ImpactText `json:"impactTexts"`
}

// FreezeHold holds the frame at At for Duration seconds. Audio goes silent for
// the same time, and the rest of the clip moves later by that amount.
type FreezeHold struct {
	At       float64 `json:"at"`
	Duration float64 `json:"duration"`
}

// ImpactText shows one line of text from At for Duration seconds.
type ImpactText struct {
	At       float64 `json:"at"`
	Duration float64 `json:"duration"`
	Text     string  `json:"text"`
}

// FadeEffect fades the clip in at its start and out at its end. InDuration and
// OutDuration are seconds; 0 on a side disables that side. The fade covers the
// finished picture, captions and impact text included, and the audio fades with
// it. Timing is relative to the output length, so it already accounts for any
// frames freeze holds add.
type FadeEffect struct {
	Enabled     bool    `json:"enabled"`
	InDuration  float64 `json:"inDuration"`
	OutDuration float64 `json:"outDuration"`
}

// Active reports whether at least one effect is enabled.
func (e *VideoEffects) Active() bool {
	return e != nil &&
		((e.Captions != nil && e.Captions.Enabled) ||
			(e.Zoom != nil && e.Zoom.Enabled) ||
			(e.Grade != nil && e.Grade.Enabled) ||
			(e.Freeze != nil && e.Freeze.Enabled) ||
			(e.Fade != nil && e.Fade.Enabled))
}

// Validate checks every configured effect, enabled or not, so a saved value
// is always a value a render can use. It returns the first problem found.
func (e *VideoEffects) Validate() error {
	if e == nil {
		return nil
	}
	if e.Captions != nil {
		if err := e.Captions.Validate(); err != nil {
			return err
		}
	}
	if e.Zoom != nil {
		if err := e.Zoom.Validate(); err != nil {
			return err
		}
	}
	if e.Grade != nil {
		if err := e.Grade.Validate(); err != nil {
			return err
		}
	}
	if e.Freeze != nil {
		if err := e.Freeze.Validate(); err != nil {
			return err
		}
	}
	if e.Fade != nil {
		if err := e.Fade.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// Validate checks the animated caption settings.
func (c AnimatedCaptions) Validate() error {
	if err := checkIntRange("captions intensity", c.Intensity, 0, 100); err != nil {
		return err
	}
	switch c.Font {
	case "sans", "serif", "condensed":
	default:
		return fmt.Errorf("unknown caption font %q: must be sans, serif or condensed", c.Font)
	}
	if err := checkIntRange("caption font size", c.FontSize, 8, 72); err != nil {
		return err
	}
	if !hexColor.MatchString(c.Color) {
		return fmt.Errorf("caption color %q must be #RRGGBB", c.Color)
	}
	if !hexColor.MatchString(c.Highlight) {
		return fmt.Errorf("caption highlight color %q must be #RRGGBB", c.Highlight)
	}
	switch c.Position {
	case "top", "middle", "bottom":
	default:
		return fmt.Errorf("unknown caption position %q: must be top, middle or bottom", c.Position)
	}
	if err := checkIntRange("caption outline", c.Outline, 0, maxCaptionOutline); err != nil {
		return err
	}
	switch c.Background {
	case "none", "dark", "light":
	default:
		return fmt.Errorf("unknown caption background %q: must be none, dark or light", c.Background)
	}
	return nil
}

// Validate checks the zoom settings and the punch-in times.
func (z ZoomEffect) Validate() error {
	if err := checkIntRange("zoom intensity", z.Intensity, 0, 100); err != nil {
		return err
	}
	if len(z.PunchIns) > maxPunchIns {
		return fmt.Errorf("too many punch-ins: %d, the limit is %d", len(z.PunchIns), maxPunchIns)
	}
	if z.Enabled && len(z.PunchIns) == 0 {
		return fmt.Errorf("zoom needs at least one punch-in time")
	}
	spans := make([]span, 0, len(z.PunchIns))
	for i, p := range z.PunchIns {
		if err := checkSeconds(fmt.Sprintf("punch-in %d start", i+1), p.At); err != nil {
			return err
		}
		if err := checkFloatRange(fmt.Sprintf("punch-in %d duration", i+1), p.Duration, punchInMinDuration, punchInMaxDuration); err != nil {
			return err
		}
		spans = append(spans, span{p.At, p.At + p.Duration})
	}
	return checkNoOverlap("punch-ins", spans)
}

// Validate checks the grade look and intensity.
func (g GradeEffect) Validate() error {
	switch g.Look {
	case "cinematic", "warm", "cool", "high-contrast":
	default:
		return fmt.Errorf("unknown grade look %q: must be cinematic, warm, cool or high-contrast", g.Look)
	}
	return checkIntRange("grade intensity", g.Intensity, 0, 100)
}

// Validate checks the freeze holds, the impact texts and the impact size.
func (f FreezeEffect) Validate() error {
	if err := checkIntRange("freeze intensity", f.Intensity, 0, 100); err != nil {
		return err
	}
	if len(f.Freezes) > maxFreezes {
		return fmt.Errorf("too many freezes: %d, the limit is %d", len(f.Freezes), maxFreezes)
	}
	total := 0.0
	starts := make([]float64, 0, len(f.Freezes))
	for i, h := range f.Freezes {
		if err := checkSeconds(fmt.Sprintf("freeze %d time", i+1), h.At); err != nil {
			return err
		}
		if err := checkFloatRange(fmt.Sprintf("freeze %d duration", i+1), h.Duration, freezeMinDuration, freezeMaxDuration); err != nil {
			return err
		}
		total += h.Duration
		starts = append(starts, h.At)
	}
	if total > freezeMaxTotal {
		return fmt.Errorf("freezes add up to %.1fs, the limit is %.0fs", total, freezeMaxTotal)
	}
	sort.Float64s(starts)
	for i := 1; i < len(starts); i++ {
		if starts[i]-starts[i-1] < freezeMinGap {
			return fmt.Errorf("freezes at %.2fs and %.2fs are less than %.1fs apart", starts[i-1], starts[i], freezeMinGap)
		}
	}
	if len(f.ImpactTexts) > maxImpactTexts {
		return fmt.Errorf("too many impact texts: %d, the limit is %d", len(f.ImpactTexts), maxImpactTexts)
	}
	spans := make([]span, 0, len(f.ImpactTexts))
	for i, t := range f.ImpactTexts {
		if err := checkSeconds(fmt.Sprintf("impact text %d time", i+1), t.At); err != nil {
			return err
		}
		if err := checkFloatRange(fmt.Sprintf("impact text %d duration", i+1), t.Duration, impactMinDuration, impactMaxDuration); err != nil {
			return err
		}
		if err := validateImpactText(i+1, t.Text); err != nil {
			return err
		}
		spans = append(spans, span{t.At, t.At + t.Duration})
	}
	return checkNoOverlap("impact texts", spans)
}

// Validate checks the fade durations. Each side is between 0 and
// fadeMaxDuration seconds, and an enabled fade needs at least one side.
func (f FadeEffect) Validate() error {
	if err := checkFloatRange("fade in duration", f.InDuration, 0, fadeMaxDuration); err != nil {
		return err
	}
	if err := checkFloatRange("fade out duration", f.OutDuration, 0, fadeMaxDuration); err != nil {
		return err
	}
	if f.Enabled && f.InDuration == 0 && f.OutDuration == 0 {
		return fmt.Errorf("fade needs a fade-in or fade-out longer than 0 seconds")
	}
	return nil
}

// validateImpactText accepts one non-empty line of at most impactMaxChars
// characters. Any punctuation is allowed. Control characters, including
// newlines and tabs, are not, because impact text is drawn on a single line.
func validateImpactText(n int, s string) error {
	if strings.TrimSpace(s) == "" {
		return fmt.Errorf("impact text %d is empty", n)
	}
	if !utf8.ValidString(s) {
		return fmt.Errorf("impact text %d is not valid UTF-8", n)
	}
	if utf8.RuneCountInString(s) > impactMaxChars {
		return fmt.Errorf("impact text %d is longer than %d characters", n, impactMaxChars)
	}
	for _, r := range s {
		if unicode.IsControl(r) {
			return fmt.Errorf("impact text %d must be one line without tabs or line breaks", n)
		}
	}
	return nil
}

// span is a time window in seconds, used for overlap checks.
type span struct{ start, end float64 }

// checkNoOverlap rejects windows that overlap once sorted by start time.
func checkNoOverlap(what string, spans []span) error {
	sort.Slice(spans, func(i, j int) bool { return spans[i].start < spans[j].start })
	for i := 1; i < len(spans); i++ {
		if spans[i].start < spans[i-1].end {
			return fmt.Errorf("%s overlap: %.2fs to %.2fs overlaps %.2fs to %.2fs",
				what, spans[i-1].start, spans[i-1].end, spans[i].start, spans[i].end)
		}
	}
	return nil
}

// checkSeconds rejects times outside a clip's possible range, including NaN
// and infinity.
func checkSeconds(what string, v float64) error {
	if math.IsNaN(v) || v < 0 || v > effectMaxSeconds {
		return fmt.Errorf("%s %v must be between 0 and %.0f seconds", what, v, effectMaxSeconds)
	}
	return nil
}

// checkFloatRange rejects values outside [lo, hi], including NaN.
func checkFloatRange(what string, v, lo, hi float64) error {
	if math.IsNaN(v) || v < lo || v > hi {
		return fmt.Errorf("%s %v must be between %g and %g seconds", what, v, lo, hi)
	}
	return nil
}

// checkIntRange rejects integers outside [lo, hi].
func checkIntRange(what string, v, lo, hi int) error {
	if v < lo || v > hi {
		return fmt.Errorf("%s %d out of range %d-%d", what, v, lo, hi)
	}
	return nil
}
