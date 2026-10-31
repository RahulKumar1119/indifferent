package narrator

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/polly"
	"github.com/aws/aws-sdk-go-v2/service/polly/types"
)

// recordingPolly keeps every request it receives and fails the first failN.
type recordingPolly struct {
	inputs []*polly.SynthesizeSpeechInput
	failN  int
	body   string
}

func (r *recordingPolly) SynthesizeSpeech(_ context.Context, in *polly.SynthesizeSpeechInput, _ ...func(*polly.Options)) (*polly.SynthesizeSpeechOutput, error) {
	r.inputs = append(r.inputs, in)
	if len(r.inputs) <= r.failN {
		return nil, errors.New("throttled")
	}
	return &polly.SynthesizeSpeechOutput{AudioStream: io.NopCloser(bytes.NewReader([]byte(r.body)))}, nil
}

func TestSynthesizeMarks_AsksForWordMarksAsJSON(t *testing.T) {
	r := &recordingPolly{body: `{"time":6,"type":"word","start":0,"end":3,"value":"The"}`}
	got, err := NewPollyNarrationService(r).SynthesizeMarks(context.Background(), "The cliff.", "Joanna")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != r.body {
		t.Errorf("body = %q", got)
	}
	in := r.inputs[0]
	if in.OutputFormat != types.OutputFormatJson {
		t.Errorf("OutputFormat = %s, want json", in.OutputFormat)
	}
	if len(in.SpeechMarkTypes) != 1 || in.SpeechMarkTypes[0] != types.SpeechMarkTypeWord {
		t.Errorf("SpeechMarkTypes = %v, want [word]", in.SpeechMarkTypes)
	}
}

// The word times are only correct for the audio if Polly is asked for the same
// words in the same voice by the same engine.
func TestSynthesizeMarks_DescribesExactlyTheAudioSynthesizeTextMakes(t *testing.T) {
	for _, voice := range SupportedVoices {
		r := &recordingPolly{body: "x"}
		svc := NewPollyNarrationService(r)
		text := "One scene of narration, with a comma."
		if _, err := svc.SynthesizeText(context.Background(), text, voice); err != nil {
			t.Fatal(err)
		}
		if _, err := svc.SynthesizeMarks(context.Background(), text, voice); err != nil {
			t.Fatal(err)
		}
		audio, marks := r.inputs[0], r.inputs[1]
		if audio.OutputFormat != types.OutputFormatMp3 {
			t.Errorf("%s: audio format = %s", voice, audio.OutputFormat)
		}
		if *audio.Text != *marks.Text || audio.VoiceId != marks.VoiceId || audio.Engine != marks.Engine ||
			audio.LanguageCode != marks.LanguageCode || audio.TextType != marks.TextType {
			t.Errorf("%s: the audio and its marks were asked for differently:\n audio %+v\n marks %+v", voice, audio, marks)
		}
	}
}

func TestSynthesizeMarks_RefusesAnUnknownVoice(t *testing.T) {
	r := &recordingPolly{}
	if _, err := NewPollyNarrationService(r).SynthesizeMarks(context.Background(), "hi", "Nobody"); err == nil {
		t.Error("an unsupported voice was accepted")
	}
	if len(r.inputs) != 0 {
		t.Error("Polly was called for an unsupported voice")
	}
}

func TestSynthesizeMarks_RetriesLikeTheAudioCallDoes(t *testing.T) {
	r := &recordingPolly{failN: 2, body: "ok"}
	got, err := NewPollyNarrationService(r).SynthesizeMarks(context.Background(), "hi", "Joanna")
	if err != nil || string(got) != "ok" {
		t.Fatalf("got %q, %v after two failures", got, err)
	}
	if len(r.inputs) != 3 {
		t.Errorf("%d calls, want 3", len(r.inputs))
	}
}

func TestSynthesizeMarks_GivesUpAfterTheRetries(t *testing.T) {
	r := &recordingPolly{failN: 99}
	_, err := NewPollyNarrationService(r).SynthesizeMarks(context.Background(), "hi", "Joanna")
	if err == nil || !strings.Contains(err.Error(), "after 3 attempts") {
		t.Errorf("err = %v", err)
	}
}

func TestSynthesizeMarks_StopsWhenTheContextIsCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r := &recordingPolly{failN: 99}
	_, err := NewPollyNarrationService(r).SynthesizeMarks(ctx, "hi", "Joanna")
	if err == nil || !strings.Contains(err.Error(), "cancelled") {
		t.Errorf("err = %v", err)
	}
	if len(r.inputs) > 1 {
		t.Errorf("%d calls after cancel", len(r.inputs))
	}
}
