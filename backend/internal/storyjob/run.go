package storyjob

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Store reads and writes the job record. The worker is the only writer while a
// job is running, so every write is a whole-field update, not a read-modify-
// write of the record.
type Store interface {
	// Load reads the job. ErrJobGone means it no longer exists (the user
	// deleted it).
	Load(ctx context.Context, userID, jobID string) (*Job, error)
	// Save writes the job's progress: its status, percent, scenes (the copies
	// and the narration made so far) and length. It fails with ErrJobGone if
	// the job was deleted, and never creates a record.
	Save(ctx context.Context, job *Job) error
	// Finish records the end of the job: completed with its output, or failed
	// with a message and a code. It fails with ErrJobGone if the job was
	// deleted.
	Finish(ctx context.Context, job *Job) error
	// Status reads only the job's status, with a consistent read, for noticing
	// a cancel.
	Status(ctx context.Context, userID, jobID string) (string, error)
}

// ErrJobGone means the job record no longer exists.
var ErrJobGone = errors.New("the job no longer exists")

// errStoppedElsewhere means the job reached an end state by another route (the
// user cancelled it), so there is nothing left for this run to do.
var errStoppedElsewhere = errors.New("the job was ended elsewhere")

// Renderer turns a job whose inputs are prepared into a finished video.
type Renderer interface {
	// Render writes the video to outPath, calling onPercent (0 to 99) as it
	// goes, and returns the video's checked length in seconds.
	Render(ctx context.Context, job *Job, workDir, outPath string, onPercent func(int)) (float64, error)
}

// Outcome says how Run ended, which is what the process exit code is chosen from.
type Outcome int

const (
	// OutcomeCompleted: the video is made and uploaded. Exit 0.
	OutcomeCompleted Outcome = iota
	// OutcomeFailed: the job failed for a reason that running it again will not
	// fix, and the reason is on the record. Exit 0, so Step Functions does not
	// retry it; the state machine reads the record.
	OutcomeFailed
	// OutcomeRetry: a temporary failure. The job is left as it was so that the
	// next run resumes it, and the reason is on the record if retries run out.
	// Exit non-zero.
	OutcomeRetry
	// OutcomeCancelled: the user deleted or cancelled the job. Exit 0: there is
	// nothing to retry and nothing to report.
	OutcomeCancelled
)

func (o Outcome) String() string {
	return [...]string{"completed", "failed", "retry", "cancelled"}[o]
}

// Runner runs a job from start to finish.
type Runner struct {
	Deps     Deps
	Store    Store
	Renderer Renderer
	// Now is the clock; time.Now when nil.
	Now func() time.Time
	// CancelCheck is how often the job record is read to notice a cancel. Zero
	// means every 10 seconds.
	CancelCheck time.Duration
}

func (r *Runner) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now()
}

// Run makes the video for a job. It is safe to run again on the same job: the
// copy and narration steps skip what is already in storage, and a job that is
// already finished is left alone.
//
// It returns the Outcome and, when the outcome is anything but completed, the
// reason. The reason is also on the job record, in words fit for the user.
func (r *Runner) Run(ctx context.Context, userID, jobID string) (Outcome, error) {
	job, err := r.Store.Load(ctx, userID, jobID)
	if err != nil {
		if errors.Is(err, ErrJobGone) {
			return OutcomeCancelled, err
		}
		return OutcomeRetry, fmt.Errorf("loading the job: %w", err)
	}
	if Terminal(job.Status) {
		r.Deps.logf("storyjob job=%s status=%s nothing to do", jobID, job.Status)
		if job.Status == StatusFailed {
			return OutcomeFailed, errors.New(job.Error)
		}
		return OutcomeCompleted, nil
	}

	// A reason saved by an earlier attempt that asked to be retried belongs to
	// that attempt. This one starts clean, so the user does not see an error on
	// a job that is running.
	job.Error, job.ErrorCode, job.FailedStage = "", "", ""

	// A cancel only writes the record. Watch it, and stop the work when the job
	// is deleted or no longer running.
	ctx, stop := context.WithCancelCause(ctx)
	defer stop(nil)
	go r.watch(ctx, userID, jobID, stop)

	if err := os.MkdirAll(r.Deps.WorkDir, 0o700); err != nil {
		return r.fail(ctx, job, internalError(0, fmt.Errorf("preparing the work directory: %w", err)))
	}
	workDir, err := os.MkdirTemp(r.Deps.WorkDir, "job-*")
	if err != nil {
		return r.fail(ctx, job, internalError(0, err))
	}
	defer os.RemoveAll(workDir)
	deps := r.Deps
	deps.WorkDir = workDir

	timings := map[string]time.Duration{}
	steps := []struct {
		status string
		run    func() error
	}{
		{StatusPreparing, func() error {
			return CopyMedia(ctx, deps, job, r.stepProgress(ctx, job, StatusPreparing, StatusNarrating, stop))
		}},
		{StatusNarrating, func() error {
			return NarrateScenes(ctx, deps, job, r.stepProgress(ctx, job, StatusNarrating, StatusRendering, stop))
		}},
	}
	for _, step := range steps {
		if err := r.enter(ctx, job, step.status); err != nil {
			return r.end(ctx, job, err)
		}
		began := time.Now()
		if err := step.run(); err != nil {
			return r.end(ctx, job, err)
		}
		timings[step.status] = time.Since(began)
		// Keep what was made, so a restart picks up here.
		if err := r.Store.Save(ctx, job); err != nil {
			return r.end(ctx, job, err)
		}
	}

	if err := r.enter(ctx, job, StatusRendering); err != nil {
		return r.end(ctx, job, err)
	}
	outPath := filepath.Join(workDir, "video.mp4")
	began := time.Now()
	seconds, err := r.Renderer.Render(ctx, job, workDir, outPath, func(pct int) {
		r.progress(ctx, job, scale(pct, StageProgress[StatusRendering], 99), stop)
	})
	if err != nil {
		return r.end(ctx, job, err)
	}
	timings[StatusRendering] = time.Since(began)
	key := OutputKey(job.UserID, job.JobID)
	began = time.Now()
	if err := deps.Files.PutFile(ctx, deps.Bucket, key, outPath, "video/mp4"); err != nil {
		return r.end(ctx, job, internalError(0, fmt.Errorf("storing the video: %w", err)))
	}

	uploaded := time.Since(began)
	var size int64
	if fi, err := os.Stat(outPath); err == nil {
		size = fi.Size()
	}
	job.Status, job.Progress, job.OutputKey, job.TotalSeconds = StatusCompleted, StageProgress[StatusCompleted], key, seconds
	job.Error, job.ErrorCode, job.FailedStage = "", "", ""
	job.UpdatedAt = r.now().UTC().Format(time.RFC3339)
	job.CompletedAt = job.UpdatedAt
	if err := r.Store.Finish(ctx, job); err != nil {
		return r.end(ctx, job, err)
	}
	// One line with every number needed to size and cost a render. It is how the
	// real time on Fargate is measured: nothing here has been run there yet.
	r.Deps.logf("storyjob_timing job=%s scenes=%d prepare=%.1fs narrate=%.1fs render=%.1fs upload=%.1fs video_seconds=%.1f output_bytes=%d",
		jobID, len(job.Scenes), timings[StatusPreparing].Seconds(), timings[StatusNarrating].Seconds(),
		timings[StatusRendering].Seconds(), uploaded.Seconds(), seconds, size)
	return OutcomeCompleted, nil
}

// enter moves the job into a stage.
func (r *Runner) enter(ctx context.Context, job *Job, status string) error {
	job.Status = status
	job.Progress = max(job.Progress, StageProgress[status])
	job.UpdatedAt = r.now().UTC().Format(time.RFC3339)
	return r.Store.Save(ctx, job)
}

// stepProgress reports "done of total" scenes as progress between this stage's
// start and the next one's.
func (r *Runner) stepProgress(ctx context.Context, job *Job, from, to string, stop context.CancelCauseFunc) Progress {
	return func(done, total int) {
		if total <= 0 {
			return
		}
		lo, hi := StageProgress[from], StageProgress[to]
		r.progress(ctx, job, lo+(hi-lo)*done/total, stop)
	}
}

// progress records a percent. Progress only moves forward. A failed write is
// only logged: a progress bar is never worth failing a render for. The one
// exception is learning that the job is gone or ended, which stops the run, so
// the worker does not keep spending on a job nobody is waiting for.
func (r *Runner) progress(ctx context.Context, job *Job, pct int, stop context.CancelCauseFunc) {
	if pct <= job.Progress {
		return
	}
	job.Progress = pct
	job.UpdatedAt = r.now().UTC().Format(time.RFC3339)
	err := r.Store.Save(ctx, job)
	switch {
	case err == nil || ctx.Err() != nil:
	case errors.Is(err, ErrJobGone), errors.Is(err, errStoppedElsewhere):
		stop(err)
	default:
		r.Deps.logf("storyjob job=%s progress write failed: %v", job.JobID, err)
	}
}

// end turns an error from a step into an Outcome and records it.
func (r *Runner) end(ctx context.Context, job *Job, err error) (Outcome, error) {
	if errors.Is(err, ErrJobGone) {
		return OutcomeCancelled, err
	}
	if ctx.Err() != nil {
		cause := context.Cause(ctx)
		r.Deps.logf("storyjob job=%s stopped: %v", job.JobID, cause)
		if errors.Is(cause, ErrJobGone) || errors.Is(cause, errStoppedElsewhere) {
			// The user ended the job. Nothing to retry.
			return OutcomeCancelled, cause
		}
		// The process was told to stop (a Spot interruption, a deploy) or ran
		// out of time. The job is not finished and not failed: ask to be run
		// again. Exiting cleanly here would tell Step Functions it succeeded.
		return OutcomeRetry, cause
	}
	return r.fail(ctx, job, err)
}

// fail records a failure. A failure the user must fix is final: the job is
// marked failed with its message. A temporary one leaves the job to be run
// again, and keeps the reason in case the retries run out.
func (r *Runner) fail(ctx context.Context, job *Job, err error) (Outcome, error) {
	var se *StepError
	if !errors.As(err, &se) {
		se = internalError(0, err).(*StepError)
	}
	// Use a fresh context: the one we have may be the reason we are here.
	wctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
	defer cancel()

	job.Error, job.ErrorCode, job.FailedStage = se.Message, se.Code, job.Status
	job.UpdatedAt = r.now().UTC().Format(time.RFC3339)
	r.Deps.logf("storyjob job=%s failed stage=%s code=%s retryable=%v: %v", job.JobID, job.Status, se.Code, se.Retryable, err)

	if se.Retryable {
		// Not final: the status stays at the stage that failed. The reason is
		// saved so that, if the state machine gives up, the user sees it.
		if serr := r.Store.Save(wctx, job); serr != nil && !errors.Is(serr, ErrJobGone) {
			r.Deps.logf("storyjob job=%s could not save the retry reason: %v", job.JobID, serr)
		}
		return OutcomeRetry, err
	}
	job.Status = StatusFailed
	if ferr := r.Store.Finish(wctx, job); ferr != nil {
		if errors.Is(ferr, ErrJobGone) {
			return OutcomeCancelled, ferr
		}
		// We could not record a final failure. Do not claim one: ask to be
		// run again, so the reason is not lost.
		return OutcomeRetry, fmt.Errorf("recording the failure: %w (the failure: %v)", ferr, err)
	}
	return OutcomeFailed, err
}

// watch reads the job's status now and then and stops the work if the job is
// gone or is no longer running.
func (r *Runner) watch(ctx context.Context, userID, jobID string, stop context.CancelCauseFunc) {
	every := r.CancelCheck
	if every <= 0 {
		every = 10 * time.Second
	}
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			status, err := r.Store.Status(ctx, userID, jobID)
			switch {
			case errors.Is(err, ErrJobGone):
				stop(ErrJobGone)
				return
			case err != nil:
				// A failed read is not a cancel.
			case Terminal(status):
				stop(fmt.Errorf("%w: it is %s", errStoppedElsewhere, status))
				return
			}
		}
	}
}

// scale maps pct (0 to 99) onto the range from lo to hi.
func scale(pct, lo, hi int) int {
	if pct < 0 {
		pct = 0
	}
	if pct > 99 {
		pct = 99
	}
	return lo + (hi-lo)*pct/99
}
