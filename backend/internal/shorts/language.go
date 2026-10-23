package shorts

import (
	"fmt"
	"strings"
)

// Transcription language selections. The codes are BCP-47 language tags
// (matching Amazon Transcribe) except LanguageAuto, which asks Transcribe to
// identify the spoken language from SupportedLanguageCodes.
const (
	LanguageAuto      = "auto"
	LanguageEnglishUS = "en-US"
	LanguageEnglishIN = "en-IN"
	LanguageHindi     = "hi-IN"
)

// LanguageOption is a selectable transcription language, exposed by the API
// so the UI never drifts from the codes the backend accepts.
type LanguageOption struct {
	Code  string `json:"code"`
	Label string `json:"label"`
}

// SupportedLanguages lists every language selection the pipeline accepts.
var SupportedLanguages = []LanguageOption{
	{Code: LanguageAuto, Label: "Auto-detect"},
	{Code: LanguageEnglishUS, Label: "English (US)"},
	{Code: LanguageEnglishIN, Label: "English (India)"},
	{Code: LanguageHindi, Label: "Hindi"},
}

// SupportedLanguageCodes are the codes offered to Amazon Transcribe when
// automatic language identification runs.
var SupportedLanguageCodes = []string{LanguageEnglishUS, LanguageEnglishIN, LanguageHindi}

// NormalizeLanguage resolves aliases (en, hi, empty) to a canonical language
// code. An empty selection means auto-detect, so existing jobs keep working.
func NormalizeLanguage(language string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(language)) {
	case "", "auto", "detect":
		return LanguageAuto, nil
	case "en", "en-us":
		return LanguageEnglishUS, nil
	case "en-in":
		return LanguageEnglishIN, nil
	case "hi", "hi-in":
		return LanguageHindi, nil
	default:
		return "", fmt.Errorf("unsupported language %q: choose auto, en-US, en-IN or hi-IN", language)
	}
}
