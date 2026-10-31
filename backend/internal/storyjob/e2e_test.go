package storyjob

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rahul/indifferent/backend/internal/models"
	"github.com/rahul/indifferent/backend/internal/shorts/render"
	"github.com/rahul/indifferent/backend/internal/stockmedia"
	"github.com/rahul/indifferent/backend/internal/storyplan"
	"github.com/rahul/indifferent/backend/internal/storyvideo"
)

// These tests run a whole job: the real Runner, VideoRenderer, FFmpeg and
// ffprobe, with in-memory stand-ins only for AWS and the stock providers. They
// take about half a minute each, so -short skips them.

func needMedia(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("renders real video; skipped with -short")
	}
	for _, b := range []string{"ffmpeg", "ffprobe"} {
		if _, err := exec.LookPath(b); err != nil {
			t.Skipf("%s is not installed", b)
		}
	}
}

func ff(t *testing.T, args ...string) []byte {
	t.Helper()
	var stderr bytes.Buffer
	cmd := exec.Command("ffmpeg", append([]string{"-v", "error", "-y"}, args...)...)
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("ffmpeg %v: %v\n%s", args, err, stderr.String())
	}
	return out
}

// makeFile runs ffmpeg to make a file and returns its bytes.
func makeFile(t *testing.T, name string, args ...string) []byte {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	ff(t, append(args, p)...)
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

type e2e struct {
	*env
	job    *Job
	store  *memStore
	runner *Runner
}

// newE2E builds a three-scene job with real picture files: a red portrait photo
// (cropped), a green landscape clip (fitted, looped) and a blue landscape png
// (fitted), narrated in 2.5, 3.0 and 3.5 seconds of real audio.
func newE2E(t *testing.T) *e2e {
	t.Helper()
	needMedia(t)
	e := newEnv(t)
	e.deps.Prober = FFprobe{}
	e.deps.Logf = t.Logf

	type spec struct {
		provider stockmedia.ProviderName
		typ      stockmedia.MediaType
		id       string
		format   string
		w, h     int
		bytes    []byte
	}
	specs := []spec{
		{stockmedia.ProviderPixabay, stockmedia.MediaPhoto, "501", "image/jpeg", 1280, 1920,
			makeFile(t, "a.jpg", "-f", "lavfi", "-i", "color=c=0xFF0000:s=1280x1920", "-frames:v", "1")},
		{stockmedia.ProviderPexels, stockmedia.MediaVideo, "502", "video/mp4", 1920, 1080,
			makeFile(t, "b.mp4", "-f", "lavfi", "-i", "color=c=0x00FF00:s=1920x1080:r=25:d=2", "-c:v", "libx264", "-pix_fmt", "yuv420p")},
		{stockmedia.ProviderPixabay, stockmedia.MediaPhoto, "503", "image/png", 1920, 1080,
			makeFile(t, "c.png", "-f", "lavfi", "-i", "color=c=0x0000FF:s=1920x1080", "-frames:v", "1")},
	}
	job := &Job{UserID: "u1", JobID: "e2e", Voice: "Joanna", Language: "en", Status: StatusQueued,
		Caption: models.CaptionStyle{Preset: "bold"}}
	texts := []string{
		"The first scene is a red photograph held on screen.",
		"The second scene is a green clip that has to loop.",
		"The third scene is a blue picture that is shown whole.",
	}
	e.narrator.audioFor = map[string][]byte{}
	for i, sp := range specs {
		a := e.assets.add(sp.provider, sp.typ, sp.id)
		a.Format, a.Width, a.Height = sp.format, sp.w, sp.h
		e.assets.assets[ref(string(sp.provider), string(sp.typ), sp.id)] = a
		e.assets.dlBytes[ref(string(sp.provider), string(sp.typ), sp.id)] = string(sp.bytes)
		secs := 2.5 + 0.5*float64(i)
		e.narrator.audioFor[texts[i]] = makeFile(t, "n.mp3", "-f", "lavfi", "-i", fmt.Sprintf("sine=frequency=%d:duration=%.1f", 300+100*i, secs), "-c:a", "libmp3lame")
		job.Scenes = append(job.Scenes, Scene{Narration: texts[i], Pick: storyplan.MediaRef{Provider: string(sp.provider), Type: string(sp.typ), ID: sp.id}})
	}
	st := newStore(job)
	runner := &Runner{
		Deps: e.deps, Store: st,
		Renderer: &VideoRenderer{Fetch: e.files, Bucket: "bucket", FFmpeg: storyvideo.FFmpeg{}, Prober: FFprobe{}, Logf: t.Logf},
		Now:      time.Now,
	}
	return &e2e{env: e, job: job, store: st, runner: runner}
}

func (x *e2e) storedVideo(t *testing.T) string {
	t.Helper()
	data, ok := x.files.files["bucket/storyvideo/u1/e2e/video.mp4"]
	if !ok {
		t.Fatalf("no video in the bucket; stored: %v", x.files.puts)
	}
	p := filepath.Join(t.TempDir(), "video.mp4")
	if err := os.WriteFile(p, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func rgbAt(t *testing.T, video string, at float64, x, y int) [3]int {
	t.Helper()
	cmd := exec.Command("ffmpeg", "-v", "error", "-ss", fmt.Sprintf("%.3f", at), "-i", video, "-frames:v", "1",
		"-vf", fmt.Sprintf("format=rgb24,crop=1:1:%d:%d", x, y), "-f", "rawvideo", "-")
	out, err := cmd.Output()
	if err != nil || len(out) != 3 {
		t.Fatalf("no frame at %.2fs: %v (%d bytes)", at, err, len(out))
	}
	return [3]int{int(out[0]), int(out[1]), int(out[2])}
}

func closeTo(a, b [3]int, tol int) bool {
	for i := range a {
		if math.Abs(float64(a[i]-b[i])) > float64(tol) {
			return false
		}
	}
	return true
}

func TestE2E_AQueuedJobBecomesAFinishedVideoWithItsPicturesNarrationAndCaptions(t *testing.T) {
	x := newE2E(t)
	out, err := x.runner.Run(context.Background(), "u1", "e2e")
	if out != OutcomeCompleted || err != nil {
		t.Fatalf("outcome %v, err %v\njob: %+v", out, err, x.store.current())
	}
	got := x.store.current()
	if got.Status != StatusCompleted || got.OutputKey != "storyvideo/u1/e2e/video.mp4" || got.Progress != 100 {
		t.Fatalf("job = %+v", got)
	}

	video := x.storedVideo(t)
	info, err := FFprobe{}.Probe(context.Background(), video)
	if err != nil {
		t.Fatal(err)
	}
	// 1080x1920, picture and sound, and the length the narration implies.
	want := (2.5 + 0.35) + (3.0 + 0.35) + (3.5 + 0.35) + storyvideo.TransitionSeconds
	if info.Width != 1080 || info.Height != 1920 || !info.HasVideo || !info.HasAudio {
		t.Errorf("video = %+v", info)
	}
	if math.Abs(info.Duration-want) > 0.4 {
		t.Errorf("length %.2fs, want about %.2fs", info.Duration, want)
	}
	if math.Abs(got.TotalSeconds-info.Duration) > 0.05 {
		t.Errorf("the job records %.2fs but the file is %.2fs", got.TotalSeconds, info.Duration)
	}

	// Scene order and framing, read off the pixels at the middle of each scene.
	spans := []float64{2.85, 3.35, 3.85}
	start, mids := 0.0, []float64{}
	for _, s := range spans {
		mids = append(mids, start+s/2)
		start += s
	}
	for i, c := range [][3]int{{255, 0, 0}, {0, 255, 0}, {0, 0, 255}} {
		for _, pt := range [][2]int{{540, 960}, {540, 120}} { // the centre, and near the top edge
			if px := rgbAt(t, video, mids[i], pt[0], pt[1]); !closeTo(px, c, 45) {
				t.Errorf("scene %d at %.2fs, pixel %v = %v, want about %v", i+1, mids[i], pt, px, c)
			}
		}
	}
	// The looped clip (2 s long) still fills its 3.35 s scene.
	if px := rgbAt(t, video, spans[0]+spans[1]-0.4, 540, 960); !closeTo(px, [3]int{0, 255, 0}, 45) {
		t.Errorf("end of the looped clip scene = %v, want green", px)
	}
}

func TestE2E_CaptionsAreInTheVideoWhenWordTimingsAreUsedAndWhenTheyAreNot(t *testing.T) {
	if err := render.EnsureCaptionFonts(); err != nil {
		t.Skip(err)
	}
	for _, useMarks := range []bool{true, false} {
		t.Run(fmt.Sprintf("word timings=%v", useMarks), func(t *testing.T) {
			x := newE2E(t)
			if !useMarks {
				x.narrator.marksErr = fmt.Errorf("polly throttled")
			}
			if out, err := x.runner.Run(context.Background(), "u1", "e2e"); out != OutcomeCompleted {
				t.Fatalf("outcome %v err %v", out, err)
			}
			got := x.store.current()
			if hasMarks := got.Scenes[0].Voice.MarksKey != ""; hasMarks != useMarks {
				t.Fatalf("marks stored = %v, want %v", hasMarks, useMarks)
			}
			// Compare with the same video rendered without captions: the bottom of
			// the frame must differ where a caption is drawn.
			// A caption is drawn in the bottom third, where the pictures are flat colour.
			if !bottomHasText(t, x.storedVideo(t), 1.0) {
				t.Errorf("no caption was drawn in the bottom third (word timings=%v)", useMarks)
			}
		})
	}
}

// bottomHasText reports whether the bottom third of the frame at t seconds
// has the variation a caption adds. The pictures in these tests are flat
// colours, so anything else there is text.
func bottomHasText(t *testing.T, video string, at float64) bool {
	t.Helper()
	cmd := exec.Command("ffmpeg", "-v", "error", "-ss", fmt.Sprintf("%.3f", at), "-i", video, "-frames:v", "1",
		"-vf", "crop=1080:640:0:1280,format=gray", "-f", "rawvideo", "-")
	out, err := cmd.Output()
	if err != nil || len(out) == 0 {
		t.Fatal(err)
	}
	min, max := byte(255), byte(0)
	for _, b := range out {
		if b < min {
			min = b
		}
		if b > max {
			max = b
		}
	}
	return int(max)-int(min) > 100
}

func TestE2E_AMissingPictureFailsTheJobWithAMessageAndStoresNoVideo(t *testing.T) {
	x := newE2E(t)
	delete(x.assets.assets, "pexels:video:502")
	out, err := x.runner.Run(context.Background(), "u1", "e2e")
	got := x.store.current()
	if out != OutcomeFailed || err == nil || got.Status != StatusFailed {
		t.Fatalf("outcome %v err %v job %+v", out, err, got)
	}
	if got.ErrorCode != CodeMediaUnavailable || !strings.Contains(got.Error, "Scene 2") || !strings.Contains(got.Error, "Pexels") {
		t.Errorf("job error = %q (%s)", got.Error, got.ErrorCode)
	}
	if _, ok := x.files.files["bucket/storyvideo/u1/e2e/video.mp4"]; ok {
		t.Error("a video was stored for a failed job")
	}
	if len(x.narrator.calls) != 0 {
		t.Error("Polly was called for a job that failed while copying pictures")
	}
}

func TestE2E_ARestartedJobDoesNotRepeatWhatItAlreadyDid(t *testing.T) {
	x := newE2E(t)
	// First run: the provider fails for the last scene's picture.
	x.assets.getErr["pixabay:photo:503"] = &stockmedia.ProviderError{Provider: "pixabay", Kind: stockmedia.KindUnavailable}
	if out, _ := x.runner.Run(context.Background(), "u1", "e2e"); out != OutcomeRetry {
		t.Fatalf("first run outcome %v, want retry", out)
	}
	firstDownloads := len(x.assets.downloads)
	// The provider recovers and the state machine starts the task again.
	delete(x.assets.getErr, "pixabay:photo:503")
	if out, err := x.runner.Run(context.Background(), "u1", "e2e"); out != OutcomeCompleted {
		t.Fatalf("second run outcome %v err %v", out, err)
	}
	if n := len(x.assets.downloads) - firstDownloads; n != 1 {
		t.Errorf("the second run made %d downloads, want 1 (only the picture that failed)", n)
	}
	if got := x.store.current(); got.Status != StatusCompleted || got.Error != "" {
		t.Errorf("job = %+v", got)
	}
}

func TestE2E_TheFinishedFileIsCheckedBeforeItIsCalledDone(t *testing.T) {
	x := newE2E(t)
	// An FFmpeg that "succeeds" but makes a file that is not the video asked for.
	x.runner.Renderer = &VideoRenderer{Fetch: x.files, Bucket: "bucket", Prober: FFprobe{}, Logf: t.Logf,
		FFmpeg: wrongOutput{}}
	out, err := x.runner.Run(context.Background(), "u1", "e2e")
	got := x.store.current()
	if out != OutcomeRetry || err == nil || got.Status == StatusCompleted {
		t.Fatalf("outcome %v err %v job %+v: a wrong file was accepted", out, err, got)
	}
	if _, ok := x.files.files["bucket/storyvideo/u1/e2e/video.mp4"]; ok {
		t.Error("the wrong file was stored")
	}
}

// wrongOutput exits 0 having made a small, silent, wrongly sized video.
type wrongOutput struct{}

func (wrongOutput) Run(ctx context.Context, dir string, args []string, _ float64, _ func(int)) error {
	out := args[len(args)-1]
	cmd := exec.CommandContext(ctx, "ffmpeg", "-v", "error", "-y", "-f", "lavfi", "-i", "color=c=black:s=320x240:r=30:d=2", "-c:v", "libx264", "-pix_fmt", "yuv420p", out)
	return cmd.Run()
}

func TestE2E_TimingOfTheStagesIsLoggedSoTheCostOnFargateCanBeMeasured(t *testing.T) {
	x := newE2E(t)
	var logged []string
	x.runner.Deps.Logf = func(f string, a ...any) { logged = append(logged, fmt.Sprintf(f, a...)) }
	if out, err := x.runner.Run(context.Background(), "u1", "e2e"); out != OutcomeCompleted {
		t.Fatalf("outcome %v err %v", out, err)
	}
	all := strings.Join(logged, "\n")
	for _, want := range []string{"storyjob_timing", "prepare=", "narrate=", "render=", "upload=", "video_seconds=", "output_bytes="} {
		if !strings.Contains(all, want) {
			t.Errorf("the timing line lacks %q:\n%s", want, all)
		}
	}
}
