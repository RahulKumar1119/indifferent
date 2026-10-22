package models

import (
	"testing"
)

func TestDesignOptionsNormalizeDefaults(t *testing.T) {
	var o DesignOptions
	if err := ValidateDesignOptions("classic", &o); err != nil {
		t.Fatal(err)
	}
	if o.TextSize != "m" || o.AnswerLabels != "letters" || !o.ShowNumbers() {
		t.Errorf("defaults = %+v", o)
	}
	if o.Accent != "" {
		t.Errorf("accent = %q, want design default", o.Accent)
	}
}

func TestDesignOptionsRejectUnknownValues(t *testing.T) {
	tests := []struct {
		name   string
		design string
		opts   DesignOptions
	}{
		{"text size", "classic", DesignOptions{TextSize: "xl"}},
		{"answer labels", "classic", DesignOptions{AnswerLabels: "roman"}},
		{"accent for design", "classic", DesignOptions{Accent: "#FF3DCB"}},
		{"unknown design", "nope", DesignOptions{Accent: "#7C3AED"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := tt.opts
			if err := ValidateDesignOptions(tt.design, &o); err == nil {
				t.Errorf("ValidateDesignOptions(%q, %+v) = nil, want error", tt.design, tt.opts)
			}
		})
	}
}

func TestDesignOptionsAcceptEveryCuratedAccent(t *testing.T) {
	designs := []string{"classic", "modern", "education", "dark", "minimal", "neon"}
	for _, design := range designs {
		accents := AllowedAccents(design)
		if len(accents) < 2 {
			t.Errorf("%s: only %d accents", design, len(accents))
		}
		for _, accent := range accents {
			o := DesignOptions{Accent: accent, TextSize: "s", AnswerLabels: "numbers"}
			if err := ValidateDesignOptions(design, &o); err != nil {
				t.Errorf("%s / %s: %v", design, accent, err)
			}
		}
	}
	if got := AllowedAccents("nope"); len(got) != 0 {
		t.Errorf("unknown design accents = %v", got)
	}
}

func TestDesignOptionsShowNumbers(t *testing.T) {
	if !(DesignOptions{}).ShowNumbers() {
		t.Error("nil ShowQuestionNumber must default to true")
	}
	falseValue := false
	if (DesignOptions{ShowQuestionNumber: &falseValue}).ShowNumbers() {
		t.Error("explicit false must be honored")
	}
}
