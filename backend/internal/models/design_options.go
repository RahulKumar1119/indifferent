package models

import (
	"fmt"
	"strings"
)

// DesignOptions are the small overlay choices applied on top of a design.
// Empty fields mean "use the design default"; Normalize fills them in before
// a project or preview is stored so every consumer sees the same values.
type DesignOptions struct {
	Accent             string `json:"accent,omitempty"`
	TextSize           string `json:"textSize,omitempty"` // s|m|l
	ShowQuestionNumber *bool  `json:"showQuestionNumber,omitempty"`
	AnswerLabels       string `json:"answerLabels,omitempty"` // letters|numbers
}

var designAccents = map[string][]string{
	"classic":   {"#7C3AED", "#92400E"},
	"modern":    {"#0369A1", "#9D174D"},
	"education": {"#B3261E", "#1E40AF"},
	"dark":      {"#A78BFA", "#22D3EE"},
	"minimal":   {"#111111", "#B3261E"},
	"neon":      {"#FF3DCB", "#22D3EE"},
}

// AllowedAccents returns the curated accents for a design. The first entry is
// the design's default. Unknown designs have no overrides.
func AllowedAccents(design string) []string {
	return append([]string(nil), designAccents[strings.ToLower(strings.TrimSpace(design))]...)
}

func (o DesignOptions) IsZero() bool {
	return o.Accent == "" && o.TextSize == "" && o.ShowQuestionNumber == nil && o.AnswerLabels == ""
}

func (o *DesignOptions) Normalize() {
	o.Accent = strings.ToLower(strings.TrimSpace(o.Accent))
	o.TextSize = strings.ToLower(strings.TrimSpace(o.TextSize))
	o.AnswerLabels = strings.ToLower(strings.TrimSpace(o.AnswerLabels))
	if o.TextSize == "" {
		o.TextSize = "m"
	}
	if o.AnswerLabels == "" {
		o.AnswerLabels = "letters"
	}
	if o.ShowQuestionNumber == nil {
		show := true
		o.ShowQuestionNumber = &show
	}
}

// ValidateDesignOptions normalizes known fields and rejects values that are
// not in the allow-list for the design. Unknown designs are caught by the
// caller's template validation.
func ValidateDesignOptions(design string, o *DesignOptions) error {
	o.Normalize()
	switch o.TextSize {
	case "s", "m", "l":
	default:
		return fmt.Errorf("invalid text size %q: must be s, m or l", o.TextSize)
	}
	switch o.AnswerLabels {
	case "letters", "numbers":
	default:
		return fmt.Errorf("invalid answer labels %q: must be letters or numbers", o.AnswerLabels)
	}
	if o.Accent != "" {
		allowed := AllowedAccents(design)
		for _, a := range allowed {
			if strings.EqualFold(o.Accent, a) {
				o.Accent = a
				return nil
			}
		}
		return fmt.Errorf("accent %q is not available for design %q", o.Accent, design)
	}
	return nil
}

// ShowNumbers reports whether question numbers are enabled (nil means yes).
func (o DesignOptions) ShowNumbers() bool {
	return o.ShowQuestionNumber == nil || *o.ShowQuestionNumber
}
