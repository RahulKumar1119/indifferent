package storyplan

import (
	"strings"
	"unicode"
)

// Script is the writing system a text is mostly in.
type Script string

// Scripts the package tells apart. Hindi is Devanagari; the other Indian
// scripts are grouped because stock providers cannot search any of them.
const (
	ScriptLatin      Script = "latin"
	ScriptDevanagari Script = "devanagari"
	ScriptIndic      Script = "indic" // Bengali, Tamil, Telugu and the other Indian scripts besides Devanagari
	ScriptOther      Script = "other" // anything else with letters
	ScriptNone       Script = ""      // no letters at all
)

// indicScripts are the Indian scripts other than Devanagari. They are the ones
// Pexels and Pixabay cannot search (see stockmedia/TERMS.md).
var indicScripts = []*unicode.RangeTable{
	unicode.Bengali, unicode.Gurmukhi, unicode.Gujarati, unicode.Oriya,
	unicode.Tamil, unicode.Telugu, unicode.Kannada, unicode.Malayalam, unicode.Sinhala,
}

// scriptOf classifies one letter. Combining marks used by Indian scripts
// (vowel signs, virama) are not letters in Unicode, so callers count letters
// with isLetterLike, which includes them.
func scriptOf(r rune) Script {
	switch {
	case unicode.Is(unicode.Devanagari, r):
		return ScriptDevanagari
	case unicode.IsOneOf(indicScripts, r):
		return ScriptIndic
	case unicode.Is(unicode.Latin, r):
		return ScriptLatin
	default:
		return ScriptOther
	}
}

// isLetterLike reports whether r counts toward a script: a letter, or a mark
// that is part of a word in an Indian script (the vowel signs of Devanagari are
// category Mc/Mn and would otherwise be ignored, shrinking Hindi text's weight
// against Latin).
func isLetterLike(r rune) bool {
	if unicode.IsLetter(r) {
		return true
	}
	return unicode.IsMark(r) && (unicode.Is(unicode.Devanagari, r) || unicode.IsOneOf(indicScripts, r))
}

// scriptCounts counts the letter-like characters of s per script.
func scriptCounts(s string) (counts map[Script]int, total int) {
	counts = make(map[Script]int, 4)
	for _, r := range s {
		if !isLetterLike(r) {
			continue
		}
		counts[scriptOf(r)]++
		total++
	}
	return counts, total
}

// DominantScript returns the script most of s's letters are in, or ScriptNone
// when s has no letters. Digits, punctuation and spaces are ignored.
func DominantScript(s string) Script {
	counts, total := scriptCounts(s)
	if total == 0 {
		return ScriptNone
	}
	best, bestN := ScriptNone, -1
	// A fixed order keeps ties deterministic: Latin wins a tie, since mixed
	// Hindi-English text is common and English is the safer assumption.
	for _, sc := range []Script{ScriptLatin, ScriptDevanagari, ScriptIndic, ScriptOther} {
		if counts[sc] > bestN {
			best, bestN = sc, counts[sc]
		}
	}
	return best
}

// ShareInScript returns the fraction of s's letters that are in the given
// script, from 0 to 1. A text with no letters has share 0.
func ShareInScript(s string, sc Script) float64 {
	counts, total := scriptCounts(s)
	if total == 0 {
		return 0
	}
	return float64(counts[sc]) / float64(total)
}

// HasIndianScript reports whether s contains any letter in an Indian script.
// Keywords must not: a single such letter makes a search term unsearchable on
// both providers.
func HasIndianScript(s string) bool {
	for _, r := range s {
		if isLetterLike(r) {
			if sc := scriptOf(r); sc == ScriptDevanagari || sc == ScriptIndic {
				return true
			}
		}
	}
	return false
}

// languageForScript is the best default language code for a script when the
// model does not give a usable one. Devanagari is taken to be Hindi (Marathi,
// Nepali and Sanskrit also use it; the product supports Hindi). Latin is taken
// to be English, which also covers Hindi written in Latin letters ("Hinglish"):
// the script, not the language, is what the checks depend on.
func languageForScript(sc Script) string {
	switch sc {
	case ScriptDevanagari:
		return "hi"
	case ScriptLatin:
		return "en"
	default:
		return ""
	}
}

// wordCount counts whitespace-separated words.
func wordCount(s string) int { return len(strings.Fields(s)) }
