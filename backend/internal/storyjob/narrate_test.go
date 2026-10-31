package storyjob

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestNarrateScenes_MakesAndStoresTheVoiceAndTimingsOfEachScene(t *testing.T) {
	e := newEnv(t)
	job := e.jobWith(3)
	var seen []string
	if err := NarrateScenes(context.Background(), e.deps, job, func(d, n int) { seen = append(seen, fmt.Sprintf("%d/%d", d, n)) }); err != nil {
		t.Fatal(err)
	}
	for i, s := range job.Scenes {
		n := i + 1
		wantAudio := fmt.Sprintf("storyvideo-work/u1/j1/scene-%02d-voice.mp3", n)
		wantMarks := fmt.Sprintf("storyvideo-work/u1/j1/scene-%02d-marks.jsonl", n)
		if s.Voice == nil || s.Voice.AudioKey != wantAudio || s.Voice.MarksKey != wantMarks {
			t.Fatalf("scene %d voice = %+v", n, s.Voice)
		}
		if s.Voice.Seconds != 3 {
			t.Errorf("scene %d: length %v, want the 3 s measured from the file", n, s.Voice.Seconds)
		}
		if !e.files.has("bucket", wantAudio) || !e.files.has("bucket", wantMarks) {
			t.Errorf("scene %d: audio or marks missing from the bucket", n)
		}
		if got := string(e.files.files["bucket/"+wantAudio]); got != "MP3:"+s.Narration {
			t.Errorf("scene %d: stored audio is %q", n, got)
		}
	}
	if strings.Join(seen, " ") != "1/3 2/3 3/3" {
		t.Errorf("progress = %v", seen)
	}
}

func TestNarrateScenes_UsesTheJobsVoiceAndTheSceneTextForBothCalls(t *testing.T) {
	e := newEnv(t)
	job := e.jobWith(1)
	job.Voice = "Aditi"
	job.Scenes[0].Narration = "लाइटहाउस चट्टान पर अकेला खड़ा था।"
	if err := NarrateScenes(context.Background(), e.deps, job, nil); err != nil {
		t.Fatal(err)
	}
	want := []string{"audio:Aditi:" + job.Scenes[0].Narration, "marks:Aditi:" + job.Scenes[0].Narration}
	if strings.Join(e.narrator.calls, "|") != strings.Join(want, "|") {
		t.Errorf("calls = %v, want exactly the same text and voice for the audio and its timings", e.narrator.calls)
	}
}

func TestNarrateScenes_TotalLengthComesFromTheAudioNotThePlan(t *testing.T) {
	e := newEnv(t)
	job := e.jobWith(3)
	if err := NarrateScenes(context.Background(), e.deps, job, nil); err != nil {
		t.Fatal(err)
	}
	// 3 x (3 s + 0.35 s pause) + the closing fade
	if got, want := job.TotalSeconds, 3*3.35+0.4; got < want-1e-9 || got > want+1e-9 {
		t.Errorf("TotalSeconds = %v, want %v", got, want)
	}
}

func TestNarrateScenes_SkipsWhatIsAlreadyMadeSoARestartedJobDoesNotPayForPollyTwice(t *testing.T) {
	e := newEnv(t)
	job := e.jobWith(3)
	if err := NarrateScenes(context.Background(), e.deps, job, nil); err != nil {
		t.Fatal(err)
	}
	calls := len(e.narrator.calls)
	if err := NarrateScenes(context.Background(), e.deps, job, nil); err != nil {
		t.Fatal(err)
	}
	if len(e.narrator.calls) != calls {
		t.Errorf("a second run made %d more Polly calls", len(e.narrator.calls)-calls)
	}
	if job.TotalSeconds == 0 {
		t.Error("the length was not worked out again on the second run")
	}
}

func TestNarrateScenes_RedoesOnlyTheSceneWhoseAudioWentMissing(t *testing.T) {
	e := newEnv(t)
	job := e.jobWith(3)
	if err := NarrateScenes(context.Background(), e.deps, job, nil); err != nil {
		t.Fatal(err)
	}
	delete(e.files.files, "bucket/"+job.Scenes[1].Voice.AudioKey)
	e.narrator.calls = nil
	if err := NarrateScenes(context.Background(), e.deps, job, nil); err != nil {
		t.Fatal(err)
	}
	if len(e.narrator.calls) != 2 || !strings.Contains(e.narrator.calls[0], "scene 2") {
		t.Errorf("calls = %v, want only scene 2 again", e.narrator.calls)
	}
}

func TestNarrateScenes_WithoutUsableTimingsTheSceneStillGetsItsVoice(t *testing.T) {
	cases := map[string]func(n *fakeNarrator){
		"Polly refuses the timings": func(n *fakeNarrator) { n.marksErr = errors.New("throttled") },
		"the timings are garbage":   func(n *fakeNarrator) { n.marks = func(string) string { return "not json" } },
		"the timings hold no words": func(n *fakeNarrator) { n.marks = func(string) string { return "" } },
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t)
			setup(e.narrator)
			job := e.jobWith(2)
			var logged []string
			e.deps.Logf = func(f string, a ...any) { logged = append(logged, fmt.Sprintf(f, a...)) }
			if err := NarrateScenes(context.Background(), e.deps, job, nil); err != nil {
				t.Fatalf("a captions problem failed the job: %v", err)
			}
			for i, s := range job.Scenes {
				if s.Voice == nil || s.Voice.AudioKey == "" {
					t.Fatalf("scene %d has no voice", i+1)
				}
				if s.Voice.MarksKey != "" {
					t.Errorf("scene %d records timings that are unusable", i+1)
				}
			}
			if len(logged) == 0 || !strings.Contains(strings.Join(logged, "\n"), "spread evenly") {
				t.Errorf("the fallback was not logged: %v", logged)
			}
		})
	}
}

func TestNarrateScenes_APartlyBadAnswerFromPollyIsRepairedNotRefused(t *testing.T) {
	e := newEnv(t)
	// Offsets pointing outside the text: WordsFromMarks repairs these.
	e.narrator.marks = func(string) string {
		return `{"time":0,"type":"word","start":900,"end":903,"value":"x"}`
	}
	job := e.jobWith(1)
	if err := NarrateScenes(context.Background(), e.deps, job, nil); err != nil {
		t.Fatal(err)
	}
	if job.Scenes[0].Voice.MarksKey == "" {
		t.Error("timings that can be repaired were thrown away")
	}
}

func TestNarrateScenes_FailureCases(t *testing.T) {
	cases := []struct {
		name      string
		setup     func(e *env, j *Job)
		scene     int
		code      string
		retryable bool
		mention   string
	}{
		{"Polly fails", func(e *env, _ *Job) { e.narrator.audioErr = errors.New("service unavailable") },
			1, CodeNarrationFailed, true, "voice could not be made"},
		{"the audio is empty", func(e *env, _ *Job) {
			e.prober.info = func(string, string) (MediaInfo, error) { return MediaInfo{Duration: 0.01}, nil }
		}, 1, CodeNarrationFailed, true, "voice could not be made"},
		{"the audio cannot be read", func(e *env, _ *Job) {
			e.prober.info = func(string, string) (MediaInfo, error) { return MediaInfo{}, errors.New("invalid data") }
		}, 1, CodeNarrationFailed, true, "voice could not be made"},
		{"a scene has no narration", func(_ *env, j *Job) { j.Scenes[1].Narration = "" },
			2, CodeNarrationFailed, false, "no narration"},
		{"storage fails", func(e *env, _ *Job) {
			e.files.putErr["storyvideo-work/u1/j1/scene-01-voice.mp3"] = errors.New("s3 down")
		},
			1, CodeInternal, true, "our side"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newEnv(t)
			job := e.jobWith(3)
			c.setup(e, job)
			se := stepErr(t, NarrateScenes(context.Background(), e.deps, job, nil))
			if se.Scene != c.scene || se.Code != c.code || se.Retryable != c.retryable || !strings.Contains(se.Message, c.mention) {
				t.Errorf("got %+v", se)
			}
			if job.TotalSeconds != 0 {
				t.Error("a length was recorded for a job that failed")
			}
		})
	}
}

func TestNarrateScenes_AStoryThatRunsOverTheLimitOnceSpokenFailsBeforeAnyRendering(t *testing.T) {
	e := newEnv(t)
	job := e.jobWith(10)
	// 10 scenes of 9.5 s of speech: 10 x 9.85 + 0.4 = 98.9 s, over 90.
	e.prober.info = func(string, string) (MediaInfo, error) { return MediaInfo{Duration: 9.5}, nil }
	err := NarrateScenes(context.Background(), e.deps, job, nil)
	se := stepErr(t, err)
	if se.Code != CodeNarrationTooLong || se.Retryable || se.Scene != 0 {
		t.Errorf("got %+v", se)
	}
	for _, want := range []string{"99 seconds", "90 second limit", "Shorten the narration"} {
		if !strings.Contains(se.Message, want) {
			t.Errorf("message %q lacks %q", se.Message, want)
		}
	}
}

func TestNarrateScenes_StopsWhenCancelled(t *testing.T) {
	e := newEnv(t)
	job := e.jobWith(3)
	ctx, cancel := context.WithCancel(context.Background())
	e.narrator.audioErr = nil
	e.prober.info = func(string, string) (MediaInfo, error) { cancel(); return MediaInfo{Duration: 3}, nil }
	err := NarrateScenes(ctx, e.deps, job, nil)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v", err)
	}
	if len(e.narrator.calls) > 2 {
		t.Errorf("%d Polly calls after cancel", len(e.narrator.calls))
	}
}
