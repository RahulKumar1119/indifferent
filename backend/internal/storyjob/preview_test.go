package storyjob

import (
	"context"
	"strings"
	"testing"
)

func TestPreviewKey_IsStableAndSeparatesWhatChangesTheSound(t *testing.T) {
	a := PreviewKey("u1", "Hello there.", "Joanna")
	if a != PreviewKey("u1", "Hello there.", "Joanna") {
		t.Fatal("the same text, voice and user must give the same key")
	}
	for name, other := range map[string]string{
		"another text":  PreviewKey("u1", "Hello there!", "Joanna"),
		"another voice": PreviewKey("u1", "Hello there.", "Matthew"),
		"another user":  PreviewKey("u2", "Hello there.", "Joanna"),
	} {
		if other == a {
			t.Errorf("%s must give another key", name)
		}
	}
	// A space moved between the voice and the text must not collide.
	if PreviewKey("u1", "Joanna x", "") == PreviewKey("u1", "x", "Joanna") {
		t.Error("fields must be separated in the hash")
	}
	if !strings.HasPrefix(a, PreviewPrefix+"u1/") || !strings.HasSuffix(a, ".mp3") {
		t.Errorf("key %q is not under the user's preview prefix", a)
	}
	if strings.Contains(a, "Hello") {
		t.Error("the key must not contain the text")
	}
}

func TestPreviewKey_PrefixesDoNotOverlapWithJobFilesOrVideos(t *testing.T) {
	for _, other := range []string{WorkPrefix, OutputPrefix} {
		if strings.HasPrefix(PreviewPrefix, other) || strings.HasPrefix(other, PreviewPrefix) {
			t.Errorf("%q overlaps %q: each needs its own lifecycle rule", PreviewPrefix, other)
		}
	}
}

func TestNarrateScenes_UsesTheTakeTheUserHeardInsteadOfCallingPollyAgain(t *testing.T) {
	e := newEnv(t)
	job := e.jobWith(2)
	heard := PreviewKey(job.UserID, job.Scenes[0].Narration, job.Voice)
	e.files.files["bucket/"+heard] = []byte("MP3:the take the user heard")
	if err := NarrateScenes(context.Background(), e.deps, job, nil); err != nil {
		t.Fatal(err)
	}
	if got := string(e.files.files["bucket/"+job.Scenes[0].Voice.AudioKey]); got != "MP3:the take the user heard" {
		t.Errorf("scene 1 audio = %q, want the previewed bytes", got)
	}
	for _, c := range e.narrator.calls {
		if strings.HasPrefix(c, "audio:") && strings.Contains(c, job.Scenes[0].Narration) {
			t.Errorf("Polly audio was called again for the previewed scene: %v", e.narrator.calls)
		}
	}
	// The scene that was not previewed is made as before, and the timings of
	// both are still made at render time.
	if got := string(e.files.files["bucket/"+job.Scenes[1].Voice.AudioKey]); got != "MP3:"+job.Scenes[1].Narration {
		t.Errorf("scene 2 audio = %q", got)
	}
	marks := 0
	for _, c := range e.narrator.calls {
		if strings.HasPrefix(c, "marks:") {
			marks++
		}
	}
	if marks != 2 {
		t.Errorf("%d timing calls, want 2: %v", marks, e.narrator.calls)
	}
}

func TestNarrateScenes_APreviewOfAnotherTextOrVoiceIsNotUsed(t *testing.T) {
	e := newEnv(t)
	job := e.jobWith(1)
	e.files.files["bucket/"+PreviewKey(job.UserID, "some other words", job.Voice)] = []byte("WRONG")
	e.files.files["bucket/"+PreviewKey(job.UserID, job.Scenes[0].Narration, "Matthew")] = []byte("WRONG")
	e.files.files["bucket/"+PreviewKey("someone-else", job.Scenes[0].Narration, job.Voice)] = []byte("WRONG")
	if err := NarrateScenes(context.Background(), e.deps, job, nil); err != nil {
		t.Fatal(err)
	}
	if got := string(e.files.files["bucket/"+job.Scenes[0].Voice.AudioKey]); got != "MP3:"+job.Scenes[0].Narration {
		t.Errorf("audio = %q, want a take made for this text and voice", got)
	}
}

func TestNarrateScenes_AnEmptyOrUnreadablePreviewFallsBackToPolly(t *testing.T) {
	for name, content := range map[string][]byte{"empty": {}, "present": []byte("ok")} {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t)
			job := e.jobWith(1)
			key := PreviewKey(job.UserID, job.Scenes[0].Narration, job.Voice)
			e.files.files["bucket/"+key] = content
			if name == "present" {
				// Listed but not readable: the object vanished between the check and the read.
				e.deps.Files = &vanishingFiles{fakeFiles: e.files, key: "bucket/" + key}
			}
			if err := NarrateScenes(context.Background(), e.deps, job, nil); err != nil {
				t.Fatal(err)
			}
			got := string(e.files.files["bucket/"+job.Scenes[0].Voice.AudioKey])
			if name == "empty" && got != "MP3:"+job.Scenes[0].Narration {
				t.Errorf("an empty take must not be used: %q", got)
			}
			if name == "present" && got != "MP3:"+job.Scenes[0].Narration {
				t.Errorf("an unreadable take must fall back to Polly: %q", got)
			}
		})
	}
}

// vanishingFiles reports key as present but fails to read it.
type vanishingFiles struct {
	*fakeFiles
	key string
}

func (v *vanishingFiles) DownloadToFile(ctx context.Context, bucket, key, path string) error {
	if bucket+"/"+key == v.key {
		return context.DeadlineExceeded
	}
	return v.fakeFiles.DownloadToFile(ctx, bucket, key, path)
}
