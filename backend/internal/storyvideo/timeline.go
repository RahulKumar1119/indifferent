// Package storyvideo turns a confirmed story plan into a vertical video: it
// lays the scenes out on a timeline from the real narration audio, builds
// word-timed captions from the speech marks of the same narration, and builds
// the FFmpeg command that renders it.
//
// Everything here is pure: no network, no disk and no process is touched, so
// all of it is testable. The Fargate worker (cmd/storyrender) downloads the
// media, calls these functions and runs the command.
//
// Why the narration sets the timeline: the plan's scene durations are
// estimates at 2.5 words a second. Measured against Amazon Polly (English,
// Joanna), a 16-word scene is spoken in 4.4 seconds, not the planned 6.4, and
// Hindi (Aditi) runs about 2.25 words a second. Only the audio itself says how
// long a scene lasts, so the plan's durations are ignored at render time.
package storyvideo

import (
	"fmt"

	"github.com/rahul/indifferent/backend/internal/storyplan"
)

// Output format. It matches the AI Shorts renderer, so a story video can be
// uploaded wherever a Short can.
const (
	Width  = 1080
	Height = 1920
	FPS    = 30
)

// Timing, in seconds.
const (
	// TransitionSeconds is the cross-fade between two scenes. The next picture
	// starts to appear as its narration begins.
	TransitionSeconds = 0.4
	// PauseSeconds is the silence after each scene's narration, so the voice
	// does not run into the next scene's.
	PauseSeconds = 0.35
	// MinSceneSeconds keeps a one-word scene from flashing by.
	MinSceneSeconds = 2.0
	// MaxTotalSeconds is the longest video. It is the plan's limit, which is
	// the longest AI Shorts clip, so every video fits the same platforms.
	MaxTotalSeconds = storyplan.MaxTotalSeconds
)

// Slot is where one scene sits on the timeline.
type Slot struct {
	// Start is when the scene's narration begins, and when its picture starts
	// to cross-fade in.
	Start float64
	// Span is how long the scene owns the audio: its narration plus the pause,
	// or MinSceneSeconds if that is longer. The next scene starts at Start+Span.
	Span float64
	// Show is how long the scene's picture must exist: Span, plus the
	// cross-fade into the next scene (or the closing hold, for the last).
	Show float64
}

// Timeline is the whole video's layout.
type Timeline struct {
	Slots []Slot
	// Total is the video's length: every span, plus one closing cross-fade
	// length that the last picture is held for.
	Total float64
}

// TooLongError means the narration, once spoken, does not fit in
// MaxTotalSeconds. The plan was within the limit by estimate, but speech can
// run longer than the estimate (Hindi does), so this is found here.
type TooLongError struct {
	Seconds float64
	Max     float64
}

func (e *TooLongError) Error() string {
	return fmt.Sprintf("the narration runs %.0f seconds, over the %.0f second limit", e.Seconds, e.Max)
}

// UserMessage says what to do about it, in words fit to show the user.
func (e *TooLongError) UserMessage() string {
	return fmt.Sprintf("The narration runs %.0f seconds, over the %.0f second limit. Shorten the narration in the plan and try again.", e.Seconds, e.Max)
}

// BuildTimeline lays scenes out one after another from the lengths of their
// narration audio, in seconds, in plan order.
func BuildTimeline(narration []float64) (Timeline, error) {
	if len(narration) == 0 {
		return Timeline{}, fmt.Errorf("a video needs at least one scene")
	}
	tl := Timeline{Slots: make([]Slot, len(narration))}
	at := 0.0
	for i, n := range narration {
		if n != n || n <= 0 { // NaN or no audio at all
			return Timeline{}, fmt.Errorf("scene %d has no narration audio", i+1)
		}
		span := n + PauseSeconds
		if span < MinSceneSeconds {
			span = MinSceneSeconds
		}
		tl.Slots[i] = Slot{Start: at, Span: span, Show: span + TransitionSeconds}
		at += span
	}
	tl.Total = at + TransitionSeconds
	// A hair of tolerance: 90.0 plus float noise is still 90.
	if tl.Total > MaxTotalSeconds+0.05 {
		return Timeline{}, &TooLongError{Seconds: tl.Total, Max: MaxTotalSeconds}
	}
	return tl, nil
}
