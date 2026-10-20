package thumbnail

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/rahul/indifferent/backend/internal/models"
)

// ObjectStore is the S3 surface the worker needs.
type ObjectStore interface {
	DownloadToFile(ctx context.Context, bucket, key, path string) error
	GetObject(ctx context.Context, bucket, key string) ([]byte, error)
	PutObject(ctx context.Context, bucket, key string, data []byte, contentType string) error
	DeleteObjects(ctx context.Context, bucket string, keys []string) error
	ListKeys(ctx context.Context, bucket, prefix string) ([]string, error)
}

// Pipeline produces thumbnails from a local video. progress is told which
// stage is starting.
type Pipeline interface {
	Run(ctx context.Context, in Input, progress func(Stage)) (*Output, error)
}

// ServicePipeline adapts Service to Pipeline. It copies the service for each
// run so the per-run progress callback never leaks between runs.
type ServicePipeline struct {
	Service Service
}

// Run implements Pipeline.
func (p ServicePipeline) Run(ctx context.Context, in Input, progress func(Stage)) (*Output, error) {
	s := p.Service
	s.Progress = progress
	return s.Run(ctx, in)
}

// Job identifies the video to work on.
type Job struct {
	JobID         string
	UserID        string
	SourceKey     string
	TranscriptKey string  // optional
	Duration      float64 // optional fallback
}

// Worker runs one thumbnail generation end to end: claim the record, fetch the
// video, run the pipeline, save the images and the result. It is what the
// Fargate task executes.
type Worker struct {
	Store    *Store
	Objects  ObjectStore
	Pipeline Pipeline
	Bucket   string
	WorkRoot string // parent of the scratch directory; "" uses the system default

	// JobExists reports whether the Shorts job still exists, so a job deleted
	// before the task started is not given a fresh record.
	JobExists func(ctx context.Context, userID, jobID string) (bool, error)
	// Brand returns the channel name to print on the thumbnails, or "".
	Brand func(ctx context.Context, userID, jobID string) string
	Logf  func(format string, args ...any)
}

func (w *Worker) logf(format string, args ...any) {
	if w.Logf != nil {
		w.Logf(format, args...)
	}
}

// Run does the work. A failure that was recorded on the job (and will show in
// the UI with a Retry button) returns nil. Only a failure to record anything,
// such as DynamoDB being unreachable, comes back as an error so the task exits
// non-zero and the state machine notices.
func (w *Worker) Run(ctx context.Context, job Job) error {
	if w.JobExists != nil {
		ok, err := w.JobExists(ctx, job.UserID, job.JobID)
		if err != nil {
			return fmt.Errorf("check job: %w", err)
		}
		if !ok {
			w.logf("job %s no longer exists; nothing to do", job.JobID)
			return nil
		}
	}

	gen, style, format, err := w.Store.Begin(ctx, job.UserID, job.JobID)
	if errors.Is(err, ErrInFlight) {
		w.logf("job %s already has a thumbnail run in progress; exiting", job.JobID)
		return nil
	}
	if err != nil {
		return err
	}
	w.logf("job %s generation %d style=%q format=%q", job.JobID, gen, style, format)

	// record writes a failure to the job. Writing to a record that has been
	// deleted is not an error: there is nobody left to tell.
	record := func(cause error) error {
		fctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		w.logf("job %s failed: code=%s err=%v", job.JobID, CodeOf(cause), cause)
		if err := w.Store.Fail(fctx, job.UserID, job.JobID, gen, CodeOf(cause), MessageOf(cause)); err != nil && !errors.Is(err, ErrGone) {
			return err
		}
		return nil
	}

	workDir, err := os.MkdirTemp(w.WorkRoot, "thumbnails-*")
	if err != nil {
		return record(Errorf(CodeTaskFailed, "Thumbnail generation could not start.", err))
	}
	defer os.RemoveAll(workDir)

	srcPath := filepath.Join(workDir, "source"+safeExt(job.SourceKey))
	if err := w.Objects.DownloadToFile(ctx, w.Bucket, job.SourceKey, srcPath); err != nil {
		return record(Errorf(CodeSourceUnreadable, "We couldn't download this video.", err))
	}

	transcript := w.loadTranscript(ctx, job)
	brand := ""
	if w.Brand != nil {
		brand = w.Brand(ctx, job.UserID, job.JobID)
	}

	// If the record disappears or is taken over by a newer run, stop early.
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	var gone atomic.Bool
	progress := func(st Stage) {
		err := w.Store.SetStage(runCtx, job.UserID, job.JobID, gen, st)
		switch {
		case errors.Is(err, ErrGone):
			gone.Store(true)
			cancel()
		case err != nil && runCtx.Err() == nil:
			w.logf("could not record stage %s: %v", st, err)
		}
	}

	out, err := w.Pipeline.Run(runCtx, Input{
		VideoPath:  srcPath,
		WorkDir:    workDir,
		Duration:   job.Duration,
		Transcript: transcript,
		Style:      style,
		Format:     format,
		Brand:      brand,
	}, progress)
	if gone.Load() {
		w.logf("job %s thumbnails were removed or replaced during the run; stopping", job.JobID)
		return nil
	}
	if err != nil {
		return record(err)
	}

	assets := make([]Asset, 0, len(out.Images))
	var keys []string
	for i, im := range out.Images {
		key := ObjectKey(job.UserID, job.JobID, gen, i+1)
		if err := w.Objects.PutObject(ctx, w.Bucket, key, im.JPEG, "image/jpeg"); err != nil {
			w.discard(keys)
			return record(Errorf(CodeRenderFailed, "We couldn't save the thumbnails.", err))
		}
		keys = append(keys, key)
		assets = append(assets, Asset{
			ID:            AssetID(i + 1),
			Concept:       im.Concept,
			Style:         out.Style,
			Format:        out.Format,
			Key:           key,
			Headline:      im.Headline,
			SecondaryText: im.SecondaryText,
			FrameTime:     im.FrameTime,
			Width:         im.Width,
			Height:        im.Height,
		})
	}

	err = w.Store.Complete(ctx, job.UserID, job.JobID, gen, Result{
		Thumbnails:     assets,
		Analysis:       out.Analysis,
		Funnel:         out.Funnel,
		Style:          out.Style,
		Format:         out.Format,
		TranscriptUsed: out.TranscriptUsed,
		DetectionUsed:  out.DetectionUsed,
	})
	if errors.Is(err, ErrGone) {
		w.discard(keys)
		return nil
	}
	if err != nil {
		w.discard(keys)
		return err
	}
	w.pruneOlder(ctx, job, gen)
	w.logf("job %s generation %d complete: %d thumbnails", job.JobID, gen, len(assets))
	return nil
}

// loadTranscript reads the transcript text. The transcript only improves the
// headline, so any problem is logged and the run goes on without it.
func (w *Worker) loadTranscript(ctx context.Context, job Job) string {
	if job.TranscriptKey == "" {
		return ""
	}
	data, err := w.Objects.GetObject(ctx, w.Bucket, job.TranscriptKey)
	if err != nil {
		w.logf("transcript unavailable (%s): %v", job.TranscriptKey, err)
		return ""
	}
	var t models.Transcript
	if err := json.Unmarshal(data, &t); err != nil {
		w.logf("transcript unreadable: %v", err)
		return ""
	}
	if text := strings.TrimSpace(t.Text); text != "" {
		return text
	}
	words := make([]string, 0, len(t.Words))
	for _, word := range t.Words {
		words = append(words, word.Text)
	}
	return strings.TrimSpace(strings.Join(words, " "))
}

// discard removes objects that were uploaded for a run that did not complete.
func (w *Worker) discard(keys []string) {
	if len(keys) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := w.Objects.DeleteObjects(ctx, w.Bucket, keys); err != nil {
		w.logf("could not remove %d orphaned thumbnails: %v", len(keys), err)
	}
}

// pruneOlder deletes earlier generations' images once the new set is saved.
func (w *Worker) pruneOlder(ctx context.Context, job Job, gen int) {
	root := fmt.Sprintf("shorts/%s/%s/thumbnails/", job.UserID, job.JobID)
	keep := GenerationPrefix(job.UserID, job.JobID, gen)
	all, err := w.Objects.ListKeys(ctx, w.Bucket, root)
	if err != nil {
		w.logf("could not list old thumbnails: %v", err)
		return
	}
	var old []string
	for _, k := range all {
		if !strings.HasPrefix(k, keep) {
			old = append(old, k)
		}
	}
	if len(old) == 0 {
		return
	}
	if err := w.Objects.DeleteObjects(ctx, w.Bucket, old); err != nil {
		w.logf("could not remove %d old thumbnails: %v", len(old), err)
	}
}

// safeExt keeps a known video extension for the scratch file so ffmpeg and
// ffprobe pick the right demuxer, and drops anything unexpected.
func safeExt(key string) string {
	ext := strings.ToLower(filepath.Ext(key))
	switch ext {
	case ".mp4", ".mov", ".m4v", ".webm", ".mkv", ".mp3", ".wav", ".m4a":
		return ext
	}
	return ".bin"
}
