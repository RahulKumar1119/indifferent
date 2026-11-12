package storyplan

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/smithy-go"
)

// Issue #293: when the first model fails for a reason of the model's, a second
// one is asked, inside the same timeout.

func fallbackService(t *testing.T, primary, fallback Completer, opts ...Option) (*Service, *[]string) {
	t.Helper()
	var mu sync.Mutex
	var logs []string
	base := []Option{
		WithFallback(fallback, "fallback-model"),
		WithLogger(func(f string, a ...any) { mu.Lock(); logs = append(logs, fmt.Sprintf(f, a...)); mu.Unlock() }),
	}
	return NewService(primary, "primary-model", append(base, opts...)...), &logs
}

func blockUntilDone(ctx context.Context) error { <-ctx.Done(); return ctx.Err() }

func TestFallback_AnswersWhenThePrimaryFails(t *testing.T) {
	good := func() *stubModel { return &stubModel{answer: mustJSON(t, englishPlan(6))} }
	for name, primary := range map[string]*stubModel{
		"timeout":       {delay: blockUntilDone},
		"throttled":     {err: &smithy.GenericAPIError{Code: "ThrottlingException", Message: "slow down"}},
		"service error": {err: errors.New("connection reset")},
		"unusable plan": {answer: "I'm sorry, I can't help with that."},
		"empty answer":  {answer: ""},
	} {
		t.Run(name, func(t *testing.T) {
			fb := good()
			svc, logs := fallbackService(t, primary, fb, WithTimeout(400*time.Millisecond), WithPrimaryBudget(100*time.Millisecond))
			res, err := svc.Generate(context.Background(), englishStory)
			if err != nil {
				t.Fatalf("err = %v, want the fallback's plan", err)
			}
			if res.Model != "fallback-model" {
				t.Errorf("Model = %q, want the fallback", res.Model)
			}
			if primary.calls() != 1 || fb.calls() != 1 {
				t.Errorf("calls primary=%d fallback=%d, want 1 and 1", primary.calls(), fb.calls())
			}
			joined := strings.Join(*logs, "\n")
			for _, want := range []string{"storyplan_failed model=primary-model attempt=primary", "storyplan_ok model=fallback-model attempt=fallback"} {
				if !strings.Contains(joined, want) {
					t.Errorf("logs missing %q:\n%s", want, joined)
				}
			}
		})
	}
}

func TestFallback_NotAskedWhenThePrimaryAnswers(t *testing.T) {
	primary, fb := &stubModel{answer: mustJSON(t, englishPlan(6))}, &stubModel{answer: mustJSON(t, englishPlan(6))}
	svc, logs := fallbackService(t, primary, fb)
	res, err := svc.Generate(context.Background(), englishStory)
	if err != nil || res.Model != "primary-model" || fb.calls() != 0 {
		t.Fatalf("err=%v model=%q fallback calls=%d", err, res.Model, fb.calls())
	}
	if !strings.Contains(strings.Join(*logs, "\n"), "storyplan_ok model=primary-model attempt=primary") {
		t.Errorf("logs: %v", *logs)
	}
}

func TestFallback_NeverForAnInvalidStoryOrTheCallersCancel(t *testing.T) {
	t.Run("invalid story", func(t *testing.T) {
		primary, fb := &stubModel{}, &stubModel{answer: mustJSON(t, englishPlan(6))}
		svc, _ := fallbackService(t, primary, fb)
		if _, err := svc.Generate(context.Background(), "hi"); !errors.Is(err, ErrInvalidStory) {
			t.Fatalf("err = %v", err)
		}
		if primary.calls()+fb.calls() != 0 {
			t.Error("a model was called for an invalid story")
		}
	})
	t.Run("the caller cancels", func(t *testing.T) {
		primary, fb := &stubModel{delay: blockUntilDone}, &stubModel{answer: mustJSON(t, englishPlan(6))}
		svc, _ := fallbackService(t, primary, fb)
		ctx, cancel := context.WithCancel(context.Background())
		go func() { time.Sleep(20 * time.Millisecond); cancel() }()
		if _, err := svc.Generate(ctx, englishStory); !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v, want the caller's cancel", err)
		}
		if fb.calls() != 0 {
			t.Error("the fallback was asked after the user left")
		}
	})
}

func TestFallback_BothFailReturnsTheFallbacksError(t *testing.T) {
	primary := &stubModel{delay: blockUntilDone}
	fb := &stubModel{answer: "not a plan"}
	svc, _ := fallbackService(t, primary, fb, WithTimeout(300*time.Millisecond), WithPrimaryBudget(50*time.Millisecond))
	_, err := svc.Generate(context.Background(), englishStory)
	if !errors.Is(err, ErrInvalidPlan) {
		t.Errorf("err = %v, want the fallback's ErrInvalidPlan", err)
	}
}

// The whole call, both models included, ends within the timeout: the API is
// built around it, and the gateway gives up at 29 seconds.
func TestFallback_TheWholeCallStaysInsideTheTimeout(t *testing.T) {
	primary, fb := &stubModel{delay: blockUntilDone}, &stubModel{delay: blockUntilDone}
	svc, _ := fallbackService(t, primary, fb, WithTimeout(200*time.Millisecond), WithPrimaryBudget(80*time.Millisecond))
	start := time.Now()
	_, err := svc.Generate(context.Background(), englishStory)
	took := time.Since(start)
	if !errors.Is(err, ErrModelTimeout) {
		t.Errorf("err = %v, want ErrModelTimeout", err)
	}
	if took > 300*time.Millisecond {
		t.Errorf("took %v, longer than the 200ms timeout", took)
	}
	if took < 150*time.Millisecond {
		t.Errorf("took %v: the fallback did not get the rest of the time", took)
	}
	if primary.calls() != 1 || fb.calls() != 1 {
		t.Errorf("calls primary=%d fallback=%d", primary.calls(), fb.calls())
	}
}

func TestFallback_SkippedWhenTooLittleTimeIsLeft(t *testing.T) {
	// The primary used almost all of the timeout before failing.
	base := time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
	now := base
	clock := func() time.Time { return now }
	primary := &stubModel{delay: func(context.Context) error { now = now.Add(23 * time.Second); return errors.New("boom") }}
	fb := &stubModel{answer: mustJSON(t, englishPlan(6))}
	svc, logs := fallbackService(t, primary, fb, WithClock(clock))
	_, err := svc.Generate(context.Background(), englishStory)
	if !errors.Is(err, ErrModelFailed) || fb.calls() != 0 {
		t.Errorf("err=%v fallback calls=%d: with 2s left the fallback must not run", err, fb.calls())
	}
	if !strings.Contains(strings.Join(*logs, "\n"), "storyplan_fallback_skipped model=fallback-model") {
		t.Errorf("logs: %v", *logs)
	}
}

func TestFallback_DefaultBudgets(t *testing.T) {
	if DefaultPrimaryBudget+10*time.Second > DefaultTimeout {
		t.Errorf("the primary's %v leaves the fallback less than 10s of %v", DefaultPrimaryBudget, DefaultTimeout)
	}
	svc := NewService(&stubModel{}, "m")
	if svc.fallback != nil || svc.primaryBudget != DefaultPrimaryBudget {
		t.Error("no fallback unless one is set")
	}
	// Without a fallback, the primary keeps the whole timeout (the old behaviour).
	m := &stubModel{delay: blockUntilDone}
	start := time.Now()
	NewService(m, "m", WithTimeout(120*time.Millisecond), WithPrimaryBudget(10*time.Millisecond), WithLogger(func(string, ...any) {})).Generate(context.Background(), englishStory)
	if took := time.Since(start); took < 100*time.Millisecond {
		t.Errorf("took %v: without a fallback the primary budget must not apply", took)
	}
}
