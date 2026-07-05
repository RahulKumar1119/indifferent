// Package narrator provides voice-over narration generation using Amazon Polly.
package narrator

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/polly"
	"github.com/aws/aws-sdk-go-v2/service/polly/types"
	"github.com/rahul/indifferent/backend/internal/models"
)

// SupportedVoices lists all valid Amazon Polly voice IDs.
var SupportedVoices = []string{"Joanna", "Matthew", "Ruth", "Danielle", "Aditi"}

// maxRetries is the number of additional attempts after the first failure (3 total attempts).
const maxRetries = 2

// retryDelay is the base delay between retry attempts.
const retryDelay = 500 * time.Millisecond

// NarrationService defines the interface for generating narration audio from question data.
type NarrationService interface {
	// Synthesize generates MP3 audio bytes for the given question using the specified voice.
	Synthesize(ctx context.Context, question models.Question, voiceID string) ([]byte, error)
}

// PollyClient defines the subset of the Amazon Polly API used by the narration service.
// This allows for easy mocking in tests.
type PollyClient interface {
	SynthesizeSpeech(ctx context.Context, params *polly.SynthesizeSpeechInput, optFns ...func(*polly.Options)) (*polly.SynthesizeSpeechOutput, error)
}

// PollyNarrationService implements NarrationService using Amazon Polly.
type PollyNarrationService struct {
	client PollyClient
}

// NewPollyNarrationService creates a new PollyNarrationService with the given Polly client.
func NewPollyNarrationService(client PollyClient) *PollyNarrationService {
	return &PollyNarrationService{client: client}
}

// BuildNarrationText constructs the narration script from a question.
// Format: "<question text>. Option A: <text>. Option B: <text>. ..."
func BuildNarrationText(question models.Question) string {
	var sb strings.Builder
	sb.WriteString(question.Text)
	sb.WriteString(". ")
	for i, opt := range question.Options {
		sb.WriteString(fmt.Sprintf("Option %s: %s", opt.Label, opt.Text))
		if i < len(question.Options)-1 {
			sb.WriteString(". ")
		} else {
			sb.WriteString(".")
		}
	}
	return sb.String()
}

// IsValidVoice checks whether the provided voice ID is in the supported voices list.
func IsValidVoice(voiceID string) bool {
	for _, v := range SupportedVoices {
		if v == voiceID {
			return true
		}
	}
	return false
}

// Synthesize generates MP3 audio bytes for the given question using Amazon Polly.
// It retries up to 2 additional times (3 total attempts) on Polly errors.
func (s *PollyNarrationService) Synthesize(ctx context.Context, question models.Question, voiceID string) ([]byte, error) {
	if !IsValidVoice(voiceID) {
		return nil, fmt.Errorf("unsupported voice ID: %s", voiceID)
	}

	narrationText := BuildNarrationText(question)
	voice := types.VoiceId(voiceID)

	input := &polly.SynthesizeSpeechInput{
		OutputFormat: types.OutputFormatMp3,
		VoiceId:      voice,
		Engine:       types.EngineNeural,
		Text:         &narrationText,
	}

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("context cancelled during retry: %w", ctx.Err())
			case <-time.After(retryDelay * time.Duration(attempt)):
			}
		}

		output, err := s.client.SynthesizeSpeech(ctx, input)
		if err != nil {
			lastErr = err
			continue
		}

		audioBytes, err := io.ReadAll(output.AudioStream)
		if err != nil {
			lastErr = fmt.Errorf("failed to read audio stream: %w", err)
			continue
		}

		return audioBytes, nil
	}

	return nil, fmt.Errorf("polly synthesis failed after %d attempts: %w", maxRetries+1, lastErr)
}
