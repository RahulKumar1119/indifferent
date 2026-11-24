package videocopy

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ValidateSaved checks copy a person edited before it is stored. It applies the
// same limits as generation, counted the same way, and it never changes what it
// is given: a copy that is too long is refused with a message that names the
// platform, the limit and the length, so the person decides what to cut. Nothing
// is truncated, so no word, link or hashtag is ever cut short.
//
// A link a person types is theirs and is kept; only a model's links are removed.
func ValidateSaved(specs []PlatformSpec, in map[Platform]PlatformCopy) error {
	for id := range in {
		if _, ok := PlatformByID(specs, id); !ok {
			return fmt.Errorf("%q is not a platform", string(id))
		}
	}
	for _, sp := range specs {
		c, ok := in[sp.ID]
		if !ok {
			return fmt.Errorf("%s is missing", sp.Label)
		}
		if err := validateSavedPlatform(sp, c); err != nil {
			return err
		}
	}
	return nil
}

func validateSavedPlatform(sp PlatformSpec, c PlatformCopy) error {
	if sp.HasTitle {
		if strings.TrimSpace(c.Title) == "" {
			return fmt.Errorf("%s needs a title", sp.Label)
		}
		if err := plainText(sp.Label+" title", c.Title, false); err != nil {
			return err
		}
		if n := Count(c.Title, sp.TitleUnit); n > sp.TitleMax {
			return fmt.Errorf("%s title is %s %s; the limit is %s %s.", sp.Label, commas(n), unitName(sp.TitleUnit), commas(sp.TitleMax), unitName(sp.TitleUnit))
		}
	} else if c.Title != "" {
		return fmt.Errorf("%s has no title", sp.Label)
	}
	if strings.TrimSpace(c.Text) == "" {
		return fmt.Errorf("%s needs its %s", sp.Label, strings.ToLower(sp.TextLabel))
	}
	if err := plainText(sp.Label+" "+strings.ToLower(sp.TextLabel), c.Text, true); err != nil {
		return err
	}
	if len(c.Hashtags) > sp.HashtagsMax {
		return fmt.Errorf("%s allows at most %d hashtags.", sp.Label, sp.HashtagsMax)
	}
	seen := map[string]bool{}
	for _, h := range c.Hashtags {
		if err := ValidateHashtag(h); err != nil {
			return fmt.Errorf("%s: %w", sp.Label, err)
		}
		k := strings.ToLower(h)
		if seen[k] {
			return fmt.Errorf("%s has %s twice.", sp.Label, h)
		}
		seen[k] = true
	}
	if n := Count(FullText(c), sp.TextUnit); n > sp.TextMax {
		return fmt.Errorf("%s %s with its hashtags is %s %s; the limit is %s %s.", sp.Label, strings.ToLower(sp.TextLabel), commas(n), unitName(sp.TextUnit), commas(sp.TextMax), unitName(sp.TextUnit))
	}
	return nil
}

// plainText refuses text that is not valid UTF-8 or holds control characters. A
// line break is allowed where multiline is true.
func plainText(what, s string, multiline bool) error {
	if !utf8.ValidString(s) {
		return fmt.Errorf("The %s is not valid text.", what)
	}
	for _, r := range s {
		if r == '\n' && multiline {
			continue
		}
		if unicode.IsControl(r) {
			return fmt.Errorf("The %s has a character that cannot be saved.", what)
		}
	}
	return nil
}

func unitName(u Unit) string {
	switch u {
	case UnitBytes:
		return "bytes"
	case UnitUTF16:
		return "characters"
	}
	return "characters"
}

// commas writes 5000 as 5,000.
func commas(n int) string {
	s := fmt.Sprint(n)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}
