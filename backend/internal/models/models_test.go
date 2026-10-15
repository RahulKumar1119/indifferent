package models

import "testing"

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
