package models

import (
	"math"
	"strings"
	"testing"
)

func TestHookTextLength_CountsWhatAReaderSees(t *testing.T) {
	cases := []struct {
		text string
		want int
	}{
		{"", 0},
		{"Stop scrolling", 14},
		{"café", 4},
		{"cafe\u0301", 4}, // e + combining acute
		// क्ष is a conjunct (क + virama + ष): one character. मा is one.
		{"क्षमा", 2},
		{"नमस्ते", 3},  // न, म, स्ते
		{"हिंदी", 2},   // हिं, दी
		{"प्रश्न", 2},  // प्र, श्न
		{"ज़िंदगी", 3}, // ज़िं (nukta), द, गी
		{"👍🏽", 1},      // skin tone
		{"👨‍👩‍👧", 1},   // ZWJ family
		{"🇮🇳🇺🇸", 2},    // two flags
		{"a\r\nb", 3},
	}
	for _, c := range cases {
		if got := HookTextLength(c.text); got != c.want {
			t.Errorf("HookTextLength(%q) = %d, want %d", c.text, got, c.want)
		}
	}
}

func TestClipHook_NormalizedFillsDefaults(t *testing.T) {
	h := ClipHook{Enabled: true, Text: "  hello  "}.Normalized()
	if h.Text != "hello" || h.Style != HookStyleBox || h.Position != HookTop || h.Seconds != DefaultHookSeconds {
		t.Fatalf("normalized = %+v", h)
	}
}

func TestClipHook_Validate(t *testing.T) {
	ok := ClipHook{Enabled: true, Text: "This changes everything", Style: HookStyleBand, Position: HookMiddle, Seconds: 1}
	if err := ok.Validate(); err != nil {
		t.Fatalf("valid hook: %v", err)
	}
	hindi80 := strings.Repeat("क्ष", 80) // 80 conjuncts, 240 code points
	if err := (ClipHook{Enabled: true, Text: hindi80, Style: HookStyleBox, Position: HookTop, Seconds: 2.5}).Validate(); err != nil {
		t.Fatalf("80 Hindi characters must be allowed: %v", err)
	}
	bad := map[string]ClipHook{
		"style":        {Enabled: true, Text: "x", Style: "neon", Position: HookTop, Seconds: 2},
		"position":     {Enabled: true, Text: "x", Style: HookStyleBox, Position: "bottom", Seconds: 2},
		"short":        {Enabled: true, Text: "x", Style: HookStyleBox, Position: HookTop, Seconds: 0.9},
		"long":         {Enabled: true, Text: "x", Style: HookStyleBox, Position: HookTop, Seconds: 5.1},
		"nan":          {Enabled: true, Text: "x", Style: HookStyleBox, Position: HookTop, Seconds: math.NaN()},
		"empty on":     {Enabled: true, Text: "  ", Style: HookStyleBox, Position: HookTop, Seconds: 2},
		"81 chars":     {Enabled: true, Text: strings.Repeat("a", 81), Style: HookStyleBox, Position: HookTop, Seconds: 2},
		"81 hindi":     {Enabled: true, Text: strings.Repeat("क्ष", 81), Style: HookStyleBox, Position: HookTop, Seconds: 2},
		"line break":   {Enabled: true, Text: "one\ntwo", Style: HookStyleBox, Position: HookTop, Seconds: 2},
		"line sep":     {Enabled: true, Text: "one\u2028two", Style: HookStyleBox, Position: HookTop, Seconds: 2},
		"control char": {Enabled: true, Text: "one\x00two", Style: HookStyleBox, Position: HookTop, Seconds: 2},
	}
	for name, h := range bad {
		if err := h.Validate(); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
	// Off, half written: saved without text.
	if err := (ClipHook{Style: HookStyleBox, Position: HookTop, Seconds: 2.5}).Validate(); err != nil {
		t.Fatalf("an off hook without text: %v", err)
	}
}

func TestClipHook_Active(t *testing.T) {
	var nilHook *ClipHook
	if nilHook.Active() || (&ClipHook{Text: "x"}).Active() || (&ClipHook{Enabled: true, Text: " "}).Active() {
		t.Fatal("only an enabled hook with text is active")
	}
	if !(&ClipHook{Enabled: true, Text: "x"}).Active() {
		t.Fatal("enabled hook with text is active")
	}
}
