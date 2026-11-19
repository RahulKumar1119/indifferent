package render_test

import (
	"reflect"
	"sort"
	"testing"

	"github.com/rahul/indifferent/backend/internal/shorts"
	"github.com/rahul/indifferent/backend/internal/storyjob"
)

// TestCaptionLanguagesAreTheOnesCaptionFittingKnows pins the languages a caption can be in today:
// English and Hindi. Caption fitting (fit.go) measures Latin and Devanagari text, and the render
// images have the fonts for those two (DejaVu and Noto Sans Devanagari).
//
// Arabic, Hebrew and the other right-to-left languages are NOT supported, and neither are Gujarati,
// Marathi in other scripts, Tamil and the other Indian scripts. They need three things together,
// and none of them is done:
//
//  1. right-to-left word order for the karaoke sweep (libass lights words left to right, so an
//     Arabic cue lights its last-spoken word first: checked with an Arabic render for #327),
//  2. shaping and measuring for the script (fit.go measures Latin and Devanagari only; any other
//     character gets a default width), and
//  3. a font in the render images (without one libass draws empty boxes: checked for Gujarati).
//
// If this test fails because a language was added, do those three things, add the language to the
// real-render check in scripts/ops/caption_fit_check.sh, and only then change the lists here.
func TestCaptionLanguagesAreTheOnesCaptionFittingKnows(t *testing.T) {
	var shortsLangs []string
	for _, l := range shorts.SupportedLanguages {
		shortsLangs = append(shortsLangs, l.Code)
	}
	sort.Strings(shortsLangs)
	if want := []string{"auto", "en-IN", "en-US", "hi-IN"}; !reflect.DeepEqual(shortsLangs, want) {
		t.Errorf("AI Shorts transcription languages changed to %v (were %v): captions in a new language need right-to-left ordering, shaping and a font first; see the comment on this test", shortsLangs, want)
	}
	if want := []string{"en", "hi"}; !reflect.DeepEqual(storyjob.Languages(), want) {
		t.Errorf("story video languages changed to %v (were %v): captions in a new language need right-to-left ordering, shaping and a font first; see the comment on this test", storyjob.Languages(), want)
	}
}
