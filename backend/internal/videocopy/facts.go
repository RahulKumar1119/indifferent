package videocopy

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// material is what the video says, prepared for checking what a model wrote
// about it. The model may only repeat facts that are here.
//
// The checks are a floor, not a proof. They catch the claims a reader would
// call invented without reading the source: a number, a year, a size or a
// percentage that the video never says, a promise word it never uses, and a
// capitalised name it never mentions. They cannot judge whether a sentence is
// a fair summary; that rests on the prompt, and on the live checks reported
// with each change to it.
type material struct {
	text   string          // lower case, one space between words
	digits map[string]bool // every run of digits, in ASCII
	tokens map[string]bool // every word, lower case
	words  []string        // tokens as a list, for stem checks
}

var (
	digitRun     = regexp.MustCompile(`[0-9]+`)
	thousandsSep = regexp.MustCompile(`([0-9]),([0-9]{3})`)
	spaces       = regexp.MustCompile(`\s+`)
)

// numberWords are the spelled-out numbers a speaker may say where the model
// writes digits. A spoken "five" supports a written "5".
var numberWords = map[string]string{
	"zero": "0", "one": "1", "two": "2", "three": "3", "four": "4", "five": "5", "six": "6", "seven": "7", "eight": "8",
	"nine": "9", "ten": "10", "eleven": "11", "twelve": "12", "thirteen": "13", "fourteen": "14", "fifteen": "15",
	"sixteen": "16", "seventeen": "17", "eighteen": "18", "nineteen": "19", "twenty": "20", "thirty": "30", "forty": "40",
	"fifty": "50", "sixty": "60", "seventy": "70", "eighty": "80", "ninety": "90", "hundred": "100",
	"first": "1", "second": "2", "third": "3",
}

// magnitude words say how big something is. Written by the model with none in
// the video, they are an invented statistic.
var magnitude = []string{
	"million", "billion", "trillion", "thousand", "percent", "lakh", "lakhs", "crore", "crores",
	"\u0932\u093e\u0916", "\u0915\u0930\u094b\u0921\u093c", "\u0939\u091c\u093c\u093e\u0930", "\u0939\u091c\u093e\u0930", "\u092a\u094d\u0930\u0924\u093f\u0936\u0924", "\u092b\u0940\u0938\u0926\u0940",
}

// promises are the words of a claim no caption may make unless the video does.
var promises = []string{
	"guarantee", "guaranteed", "guarantees", "proven", "miracle", "cure", "cures", "cured", "secret trick", "number one",
	"best in the world", "100% ", "risk-free", "risk free", "get rich", "overnight",
	"\u0917\u093e\u0930\u0902\u091f\u0940", "\u091a\u092e\u0924\u094d\u0915\u093e\u0930",
}

// devanagariDigits maps the Devanagari numerals to ASCII.
var devanagariDigits = strings.NewReplacer(
	"\u0966", "0", "\u0967", "1", "\u0968", "2", "\u0969", "3", "\u096a", "4",
	"\u096b", "5", "\u096c", "6", "\u096d", "7", "\u096e", "8", "\u096f", "9",
)

func normalizeForFacts(s string) string {
	s = devanagariDigits.Replace(s)
	s = thousandsSep.ReplaceAllString(s, "$1$2")
	s = strings.ReplaceAll(s, "%", " percent ")
	s = strings.ToLower(s)
	return spaces.ReplaceAllString(strings.TrimSpace(s), " ")
}

func newMaterial(s *source) *material {
	var b strings.Builder
	b.WriteString(s.topic)
	for _, g := range s.segments {
		b.WriteString(" ")
		b.WriteString(g.Text)
	}
	m := &material{text: normalizeForFacts(b.String()), digits: map[string]bool{}, tokens: map[string]bool{}}
	for _, d := range digitRun.FindAllString(m.text, -1) {
		m.digits[strings.TrimLeft(d, "0")+zeroIfEmpty(d)] = true
	}
	for _, w := range tokenize(m.text) {
		m.tokens[w] = true
		m.words = append(m.words, w)
		if d, ok := numberWords[w]; ok {
			m.digits[d] = true
		}
	}
	return m
}

func zeroIfEmpty(d string) string {
	if strings.Trim(d, "0") == "" {
		return "0"
	}
	return ""
}

// tokenize splits text into words: runs of letters, marks and digits.
func tokenize(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool {
		return !(unicode.IsLetter(r) || unicode.IsMark(r) || unicode.IsDigit(r) || r == '\u200c' || r == '\u200d')
	})
}

// has reports whether quote appears in the material, ignoring case and spacing.
func (m *material) has(quote string) bool {
	q := normalizeForFacts(quote)
	return utf8.RuneCountInString(q) >= 3 && strings.Contains(m.text, q)
}

// stemShared reports whether the material has a word that starts the same way
// as w for at least five characters, so a hashtag for "restoration" is
// supported by a video that says "restoring".
func (m *material) stemShared(w string) bool {
	r := []rune(w)
	if len(r) < 5 {
		return m.tokens[w]
	}
	stem := string(r[:5])
	for _, t := range m.words {
		if strings.HasPrefix(t, stem) {
			return true
		}
	}
	return false
}

// tagSupported says a hashtag is about something the video says. evidence is
// the words the model quoted from the material to justify it. The tag need not
// be a word of the video: "#Restoration" is supported by "restoring", and a
// topic tag by a quoted phrase that is there. What must hold is that the
// evidence is real, or the tag's own word is.
func (m *material) tagSupported(tag, evidence string) bool {
	if strings.TrimSpace(evidence) != "" && m.has(evidence) && hasLetterOrMore(evidence) {
		return true
	}
	body := normalizeForFacts(strings.TrimPrefix(tag, "#"))
	if body == "" {
		return false
	}
	return strings.Contains(m.text, body) || m.stemShared(body)
}

func hasLetterOrMore(s string) bool {
	return strings.IndexFunc(s, unicode.IsLetter) >= 0 && utf8.RuneCountInString(strings.TrimSpace(s)) >= 4
}

// unsupported returns why text states something the material does not, or "".
// names turns on the check for capitalised names, which only means something in
// running text: a title is written in Title Case and would trip it.
func (m *material) unsupported(text string, names bool) string {
	t := normalizeForFacts(text)
	for _, d := range digitRun.FindAllString(t, -1) {
		if !m.digits[strings.TrimLeft(d, "0")+zeroIfEmpty(d)] {
			return "a number the video does not say"
		}
	}
	for _, w := range magnitude {
		if strings.Contains(t, w) && !strings.Contains(m.text, w) {
			return "a size or percentage the video does not say"
		}
	}
	for _, w := range promises {
		if strings.Contains(t, w) && !strings.Contains(m.text, w) {
			return "a promise the video does not make"
		}
	}
	if names {
		if n := m.unknownName(text); n != "" {
			return "a name the video does not say"
		}
	}
	return ""
}

// unknownName finds a capitalised word in the middle of a sentence that the
// material has no word for. Latin script only: other scripts have no capitals.
func (m *material) unknownName(text string) string {
	for _, s := range splitSentences(strings.ReplaceAll(text, "\n", " ")) {
		toks := strings.Fields(s)
		for i, tok := range toks {
			if i == 0 {
				continue
			}
			w := strings.TrimFunc(tok, func(r rune) bool { return !unicode.IsLetter(r) })
			if utf8.RuneCountInString(w) < 3 {
				continue
			}
			first, _ := utf8.DecodeRuneInString(w)
			if !unicode.IsUpper(first) || !isLatin(w) {
				continue
			}
			if possessive := strings.TrimSuffix(strings.ToLower(w), "s"); !m.tokens[strings.ToLower(w)] && !m.stemShared(strings.ToLower(w)) && !m.tokens[possessive] {
				return w
			}
		}
	}
	return ""
}

func isLatin(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) && !unicode.Is(unicode.Latin, r) {
			return false
		}
	}
	return true
}
