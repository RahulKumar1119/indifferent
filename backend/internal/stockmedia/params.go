package stockmedia

import (
	"fmt"
	"strings"
	"unicode"
)

const maxPage = 1000

// normalizeParams validates p and fills defaults, so a provider and the cache
// both see one canonical form. The query is trimmed, stripped of control
// characters and has runs of whitespace collapsed.
func normalizeParams(p SearchParams) (SearchParams, error) {
	q := strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, p.Query)
	q = strings.Join(strings.Fields(q), " ")
	if q == "" {
		return p, fmt.Errorf("%w: query is empty", ErrInvalidParams)
	}
	if n := len([]rune(q)); n > MaxQueryRunes {
		return p, fmt.Errorf("%w: query is %d characters, the limit is %d", ErrInvalidParams, n, MaxQueryRunes)
	}
	if hasOnlyUnsupportedScript(q) {
		// Checked live on 2026-10-29: Hindi-only queries return 0 hits on
		// Pixabay images and videos (5 of 5 words tried), where the English
		// equivalent returns up to 500, and lang=hi is accepted but changes
		// nothing. Failing here saves calls (and the rate limit they use) that
		// could only come back empty, and tells the caller why.
		return p, fmt.Errorf("%w: %w", ErrInvalidParams, ErrUnsupportedLanguage)
	}
	p.Query = q

	if p.Type != MediaPhoto && p.Type != MediaVideo {
		return p, fmt.Errorf("%w: type must be %q or %q", ErrInvalidParams, MediaPhoto, MediaVideo)
	}

	switch p.Orientation {
	case "":
		p.Orientation = OrientationPortrait
	case OrientationAny, OrientationPortrait, OrientationLandscape, OrientationSquare:
	default:
		return p, fmt.Errorf("%w: unknown orientation %q", ErrInvalidParams, p.Orientation)
	}

	if p.Page == 0 {
		p.Page = 1
	}
	if p.Page < 1 || p.Page > maxPage {
		return p, fmt.Errorf("%w: page must be between 1 and %d", ErrInvalidParams, maxPage)
	}

	if p.PerPage == 0 {
		p.PerPage = DefaultPerPage
	}
	if p.PerPage < MinPerPage || p.PerPage > MaxPerPage {
		return p, fmt.Errorf("%w: per-page must be between %d and %d", ErrInvalidParams, MinPerPage, MaxPerPage)
	}
	return p, nil
}

// unsupportedScripts are the Indian scripts. Neither provider lists any Indian
// language among the ones it can search: Pexels' locale list and Pixabay's
// lang list have no Hindi, Bengali, Tamil or the like. Other non-Latin scripts
// (Japanese, Korean, Chinese, Russian, Greek, Thai, Bulgarian) are on those
// lists and are deliberately allowed through.
var unsupportedScripts = []*unicode.RangeTable{
	unicode.Devanagari, unicode.Bengali, unicode.Gurmukhi, unicode.Gujarati, unicode.Oriya,
	unicode.Tamil, unicode.Telugu, unicode.Kannada, unicode.Malayalam, unicode.Sinhala,
}

// hasOnlyUnsupportedScript reports whether s is written in an unsupported
// script with no word from any other script to search on: it has at least one
// such character and no letter outside those scripts.
//
// A mixed query is deliberately not rejected. Checked live on 2026-10-29,
// "mountain पहाड़" returns the same 500 results as "mountain" on Pixabay images
// and videos, so the unsupported word is ignored and the rest still searches.
// Only a query with nothing but Hindi in it comes back empty.
//
// Digits and punctuation do not count as something to search on. "पहाड़ 2026"
// would search for "2026" alone and return arbitrary media unrelated to the
// scene, which is worse than a clear error.
func hasOnlyUnsupportedScript(s string) bool {
	unsupported := false
	for _, r := range s {
		switch {
		case unicode.IsOneOf(unsupportedScripts, r):
			unsupported = true
		case unicode.IsLetter(r):
			return false // a word the providers can match remains
		}
	}
	return unsupported
}

// cleanText makes provider-supplied text safe to store and show: control
// characters and runs of whitespace become single spaces, and the result is
// cut to max runes. Providers return author names and descriptions that end
// up in the UI and in credit lines, so they are treated as untrusted.
func cleanText(s string, max int) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, s)
	s = strings.Join(strings.Fields(s), " ")
	if runes := []rune(s); len(runes) > max {
		s = strings.TrimSpace(string(runes[:max]))
	}
	return s
}

// maxAssetIDDigits bounds an asset id. Both providers use positive integers;
// 18 digits is below the int64 limit, so an id can always be parsed.
const maxAssetIDDigits = 18

// validateAssetID checks that id looks like a provider asset id: 1 to 18 ASCII
// digits and nothing else. Ids go into request paths and query strings, so
// anything else is refused before it can reach a URL.
func validateAssetID(id string) error {
	if id == "" || len(id) > maxAssetIDDigits {
		return fmt.Errorf("%w: asset id must be 1 to %d digits", ErrInvalidParams, maxAssetIDDigits)
	}
	for i := 0; i < len(id); i++ {
		if id[i] < '0' || id[i] > '9' {
			return fmt.Errorf("%w: asset id must be digits only", ErrInvalidParams)
		}
	}
	return nil
}

// ValidAssetID reports whether id has the shape of a provider asset id.
func ValidAssetID(id string) bool { return validateAssetID(id) == nil }
