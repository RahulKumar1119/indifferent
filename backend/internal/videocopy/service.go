package videocopy

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/aws/smithy-go"
	"github.com/rahul/indifferent/backend/internal/shorts/rank"
)

// DefaultTimeout bounds one call, under the API Gateway limit of 29 seconds.
const DefaultTimeout = 25 * time.Second

// DefaultPrimaryBudget is how much of the timeout the first model gets when a
// fallback is set.
const DefaultPrimaryBudget = 15 * time.Second

const minFallbackBudget = 3 * time.Second

// Completer sends one prompt to a model and returns its raw text answer.
// *rank.Service implements it.
type Completer interface {
	Complete(ctx context.Context, prompt string, opts rank.CompleteOptions) (string, error)
}

// Service writes titles, a description and chapters.
type Service struct {
	completer     Completer
	model         string
	timeout       time.Duration
	logf          func(format string, args ...any)
	now           func() time.Time
	fallback      Completer
	fallbackModel string
	primaryBudget time.Duration
}

// Option customizes a Service.
type Option func(*Service)

func WithTimeout(d time.Duration) Option       { return func(s *Service) { s.timeout = d } }
func WithLogger(f func(string, ...any)) Option { return func(s *Service) { s.logf = f } }
func WithClock(now func() time.Time) Option    { return func(s *Service) { s.now = now } }
func WithPrimaryBudget(d time.Duration) Option { return func(s *Service) { s.primaryBudget = d } }
func WithFallback(c Completer, model string) Option {
	return func(s *Service) { s.fallback, s.fallbackModel = c, model }
}

// NewService returns a Service. model names the model for logs; it is not interpreted.
func NewService(c Completer, model string, opts ...Option) *Service {
	s := &Service{completer: c, model: model, timeout: DefaultTimeout, logf: log.Printf, now: time.Now, primaryBudget: DefaultPrimaryBudget}
	for _, o := range opts {
		o(s)
	}
	return s
}

// Model returns the model name given to NewService.
func (s *Service) Model() string { return s.model }

// Result is the checked copy with facts about how it was made.
type Result struct {
	Copy
	// HasTimestamps says the source had real times, so chapters were possible.
	HasTimestamps bool
	Model         string
	Latency       time.Duration
}

// Generate writes the copy for r. The request is validated first
// (ErrInvalidInput, no model call). A model that times out, fails or answers
// with unusable text is followed by one call to the fallback, inside the same
// timeout. Neither the source nor the answer is logged: only sizes, counts, the
// model name and the outcome.
func (s *Service) Generate(ctx context.Context, r Request) (Result, error) {
	src, err := normalize(r)
	if err != nil {
		return Result{}, err
	}
	prompt := buildPrompt(src)
	callCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	start := s.now()
	budget := s.timeout
	if s.fallback != nil {
		budget = s.primaryBudget
	}
	c, err := s.try(ctx, callCtx, s.completer, s.model, "primary", budget, prompt, src)
	model := s.model
	if err != nil {
		if s.fallback == nil || ctx.Err() != nil {
			return Result{}, err
		}
		remaining := s.timeout - s.now().Sub(start)
		floor := minFallbackBudget
		if q := s.timeout / 4; q < floor {
			floor = q
		}
		if remaining < floor {
			s.logf("videocopy_fallback_skipped model=%s remainingMs=%d", s.fallbackModel, remaining.Milliseconds())
			return Result{}, err
		}
		c, err = s.try(ctx, callCtx, s.fallback, s.fallbackModel, "fallback", remaining, prompt, src)
		if err != nil {
			return Result{}, err
		}
		model = s.fallbackModel
	}
	return Result{Copy: c, HasTimestamps: src.timed, Model: model, Latency: s.now().Sub(start)}, nil
}

func (s *Service) try(ctx, callCtx context.Context, c Completer, model, name string, budget time.Duration, prompt string, src *source) (Copy, error) {
	tryCtx, cancel := context.WithTimeout(callCtx, budget)
	defer cancel()
	start := s.now()
	text, err := c.Complete(tryCtx, prompt, rank.CompleteOptions{MaxTokens: MaxOutputTokens})
	latency := s.now().Sub(start)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return Copy{}, ctxErr
		}
		code, typed := classify(tryCtx, err)
		s.logf("videocopy_failed model=%s attempt=%s sourceRunes=%d latencyMs=%d outcome=%s", model, name, src.runes, latency.Milliseconds(), code)
		return Copy{}, typed
	}
	out, err := parseAndCheck(text, src, rank.ExtractJSON)
	if err != nil {
		s.logf("videocopy_failed model=%s attempt=%s sourceRunes=%d latencyMs=%d outcome=invalid_answer answerRunes=%d", model, name, src.runes, latency.Milliseconds(), len([]rune(text)))
		return Copy{}, err
	}
	s.logf("videocopy_ok model=%s attempt=%s sourceRunes=%d latencyMs=%d titles=%d chapters=%d", model, name, src.runes, latency.Milliseconds(), len(out.Titles), len(out.Chapters))
	return out, nil
}

// classify maps a failed model call to a stable code and a typed error. AWS
// error text can carry request ids and account details, so it is not wrapped.
func classify(callCtx context.Context, err error) (string, error) {
	if errors.Is(err, context.DeadlineExceeded) || callCtx.Err() == context.DeadlineExceeded {
		return "timeout", fmt.Errorf("%w: %w", ErrModelTimeout, ErrModelFailed)
	}
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "ThrottlingException", "TooManyRequestsException", "ServiceQuotaExceededException":
			return "throttled", fmt.Errorf("%w: the model is busy", ErrModelFailed)
		case "AccessDeniedException":
			return "access_denied", fmt.Errorf("%w: access to the model was denied", ErrModelFailed)
		case "ModelTimeoutException":
			return "timeout", fmt.Errorf("%w: %w", ErrModelTimeout, ErrModelFailed)
		case "ValidationException", "ResourceNotFoundException":
			return "config", fmt.Errorf("%w: the model is not configured correctly", ErrModelFailed)
		}
		return "api_error", fmt.Errorf("%w: the model service returned an error", ErrModelFailed)
	}
	return "error", fmt.Errorf("%w: the model could not be reached", ErrModelFailed)
}
