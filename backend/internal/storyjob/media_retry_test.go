package storyjob

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rahul/indifferent/backend/internal/stockmedia"
)

// quickRetry is a retry policy fast enough for a test: three tries, a
// millisecond apart.
func quickRetry() stockmedia.RetryPolicy {
	return stockmedia.RetryPolicy{MaxAttempts: 3, BaseDelay: time.Millisecond, MaxDelay: 2 * time.Millisecond}
}

// cdnDown is what Service.Download returns when the provider's CDN fails or the
// transfer breaks off: not the asset's fault, so worth trying again.
func cdnDown(status int) error {
	return fmt.Errorf("%w: %w: pixabay photo 102 answered HTTP %d", stockmedia.ErrAssetUnavailable, stockmedia.ErrUnavailable, status)
}

// logSink collects what a step logs.
type logSink struct {
	mu    sync.Mutex
	lines []string
}

func (l *logSink) logf(format string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.lines = append(l.lines, fmt.Sprintf(format, args...))
}

func (l *logSink) text() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return strings.Join(l.lines, "\n")
}

func TestCopyMedia_TriesABrokenDownloadAgainAndKeepsOnlyTheGoodFile(t *testing.T) {
	e := newEnv(t)
	logs := &logSink{}
	e.deps.Logf = logs.logf
	e.deps.Retry = quickRetry()
	job := e.jobWith(3)
	// Scene 2: the CDN fails once, then the transfer breaks off part way, then it works.
	e.assets.dlSeq["pixabay:photo:102"] = []error{cdnDown(503), cdnDown(502)}
	e.assets.dlPartial["pixabay:photo:102"] = "PARTIAL-JUNK"

	if err := CopyMedia(context.Background(), e.deps, job, nil); err != nil {
		t.Fatalf("a CDN that recovers must not fail the job: %v", err)
	}
	if n := countOf(e.assets.downloads, "pixabay:photo:102"); n != 3 {
		t.Errorf("downloads of scene 2's picture = %d, want 3", n)
	}
	if n := countOf(e.assets.downloads, "pixabay:photo:101"); n != 1 {
		t.Errorf("scene 1 was downloaded %d times; a retry in scene 2 must not repeat it", n)
	}
	// The partial bytes of the failed attempts must not survive into the copy.
	got := string(e.files.files["bucket/"+job.Scenes[1].Media.Key])
	if got != "bytes-of-pixabay:photo:102" {
		t.Errorf("stored copy = %q, want the complete file and nothing of the partial ones", got)
	}
	if !strings.Contains(logs.text(), "scene=2 media=download_retry attempt=1") || !strings.Contains(logs.text(), "attempt=2") {
		t.Errorf("the retries were not logged:\n%s", logs.text())
	}
	if left := listDir(t, e.deps.WorkDir); len(left) != 0 {
		t.Errorf("scratch files left behind: %v", left)
	}
}

func TestCopyMedia_ACDNThatKeepsFailingEndsInARetryableFailureAfterTheLastTry(t *testing.T) {
	e := newEnv(t)
	e.deps.Retry = quickRetry()
	job := e.jobWith(3)
	e.assets.dlErr["pixabay:photo:102"] = cdnDown(503)

	se := stepErr(t, CopyMedia(context.Background(), e.deps, job, nil))
	if se.Scene != 2 || se.Code != CodeMediaTemporary || !se.Retryable {
		t.Errorf("got scene %d code %s retryable %v; want scene 2 MEDIA_TEMPORARY retryable", se.Scene, se.Code, se.Retryable)
	}
	if n := countOf(e.assets.downloads, "pixabay:photo:102"); n != 3 {
		t.Errorf("downloads = %d, want exactly the 3 the policy allows", n)
	}
	if job.Scenes[2].Media != nil {
		t.Error("scene 3 was copied after scene 2 failed")
	}
}

func TestCopyMedia_NeverTriesAgainAtAnythingTheSecondTryCannotFix(t *testing.T) {
	for name, err := range map[string]error{
		"the file is gone":           fmt.Errorf("%w: HTTP 404", stockmedia.ErrAssetUnavailable),
		"the file is invalid":        fmt.Errorf("%w: wrong host", stockmedia.ErrInvalidAsset),
		"the file is too big":        fmt.Errorf("%w: over the limit", stockmedia.ErrAssetUnavailable),
		"the worker's disk is full":  fmt.Errorf("writing the download: %w", errors.New("no space left on device")),
		"someone else's plain error": errors.New("boom"),
	} {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t)
			e.deps.Retry = quickRetry()
			job := e.jobWith(2)
			e.assets.dlErr["pixabay:photo:102"] = err
			if CopyMedia(context.Background(), e.deps, job, nil) == nil {
				t.Fatal("no error")
			}
			if n := countOf(e.assets.downloads, "pixabay:photo:102"); n != 1 {
				t.Errorf("downloads = %d, want 1: this failure would repeat", n)
			}
		})
	}
}

func TestCopyMedia_WithoutARetryPolicyADownloadIsTriedOnce(t *testing.T) {
	e := newEnv(t) // Deps.Retry is the zero value
	job := e.jobWith(2)
	e.assets.dlErr["pixabay:photo:102"] = cdnDown(503)
	if CopyMedia(context.Background(), e.deps, job, nil) == nil {
		t.Fatal("no error")
	}
	if n := countOf(e.assets.downloads, "pixabay:photo:102"); n != 1 {
		t.Errorf("downloads = %d, want 1", n)
	}
}

func TestCopyMedia_CancellingDuringARetryWaitStopsAtOnce(t *testing.T) {
	e := newEnv(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	e.deps.Retry = stockmedia.RetryPolicy{MaxAttempts: 3, BaseDelay: time.Hour, MaxDelay: time.Hour}
	// The worker is told to stop just as it decides to wait before a retry.
	e.deps.Logf = func(format string, _ ...any) {
		if strings.Contains(format, "download_retry") {
			cancel()
		}
	}
	job := e.jobWith(2)
	e.assets.dlErr["pixabay:photo:102"] = cdnDown(503)

	done := make(chan error, 1)
	go func() { done <- CopyMedia(ctx, e.deps, job, nil) }()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("err = %v, want context.Canceled", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the step kept waiting after it was cancelled")
	}
	if n := countOf(e.assets.downloads, "pixabay:photo:102"); n != 1 {
		t.Errorf("downloads = %d, want 1", n)
	}
	if left := listDir(t, e.deps.WorkDir); len(left) != 0 {
		t.Errorf("scratch files left behind: %v", left)
	}
}

func countOf(list []string, want string) int {
	n := 0
	for _, s := range list {
		if s == want {
			n++
		}
	}
	return n
}

// throttled is what Service.Download returns when the file host answers 429.
func throttled() error {
	return fmt.Errorf("%w: %w: %w: pixabay photo 102 answered HTTP 429", stockmedia.ErrAssetUnavailable, stockmedia.ErrUnavailable, stockmedia.ErrThrottled)
}

// sleepLog records the waits a step asks for, without waiting.
type sleepLog struct{ waits []time.Duration }

func (s *sleepLog) sleep(_ context.Context, d time.Duration) error {
	s.waits = append(s.waits, d)
	return nil
}

func TestCopyMedia_AThrottledFileHostIsSatOutWithTheLongWaitsAndThenWorks(t *testing.T) {
	e := newEnv(t)
	logs := &logSink{}
	slept := &sleepLog{}
	e.deps.Logf, e.deps.Sleep = logs.logf, slept.sleep
	e.deps.Retry = quickRetry()
	e.deps.ThrottleWaits = []time.Duration{20 * time.Second, 40 * time.Second, 80 * time.Second}
	job := e.jobWith(3)
	// Two whole rounds of 3 tries answer 429, then the host lets it through.
	e.assets.dlSeq["pixabay:photo:102"] = []error{throttled(), throttled(), throttled(), throttled(), throttled(), throttled()}
	if err := CopyMedia(context.Background(), e.deps, job, nil); err != nil {
		t.Fatalf("a host that stops throttling must not fail the job: %v", err)
	}
	if n := countOf(e.assets.downloads, "pixabay:photo:102"); n != 7 {
		t.Errorf("downloads = %d, want 7 (two rounds of 3, then the one that worked)", n)
	}
	if want := []time.Duration{20 * time.Second, 40 * time.Second}; fmt.Sprint(slept.waits) != fmt.Sprint(want) {
		t.Errorf("waits = %v, want %v", slept.waits, want)
	}
	if !strings.Contains(logs.text(), "scene=2 media=throttled wait_s=20") {
		t.Errorf("the long wait was not logged:\n%s", logs.text())
	}
}

func TestCopyMedia_ThrottlingThatOutlastsTheWaitsFailsRetryablyAndSaysSo(t *testing.T) {
	e := newEnv(t)
	slept := &sleepLog{}
	e.deps.Sleep = slept.sleep
	e.deps.Retry = quickRetry()
	e.deps.ThrottleWaits = []time.Duration{time.Second, 2 * time.Second}
	job := e.jobWith(3)
	e.assets.dlErr["pixabay:photo:102"] = throttled()
	se := stepErr(t, CopyMedia(context.Background(), e.deps, job, nil))
	if se.Scene != 2 || se.Code != CodeMediaTemporary || !se.Retryable {
		t.Errorf("got scene %d code %s retryable %v; want scene 2 MEDIA_TEMPORARY retryable", se.Scene, se.Code, se.Retryable)
	}
	if !strings.Contains(se.Message, "Pixabay is limiting downloads right now") {
		t.Errorf("message = %q, want it to say the library is limiting downloads", se.Message)
	}
	if n := countOf(e.assets.downloads, "pixabay:photo:102"); n != 9 {
		t.Errorf("downloads = %d, want 9 (three rounds of 3)", n)
	}
	if len(slept.waits) != 2 {
		t.Errorf("waits = %v, want both long waits used once", slept.waits)
	}
}

func TestCopyMedia_TheJobsThrottleBudgetIsSharedByItsScenes(t *testing.T) {
	e := newEnv(t)
	logs := &logSink{}
	slept := &sleepLog{}
	e.deps.Logf, e.deps.Sleep = logs.logf, slept.sleep
	e.deps.Retry = quickRetry()
	e.deps.ThrottleWaits = []time.Duration{30 * time.Second, 30 * time.Second}
	e.deps.ThrottleBudget = 45 * time.Second
	job := e.jobWith(3)
	// Scene 1 is throttled for one round and uses 30 s of the 45 s allowance.
	e.assets.dlSeq["pixabay:photo:101"] = []error{throttled(), throttled(), throttled()}
	// Scene 2 is throttled for good: 15 s is left, less than its first wait.
	e.assets.dlErr["pixabay:photo:102"] = throttled()
	se := stepErr(t, CopyMedia(context.Background(), e.deps, job, nil))
	if se.Scene != 2 {
		t.Errorf("failed scene = %d, want 2", se.Scene)
	}
	if fmt.Sprint(slept.waits) != fmt.Sprint([]time.Duration{30 * time.Second}) {
		t.Errorf("waits = %v, want only scene 1's 30 s", slept.waits)
	}
	if n := countOf(e.assets.downloads, "pixabay:photo:102"); n != 3 {
		t.Errorf("scene 2 downloads = %d, want one round of 3 and no long wait", n)
	}
	if !strings.Contains(logs.text(), "scene=2 media=throttled no_wait_left") {
		t.Errorf("running out of allowance was not logged:\n%s", logs.text())
	}
}

func TestCopyMedia_AnOutageThatIsNotThrottlingGetsNoLongWait(t *testing.T) {
	e := newEnv(t)
	slept := &sleepLog{}
	e.deps.Sleep = slept.sleep
	e.deps.Retry = quickRetry()
	e.deps.ThrottleWaits = []time.Duration{time.Minute}
	job := e.jobWith(3)
	e.assets.dlErr["pixabay:photo:102"] = cdnDown(503)
	stepErr(t, CopyMedia(context.Background(), e.deps, job, nil))
	if len(slept.waits) != 0 {
		t.Errorf("a 503 waited %v; only a 429 is worth the long waits", slept.waits)
	}
}

func TestCopyMedia_ACancelDuringALongWaitStopsAtOnce(t *testing.T) {
	e := newEnv(t)
	e.deps.Retry = quickRetry()
	e.deps.ThrottleWaits = []time.Duration{time.Hour}
	job := e.jobWith(3)
	e.assets.dlErr["pixabay:photo:102"] = throttled()
	ctx, cancel := context.WithCancel(context.Background())
	e.deps.Sleep = func(ctx context.Context, d time.Duration) error {
		cancel()
		<-ctx.Done()
		return ctx.Err()
	}
	err := CopyMedia(ctx, e.deps, job, nil)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}
