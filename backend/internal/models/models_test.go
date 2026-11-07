package models

import (
	"strings"
	"testing"
)

func TestClipEditValidate(t *testing.T) {
	if err := (ClipEdit{Start: 5, End: 10}).Validate(120); err != nil {
		t.Errorf("trim-only edit must validate: %v", err)
	}
	withWords := ClipEdit{Start: 5, End: 10, Words: []Word{{Text: "hi", Start: 6, End: 6.5}}}
	if err := withWords.Validate(120); err != nil {
		t.Errorf("text edit must validate: %v", err)
	}
	for name, edit := range map[string]ClipEdit{
		"end-before-start": {Start: 10, End: 5},
		"negative":         {Start: -1, End: 5},
		"too-short":        {Start: 5, End: 5.5},
		"over-duration":    {Start: 5, End: 200},
		"empty-word":       {Start: 5, End: 10, Words: []Word{{Text: " ", Start: 6, End: 6.5}}},
		"bad-timing":       {Start: 5, End: 10, Words: []Word{{Text: "x", Start: 7, End: 6}}},
	} {
		if err := edit.Validate(120); err == nil {
			t.Errorf("%s: expected validation error", name)
		}
	}
}

func TestClipEditValidate_CaptionWordLimits(t *testing.T) {
	word := func(text string) []Word { return []Word{{Text: text, Start: 6, End: 6.5}} }
	ok := ClipEdit{Start: 5, End: 10}

	for name, text := range map[string]string{
		"line feed":           "hi\nthere",
		"carriage return":     "hi\rthere",
		"line break then ASS": "x\nDialogue: 0,0:00:00.00,0:00:09.00,Default,,0,0,0,,injected",
		"tab":                 "hi\tthere",
		"NUL":                 "hi\x00",
		"escape":              "hi\x1b[0m",
		"too long":            strings.Repeat("a", maxEditWordRunes+1),
		"too long, multibyte": strings.Repeat("\u0905", maxEditWordRunes+1),
	} {
		e := ok
		e.Words = word(text)
		if err := e.Validate(120); err == nil {
			t.Errorf("%s: expected a validation error", name)
		}
	}

	for name, text := range map[string]string{
		"plain":                    "hello",
		"punctuation and quote":    `"Don't,"`,
		"hindi":                    "\u0928\u092e\u0938\u094d\u0924\u0947",
		"exactly the limit":        strings.Repeat("a", maxEditWordRunes),
		"limit in multibyte":       strings.Repeat("\u0905", maxEditWordRunes),
		"braces are escaped later": "{x}",
	} {
		e := ok
		e.Words = word(text)
		if err := e.Validate(120); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}

	many := ok
	for i := 0; i <= maxEditWords; i++ {
		many.Words = append(many.Words, Word{Text: "w", Start: 6, End: 6.5})
	}
	if err := many.Validate(120); err == nil {
		t.Errorf("%d words should be refused", len(many.Words))
	}
	many.Words = many.Words[:maxEditWords]
	if err := many.Validate(120); err != nil {
		t.Errorf("exactly %d words should be accepted: %v", maxEditWords, err)
	}
}
