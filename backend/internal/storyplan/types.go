package storyplan

import (
	"strings"
	"unicode/utf8"
)

// Scene is one visual with the narration spoken over it.
type Scene struct {
	// Narration is what the narrator says, in the story's own language and
	// script.
	Narration string `json:"narration"`
	// Description is one English sentence saying what to show. It doubles as
	// alt text and as a fallback search phrase.
	Description string `json:"description"`
	// Keywords are English stock-search terms, most concrete first.
	Keywords []string `json:"keywords"`
	// MediaType is MediaPhoto or MediaVideo: what to look for first.
	MediaType string `json:"mediaType"`
	// Duration is how long the scene is shown, in seconds. It is an estimate
	// from the narration's length; the real length is known once the speech is
	// synthesised.
	Duration float64 `json:"duration"`
	// Media is the stock asset the user picked for this scene, or nil while none
	// is picked. It holds a reference and never a URL: Pixabay forbids keeping
	// its URLs and Pexels video links are signed, so the API looks the asset up
	// again each time a plan is opened.
	Media *MediaRef `json:"media,omitempty"`
}

// MediaRef names a stock asset without any URL.
type MediaRef struct {
	Provider string `json:"provider"` // "pexels" or "pixabay"
	Type     string `json:"type"`     // MediaPhoto or MediaVideo
	ID       string `json:"id"`       // the provider's numeric id
}

// SearchQuery joins the scene's strongest keywords into one stock-search query
// of at most maxRunes characters (pass stockmedia.MaxQueryRunes, 100). The
// stock providers match every word, so a short query of the best keywords finds
// more than all of them together. It is empty only for a scene with no
// keywords, which a Plan never has.
func (s Scene) SearchQuery(maxRunes int) string {
	var b strings.Builder
	for i, k := range s.Keywords {
		if i >= 3 {
			break
		}
		next := k
		if b.Len() > 0 {
			next = " " + k
		}
		if utf8.RuneCountInString(b.String())+utf8.RuneCountInString(next) > maxRunes {
			break
		}
		b.WriteString(next)
	}
	return b.String()
}

// Plan is a short video planned from a story.
type Plan struct {
	// Language is the ISO 639-1 code of the story's language ("hi", "en"). It
	// is what a later phase uses to pick a narration voice.
	Language string `json:"language"`
	// Hook is the attention-grabbing opening line, in the story's language.
	// The prompt asks for Scenes[0]'s narration to open with it; that is not
	// enforced, since a paraphrase is harmless.
	Hook   string  `json:"hook"`
	Scenes []Scene `json:"scenes"`
	// TotalSeconds is the sum of the scene durations.
	TotalSeconds float64 `json:"totalSeconds"`
	// Warnings lists repairs made to the model's answer (a scene dropped, a
	// duration corrected). They name scenes and reasons, never story text.
	Warnings []string `json:"warnings,omitempty"`
}
