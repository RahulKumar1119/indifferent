package storyjob

import (
	"context"
	"sync"
)

// runParallel calls fn(ctx, i) for each index in idx, at most workers at a time,
// taking the indexes in order, and returns when every call it started has
// returned. With one worker it is a plain loop.
//
// It stops starting new indexes as soon as one call fails, ctx ends, or stop (if
// not nil) reports true. A call already running is left to finish, not
// cancelled: a scene that is nearly done is worth keeping, because the caller
// records each one as it completes and a restarted task then skips it.
//
// It returns the error of each failed index, and the indexes it did not start
// (a caller that stopped on stop() carries on with those; one that failed or was
// cancelled does not).
func runParallel(ctx context.Context, workers int, idx []int, stop func() bool, fn func(ctx context.Context, i int) error) (errs map[int]error, notStarted []int) {
	errs = map[int]error{}
	if len(idx) == 0 {
		return errs, nil
	}
	workers = max(1, min(workers, len(idx)))
	var (
		mu     sync.Mutex // guards next, failed and errs
		next   int
		failed bool
		wg     sync.WaitGroup
	)
	take := func() (int, bool) {
		mu.Lock()
		defer mu.Unlock()
		if failed || next >= len(idx) || ctx.Err() != nil || (stop != nil && stop()) {
			return 0, false
		}
		i := idx[next]
		next++
		return i, true
	}
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				i, ok := take()
				if !ok {
					return
				}
				if err := fn(ctx, i); err != nil {
					mu.Lock()
					errs[i] = err
					failed = true
					mu.Unlock()
				}
			}
		}()
	}
	wg.Wait()
	return errs, idx[next:]
}

// firstError is the error of the lowest-numbered scene that failed, or nil. Several
// scenes can fail at once when they run side by side, and the lowest one is the
// one a one-at-a-time run would have stopped on, so the user is told about the
// same scene either way.
func firstError(errs map[int]error) error {
	best := -1
	for i := range errs {
		if best < 0 || i < best {
			best = i
		}
	}
	if best < 0 {
		return nil
	}
	return errs[best]
}
