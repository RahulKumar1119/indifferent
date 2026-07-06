package narrator

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/polly"
	"github.com/aws/aws-sdk-go-v2/service/polly/types"
	"github.com/rahul/indifferent/backend/internal/models"
)

// mockPollyClient implements PollyClient for testing.
type mockPollyClient struct {
	calls    int
	failN    int // fail the first N calls
	audioOut []byte
}

func (m *mockPollyClient) SynthesizeSpeech(ctx context.Context, params *polly.SynthesizeSpeechInput, optFns ...func(*polly.Options)) (*polly.SynthesizeSpeechOutput, error) {
	m.calls++
	if m.calls <= m.failN {
		return nil, errors.New("polly throttled")
	}
	return &polly.SynthesizeSpeechOutput{
		AudioStream: io.NopCloser(bytes.NewReader(m.audioOut)),
	}, nil
}

func TestBuildNarrationText(t *testing.T) {
	q := models.Question{
		Index: 0,
		Text:  "What is the capital of France",
		Options: []models.Option{
			{Label: "A", Text: "Berlin"},
			{Label: "B", Text: "Paris"},
			{Label: "C", Text: "London"},
			{Label: "D", Text: "Madrid"},
		},
		CorrectIndex: 1,
	}

	result := BuildNarrationText(q)
	expected := "What is the capital of France. Option A: Berlin. Option B: Paris. Option C: London. Option D: Madrid."
	if result != expected {
		t.Errorf("BuildNarrationText() =\n  %q\nwant:\n  %q", result, expected)
	}
}

func TestBuildNarrationText_TwoOptions(t *testing.T) {
	q := models.Question{
		Index: 1,
		Text:  "Is Go compiled",
		Options: []models.Option{
			{Label: "A", Text: "Yes"},
			{Label: "B", Text: "No"},
		},
		CorrectIndex: 0,
	}

	result := BuildNarrationText(q)
	expected := "Is Go compiled. Option A: Yes. Option B: No."
	if result != expected {
		t.Errorf("BuildNarrationText() =\n  %q\nwant:\n  %q", result, expected)
	}
}

func TestIsValidVoice(t *testing.T) {
	validVoices := []string{"Joanna", "Matthew", "Amy", "Brian", "Aditi"}
	for _, v := range validVoices {
		if !IsValidVoice(v) {
			t.Errorf("IsValidVoice(%q) = false, want true", v)
		}
	}

	invalidVoices := []string{"joanna", "MATTHEW", "Ruth", "Danielle", ""}
	for _, v := range invalidVoices {
		if IsValidVoice(v) {
			t.Errorf("IsValidVoice(%q) = true, want false", v)
		}
	}
}

func TestSynthesize_Success(t *testing.T) {
	expectedAudio := []byte("fake-mp3-data")
	mock := &mockPollyClient{audioOut: expectedAudio}
	svc := NewPollyNarrationService(mock)

	q := models.Question{
		Index: 0,
		Text:  "What is 2+2",
		Options: []models.Option{
			{Label: "A", Text: "3"},
			{Label: "B", Text: "4"},
		},
		CorrectIndex: 1,
	}

	result, err := svc.Synthesize(context.Background(), q, "Joanna")
	if err != nil {
		t.Fatalf("Synthesize() unexpected error: %v", err)
	}
	if !bytes.Equal(result, expectedAudio) {
		t.Errorf("Synthesize() returned %v, want %v", result, expectedAudio)
	}
	if mock.calls != 1 {
		t.Errorf("expected 1 Polly call, got %d", mock.calls)
	}
}

func TestSynthesize_InvalidVoice(t *testing.T) {
	mock := &mockPollyClient{audioOut: []byte("data")}
	svc := NewPollyNarrationService(mock)

	q := models.Question{
		Index: 0,
		Text:  "Test question",
		Options: []models.Option{
			{Label: "A", Text: "Answer"},
		},
		CorrectIndex: 0,
	}

	_, err := svc.Synthesize(context.Background(), q, "InvalidVoice")
	if err == nil {
		t.Fatal("Synthesize() expected error for invalid voice, got nil")
	}
	if mock.calls != 0 {
		t.Errorf("expected 0 Polly calls for invalid voice, got %d", mock.calls)
	}
}

func TestSynthesize_RetryOnError(t *testing.T) {
	expectedAudio := []byte("audio-after-retry")
	mock := &mockPollyClient{failN: 2, audioOut: expectedAudio}
	svc := NewPollyNarrationService(mock)

	q := models.Question{
		Index: 0,
		Text:  "Retry question",
		Options: []models.Option{
			{Label: "A", Text: "Opt1"},
			{Label: "B", Text: "Opt2"},
		},
		CorrectIndex: 0,
	}

	result, err := svc.Synthesize(context.Background(), q, "Matthew")
	if err != nil {
		t.Fatalf("Synthesize() unexpected error after retries: %v", err)
	}
	if !bytes.Equal(result, expectedAudio) {
		t.Errorf("Synthesize() returned %v, want %v", result, expectedAudio)
	}
	// Should have tried 3 times: first call fails, first retry fails, second retry succeeds
	if mock.calls != 3 {
		t.Errorf("expected 3 Polly calls (1 initial + 2 retries), got %d", mock.calls)
	}
}

func TestSynthesize_AllRetriesExhausted(t *testing.T) {
	mock := &mockPollyClient{failN: 10, audioOut: []byte("never")}
	svc := NewPollyNarrationService(mock)

	q := models.Question{
		Index: 0,
		Text:  "Exhausted retries",
		Options: []models.Option{
			{Label: "A", Text: "Opt1"},
		},
		CorrectIndex: 0,
	}

	_, err := svc.Synthesize(context.Background(), q, "Amy")
	if err == nil {
		t.Fatal("Synthesize() expected error when all retries exhausted, got nil")
	}
	// Should have attempted exactly 3 times total
	if mock.calls != 3 {
		t.Errorf("expected 3 Polly calls total, got %d", mock.calls)
	}
}

func TestSynthesize_UsesStandardEngine(t *testing.T) {
	var capturedInput *polly.SynthesizeSpeechInput
	mock := &capturingPollyClient{audioOut: []byte("data"), captured: &capturedInput}
	svc := NewPollyNarrationService(mock)

	q := models.Question{
		Index: 0,
		Text:  "Engine test",
		Options: []models.Option{
			{Label: "A", Text: "Answer"},
		},
		CorrectIndex: 0,
	}

	_, err := svc.Synthesize(context.Background(), q, "Brian")
	if err != nil {
		t.Fatalf("Synthesize() unexpected error: %v", err)
	}
	if capturedInput == nil {
		t.Fatal("expected captured input, got nil")
	}
	if capturedInput.Engine != types.EngineStandard {
		t.Errorf("expected standard engine, got %v", capturedInput.Engine)
	}
	if capturedInput.OutputFormat != types.OutputFormatMp3 {
		t.Errorf("expected mp3 output format, got %v", capturedInput.OutputFormat)
	}
	if capturedInput.VoiceId != types.VoiceId("Brian") {
		t.Errorf("expected Brian voice, got %v", capturedInput.VoiceId)
	}
}

// capturingPollyClient captures the input for assertions.
type capturingPollyClient struct {
	audioOut []byte
	captured **polly.SynthesizeSpeechInput
}

func (c *capturingPollyClient) SynthesizeSpeech(ctx context.Context, params *polly.SynthesizeSpeechInput, optFns ...func(*polly.Options)) (*polly.SynthesizeSpeechOutput, error) {
	*c.captured = params
	return &polly.SynthesizeSpeechOutput{
		AudioStream: io.NopCloser(bytes.NewReader(c.audioOut)),
	}, nil
}
