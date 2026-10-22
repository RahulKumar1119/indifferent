package slidegen

import (
	"testing"

	"github.com/rahul/indifferent/backend/internal/models"
)

func TestDefaultAccentMatchesDesign(t *testing.T) {
	for _, name := range ThemeNames() {
		accents := models.AllowedAccents(name)
		if len(accents) == 0 {
			t.Errorf("%s: no curated accents", name)
			continue
		}
		if ThemeFor(name).Accent != mustHex(accents[0]) {
			t.Errorf("%s: default accent %v, first curated %s", name, ThemeFor(name).Accent, accents[0])
		}
	}
}

// The acceptance rule is every design and every allowed option combination.
// Colours only vary with accent, but looping the whole matrix catches future
// options that start affecting text colour.
func TestEveryAllowedDesignOptionCombinationMeetsContrast(t *testing.T) {
	show := []*bool{boolPtr(true), boolPtr(false)}
	for _, name := range ThemeNames() {
		for _, accent := range models.AllowedAccents(name) {
			for _, size := range []string{"s", "m", "l"} {
				for _, showNumber := range show {
					for _, labels := range []string{"letters", "numbers"} {
						opts := models.DesignOptions{
							Accent:             accent,
							TextSize:           size,
							ShowQuestionNumber: showNumber,
							AnswerLabels:       labels,
						}
						th := themeForOptions(name, opts)
						for _, pair := range th.contrastPairs() {
							if got := contrastRatio(pair.fg, pair.bg); got < pair.min {
								t.Errorf("%s %+v / %s: %.2f:1, need %.1f:1", name, opts, pair.label, got, pair.min)
							}
						}
					}
				}
			}
		}
	}
}

func TestOptionScalesNeverDropBelowBaseType(t *testing.T) {
	for _, name := range ThemeNames() {
		base := ThemeFor(name).layout()
		for _, size := range []string{"s", "m", "l"} {
			scale := textScale(models.DesignOptions{TextSize: size})
			if q := base.QuestionSize * scale; q < 64 {
				t.Errorf("%s/%s: question %.1f, want at least 64", name, size, q)
			}
			if o := base.OptionSize * scale; o < 44 {
				t.Errorf("%s/%s: option %.1f, want at least 44", name, size, o)
			}
		}
	}
}

func TestDesignOptionsChangeTheSlide(t *testing.T) {
	data := themedQuestion()
	plain := renderImage(t, "classic", "question.html", data)

	secondAccent := models.AllowedAccents("classic")[1]
	data.DesignOptions = models.DesignOptions{
		Accent:             secondAccent,
		TextSize:           "l",
		ShowQuestionNumber: boolPtr(false),
		AnswerLabels:       "numbers",
	}
	custom := renderImage(t, "classic", "question.html", data)
	if fingerprint(plain) == fingerprint(custom) {
		t.Fatal("design options did not change the render")
	}
	if n := countPixels(custom, mustHex(secondAccent)); n < 500 {
		t.Errorf("custom accent: only %d pixels", n)
	}
}

func TestOptionLabelStyle(t *testing.T) {
	tests := []struct {
		index int
		label string
		style string
		want  string
	}{
		{0, "A", "", "A"},
		{3, "D", "letters", "D"},
		{0, "A", "numbers", "1"},
		{3, "D", "numbers", "4"},
		{2, "", "letters", "C"},
	}
	for _, tt := range tests {
		if got := optionLabel(tt.index, tt.label, tt.style); got != tt.want {
			t.Errorf("optionLabel(%d, %q, %q) = %q, want %q", tt.index, tt.label, tt.style, got, tt.want)
		}
	}
}

func TestInvalidAccentIsIgnoredNotRendered(t *testing.T) {
	data := themedQuestion()
	data.DesignOptions = models.DesignOptions{Accent: "#FF00FF"}
	withBad := renderImage(t, "classic", "question.html", data)
	plain := renderImage(t, "classic", "question.html", themedQuestion())
	if fingerprint(withBad) != fingerprint(plain) {
		t.Error("an unknown stored accent must fall back to the design default")
	}
}

func boolPtr(v bool) *bool { return &v }
