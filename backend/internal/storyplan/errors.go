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
	// ErrInvalidEdit means a user's edit to a plan was rejected. The message is
	// safe to show to the user.
	ErrInvalidEdit = errors.New("invalid edit")
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

// Reasons a model's answer could not be made into a plan (#299). They are
// logged and counted, so they come from this fixed list and never carry story
// or model text.
const (
	ReasonNoJSON   = "no_json"   // no JSON object in the answer
	ReasonBadShape = "bad_shape" // JSON, but not a plan
	ReasonNoScenes = "no_scenes" // a plan with an empty scene list
	ReasonTooFew   = "too_few"   // too few scenes survived validation
	ReasonTooShort = "too_short" // the scenes left are under the minimum length
	ReasonOther    = "other"     // anything not above
)

// Why a scene was dropped. With ReasonTooFew, the most common of these is the
// cause worth fixing, so they are counted per plan.
const (
	DropNotObject   = "not_object"
	DropNoNarration = "no_narration"
	DropWrongScript = "wrong_language"
	DropNoKeywords  = "no_keywords"
	DropTooLong     = "narration_too_long"
)

// PlanError is an ErrInvalidPlan with its reason and, when scenes were
// dropped, how many for each drop reason. errors.Is(err, ErrInvalidPlan) holds.
type PlanError struct {
	Reason string
	Drops  map[string]int
	msg    string
}

func (e *PlanError) Error() string { return ErrInvalidPlan.Error() + ": " + e.msg }
func (e *PlanError) Unwrap() error { return ErrInvalidPlan }

// invalidPlan returns a PlanError with reason and a message that names what
// failed and never contains story or model text.
func invalidPlan(reason string, drops map[string]int, format string, args ...any) error {
	return &PlanError{Reason: reason, Drops: drops, msg: fmt.Sprintf(format, args...)}
}

// PlanFailure returns the reason and the scene drops of an invalid plan, or
// ReasonOther when err is not a PlanError.
func PlanFailure(err error) (string, map[string]int) {
	var pe *PlanError
	if errors.As(err, &pe) {
		return pe.Reason, pe.Drops
	}
	return ReasonOther, nil
}
