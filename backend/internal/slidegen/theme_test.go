package slidegen

import (
	"crypto/sha256"
	"fmt"
	"image"
	"image/color"
	"sort"
	"strings"
	"testing"

	"github.com/rahul/indifferent/backend/internal/models"
)

func themedQuestion() QuestionSlideData {
	return QuestionSlideData{
		QuestionNumber: 2,
		TotalQuestions: 5,
		QuestionText:   "Which AWS service runs containers without managing servers?",
		Options: []models.Option{
			{Label: "A", Text: "Amazon EC2"},
			{Label: "B", Text: "AWS Fargate"},
			{Label: "C", Text: "Amazon Lightsail"},
			{Label: "D", Text: "AWS Batch"},
		},
		CorrectIndex:   1,
		CorrectIndices: []int{1},
	}
}

func renderImage(t *testing.T, name, file string, data interface{}) *image.RGBA {
	t.Helper()
	b, err := NewNativeRenderer().RenderTemplate(name+"/"+file, data)
	if err != nil {
		t.Fatalf("%s/%s: %v", name, file, err)
	}
	img := decodePNG(t, b)
	rgba := image.NewRGBA(img.Bounds())
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			rgba.Set(x, y, img.At(x, y))
		}
	}
	return rgba
}

func pixel(img *image.RGBA, x, y int) color.RGBA { return img.RGBAAt(x, y) }

func countPixels(img *image.RGBA, want color.RGBA) int {
	n := 0
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			if img.RGBAAt(x, y) == want {
				n++
			}
		}
	}
	return n
}

func fingerprint(img *image.RGBA) string {
	h := sha256.Sum256(img.Pix)
	return fmt.Sprintf("%x", h[:8])
}

// --- registry ---------------------------------------------------------------

func TestRegistryHasTheSixTemplateNames(t *testing.T) {
	want := []string{"classic", "dark", "education", "minimal", "modern", "neon"}
	got := ThemeNames()
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("themes = %v, want %v", got, want)
	}
	for _, name := range want {
		if ThemeFor(name).Name != name {
			t.Errorf("ThemeFor(%q).Name = %q", name, ThemeFor(name).Name)
		}
	}
}

func TestThemeForFallsBackToClassic(t *testing.T) {
	for _, name := range []string{"", "nonsense", "MODERN-v2", "../classic"} {
		if got := ThemeFor(name).Name; got != DefaultTheme {
			t.Errorf("ThemeFor(%q) = %q, want fallback %q", name, got, DefaultTheme)
		}
	}
	// Case and whitespace are forgiven for a real name.
	if got := ThemeFor("  Dark ").Name; got != "dark" {
		t.Errorf("ThemeFor(\"  Dark \") = %q, want dark", got)
	}
}

func TestUnknownTemplateRendersAsClassic(t *testing.T) {
	q := themedQuestion()
	a := renderImage(t, "classic", "question.html", q)
	b := renderImage(t, "no-such-template", "question.html", q)
	if fingerprint(a) != fingerprint(b) {
		t.Error("an unknown template name must render exactly like Classic")
	}
	// A bare file name (no theme part) is Classic too.
	raw, err := NewNativeRenderer().RenderTemplate("question.html", q)
	if err != nil {
		t.Fatal(err)
	}
	if ok, why := samePixels(decodePNG(t, raw), a); !ok {
		t.Errorf("bare file name must render as Classic: %s", why)
	}
}

func TestSplitTemplateName(t *testing.T) {
	tests := []struct{ in, theme, file string }{
		{"modern/answer-reveal.html", "modern", "answer-reveal.html"},
		{"classic/question.html", "classic", "question.html"},
		{"question.html", "", "question.html"},
		{"a/b/c.html", "a", "b/c.html"},
		{"", "", ""},
	}
	for _, tt := range tests {
		th, f := splitTemplateName(tt.in)
		if th != tt.theme || f != tt.file {
			t.Errorf("splitTemplateName(%q) = (%q, %q), want (%q, %q)", tt.in, th, f, tt.theme, tt.file)
		}
	}
}

func TestMustHexRejectsBadInput(t *testing.T) {
	for _, bad := range []string{"", "#FFF", "#GGGGGG", "12345", "#1234567"} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("mustHex(%q) should panic", bad)
				}
			}()
			mustHex(bad)
		}()
	}
	if got := mustHex("#7C3AED"); got != (color.RGBA{R: 124, G: 58, B: 237, A: 255}) {
		t.Errorf("mustHex(#7C3AED) = %v", got)
	}
}

// --- contrast ----------------------------------------------------------------

// Every design must clear the issue's 4.5:1 text rule on every pair the
// renderer draws, so no design (including Classic) can ship unreadable.
func TestThemesMeetContrast(t *testing.T) {
	for _, name := range ThemeNames() {
		th := ThemeFor(name)
		for _, p := range th.contrastPairs() {
			if got := contrastRatio(p.fg, p.bg); got < p.min {
				t.Errorf("%s / %s: contrast %.2f:1, need %.1f:1", name, p.label, got, p.min)
			}
		}
	}
}

func TestContrastRatioKnownValues(t *testing.T) {
	black, white := color.RGBA{A: 255}, color.RGBA{R: 255, G: 255, B: 255, A: 255}
	if got := contrastRatio(black, white); got < 20.9 || got > 21.1 {
		t.Errorf("black on white = %.2f, want 21", got)
	}
	if got := contrastRatio(white, white); got != 1 {
		t.Errorf("white on white = %.2f, want 1", got)
	}
	if contrastRatio(black, white) != contrastRatio(white, black) {
		t.Error("contrast must be symmetric")
	}
}

// --- the template choice changes the slides ---------------------------------

func TestEachThemeDrawsItsOwnColours(t *testing.T) {
	for _, name := range ThemeNames() {
		th := ThemeFor(name)
		t.Run(name, func(t *testing.T) {
			q := renderImage(t, name, "question.html", themedQuestion())
			if got := pixel(q, 1900, 1050); got != th.QuestionBG {
				t.Errorf("question background = %v, want %v", got, th.QuestionBG)
			}
			if got := pixel(q, 960, 3); got != th.Accent {
				t.Errorf("question accent bar = %v, want %v", got, th.Accent)
			}

			a := renderImage(t, name, "answer-reveal.html", themedQuestion())
			if got := pixel(a, 1900, 1050); got != th.AnswerBG {
				t.Errorf("answer background = %v, want %v", got, th.AnswerBG)
			}
			if got := pixel(a, 960, 3); got != th.Accent {
				t.Errorf("answer accent bar = %v, want %v", got, th.Accent)
			}
			// The correct option is drawn in the theme's correct colour: some
			// fully covered glyph pixels must carry it exactly.
			if n := countPixels(a, th.Correct); n < 50 {
				t.Errorf("only %d pixels in the correct-answer colour %v", n, th.Correct)
			}

			c := renderImage(t, name, "countdown.html", CountdownSlideData{Seconds: 3})
			if got := pixel(c, 20, 20); got != th.CountdownBG {
				t.Errorf("countdown background = %v, want %v", got, th.CountdownBG)
			}
			if n := countPixels(c, th.Accent); n < 500 {
				t.Errorf("countdown number: only %d accent pixels", n)
			}

			o := renderImage(t, name, "outro.html", OutroSlideData{ChannelName: "Quiz Channel"})
			if got := pixel(o, 20, 20); got != th.OutroBG {
				t.Errorf("outro background = %v, want %v", got, th.OutroBG)
			}
			if n := countPixels(o, th.Accent); n < 500 {
				t.Errorf("outro channel name: only %d accent pixels", n)
			}
		})
	}
}

// No template may be selectable unless it renders differently, so every pair of
// themes must produce different pixels for every slide kind.
func TestNoTwoThemesRenderTheSame(t *testing.T) {
	slides := []struct {
		file string
		data interface{}
	}{
		{"question.html", themedQuestion()},
		{"answer-reveal.html", themedQuestion()},
		{"countdown.html", CountdownSlideData{Seconds: 3}},
		{"outro.html", OutroSlideData{ChannelName: "Quiz Channel"}},
	}
	for _, s := range slides {
		seen := map[string]string{}
		var names []string
		for _, name := range ThemeNames() {
			fp := fingerprint(renderImage(t, name, s.file, s.data))
			if other, dup := seen[fp]; dup {
				t.Errorf("%s: themes %q and %q render identically", s.file, other, name)
			}
			seen[fp] = name
			names = append(names, name)
		}
		sort.Strings(names)
	}
}

// Every design must stay readable at video resolution: the issue sets a floor
// for question and option type, and the base layout assumes room for a header.
func TestEveryDesignHasReadableTypeScale(t *testing.T) {
	for _, name := range ThemeNames() {
		l := ThemeFor(name).layout()
		if l.QuestionSize < 64 {
			t.Errorf("%s: question size %.0f, want at least 64", name, l.QuestionSize)
		}
		if l.OptionSize < 44 {
			t.Errorf("%s: option size %.0f, want at least 44", name, l.OptionSize)
		}
		if l.HeaderSize < 24 {
			t.Errorf("%s: header size %.0f, want at least 24", name, l.HeaderSize)
		}
		if l.QuestionMaxPx < 1200 {
			t.Errorf("%s: question width %.0f, want at least 1200", name, l.QuestionMaxPx)
		}
	}
}

// Only the minimal design drops cards; every other design must say so
// explicitly, otherwise an empty Layout would silently render the base card
// layout with zero-valued sizes.
func TestDesignsDeclareALayout(t *testing.T) {
	for _, name := range ThemeNames() {
		l := ThemeFor(name).Layout
		if !l.Cards && name != "minimal" {
			t.Errorf("%s: cards disabled, only minimal may do that", name)
		}
		if l.Cards && l.CardRadius < 8 {
			t.Errorf("%s: card radius %d, want at least 8", name, l.CardRadius)
		}
	}
}

// Card designs must actually draw the card fills the palette promises; a
// regression to text-on-background would still pass the colour-only tests.
func TestCardDesignsDrawTheirCards(t *testing.T) {
	for _, name := range ThemeNames() {
		th := ThemeFor(name)
		if !th.layout().Cards {
			continue
		}
		t.Run(name, func(t *testing.T) {
			q := renderImage(t, name, "question.html", themedQuestion())
			if n := countPixels(q, th.cardBG()); n < 10000 {
				t.Errorf("question slide has only %d pixels in the card colour %v", n, th.cardBG())
			}
			a := renderImage(t, name, "answer-reveal.html", themedQuestion())
			if n := countPixels(a, th.correctCardBG()); n < 10000 {
				t.Errorf("answer slide has only %d pixels in the correct-card colour %v", n, th.correctCardBG())
			}
		})
	}
}

func TestEveryThemeRendersEverySlideKind(t *testing.T) {
	for _, name := range ThemeNames() {
		for _, s := range []struct {
			file string
			data interface{}
		}{
			{"question.html", themedQuestion()},
			{"answer-reveal.html", themedQuestion()},
			{"countdown.html", CountdownSlideData{Seconds: 5}},
			{"outro.html", OutroSlideData{ChannelName: "Quiz Channel"}},
		} {
			img := renderImage(t, name, s.file, s.data)
			if img.Bounds().Dx() != 1920 || img.Bounds().Dy() != 1080 {
				t.Errorf("%s/%s: %v, want 1920x1080", name, s.file, img.Bounds())
			}
		}
	}
}
