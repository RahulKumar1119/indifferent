package storyjob

import (
	"context"
	"errors"
	"io"
	"log"
	"time"

	"github.com/rahul/indifferent/backend/internal/stockmedia"
)

// Assets is the stock-media provider layer. *stockmedia.Service satisfies it.
type Assets interface {
	Get(ctx context.Context, provider stockmedia.ProviderName, typ stockmedia.MediaType, id string) (stockmedia.Asset, error)
	Download(ctx context.Context, a stockmedia.Asset, w io.Writer) (int64, error)
}

// Files is the object storage. *storage.S3Client satisfies it.
type Files interface {
	PutFile(ctx context.Context, bucket, key, path, contentType string) error
	PutObject(ctx context.Context, bucket, key string, data []byte, contentType string) error
	HeadObject(ctx context.Context, bucket, key string) (bool, error)
	DownloadToFile(ctx context.Context, bucket, key, path string) error
}

// Narrator makes the voice. *narrator.PollyNarrationService satisfies it.
type Narrator interface {
	SynthesizeText(ctx context.Context, text string, voiceID string) ([]byte, error)
	SynthesizeMarks(ctx context.Context, text string, voiceID string) ([]byte, error)
}

// MediaInfo is what the prober learns about a file.
type MediaInfo struct {
	Width, Height int
	// Duration is in seconds; 0 for a still picture.
	Duration float64
	// HasVideo is true when the file has a picture stream. A still image counts.
	HasVideo bool
	// HasAudio is true when the file has a sound stream.
	HasAudio bool
}

// Prober reads the real properties of a file. FFprobe satisfies it. The
// properties are measured, not taken from the provider: a mislabelled file
// must not reach the renderer.
type Prober interface {
	Probe(ctx context.Context, path string) (MediaInfo, error)
}

// Deps are what the steps need from the outside world.
type Deps struct {
	Assets   Assets
	Files    Files
	Narrator Narrator
	Prober   Prober
	// Bucket is the assets bucket working files and the finished video go to.
	Bucket string
	// WorkDir is a scratch directory with room for the largest stock clip.
	WorkDir string
	// Retry governs the download of a stock file. The zero value never retries.
	// Provider lookups are retried inside Assets (see stockmedia.WithRetry).
	Retry stockmedia.RetryPolicy
	// ThrottleWaits are further waits, in order, for a file host that is still
	// answering 429 (stockmedia.ErrThrottled) after a whole round of Retry. Such
	// throttling lasts minutes, longer than Retry waits, and a failed scene
	// costs a task restart. Nil means no further waits.
	ThrottleWaits []time.Duration
	// ThrottleBudget caps the total of those waits over one CopyMedia call, so
	// several throttled scenes cannot use up the worker's deadline. Zero means
	// no cap beyond ThrottleWaits.
	ThrottleBudget time.Duration
	// Sleep waits for d or until ctx ends. Nil means a real timer. A seam for tests.
	Sleep func(ctx context.Context, d time.Duration) error
	// SaveProgress persists the job as it stands. CopyMedia calls it after each
	// scene is copied, so a task that is restarted (a Spot interruption, or a
	// retry after a throttled scene) skips the scenes already in storage instead
	// of downloading every scene again and re-hitting the same throttle. Unlike
	// the progress percent, which may not move between two scenes and so may not
	// write, this writes once per copied scene. Nil means progress is not saved
	// mid-step. A seam the runner fills in.
	//
	// It may return ErrJobGone or errStoppedElsewhere when the user has ended the
	// job; CopyMedia then stops, so the worker does not keep copying for a job
	// nobody is waiting on. Any other save error is not fatal (the copy already
	// exists in storage, it is only not yet recorded), so it is logged and the
	// step goes on.
	SaveProgress func(ctx context.Context) error
	// Logf receives progress notes. It defaults to log.Printf.
	Logf func(format string, args ...any)
}

// saveProgress persists the job after a scene is copied. It returns an error
// only when the job has ended and the step should stop; an ordinary save
// failure is logged and swallowed.
func (d Deps) saveProgress(ctx context.Context) error {
	if d.SaveProgress == nil {
		return nil
	}
	err := d.SaveProgress(ctx)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, ErrJobGone), errors.Is(err, errStoppedElsewhere):
		return err
	default:
		d.logf("storyjob save progress failed: %v", err)
		return nil
	}
}

func (d Deps) sleep(ctx context.Context, wait time.Duration) error {
	if d.Sleep != nil {
		return d.Sleep(ctx, wait)
	}
	t := time.NewTimer(wait)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func (d Deps) logf(format string, args ...any) {
	if d.Logf != nil {
		d.Logf(format, args...)
		return
	}
	log.Printf(format, args...)
}

// Progress is told how many scenes of a step are done. It may be nil.
type Progress func(done, total int)

func (p Progress) report(done, total int) {
	if p != nil {
		p(done, total)
	}
}
