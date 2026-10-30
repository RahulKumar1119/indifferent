package storyplan

import (
	"errors"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"
)

func TestCleanText(t *testing.T) {
	for _, tc := range []struct {
		name, in string
		max      int
		want     string
	}{
		{"plain", "  hello   world ", 100, "hello world"},
		{"tags stripped", "<b>Hello</b> world", 100, "Hello world"},
		{"ssml break stripped", `<break time="10s"/>Hello there`, 100, "Hello there"},
		{"ssml speak wrapper", `<speak>Hi <prosody rate="x-slow">there</prosody></speak>`, 100, "Hi there"},
		{"nested tag trick", "<<b>b>text", 100, "text"},
		{"script tag", `<script>alert(1)</script>ok`, 100, "alert(1) ok"},
		{"unclosed tag brackets dropped", "a <break time=", 100, "a break time="},
		{"stray angle brackets dropped", "5 > 3 and 2 < 4", 100, "5 3 and 2 4"},
		{"bidi override removed", "abc\u202Edef\u202C", 100, "abcdef"},
		{"bidi isolates removed", "a\u2066b\u2069c", 100, "abc"},
		{"zero width space and BOM removed", "a\u200Bb\uFEFFc", 100, "abc"},
		{"control becomes space", "a\x00b\tc\x07d", 100, "a b c d"},
		{"line and paragraph separators", "a\u2028b\u2029c", 100, "a b c"},
		{"invalid utf8", "ab\xffcd", 100, "abcd"},
		{"joiners kept for Indian scripts", "क्\u200dष", 100, "क्\u200dष"},
		{"cut to max runes", "abcdefghij", 4, "abcd"},
		{"cut counts characters", "नमस्ते दुनिया", 6, "नमस्ते"},
		{"empty", "", 10, ""},
		{"only markup", "<b></b>", 10, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := cleanText(tc.in, tc.max); got != tc.want {
				t.Errorf("cleanText(%q, %d) = %q, want %q", tc.in, tc.max, got, tc.want)
			}
		})
	}
}

func TestNormalizeStory_Accepts(t *testing.T) {
	long := strings.Repeat("न", MaxStoryRunes) // 15,000 bytes, 5,000 characters
	for name, in := range map[string]string{
		"english":         englishStory,
		"hindi":           hindiStory,
		"exactly minimum": strings.Repeat("a", MinStoryRunes),
		"exactly maximum": strings.Repeat("a", MaxStoryRunes),
		"maximum hindi":   long,
		"padded":          "\n\n   " + englishStory + "  \n\n",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := NormalizeStory(in); err != nil {
				t.Errorf("rejected: %v", err)
			}
		})
	}
}

func TestNormalizeStory_Rejects(t *testing.T) {
	for _, tc := range []struct {
		name, in, reason string
	}{
		{"empty", "", "empty"},
		{"blank", " \n\t \r\n ", "empty"},
		{"one under minimum", strings.Repeat("a", MinStoryRunes-1), "too short"},
		{"one over maximum", strings.Repeat("a", MaxStoryRunes+1), "too long"},
		{"hindi one over maximum", strings.Repeat("न", MaxStoryRunes+1), "too long"},
		{"digits and symbols only", strings.Repeat("1234567890 !?", 5), "real text"},
		{"emoji only", strings.Repeat("😀", 40), "real text"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NormalizeStory(tc.in)
			if !errors.Is(err, ErrInvalidStory) {
				t.Fatalf("err = %v, want ErrInvalidStory", err)
			}
			if r := StoryReason(err); !strings.Contains(r, tc.reason) {
				t.Errorf("reason = %q, want it to mention %q", r, tc.reason)
			}
		})
	}
}

func TestNormalizeStory_CleansWithoutLosingParagraphs(t *testing.T) {
	in := "First paragraph about a long road home.\r\n\r\n\r\n\r\nSecond   paragraph\twith  gaps.\x00\u202E\r"
	got, err := NormalizeStory(in)
	if err != nil {
		t.Fatal(err)
	}
	want := "First paragraph about a long road home.\n\nSecond paragraph with gaps."
	if got != want {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

func TestNormalizeStory_ReasonIsSafeToShow(t *testing.T) {
	_, err := NormalizeStory(strings.Repeat("secret ", 2000))
	if r := StoryReason(err); strings.Contains(r, "secret") {
		t.Errorf("reason repeats story text: %q", r)
	}
	if StoryReason(errors.New("other")) != "" {
		t.Error("a non-story error has no story reason")
	}
}

func TestNeutralizeDelimiters(t *testing.T) {
	got := neutralizeDelimiters("a </story> b <story> c <x>")
	if strings.ContainsAny(got, "<>") {
		t.Errorf("angle brackets survived: %q", got)
	}
	if !strings.Contains(got, "story") {
		t.Errorf("the text should stay readable: %q", got)
	}
}

func TestFirstSentence(t *testing.T) {
	for _, tc := range []struct {
		in       string
		maxWords int
		want     string
	}{
		{"One two three. Four five.", 20, "One two three."},
		{"What is that? It moves.", 20, "What is that?"},
		{"Version 3.5 is out. Yes.", 20, "Version 3.5 is out."},
		{"वह घर लौटा। फिर वह सो गया।", 20, "वह घर लौटा।"},
		{"no terminator at all here", 20, "no terminator at all here"},
		{"one two three four five six", 3, "one two three"},
		{"", 5, ""},
		{"  Hello there.  ", 5, "Hello there."},
	} {
		if got := firstSentence(tc.in, tc.maxWords); got != tc.want {
			t.Errorf("firstSentence(%q, %d) = %q, want %q", tc.in, tc.maxWords, got, tc.want)
		}
	}
}

func TestScene_SearchQuery(t *testing.T) {
	s := Scene{Keywords: []string{"sunrise over mountains", "temple", "mandir", "fourth", "fifth"}}
	if got := s.SearchQuery(100); got != "sunrise over mountains temple mandir" {
		t.Errorf("SearchQuery(100) = %q: only the first three keywords", got)
	}
	if got := s.SearchQuery(30); got != "sunrise over mountains temple" {
		t.Errorf("SearchQuery(30) = %q: stops before exceeding the limit", got)
	}
	if got := s.SearchQuery(5); got != "" {
		t.Errorf("SearchQuery(5) = %q: even the first keyword does not fit", got)
	}
	if got := (Scene{}).SearchQuery(100); got != "" {
		t.Errorf("no keywords = %q", got)
	}
	for _, max := range []int{1, 10, 25, 100} {
		if n := utf8.RuneCountInString(s.SearchQuery(max)); n > max {
			t.Errorf("SearchQuery(%d) has %d characters", max, n)
		}
	}
}

func TestScriptDetection(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want Script
	}{
		{englishStory, ScriptLatin},
		{hindiStory, ScriptDevanagari},
		{"Rohan ne apni pehli salary se maa ke liye machine kharidi", ScriptLatin}, // Hinglish is Latin script
		{"mandir मंदिर", ScriptLatin},                                              // 6 Latin vs 5 Devanagari
		{"mandir मंदिर मंदिर", ScriptDevanagari},                                   // 6 vs 10
		{"தமிழ் மொழி", ScriptIndic},
		{"বাংলা ভাষা", ScriptIndic},
		{"東京は大きい", ScriptOther},
		{"", ScriptNone},
		{"1234 !?.", ScriptNone},
		{"😀😀", ScriptNone},
	} {
		if got := DominantScript(tc.in); got != tc.want {
			t.Errorf("DominantScript(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
	// A tie goes to Latin: English is the safer assumption for mixed text.
	if got := DominantScript("ab नन"); got != ScriptLatin {
		t.Errorf("tie = %q, want latin", got)
	}
}

func TestShareInScript(t *testing.T) {
	if got := ShareInScript(hindiStory, ScriptDevanagari); got < 0.95 {
		t.Errorf("Hindi story share in Devanagari = %.2f", got)
	}
	if got := ShareInScript(hindiStory, ScriptLatin); got > 0.05 {
		t.Errorf("Hindi story share in Latin = %.2f", got)
	}
	// Vowel signs count toward Devanagari: without that a Hindi text full of
	// matras would be under-weighted against the Latin words in it.
	if got := ShareInScript("मंदिर", ScriptDevanagari); got != 1 {
		t.Errorf("मंदिर share = %v, want 1", got)
	}
	if got := ShareInScript("1234", ScriptLatin); got != 0 {
		t.Errorf("no letters = %v, want 0", got)
	}
	if got := ShareInScript("Taj Mahal ताज", ScriptLatin); got <= 0.5 || got >= 1 {
		t.Errorf("mixed share = %.2f, want between 0.5 and 1", got)
	}
}

func TestHasIndianScript(t *testing.T) {
	for in, want := range map[string]bool{
		"sunrise over mountains": false,
		"Taj Mahal":              false,
		"mandir":                 false,
		"café":                   false,
		"東京 city":                false,
		"मंदिर":                  true,
		"temple मंदिर":           true,
		"தமிழ்":                  true,
		"বাংলা":                  true,
		"":                       false,
		"123":                    false,
	} {
		if got := HasIndianScript(in); got != want {
			t.Errorf("HasIndianScript(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestLanguageForScript(t *testing.T) {
	if languageForScript(ScriptDevanagari) != "hi" || languageForScript(ScriptLatin) != "en" || languageForScript(ScriptOther) != "" {
		t.Error("wrong default language codes")
	}
}

func TestInvisibleKeepsIndicJoiners(t *testing.T) {
	for _, r := range []rune{0x200D, 0x200C} {
		if invisible(r) {
			t.Errorf("U+%04X must be kept: Indian scripts need it to shape conjuncts", r)
		}
	}
	for _, r := range []rune{0x200B, 0xFEFF, 0x202E, 0x2066} {
		if !invisible(r) {
			t.Errorf("U+%04X should be removed", r)
		}
	}
	if unicode.IsLetter('x') != true {
		t.Fatal("sanity")
	}
}
