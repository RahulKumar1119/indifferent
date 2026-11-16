// Package videocopy writes the words that go with a finished video: ten title
// ideas, a description and, when the source has real timestamps, chapters (#331).
//
// The model's answer is untrusted text. It is parsed and checked here before
// anything is shown: titles and the description are cleaned of markup and held
// to YouTube's limits, and chapter times are never taken from the model. It names
// a block of the transcript and the server supplies that block's real start time.
package videocopy

import (
	"errors"
	"fmt"
)

var (
	// ErrInvalidInput means the request was rejected before any model call. The
	// message is safe to show to the user.
	ErrInvalidInput = errors.New("invalid input")
	// ErrModelFailed means the model call failed (throttled, access denied,
	// service error). Retrying later may work.
	ErrModelFailed = errors.New("the writing model failed")
	// ErrModelTimeout means the model did not answer in time. It wraps with
	// ErrModelFailed.
	ErrModelTimeout = errors.New("the writing model took too long")
	// ErrInvalidAnswer means the model answered but too little of it was usable.
	// Asking again often works.
	ErrInvalidAnswer = errors.New("the model did not produce usable text")
)

type inputError struct{ reason string }

func (e *inputError) Error() string        { return "invalid input: " + e.reason }
func (e *inputError) Is(target error) bool { return target == ErrInvalidInput }

func invalidInput(format string, args ...any) error {
	return &inputError{reason: fmt.Sprintf(format, args...)}
}

// InputReason extracts the user-safe reason from an ErrInvalidInput, or "".
func InputReason(err error) string {
	var ie *inputError
	if errors.As(err, &ie) {
		return ie.reason
	}
	return ""
}
