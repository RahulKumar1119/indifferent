package render

// The hook line (#329): a short line of text burned into the first seconds of a
// clip, drawn by libass as its own ASS file, after the captions.
//
// It reuses the caption fitting from #327 (fit.go): the text is measured with
// the metrics of the fonts the render images have, wrapped onto two lines at a
// space, and made smaller only when two lines are not enough. A line is never
// broken inside a word, so a Hindi letter, its vowel signs and a conjunct always
// stay together. Text that does not fit two lines at the smallest size is not
// drawn at all (the API refuses to save it, so this only guards old records).
//
// The hook is placed in its slot (top or middle) unless the captions are there,
// in which case it moves just above or below them. It never covers the captions
// and never goes into the bottom 15% of the frame, where the platforms draw
// their buttons. The impact text of the Freeze effect is a separate drawtext
// layer and is not changed by any of this.

import (
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/rahul/indifferent/backend/internal/models"
)

const (
	// hookBasePx is the hook's size when the text fits at it, in pixels.
	hookBasePx = 80
	// hookFloorPx is the smallest size a hook is drawn at.
	hookFloorPx = 40
	// hookBoldWidth is how much wider the bold hook font is than the regular
	// font the tables measure. DejaVu Sans Bold is about 10% wider than DejaVu
	// Sans and Noto Sans Devanagari Bold about 5%; 15% errs on the side of
	// fitting, and the real renders check it (scripts/ops/hook_fit_check.sh).
	hookBoldWidth = 1.15
	// hookLineHeight is one line's height for a size of 1 pixel. libass makes
	// the ASS size the height of the font's line; the extra 10% covers marks
	// above and below a Hindi line.
	hookLineHeight = 1.1
	// hookTopSafePx keeps the hook below the bar the platforms draw at the top.
	hookTopSafePx = 160
	// hookBottomSafePx keeps the hook above the platforms' buttons and text.
	hookBottomSafePx = aboveControlsMarginPx
	// hookCaptionGapPx is the least space between the hook and the captions.
	hookCaptionGapPx = 24
	// hookFadeMs is the fade in and the fade out.
	hookFadeMs = 200
	// captionSlackPx is added above and below the captions' estimated band, for
	// shadows and the box libass draws round a bold caption.
	captionSlackPx = 20
)

// ErrHookTooLong means the text does not fit on two lines at the smallest size.
var ErrHookTooLong = errors.New("the hook text does not fit on two lines; shorten it")

// ErrHookNoRoom means there is no place for the hook that keeps it clear of the
// captions.
var ErrHookNoRoom = errors.New("there is no room for the hook beside these captions; make the captions smaller or move them")

// hookLook is how one hook style is drawn.
type hookLook struct {
	primary, outline, back string
	borderStyle            int
	// pad is the outline width or the box padding, in pixels.
	pad    int
	shadow int
}

var hookLooks = map[string]hookLook{
	// Bold white text on a dark box, 80% opaque.
	models.HookStyleBox: {primary: "&H00FFFFFF", outline: "&H33000000", back: "&H33000000", borderStyle: 3, pad: 18},
	// White text with a dark outline and a soft shadow, no box.
	models.HookStyleOutline: {primary: "&H00FFFFFF", outline: "&H00000000", back: "&H80000000", borderStyle: 1, pad: 6, shadow: 3},
	// Near-black text on a band of the accent yellow (#FFD60A).
	models.HookStyleBand: {primary: "&H00111111", outline: "&H000AD6FF", back: "&H000AD6FF", borderStyle: 3, pad: 22},
}

// HookCaptions describes the captions a hook must stay clear of.
type HookCaptions struct {
	// None is true when the clip has no captions.
	None bool
	// Animated, when set, are the captions drawn instead of Style.
	Animated *models.AnimatedCaptions
	Style    models.CaptionStyle
}

// HookCaptionsFor is the captions a job's renders draw: animated captions when
// that effect is on, otherwise the caption style.
func HookCaptionsFor(style models.CaptionStyle, effects *models.VideoEffects) HookCaptions {
	if effects.Active() && effects.Captions != nil && effects.Captions.Enabled {
		c := *effects.Captions
		return HookCaptions{Animated: &c}
	}
	return HookCaptions{Style: style}
}

// band is a stretch of the frame from y0 to y1, in pixels from the top.
type band struct{ y0, y1 float64 }

func (b band) overlaps(o band) bool { return b.y0 < o.y1 && o.y0 < b.y1 }

// captionBand estimates where the captions can be. It assumes the largest a cue
// can be: two lines at the size asked for (a cue is only ever made smaller).
func captionBand(c HookCaptions) (band, bool) {
	if c.None {
		return band{}, false
	}
	var sizePx, padY, marginV float64
	var align int
	if a := c.Animated; a != nil {
		sizePx = float64(a.FontSize) * assScale
		padY = float64(a.Outline)
		marginV = 90 // BuildAnimatedASS
		align = captionAlignment(models.CaptionStyle{Position: a.Position})
	} else {
		r := resolvedCaption(c.Style)
		sizePx = float64(int(float64(r.FontSize)*assScale + 0.5))
		align = captionAlignment(r)
		if isKaraokeStyle(r) {
			padY = 3
			marginV = legacyKaraokeMarginPx
			if align == 2 {
				marginV = float64(bottomMarginPx(r, legacyKaraokeMarginPx))
			}
		} else {
			ps, ok := presetStyles[r.Preset]
			if !ok {
				ps = presetStyles[CaptionBold]
			}
			padY = float64(presetOutlineYPx(ps))
			marginV = float64(forceMarginV(r, ps.marginV)) * assScale
		}
	}
	h := float64(maxCaptionLines)*sizePx*hookLineHeight + 2*padY + 2*captionSlackPx
	switch align {
	case 8:
		return band{marginV - captionSlackPx, marginV + h}, true
	case 5:
		return band{float64(assPlayResY)/2 - h/2, float64(assPlayResY)/2 + h/2}, true
	default:
		y1 := float64(assPlayResY) - marginV + captionSlackPx
		return band{y1 - h, y1}, true
	}
}

// hookPlan is a hook laid out and placed.
type hookPlan struct {
	sizePx  int
	breakAt int
	lines   int
	// top is the y of the top of the text (the box or outline is round it).
	top  float64
	look hookLook
}

// hookBlockHeight is the hook's height with its box or outline.
func hookBlockHeight(sizePx, lines int, look hookLook) float64 {
	return float64(lines)*float64(sizePx)*hookLineHeight + 2*float64(look.pad) + float64(look.shadow)
}

// layoutHookText finds where the text breaks at sizePx, or false when it does
// not fit on two lines at that size.
func layoutHookText(words []string, sizePx int, look hookLook) (int, bool) {
	limit := float64(frameWidth - 2*captionSideMarginPx - 2*look.pad - look.shadow)
	measured := int(math.Ceil(float64(sizePx) * hookBoldWidth))
	return splitCue(words, fontDefault, measured, limit)
}

// planHook lays out and places a normalized hook. It tries the largest size
// first, and makes the hook smaller when the text is too wide or when there is
// no room for it beside the captions.
func planHook(h models.ClipHook, caps HookCaptions) (hookPlan, error) {
	look, ok := hookLooks[h.Style]
	if !ok {
		return hookPlan{}, fmt.Errorf("unknown hook style %q", h.Style)
	}
	words := strings.Fields(h.Text)
	if len(words) == 0 {
		return hookPlan{}, ErrHookTooLong
	}
	capBand, hasCaps := captionBand(caps)
	fitsWidth := false
	for size := hookBasePx; size >= hookFloorPx; size -= shrinkStepPx {
		breakAt, ok := layoutHookText(words, size, look)
		if !ok {
			continue
		}
		fitsWidth = true
		lines := 1
		if breakAt > 0 {
			lines = 2
		}
		height := hookBlockHeight(size, lines, look)
		if top, ok := placeHook(h.Position, height, look, capBand, hasCaps); ok {
			return hookPlan{sizePx: size, breakAt: breakAt, lines: lines, top: top, look: look}, nil
		}
	}
	if !fitsWidth {
		return hookPlan{}, ErrHookTooLong
	}
	return hookPlan{}, ErrHookNoRoom
}

// placeHook returns the top of the hook's text: in its slot when that is clear
// of the captions, otherwise as close to it as it can be, just above or just
// below them. false means no place is clear.
func placeHook(position string, height float64, look hookLook, caps band, hasCaps bool) (float64, bool) {
	minY := float64(hookTopSafePx)
	maxY := float64(assPlayResY-hookBottomSafePx) - height
	want := minY
	if position == models.HookMiddle {
		want = float64(assPlayResY)/2 - height/2
	}
	candidates := []float64{want}
	if hasCaps {
		candidates = append(candidates, caps.y0-hookCaptionGapPx-height, caps.y1+hookCaptionGapPx)
		// Nearest to the slot first.
		if math.Abs(candidates[2]-want) < math.Abs(candidates[1]-want) {
			candidates[1], candidates[2] = candidates[2], candidates[1]
		}
	}
	for _, y := range candidates {
		if y < minY || y > maxY {
			continue
		}
		block := band{y, y + height}
		if hasCaps && block.overlaps(band{caps.y0 - hookCaptionGapPx, caps.y1 + hookCaptionGapPx}) {
			continue
		}
		return y + float64(look.pad), true
	}
	return 0, false
}

// CheckHook reports whether a hook can be drawn on a job's clips: its text fits
// two lines and it has a place clear of the captions. An inactive hook is
// always fine.
func CheckHook(h models.ClipHook, style models.CaptionStyle, effects *models.VideoEffects) error {
	if !h.Active() {
		return nil
	}
	_, err := planHook(h.Normalized(), HookCaptionsFor(style, effects))
	return err
}

// hookFade is the fade in and out for a hook shown for durMs, in milliseconds.
// The two never overlap: on a very short showing each takes at most half.
func hookFade(durMs int) int {
	if half := durMs / 2; half < hookFadeMs {
		return half
	}
	return hookFadeMs
}

// BuildHookASS writes the ASS file for an active hook on a clip whose output is
// outSec long. The hook shows from 0 for its seconds, or for the whole clip when
// that is shorter, fading in and out over 0.2 s.
func BuildHookASS(h models.ClipHook, caps HookCaptions, outSec float64) (string, error) {
	h = h.Normalized()
	p, err := planHook(h, caps)
	if err != nil {
		return "", err
	}
	end := h.Seconds
	if outSec > 0 && outSec < end {
		end = outSec
	}
	fade := hookFade(int(end*1000 + 0.5))
	words := strings.Fields(h.Text)
	parts := make([]string, len(words))
	for i, w := range words {
		parts[i] = assEscape(w)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "[Script Info]\nScriptType: v4.00+\nPlayResX: %d\nPlayResY: %d\nWrapStyle: 2\nScaledBorderAndShadow: yes\n\n",
		assPlayResX, assPlayResY)
	b.WriteString("[V4+ Styles]\nFormat: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding\n")
	fmt.Fprintf(&b, "Style: Hook,Arial,%d,%s,%s,%s,%s,-1,0,0,0,100,100,0,0,%d,%d,%d,8,0,0,0,1\n\n",
		p.sizePx, p.look.primary, p.look.primary, p.look.outline, p.look.back, p.look.borderStyle, p.look.pad, p.look.shadow)
	b.WriteString("[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n")
	fmt.Fprintf(&b, "Dialogue: 1,%s,%s,Hook,,0,0,0,,{\\an8\\pos(%d,%d)\\fad(%d,%d)}%s\n",
		formatASSTime(0), formatASSTime(end), assPlayResX/2, int(math.Round(p.top)), fade, fade, joinCue(parts, p.breakAt))
	return b.String(), nil
}
