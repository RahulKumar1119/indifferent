package storyjob

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rahul/indifferent/backend/internal/stockmedia"
)

// memStore is a Store in memory. Like the real one it never creates a record
// and never touches a job that has ended.
type memStore struct {
	mu       sync.Mutex
	job      *Job
	gone     bool
	saves    []string // "status:progress" at each Save
	finishes int
	saveErr  func(n int) error
	finErr   error
	loadErr  error
	statusFn func() (string, error)
	saveN    int
}

func newStore(j *Job) *memStore { c := clone(j); return &memStore{job: &c} }

func clone(j *Job) Job {
	b, _ := json.Marshal(j)
	var c Job
	_ = json.Unmarshal(b, &c)
	return c
}

func (s *memStore) Load(_ context.Context, _, _ string) (*Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.loadErr != nil {
		return nil, s.loadErr
	}
	if s.gone {
		return nil, ErrJobGone
	}
	c := clone(s.job)
	return &c, nil
}

func (s *memStore) write(j *Job) error {
	if s.gone {
		return ErrJobGone
	}
	if Terminal(s.job.Status) {
		return fmt.Errorf("%w: it is %s", errStoppedElsewhere, s.job.Status)
	}
	c := clone(j)
	s.job = &c
	return nil
}

func (s *memStore) Save(_ context.Context, j *Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.saveN++
	if s.saveErr != nil {
		if err := s.saveErr(s.saveN); err != nil {
			return err
		}
	}
	if err := s.write(j); err != nil {
		return err
	}
	s.saves = append(s.saves, fmt.Sprintf("%s:%d", j.Status, j.Progress))
	return nil
}

func (s *memStore) Finish(_ context.Context, j *Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.finishes++
	if s.finErr != nil {
		return s.finErr
	}
	return s.write(j)
}

func (s *memStore) Status(_ context.Context, _, _ string) (string, error) {
	s.mu.Lock()
	fn, gone, st := s.statusFn, s.gone, s.job.Status
	s.mu.Unlock()
	if fn != nil {
		return fn()
	}
	if gone {
		return "", ErrJobGone
	}
	return st, nil
}

func (s *memStore) current() Job { s.mu.Lock(); defer s.mu.Unlock(); return clone(s.job) }

// fakeRenderer stands in for FFmpeg.
type fakeRenderer struct {
	calls   int
	err     error
	seconds float64
	body    string
	started chan struct{}
	block   bool
}

func (f *fakeRenderer) Render(ctx context.Context, job *Job, _, outPath string, onPercent func(int)) (float64, error) {
	f.calls++
	if f.started != nil {
		close(f.started)
	}
	if f.block {
		<-ctx.Done()
		return 0, ctx.Err()
	}
	if f.err != nil {
		return 0, f.err
	}
	for _, p := range []int{10, 50, 99} {
		onPercent(p)
	}
	body := f.body
	if body == "" {
		body = "MP4:" + job.JobID
	}
	if err := os.WriteFile(outPath, []byte(body), 0o600); err != nil {
		return 0, err
	}
	if f.seconds == 0 {
		f.seconds = 31.5
	}
	return f.seconds, nil
}

func (e *env) runner(st *memStore, r Renderer) *Runner {
	return &Runner{Deps: e.deps, Store: st, Renderer: r, Now: func() time.Time { return time.Date(2026, 10, 31, 15, 0, 0, 0, time.UTC) }, CancelCheck: 20 * time.Millisecond}
}

func TestRun_AJobGoesThroughEveryStageAndEndsCompletedWithItsVideo(t *testing.T) {
	e := newEnv(t)
	st := newStore(e.jobWith(3))
	rend := &fakeRenderer{}
	out, err := e.runner(st, rend).Run(context.Background(), "u1", "j1")
	if err != nil || out != OutcomeCompleted {
		t.Fatalf("outcome %v, err %v", out, err)
	}
	got := st.current()
	if got.Status != StatusCompleted || got.Progress != 100 || got.OutputKey != "storyvideo/u1/j1/video.mp4" || got.TotalSeconds != 31.5 {
		t.Errorf("job = %+v", got)
	}
	if got.CompletedAt != "2026-10-31T15:00:00Z" || got.Error != "" || got.ErrorCode != "" {
		t.Errorf("completion fields = %q / error %q %q", got.CompletedAt, got.Error, got.ErrorCode)
	}
	if string(e.files.files["bucket/storyvideo/u1/j1/video.mp4"]) != "MP4:j1" {
		t.Errorf("the video was not stored: %v", e.files.puts)
	}
	for i, s := range got.Scenes {
		if s.Media == nil || s.Voice == nil {
			t.Errorf("scene %d did not keep what was made for it: %+v", i+1, s)
		}
	}
	// Stages in order, progress only forward.
	var stages []string
	last := -1
	for _, rec := range st.saves {
		parts := strings.Split(rec, ":")
		var p int
		fmt.Sscan(parts[1], &p)
		if p < last {
			t.Errorf("progress went backwards at %q after %d", rec, last)
		}
		last = p
		if len(stages) == 0 || stages[len(stages)-1] != parts[0] {
			stages = append(stages, parts[0])
		}
	}
	if strings.Join(stages, ",") != "preparing,narrating,rendering" {
		t.Errorf("stages = %v", stages)
	}
	if st.finishes != 1 {
		t.Errorf("Finish called %d times", st.finishes)
	}
}

func TestRun_ProgressStaysInsideItsStage(t *testing.T) {
	e := newEnv(t)
	st := newStore(e.jobWith(4))
	if _, err := e.runner(st, &fakeRenderer{}).Run(context.Background(), "u1", "j1"); err != nil {
		t.Fatal(err)
	}
	band := map[string][2]int{"preparing": {5, 25}, "narrating": {25, 40}, "rendering": {40, 99}}
	for _, rec := range st.saves {
		var status string
		var p int
		parts := strings.Split(rec, ":")
		status = parts[0]
		fmt.Sscan(parts[1], &p)
		if b := band[status]; p < b[0] || p > b[1] {
			t.Errorf("%s reported %d%%, outside %d to %d", status, p, b[0], b[1])
		}
	}
}

func TestRun_AJobThatIsAlreadyOverIsLeftAlone(t *testing.T) {
	e := newEnv(t)
	done := e.jobWith(2)
	done.Status = StatusCompleted
	rend := &fakeRenderer{}
	out, err := e.runner(newStore(done), rend).Run(context.Background(), "u1", "j1")
	if out != OutcomeCompleted || err != nil || rend.calls != 0 || len(e.assets.gets) != 0 {
		t.Errorf("outcome %v err %v renders %d lookups %d", out, err, rend.calls, len(e.assets.gets))
	}
	failed := e.jobWith(2)
	failed.Status, failed.Error = StatusFailed, "Scene 2: gone."
	out, err = e.runner(newStore(failed), rend).Run(context.Background(), "u1", "j1")
	if out != OutcomeFailed || err == nil || err.Error() != "Scene 2: gone." {
		t.Errorf("outcome %v err %v", out, err)
	}
}

func TestRun_AFailureTheUserMustFixIsFinalAndSaysWhere(t *testing.T) {
	e := newEnv(t)
	job := e.jobWith(3)
	delete(e.assets.assets, "pixabay:photo:102") // after jobWith, which adds every asset it uses
	st := newStore(job)
	rend := &fakeRenderer{}
	out, err := e.runner(st, rend).Run(context.Background(), "u1", "j1")
	if out != OutcomeFailed || err == nil {
		t.Fatalf("outcome %v err %v", out, err)
	}
	got := st.current()
	if got.Status != StatusFailed || got.ErrorCode != CodeMediaUnavailable || got.FailedStage != StatusPreparing {
		t.Errorf("job = %+v", got)
	}
	if !strings.Contains(got.Error, "Scene 2") || !strings.Contains(got.Error, "Choose another") {
		t.Errorf("the user is told %q", got.Error)
	}
	if rend.calls != 0 || len(e.narrator.calls) != 0 {
		t.Errorf("work went on after the failure: %d renders, %d Polly calls", rend.calls, len(e.narrator.calls))
	}
	if got.OutputKey != "" {
		t.Errorf("a failed job has an output: %q", got.OutputKey)
	}
}

func TestRun_ATemporaryFailureIsNotFinalAndKeepsItsReasonForIfRetriesRunOut(t *testing.T) {
	e := newEnv(t)
	e.assets.getErr["pixabay:photo:102"] = fmt.Errorf("provider: %w", errTemporary())
	st := newStore(e.jobWith(3))
	out, err := e.runner(st, &fakeRenderer{}).Run(context.Background(), "u1", "j1")
	if out != OutcomeRetry || err == nil {
		t.Fatalf("outcome %v err %v", out, err)
	}
	got := st.current()
	if Terminal(got.Status) || st.finishes != 0 {
		t.Errorf("a failure that a retry may fix was recorded as final: %+v (finishes %d)", got, st.finishes)
	}
	if !strings.Contains(got.Error, "Scene 2") || got.ErrorCode != CodeMediaTemporary {
		t.Errorf("the reason was not kept: %q %q", got.Error, got.ErrorCode)
	}
}

func TestRun_ARestartResumesWhereTheLastRunStoppedAndClearsItsOldReason(t *testing.T) {
	e := newEnv(t)
	e.assets.getErr["pixabay:photo:102"] = fmt.Errorf("provider: %w", errTemporary())
	st := newStore(e.jobWith(3))
	if out, _ := e.runner(st, &fakeRenderer{}).Run(context.Background(), "u1", "j1"); out != OutcomeRetry {
		t.Fatal("setup: the first run should ask to be retried")
	}
	downloadsAfterFirst := len(e.assets.downloads) // scene 1 only

	delete(e.assets.getErr, "pixabay:photo:102") // the provider is back
	var sawReason []string
	rend := &renderSpy{inner: &fakeRenderer{}, onRender: func(j *Job) { sawReason = append(sawReason, j.Error+"|"+j.ErrorCode) }}
	out, err := e.runner(st, rend).Run(context.Background(), "u1", "j1")
	if out != OutcomeCompleted || err != nil {
		t.Fatalf("outcome %v err %v", out, err)
	}
	if n := len(e.assets.downloads) - downloadsAfterFirst; n != 2 {
		t.Errorf("the second run made %d downloads, want 2 (scenes 2 and 3; scene 1 was kept)", n)
	}
	if len(sawReason) != 1 || sawReason[0] != "|" {
		t.Errorf("the old failure was still on the job while it rendered: %v", sawReason)
	}
	if got := st.current(); got.Error != "" || got.ErrorCode != "" || got.FailedStage != "" {
		t.Errorf("a completed job carries an old failure: %+v", got)
	}
}

type renderSpy struct {
	inner    Renderer
	onRender func(*Job)
}

func (r *renderSpy) Render(ctx context.Context, j *Job, w, o string, p func(int)) (float64, error) {
	r.onRender(j)
	return r.inner.Render(ctx, j, w, o, p)
}

func TestRun_AStoryThatIsTooLongSpokenFailsBeforeRendering(t *testing.T) {
	e := newEnv(t)
	e.prober.info = func(path, c string) (MediaInfo, error) {
		if strings.HasPrefix(c, "MP3:") {
			return MediaInfo{Duration: 9.5, HasAudio: true}, nil
		}
		return MediaInfo{Width: 1280, Height: 1920, HasVideo: true}, nil
	}
	st := newStore(e.jobWith(10))
	rend := &fakeRenderer{}
	out, _ := e.runner(st, rend).Run(context.Background(), "u1", "j1")
	got := st.current()
	if out != OutcomeFailed || got.ErrorCode != CodeNarrationTooLong || got.FailedStage != StatusNarrating || rend.calls != 0 {
		t.Errorf("outcome %v job %+v renders %d", out, got, rend.calls)
	}
}

func TestRun_RenderFailures(t *testing.T) {
	t.Run("a render that broke is retried", func(t *testing.T) {
		e := newEnv(t)
		st := newStore(e.jobWith(2))
		out, err := e.runner(st, &fakeRenderer{err: internalError(0, errors.New("ffmpeg died"))}).Run(context.Background(), "u1", "j1")
		if out != OutcomeRetry || err == nil || Terminal(st.current().Status) {
			t.Errorf("outcome %v err %v job %+v", out, err, st.current())
		}
		if st.current().FailedStage != StatusRendering {
			t.Errorf("FailedStage = %q", st.current().FailedStage)
		}
	})
	t.Run("an unexpected error type is treated as ours and retried", func(t *testing.T) {
		e := newEnv(t)
		st := newStore(e.jobWith(2))
		out, _ := e.runner(st, &fakeRenderer{err: errors.New("segfault")}).Run(context.Background(), "u1", "j1")
		if out != OutcomeRetry {
			t.Errorf("outcome %v", out)
		}
		if msg := st.current().Error; strings.Contains(msg, "segfault") || !strings.Contains(msg, "our side") {
			t.Errorf("the user is shown %q", msg)
		}
	})
	t.Run("a video that cannot be stored is retried", func(t *testing.T) {
		e := newEnv(t)
		e.files.putErr["storyvideo/u1/j1/video.mp4"] = errors.New("s3 down")
		st := newStore(e.jobWith(2))
		out, _ := e.runner(st, &fakeRenderer{}).Run(context.Background(), "u1", "j1")
		got := st.current()
		if out != OutcomeRetry || got.Status == StatusCompleted || got.OutputKey != "" {
			t.Errorf("outcome %v job %+v", out, got)
		}
	})
}

func TestRun_ADeletedJobStopsTheWork(t *testing.T) {
	e := newEnv(t)
	st := newStore(e.jobWith(2))
	rend := &fakeRenderer{block: true, started: make(chan struct{})}
	go func() { <-rend.started; st.mu.Lock(); st.gone = true; st.mu.Unlock() }()
	done := make(chan Outcome, 1)
	go func() { out, _ := e.runner(st, rend).Run(context.Background(), "u1", "j1"); done <- out }()
	select {
	case out := <-done:
		if out != OutcomeCancelled {
			t.Errorf("outcome %v", out)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the worker kept rendering a job that was deleted")
	}
	if len(e.files.puts) > 0 && strings.Contains(strings.Join(e.files.puts, ","), "video.mp4") {
		t.Error("a video was stored for a deleted job")
	}
}

func TestRun_AJobCancelledByTheUserIsNotRevivedOrOverwritten(t *testing.T) {
	e := newEnv(t)
	st := newStore(e.jobWith(2))
	rend := &fakeRenderer{block: true, started: make(chan struct{})}
	go func() {
		<-rend.started
		st.mu.Lock()
		st.job.Status, st.job.Error, st.job.ErrorCode = StatusFailed, "Cancelled.", "CANCELLED"
		st.mu.Unlock()
	}()
	done := make(chan Outcome, 1)
	go func() { out, _ := e.runner(st, rend).Run(context.Background(), "u1", "j1"); done <- out }()
	select {
	case out := <-done:
		if out != OutcomeCancelled {
			t.Errorf("outcome %v", out)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the worker kept going after the job was cancelled")
	}
	if got := st.current(); got.Status != StatusFailed || got.ErrorCode != "CANCELLED" {
		t.Errorf("the cancel was overwritten: %+v", got)
	}
}

// The one that matters most for the state machine: a stop signal is not a success.
func TestRun_AProcessThatIsToldToStopAsksToBeRunAgainAndNeverReportsSuccess(t *testing.T) {
	e := newEnv(t)
	st := newStore(e.jobWith(2))
	rend := &fakeRenderer{block: true, started: make(chan struct{})}
	ctx, cancel := context.WithCancelCause(context.Background())
	go func() { <-rend.started; cancel(errors.New("SIGTERM from ECS")) }()
	out, err := e.runner(st, rend).Run(ctx, "u1", "j1")
	if out != OutcomeRetry {
		t.Errorf("outcome %v: exiting cleanly on SIGTERM would tell Step Functions the job succeeded", out)
	}
	if err == nil {
		t.Error("no reason")
	}
	if got := st.current(); Terminal(got.Status) {
		t.Errorf("a stop marked the job final: %+v", got)
	}
}

func TestRun_ProcessTimeoutIsAlsoARetryNotAFailure(t *testing.T) {
	e := newEnv(t)
	st := newStore(e.jobWith(2))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	out, _ := e.runner(st, &fakeRenderer{block: true}).Run(ctx, "u1", "j1")
	if out != OutcomeRetry || Terminal(st.current().Status) {
		t.Errorf("outcome %v job %+v", out, st.current())
	}
}

func TestRun_StoreProblems(t *testing.T) {
	t.Run("a job that cannot be read is retried", func(t *testing.T) {
		e := newEnv(t)
		st := newStore(e.jobWith(1))
		st.loadErr = errors.New("dynamo throttled")
		if out, err := e.runner(st, &fakeRenderer{}).Run(context.Background(), "u1", "j1"); out != OutcomeRetry || err == nil {
			t.Errorf("outcome %v err %v", out, err)
		}
	})
	t.Run("a job that does not exist is cancelled, not retried forever", func(t *testing.T) {
		e := newEnv(t)
		st := newStore(e.jobWith(1))
		st.gone = true
		if out, _ := e.runner(st, &fakeRenderer{}).Run(context.Background(), "u1", "j1"); out != OutcomeCancelled {
			t.Errorf("outcome %v", out)
		}
	})
	t.Run("a job deleted while saving is cancelled", func(t *testing.T) {
		e := newEnv(t)
		st := newStore(e.jobWith(2))
		st.saveErr = func(n int) error {
			if n == 2 {
				return ErrJobGone
			}
			return nil
		}
		if out, _ := e.runner(st, &fakeRenderer{}).Run(context.Background(), "u1", "j1"); out != OutcomeCancelled {
			t.Errorf("outcome %v", out)
		}
	})
	t.Run("progress writes that fail do not fail the job", func(t *testing.T) {
		e := newEnv(t)
		st := newStore(e.jobWith(3))
		var logged []string
		e.deps.Logf = func(f string, a ...any) { logged = append(logged, fmt.Sprintf(f, a...)) }
		// Every write that only moves a progress bar fails. The writes at the end
		// of a stage (which carry what was made) and the final one go through.
		r := e.runner(st, &fakeRenderer{})
		failing := &flakyStore{memStore: st}
		r.Store = failing
		out, err := r.Run(context.Background(), "u1", "j1")
		if out != OutcomeCompleted || err != nil {
			t.Fatalf("outcome %v err %v: a failed progress write failed the render", out, err)
		}
		if failing.failed == 0 {
			t.Fatal("the test never failed a progress write")
		}
		if !strings.Contains(strings.Join(logged, "\n"), "progress write failed") {
			t.Errorf("the failed writes were not logged: %v", logged)
		}
		if st.current().Status != StatusCompleted {
			t.Errorf("job = %+v", st.current())
		}
	})
	t.Run("a failure that cannot be recorded is not claimed as final", func(t *testing.T) {
		e := newEnv(t)
		job := e.jobWith(2)
		delete(e.assets.assets, "pixabay:photo:101")
		st := newStore(job)
		st.finErr = errors.New("dynamo down")
		out, err := e.runner(st, &fakeRenderer{}).Run(context.Background(), "u1", "j1")
		if out != OutcomeRetry || err == nil || !strings.Contains(err.Error(), "recording the failure") {
			t.Errorf("outcome %v err %v: if the failure cannot be written the task must run again, or the reason is lost", out, err)
		}
	})
}

func TestRun_TheWorkDirectoryIsRemovedWhateverHappens(t *testing.T) {
	for name, setup := range map[string]func(e *env) Renderer{
		"success":      func(e *env) Renderer { return &fakeRenderer{} },
		"failure":      func(e *env) Renderer { return &fakeRenderer{err: errors.New("x")} },
		"user failure": func(e *env) Renderer { return &fakeRenderer{} },
	} {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t)
			rend := setup(e)
			job := e.jobWith(2)
			if name == "user failure" {
				delete(e.assets.assets, "pixabay:photo:101")
			}
			_, _ = e.runner(newStore(job), rend).Run(context.Background(), "u1", "j1")
			if left := listDir(t, e.deps.WorkDir); len(left) != 0 {
				t.Errorf("left behind: %v", left)
			}
		})
	}
}

func TestOutcome_String(t *testing.T) {
	for o, want := range map[Outcome]string{OutcomeCompleted: "completed", OutcomeFailed: "failed", OutcomeRetry: "retry", OutcomeCancelled: "cancelled"} {
		if o.String() != want {
			t.Errorf("%d = %q", o, o.String())
		}
	}
}

// flakyStore fails the writes that only move the progress bar: a Save in the
// same stage as the one before it whose percent has changed. Writes that enter a
// stage, and the write that keeps what a stage made (same percent as the last
// attempt), go through.
type flakyStore struct {
	*memStore
	failed       int
	lastStatus   string
	lastProgress int
}

func (f *flakyStore) Save(ctx context.Context, j *Job) error {
	if j.Status == f.lastStatus && j.Progress != f.lastProgress {
		f.lastProgress = j.Progress
		f.failed++
		return errors.New("throttled")
	}
	f.lastStatus, f.lastProgress = j.Status, j.Progress
	return f.memStore.Save(ctx, j)
}

// errTemporary is a provider failure that a retry may fix.
func errTemporary() error {
	return &stockmedia.ProviderError{Provider: "pixabay", Kind: stockmedia.KindUnavailable}
}

func TestRun_AProgressBarNeverGoesBackwards(t *testing.T) {
	e := newEnv(t)
	st := newStore(e.jobWith(2))
	rend := &steppedRenderer{percents: []int{60, 30, 99, 10}}
	if out, err := e.runner(st, rend).Run(context.Background(), "u1", "j1"); out != OutcomeCompleted {
		t.Fatalf("outcome %v err %v", out, err)
	}
	last := -1
	for _, rec := range st.saves {
		var p int
		fmt.Sscan(strings.Split(rec, ":")[1], &p)
		if p < last {
			t.Fatalf("progress went from %d back to %d: %v", last, p, st.saves)
		}
		last = p
	}
}

// steppedRenderer reports the given percents in order, out of order on purpose.
type steppedRenderer struct{ percents []int }

func (r *steppedRenderer) Render(_ context.Context, _ *Job, _, outPath string, onPercent func(int)) (float64, error) {
	for _, p := range r.percents {
		onPercent(p)
	}
	return 12, os.WriteFile(outPath, []byte("MP4"), 0o600)
}

func TestScale_MapsRenderProgressOntoTheRenderBand(t *testing.T) {
	// FFmpeg's 0 to 99 fills the render stage's band: 40 up to 99.
	for _, tc := range []struct{ pct, want int }{{0, 40}, {99, 99}, {50, 69}, {-5, 40}, {250, 99}} {
		if got := scale(tc.pct, StageProgress[StatusRendering], 99); got != tc.want {
			t.Errorf("scale(%d) = %d, want %d", tc.pct, got, tc.want)
		}
	}
	prev := -1
	for p := 0; p <= 99; p++ {
		got := scale(p, 40, 99)
		if got < prev {
			t.Fatalf("scale is not monotonic at %d", p)
		}
		prev = got
	}
}

func TestRunner_ItsClockDefaultsToTheRealOne(t *testing.T) {
	r := &Runner{}
	if d := time.Since(r.now()); d < 0 || d > time.Minute {
		t.Errorf("the default clock is %v from now", d)
	}
}
