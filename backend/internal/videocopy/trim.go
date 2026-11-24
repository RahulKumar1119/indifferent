package videocopy

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// sentenceEnd are the characters that end a sentence in English and Hindi.
const sentenceEnd = ".!?\u0964\u0965\u2026"

// fitPrefix returns the longest whole-sentence prefix of text for which fits is
// true, or, when not even the first sentence fits, the longest whole-word
// prefix. It never cuts inside a word, and it never leaves a hashtag or a link
// cut short, because the sentences and words it keeps are whole. The second
// result is false when nothing fits.
//
// Whole sentences are preferred, but a text is not thrown away because its
// first sentence is the only one that fits, and a caption that is one long
// sentence is cut at a word.
func fitPrefix(text string, fits func(string) bool) (string, bool) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", false
	}
	if fits(text) {
		return text, true
	}
	var sentence, word []int // byte offsets just after a boundary
	runes := []rune(text)
	off := 0
	for i, r := range runes {
		n := utf8.RuneLen(r)
		next := off + n
		if strings.ContainsRune(sentenceEnd, r) && (i+1 == len(runes) || unicode.IsSpace(runes[i+1])) {
			sentence = append(sentence, next)
		}
		if unicode.IsSpace(r) {
			word = append(word, off) // cut before the space
		}
		off = next
	}
	best := func(cuts []int) (string, bool) {
		for i := len(cuts) - 1; i >= 0; i-- {
			if cand := strings.TrimSpace(text[:cuts[i]]); cand != "" && fits(cand) {
				return cand, true
			}
		}
		return "", false
	}
	if s, ok := best(sentence); ok {
		return s, true
	}
	return best(word)
}

// dropSentences removes every sentence for which bad is true and returns the
// rest, keeping the paragraph breaks. A sentence ends at . ! ? or the Hindi
// danda followed by a space or the end, or at a line break.
func dropSentences(text string, bad func(string) bool) (string, int) {
	dropped := 0
	var paras []string
	for _, p := range strings.Split(text, "\n\n") {
		var kept []string
		for _, line := range strings.Split(p, "\n") {
			var out []string
			for _, s := range splitSentences(line) {
				if bad(s) {
					dropped++
					continue
				}
				out = append(out, s)
			}
			if l := strings.TrimSpace(strings.Join(out, " ")); l != "" {
				kept = append(kept, l)
			}
		}
		if len(kept) > 0 {
			paras = append(paras, strings.Join(kept, "\n"))
		}
	}
	return strings.Join(paras, "\n\n"), dropped
}

func splitSentences(line string) []string {
	var out []string
	runes := []rune(line)
	start := 0
	for i, r := range runes {
		if strings.ContainsRune(sentenceEnd, r) && (i+1 == len(runes) || unicode.IsSpace(runes[i+1])) {
			if s := strings.TrimSpace(string(runes[start : i+1])); s != "" {
				out = append(out, s)
			}
			start = i + 1
		}
	}
	if s := strings.TrimSpace(string(runes[start:])); s != "" {
		out = append(out, s)
	}
	return out
}
