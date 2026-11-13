package storyplan

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
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

// DefaultPrimaryBudget is how much of the timeout the first model gets when a
// fallback is set. The rest is the fallback's. Measured over 21 plans each
// (#293), the slowest Qwen3 plan took 8.6s and the slowest DeepSeek one 9.1s,
// so both fit their share with room to spare.
const DefaultPrimaryBudget = 15 * time.Second

// minFallbackBudget is the least time worth giving the fallback. With less
// left, it would only add a second failure.
const minFallbackBudget = 3 * time.Second

// Service generates plans from stories.
type Service struct {
	completer Completer
	model     string
	timeout   time.Duration
	logf      func(format string, args ...any)
	now       func() time.Time

	// fallback, when set, is asked when the first model fails for a reason
	// of the model's: a timeout, a service error or an unusable answer.
	fallback      Completer
	fallbackModel string
	primaryBudget time.Duration
}

// Option customizes a Service.
type Option func(*Service)

// WithTimeout replaces DefaultTimeout.
func WithTimeout(d time.Duration) Option { return func(s *Service) { s.timeout = d } }

// WithLogger replaces log.Printf as the destination for service logs.
func WithLogger(f func(format string, args ...any)) Option { return func(s *Service) { s.logf = f } }

// WithClock replaces time.Now, for tests.
func WithClock(now func() time.Time) Option { return func(s *Service) { s.now = now } }

// WithFallback sets a second model, asked when the first fails for a reason of
// the model's (timeout, service error, unusable plan). The first model gets
// DefaultPrimaryBudget of the timeout and the fallback the rest, so the whole
// call still ends within the timeout.
func WithFallback(c Completer, model string) Option {
	return func(s *Service) { s.fallback, s.fallbackModel = c, model }
}

// WithPrimaryBudget replaces DefaultPrimaryBudget.
func WithPrimaryBudget(d time.Duration) Option { return func(s *Service) { s.primaryBudget = d } }

// NewService returns a Service. model names the model for logs and for the
// record stored with each plan; it is not interpreted.
func NewService(c Completer, model string, opts ...Option) *Service {
	s := &Service{completer: c, model: model, timeout: DefaultTimeout, logf: log.Printf, now: time.Now, primaryBudget: DefaultPrimaryBudget}
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
	// Latency is how long planning took, both attempts included when the
	// fallback answered.
	Latency time.Duration
	// Model is the model that produced the plan: the first one, or the
	// fallback. Empty when no plan was made.
	Model string
}

// Generate plans a video from story.
//
// The story is validated first (ErrInvalidStory, no model call). The model call
// has its own deadline, so a slow model returns ErrModelTimeout within
// the configured timeout whatever deadline the caller's context has. An answer
// that cannot be made into a valid plan is ErrInvalidPlan.
//
// With a fallback, a primary that times out, fails or answers with an unusable
// plan is followed by one call to the fallback, inside the same timeout. When
// both fail, the fallback's error is returned. An invalid story and the
// caller's own cancellation are never retried.
//
// Neither the story nor the model's answer is logged: only lengths, counts,
// the model name, the attempt and the outcome.
func (s *Service) Generate(ctx context.Context, story string) (Result, error) {
	story, err := NormalizeStory(story)
	if err != nil {
		return Result{}, err
	}
	script := DominantScript(story)
	runes := len([]rune(story))
	prompt := BuildPrompt(story, script)

	// One deadline for the whole call, whichever models answer it.
	callCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	start := s.now()

	a := attempt{name: "primary", completer: s.completer, model: s.model, budget: s.timeout}
	if s.fallback != nil {
		a.budget = s.primaryBudget
	}
	plan, err := s.try(ctx, callCtx, a, prompt, script, runes)
	if err == nil {
		return Result{Plan: plan, Script: script, StoryRunes: runes, Latency: s.now().Sub(start), Model: s.model}, nil
	}
	if s.fallback == nil || ctx.Err() != nil {
		return Result{}, err
	}
	// Only the time left of the whole call: the fallback can never push the
	// call past the timeout the API is built around.
	// callCtx enforces the deadline either way; this decides whether trying is
	// worth it.
	remaining := s.timeout - s.now().Sub(start)
	floor := minFallbackBudget
	if q := s.timeout / 4; q < floor {
		floor = q // a short timeout (tests) scales the floor down with it
	}
	if remaining < floor {
		s.logf("storyplan_fallback_skipped model=%s remainingMs=%d", s.fallbackModel, remaining.Milliseconds())
		return Result{}, err
	}
	plan, err = s.try(ctx, callCtx, attempt{name: "fallback", completer: s.fallback, model: s.fallbackModel, budget: remaining}, prompt, script, runes)
	if err != nil {
		return Result{}, err
	}
	return Result{Plan: plan, Script: script, StoryRunes: runes, Latency: s.now().Sub(start), Model: s.fallbackModel}, nil
}

// attempt is one model's go at a story.
type attempt struct {
	name      string // primary or fallback, for the logs
	completer Completer
	model     string
	budget    time.Duration
}

// try asks one model for a plan within its budget, inside the call's own
// deadline, and logs the outcome. The returned error is one of the typed
// errors Generate documents, or the caller's own context error.
func (s *Service) try(ctx, callCtx context.Context, a attempt, prompt string, script Script, runes int) (Plan, error) {
	tryCtx, cancel := context.WithTimeout(callCtx, a.budget)
	defer cancel()
	start := s.now()
	text, err := a.completer.Complete(tryCtx, prompt, rank.CompleteOptions{MaxTokens: MaxOutputTokens})
	latency := s.now().Sub(start)

	if err != nil {
		// The caller's own cancellation is not the model's fault.
		if ctxErr := ctx.Err(); ctxErr != nil {
			return Plan{}, ctxErr
		}
		kind := classifyModelError(tryCtx, err)
		s.logf("storyplan_failed model=%s attempt=%s storyRunes=%d script=%s latencyMs=%d outcome=%s", a.model, a.name, runes, script, latency.Milliseconds(), kind.code)
		return Plan{}, kind.err
	}

	rp, err := parseRawPlan(text, rank.ExtractJSON)
	if err == nil {
		var plan Plan
		plan, err = validate(rp, script)
		if err == nil {
			s.logf("storyplan_ok model=%s attempt=%s storyRunes=%d script=%s latencyMs=%d scenes=%d totalSeconds=%.1f warnings=%d",
				a.model, a.name, runes, script, latency.Milliseconds(), len(plan.Scenes), plan.TotalSeconds, len(plan.Warnings))
			return plan, nil
		}
	}
	// Which check failed, and why scenes were dropped (#299): a fixed list of
	// codes and counts, never story or answer text.
	reason, drops := PlanFailure(err)
	s.logf("storyplan_failed model=%s attempt=%s storyRunes=%d script=%s latencyMs=%d outcome=invalid_plan reason=%s drops=%s answerRunes=%d",
		a.model, a.name, runes, script, latency.Milliseconds(), reason, formatDrops(drops), len([]rune(text)))
	return Plan{}, err
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

// formatDrops writes scene-drop counts as "code:n,code:n" in a fixed order,
// or "-" when none were dropped.
func formatDrops(drops map[string]int) string {
	var parts []string
	for _, k := range []string{DropTooLong, DropWrongScript, DropNoKeywords, DropNoNarration, DropNotObject} {
		if n := drops[k]; n > 0 {
			parts = append(parts, fmt.Sprintf("%s:%d", k, n))
		}
	}
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, ",")
}
