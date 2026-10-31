package storyjob

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/rahul/indifferent/backend/internal/stockmedia"
)

func TestCopyMedia_CopiesEverySceneIntoOurBucketInPlanOrder(t *testing.T) {
	e := newEnv(t)
	job := e.jobWith(3)
	var seen []string
	err := CopyMedia(context.Background(), e.deps, job, func(done, total int) { seen = append(seen, fmt.Sprintf("%d/%d", done, total)) })
	if err != nil {
		t.Fatal(err)
	}
	for i, s := range job.Scenes {
		wantKey := fmt.Sprintf("storyvideo-work/u1/j1/scene-%02d-media.jpg", i+1)
		if s.Media == nil || s.Media.Key != wantKey {
			t.Fatalf("scene %d media = %+v, want key %s", i+1, s.Media, wantKey)
		}
		if !e.files.has("bucket", wantKey) {
			t.Errorf("scene %d: %s is not in the bucket", i+1, wantKey)
		}
		if got := string(e.files.files["bucket/"+wantKey]); got != "bytes-of-pixabay:photo:"+fmt.Sprint(101+i) {
			t.Errorf("scene %d: the stored file is %q, not that scene's picture", i+1, got)
		}
		if s.Media.Width != 1280 || s.Media.Height != 1920 || s.Media.Kind != "photo" || s.Media.Format != "image/jpeg" {
			t.Errorf("scene %d media = %+v", i+1, s.Media)
		}
	}
	if strings.Join(seen, " ") != "1/3 2/3 3/3" {
		t.Errorf("progress = %v", seen)
	}
}

func TestCopyMedia_KeepsTheCreditAndNeverStoresAProviderURL(t *testing.T) {
	e := newEnv(t)
	job := e.jobWith(1)
	if err := CopyMedia(context.Background(), e.deps, job, nil); err != nil {
		t.Fatal(err)
	}
	m := job.Scenes[0].Media
	if m.Credit == "" || m.PageURL != "https://example/101" {
		t.Errorf("attribution lost: %+v", m)
	}
	item, err := ToItem(*job)
	if err != nil {
		t.Fatal(err)
	}
	// MediaURL is the provider's signed, temporary link: it must not reach the database.
	for name, v := range item {
		if strings.Contains(fmt.Sprint(v), "should-never-be-stored") {
			t.Errorf("a provider URL reached the stored job in %q", name)
		}
	}
}

func TestCopyMedia_VideoAndPhotoKeepTheirOwnExtensions(t *testing.T) {
	e := newEnv(t)
	job := e.jobWith(2)
	e.assets.add(stockmedia.ProviderPexels, stockmedia.MediaVideo, "900")
	job.Scenes[1].Pick.Provider, job.Scenes[1].Pick.Type, job.Scenes[1].Pick.ID = "pexels", "video", "900"
	if err := CopyMedia(context.Background(), e.deps, job, nil); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(job.Scenes[0].Media.Key, "media.jpg") || !strings.HasSuffix(job.Scenes[1].Media.Key, "media.mp4") {
		t.Errorf("keys = %s, %s", job.Scenes[0].Media.Key, job.Scenes[1].Media.Key)
	}
	if job.Scenes[1].Media.Kind != "video" {
		t.Errorf("kind = %s", job.Scenes[1].Media.Kind)
	}
}

func TestCopyMedia_UsesTheFilesRealSizeNotTheProvidersLabel(t *testing.T) {
	e := newEnv(t)
	job := e.jobWith(1)
	e.prober.info = func(string, string) (MediaInfo, error) {
		return MediaInfo{Width: 1920, Height: 1080, HasVideo: true}, nil // provider said 1280x1920
	}
	if err := CopyMedia(context.Background(), e.deps, job, nil); err != nil {
		t.Fatal(err)
	}
	if m := job.Scenes[0].Media; m.Width != 1920 || m.Height != 1080 {
		t.Errorf("media = %+v, want the measured 1920x1080 (the framing depends on it)", m)
	}
}

func TestCopyMedia_SkipsWhatIsAlreadyCopiedSoARestartedJobDownloadsNothingTwice(t *testing.T) {
	e := newEnv(t)
	job := e.jobWith(3)
	if err := CopyMedia(context.Background(), e.deps, job, nil); err != nil {
		t.Fatal(err)
	}
	first := len(e.assets.downloads)
	// The worker is interrupted and started again with the saved job.
	if err := CopyMedia(context.Background(), e.deps, job, nil); err != nil {
		t.Fatal(err)
	}
	if len(e.assets.downloads) != first || len(e.assets.gets) != 3 {
		t.Errorf("a second run made %d more downloads and %d more provider lookups", len(e.assets.downloads)-first, len(e.assets.gets)-3)
	}
}

func TestCopyMedia_RedoesACopyThatIsRecordedButMissingFromTheBucket(t *testing.T) {
	e := newEnv(t)
	job := e.jobWith(2)
	if err := CopyMedia(context.Background(), e.deps, job, nil); err != nil {
		t.Fatal(err)
	}
	delete(e.files.files, "bucket/"+job.Scenes[1].Media.Key) // lifecycle rule, or a manual delete
	before := len(e.assets.downloads)
	if err := CopyMedia(context.Background(), e.deps, job, nil); err != nil {
		t.Fatal(err)
	}
	if len(e.assets.downloads)-before != 1 || !e.files.has("bucket", job.Scenes[1].Media.Key) {
		t.Errorf("the missing copy was not remade (%d new downloads)", len(e.assets.downloads)-before)
	}
}

// The acceptance criterion: a job whose media is unavailable at render time
// fails with a clear reason and does not hang.
func TestCopyMedia_FailuresSayWhichSceneAndWhatToDo(t *testing.T) {
	cases := []struct {
		name      string
		setup     func(e *env)
		code      string
		retryable bool
		mention   []string
	}{
		{"the provider removed the picture", func(e *env) { delete(e.assets.assets, "pixabay:photo:102") },
			CodeMediaUnavailable, false, []string{"Scene 2", "no longer available", "Pixabay", "Choose another"}},
		{"the lookup says it is gone", func(e *env) { e.assets.getErr["pixabay:photo:102"] = fmt.Errorf("%w: gone", stockmedia.ErrNotFound) },
			CodeMediaUnavailable, false, []string{"Scene 2", "Choose another"}},
		{"the file is gone from the CDN", func(e *env) {
			e.assets.dlErr["pixabay:photo:102"] = fmt.Errorf("%w: HTTP 404", stockmedia.ErrAssetUnavailable)
		},
			CodeMediaUnavailable, false, []string{"Scene 2", "Choose another"}},
		{"the asset fails validation", func(e *env) {
			e.assets.dlErr["pixabay:photo:102"] = fmt.Errorf("%w: wrong host", stockmedia.ErrInvalidAsset)
		},
			CodeMediaInvalid, false, []string{"Scene 2", "can't be used", "Choose another"}},
		{"the provider is down", func(e *env) {
			e.assets.getErr["pixabay:photo:102"] = &stockmedia.ProviderError{Provider: "pixabay", Kind: stockmedia.KindUnavailable}
		}, CodeMediaTemporary, true, []string{"Scene 2", "did not answer", "Try again"}},
		{"the provider rate limits us", func(e *env) {
			e.assets.getErr["pixabay:photo:102"] = &stockmedia.ProviderError{Provider: "pixabay", Kind: stockmedia.KindRateLimited}
		}, CodeMediaTemporary, true, []string{"Scene 2", "Try again"}},
		{"the CDN fails with a server error", func(e *env) {
			e.assets.dlErr["pixabay:photo:102"] = fmt.Errorf("%w: %w: HTTP 503", stockmedia.ErrAssetUnavailable, stockmedia.ErrUnavailable)
		}, CodeMediaTemporary, true, []string{"Scene 2", "Try again"}},
		{"the download times out", func(e *env) { e.assets.dlErr["pixabay:photo:102"] = context.DeadlineExceeded },
			CodeMediaTemporary, true, []string{"Scene 2"}},
		{"the file is not a readable picture", func(e *env) {
			e.assets.dlBytes["pixabay:photo:102"] = "<html>not an image</html>"
			e.prober.info = func(_ string, c string) (MediaInfo, error) {
				if strings.HasPrefix(c, "<html>") {
					return MediaInfo{}, errors.New("invalid data found")
				}
				return MediaInfo{Width: 1280, Height: 1920, HasVideo: true}, nil
			}
		}, CodeMediaInvalid, false, []string{"Scene 2", "can't be used"}},
		{"the storage write fails", func(e *env) { e.files.putErr["storyvideo-work/u1/j1/scene-02-media.jpg"] = errors.New("s3 down") },
			CodeInternal, true, []string{"Scene 2", "our side"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newEnv(t)
			job := e.jobWith(3)
			c.setup(e)
			err := CopyMedia(context.Background(), e.deps, job, nil)
			se := stepErr(t, err)
			if se.Scene != 2 || se.Code != c.code || se.Retryable != c.retryable {
				t.Errorf("got scene %d code %s retryable %v; want scene 2 code %s retryable %v", se.Scene, se.Code, se.Retryable, c.code, c.retryable)
			}
			for _, want := range c.mention {
				if !strings.Contains(se.Message, want) {
					t.Errorf("message %q lacks %q", se.Message, want)
				}
			}
			if strings.Contains(se.Message, "http") || strings.Contains(se.Message, "invalid") {
				t.Errorf("the message leaks internals: %q", se.Message)
			}
			if IsRetryable(err) != c.retryable {
				t.Errorf("IsRetryable = %v", IsRetryable(err))
			}
			// Scene 1 was fine and is kept; scene 3 was never reached.
			if job.Scenes[0].Media == nil || job.Scenes[2].Media != nil {
				t.Errorf("progress before the failure was not kept as it should be: %+v", job.Scenes)
			}
		})
	}
}

func TestCopyMedia_AFullScratchDiskIsOursNotTheProvidersAndIsNeverBlamedOnThePicture(t *testing.T) {
	e := newEnv(t)
	job := e.jobWith(2)
	e.deps.WorkDir = "/no/such/directory"
	se := stepErr(t, CopyMedia(context.Background(), e.deps, job, nil))
	if se.Code != CodeInternal || !se.Retryable || strings.Contains(se.Message, "picture") {
		t.Errorf("got %+v", se)
	}
}

func TestCopyMedia_StopsAtOnceWhenCancelledAndRemovesItsScratchFiles(t *testing.T) {
	e := newEnv(t)
	job := e.jobWith(3)
	ctx, cancel := context.WithCancel(context.Background())
	e.prober.info = func(string, string) (MediaInfo, error) {
		cancel() // the worker is told to stop during the first scene
		return MediaInfo{Width: 1280, Height: 1920, HasVideo: true}, nil
	}
	err := CopyMedia(ctx, e.deps, job, nil)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
	if len(e.assets.downloads) > 1 {
		t.Errorf("%d downloads after cancel", len(e.assets.downloads))
	}
	if left := listDir(t, e.deps.WorkDir); len(left) != 0 {
		t.Errorf("scratch files left behind: %v", left)
	}
}

func TestCopyMedia_DoesNotHangOnAStalledProvider(t *testing.T) {
	e := newEnv(t)
	job := e.jobWith(1)
	// A stand-in whose download waits for the context, as a stalled CDN does.
	e.deps.Assets = stalled{e.assets}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	start := time.Now()
	err := CopyMedia(ctx, e.deps, job, nil)
	if err == nil {
		t.Fatal("no error")
	}
	if time.Since(start) > 3*time.Second {
		t.Errorf("took %v to give up", time.Since(start))
	}
	se := stepErr(t, err)
	if !se.Retryable {
		t.Errorf("a stall is worth retrying: %+v", se)
	}
}
