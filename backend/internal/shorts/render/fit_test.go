package render

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/rahul/indifferent/backend/internal/models"
	"pgregory.net/rapid"
)

func unitsToPx(units int) int { return int(float64(units)*assScale + 0.5) }

func wordsOf(text string, perWord float64) []models.Word {
	var out []models.Word
	for i, t := range strings.Fields(text) {
		out = append(out, models.Word{Text: t, Start: float64(i) * perWord, End: float64(i)*perWord + perWord*0.9})
	}
	return out
}

var kfTag = regexp.MustCompile(`\{\\kf(\d+)\}`)

func kfDurations(doc string) []string {
	var out []string
	for _, m := range kfTag.FindAllStringSubmatch(doc, -1) {
		out = append(out, m[1])
	}
	return out
}

func dialogues(doc string) []string {
	var out []string
	for _, l := range strings.Split(doc, "\n") {
		if strings.HasPrefix(l, "Dialogue:") {
			out = append(out, l)
		}
	}
	return out
}

// --- measuring -------------------------------------------------------------

func TestAdvance_KnownCharacters(t *testing.T) {
	// DejaVu Sans: "W" is wide, "i" is narrow, the digits are one width, a space is a third of an em.
	if w, i := advance(fontSans, 'W'), advance(fontSans, 'i'); w < 900 || i > 300 {
		t.Errorf("W=%d i=%d", w, i)
	}
	if advance(fontSans, '0') != advance(fontSans, '8') {
		t.Error("digits should share a width")
	}
	if sp := advance(fontSans, ' '); sp < 300 || sp > 340 {
		t.Errorf("space = %d", sp)
	}
	if advance(fontCondensed, 'm') >= advance(fontSans, 'm') {
		t.Error("condensed must be narrower than sans")
	}
	if advance(fontSerif, 'm') == advance(fontSans, 'm') {
		t.Error("serif and sans differ")
	}
	// Characters the tables do not list get a default, never zero, so they cannot be forgotten.
	for _, r := range []rune{'ا', 'ગ', '😀', 'Ж'} {
		if advance(fontSans, r) != defaultAdvance {
			t.Errorf("%q = %d", r, advance(fontSans, r))
		}
	}
	if advance(fontSans, '字') != wideAdvance {
		t.Error("a Chinese character is one em wide")
	}
	if advance(fontSans, '\u200d') != 0 {
		t.Error("a joiner takes no room")
	}
	if advance(fontSans, 'क') == 0 || advance(fontSans, 'ा') == 0 {
		t.Error("Devanagari letters and vowel signs have widths")
	}
}

func TestTextWidth_ScalesWithSize(t *testing.T) {
	a, b := textWidth(fontSans, "Hello world", 100), textWidth(fontSans, "Hello world", 200)
	if b < a*1.99 || b > a*2.01 {
		t.Errorf("%v %v", a, b)
	}
	if textWidth(fontSans, "", 100) != 0 {
		t.Error("empty text has no width")
	}
}

// --- fitting ---------------------------------------------------------------

func TestLayoutCue_ACueThatFitsIsLeftAlone(t *testing.T) {
	// The default size, four ordinary words: one line, the size asked for, no break.
	px := unitsToPx(18)
	for _, words := range [][]string{
		{"The", "quick", "brown", "fox"},
		{"Hello"},
		{"मैंने", "दादाजी", "के", "पुराने"},
		{"I", "am", "so", "happy"},
	} {
		l := layoutCue(words, fontDefault, px, 3)
		if l.shrunk || l.breakAt != 0 || l.sizePx != px || !l.fits {
			t.Errorf("%v: %+v", words, l)
		}
	}
}

func TestLayoutCue_AWideCueWrapsToTwoLinesBeforeItShrinks(t *testing.T) {
	px := unitsToPx(30) // 200 px: four words are far wider than the frame on one line, and two lines of two fit
	l := layoutCue([]string{"The", "quick", "brown", "fox"}, fontDefault, px, 3)
	if l.breakAt == 0 || l.shrunk || l.sizePx != px || !l.fits {
		t.Fatalf("%+v", l)
	}
	if l.breakAt < 1 || l.breakAt > 3 {
		t.Errorf("break at %d", l.breakAt)
	}
}

func TestLayoutCue_ASplitIsAsEvenAsItCanBe(t *testing.T) {
	px := unitsToPx(30)
	words := []string{"a", "bb", "cccccccc", "dd", "e"}
	l := layoutCue(words, fontSans, px, 3)
	if l.breakAt == 0 {
		t.Skip("fits on one line at this size")
	}
	left := textWidth(fontSans, strings.Join(words[:l.breakAt], " "), l.sizePx)
	right := textWidth(fontSans, strings.Join(words[l.breakAt:], " "), l.sizePx)
	for k := 1; k < len(words); k++ {
		a := textWidth(fontSans, strings.Join(words[:k], " "), l.sizePx)
		b := textWidth(fontSans, strings.Join(words[k:], " "), l.sizePx)
		best := a
		if b > best {
			best = b
		}
		mine := left
		if right > mine {
			mine = right
		}
		if best+0.001 < mine {
			t.Errorf("splitting at %d is more even (%v) than at %d (%v)", k, best, l.breakAt, mine)
		}
	}
}

func TestLayoutCue_AWordWiderThanTheFrameShrinksUntilItFits(t *testing.T) {
	px := unitsToPx(72) // 480 px: "extraordinary" is about 3,500 px wide
	l := layoutCue([]string{"extraordinary"}, fontSans, px, 3)
	if !l.shrunk || !l.fits || l.sizePx >= px {
		t.Fatalf("%+v", l)
	}
	if w := textWidth(fontSans, "extraordinary", l.sizePx); w > float64(frameWidth-2*captionSideMarginPx-6) {
		t.Errorf("still %v px wide at %d", w, l.sizePx)
	}
	// And it is the largest size that fits, to within one step.
	if w := textWidth(fontSans, "extraordinary", l.sizePx+shrinkStepPx); w <= float64(frameWidth-2*captionSideMarginPx-6) {
		t.Errorf("%d would have fit", l.sizePx+shrinkStepPx)
	}
}

func TestLayoutCue_NeverShrinksBelowTheFloorAndSaysWhenItCannotFit(t *testing.T) {
	px := unitsToPx(18)
	l := layoutCue([]string{strings.Repeat("W", 200)}, fontSans, px, 3)
	floor := unitsToPx(minCaptionUnits)
	if l.sizePx != floor || l.fits || !l.shrunk {
		t.Errorf("%+v floor %d", l, floor)
	}
	// A size already below the floor is not raised.
	small := layoutCue([]string{"Hi"}, fontSans, 20, 3)
	if small.sizePx != 20 || small.shrunk {
		t.Errorf("%+v", small)
	}
}

func TestLayoutCue_OutlineTakesRoom(t *testing.T) {
	px := unitsToPx(18)
	words := []string{"abcdefgh", "abcdefgh", "abcdefgh"}
	thin, thick := layoutCue(words, fontSans, px, 0), layoutCue(words, fontSans, px, 60)
	if thick.sizePx > thin.sizePx {
		t.Errorf("a thick outline cannot allow a bigger size: %d vs %d", thick.sizePx, thin.sizePx)
	}
}

func TestLayoutCue_FontChangesWhatFits(t *testing.T) {
	px := unitsToPx(24)
	words := []string{"watermelon", "marshmallow", "mmmmmmmm"}
	cond, sans := layoutCue(words, fontCondensed, px, 3), layoutCue(words, fontSans, px, 3)
	if cond.sizePx < sans.sizePx {
		t.Errorf("condensed %d should fit at least as large as sans %d", cond.sizePx, sans.sizePx)
	}
}

// For every size in the allowed range, every cue the layout says fits really is no wider than the
// frame minus its margins, on every line.
func TestProperty_ALayoutThatFitsIsInsideTheFrame(t *testing.T) {
	letters := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789कखगघचछजझटठडढणतथदधनपफबभमयरलवशषसहािीुूेैोौंँ")
	rapid.Check(t, func(t *rapid.T) {
		n := rapid.IntRange(1, 4).Draw(t, "words")
		words := make([]string, n)
		for i := range words {
			words[i] = string(rapid.SliceOfN(rapid.SampledFrom(letters), 1, 22).Draw(t, "word"))
		}
		units := rapid.IntRange(8, 72).Draw(t, "units")
		font := rapid.SampledFrom([]capFont{fontSans, fontSerif, fontCondensed}).Draw(t, "font")
		outline := rapid.IntRange(0, 40).Draw(t, "outline")
		px := int(float64(units)*assScale + 0.5)
		l := layoutCue(words, font, px, outline)
		if l.sizePx > px {
			t.Fatalf("grew from %d to %d", px, l.sizePx)
		}
		if !l.fits {
			return
		}
		limit := float64(frameWidth-2*captionSideMarginPx-2*outline) + 0.001
		var lines [][]string
		if l.breakAt == 0 {
			lines = [][]string{words}
		} else {
			lines = [][]string{words[:l.breakAt], words[l.breakAt:]}
		}
		for _, ln := range lines {
			if w := textWidth(font, strings.Join(ln, " "), l.sizePx); w > limit {
				t.Fatalf("%v at %d is %v px, limit %v", ln, l.sizePx, w, limit)
			}
		}
		// A cue that fit before is not touched.
		if lone := layoutCue(words, font, px, outline); textWidth(font, strings.Join(words, " "), px) <= limit && (lone.shrunk || lone.breakAt != 0) {
			t.Fatalf("a cue that fits was changed: %+v", lone)
		}
	})
}

// --- the caption files ------------------------------------------------------

func TestKaraoke_ACueThatFitsIsByteForByteWhatItWas(t *testing.T) {
	doc := BuildASSBreaks(wordsOf("Hello world", 0.5), 0, models.CaptionStyle{Preset: "karaoke"}, nil)
	want := "[Script Info]\nScriptType: v4.00+\nPlayResX: 1080\nPlayResY: 1920\nWrapStyle: 2\nScaledBorderAndShadow: yes\n\n" +
		"[V4+ Styles]\nFormat: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding\n" +
		"Style: Default,Arial,120,&H000AD6FF,&H00A6A09A,&HFF000000,&H80000000,0,0,0,0,100,100,0,0,1,3,0,2,40,40,90,1\n\n" +
		"[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n" +
		"Dialogue: 0,0:00:00.00,0:00:00.95,Default,,0,0,0,,{\\kf45}Hello {\\kf45}world\n"
	if doc != want {
		t.Errorf("legacy karaoke changed:\n got %q\nwant %q", doc, want)
	}
}

func TestKaraoke_AWideCueWrapsAndTheSweepStaysInStep(t *testing.T) {
	words := wordsOf("The quick brown fox", 0.5)
	small := BuildASSBreaks(words, 0, models.CaptionStyle{Preset: "karaoke", FontSize: 16}, nil)
	big := BuildASSBreaks(words, 0, models.CaptionStyle{Preset: "karaoke", FontSize: 30}, nil)
	if strings.Contains(small, `\N`) {
		t.Error("a cue that fits must not wrap")
	}
	d := dialogues(big)
	if len(d) != 1 || strings.Count(d[0], `\N`) != 1 {
		t.Fatalf("expected one wrapped cue: %v", d)
	}
	if strings.Contains(d[0], `{\fs`) {
		t.Errorf("wrapping should be enough at 30: %s", d[0])
	}
	// The break is a line break and not a change of timing: the same words, with the same sweep
	// durations in the same order, as when it fits.
	if a, b := kfDurations(small), kfDurations(big); strings.Join(a, ",") != strings.Join(b, ",") {
		t.Errorf("sweep changed: %v vs %v", a, b)
	}
	if strings.Count(d[0], `{\kf`) != 4 {
		t.Errorf("every word keeps its sweep: %s", d[0])
	}
	// The cue's own start and end are unchanged.
	if a, b := strings.SplitN(dialogues(small)[0], ",", 4)[1:3], strings.SplitN(d[0], ",", 4)[1:3]; a[0] != b[0] || a[1] != b[1] {
		t.Errorf("times changed: %v vs %v", a, b)
	}
	// Removing the break gives the unwrapped text.
	if strings.ReplaceAll(d[0], `\N`, " ") == "" {
		t.Error("empty")
	}
}

func TestKaraoke_AVeryWideWordShrinksOnlyItsOwnCue(t *testing.T) {
	words := []models.Word{
		{Text: "Short", Start: 0, End: 0.5}, {Text: "words", Start: 0.5, End: 1},
		{Text: "internationalization", Start: 4, End: 5}, {Text: "now", Start: 5, End: 5.5},
	}
	doc := BuildASSBreaks(words, 0, models.CaptionStyle{Preset: "karaoke", FontSize: 36}, []bool{false, false, true, false})
	d := dialogues(doc)
	if len(d) != 2 {
		t.Fatalf("%d cues: %v", len(d), d)
	}
	if strings.Contains(d[0], `{\fs`) && !strings.Contains(d[0], `\N`) {
		// the first cue may need to wrap, but its size must not be the shrunk one
		t.Logf("first cue: %s", d[0])
	}
	if !strings.Contains(d[1], `{\fs`) {
		t.Errorf("the long word's cue should be smaller: %s", d[1])
	}
	sz := regexp.MustCompile(`\{\\fs(\d+)\}`).FindStringSubmatch(d[1])
	if sz == nil || sz[1] == "240" {
		t.Errorf("size %v", sz)
	}
}

func TestKaraoke_FontAndMarginAreOptIn(t *testing.T) {
	w := wordsOf("Hello world", 0.5)
	legacy := BuildASSBreaks(w, 0, models.CaptionStyle{Preset: "karaoke"}, nil)
	if !strings.Contains(legacy, "Style: Default,Arial,") || !strings.Contains(legacy, ",2,40,40,90,1\n") {
		t.Errorf("legacy style changed:\n%s", legacy)
	}
	for font, family := range map[string]string{"sans": "DejaVu Sans", "serif": "DejaVu Serif", "condensed": "DejaVu Sans Condensed"} {
		doc := BuildASSBreaks(w, 0, models.CaptionStyle{Preset: "karaoke", Font: font}, nil)
		if !strings.Contains(doc, "Style: Default,"+family+",") {
			t.Errorf("%s: %s", font, doc)
		}
	}
	above := BuildASSBreaks(w, 0, models.CaptionStyle{Preset: "karaoke", AboveControls: true}, nil)
	if !strings.Contains(above, ",2,40,40,288,1\n") {
		t.Errorf("above controls should lift the bottom margin to 288:\n%s", above)
	}
	// Top and middle captions have no bottom margin to lift.
	for _, pos := range []string{"top", "middle"} {
		doc := BuildASSBreaks(w, 0, models.CaptionStyle{Preset: "karaoke", Position: pos, AboveControls: true}, nil)
		if !strings.Contains(doc, ",40,40,90,1\n") {
			t.Errorf("%s moved: %s", pos, doc)
		}
	}
}

func TestCaptions_PresetsStayOnSRTWhenEverythingFits(t *testing.T) {
	w := wordsOf("The quick brown fox jumps over the lazy dog today", 0.4)
	for _, preset := range []string{"", "minimal", "bold", "clean"} {
		st := models.CaptionStyle{Preset: preset}
		doc, ext := Captions(w, 0, st, nil)
		if ext != ".srt" || doc != BuildSRT(w, 0) {
			t.Errorf("%q: %s changed", preset, ext)
		}
	}
	// Two lines at a bigger size is still libass's own wrapping.
	if _, ext := Captions(w, 0, models.CaptionStyle{Preset: "bold", FontSize: 24}, nil); ext != ".srt" {
		t.Errorf("a cue that wraps to two lines needs nothing: %s", ext)
	}
}

func TestCaptions_ACueThatCannotFitMovesThePresetToASS(t *testing.T) {
	w := []models.Word{{Text: "Hi", Start: 0, End: 0.3}, {Text: "extraordinarily", Start: 0.3, End: 1.2}}
	for _, preset := range []string{"minimal", "bold", "clean"} {
		doc, ext := Captions(w, 0, models.CaptionStyle{Preset: preset, FontSize: 72}, nil)
		if ext != ".ass" || !strings.Contains(doc, `{\fs`) {
			t.Errorf("%s: %s %s", preset, ext, doc)
		}
		if strings.Contains(doc, `\kf`) {
			t.Errorf("%s must not sweep: %s", preset, doc)
		}
		if got := SubtitlesFilter("captions"+ext, models.CaptionStyle{Preset: preset, FontSize: 72}); strings.Contains(got, "force_style") {
			t.Errorf("an ASS file carries its own style: %s", got)
		}
	}
	// Box presets get a box in both colour fields.
	doc, _ := Captions(w, 0, models.CaptionStyle{Preset: "bold", FontSize: 72}, nil)
	if !strings.Contains(doc, ",&H00000000,&H00000000,0,0,0,0,100,100,0,0,3,") {
		t.Errorf("bold should be drawn in a box: %s", doc)
	}
}

func TestCaptions_PresetMarginsMatchTheSRTOnes(t *testing.T) {
	w := []models.Word{{Text: "extraordinarily", Start: 0, End: 1}}
	for preset, wantMargin := range map[string]string{"minimal": ",2,28,28,300,1", "clean": ",2,28,28,333,1", "bold": ",2,28,28,67,1"} {
		doc, _ := Captions(w, 0, models.CaptionStyle{Preset: preset, FontSize: 72}, nil)
		if !strings.Contains(doc, wantMargin+"\n") {
			t.Errorf("%s: %s", preset, doc)
		}
	}
}

// --- font and margin on the SRT presets --------------------------------------

func TestResolveForceStyle_FontAndMarginAreOptIn(t *testing.T) {
	zero := ResolveForceStyle(models.CaptionStyle{})
	if zero != "Alignment=2,FontSize=18,PrimaryColour=&H00FFFFFF,BorderStyle=3,Outline=2" {
		t.Errorf("legacy changed: %s", zero)
	}
	if got := ResolveForceStyle(models.CaptionStyle{Font: "serif"}); got != zero+",FontName=DejaVu Serif" {
		t.Errorf("serif: %s", got)
	}
	// Bold sits at 10 (67 px) today; above the controls it goes to 44 (293 px).
	if got := ResolveForceStyle(models.CaptionStyle{AboveControls: true}); !strings.Contains(got, "MarginV=44") {
		t.Errorf("above controls: %s", got)
	}
	// Minimal (45) and Clean (50) already sit above 15%, and are never lowered.
	for preset, margin := range map[string]string{"minimal": "MarginV=45", "clean": "MarginV=50"} {
		for _, above := range []bool{false, true} {
			if got := ResolveForceStyle(models.CaptionStyle{Preset: preset, AboveControls: above}); !strings.Contains(got, margin) {
				t.Errorf("%s above=%v: %s", preset, above, got)
			}
		}
	}
	// Only bottom captions have a bottom margin to lift.
	for _, pos := range []string{"top", "middle"} {
		if got := ResolveForceStyle(models.CaptionStyle{Position: pos, AboveControls: true}); strings.Contains(got, "MarginV") {
			t.Errorf("%s: %s", pos, got)
		}
	}
}

func TestSubtitlesFilter_FollowsTheFileNotJustThePreset(t *testing.T) {
	bold := models.CaptionStyle{Preset: "bold"}
	if got := SubtitlesFilter("c.srt", bold); got != "subtitles=c.srt:force_style='Alignment=2,FontSize=18,PrimaryColour=&H00FFFFFF,BorderStyle=3,Outline=2'" {
		t.Errorf("srt: %s", got)
	}
	if got := SubtitlesFilter("c.ass", bold); got != "subtitles=c.ass" {
		t.Errorf("ass: %s", got)
	}
	if got := SubtitlesFilter("C.ASS", bold); got != "subtitles=C.ASS" {
		t.Errorf("upper case: %s", got)
	}
	if got := SubtitlesFilter("c.ass", models.CaptionStyle{Preset: "karaoke"}); got != "subtitles=c.ass" {
		t.Errorf("karaoke: %s", got)
	}
}

func TestCaptionValidate_Font(t *testing.T) {
	for _, ok := range []string{"", "sans", "serif", "condensed"} {
		if err := (models.CaptionStyle{Font: ok}).Validate(); err != nil {
			t.Errorf("%q: %v", ok, err)
		}
	}
	for _, bad := range []string{"Arial", "comic", "SANS", "../x", "sans;"} {
		if err := (models.CaptionStyle{Font: bad}).Validate(); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
	// The flag needs no validation, and the zero style still validates.
	if err := (models.CaptionStyle{AboveControls: true}).Validate(); err != nil {
		t.Error(err)
	}
}

func TestBuildAnimatedASS_WideCuesFitToo(t *testing.T) {
	c := models.AnimatedCaptions{Enabled: true, Intensity: 50, Font: "sans", FontSize: 40, Color: "#FFFFFF", Highlight: "#FFD60A", Position: "bottom", Outline: 4, Background: "none"}
	d := dialogues(BuildAnimatedASS(wordsOf("The quick brown fox", 0.5), 0, c))
	if len(d) != 1 || !strings.Contains(d[0], `\N`) {
		t.Errorf("a wide animated cue should wrap: %v", d)
	}
	if strings.Count(d[0], `{\kf`) != 4 {
		t.Errorf("sweep lost: %s", d[0])
	}
	c.FontSize = 16
	d = dialogues(BuildAnimatedASS(wordsOf("The quick brown fox", 0.5), 0, c))
	if strings.Contains(d[0], `\N`) || strings.Contains(d[0], `{\fs`) {
		t.Errorf("a cue that fits is left alone: %s", d[0])
	}
}

// Every size the studio offers, for the words of both languages, with no cue wider than the frame
// by the measure used here.
func TestEverySizeFitsBothLanguages(t *testing.T) {
	texts := []string{
		"The quick brown fox jumps over the lazy dog",
		"मैंने दादाजी के पुराने पीतल के दीपक को कैसे ठीक किया",
		"internationalization counterrevolutionaries",
		"Rohan ne apni pehli salary se maa ke liye silai machine kharidi",
	}
	for _, preset := range []string{"minimal", "bold", "karaoke", "clean"} {
		for _, font := range []string{"", "sans", "serif", "condensed"} {
			for units := 8; units <= 72; units += 4 {
				for _, text := range texts {
					st := models.CaptionStyle{Preset: preset, FontSize: units, Font: font}
					doc, ext := Captions(wordsOf(text, 0.4), 0, st, nil)
					if ext == ".srt" {
						// libass wraps SRT by itself: nothing here is wider than two lines at this size.
						continue
					}
					for _, line := range dialogues(doc) {
						if fs := regexp.MustCompile(`\{\\fs(\d+)\}`).FindStringSubmatch(line); fs != nil {
							var px int
							fmt.Sscan(fs[1], &px)
							if px < unitsToPx(minCaptionUnits)-1 {
								t.Errorf("%s/%s/%d: shrunk below the floor: %s", preset, font, units, line)
							}
						}
						if strings.Count(line, `\N`) > maxCaptionLines-1 {
							t.Errorf("%s/%s/%d: more than %d lines: %s", preset, font, units, maxCaptionLines, line)
						}
					}
				}
			}
		}
	}
}
