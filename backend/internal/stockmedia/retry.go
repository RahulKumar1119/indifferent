package stockmedia

import (
	"context"
	"errors"
	"math/rand/v2"
	"time"
)

const (
	// minRetryAttempt is the least time worth starting a retry with. A retry
	// that would be cut off almost at once only adds load to a provider that is
	// already struggling.
	minRetryAttempt = 2 * time.Second
	// maxBackoff is an upper bound on any computed wait, so a policy with no
	// MaxDelay and a large MaxAttempts cannot overflow a Duration.
	maxBackoff = time.Hour
)

// RetryPolicy says when a failed provider call is tried again, and how long to
// wait first. The zero value never retries.
//
// Two kinds of failure are retried, and nothing else:
//
//   - The provider could not be reached or answered 5xx (ErrUnavailable), after
//     a growing, jittered wait.
//   - The provider answered 429 and said how long to wait (a Retry-After value,
//     or Pixabay's X-RateLimit-Reset), and that wait is no more than
//     MaxRateLimitWait. The wait is honoured in full.
//
// A 429 with no wait given is not retried: Pexels sends no rate-limit headers
// on a 429, and guessing is how keys get revoked. A 429 asking for a long wait
// (a monthly quota can mean weeks) is not retried either; the caller fails
// over to the other provider, or reports the failure. A provider already in
// cooldown (ErrCoolingDown) is not waited for. Bad requests, bad keys and
// missing assets are the caller's to deal with and are never retried.
type RetryPolicy struct {
	// MaxAttempts is the number of tries, the first included. Zero or one means
	// no retries.
	MaxAttempts int
	// BaseDelay is the wait before the first retry. Each later wait doubles, up
	// to MaxDelay, and is varied by up to a quarter either way so many callers
	// that failed together do not return together.
	BaseDelay time.Duration
	MaxDelay  time.Duration
	// MaxRateLimitWait is the longest 429 wait that is worth sitting out. Zero
	// means a 429 is never retried.
	MaxRateLimitWait time.Duration
	// Budget bounds the whole call, counted from the start of the first attempt:
	// no retry begins unless at least minRetryAttempt of it would remain, and a
	// retry is cut off when the budget ends. The first attempt always gets its
	// normal allowance. Zero means no bound beyond MaxAttempts.
	Budget time.Duration

	// Seams for tests. Nil or zero means the real thing.
	sleep      func(context.Context, time.Duration) error
	now        func() time.Time
	jitter     func() float64
	minAttempt time.Duration
}

func (p RetryPolicy) minRetry() time.Duration {
	if p.minAttempt > 0 {
		return p.minAttempt
	}
	return minRetryAttempt
}

// DefaultRetryPolicy is for calls someone is waiting on, such as a search in
// the plan editor: two quick retries after a fast failure.
//
// Its budget is deliberately shorter than requestTimeout. A provider that
// failed fast (a 503, a reset connection) is tried again; one that hung for the
// whole request timeout is not, because a hang is rarely brief and the caller
// has already waited as long as it is ever made to. So retries never make a
// call slower than its first attempt could already have been, which matters
// where several lookups run in turn under the API gateway's 29 second limit.
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxAttempts:      3,
		BaseDelay:        250 * time.Millisecond,
		MaxDelay:         2 * time.Second,
		MaxRateLimitWait: 2 * time.Second,
		Budget:           6 * time.Second,
	}
}

// BatchRetryPolicy is for background work such as the video worker, where no
// one is waiting and a failed attempt costs a whole task restart. It tries
// harder, and will sit out a rate-limit window of up to 30 seconds.
func BatchRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxAttempts:      4,
		BaseDelay:        time.Second,
		MaxDelay:         8 * time.Second,
		MaxRateLimitWait: 30 * time.Second,
		Budget:           90 * time.Second,
	}
}

// Do calls fn until it succeeds or the policy gives up, and returns the last
// error. fn receives the attempt number from 1; on a retry its context is also
// bounded by what is left of Budget. onRetry, which may be nil, is told of each
// retry just before the wait.
//
// A cancelled or expired ctx ends the loop at once, with ctx's own error if it
// came during a wait.
func (p RetryPolicy) Do(ctx context.Context, fn func(ctx context.Context, attempt int) error, onRetry func(attempt int, wait time.Duration, err error)) error {
	now := p.now
	if now == nil {
		now = time.Now
	}
	start := now()
	for attempt := 1; ; attempt++ {
		attemptCtx, cancel := ctx, context.CancelFunc(func() {})
		if attempt > 1 && p.Budget > 0 {
			attemptCtx, cancel = context.WithTimeout(ctx, max(p.Budget-now().Sub(start), p.minRetry()))
		}
		err := fn(attemptCtx, attempt)
		cancel()
		if err == nil || ctx.Err() != nil {
			return err
		}
		wait, ok := p.next(err, attempt, now().Sub(start))
		if !ok {
			return err
		}
		if onRetry != nil {
			onRetry(attempt, wait, err)
		}
		if serr := p.sleepFor(ctx, wait); serr != nil {
			return serr
		}
	}
}

// next decides whether a failed attempt is followed by another, and after how
// long. elapsed is the time since the first attempt began.
func (p RetryPolicy) next(err error, attempt int, elapsed time.Duration) (time.Duration, bool) {
	if attempt >= p.MaxAttempts {
		return 0, false
	}
	var wait time.Duration
	switch {
	case errors.Is(err, ErrCoolingDown):
		// The service is already holding off this provider. Another try would
		// only be refused, and waiting would hold up a caller that could use
		// the other provider's answer.
		return 0, false
	case errors.Is(err, ErrRateLimited):
		var pe *ProviderError
		if !errors.As(err, &pe) || pe.RetryAfter <= 0 || pe.RetryAfter > p.MaxRateLimitWait {
			return 0, false
		}
		wait = max(p.backoff(attempt), pe.RetryAfter)
	case errors.Is(err, ErrUnavailable):
		wait = p.backoff(attempt)
	default:
		return 0, false
	}
	if p.Budget > 0 && elapsed+wait+p.minRetry() > p.Budget {
		return 0, false
	}
	return wait, true
}

// backoff is the wait after the given failed attempt: BaseDelay doubled for
// each earlier retry, capped at MaxDelay, then varied by up to a quarter.
func (p RetryPolicy) backoff(attempt int) time.Duration {
	d := p.BaseDelay
	for i := 1; i < attempt; i++ {
		if (p.MaxDelay > 0 && d >= p.MaxDelay) || d >= maxBackoff {
			break
		}
		d *= 2
	}
	d = min(d, maxBackoff)
	if p.MaxDelay > 0 {
		d = min(d, p.MaxDelay)
	}
	j := rand.Float64
	if p.jitter != nil {
		j = p.jitter
	}
	d = time.Duration(float64(d) * (0.75 + 0.5*j()))
	if p.MaxDelay > 0 {
		d = min(d, p.MaxDelay)
	}
	return d
}

func (p RetryPolicy) sleepFor(ctx context.Context, d time.Duration) error {
	if p.sleep != nil {
		return p.sleep(ctx, d)
	}
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
