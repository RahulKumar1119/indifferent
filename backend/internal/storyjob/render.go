package storyjob

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"

	"github.com/rahul/indifferent/backend/internal/shorts/render"
	"github.com/rahul/indifferent/backend/internal/storyvideo"
)

// FFmpegRunner runs FFmpeg. storyvideo.FFmpeg satisfies it.
type FFmpegRunner interface {
	Run(ctx context.Context, dir string, args []string, total float64, onPercent func(int)) error
}

// Fetcher downloads a stored file to disk. *storage.S3Client satisfies it.
type Fetcher interface {
	DownloadToFile(ctx context.Context, bucket, key, path string) error
}

// VideoRenderer renders a job whose media and narration are prepared.
type VideoRenderer struct {
	Fetch  Fetcher
	Bucket string
	FFmpeg FFmpegRunner
	Prober Prober
	Logf   func(format string, args ...any)
}

var _ Renderer = (*VideoRenderer)(nil)

// lengthTolerance is how far the finished video's length may differ from the
// timeline's. The encoder rounds to whole frames and the audio to whole
// packets, so an exact match is not expected.
const lengthTolerance = 0.5

// Render makes the video. It fetches each scene's picture, narration and word
// timings, lays the scenes out from the narration's real length, builds the
// captions and the FFmpeg command (package storyvideo), runs it, and then
// checks the file before it is called done: a picture and a sound stream, the
// right size, and a length that matches the plan. FFmpeg exiting without error
// is not proof of a good video.
//
// The captions file is written into workDir and named relatively, and FFmpeg is
// run in workDir, so no path has to be escaped inside a filter.
func (v *VideoRenderer) Render(ctx context.Context, job *Job, workDir, outPath string, onPercent func(int)) (float64, error) {
	n := len(job.Scenes)
	scenes := make([]storyvideo.Scene, n)
	seconds := make([]float64, n)
	words := make([]storyvideo.SceneWords, n)

	for i, s := range job.Scenes {
		num := i + 1
		if s.Media == nil || s.Voice == nil {
			return 0, internalError(num, fmt.Errorf("scene %d is not prepared for rendering", num))
		}
		mediaPath := filepath.Join(workDir, fmt.Sprintf("scene-%02d-media%s", num, filepath.Ext(s.Media.Key)))
		voicePath := filepath.Join(workDir, fmt.Sprintf("scene-%02d-voice.mp3", num))
		for _, f := range []struct{ key, path string }{{s.Media.Key, mediaPath}, {s.Voice.AudioKey, voicePath}} {
			if err := v.Fetch.DownloadToFile(ctx, v.Bucket, f.key, f.path); err != nil {
				return 0, internalError(num, fmt.Errorf("fetching a prepared file: %w", err))
			}
		}
		scenes[i] = storyvideo.Scene{
			Kind: storyvideo.MediaKind(s.Media.Kind), MediaPath: mediaPath,
			Width: s.Media.Width, Height: s.Media.Height, NarrationPath: voicePath,
		}
		seconds[i] = s.Voice.Seconds
		words[i] = v.wordsFor(ctx, job, i, workDir)
	}

	tl, err := storyvideo.BuildTimeline(seconds)
	if err != nil {
		return 0, wrapTimelineError(err)
	}
	placed, err := storyvideo.PlaceWords(words, tl)
	if err != nil {
		return 0, internalError(0, err)
	}

	captions := ""
	if len(placed.Words) > 0 {
		doc, ext := render.BuildCaptions(placed.Words, job.Caption, placed.BreakBefore)
		captions = "captions" + ext
		if err := os.WriteFile(filepath.Join(workDir, captions), []byte(doc), 0o600); err != nil {
			return 0, internalError(0, fmt.Errorf("writing the captions: %w", err))
		}
	}

	args, err := storyvideo.BuildArgs(storyvideo.Spec{
		Scenes: scenes, Timeline: tl, CaptionsPath: captions, Caption: job.Caption, OutputPath: outPath,
	})
	if err != nil {
		return 0, internalError(0, err)
	}
	if err := v.FFmpeg.Run(ctx, workDir, args, tl.Total, onPercent); err != nil {
		if ctx.Err() != nil {
			return 0, ctx.Err()
		}
		return 0, internalError(0, fmt.Errorf("rendering: %w", err))
	}
	return v.check(ctx, outPath, tl.Total)
}

// wordsFor gets the caption words of scene i: from Polly's timings if they are
// stored and usable, otherwise spread evenly across the audio.
func (v *VideoRenderer) wordsFor(ctx context.Context, job *Job, i int, workDir string) storyvideo.SceneWords {
	s := job.Scenes[i]
	sw := storyvideo.SceneWords{AudioSeconds: s.Voice.Seconds}
	if s.Voice.MarksKey != "" {
		path := filepath.Join(workDir, fmt.Sprintf("scene-%02d-marks.jsonl", i+1))
		if err := v.Fetch.DownloadToFile(ctx, v.Bucket, s.Voice.MarksKey, path); err == nil {
			if data, err := os.ReadFile(path); err == nil {
				if w, err := storyvideo.WordsFromMarks(s.Narration, data, s.Voice.Seconds); err == nil {
					sw.Words = w
					return sw
				}
			}
		}
		v.logf("storyjob scene=%d captions=spread_evenly the stored word timings could not be used", i+1)
	}
	sw.Words = storyvideo.EvenWords(s.Narration, s.Voice.Seconds)
	return sw
}

// check proves the file is the video that was asked for and returns its length.
func (v *VideoRenderer) check(ctx context.Context, path string, want float64) (float64, error) {
	info, err := v.Prober.Probe(ctx, path)
	if err != nil {
		return 0, internalError(0, fmt.Errorf("the finished video cannot be read: %w", err))
	}
	var problems []string
	if !info.HasVideo {
		problems = append(problems, "it has no picture")
	}
	if !info.HasAudio {
		problems = append(problems, "it has no sound")
	}
	if info.Width != storyvideo.Width || info.Height != storyvideo.Height {
		problems = append(problems, fmt.Sprintf("it is %dx%d, not %dx%d", info.Width, info.Height, storyvideo.Width, storyvideo.Height))
	}
	if math.Abs(info.Duration-want) > lengthTolerance {
		problems = append(problems, fmt.Sprintf("it runs %.1f s, not the planned %.1f s", info.Duration, want))
	}
	if len(problems) > 0 {
		return 0, internalError(0, fmt.Errorf("the finished video is wrong: %v", problems))
	}
	return info.Duration, nil
}

func (v *VideoRenderer) logf(format string, args ...any) {
	if v.Logf != nil {
		v.Logf(format, args...)
	}
}

func wrapTimelineError(err error) error {
	var tooLong *storyvideo.TooLongError
	if asTooLong(err, &tooLong) {
		return &StepError{Code: CodeNarrationTooLong, Message: tooLong.UserMessage(), Err: err}
	}
	return internalError(0, err)
}
