package storyjob

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/rahul/indifferent/backend/internal/models"
)

// capturingFFmpeg records how it was run and what the captions file held at
// that moment, and makes no video.
type capturingFFmpeg struct {
	dir      string
	args     []string
	total    float64
	captions string
	capFile  string
	err      error
	percents []int
}

func (c *capturingFFmpeg) Run(_ context.Context, dir string, args []string, total float64, onPercent func(int)) error {
	c.dir, c.args, c.total = dir, args, total
	for _, ext := range []string{".srt", ".ass"} {
		if b, err := os.ReadFile(filepath.Join(dir, "captions"+ext)); err == nil {
			c.captions, c.capFile = string(b), "captions"+ext
		}
	}
	for _, p := range c.percents {
		onPercent(p)
	}
	if c.err != nil {
		return c.err
	}
	// A real ffmpeg writes its output, which is the last argument.
	if len(args) > 0 {
		if err := os.WriteFile(args[len(args)-1], []byte("MP4"), 0o600); err != nil {
			return err
		}
	}
	return nil
}

// preparedJob is a job whose media and narration exist in the bucket, with the
// given stored marks.
func (e *env) preparedJob(t *testing.T, scenes int, withMarks bool) *Job {
	t.Helper()
	job := e.jobWith(scenes)
	if err := CopyMedia(context.Background(), e.deps, job, nil); err != nil {
		t.Fatal(err)
	}
	if !withMarks {
		e.narrator.marksErr = errors.New("no marks")
	}
	if err := NarrateScenes(context.Background(), e.deps, job, nil); err != nil {
		t.Fatal(err)
	}
	return job
}

func (e *env) renderer(ff FFmpegRunner, info func(path, content string) (MediaInfo, error)) *VideoRenderer {
	return &VideoRenderer{Fetch: e.files, Bucket: "bucket", FFmpeg: ff, Prober: &fakeProber{info: info}, Logf: func(string, ...any) {}}
}

func goodOutput(seconds float64) func(string, string) (MediaInfo, error) {
	return func(string, string) (MediaInfo, error) {
		return MediaInfo{Width: 1080, Height: 1920, Duration: seconds, HasVideo: true, HasAudio: true}, nil
	}
}

func TestVideoRenderer_UsesPollysWordTimingsForCaptionsWhenItHasThem(t *testing.T) {
	e := newEnv(t)
	job := e.preparedJob(t, 2, true)
	job.Caption = models.CaptionStyle{Preset: "bold"}
	ff := &capturingFFmpeg{}
	work := t.TempDir()
	if _, err := e.renderer(ff, goodOutput(7.1)).Render(context.Background(), job, work, filepath.Join(work, "o.mp4"), func(int) {}); err != nil {
		t.Fatal(err)
	}
	// The fake narrator says a word every 0.3 s from the start of the scene.
	// Scene 1's words are timed from its start; scene 2's from its place on the
	// timeline (3.35 s in).
	if ff.capFile != "captions.srt" {
		t.Fatalf("captions file = %q", ff.capFile)
	}
	if !strings.Contains(ff.captions, "00:00:00,000 -->") {
		t.Errorf("scene 1's first word does not start with its narration:\n%s", ff.captions)
	}
	if !regexp.MustCompile(`00:00:03,35\d --> `).MatchString(ff.captions) {
		t.Errorf("scene 2's captions do not start where its narration does (3.35 s):\n%s", ff.captions)
	}
}

func TestVideoRenderer_SpreadsCaptionsEvenlyWhenThereAreNoStoredTimings(t *testing.T) {
	e := newEnv(t)
	job := e.preparedJob(t, 1, false)
	if job.Scenes[0].Voice.MarksKey != "" {
		t.Fatal("setup: the scene should have no stored timings")
	}
	ff := &capturingFFmpeg{}
	var logged []string
	r := e.renderer(ff, goodOutput(3.75))
	r.Logf = func(f string, a ...any) { logged = append(logged, fmt.Sprintf(f, a...)) }
	work := t.TempDir()
	if _, err := r.Render(context.Background(), job, work, filepath.Join(work, "o.mp4"), func(int) {}); err != nil {
		t.Fatal(err)
	}
	if ff.captions == "" {
		t.Fatal("no captions at all: a video without word timings should still be captioned")
	}
	// Evenly spread, not at the fake narrator's 0.3 s rhythm: the first cue
	// covers several words.
	if strings.Contains(ff.captions, "00:00:00,000 --> 00:00:00,3") {
		t.Errorf("the captions follow word timings that do not exist:\n%s", ff.captions)
	}
}

func TestVideoRenderer_FallsBackWhenTheStoredTimingsAreGoneOrUnreadable(t *testing.T) {
	for name, damage := range map[string]func(e *env, j *Job){
		"the object is gone": func(e *env, j *Job) { delete(e.files.files, "bucket/"+j.Scenes[0].Voice.MarksKey) },
		"it is not timings":  func(e *env, j *Job) { e.files.files["bucket/"+j.Scenes[0].Voice.MarksKey] = []byte("<html>") },
	} {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t)
			job := e.preparedJob(t, 1, true)
			damage(e, job)
			ff := &capturingFFmpeg{}
			var logged []string
			r := e.renderer(ff, goodOutput(3.75))
			r.Logf = func(f string, a ...any) { logged = append(logged, fmt.Sprintf(f, a...)) }
			work := t.TempDir()
			if _, err := r.Render(context.Background(), job, work, filepath.Join(work, "o.mp4"), func(int) {}); err != nil {
				t.Fatalf("damaged timings failed the render: %v", err)
			}
			if ff.captions == "" || !strings.Contains(strings.Join(logged, "\n"), "spread_evenly") {
				t.Errorf("captions %q, log %v", ff.captions, logged)
			}
		})
	}
}

func TestVideoRenderer_KaraokeCaptionsAreASS(t *testing.T) {
	e := newEnv(t)
	job := e.preparedJob(t, 1, true)
	job.Caption = models.CaptionStyle{Preset: "karaoke"}
	ff := &capturingFFmpeg{}
	work := t.TempDir()
	if _, err := e.renderer(ff, goodOutput(3.75)).Render(context.Background(), job, work, filepath.Join(work, "o.mp4"), func(int) {}); err != nil {
		t.Fatal(err)
	}
	if ff.capFile != "captions.ass" || !strings.Contains(ff.captions, `{\kf`) {
		t.Errorf("file %q:\n%s", ff.capFile, ff.captions)
	}
}

func TestVideoRenderer_RunsFFmpegInTheWorkDirWithARelativeCaptionsPath(t *testing.T) {
	e := newEnv(t)
	job := e.preparedJob(t, 2, true)
	ff := &capturingFFmpeg{}
	work := t.TempDir()
	// A work directory whose name would need escaping inside an FFmpeg filter.
	odd := filepath.Join(work, "it's a: odd dir [x]")
	if err := os.MkdirAll(odd, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := e.renderer(ff, goodOutput(7.1)).Render(context.Background(), job, odd, filepath.Join(odd, "o.mp4"), func(int) {}); err != nil {
		t.Fatalf("an unusual directory name broke the render: %v", err)
	}
	if ff.dir != odd {
		t.Errorf("ffmpeg ran in %q, want %q", ff.dir, odd)
	}
	joined := strings.Join(ff.args, " ")
	if !strings.Contains(joined, "subtitles=captions.srt") || strings.Contains(joined, "subtitles="+odd) {
		t.Errorf("the captions path is not relative: %s", joined)
	}
	if ff.total < 7 || ff.total > 7.2 {
		t.Errorf("progress is measured against %.2fs, want the timeline's 7.1s", ff.total)
	}
}

func TestVideoRenderer_PassesEachScenesMeasuredSizeAndKindToTheCommand(t *testing.T) {
	e := newEnv(t)
	job := e.preparedJob(t, 2, true)
	job.Scenes[0].Media.Width, job.Scenes[0].Media.Height = 1920, 1080 // measured landscape
	ff := &capturingFFmpeg{}
	work := t.TempDir()
	if _, err := e.renderer(ff, goodOutput(7.1)).Render(context.Background(), job, work, filepath.Join(work, "o.mp4"), func(int) {}); err != nil {
		t.Fatal(err)
	}
	graph := ""
	for i, a := range ff.args {
		if a == "-filter_complex" {
			graph = ff.args[i+1]
		}
	}
	// Scene 1 is landscape so it is fitted (blur-filled); scene 2 is 1280x1920 so it is cropped.
	if !strings.Contains(graph, "gblur") || strings.Count(graph, "gblur") != 1 {
		t.Errorf("expected exactly one blur-filled scene:\n%s", graph)
	}
}

func TestVideoRenderer_ReportsProgress(t *testing.T) {
	e := newEnv(t)
	job := e.preparedJob(t, 1, true)
	ff := &capturingFFmpeg{percents: []int{10, 60, 99}}
	var seen []int
	work := t.TempDir()
	if _, err := e.renderer(ff, goodOutput(3.75)).Render(context.Background(), job, work, filepath.Join(work, "o.mp4"), func(p int) { seen = append(seen, p) }); err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(seen) != "[10 60 99]" {
		t.Errorf("progress = %v", seen)
	}
}

func TestVideoRenderer_RefusesAJobThatIsNotPrepared(t *testing.T) {
	e := newEnv(t)
	job := e.jobWith(2) // nothing copied, nothing narrated
	work := t.TempDir()
	_, err := e.renderer(&capturingFFmpeg{}, goodOutput(5)).Render(context.Background(), job, work, filepath.Join(work, "o.mp4"), func(int) {})
	se := stepErr(t, err)
	if se.Code != CodeInternal || !se.Retryable || se.Scene != 1 {
		t.Errorf("got %+v", se)
	}
}

func TestVideoRenderer_AFileMissingFromStorageIsOurFaultAndRetried(t *testing.T) {
	e := newEnv(t)
	job := e.preparedJob(t, 2, true)
	delete(e.files.files, "bucket/"+job.Scenes[1].Voice.AudioKey)
	work := t.TempDir()
	_, err := e.renderer(&capturingFFmpeg{}, goodOutput(7.1)).Render(context.Background(), job, work, filepath.Join(work, "o.mp4"), func(int) {})
	se := stepErr(t, err)
	if se.Scene != 2 || !se.Retryable {
		t.Errorf("got %+v", se)
	}
}

func TestVideoRenderer_TheFinishedFileMustBeTheVideoThatWasAskedFor(t *testing.T) {
	cases := map[string]struct {
		info    MediaInfo
		problem string
	}{
		"no sound":     {MediaInfo{Width: 1080, Height: 1920, Duration: 7.1, HasVideo: true}, "no sound"},
		"no picture":   {MediaInfo{Duration: 7.1, HasAudio: true}, "no picture"},
		"wrong size":   {MediaInfo{Width: 1920, Height: 1080, Duration: 7.1, HasVideo: true, HasAudio: true}, "1920x1080"},
		"wrong width":  {MediaInfo{Width: 1000, Height: 1920, Duration: 7.1, HasVideo: true, HasAudio: true}, "1000x1920"},
		"wrong height": {MediaInfo{Width: 1080, Height: 1900, Duration: 7.1, HasVideo: true, HasAudio: true}, "1080x1900"},
		"too short":    {MediaInfo{Width: 1080, Height: 1920, Duration: 2, HasVideo: true, HasAudio: true}, "2.0 s"},
		"too long":     {MediaInfo{Width: 1080, Height: 1920, Duration: 12, HasVideo: true, HasAudio: true}, "12.0 s"},
		"several ways": {MediaInfo{Width: 320, Height: 240, Duration: 1}, "no picture"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t)
			job := e.preparedJob(t, 2, true)
			work := t.TempDir()
			_, err := e.renderer(&capturingFFmpeg{}, func(string, string) (MediaInfo, error) { return tc.info, nil }).
				Render(context.Background(), job, work, filepath.Join(work, "o.mp4"), func(int) {})
			se := stepErr(t, err)
			if !se.Retryable || !strings.Contains(se.Err.Error(), tc.problem) {
				t.Errorf("got %+v / %v, want a retryable error mentioning %q", se, se.Err, tc.problem)
			}
			if strings.Contains(se.Message, "1920x1080") || strings.Contains(se.Message, "no sound") {
				t.Errorf("the user is shown internals: %q", se.Message)
			}
		})
	}
	t.Run("a length within the encoder's rounding is fine", func(t *testing.T) {
		e := newEnv(t)
		job := e.preparedJob(t, 2, true)
		work := t.TempDir()
		got, err := e.renderer(&capturingFFmpeg{}, goodOutput(7.4)).Render(context.Background(), job, work, filepath.Join(work, "o.mp4"), func(int) {})
		if err != nil || got != 7.4 {
			t.Errorf("got %v, %v", got, err)
		}
	})
	t.Run("an output that cannot be read at all", func(t *testing.T) {
		e := newEnv(t)
		job := e.preparedJob(t, 2, true)
		work := t.TempDir()
		_, err := e.renderer(&capturingFFmpeg{}, func(string, string) (MediaInfo, error) { return MediaInfo{}, errors.New("moov atom not found") }).
			Render(context.Background(), job, work, filepath.Join(work, "o.mp4"), func(int) {})
		if se := stepErr(t, err); !se.Retryable {
			t.Errorf("got %+v", se)
		}
	})
}

func TestVideoRenderer_FFmpegFailures(t *testing.T) {
	t.Run("a failed render is ours and retried, without the paths", func(t *testing.T) {
		e := newEnv(t)
		job := e.preparedJob(t, 1, true)
		work := t.TempDir()
		_, err := e.renderer(&capturingFFmpeg{err: errors.New("Invalid argument")}, goodOutput(3.75)).
			Render(context.Background(), job, work, filepath.Join(work, "o.mp4"), func(int) {})
		se := stepErr(t, err)
		if se.Code != CodeInternal || !se.Retryable || strings.Contains(se.Message, "Invalid argument") {
			t.Errorf("got %+v", se)
		}
	})
	t.Run("a cancel is passed through untouched", func(t *testing.T) {
		e := newEnv(t)
		job := e.preparedJob(t, 1, true)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		work := t.TempDir()
		_, err := e.renderer(&capturingFFmpeg{err: context.Canceled}, goodOutput(3.75)).
			Render(ctx, job, work, filepath.Join(work, "o.mp4"), func(int) {})
		if !errors.Is(err, context.Canceled) {
			t.Errorf("err = %v", err)
		}
	})
}

func TestVideoRenderer_AStoryThatRunsLongSpokenIsRefusedWithTheUserFacingMessage(t *testing.T) {
	e := newEnv(t)
	job := e.preparedJob(t, 10, true)
	for i := range job.Scenes {
		job.Scenes[i].Voice.Seconds = 9.5
	}
	work := t.TempDir()
	_, err := e.renderer(&capturingFFmpeg{}, goodOutput(90)).Render(context.Background(), job, work, filepath.Join(work, "o.mp4"), func(int) {})
	se := stepErr(t, err)
	if se.Code != CodeNarrationTooLong || se.Retryable {
		t.Errorf("got %+v", se)
	}
}
