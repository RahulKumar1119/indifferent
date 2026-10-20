package thumbnail

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"math"
	"strings"
	"sync"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/gomedium"
	"golang.org/x/image/font/gofont/gomonobold"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// Rendered is a finished thumbnail plus the boxes its text occupies, which
// tests use to check the safe margins.
type Rendered struct {
	Image      *image.RGBA
	TextBounds []image.Rectangle
}

// JPEG encodes the thumbnail. Quality 90 keeps a 1280x720 image far below
// YouTube's 2 MB limit.
func (r *Rendered) JPEG() ([]byte, error) {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, r.Image, &jpeg.Options{Quality: 90}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ---------------------------------------------------------------------------
// Fonts

var (
	fontOnce  sync.Once
	fontFiles map[FontKind]*opentype.Font
	fontErr   error
	faceMu    sync.Mutex
	faceCache = map[faceKey]font.Face{}
)

type faceKey struct {
	kind FontKind
	px   int
}

func loadFonts() {
	fontFiles = map[FontKind]*opentype.Font{}
	for kind, ttf := range map[FontKind][]byte{
		FontBold:   gobold.TTF,
		FontMedium: gomedium.TTF,
		FontMono:   gomonobold.TTF,
	} {
		f, err := opentype.Parse(ttf)
		if err != nil {
			fontErr = err
			return
		}
		fontFiles[kind] = f
	}
}

func faceFor(kind FontKind, px int) (font.Face, error) {
	fontOnce.Do(loadFonts)
	if fontErr != nil {
		return nil, fontErr
	}
	faceMu.Lock()
	defer faceMu.Unlock()
	k := faceKey{kind, px}
	if f, ok := faceCache[k]; ok {
		return f, nil
	}
	f, ok := fontFiles[kind]
	if !ok {
		f = fontFiles[FontBold]
	}
	face, err := opentype.NewFace(f, &opentype.FaceOptions{Size: float64(px), DPI: 72, Hinting: font.HintingNone})
	if err != nil {
		return nil, err
	}
	faceCache[k] = face
	return face, nil
}

// ---------------------------------------------------------------------------
// Text fitting

type textBlock struct {
	lines []string
	px    int
	face  font.Face
	lineH int
	width int // widest line
}

func measure(face font.Face, s string) int {
	return font.MeasureString(face, s).Ceil()
}

// splits returns every way to cut words into k contiguous groups.
func splits(words []string, k int) [][]string {
	if k == 1 {
		return [][]string{{strings.Join(words, " ")}}
	}
	var out [][]string
	for i := 1; i <= len(words)-(k-1); i++ {
		head := strings.Join(words[:i], " ")
		for _, rest := range splits(words[i:], k-1) {
			out = append(out, append([]string{head}, rest...))
		}
	}
	return out
}

// fitText finds the largest type size at which text fits inside maxW by maxH
// in at most maxLines lines, choosing the most even split of words across
// lines. It never goes below minPx; if the text still does not fit there it
// reports false.
func fitText(kind FontKind, text string, maxW, maxH, maxLines, maxPx, minPx int) (textBlock, bool) {
	words := strings.Fields(text)
	if len(words) == 0 {
		return textBlock{}, false
	}
	if maxLines > len(words) {
		maxLines = len(words)
	}
	for px := maxPx; px >= minPx; px -= 2 {
		face, err := faceFor(kind, px)
		if err != nil {
			return textBlock{}, false
		}
		lineH := int(float64(px) * 1.04)
		for k := 1; k <= maxLines; k++ {
			if k*lineH > maxH {
				break
			}
			var best []string
			bestW := math.MaxInt
			for _, cand := range splits(words, k) {
				w := 0
				for _, l := range cand {
					if lw := measure(face, l); lw > w {
						w = lw
					}
				}
				if w < bestW {
					best, bestW = cand, w
				}
			}
			if best != nil && bestW <= maxW {
				return textBlock{lines: best, px: px, face: face, lineH: lineH, width: bestW}, true
			}
		}
	}
	return textBlock{}, false
}

// drawLine paints one line of text with a soft shadow, an outline and the fill.
func drawLine(dst *image.RGBA, face font.Face, s string, x, baseline int, fill, stroke color.RGBA, strokePx float64) {
	paint := func(dx, dy int, c color.Color) {
		d := font.Drawer{Dst: dst, Src: image.NewUniform(c), Face: face, Dot: fixed.P(x+dx, baseline+dy)}
		d.DrawString(s)
	}
	if strokePx > 0 {
		shadow := color.NRGBA{R: 0, G: 0, B: 0, A: 120}
		off := int(math.Round(strokePx * 0.9))
		paint(off, off+int(math.Round(strokePx*0.4)), shadow)
		steps := int(math.Max(10, math.Round(strokePx*5)))
		for i := 0; i < steps; i++ {
			a := 2 * math.Pi * float64(i) / float64(steps)
			paint(int(math.Round(math.Cos(a)*strokePx)), int(math.Round(math.Sin(a)*strokePx)), stroke)
		}
	}
	paint(0, 0, fill)
}

// align says how lines sit inside their box.
type align int

const (
	alignLeft align = iota
	alignRight
	alignCenter
)

// drawBlock paints a fitted text block inside box and returns the area it used.
func drawBlock(dst *image.RGBA, tb textBlock, box image.Rectangle, a align, anchorBottom bool, look Look) image.Rectangle {
	total := len(tb.lines) * tb.lineH
	y := box.Min.Y
	if anchorBottom {
		y = box.Max.Y - total
	} else if total < box.Dy() {
		y = box.Min.Y + (box.Dy()-total)/2
	}
	strokePx := look.StrokeEm * float64(tb.px)
	asc := tb.face.Metrics().Ascent.Ceil()
	used := image.Rectangle{}
	for i, l := range tb.lines {
		w := measure(tb.face, l)
		x := box.Min.X
		switch a {
		case alignRight:
			x = box.Max.X - w
		case alignCenter:
			x = box.Min.X + (box.Dx()-w)/2
		}
		top := y + i*tb.lineH
		drawLine(dst, tb.face, l, x, top+asc, look.Headline, look.Stroke, strokePx)
		r := image.Rect(x, top, x+w, top+tb.lineH)
		if used.Empty() {
			used = r
		} else {
			used = used.Union(r)
		}
	}
	return used
}

// drawTag paints text on a solid accent block (the secondary line or brand).
func drawTag(dst *image.RGBA, text string, kind FontKind, px, x, y int, bg, ink color.RGBA, a align, maxW int) image.Rectangle {
	for ; px > 10; px -= 2 {
		face, err := faceFor(kind, px)
		if err != nil {
			return image.Rectangle{}
		}
		padX, padY := px*6/10, px*3/10
		w := measure(face, text)
		if w+2*padX > maxW {
			continue
		}
		h := face.Metrics().Height.Ceil()
		left := x
		switch a {
		case alignRight:
			left = x - (w + 2*padX)
		case alignCenter:
			left = x - (w+2*padX)/2
		}
		rect := image.Rect(left, y, left+w+2*padX, y+h+2*padY)
		draw.Draw(dst, rect, image.NewUniform(bg), image.Point{}, draw.Src)
		d := font.Drawer{Dst: dst, Src: image.NewUniform(ink), Face: face, Dot: fixed.P(left+padX, y+padY+face.Metrics().Ascent.Ceil())}
		d.DrawString(text)
		return rect
	}
	return image.Rectangle{}
}

// ---------------------------------------------------------------------------
// Pixels

func clamp8(v float64) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v + 0.5)
}

// grade applies saturation, contrast and brightness to an opaque image.
func grade(img *image.RGBA, sat, contrast, bright float64) {
	p := img.Pix
	for i := 0; i+3 < len(p); i += 4 {
		r, g, b := float64(p[i]), float64(p[i+1]), float64(p[i+2])
		l := 0.299*r + 0.587*g + 0.114*b
		r, g, b = l+(r-l)*sat, l+(g-l)*sat, l+(b-l)*sat
		r, g, b = (r-128)*contrast+128, (g-128)*contrast+128, (b-128)*contrast+128
		p[i], p[i+1], p[i+2] = clamp8(r*bright), clamp8(g*bright), clamp8(b*bright)
	}
}

// scrim darkens part of the image towards shade with opacity given by alpha(x,y).
func scrim(img *image.RGBA, shade color.RGBA, alpha func(x, y int) float64) {
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			a := alpha(x, y)
			if a <= 0 {
				continue
			}
			if a > 1 {
				a = 1
			}
			i := img.PixOffset(x, y)
			img.Pix[i] = clamp8(float64(img.Pix[i])*(1-a) + float64(shade.R)*a)
			img.Pix[i+1] = clamp8(float64(img.Pix[i+1])*(1-a) + float64(shade.G)*a)
			img.Pix[i+2] = clamp8(float64(img.Pix[i+2])*(1-a) + float64(shade.B)*a)
		}
	}
}

func smooth(t float64) float64 {
	t = math.Max(0, math.Min(1, t))
	return t * t * (3 - 2*t)
}

// vignette darkens the corners.
func vignette(img *image.RGBA, strength float64) {
	if strength <= 0 {
		return
	}
	b := img.Bounds()
	cx, cy := float64(b.Dx())/2, float64(b.Dy())/2
	scrim(img, color.RGBA{A: 255}, func(x, y int) float64 {
		dx, dy := (float64(x)-cx)/cx, (float64(y)-cy)/cy
		return strength * smooth((math.Hypot(dx, dy)-0.55)/0.9)
	})
}

// scaleCrop draws src[crop] scaled to fill dst.
func scaleCrop(dst *image.RGBA, dstRect image.Rectangle, src image.Image, crop image.Rectangle) {
	xdraw.CatmullRom.Scale(dst, dstRect, src, crop, xdraw.Src, nil)
}

// coverBackdrop makes a soft, blurred copy of the whole frame that fills size.
func coverBackdrop(src image.Image, w, h int) *image.RGBA {
	sb := src.Bounds()
	crop := CropRect(sb.Dx(), sb.Dy(), float64(w)/float64(h), 1, [2]float64{0.5, 0.5}, [2]float64{0.5, 0.5}, 0)
	crop = crop.Add(sb.Min)
	tiny := image.NewRGBA(image.Rect(0, 0, max(1, w/14), max(1, h/14)))
	xdraw.ApproxBiLinear.Scale(tiny, tiny.Bounds(), src, crop, xdraw.Src, nil)
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	xdraw.CatmullRom.Scale(out, out.Bounds(), tiny, tiny.Bounds(), xdraw.Src, nil)
	return out
}

// ---------------------------------------------------------------------------
// Rendering

// ErrNoFont is returned when the embedded fonts cannot be loaded.
var ErrNoFont = errors.New("thumbnail fonts unavailable")

// geometry holds the rectangles one layout uses.
type geometry struct {
	w, h   int
	margin int
	// text is where the headline block goes.
	text   image.Rectangle
	textAl align
	// bottomUp anchors the headline to the bottom of its box.
	bottomUp bool
	maxLines int
	// tagAbove puts the secondary line above the headline instead of below.
	tagAbove bool
	// card is the picture card for LayoutCard.
	card image.Rectangle
}

func layoutGeometry(p Plan, textOnRight bool) geometry {
	w, h := p.Format.Size()
	m := int(math.Round(0.055 * float64(min(w, h))))
	g := geometry{w: w, h: h, margin: m, maxLines: 3}
	portrait := p.Format == FormatPortrait
	switch p.Layout {
	case LayoutBand:
		if portrait {
			g.text = image.Rect(m, int(0.50*float64(h)), w-m, int(0.78*float64(h)))
		} else {
			g.text = image.Rect(m, int(0.56*float64(h)), w-m, int(0.88*float64(h)))
		}
		g.bottomUp, g.maxLines, g.tagAbove, g.textAl = true, 2, true, alignLeft
	case LayoutCard:
		if portrait {
			g.text = image.Rect(m, int(0.06*float64(h)), w-m, int(0.30*float64(h)))
			g.card = image.Rect(m, int(0.33*float64(h)), w-m, int(0.74*float64(h)))
			g.textAl = alignLeft
		} else {
			colW := int(0.46 * float64(w))
			g.text = image.Rect(m, int(0.12*float64(h)), m+colW, int(0.84*float64(h)))
			g.card = image.Rect(int(0.53*float64(w)), int(0.12*float64(h)), w-m, int(0.84*float64(h)))
			g.textAl = alignLeft
		}
	default: // side
		if portrait {
			g.text = image.Rect(m, int(0.06*float64(h)), w-m, int(0.34*float64(h)))
			g.textAl = alignLeft
		} else {
			colW := int(0.47 * float64(w))
			if textOnRight {
				g.text = image.Rect(w-m-colW, int(0.12*float64(h)), w-m, int(0.84*float64(h)))
				g.textAl = alignLeft
			} else {
				g.text = image.Rect(m, int(0.12*float64(h)), m+colW, int(0.84*float64(h)))
				g.textAl = alignLeft
			}
		}
	}
	return g
}

// Render draws a plan onto its source frame. brand, when set, is drawn as a
// small tag in the top corner (the channel name from the project's branding).
func Render(src image.Image, p Plan, brand string) (*Rendered, error) {
	if _, err := faceFor(FontBold, 20); err != nil {
		return nil, errors.Join(ErrNoFont, err)
	}
	look := LookFor(p.Style)
	textOnRight := p.Anchor[0] <= 0.5 // subject on the left leaves the right for text
	g := layoutGeometry(p, textOnRight)
	if look.Letterbox {
		// The bars cover the top and bottom twelfth; keep all text clear of them.
		keep := g.h/12 + g.margin/2
		g.text = g.text.Intersect(image.Rect(0, keep, g.w, g.h-keep))
		if !g.card.Empty() {
			g.card = g.card.Intersect(image.Rect(0, keep, g.w, g.h-keep))
		}
	}
	canvas := image.NewRGBA(image.Rect(0, 0, g.w, g.h))
	sb := src.Bounds()

	switch p.Layout {
	case LayoutCard:
		bg := coverBackdrop(src, g.w, g.h)
		draw.Draw(canvas, canvas.Bounds(), bg, image.Point{}, draw.Src)
		grade(canvas, look.Sat*1.25, look.Contrast, 0.5)
		// Picture card with a shadow and an accent border.
		cardAspect := float64(g.card.Dx()) / float64(g.card.Dy())
		crop := CropRect(sb.Dx(), sb.Dy(), cardAspect, p.Zoom, p.Focus, [2]float64{0.5, 0.5}, g.card.Dy()).Add(sb.Min)
		shadow := g.card.Inset(-int(0.012 * float64(g.w)))
		draw.Draw(canvas, shadow.Add(image.Pt(0, int(0.012*float64(g.h)))), image.NewUniform(color.NRGBA{A: 110}), image.Point{}, draw.Over)
		border := g.card.Inset(-int(0.007 * float64(g.w)))
		draw.Draw(canvas, border, image.NewUniform(look.Accent), image.Point{}, draw.Src)
		picture := image.NewRGBA(image.Rect(0, 0, g.card.Dx(), g.card.Dy()))
		scaleCrop(picture, picture.Bounds(), src, crop)
		if p.Enhance {
			grade(picture, 1.18, 1.08, 1.02)
		}
		draw.Draw(canvas, g.card, picture, image.Point{}, draw.Src)
		vignette(canvas, look.Vignette)
	case LayoutBand:
		crop := CropRect(sb.Dx(), sb.Dy(), p.Format.Aspect(), p.Zoom, p.Focus, p.Anchor, g.h).Add(sb.Min)
		scaleCrop(canvas, canvas.Bounds(), src, crop)
		grade(canvas, look.Sat, look.Contrast, look.Bright)
		top := float64(g.text.Min.Y) - 0.12*float64(g.h)
		scrim(canvas, look.Shade, func(_, y int) float64 {
			return look.ShadeA * smooth((float64(y)-top)/(float64(g.h)-top))
		})
		vignette(canvas, look.Vignette)
	default: // side
		crop := CropRect(sb.Dx(), sb.Dy(), p.Format.Aspect(), p.Zoom, p.Focus, p.Anchor, g.h).Add(sb.Min)
		scaleCrop(canvas, canvas.Bounds(), src, crop)
		grade(canvas, look.Sat, look.Contrast, look.Bright)
		if p.Format == FormatPortrait {
			end := float64(g.text.Max.Y) + 0.1*float64(g.h)
			scrim(canvas, look.Shade, func(_, y int) float64 {
				return look.ShadeA * (1 - smooth(float64(y)/end))
			})
		} else {
			span := float64(g.w) * 0.62
			onRight := textOnRight
			scrim(canvas, look.Shade, func(x, _ int) float64 {
				var d float64
				if onRight {
					d = float64(x) - (float64(g.w) - span)
					return look.ShadeA * smooth(d/span)
				}
				d = span - float64(x)
				return look.ShadeA * smooth(d/span)
			})
		}
		vignette(canvas, look.Vignette)
	}

	if look.Letterbox {
		bar := g.h / 12
		draw.Draw(canvas, image.Rect(0, 0, g.w, bar), image.NewUniform(color.RGBA{A: 255}), image.Point{}, draw.Src)
		draw.Draw(canvas, image.Rect(0, g.h-bar, g.w, g.h), image.NewUniform(color.RGBA{A: 255}), image.Point{}, draw.Src)
	}

	out := &Rendered{Image: canvas}

	// Headline, then the secondary tag.
	box := g.text
	secondary := strings.TrimSpace(p.Secondary)
	tagPx := 0
	if secondary != "" {
		tagPx = max(18, int(0.052*float64(g.h)))
		if p.Format == FormatPortrait {
			tagPx = max(22, int(0.026*float64(g.h)))
		}
	}
	tagH := 0
	if tagPx > 0 {
		tagH = int(float64(tagPx)*1.6) + int(0.02*float64(g.h))
	}
	headBox := box
	if tagH > 0 {
		if g.tagAbove {
			headBox.Min.Y += tagH
		} else {
			headBox.Max.Y -= tagH
		}
	}
	maxPx := int(0.2 * float64(g.h))
	if p.Format == FormatPortrait {
		maxPx = int(0.092 * float64(g.h))
	}
	minPx := int(0.04 * float64(min(g.w, g.h)))
	tb, ok := fitText(look.Font, strings.ToUpper(p.Headline), headBox.Dx(), headBox.Dy(), g.maxLines, maxPx, minPx)
	if !ok {
		return nil, Errorf(CodeRenderFailed, "The headline did not fit on the thumbnail.", nil)
	}
	used := drawBlock(canvas, tb, headBox, g.textAl, g.bottomUp, look)
	out.TextBounds = append(out.TextBounds, used)

	if secondary != "" {
		ty := used.Max.Y + int(0.015*float64(g.h))
		if g.tagAbove {
			ty = used.Min.Y - tagH + int(0.01*float64(g.h))
		}
		tagKind := look.Font
		if r := drawTag(canvas, strings.ToUpper(secondary), tagKind, tagPx, used.Min.X, ty, look.Accent, look.AccentInk, alignLeft, box.Dx()); !r.Empty() {
			out.TextBounds = append(out.TextBounds, r)
		}
	}

	if b := strings.TrimSpace(brand); b != "" {
		bpx := max(16, int(0.032*float64(g.h)))
		if p.Format == FormatPortrait {
			bpx = max(20, int(0.017*float64(g.h)))
		}
		y := g.margin / 2
		if look.Letterbox {
			y += g.h / 12
		}
		if r := drawTag(canvas, strings.ToUpper(b), FontBold, bpx, g.margin, y, color.RGBA{A: 255}, color.RGBA{R: 255, G: 255, B: 255, A: 255}, alignLeft, g.w/2); !r.Empty() {
			out.TextBounds = append(out.TextBounds, r)
		}
	}
	return out, nil
}
