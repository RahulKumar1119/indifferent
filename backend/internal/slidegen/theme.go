package slidegen

import (
	"fmt"
	"image/color"
	"math"
	"sort"
	"strconv"
	"strings"
)

// DesignLayout is the geometry behind a template name: whether options sit in
// cards, how round those cards are, and the type scale. The palette and the
// layout are chosen together by the design id (issue #104).
type DesignLayout struct {
	Cards         bool
	CardRadius    int
	QuestionSize  float64
	OptionSize    float64
	HeaderSize    float64
	LabelSize     float64
	QuestionMaxPx float64
}

// Theme is the palette and layout behind a template name: every colour and
// measurement the native renderer draws with. A project's template name
// (classic, modern, ...) is resolved to a Theme once per slide, so question,
// answer, countdown and outro slides of one project always match.
type Theme struct {
	Name   string
	Layout DesignLayout

	// Slide backgrounds. The answer slide is tinted differently from the
	// question slide so the reveal reads as a change, not a repeat.
	QuestionBG  color.RGBA
	AnswerBG    color.RGBA
	CountdownBG color.RGBA
	OutroBG     color.RGBA

	Text        color.RGBA // question text, outro "thanks" line
	Option      color.RGBA // answer options on the question slide
	OptionWrong color.RGBA // options that are not correct, on the answer slide
	Subtle      color.RGBA // countdown label, outro subtitle
	Accent      color.RGBA // top bar, "Question X of Y" header, countdown number, channel name
	Correct     color.RGBA // correct option(s) on the answer slide
}

// DefaultTheme is used for an unknown or empty template name.
const DefaultTheme = "classic"

// mustHex parses "#RRGGBB". It panics on a malformed literal, which the
// package tests catch long before runtime.
func mustHex(s string) color.RGBA {
	v, err := strconv.ParseUint(strings.TrimPrefix(s, "#"), 16, 32)
	if err != nil || len(strings.TrimPrefix(s, "#")) != 6 {
		panic(fmt.Sprintf("slidegen: bad colour %q", s))
	}
	return color.RGBA{R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v), A: 255}
}

// themes is the registry. Keys are exactly the template names the API accepts
// (validTemplates in internal/api); a test keeps the two in step.
var themes = map[string]Theme{
	// classic keeps its colours; issue #104 gives every design the new base
	// layout. Golden images in testdata/golden are regenerated alongside it.
	// Its answer slide still predates the 4.5:1 contrast rule; see
	// TestClassicKnownContrastShortfalls.
	"classic": {
		Name:        "classic",
		Layout:      DesignLayout{Cards: true, CardRadius: 20, QuestionSize: 64, OptionSize: 44, HeaderSize: 30, LabelSize: 30, QuestionMaxPx: 1560},
		QuestionBG:  mustHex("#FFFFFF"),
		AnswerBG:    mustHex("#FFFFFF"),
		CountdownBG: mustHex("#F5F5FA"),
		OutroBG:     mustHex("#FAF5FF"),
		Text:        mustHex("#1E1E1E"),
		Option:      mustHex("#505050"),
		OptionWrong: mustHex("#8C8C8C"),
		Subtle:      mustHex("#646464"),
		Accent:      mustHex("#7C3AED"),
		Correct:     mustHex("#16A34A"),
	},
	// modern: cool slate with a deep sky-blue accent.
	"modern": {
		Name:        "modern",
		Layout:      DesignLayout{Cards: true, CardRadius: 28, QuestionSize: 72, OptionSize: 48, HeaderSize: 30, LabelSize: 32, QuestionMaxPx: 1560},
		QuestionBG:  mustHex("#F8FAFC"),
		AnswerBG:    mustHex("#EAF3FF"),
		CountdownBG: mustHex("#E2E8F0"),
		OutroBG:     mustHex("#F1F5F9"),
		Text:        mustHex("#0F172A"),
		Option:      mustHex("#334155"),
		OptionWrong: mustHex("#475569"),
		Subtle:      mustHex("#334155"),
		Accent:      mustHex("#0369A1"),
		Correct:     mustHex("#047857"),
	},
	// education: warm cream paper, navy ink, a red pencil accent.
	"education": {
		Name:        "education",
		Layout:      DesignLayout{Cards: true, CardRadius: 16, QuestionSize: 68, OptionSize: 46, HeaderSize: 30, LabelSize: 30, QuestionMaxPx: 1560},
		QuestionBG:  mustHex("#FFF9E8"),
		AnswerBG:    mustHex("#FFF1CC"),
		CountdownBG: mustHex("#FDEFC8"),
		OutroBG:     mustHex("#FFF3D1"),
		Text:        mustHex("#1B2A4A"),
		Option:      mustHex("#3B4A68"),
		OptionWrong: mustHex("#44506B"),
		Subtle:      mustHex("#3B4A68"),
		Accent:      mustHex("#B3261E"),
		Correct:     mustHex("#166534"),
	},
	// dark: near-black with a soft violet accent.
	"dark": {
		Name:        "dark",
		Layout:      DesignLayout{Cards: true, CardRadius: 24, QuestionSize: 68, OptionSize: 46, HeaderSize: 30, LabelSize: 30, QuestionMaxPx: 1560},
		QuestionBG:  mustHex("#0F1115"),
		AnswerBG:    mustHex("#151A24"),
		CountdownBG: mustHex("#1A1F2B"),
		OutroBG:     mustHex("#12151C"),
		Text:        mustHex("#F3F4F6"),
		Option:      mustHex("#C7CCD6"),
		OptionWrong: mustHex("#AEB5C2"),
		Subtle:      mustHex("#C7CCD6"),
		Accent:      mustHex("#A78BFA"),
		Correct:     mustHex("#4ADE80"),
	},
	// minimal: off-white and black, no colour except the answer green.
	"minimal": {
		Name:        "minimal",
		Layout:      DesignLayout{Cards: false, CardRadius: 0, QuestionSize: 84, OptionSize: 52, HeaderSize: 28, LabelSize: 34, QuestionMaxPx: 1500},
		QuestionBG:  mustHex("#FAFAFA"),
		AnswerBG:    mustHex("#F0F0F0"),
		CountdownBG: mustHex("#F0F0F0"),
		OutroBG:     mustHex("#FAFAFA"),
		Text:        mustHex("#111111"),
		Option:      mustHex("#3F3F3F"),
		OptionWrong: mustHex("#525252"),
		Subtle:      mustHex("#3F3F3F"),
		Accent:      mustHex("#111111"),
		Correct:     mustHex("#0B6B3A"),
	},
	// neon: deep indigo-black with magenta and green highlights.
	"neon": {
		Name:        "neon",
		Layout:      DesignLayout{Cards: true, CardRadius: 24, QuestionSize: 68, OptionSize: 46, HeaderSize: 30, LabelSize: 30, QuestionMaxPx: 1560},
		QuestionBG:  mustHex("#0A0A14"),
		AnswerBG:    mustHex("#130A26"),
		CountdownBG: mustHex("#0A0A14"),
		OutroBG:     mustHex("#0A0A14"),
		Text:        mustHex("#F5F7FF"),
		Option:      mustHex("#BFC6FF"),
		OptionWrong: mustHex("#A9B2F0"),
		Subtle:      mustHex("#BFC6FF"),
		Accent:      mustHex("#FF3DCB"),
		Correct:     mustHex("#39FF88"),
	},
}

// layout returns the design measurements with safe defaults, so a theme
// written before a field existed still renders sensibly.
func (t Theme) layout() DesignLayout {
	l := t.Layout
	if l.CardRadius == 0 && l.Cards {
		l.CardRadius = 20
	}
	if l.QuestionSize == 0 {
		l.QuestionSize = 64
	}
	if l.OptionSize == 0 {
		l.OptionSize = 44
	}
	if l.HeaderSize == 0 {
		l.HeaderSize = 30
	}
	if l.LabelSize == 0 {
		l.LabelSize = 30
	}
	if l.QuestionMaxPx == 0 {
		l.QuestionMaxPx = 1560
	}
	return l
}

// cardBG is the fill behind an option card. It is the slide background with a
// hint of the accent, so each design feels like one system without needing a
// second full palette.
func (t Theme) cardBG() color.RGBA {
	if !t.layout().Cards {
		return t.QuestionBG
	}
	return blend(t.QuestionBG, t.Accent, 0.08)
}

// answerCardBG and correctCardBG are the answer-slide equivalents.
func (t Theme) answerCardBG() color.RGBA {
	if !t.layout().Cards {
		return t.AnswerBG
	}
	return blend(t.AnswerBG, t.Text, 0.05)
}

func (t Theme) correctCardBG() color.RGBA {
	if !t.layout().Cards {
		return t.AnswerBG
	}
	return blend(t.AnswerBG, t.Correct, 0.05)
}

// cardBorder is decoration only: it separates a card from the slide without
// carrying text, so it is not part of the contrast contract.
func (t Theme) cardBorder() color.RGBA {
	if !t.layout().Cards {
		return t.QuestionBG
	}
	return blend(t.QuestionBG, t.Text, 0.16)
}

// blend mixes fg into bg. amount is clamped to [0, 1].
func blend(bg, fg color.RGBA, amount float64) color.RGBA {
	amount = math.Max(0, math.Min(1, amount))
	mix := func(a, b uint8) uint8 {
		return uint8(math.Round(float64(a)*(1-amount) + float64(b)*amount))
	}
	return color.RGBA{R: mix(bg.R, fg.R), G: mix(bg.G, fg.G), B: mix(bg.B, fg.B), A: 255}
}

// ThemeFor returns the theme for a template name. An unknown or empty name
// falls back to Classic, so a record written with a name from some other era
// still renders.
func ThemeFor(name string) Theme {
	if t, ok := themes[strings.ToLower(strings.TrimSpace(name))]; ok {
		return t
	}
	return themes[DefaultTheme]
}

// ThemeNames lists every template name that has a real theme, sorted.
func ThemeNames() []string {
	names := make([]string, 0, len(themes))
	for n := range themes {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// splitTemplateName separates "modern/answer-reveal.html" into its theme name
// ("modern") and slide file ("answer-reveal.html"). A bare file name has no
// theme part.
func splitTemplateName(templateName string) (theme, file string) {
	if i := strings.Index(templateName, "/"); i >= 0 {
		return templateName[:i], templateName[i+1:]
	}
	return "", templateName
}

// contrastPair is one piece of drawn text (or a graphic) on its background.
type contrastPair struct {
	label string
	fg    color.RGBA
	bg    color.RGBA
	min   float64 // WCAG ratio the pair must reach
}

const (
	textContrast    = 4.5 // text, per issue #102
	graphicContrast = 3.0 // the accent bar is decoration, not text
)

// contrastPairs lists every foreground/background combination the renderer
// draws, so the contrast test cannot miss one. It mirrors the draw functions:
// change one, change the other.
func (t Theme) contrastPairs() []contrastPair {
	pairs := []contrastPair{
		{"question: header", t.Accent, t.QuestionBG, textContrast},
		{"question: question text", t.Text, t.QuestionBG, textContrast},
		{"question: options", t.Option, t.QuestionBG, textContrast},
		{"question: top bar", t.Accent, t.QuestionBG, graphicContrast},
		{"answer: header", t.Accent, t.AnswerBG, textContrast},
		{"answer: question text", t.Text, t.AnswerBG, textContrast},
		{"answer: top bar", t.Accent, t.AnswerBG, graphicContrast},
		{"countdown: number", t.Accent, t.CountdownBG, textContrast},
		{"countdown: label", t.Subtle, t.CountdownBG, textContrast},
		{"outro: channel name", t.Accent, t.OutroBG, textContrast},
		{"outro: thanks line", t.Text, t.OutroBG, textContrast},
		{"outro: subtitle", t.Subtle, t.OutroBG, textContrast},
	}
	if t.layout().Cards {
		pairs = append(pairs,
			contrastPair{"question: options on cards", t.Option, t.cardBG(), textContrast},
			contrastPair{"answer: wrong options on cards", t.OptionWrong, t.answerCardBG(), textContrast},
			contrastPair{"answer: correct option on card", t.Correct, t.correctCardBG(), textContrast},
		)
	} else {
		pairs = append(pairs,
			contrastPair{"answer: wrong options", t.OptionWrong, t.AnswerBG, textContrast},
			contrastPair{"answer: correct option", t.Correct, t.AnswerBG, textContrast},
		)
	}
	return pairs
}

// contrastRatio is the WCAG 2.x contrast ratio of two opaque colours.
func contrastRatio(a, b color.RGBA) float64 {
	la, lb := relativeLuminance(a), relativeLuminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

func relativeLuminance(c color.RGBA) float64 {
	ch := func(v uint8) float64 {
		f := float64(v) / 255
		if f <= 0.03928 {
			return f / 12.92
		}
		return math.Pow((f+0.055)/1.055, 2.4)
	}
	return 0.2126*ch(c.R) + 0.7152*ch(c.G) + 0.0722*ch(c.B)
}
