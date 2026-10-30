package storyplan

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// tagPattern matches one complete markup tag. Stripping runs to a fixed point
// so nested input such as "<<b>b>" cannot leave a tag behind.
var tagPattern = regexp.MustCompile(`<[^<>]*>`)

// invisible reports characters that change how text looks or reads without
// being visible: bidirectional overrides and isolates (which can make text
// display in a different order than it is spoken), the byte order mark and the
// zero width space. Zero width joiners (U+200D) and non-joiners (U+200C) are
// kept: Devanagari and the other Indian scripts need them to shape conjuncts.
func invisible(r rune) bool {
	switch {
	case r == 0x200B, r == 0xFEFF:
		return true
	case r >= 0x202A && r <= 0x202E:
		return true
	case r >= 0x2066 && r <= 0x2069:
		return true
	}
	return false
}

// cleanText makes model-written text safe to store and show as a single line of
// plain text: markup is removed (so it can neither become SSML in a later
// text-to-speech call nor HTML), control and invisible characters are removed,
// runs of whitespace become one space, and the result is cut to maxRunes.
func cleanText(s string, maxRunes int) string {
	if !utf8.ValidString(s) {
		s = strings.ToValidUTF8(s, "")
	}
	for {
		stripped := tagPattern.ReplaceAllString(s, " ")
		if stripped == s {
			break
		}
		s = stripped
	}
	s = strings.Map(func(r rune) rune {
		switch {
		case r == '<' || r == '>':
			return -1
		case invisible(r):
			return -1
		case unicode.IsControl(r) || r == '\u2028' || r == '\u2029':
			return ' '
		}
		return r
	}, s)
	s = strings.Join(strings.Fields(s), " ")
	if runes := []rune(s); len(runes) > maxRunes {
		s = strings.TrimSpace(string(runes[:maxRunes]))
	}
	return s
}

// NormalizeStory validates a user's story and returns it ready for the prompt.
//
// It converts line endings, removes control and invisible characters (keeping
// newlines, which separate paragraphs), collapses runs of blank lines, trims
// the ends, and enforces the length and content limits. The error is an
// ErrInvalidStory with a reason that is safe to show to the user.
func NormalizeStory(story string) (string, error) {
	if !utf8.ValidString(story) {
		story = strings.ToValidUTF8(story, "")
	}
	story = strings.ReplaceAll(story, "\r\n", "\n")
	story = strings.ReplaceAll(story, "\r", "\n")
	story = strings.Map(func(r rune) rune {
		switch {
		case r == '\n':
			return r
		case r == '\t' || r == '\u2028' || r == '\u2029':
			return ' '
		case unicode.IsControl(r) || invisible(r):
			return -1
		}
		return r
	}, story)

	lines := strings.Split(story, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRightFunc(strings.Join(strings.Fields(l), " "), unicode.IsSpace)
	}
	story = strings.Join(lines, "\n")
	for strings.Contains(story, "\n\n\n") {
		story = strings.ReplaceAll(story, "\n\n\n", "\n\n")
	}
	story = strings.TrimSpace(story)

	n := utf8.RuneCountInString(story)
	switch {
	case story == "":
		return "", invalidStory("the story is empty")
	case n < MinStoryRunes:
		return "", invalidStory("the story is too short: write at least %d characters", MinStoryRunes)
	case n > MaxStoryRunes:
		return "", invalidStory("the story is too long: %d characters, the limit is %d", n, MaxStoryRunes)
	}
	if _, letters := scriptCounts(story); letters < MinStoryRunes/3 {
		return "", invalidStory("the story needs real text to plan from")
	}
	return story, nil
}

// neutralizeDelimiters replaces angle brackets with look-alike characters so a
// story cannot contain the closing marker that fences it in the prompt. The
// text stays readable to the model and to a human.
func neutralizeDelimiters(s string) string {
	return strings.NewReplacer("<", "\u2039", ">", "\u203A").Replace(s)
}

// firstSentence returns the text up to and including the first sentence
// terminator (period, question or exclamation mark, ellipsis, or the Devanagari
// danda), cut to at most maxWords words.
func firstSentence(s string, maxWords int) string {
	s = strings.TrimSpace(s)
	end := len(s)
	runes := []rune(s)
	pos := 0
	for i, r := range runes {
		pos += utf8.RuneLen(r)
		if strings.ContainsRune(".!?\u2026\u0964\u0965", r) {
			// A terminator ends the sentence at the end of the text or before
			// whitespace; "3.5" or "e.g." mid-word does not.
			if i+1 >= len(runes) || unicode.IsSpace(runes[i+1]) {
				end = pos
				break
			}
		}
	}
	words := strings.Fields(s[:end])
	if len(words) > maxWords {
		words = words[:maxWords]
	}
	return strings.Join(words, " ")
}
