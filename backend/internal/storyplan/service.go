package storyplan

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/aws/smithy-go"
	"github.com/rahul/indifferent/backend/internal/shorts/rank"
)

// DefaultTimeout bounds one model call. It sits under the 29 second limit of
// the API Gateway integration (and the API Lambda's 30 second timeout) so the
// caller gets a clean, retryable error instead of a gateway timeout.
const DefaultTimeout = 25 * time.Second

// Completer sends one prompt to a model and returns its raw text answer.
// *rank.Service implements it, which brings the Kimi, Nova Pro and Claude
// adapters along.
type Completer interface {
	Complete(ctx context.Context, prompt string, opts rank.CompleteOptions) (string, error)
}

// Service generates plans from stories.
type Service struct {
	completer Completer
	model     string
	timeout   time.Duration
	logf      func(format string, args ...any)
	now       func() time.Time
}

// Option customizes a Service.
type Option func(*Service)

// WithTimeout replaces DefaultTimeout.
func WithTimeout(d time.Duration) Option { return func(s *Service) { s.timeout = d } }

// WithLogger replaces log.Printf as the destination for service logs.
func WithLogger(f func(format string, args ...any)) Option { return func(s *Service) { s.logf = f } }

// WithClock replaces time.Now, for tests.
func WithClock(now func() time.Time) Option { return func(s *Service) { s.now = now } }

// NewService returns a Service. model names the model for logs and for the
// record stored with each plan; it is not interpreted.
func NewService(c Completer, model string, opts ...Option) *Service {
	s := &Service{completer: c, model: model, timeout: DefaultTimeout, logf: log.Printf, now: time.Now}
	for _, o := range opts {
		o(s)
	}
	return s
}

// Model returns the model name given to NewService.
func (s *Service) Model() string { return s.model }

// Result is a generated plan with facts about how it was made.
type Result struct {
	Plan Plan
	// Script is the dominant script of the story.
	Script Script
	// StoryRunes is the length of the normalized story, in characters.
	StoryRunes int
	// Latency is how long the model call took.
	Latency time.Duration
}

// Generate plans a video from story.
//
// The story is validated first (ErrInvalidStory, no model call). The model call
// has its own deadline, so a slow model returns ErrModelTimeout within
// the configured timeout whatever deadline the caller's context has. An answer
// that cannot be made into a valid plan is ErrInvalidPlan. Neither the story
// nor the model's answer is logged: only lengths, counts, the model name and
// the outcome.
func (s *Service) Generate(ctx context.Context, story string) (Result, error) {
	story, err := NormalizeStory(story)
	if err != nil {
		return Result{}, err
	}
	script := DominantScript(story)
	runes := len([]rune(story))
	prompt := BuildPrompt(story, script)

	callCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	start := s.now()
	text, err := s.completer.Complete(callCtx, prompt, rank.CompleteOptions{MaxTokens: MaxOutputTokens})
	latency := s.now().Sub(start)

	if err != nil {
		// The caller's own cancellation is not the model's fault.
		if ctxErr := ctx.Err(); ctxErr != nil {
			return Result{}, ctxErr
		}
		kind := classifyModelError(callCtx, err)
		s.logf("storyplan_failed model=%s storyRunes=%d script=%s latencyMs=%d outcome=%s", s.model, runes, script, latency.Milliseconds(), kind.code)
		return Result{}, kind.err
	}

	rp, err := parseRawPlan(text, rank.ExtractJSON)
	if err == nil {
		var plan Plan
		plan, err = validate(rp, script)
		if err == nil {
			s.logf("storyplan_ok model=%s storyRunes=%d script=%s latencyMs=%d scenes=%d totalSeconds=%.1f warnings=%d",
				s.model, runes, script, latency.Milliseconds(), len(plan.Scenes), plan.TotalSeconds, len(plan.Warnings))
			return Result{Plan: plan, Script: script, StoryRunes: runes, Latency: latency}, nil
		}
	}
	s.logf("storyplan_failed model=%s storyRunes=%d script=%s latencyMs=%d outcome=invalid_plan answerRunes=%d",
		s.model, runes, script, latency.Milliseconds(), len([]rune(text)))
	return Result{}, err
}

type modelFailure struct {
	code string
	err  error
}

// classifyModelError maps a failed model call to a stable code and a typed
// error. AWS error text can contain request ids and account details, so it is
// not wrapped into the returned error.
func classifyModelError(callCtx context.Context, err error) modelFailure {
	if errors.Is(err, context.DeadlineExceeded) || callCtx.Err() == context.DeadlineExceeded {
		return modelFailure{"timeout", fmt.Errorf("%w: %w", ErrModelTimeout, ErrModelFailed)}
	}
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		code := apiErr.ErrorCode()
		switch code {
		case "ThrottlingException", "TooManyRequestsException", "ServiceQuotaExceededException":
			return modelFailure{"throttled", fmt.Errorf("%w: the model is busy", ErrModelFailed)}
		case "AccessDeniedException":
			return modelFailure{"access_denied", fmt.Errorf("%w: access to the model was denied", ErrModelFailed)}
		case "ModelTimeoutException":
			return modelFailure{"timeout", fmt.Errorf("%w: %w", ErrModelTimeout, ErrModelFailed)}
		case "ValidationException", "ResourceNotFoundException":
			return modelFailure{"config", fmt.Errorf("%w: the model is not configured correctly", ErrModelFailed)}
		}
		return modelFailure{"api_error", fmt.Errorf("%w: the model service returned an error", ErrModelFailed)}
	}
	return modelFailure{"error", fmt.Errorf("%w: the model could not be reached", ErrModelFailed)}
}
