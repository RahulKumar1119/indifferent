package slidegen

import (
	"fmt"
	"image/color"
	"math"
	"sort"
	"strconv"
	"strings"
)

// Theme is the palette behind a template name: every colour the native
// renderer draws with. A project's template name (classic, modern, ...) is
// resolved to a Theme once per slide, so question, answer, countdown and outro
// slides of one project always match.
//
// Themes are palette-only on purpose (issue #102, phase 1). Layout, type sizes
// and the built-in fonts are shared, which keeps slide generation as fast as
// before and the Lambda package the same size.
type Theme struct {
	Name string

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
	// classic is frozen: it must render pixel for pixel as it did before
	// themes existed (golden images in testdata/golden). Its answer slide
	// predates the 4.5:1 contrast rule; see TestClassicKnownContrastShortfalls.
	"classic": {
		Name:        "classic",
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
	return []contrastPair{
		{"question: header", t.Accent, t.QuestionBG, textContrast},
		{"question: question text", t.Text, t.QuestionBG, textContrast},
		{"question: options", t.Option, t.QuestionBG, textContrast},
		{"question: top bar", t.Accent, t.QuestionBG, graphicContrast},
		{"answer: header", t.Accent, t.AnswerBG, textContrast},
		{"answer: question text", t.Text, t.AnswerBG, textContrast},
		{"answer: wrong options", t.OptionWrong, t.AnswerBG, textContrast},
		{"answer: correct option", t.Correct, t.AnswerBG, textContrast},
		{"answer: top bar", t.Accent, t.AnswerBG, graphicContrast},
		{"countdown: number", t.Accent, t.CountdownBG, textContrast},
		{"countdown: label", t.Subtle, t.CountdownBG, textContrast},
		{"outro: channel name", t.Accent, t.OutroBG, textContrast},
		{"outro: thanks line", t.Text, t.OutroBG, textContrast},
		{"outro: subtitle", t.Subtle, t.OutroBG, textContrast},
	}
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
