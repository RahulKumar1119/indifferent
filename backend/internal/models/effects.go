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
	maxSpeedWindows    = 5
	speedMinRate       = 0.25
	speedMaxRate       = 4.0
	speedMinDuration   = 0.3
	speedMaxDuration   = effectMaxSeconds
	maxShakeWindows    = 10
	shakeMinDuration   = 0.2
	shakeMaxDuration   = 6.0
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
	Speed    *SpeedEffect      `json:"speed,omitempty"`
	Finish   *FinishEffect     `json:"finish,omitempty"`
	Waveform *WaveformEffect   `json:"waveform,omitempty"`
	Shake    *ShakeEffect      `json:"shake,omitempty"`
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

// SpeedEffect changes playback speed over chosen windows: slow motion below
// 1.0x, fast motion above. Outside a window the clip plays at 1.0x. Audio is
// retimed with it. Windows must not overlap.
type SpeedEffect struct {
	Enabled bool          `json:"enabled"`
	Windows []SpeedWindow `json:"windows"`
}

// SpeedWindow plays the source span [At, At+Duration) at Rate times speed.
// At and Duration are seconds on the source timeline; Rate is 0.25 to 4.0.
type SpeedWindow struct {
	At       float64 `json:"at"`
	Duration float64 `json:"duration"`
	Rate     float64 `json:"rate"`
}

// FinishEffect adds optional film grain, glow and light-leak looks over the
// whole clip. Each strength is 0-100; 0 turns that sub-effect off, so one
// FinishEffect can carry any mix of the three.
type FinishEffect struct {
	Enabled   bool `json:"enabled"`
	Grain     int  `json:"grain"`     // 0-100 film-grain strength
	Glow      int  `json:"glow"`      // 0-100 soft-glow strength
	LightLeak int  `json:"lightLeak"` // 0-100 light-leak wash strength
}

// WaveformEffect draws an audio visualizer onto the clip. It reads the clip's
// final (retimed) audio, so the bars track what is heard.
type WaveformEffect struct {
	Enabled  bool   `json:"enabled"`
	Style    string `json:"style"`    // waves|bars
	Color    string `json:"color"`    // #RRGGBB
	Position string `json:"position"` // top|middle|bottom
	Height   int    `json:"height"`   // 10-50 percent of frame height
}

// ShakeEffect adds camera shake over chosen windows. Intensity sets how far the
// frame moves, bounded so the overscan margin never exposes a black edge.
type ShakeEffect struct {
	Enabled   bool          `json:"enabled"`
	Intensity int           `json:"intensity"` // 0-100
	Windows   []ShakeWindow `json:"windows"`
}

// ShakeWindow is one shake moment: it starts at At and lasts Duration seconds.
type ShakeWindow struct {
	At       float64 `json:"at"`
	Duration float64 `json:"duration"`
}

// Active reports whether at least one effect is enabled.
func (e *VideoEffects) Active() bool {
	return e != nil &&
		((e.Captions != nil && e.Captions.Enabled) ||
			(e.Zoom != nil && e.Zoom.Enabled) ||
			(e.Grade != nil && e.Grade.Enabled) ||
			(e.Freeze != nil && e.Freeze.Enabled) ||
			(e.Fade != nil && e.Fade.Enabled) ||
			(e.Speed != nil && e.Speed.Enabled) ||
			(e.Finish != nil && e.Finish.Enabled) ||
			(e.Waveform != nil && e.Waveform.Enabled) ||
			(e.Shake != nil && e.Shake.Enabled))
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
	if e.Speed != nil {
		if err := e.Speed.Validate(); err != nil {
			return err
		}
	}
	if e.Finish != nil {
		if err := e.Finish.Validate(); err != nil {
			return err
		}
	}
	if e.Waveform != nil {
		if err := e.Waveform.Validate(); err != nil {
			return err
		}
	}
	if e.Shake != nil {
		if err := e.Shake.Validate(); err != nil {
			return err
		}
	}
	// Freeze and speed both retime the clip by cutting it into pieces, and
	// combining the two on one clip is not supported yet. Reject the pair
	// only when both are enabled; a disabled effect keeps its saved values.
	if e.Freeze != nil && e.Freeze.Enabled && e.Speed != nil && e.Speed.Enabled {
		return fmt.Errorf("freeze frame and speed cannot both be on for the same clip yet")
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

// Validate checks the speed windows: rate range, count, finite non-overlapping
// times, and at least one window when enabled.
func (s SpeedEffect) Validate() error {
	if len(s.Windows) > maxSpeedWindows {
		return fmt.Errorf("too many speed windows: %d, the limit is %d", len(s.Windows), maxSpeedWindows)
	}
	if s.Enabled && len(s.Windows) == 0 {
		return fmt.Errorf("speed needs at least one window")
	}
	spans := make([]span, 0, len(s.Windows))
	for i, w := range s.Windows {
		if err := checkSeconds(fmt.Sprintf("speed window %d start", i+1), w.At); err != nil {
			return err
		}
		if err := checkFloatRange(fmt.Sprintf("speed window %d duration", i+1), w.Duration, speedMinDuration, speedMaxDuration); err != nil {
			return err
		}
		if err := checkRate(fmt.Sprintf("speed window %d rate", i+1), w.Rate, speedMinRate, speedMaxRate); err != nil {
			return err
		}
		spans = append(spans, span{w.At, w.At + w.Duration})
	}
	return checkNoOverlap("speed windows", spans)
}

// Validate checks the finish strengths and requires at least one sub-effect
// when enabled.
func (f FinishEffect) Validate() error {
	if err := checkIntRange("grain strength", f.Grain, 0, 100); err != nil {
		return err
	}
	if err := checkIntRange("glow strength", f.Glow, 0, 100); err != nil {
		return err
	}
	if err := checkIntRange("light leak strength", f.LightLeak, 0, 100); err != nil {
		return err
	}
	if f.Enabled && f.Grain == 0 && f.Glow == 0 && f.LightLeak == 0 {
		return fmt.Errorf("finish needs grain, glow or light leak above 0")
	}
	return nil
}

// Validate checks the waveform style, color, position and height.
func (w WaveformEffect) Validate() error {
	switch w.Style {
	case "waves", "bars":
	default:
		return fmt.Errorf("unknown waveform style %q: must be waves or bars", w.Style)
	}
	if !hexColor.MatchString(w.Color) {
		return fmt.Errorf("waveform color %q must be #RRGGBB", w.Color)
	}
	switch w.Position {
	case "top", "middle", "bottom":
	default:
		return fmt.Errorf("unknown waveform position %q: must be top, middle or bottom", w.Position)
	}
	return checkIntRange("waveform height", w.Height, 10, 50)
}

// Validate checks the shake intensity and the windows: count, finite
// non-overlapping times, and at least one window when enabled.
func (s ShakeEffect) Validate() error {
	if err := checkIntRange("shake intensity", s.Intensity, 0, 100); err != nil {
		return err
	}
	if len(s.Windows) > maxShakeWindows {
		return fmt.Errorf("too many shake windows: %d, the limit is %d", len(s.Windows), maxShakeWindows)
	}
	if s.Enabled && len(s.Windows) == 0 {
		return fmt.Errorf("shake needs at least one window")
	}
	spans := make([]span, 0, len(s.Windows))
	for i, w := range s.Windows {
		if err := checkSeconds(fmt.Sprintf("shake window %d start", i+1), w.At); err != nil {
			return err
		}
		if err := checkFloatRange(fmt.Sprintf("shake window %d duration", i+1), w.Duration, shakeMinDuration, shakeMaxDuration); err != nil {
			return err
		}
		spans = append(spans, span{w.At, w.At + w.Duration})
	}
	return checkNoOverlap("shake windows", spans)
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

// checkRate rejects a playback rate outside [lo, hi], including NaN and
// infinity. Separate from checkFloatRange, whose message names seconds.
func checkRate(what string, v, lo, hi float64) error {
	if math.IsNaN(v) || math.IsInf(v, 0) || v < lo || v > hi {
		return fmt.Errorf("%s %v must be between %gx and %gx", what, v, lo, hi)
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
