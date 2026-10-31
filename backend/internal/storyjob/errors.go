package storyjob

import (
	"context"
	"errors"
	"fmt"

	"github.com/rahul/indifferent/backend/internal/stockmedia"
)

// Failure codes. They are stored on the job and read by the UI.
const (
	// CodeMediaUnavailable: the provider no longer has the chosen picture.
	// Nothing to retry; the user chooses another.
	CodeMediaUnavailable = "MEDIA_UNAVAILABLE"
	// CodeMediaInvalid: the picture is there but cannot be used.
	CodeMediaInvalid = "MEDIA_INVALID"
	// CodeMediaTemporary: the provider did not answer. Trying again can work.
	CodeMediaTemporary = "MEDIA_TEMPORARY"
	// CodeNarrationFailed: the voice could not be made.
	CodeNarrationFailed = "NARRATION_FAILED"
	// CodeNarrationTooLong: spoken, the story runs over the length limit.
	CodeNarrationTooLong = "NARRATION_TOO_LONG"
	// CodeInternal: something on our side broke.
	CodeInternal = "INTERNAL"
)

// StepError is a failure of one step of a job.
type StepError struct {
	// Scene is the scene the failure is about, from 1, or 0 for the whole job.
	Scene int
	Code  string
	// Message is fit to show the user: it says what happened and what to do.
	Message string
	// Retryable means running the step again may succeed. When false, the user
	// has to change something first.
	Retryable bool
	Err       error
}

func (e *StepError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *StepError) Unwrap() error { return e.Err }

// mediaError turns a failure fetching a scene's picture into a StepError.
// Errors that carry only the user's concern reach the message; the cause is
// kept for logs. Cancellation is not a failure of the step and is returned as is.
func mediaError(scene int, provider string, err error) error {
	name := providerName(provider)
	switch {
	case errors.Is(err, context.Canceled):
		return err
	case errors.Is(err, stockmedia.ErrNotFound),
		errors.Is(err, stockmedia.ErrAssetUnavailable) && !errors.Is(err, stockmedia.ErrUnavailable):
		return &StepError{Scene: scene, Code: CodeMediaUnavailable, Err: err,
			Message: fmt.Sprintf("Scene %d: the picture is no longer available from %s. Choose another and make the video again.", scene, name)}
	case errors.Is(err, stockmedia.ErrInvalidAsset):
		return &StepError{Scene: scene, Code: CodeMediaInvalid, Err: err,
			Message: fmt.Sprintf("Scene %d: the chosen picture can't be used. Choose another and make the video again.", scene)}
	case errors.Is(err, stockmedia.ErrRateLimited), errors.Is(err, stockmedia.ErrUnavailable),
		errors.Is(err, stockmedia.ErrCoolingDown), errors.Is(err, context.DeadlineExceeded):
		return &StepError{Scene: scene, Code: CodeMediaTemporary, Retryable: true, Err: err,
			Message: fmt.Sprintf("Scene %d: %s did not answer. Try again in a few minutes.", scene, name)}
	}
	return internalError(scene, err)
}

// internalError is a failure that is ours to fix. It is retried, since most are
// transient (storage, network), and the user is not blamed.
func internalError(scene int, err error) error {
	if errors.Is(err, context.Canceled) {
		return err
	}
	msg := "Something went wrong on our side while making the video. Try again."
	if scene > 0 {
		msg = fmt.Sprintf("Scene %d: something went wrong on our side. Try again.", scene)
	}
	return &StepError{Scene: scene, Code: CodeInternal, Retryable: true, Err: err, Message: msg}
}

func providerName(p string) string {
	switch p {
	case "pexels":
		return "Pexels"
	case "pixabay":
		return "Pixabay"
	}
	return "the photo library"
}

// IsRetryable reports whether err is a StepError that a retry may fix. Any
// other error is treated as retryable too: an unexpected failure is more
// likely ours than the user's.
func IsRetryable(err error) bool {
	var se *StepError
	if errors.As(err, &se) {
		return se.Retryable
	}
	return err != nil
}
