package stockmedia

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

// steadyPolicy is a policy whose waits are exact: jitter is pinned to its
// midpoint, which scales the delay by 1.0.
func steadyPolicy() RetryPolicy {
	p := RetryPolicy{
		MaxAttempts:      4,
		BaseDelay:        100 * time.Millisecond,
		MaxDelay:         time.Second,
		MaxRateLimitWait: 5 * time.Second,
	}
	p.jitter = func() float64 { return 0.5 }
	return p
}

// recordSleeps replaces the policy's sleep with one that notes each wait and
// returns at once.
func recordSleeps(p *RetryPolicy) *[]time.Duration {
	var waits []time.Duration
	p.sleep = func(ctx context.Context, d time.Duration) error {
		waits = append(waits, d)
		return ctx.Err()
	}
	return &waits
}

var errUnavailable = &ProviderError{Provider: ProviderPixabay, Kind: KindUnavailable, Status: 503}

// An interactive call may be one of several made in turn under the API gateway's
// 29 second limit (a plan opens up to ten lookups, four at a time). A hung
// provider already costs requestTimeout per call, so a retry must never add to
// that.
func TestDefaultRetryPolicy_NeverLetsRetriesOutlastAHungFirstAttempt(t *testing.T) {
	p := DefaultRetryPolicy()
	if p.Budget >= requestTimeout {
		t.Fatalf("Budget %v is not below the %v request timeout, so retries could make a hung call slower", p.Budget, requestTimeout)
	}
	if _, ok := p.next(errUnavailable, 1, requestTimeout); ok {
		t.Error("a call that hung for the whole request timeout was retried")
	}
	if _, ok := p.next(errUnavailable, 1, p.Budget); ok {
		t.Error("a call that used the whole budget was retried")
	}
	if _, ok := p.next(errUnavailable, 1, 150*time.Millisecond); !ok {
		t.Error("a call that failed fast was not retried")
	}
	// The longest rate-limit wait it will sit out still leaves room for a retry.
	if _, ok := p.next(&ProviderError{Kind: KindRateLimited, RetryAfter: p.MaxRateLimitWait}, 1, 0); !ok {
		t.Error("the longest accepted rate-limit wait does not fit in the budget, so it can never be used")
	}
}

func TestBatchRetryPolicy_FitsItsLongestRateLimitWaitAndNothingLonger(t *testing.T) {
	p := BatchRetryPolicy()
	if _, ok := p.next(&ProviderError{Kind: KindRateLimited, RetryAfter: p.MaxRateLimitWait}, 1, 0); !ok {
		t.Error("the longest accepted rate-limit wait does not fit in the budget")
	}
	if _, ok := p.next(&ProviderError{Kind: KindRateLimited, RetryAfter: p.MaxRateLimitWait + time.Second}, 1, 0); ok {
		t.Error("a wait over the maximum was accepted")
	}
}

func TestRetryPolicy_ZeroValueNeverRetries(t *testing.T) {
	calls := 0
	err := RetryPolicy{}.Do(context.Background(), func(context.Context, int) error { calls++; return errUnavailable }, nil)
	if calls != 1 || !errors.Is(err, ErrUnavailable) {
		t.Errorf("calls = %d, err = %v; want one call and the error back", calls, err)
	}
}

func TestRetryPolicy_RetriesAnOutageWithGrowingWaitsThenGivesUp(t *testing.T) {
	p := steadyPolicy()
	waits := recordSleeps(&p)
	var attempts []int
	err := p.Do(context.Background(), func(_ context.Context, a int) error { attempts = append(attempts, a); return errUnavailable }, nil)
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("err = %v, want the last error", err)
	}
	if fmt.Sprint(attempts) != "[1 2 3 4]" {
		t.Errorf("attempts = %v, want 1 to 4", attempts)
	}
	want := []time.Duration{100 * time.Millisecond, 200 * time.Millisecond, 400 * time.Millisecond}
	if fmt.Sprint(*waits) != fmt.Sprint(want) {
		t.Errorf("waits = %v, want %v", *waits, want)
	}
}

func TestRetryPolicy_StopsAsSoonAsACallSucceeds(t *testing.T) {
	p := steadyPolicy()
	recordSleeps(&p)
	calls := 0
	err := p.Do(context.Background(), func(context.Context, int) error {
		calls++
		if calls < 3 {
			return errUnavailable
		}
		return nil
	}, nil)
	if err != nil || calls != 3 {
		t.Errorf("err = %v, calls = %d; want success on the third call", err, calls)
	}
}

func TestRetryPolicy_BackoffIsCappedAtMaxDelay(t *testing.T) {
	p := steadyPolicy()
	p.MaxAttempts = 8
	for attempt, want := range map[int]time.Duration{
		1: 100 * time.Millisecond, 2: 200 * time.Millisecond, 3: 400 * time.Millisecond,
		4: 800 * time.Millisecond, 5: time.Second, 6: time.Second, 7: time.Second,
	} {
		if got := p.backoff(attempt); got != want {
			t.Errorf("backoff(%d) = %v, want %v", attempt, got, want)
		}
	}
}

func TestRetryPolicy_WithoutMaxDelayTheWaitStillGrowsAndCannotOverflow(t *testing.T) {
	p := RetryPolicy{BaseDelay: time.Second, MaxAttempts: 200}
	p.jitter = func() float64 { return 0.5 }
	if got := p.backoff(4); got != 8*time.Second {
		t.Errorf("backoff(4) = %v, want 8s", got)
	}
	if got := p.backoff(200); got <= 0 || got > maxBackoff {
		t.Errorf("backoff(200) = %v, want a positive wait of at most %v", got, maxBackoff)
	}
}

func TestRetryPolicy_JitterStaysWithinAQuarter(t *testing.T) {
	p := steadyPolicy()
	p.jitter = func() float64 { return 0 }
	if got := p.backoff(1); got != 75*time.Millisecond {
		t.Errorf("lowest jitter: %v, want 75ms", got)
	}
	p.jitter = func() float64 { return 0.999999 }
	if got := p.backoff(1); got < 124*time.Millisecond || got > 125*time.Millisecond {
		t.Errorf("highest jitter: %v, want about 125ms", got)
	}
	// Jitter never pushes a wait past MaxDelay.
	if got := p.backoff(5); got != time.Second {
		t.Errorf("jitter at the cap: %v, want 1s", got)
	}
}

func TestRetryPolicy_OnlyOutagesAndShortRateLimitsAreRetried(t *testing.T) {
	limited := func(after time.Duration) error {
		return &ProviderError{Provider: ProviderPexels, Kind: KindRateLimited, Status: 429, RetryAfter: after}
	}
	cooling := &ProviderError{Provider: ProviderPexels, Kind: KindRateLimited, RetryAfter: time.Second, Err: ErrCoolingDown}
	for _, tc := range []struct {
		name  string
		err   error
		retry bool
	}{
		{"outage", errUnavailable, true},
		{"transport failure", &ProviderError{Kind: KindUnavailable}, true},
		{"download interrupted", fmt.Errorf("%w: %w: x", ErrAssetUnavailable, ErrUnavailable), true},
		{"rate limited, told to wait 2s", limited(2 * time.Second), true},
		{"rate limited, told to wait exactly the maximum", limited(5 * time.Second), true},
		{"rate limited, told to wait longer than the maximum", limited(5*time.Second + time.Millisecond), false},
		{"rate limited, not told how long", limited(0), false},
		{"already cooling down", cooling, false},
		{"bad key", &ProviderError{Kind: KindAuth, Status: 401}, false},
		{"bad request", &ProviderError{Kind: KindBadRequest, Status: 400}, false},
		{"not found", &ProviderError{Kind: KindNotFound, Status: 404}, false},
		{"unusable response", &ProviderError{Kind: KindBadResponse, Status: 200}, false},
		{"asset gone", fmt.Errorf("%w: HTTP 404", ErrAssetUnavailable), false},
		{"invalid asset", invalidAsset("host", "x"), false},
		{"not an error of ours", errors.New("disk full"), false},
		{"cancelled", context.Canceled, false},
		{"deadline", context.DeadlineExceeded, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, ok := steadyPolicy().next(tc.err, 1, 0)
			if ok != tc.retry {
				t.Errorf("retry = %v, want %v", ok, tc.retry)
			}
		})
	}
}

func TestRetryPolicy_ARateLimitWaitIsHonouredInFull(t *testing.T) {
	p := steadyPolicy()
	// The provider asked for 3s: the 100ms backoff must not shorten it.
	wait, ok := p.next(&ProviderError{Kind: KindRateLimited, RetryAfter: 3 * time.Second}, 1, 0)
	if !ok || wait != 3*time.Second {
		t.Errorf("wait = %v, ok = %v; want 3s", wait, ok)
	}
	// A short ask is stretched to the backoff, which is longer.
	p.BaseDelay = 800 * time.Millisecond
	wait, ok = p.next(&ProviderError{Kind: KindRateLimited, RetryAfter: 100 * time.Millisecond}, 1, 0)
	if !ok || wait != 800*time.Millisecond {
		t.Errorf("wait = %v, ok = %v; want the 800ms backoff", wait, ok)
	}
}

func TestRetryPolicy_ARateLimitIsNeverRetriedWhenTheMaximumIsZero(t *testing.T) {
	p := steadyPolicy()
	p.MaxRateLimitWait = 0
	if _, ok := p.next(&ProviderError{Kind: KindRateLimited, RetryAfter: time.Millisecond}, 1, 0); ok {
		t.Error("a 429 was retried although waiting out rate limits is turned off")
	}
}

func TestRetryPolicy_NoRetryAfterTheLastAttempt(t *testing.T) {
	p := steadyPolicy()
	if _, ok := p.next(errUnavailable, p.MaxAttempts-1, 0); !ok {
		t.Error("attempt before the last should retry")
	}
	if _, ok := p.next(errUnavailable, p.MaxAttempts, 0); ok {
		t.Error("the last attempt must not be followed by another")
	}
}

func TestRetryPolicy_BudgetDecidesTheBoundaryExactly(t *testing.T) {
	p := steadyPolicy()
	p.Budget = 10 * time.Second
	p.minAttempt = 2 * time.Second
	// 100ms wait + 2s minimum: a retry fits while elapsed <= 7.9s.
	if _, ok := p.next(errUnavailable, 1, 7900*time.Millisecond); !ok {
		t.Error("a retry that exactly fits the budget should run")
	}
	if _, ok := p.next(errUnavailable, 1, 7900*time.Millisecond+time.Nanosecond); ok {
		t.Error("a retry that cannot get its minimum time should not start")
	}
	// A provider that took the whole budget to fail is not retried.
	if _, ok := p.next(errUnavailable, 1, 10*time.Second); ok {
		t.Error("retried after the budget was spent")
	}
}

func TestRetryPolicy_ABudgetCountsTheWaitAProviderAskedFor(t *testing.T) {
	p := steadyPolicy()
	p.Budget = 4 * time.Second
	p.minAttempt = 2 * time.Second
	// Told to wait 3s: 3s + 2s needed > 4s budget.
	if _, ok := p.next(&ProviderError{Kind: KindRateLimited, RetryAfter: 3 * time.Second}, 1, 0); ok {
		t.Error("a requested wait that leaves no time to retry was taken")
	}
	if _, ok := p.next(&ProviderError{Kind: KindRateLimited, RetryAfter: 2 * time.Second}, 1, 0); !ok {
		t.Error("a requested wait that leaves exactly the minimum was refused")
	}
}

func TestRetryPolicy_RetriesAreBoundedByWhatIsLeftOfTheBudgetButTheFirstAttemptIsNot(t *testing.T) {
	p := steadyPolicy()
	p.Budget = 30 * time.Second
	recordSleeps(&p)
	var deadlines []bool
	_ = p.Do(context.Background(), func(ctx context.Context, a int) error {
		_, has := ctx.Deadline()
		deadlines = append(deadlines, has)
		if a < 3 {
			return errUnavailable
		}
		return nil
	}, nil)
	if fmt.Sprint(deadlines) != "[false true true]" {
		t.Errorf("deadline on attempts = %v, want the first free and the retries bounded", deadlines)
	}
}

func TestRetryPolicy_WithNoBudgetARetryIsNotTimedAtAll(t *testing.T) {
	p := steadyPolicy() // Budget is zero
	recordSleeps(&p)
	var bounded []bool
	_ = p.Do(context.Background(), func(ctx context.Context, a int) error {
		_, has := ctx.Deadline()
		bounded = append(bounded, has)
		if a < 3 {
			return errUnavailable
		}
		return nil
	}, nil)
	if fmt.Sprint(bounded) != "[false false false]" {
		t.Errorf("deadline on attempts = %v; with no budget a slow provider must keep its full allowance", bounded)
	}
}

func TestRetryPolicy_ARetryGetsWhatIsLeftOfTheBudgetNotAllOfIt(t *testing.T) {
	p := steadyPolicy()
	p.Budget = 30 * time.Second
	p.minAttempt = 2 * time.Second
	clock := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)
	p.now = func() time.Time { return clock }
	p.sleep = func(ctx context.Context, d time.Duration) error { clock = clock.Add(d); return ctx.Err() }

	var left time.Duration
	_ = p.Do(context.Background(), func(ctx context.Context, a int) error {
		if a == 1 {
			clock = clock.Add(19 * time.Second) // a slow first attempt, then an outage
			return errUnavailable
		}
		dl, ok := ctx.Deadline()
		if !ok {
			t.Error("the retry has no deadline")
		}
		left = time.Until(dl)
		return nil
	}, nil)
	// 19s + the 100ms wait are spent: about 10.9s remain.
	if left <= 10*time.Second || left > 10900*time.Millisecond {
		t.Errorf("the retry was given %v, want what remains of the 30s budget (about 10.9s)", left)
	}
}

func TestRetryPolicy_TheHookHearsOfEachRetryBeforeItsWait(t *testing.T) {
	p := steadyPolicy()
	var events []string
	p.sleep = func(_ context.Context, d time.Duration) error {
		events = append(events, "sleep "+d.String())
		return nil
	}
	calls := 0
	_ = p.Do(context.Background(), func(context.Context, int) error {
		calls++
		if calls < 3 {
			return errUnavailable
		}
		return nil
	}, func(attempt int, wait time.Duration, err error) {
		events = append(events, fmt.Sprintf("retry after attempt %d in %v (%v)", attempt, wait, errors.Is(err, ErrUnavailable)))
	})
	want := "[retry after attempt 1 in 100ms (true) sleep 100ms retry after attempt 2 in 200ms (true) sleep 200ms]"
	if fmt.Sprint(events) != want {
		t.Errorf("events = %v\nwant     %s", events, want)
	}
}

func TestRetryPolicy_CancellingDuringAWaitEndsTheLoop(t *testing.T) {
	p := steadyPolicy()
	p.BaseDelay = time.Hour
	p.MaxDelay = time.Hour
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	done := make(chan error, 1)
	go func() {
		done <- p.Do(ctx, func(context.Context, int) error { calls++; return errUnavailable }, func(int, time.Duration, error) { cancel() })
	}()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("err = %v, want context.Canceled", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Do kept waiting after its context was cancelled")
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1: nothing may be sent after cancellation", calls)
	}
}

func TestRetryPolicy_ACancelledContextIsNotRetried(t *testing.T) {
	p := steadyPolicy()
	waits := recordSleeps(&p)
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	err := p.Do(ctx, func(context.Context, int) error { calls++; cancel(); return errUnavailable }, nil)
	if calls != 1 || len(*waits) != 0 {
		t.Errorf("calls = %d, waits = %v; want one call and no wait", calls, *waits)
	}
	if !errors.Is(err, ErrUnavailable) {
		t.Errorf("err = %v, want the call's own error", err)
	}
}
