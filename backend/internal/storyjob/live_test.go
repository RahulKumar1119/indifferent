//go:build live

package storyjob

import (
	"context"
	"os/exec"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/polly"
	"github.com/rahul/indifferent/backend/internal/narrator"
	"github.com/rahul/indifferent/backend/internal/storyvideo"
)

// Run with: go test -tags live -run TestLive ./internal/storyjob/
//
// It calls the real Amazon Polly (standard engine, a few sentences, a fraction
// of a cent) and reads the audio with the real ffprobe. Storage is in memory,
// so nothing is written to AWS. It needs AWS credentials and ffprobe.

func liveDeps(t *testing.T) Deps {
	t.Helper()
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe is not installed")
	}
	cfg, err := config.LoadDefaultConfig(context.Background(), config.WithRegion("ap-south-1"))
	if err != nil {
		t.Skipf("no AWS configuration: %v", err)
	}
	e := newEnv(t)
	e.deps.Narrator = narrator.NewPollyNarrationService(polly.NewFromConfig(cfg))
	e.deps.Prober = FFprobe{}
	e.deps.Logf = t.Logf
	return e.deps
}

func TestLive_NarrationAgainstRealPolly(t *testing.T) {
	d := liveDeps(t)
	e := &env{files: d.Files.(*fakeFiles)}
	_ = e
	for _, tc := range []struct {
		name, lang, voice string
		texts             []string
	}{
		{"English", "en", "Joanna", []string{
			"The lighthouse stood alone on the cliff, and every ship that passed knew its light by heart.",
			"Every night the keeper climbed the long spiral stairs to light the great lamp above the sea.",
		}},
		{"Hindi", "hi", "Aditi", []string{
			"लाइटहाउस चट्टान पर अकेला खड़ा था, और हर जहाज़ उसकी रोशनी को दिल से पहचानता था।",
			"हर रात रखवाला लंबी सीढ़ियाँ चढ़कर समुद्र के ऊपर बड़ा दीपक जलाता था।",
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			job := &Job{UserID: "live", JobID: "live-" + tc.lang, Language: tc.lang, Voice: tc.voice}
			for _, text := range tc.texts {
				job.Scenes = append(job.Scenes, Scene{Narration: text})
			}
			d.WorkDir = t.TempDir()
			d.Files = newFiles()
			ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
			defer cancel()
			if err := NarrateScenes(ctx, d, job, nil); err != nil {
				t.Fatalf("NarrateScenes: %v", err)
			}
			files := d.Files.(*fakeFiles)
			for i, s := range job.Scenes {
				if s.Voice == nil || s.Voice.Seconds < 1.5 || s.Voice.Seconds > 20 {
					t.Fatalf("scene %d voice = %+v", i+1, s.Voice)
				}
				if s.Voice.MarksKey == "" {
					t.Errorf("scene %d: Polly's word timings were not usable", i+1)
					continue
				}
				words, err := storyvideo.WordsFromMarks(s.Narration, files.files["bucket/"+s.Voice.MarksKey], s.Voice.Seconds)
				if err != nil {
					t.Fatalf("scene %d: %v", i+1, err)
				}
				last := words[len(words)-1]
				// The last word must start inside the audio the ffprobe measured, and
				// the number of caption words must match the words in the text.
				if last.Start >= s.Voice.Seconds {
					t.Errorf("scene %d: the last word starts at %.2fs but the audio is %.2fs", i+1, last.Start, s.Voice.Seconds)
				}
				t.Logf("scene %d: %d words over %.2fs of audio (planner would have said %.1fs); last word %q at %.2fs",
					i+1, len(words), s.Voice.Seconds, float64(len(splitWords(s.Narration)))/2.5, last.Text, last.Start)
			}
			t.Logf("total video length %.2fs", job.TotalSeconds)
		})
	}
}

func splitWords(s string) []string {
	var out []string
	cur := ""
	for _, r := range s {
		if r == ' ' || r == '\n' {
			if cur != "" {
				out = append(out, cur)
				cur = ""
			}
			continue
		}
		cur += string(r)
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}
