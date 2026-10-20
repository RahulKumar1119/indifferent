package thumbnail

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func renderPlan(t *testing.T, src image.Image, p Plan, brand string) *Rendered {
	t.Helper()
	r, err := Render(src, p, brand)
	if err != nil {
		t.Fatalf("Render(%s): %v", p.Describe(), err)
	}
	return r
}

func basePlan(layout Layout, style Style, format Format) Plan {
	concept := map[Layout]Concept{LayoutSide: ConceptFace, LayoutBand: ConceptSubject, LayoutCard: ConceptCreative}[layout]
	return Plan{
		Concept: concept, Layout: layout, Style: style, Format: format,
		Zoom: 1, Focus: [2]float64{0.5, 0.45}, Anchor: [2]float64{0.3, 0.5},
		Headline: "5 AI TOOLS YOU NEED", Secondary: "IN 2026",
		Enhance: layout == LayoutCard,
	}
}

func TestRenderEveryStyleFormatAndLayout(t *testing.T) {
	src := scene(1920, 1080, 3)
	for _, style := range Styles() {
		for _, format := range []Format{FormatLandscape, FormatPortrait} {
			for _, layout := range []Layout{LayoutSide, LayoutBand, LayoutCard} {
				name := string(style) + "/" + string(format) + "/" + string(layout)
				t.Run(name, func(t *testing.T) {
					p := basePlan(layout, style, format)
					r := renderPlan(t, src, p, "")
					w, h := format.Size()
					if b := r.Image.Bounds(); b.Dx() != w || b.Dy() != h {
						t.Fatalf("size = %v, want %dx%d", b, w, h)
					}
					if len(r.TextBounds) != 2 {
						t.Fatalf("expected headline and secondary text, got %d boxes", len(r.TextBounds))
					}
					// Safe margins: at least 5% of the shorter side from the left,
					// right and top edges, and nothing in the bottom 8% where
					// players draw their controls.
					m := int(0.05 * float64(min(w, h)))
					safe := image.Rect(m, m, w-m, int(0.92*float64(h)))
					for i, b := range r.TextBounds {
						if !b.In(safe) {
							t.Errorf("text box %d %v is outside the safe area %v", i, b, safe)
						}
					}
					data, err := r.JPEG()
					if err != nil {
						t.Fatal(err)
					}
					if len(data) > 2<<20 {
						t.Errorf("jpeg is %d bytes; YouTube accepts at most 2 MB", len(data))
					}
					if _, err := jpeg.Decode(bytes.NewReader(data)); err != nil {
						t.Errorf("output is not a valid JPEG: %v", err)
					}
				})
			}
		}
	}
}

func TestRenderedTextDoesNotOverlap(t *testing.T) {
	src := scene(1920, 1080, 3)
	for _, layout := range []Layout{LayoutSide, LayoutBand, LayoutCard} {
		for _, format := range []Format{FormatLandscape, FormatPortrait} {
			r := renderPlan(t, src, basePlan(layout, StyleViral, format), "CHANNEL")
			for i := range r.TextBounds {
				for j := i + 1; j < len(r.TextBounds); j++ {
					if o := r.TextBounds[i].Intersect(r.TextBounds[j]); !o.Empty() {
						t.Errorf("%s/%s: text boxes %d and %d overlap: %v", layout, format, i, j, o)
					}
				}
			}
		}
	}
}

func TestThreeConceptsLookDifferent(t *testing.T) {
	src := scene(1920, 1080, 8)
	for _, format := range []Format{FormatLandscape, FormatPortrait} {
		var imgs []*image.RGBA
		for _, l := range []Layout{LayoutSide, LayoutBand, LayoutCard} {
			imgs = append(imgs, renderPlan(t, src, basePlan(l, StyleViral, format), "").Image)
		}
		for i := 0; i < 3; i++ {
			for j := i + 1; j < 3; j++ {
				// Even from the very same frame the compositions must differ
				// clearly, not by a few pixels.
				if d := meanAbsDiff(imgs[i], imgs[j]); d < 12 {
					t.Errorf("%s: concepts %d and %d differ by only %.1f levels", format, i, j, d)
				}
			}
		}
	}
}

func TestHeadlineIsDrawnInTheStyleColour(t *testing.T) {
	// Viral headlines are yellow. Find yellow pixels inside the headline box.
	src := solid(1920, 1080, color.RGBA{R: 30, G: 30, B: 60, A: 255})
	r := renderPlan(t, src, basePlan(LayoutBand, StyleViral, FormatLandscape), "")
	box := r.TextBounds[0]
	want := LookFor(StyleViral).Headline
	hits := 0
	for y := box.Min.Y; y < box.Max.Y; y++ {
		for x := box.Min.X; x < box.Max.X; x++ {
			c := r.Image.RGBAAt(x, y)
			if absInt(int(c.R)-int(want.R)) < 6 && absInt(int(c.G)-int(want.G)) < 6 && absInt(int(c.B)-int(want.B)) < 6 {
				hits++
			}
		}
	}
	if hits < 500 {
		t.Errorf("only %d headline-coloured pixels inside the headline box; the text was not drawn", hits)
	}
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func TestEveryLookKeepsTextReadable(t *testing.T) {
	for _, s := range Styles() {
		l := LookFor(s)
		// The headline sits on the scrim colour (and, where a style has one,
		// inside an outline of the stroke colour).
		if r := contrastRatio(l.Headline, l.Shade); r < 4.5 {
			t.Errorf("%s: headline over scrim has contrast %.2f, want 4.5", s, r)
		}
		if r := contrastRatio(l.AccentInk, l.Accent); r < 4.5 {
			t.Errorf("%s: text on the accent tag has contrast %.2f, want 4.5", s, r)
		}
		if l.StrokeEm > 0 {
			if r := contrastRatio(l.Headline, l.Stroke); r < 4.5 {
				t.Errorf("%s: headline against its outline has contrast %.2f, want 4.5", s, r)
			}
		}
	}
}

func TestLongTextStillFits(t *testing.T) {
	src := scene(1920, 1080, 2)
	longest := "ABCDEFGHIJ KLMNOPQRS TUVWXYZ12"
	for _, format := range []Format{FormatLandscape, FormatPortrait} {
		for _, layout := range []Layout{LayoutSide, LayoutBand, LayoutCard} {
			p := basePlan(layout, StyleEducational, format)
			p.Headline = longest
			p.Secondary = "ANOTHER LONG SECONDARY"
			r, err := Render(src, p, "A VERY LONG CHANNEL NAME HERE")
			if err != nil {
				t.Errorf("%s/%s: %v", layout, format, err)
				continue
			}
			w, h := format.Size()
			for _, b := range r.TextBounds {
				if !b.In(image.Rect(0, 0, w, h)) {
					t.Errorf("%s/%s: text %v spills off the canvas", layout, format, b)
				}
			}
		}
	}
}

func TestHugeUnbreakableWordFailsCleanly(t *testing.T) {
	p := basePlan(LayoutSide, StyleViral, FormatPortrait)
	p.Headline = strings.Repeat("W", 80)
	_, err := Render(scene(640, 360, 1), p, "")
	if err != nil && CodeOf(err) != CodeRenderFailed {
		t.Errorf("code = %q, want RENDER_FAILED", CodeOf(err))
	}
}

func TestSecondaryTextAndBrandAreOptional(t *testing.T) {
	src := scene(1920, 1080, 1)
	p := basePlan(LayoutSide, StyleViral, FormatLandscape)
	p.Secondary = ""
	plain := renderPlan(t, src, p, "")
	if len(plain.TextBounds) != 1 {
		t.Errorf("headline only: %d text boxes", len(plain.TextBounds))
	}
	branded := renderPlan(t, src, p, "My Channel")
	if len(branded.TextBounds) != 2 {
		t.Errorf("headline and brand: %d text boxes", len(branded.TextBounds))
	}
	if meanAbsDiff(plain.Image, branded.Image) == 0 {
		t.Error("the brand tag changed nothing")
	}
}

func TestCinematicHasLetterbox(t *testing.T) {
	r := renderPlan(t, scene(1920, 1080, 5), basePlan(LayoutBand, StyleCinematic, FormatLandscape), "")
	for _, y := range []int{2, 717} {
		if c := r.Image.RGBAAt(640, y); c.R != 0 || c.G != 0 || c.B != 0 {
			t.Errorf("row %d should be a black bar, got %v", y, c)
		}
	}
}

func TestCinematicTextStaysBetweenTheBars(t *testing.T) {
	src := scene(1920, 1080, 5)
	for _, format := range []Format{FormatLandscape, FormatPortrait} {
		w, h := format.Size()
		bar := h / 12
		for _, layout := range []Layout{LayoutSide, LayoutBand, LayoutCard} {
			r := renderPlan(t, src, basePlan(layout, StyleCinematic, format), "CHANNEL")
			for i, b := range r.TextBounds {
				if b.Min.Y < bar || b.Max.Y > h-bar {
					t.Errorf("%s/%s: text box %d %v overlaps a letterbox bar (bars cover 0-%d and %d-%d of %dx%d)", layout, format, i, b, bar, h-bar, h, w, h)
				}
			}
		}
	}
}

func TestStylesProduceDifferentPictures(t *testing.T) {
	src := scene(1920, 1080, 9)
	var imgs []*image.RGBA
	for _, s := range Styles() {
		imgs = append(imgs, renderPlan(t, src, basePlan(LayoutBand, s, FormatLandscape), "").Image)
	}
	for i := 0; i < len(imgs); i++ {
		for j := i + 1; j < len(imgs); j++ {
			if meanAbsDiff(imgs[i], imgs[j]) < 1.5 {
				t.Errorf("styles %s and %s are nearly identical", Styles()[i], Styles()[j])
			}
		}
	}
}

func TestFitTextPrefersLargestType(t *testing.T) {
	big, ok := fitText(FontBold, "HELLO", 1000, 400, 3, 200, 20)
	if !ok {
		t.Fatal("HELLO should fit")
	}
	small, ok := fitText(FontBold, "HELLO WONDERFUL WORLD OF TEXT", 1000, 400, 3, 200, 20)
	if !ok {
		t.Fatal("long text should fit")
	}
	if small.px >= big.px {
		t.Errorf("longer text should use smaller type: %d vs %d", small.px, big.px)
	}
	for _, tb := range []textBlock{big, small} {
		if tb.width > 1000 || len(tb.lines)*tb.lineH > 400 {
			t.Errorf("block %+v does not fit its box", tb)
		}
	}
	if _, ok := fitText(FontBold, "", 1000, 400, 3, 200, 20); ok {
		t.Error("empty text cannot be fitted")
	}
	if _, ok := fitText(FontBold, "SUPERCALIFRAGILISTICEXPIALIDOCIOUS", 40, 400, 3, 200, 20); ok {
		t.Error("a word wider than the box at the smallest size must not fit")
	}
}

func TestFitTextBalancesLines(t *testing.T) {
	tb, ok := fitText(FontBold, "ONE TWO THREE FOUR", 900, 400, 2, 120, 20)
	if !ok {
		t.Fatal("did not fit")
	}
	if len(tb.lines) == 2 {
		a, b := len(tb.lines[0]), len(tb.lines[1])
		if a-b > 6 || b-a > 6 {
			t.Errorf("lines are lopsided: %q", tb.lines)
		}
	}
}

func TestSplitsEnumeratesContiguousGroups(t *testing.T) {
	got := splits([]string{"a", "b", "c"}, 2)
	if len(got) != 2 || got[0][0] != "a" || got[0][1] != "b c" || got[1][0] != "a b" || got[1][1] != "c" {
		t.Errorf("splits = %q", got)
	}
	if one := splits([]string{"a", "b"}, 1); len(one) != 1 || one[0][0] != "a b" {
		t.Errorf("single line = %q", one)
	}
}

func TestGradeIdentityAndEffects(t *testing.T) {
	src := scene(64, 36, 4)
	same := image.NewRGBA(src.Bounds())
	copy(same.Pix, src.Pix)
	grade(same, 1, 1, 1)
	if meanAbsDiff(src, same) > 0.5 {
		t.Error("grade(1,1,1) must leave the image alone")
	}
	gray := image.NewRGBA(src.Bounds())
	copy(gray.Pix, src.Pix)
	grade(gray, 0, 1, 1)
	for i := 0; i < len(gray.Pix); i += 4 {
		if absInt(int(gray.Pix[i])-int(gray.Pix[i+1])) > 1 || absInt(int(gray.Pix[i+1])-int(gray.Pix[i+2])) > 1 {
			t.Fatal("zero saturation must give grey pixels")
		}
	}
	dark := image.NewRGBA(src.Bounds())
	copy(dark.Pix, src.Pix)
	grade(dark, 1, 1, 0.5)
	if Measure(dark).Brightness >= Measure(src).Brightness {
		t.Error("brightness 0.5 must darken")
	}
}

// Set THUMBNAIL_PREVIEW_DIR to write sample thumbnails for a visual check.
func TestWritePreviewsWhenAsked(t *testing.T) {
	dir := os.Getenv("THUMBNAIL_PREVIEW_DIR")
	if dir == "" {
		t.Skip("THUMBNAIL_PREVIEW_DIR not set")
	}
	_ = os.MkdirAll(dir, 0o755)
	src := scene(1920, 1080, 14)
	for _, style := range Styles() {
		for _, layout := range []Layout{LayoutSide, LayoutBand, LayoutCard} {
			r := renderPlan(t, src, basePlan(layout, style, FormatLandscape), "CHANNEL")
			data, _ := r.JPEG()
			_ = os.WriteFile(filepath.Join(dir, string(style)+"-"+string(layout)+".jpg"), data, 0o644)
		}
	}
}
