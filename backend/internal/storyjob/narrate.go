package storyjob

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/rahul/indifferent/backend/internal/storyvideo"
)

// minNarrationSeconds is the shortest audio taken as speech. A response
// shorter than this is Polly returning silence or a header with no sound.
const minNarrationSeconds = 0.2

// NarrateScenes makes the voice for each scene: Polly's audio and its word
// timings, both stored in our bucket, with the audio's real length measured
// from the file. It then checks that the whole video, spoken, fits the length
// limit.
//
// A scene that already has its audio in storage is skipped, so an interrupted
// job does not pay for Polly twice. If the word timings cannot be had, the
// scene goes without them and its captions are spread evenly across the audio
// instead (see storyvideo.EvenWords): a video with slightly looser captions is
// better than none.
//
// A story that runs over the limit once spoken fails with a *StepError telling
// the user to shorten the narration. This is found here, after narration but
// before any rendering, so no render time is spent on a video that cannot be
// made.
//
// With d.Parallel above one, scenes are narrated side by side. As in CopyMedia,
// the first failure stops new scenes starting while those under way finish, and
// the lowest-numbered failing scene is the one reported.
func NarrateScenes(ctx context.Context, d Deps, job *Job, progress Progress) error {
	total := len(job.Scenes)
	done := 0

	// Scenes whose voice is already in storage are skipped first.
	var todo []int
	for i := range job.Scenes {
		if err := ctx.Err(); err != nil {
			return err
		}
		s := &job.Scenes[i]
		if s.Voice != nil && s.Voice.Seconds >= minNarrationSeconds {
			if have, err := d.Files.HeadObject(ctx, d.Bucket, s.Voice.AudioKey); err == nil && have {
				d.logf("storyjob scene=%d voice=already_made", i+1)
				done++
				progress.report(done, total)
				continue
			}
		}
		todo = append(todo, i)
	}

	// mu lets one scene at a time record its voice and report progress, which
	// writes the whole job.
	var mu sync.Mutex
	errs, left := runParallel(ctx, d.workers(), todo, nil, func(ctx context.Context, i int) error {
		voice, err := narrateOne(ctx, d, job, i+1)
		if err != nil {
			return err
		}
		mu.Lock()
		defer mu.Unlock()
		job.Scenes[i].Voice = voice
		done++
		progress.report(done, total)
		return nil
	})
	if err := firstError(errs); err != nil {
		return err
	}
	if len(left) > 0 {
		// Nothing failed, so the step was stopped from outside.
		return ctx.Err()
	}
	seconds := make([]float64, total)
	for i, s := range job.Scenes {
		seconds[i] = s.Voice.Seconds
	}
	tl, err := storyvideo.BuildTimeline(seconds)
	if err != nil {
		return wrapTimelineError(err)
	}
	job.TotalSeconds = tl.Total
	return nil
}

func narrateOne(ctx context.Context, d Deps, job *Job, n int) (*Voice, error) {
	text := job.Scenes[n-1].Narration
	if text == "" {
		return nil, &StepError{Scene: n, Code: CodeNarrationFailed,
			Message: fmt.Sprintf("Scene %d has no narration. Write some and make the video again.", n)}
	}

	audio, err := d.Narrator.SynthesizeText(ctx, text, job.Voice)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, &StepError{Scene: n, Code: CodeNarrationFailed, Retryable: true, Err: err,
			Message: fmt.Sprintf("Scene %d: the voice could not be made. Try again in a few minutes.", n)}
	}

	// The length comes from the file, never from the plan or from the word
	// timings: it is what the renderer will actually play.
	local := filepath.Join(d.WorkDir, fmt.Sprintf("scene-%02d-voice.mp3", n))
	defer os.Remove(local)
	if err := os.WriteFile(local, audio, 0o600); err != nil {
		return nil, internalError(n, err)
	}
	info, err := d.Prober.Probe(ctx, local)
	if err != nil || info.Duration < minNarrationSeconds {
		return nil, &StepError{Scene: n, Code: CodeNarrationFailed, Retryable: true,
			Err:     fmt.Errorf("the narration audio is unusable (length %.2fs): %v", info.Duration, err),
			Message: fmt.Sprintf("Scene %d: the voice could not be made. Try again in a few minutes.", n)}
	}

	audioKey := WorkKey(job.UserID, job.JobID, n, "voice.mp3")
	if err := d.Files.PutObject(ctx, d.Bucket, audioKey, audio, "audio/mpeg"); err != nil {
		return nil, internalError(n, fmt.Errorf("storing the narration: %w", err))
	}
	v := &Voice{AudioKey: audioKey, Seconds: info.Duration}

	marks, err := d.Narrator.SynthesizeMarks(ctx, text, job.Voice)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		d.logf("storyjob scene=%d marks=unavailable captions will be spread evenly: %v", n, err)
		return v, nil
	}
	// Prove the timings are usable now, so a bad answer is noticed here and not
	// during the render.
	if _, err := storyvideo.WordsFromMarks(text, marks, info.Duration); err != nil {
		d.logf("storyjob scene=%d marks=unusable captions will be spread evenly: %v", n, err)
		return v, nil
	}
	marksKey := WorkKey(job.UserID, job.JobID, n, "marks.jsonl")
	if err := d.Files.PutObject(ctx, d.Bucket, marksKey, marks, "application/x-ndjson"); err != nil {
		return nil, internalError(n, fmt.Errorf("storing the word timings: %w", err))
	}
	v.MarksKey = marksKey
	return v, nil
}
