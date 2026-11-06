package storyjob

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rahul/indifferent/backend/internal/stockmedia"
	"github.com/rahul/indifferent/backend/internal/storyplan"
)

// extensions maps the formats the stock layer accepts to a file extension.
var extensions = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
	"video/mp4":  ".mp4",
	"video/webm": ".webm",
}

// CopyMedia copies each scene's chosen stock asset into our storage and records
// where it went in the job.
//
// This is what makes a render independent of the provider: after it, no step
// needs a provider URL, which may be signed, may expire or may change. It also
// means a render that is interrupted and started again does not download
// anything a second time.
//
// For each scene it asks the provider for the asset as it is now, downloads the
// file under the stock layer's checks, measures the file itself (a provider's
// label is not trusted), and uploads it. A scene that already has a copy in
// storage is skipped.
//
// The first scene that cannot be copied stops the step with a *StepError: gone
// or unusable assets are not retryable and say which scene to change; a
// provider that did not answer is retryable. Nothing waits on a provider
// forever: each download has its own deadline.
//
// With d.Parallel above one, scenes are copied side by side. The first failure
// stops new scenes starting, but scenes already under way finish and are
// recorded, so a restarted task skips them. The first 429 from a file host also
// stops new scenes starting side by side: the scenes still to do then run one at
// a time through the throttle waits (see download), as they did before scenes
// ran in parallel. Scenes already under way when the 429 arrives still finish,
// so a throttle costs at most d.Parallel-1 extra downloads.
func CopyMedia(ctx context.Context, d Deps, job *Job, progress Progress) error {
	total := len(job.Scenes)
	done := 0

	// Scenes already in storage are skipped first; a HeadObject is cheap.
	var todo []int
	for i := range job.Scenes {
		if err := ctx.Err(); err != nil {
			return err
		}
		s := &job.Scenes[i]
		if s.Media != nil {
			if have, err := d.Files.HeadObject(ctx, d.Bucket, s.Media.Key); err == nil && have {
				d.logf("storyjob scene=%d media=already_copied", i+1)
				done++
				progress.report(done, total)
				continue
			}
		}
		todo = append(todo, i)
	}

	st := &copyState{left: d.ThrottleBudget}
	// mu lets one scene at a time record its copy, save the job and report
	// progress. SaveProgress and the progress callback write the whole job, and
	// neither may see another scene half recorded.
	var mu sync.Mutex
	copyScene := func(ctx context.Context, i int) error {
		media, err := copyOne(ctx, d, job, i+1, job.Scenes[i].Pick, st)
		if err != nil {
			return err
		}
		mu.Lock()
		defer mu.Unlock()
		job.Scenes[i].Media = media
		// Record the copy before moving on. A task that is restarted after a
		// later scene is throttled then skips this one (the already_copied path
		// above) instead of downloading it again and re-hitting the throttle.
		if err := d.saveProgress(ctx); err != nil {
			return err
		}
		done++
		progress.report(done, total)
		return nil
	}

	errs, left := runParallel(ctx, d.workers(), todo, st.isThrottled, copyScene)
	if err := firstError(errs); err != nil {
		return err
	}
	if len(left) == 0 {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	// A download met a throttle, so the scenes not yet started go one at a time.
	d.logf("storyjob media=serial_after_throttle remaining=%d", len(left))
	errs, left = runParallel(ctx, 1, left, nil, copyScene)
	if err := firstError(errs); err != nil {
		return err
	}
	if len(left) > 0 {
		return ctx.Err()
	}
	return nil
}

// copyState is what the scenes of one CopyMedia call share.
type copyState struct {
	mu   sync.Mutex
	left time.Duration // what is left of Deps.ThrottleBudget
	// throttled is set when any download sees a 429. From then on no new scene
	// is started side by side with another.
	throttled atomic.Bool
}

func (c *copyState) isThrottled() bool { return c.throttled.Load() }

// spend takes wait from the throttle allowance, or reports that there is not
// enough left. Scenes in parallel draw on one allowance, so several throttled
// scenes cannot between them wait longer than the budget.
func (c *copyState) spend(wait time.Duration) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if wait > c.left {
		return false
	}
	c.left -= wait
	return true
}

func copyOne(ctx context.Context, d Deps, job *Job, n int, pick storyplan.MediaRef, st *copyState) (*Media, error) {
	provider := pick.Provider
	asset, err := d.Assets.Get(ctx, stockmedia.ProviderName(pick.Provider), stockmedia.MediaType(pick.Type), pick.ID)
	if err != nil {
		return nil, mediaError(n, provider, err)
	}
	ext, ok := extensions[asset.Format]
	if !ok {
		return nil, mediaError(n, provider, fmt.Errorf("%w: format %q", stockmedia.ErrInvalidAsset, asset.Format))
	}

	local := filepath.Join(d.WorkDir, fmt.Sprintf("scene-%02d-media%s", n, ext))
	defer os.Remove(local)
	if err := download(ctx, d, n, asset, local, st); err != nil {
		return nil, mediaError(n, provider, err)
	}

	info, err := d.Prober.Probe(ctx, local)
	if err != nil || !info.HasVideo || info.Width <= 0 || info.Height <= 0 {
		// The provider said this was a picture and it is not one we can read.
		return nil, mediaError(n, provider, fmt.Errorf("%w: the file cannot be read as %s: %v", stockmedia.ErrInvalidAsset, asset.Type, err))
	}
	if asset.Type == stockmedia.MediaVideo && info.Duration <= 0 {
		return nil, mediaError(n, provider, fmt.Errorf("%w: the clip has no length", stockmedia.ErrInvalidAsset))
	}

	key := WorkKey(job.UserID, job.JobID, n, "media"+ext)
	if err := d.Files.PutFile(ctx, d.Bucket, key, local, asset.Format); err != nil {
		return nil, internalError(n, fmt.Errorf("storing the copy: %w", err))
	}
	return &Media{
		Key: key, Kind: string(asset.Type), Format: asset.Format,
		Width: info.Width, Height: info.Height,
		Credit: asset.Credit, PageURL: asset.PageURL,
	}, nil
}

// download fetches the asset's file to path, trying again under d.Retry when the
// provider's CDN fails or the transfer breaks off. Only that kind of failure is
// retried: a file that is gone, invalid, or too big would fail the same way
// again, and a full scratch disk is ours. A retry starts the file over, so a
// partial first attempt is never kept.
//
// A failed scene would otherwise end the whole task, and the state machine
// would start it again from the top after a wait. Retrying here costs seconds
// instead of a container start.
//
// A file host that is still throttling (429) after that round is given the
// longer d.ThrottleWaits, one after another, each followed by another round,
// while the job's remaining allowance (st, when d.ThrottleBudget is set) covers
// the wait. On 2026-11-04 Pixabay's photo host answered 429 to the
// worker for about three minutes; the task's own retries were over in seconds
// and the state machine's in under three minutes, and a try five minutes later
// worked.
func download(ctx context.Context, d Deps, scene int, asset stockmedia.Asset, path string, st *copyState) error {
	for i := 0; ; i++ {
		err := d.Retry.Do(ctx, func(ctx context.Context, _ int) error {
			return downloadOnce(ctx, d, asset, path)
		}, func(attempt int, wait time.Duration, err error) {
			// Say so before logging, so that whoever sees the line can rely on it.
			if errors.Is(err, stockmedia.ErrThrottled) {
				st.throttled.Store(true)
			}
			d.logf("storyjob scene=%d media=download_retry attempt=%d wait_ms=%d: %v", scene, attempt, wait.Milliseconds(), err)
		})
		if err == nil || !errors.Is(err, stockmedia.ErrThrottled) {
			return err
		}
		st.throttled.Store(true)
		if i >= len(d.ThrottleWaits) {
			return err
		}
		wait := d.ThrottleWaits[i]
		if d.ThrottleBudget > 0 && !st.spend(wait) {
			d.logf("storyjob scene=%d media=throttled no_wait_left: %v", scene, err)
			return err
		}
		d.logf("storyjob scene=%d media=throttled wait_s=%d: %v", scene, int(wait.Seconds()), err)
		if serr := d.sleep(ctx, wait); serr != nil {
			return serr
		}
	}
}

func downloadOnce(ctx context.Context, d Deps, asset stockmedia.Asset, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("%w: %v", errScratch, err)
	}
	_, err = d.Assets.Download(ctx, asset, f)
	if cerr := f.Close(); err == nil && cerr != nil {
		err = fmt.Errorf("%w: %v", errScratch, cerr)
	}
	if err != nil {
		os.Remove(path)
	}
	return err
}

// errScratch marks a failure of the worker's own disk, which is ours, not the
// provider's.
var errScratch = errors.New("scratch disk")
