package render

// Caption fitting (#327). A 9:16 frame is 1080 pixels wide, and libass does not shrink or wrap text
// that is too wide for it: the karaoke ASS file turns wrapping off, and a single word wider than
// the frame is clipped by every preset. So each cue is measured before it is written, and a cue
// that would run off the frame is put on two lines, and made smaller only if two lines are not
// enough. A cue that already fits is left exactly as it was.
//
// Measuring does not need a font file at run time: metrics_gen.go holds the width of every
// character in the fonts the render images use (see scripts/ops/gen_caption_metrics.py).
//
// Languages. The product captions English and Hindi only: AI Shorts transcribes Auto, English (US),
// English (India) and Hindi, and story videos are narrated in English and Hindi. English is
// measured exactly. Hindi is measured by adding up each character's own width, which is never
// narrower than the shaped text (conjuncts draw narrower than the sum), so the fit errs on the
// side of fitting. Right-to-left languages such as Arabic, and other Indian scripts such as
// Gujarati, are NOT supported: they need right-to-left word order for the karaoke sweep, shaping,
// and fonts the images do not have. When a language is added, handle all three together; the test
// in languages_test.go stops the lists changing without anyone reading this.

import (
	"strings"
)

const (
	// frameWidth is the width of the rendered frame, in pixels.
	frameWidth = assPlayResX
	// captionSideMarginPx is kept clear between a line (outline and box included) and each side of the
	// frame. It is small on purpose: this is what stops a caption being clipped or touching the
	// edge, not a design margin, and a caption that already sat inside it must not be changed.
	captionSideMarginPx = 20
	// maxCaptionLines is the most lines a cue may take. A third line covers too much of the picture.
	maxCaptionLines = 2
	// minCaptionUnits is the smallest size a cue is shrunk to, in the 8 to 72 units of the style.
	minCaptionUnits = 6
	// shrinkStepPx is how far the size steps down while looking for one that fits.
	shrinkStepPx = 2
	// defaultAdvance is the width, in thousandths of an em, of a character the tables do not list.
	defaultAdvance = 620
	// wideAdvance is the width of a full-width character (Chinese, Japanese, Korean).
	wideAdvance = 1000
)

// capFont is the family a caption is drawn in, for measuring.
type capFont int

const (
	// fontDefault is the font captions have always had: "Arial", which the render images answer
	// with Noto Sans Devanagari for any character it has (the space, digits, most punctuation and
	// Hindi) and DejaVu Sans for the rest.
	fontDefault capFont = iota
	fontSans
	fontSerif
	fontCondensed
)

// Caption font choices, as stored in models.CaptionStyle.Font. Empty keeps the font the captions
// have always had: Arial, which the render images answer with DejaVu Sans.
const (
	FontSans      = "sans"
	FontSerif     = "serif"
	FontCondensed = "condensed"
)

// fontFamilies are the DejaVu families the render images install, by choice.
var fontFamilies = map[string]string{
	FontSans:      "DejaVu Sans",
	FontSerif:     "DejaVu Serif",
	FontCondensed: "DejaVu Sans Condensed",
}

func measureFont(choice string) capFont {
	switch choice {
	case FontSerif:
		return fontSerif
	case FontCondensed:
		return fontCondensed
	case FontSans:
		return fontSans
	}
	return fontDefault
}

// notoAdvance is the width of r in Noto Sans Devanagari, and whether that font has it.
func notoAdvance(r rune) (int, bool) {
	var w uint16 = notoAbsent
	switch {
	case r >= notoLatinFirst && r <= notoLatinLast:
		w = advNotoLatin[r-notoLatinFirst]
	case r >= notoPunctFirst && r <= notoPunctLast:
		w = advNotoPunct[r-notoPunctFirst]
	case r >= devaFirst && r <= devaLast:
		return int(advDeva[r-devaFirst]), true
	}
	if w == notoAbsent {
		return 0, false
	}
	return int(w), true
}

// advance is the width of r in thousandths of an em.
func advance(f capFont, r rune) int {
	if f == fontDefault {
		if w, ok := notoAdvance(r); ok {
			return w
		}
	}
	switch {
	case r >= latinFirst && r <= latinLast:
		var w uint16
		switch f {
		case fontSerif:
			w = advSerif[r-latinFirst]
		case fontCondensed:
			w = advCondensed[r-latinFirst]
		default:
			w = advSans[r-latinFirst]
		}
		if w == 0 && r != ' ' {
			return defaultAdvance
		}
		return int(w)
	case r >= devaFirst && r <= devaLast:
		return int(advDeva[r-devaFirst])
	case r == '\u200c' || r == '\u200d': // joiners shape a letter and take no room
		return 0
	case r >= 0x2E80 && r <= 0x9FFF, r >= 0xAC00 && r <= 0xD7AF, r >= 0xFF00 && r <= 0xFFEF:
		return wideAdvance
	}
	return defaultAdvance
}

// widthMargin makes every estimate one percent wider than the measured ink, so rounding in the
// tables or a hinting difference never lets a line through that is a pixel too wide. Against real
// libass renders the estimate is within 0.4% of the ink (see scripts/ops/caption_fit_check.sh).
const widthMargin = 1.01

// cellHeight is how many ems tall libass makes a font of ASS size 1: the size is the height of the
// whole line of the font (its cell), not of an em, so an em is the size divided by the cell.
func cellHeight(f capFont, r rune) float64 {
	if r >= devaFirst && r <= devaLast {
		return cellDeva
	}
	if f == fontDefault {
		if _, ok := notoAdvance(r); ok {
			return cellDeva
		}
	}
	switch f {
	case fontSerif:
		return cellSerif
	case fontCondensed:
		return cellCondensed
	}
	return cellSans
}

// textWidth is how wide s is when drawn at ASS size sizePx, in pixels. Hindi is drawn by a second
// font with a taller cell, so its letters are smaller than Latin letters of the same size; each
// character is measured in the font that draws it.
func textWidth(f capFont, s string, sizePx int) float64 {
	total := 0.0
	for _, r := range s {
		total += float64(advance(f, r)) / 1000 * float64(sizePx) / cellHeight(f, r)
	}
	return total * widthMargin
}

// cueLayout is how a cue is drawn.
type cueLayout struct {
	// sizePx is the font size in pixels, which is the size asked for unless the cue had to shrink.
	sizePx int
	// breakAt is the index of the word that starts the second line, or 0 for one line.
	breakAt int
	// shrunk is true when sizePx is smaller than the size asked for.
	shrunk bool
	// fits is false only when the cue does not fit even at the smallest size.
	fits bool
}

// layoutCue decides how to draw words so every line fits inside the frame. outlinePx is the width
// of the outline (or of the box padding) round the text, which takes room on both sides.
func layoutCue(words []string, f capFont, sizePx, outlinePx int) cueLayout {
	limit := float64(frameWidth - 2*captionSideMarginPx - 2*outlinePx)
	units := float64(minCaptionUnits)
	floor := int(units*assScale + 0.5)
	if floor > sizePx {
		floor = sizePx
	}
	for size := sizePx; ; size -= shrinkStepPx {
		if size < floor {
			size = floor
		}
		if at, ok := splitCue(words, f, size, limit); ok {
			return cueLayout{sizePx: size, breakAt: at, shrunk: size < sizePx, fits: true}
		}
		if size == floor {
			at, _ := splitCue(words, f, size, limit)
			return cueLayout{sizePx: size, breakAt: at, shrunk: size < sizePx, fits: false}
		}
	}
}

// splitCue reports whether words fit on one line or two at sizePx, and where the second line
// starts (0 for one line). Two lines are split where they come out most even, which is also where
// libass puts the break when it wraps text by itself.
func splitCue(words []string, f capFont, sizePx int, limit float64) (int, bool) {
	if len(words) == 0 {
		return 0, true
	}
	space := textWidth(f, " ", sizePx)
	widths := make([]float64, len(words))
	total := 0.0
	for i, w := range words {
		widths[i] = textWidth(f, w, sizePx)
		total += widths[i]
	}
	total += space * float64(len(words)-1)
	if total <= limit {
		return 0, true
	}
	best, bestWidth := 0, 0.0
	left := 0.0
	for k := 1; k < len(words); k++ {
		left += widths[k-1]
		if k > 1 {
			left += space
		}
		right := total - left - space
		w := left
		if right > w {
			w = right
		}
		if best == 0 || w < bestWidth {
			best, bestWidth = k, w
		}
	}
	if best == 0 {
		return 0, false
	}
	return best, bestWidth <= limit
}

// joinCue joins a cue's pieces with spaces, and with a line break at breakAt (the index of the
// piece that starts the second line). A zero breakAt makes one line.
func joinCue(parts []string, breakAt int) string {
	var b strings.Builder
	for i, p := range parts {
		if i > 0 {
			if i == breakAt {
				b.WriteString(`\N`)
			} else {
				b.WriteByte(' ')
			}
		}
		b.WriteString(p)
	}
	return b.String()
}
