package storyjob

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

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
func CopyMedia(ctx context.Context, d Deps, job *Job, progress Progress) error {
	total := len(job.Scenes)
	for i := range job.Scenes {
		n := i + 1
		s := &job.Scenes[i]
		if err := ctx.Err(); err != nil {
			return err
		}
		if s.Media != nil {
			if have, err := d.Files.HeadObject(ctx, d.Bucket, s.Media.Key); err == nil && have {
				d.logf("storyjob scene=%d media=already_copied", n)
				progress.report(n, total)
				continue
			}
		}
		media, err := copyOne(ctx, d, job, n, s.Pick)
		if err != nil {
			return err
		}
		s.Media = media
		progress.report(n, total)
	}
	return nil
}

func copyOne(ctx context.Context, d Deps, job *Job, n int, pick storyplan.MediaRef) (*Media, error) {
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
	if err := download(ctx, d, asset, local); err != nil {
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

func download(ctx context.Context, d Deps, asset stockmedia.Asset, path string) error {
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
