package storyplan

import (
	"errors"
	"fmt"
)

// Sentinel errors. Use errors.Is.
var (
	// ErrInvalidStory means the story was rejected before any model call. The
	// message is safe to show to the user.
	ErrInvalidStory = errors.New("invalid story")
	// ErrModelFailed means the model call failed (throttled, access denied,
	// service error). It says nothing about the story, and retrying later may
	// work.
	ErrModelFailed = errors.New("the planning model failed")
	// ErrModelTimeout means the model did not answer in time. It wraps with
	// ErrModelFailed.
	ErrModelTimeout = errors.New("the planning model took too long")
	// ErrInvalidPlan means the model answered but nothing usable could be made
	// of it: not JSON, no valid scenes, or a plan outside the limits that
	// repair could not fix. Asking again often works, since the model's answer
	// varies from run to run.
	ErrInvalidPlan = errors.New("the model did not produce a usable plan")
)

// storyError is an ErrInvalidStory with a user-safe reason.
type storyError struct{ reason string }

func (e *storyError) Error() string        { return "invalid story: " + e.reason }
func (e *storyError) Is(target error) bool { return target == ErrInvalidStory }

// Reason returns the user-safe explanation for a rejected story.
func (e *storyError) Reason() string { return e.reason }

func invalidStory(format string, args ...any) error {
	return &storyError{reason: fmt.Sprintf(format, args...)}
}

// StoryReason extracts the user-safe reason from an ErrInvalidStory, or "".
func StoryReason(err error) string {
	var se *storyError
	if errors.As(err, &se) {
		return se.reason
	}
	return ""
}

// invalidPlan wraps ErrInvalidPlan with a reason that names what failed and
// never contains story or model text.
func invalidPlan(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidPlan, fmt.Sprintf(format, args...))
}
