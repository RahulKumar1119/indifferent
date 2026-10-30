package storyplan

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// SceneEdit is what a user may change about one scene. A nil field leaves that
// part as it is.
type SceneEdit struct {
	// Narration replaces the scene's narration.
	Narration *string `json:"narration,omitempty"`
	// Media picks the stock asset for the scene. ClearMedia removes it.
	Media      *MediaRef `json:"media,omitempty"`
	ClearMedia bool      `json:"clearMedia,omitempty"`
}

// MaxMediaIDDigits bounds a provider asset id. Both providers use positive
// integers, and 18 digits is below the int64 limit.
const MaxMediaIDDigits = 18

var mediaProviders = map[string]bool{"pexels": true, "pixabay": true}

// Valid reports whether the reference is well formed: a known provider, a
// known type and a numeric id. It does not check that the asset exists.
func (m MediaRef) Valid() bool {
	if !mediaProviders[m.Provider] || (m.Type != MediaPhoto && m.Type != MediaVideo) {
		return false
	}
	if m.ID == "" || len(m.ID) > MaxMediaIDDigits {
		return false
	}
	for i := 0; i < len(m.ID); i++ {
		if m.ID[i] < '0' || m.ID[i] > '9' {
			return false
		}
	}
	return true
}

// EditError is a rejected edit. Reason is safe to show to the user and names the
// scene, never the text submitted.
type EditError struct{ Reason string }

func (e *EditError) Error() string        { return "invalid edit: " + e.Reason }
func (e *EditError) Is(target error) bool { return target == ErrInvalidEdit }

func invalidEdit(format string, args ...any) error {
	return &EditError{Reason: fmt.Sprintf(format, args...)}
}

// ApplyEdits returns a copy of plan with the edits applied. edits[i] is for
// scene i, and len(edits) must equal the number of scenes, so an edit can never
// be applied to the wrong scene after the plan changes under the user.
//
// Edited narration is held to the same rules as the model's: it is cleaned of
// markup and control characters, must not be empty, must fit one scene, and must
// still be mostly in script, the story's own writing system (given as story).
// A narration edit re-estimates the scene's duration from the new text, since
// the old number described the old words. The plan's total must still be within
// MaxTotalSeconds, or the edit is refused: unlike a model answer, a user's edit
// is not trimmed from the end, because the user would not expect scenes to
// disappear. Nothing is applied unless every edit is valid.
func ApplyEdits(plan Plan, edits []SceneEdit, story Script) (Plan, error) {
	if len(edits) != len(plan.Scenes) {
		return Plan{}, invalidEdit("the edit has %d scenes but the plan has %d; reload the plan and try again", len(edits), len(plan.Scenes))
	}
	out := plan
	out.Scenes = make([]Scene, len(plan.Scenes))
	copy(out.Scenes, plan.Scenes)
	out.Warnings = nil // a user's edit supersedes the notes about the model's answer

	for i, e := range edits {
		n := i + 1
		sc := out.Scenes[i]
		sc.Keywords = append([]string(nil), sc.Keywords...)

		if e.Narration != nil {
			text := cleanText(*e.Narration, MaxNarrationRunes)
			if text == "" {
				return Plan{}, invalidEdit("scene %d: the narration is empty", n)
			}
			if utf8.RuneCountInString(strings.TrimSpace(*e.Narration)) > MaxNarrationRunes*2 {
				return Plan{}, invalidEdit("scene %d: the narration is too long", n)
			}
			if story != ScriptNone && story != ScriptOther && ShareInScript(text, story) < minScriptShare {
				return Plan{}, invalidEdit("scene %d: the narration should be in the same language as the story", n)
			}
			speech := estimateSpeech(text)
			if speech > MaxSceneSeconds*1.2 {
				return Plan{}, invalidEdit("scene %d: the narration is too long for one scene, split it in two", n)
			}
			sc.Narration = text
			sc.Duration, _ = sceneDuration(nil, speech)
		}

		switch {
		case e.ClearMedia && e.Media != nil:
			return Plan{}, invalidEdit("scene %d: choose a media item or clear it, not both", n)
		case e.ClearMedia:
			sc.Media = nil
		case e.Media != nil:
			if !e.Media.Valid() {
				return Plan{}, invalidEdit("scene %d: that media choice is not valid", n)
			}
			m := *e.Media
			sc.Media = &m
		}
		out.Scenes[i] = sc
	}

	total := sumDurations(out.Scenes)
	if total > MaxTotalSeconds {
		return Plan{}, invalidEdit("with these changes the video would be %.0f seconds, over the %.0f second limit; shorten the narration", total, MaxTotalSeconds)
	}
	out.TotalSeconds = round1(total)
	return out, nil
}
