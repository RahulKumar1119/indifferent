package narrator

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/rahul/indifferent/backend/internal/models"
	"github.com/rahul/indifferent/backend/internal/storage"
)

// StorageClient defines the subset of storage operations needed by the handler.
type StorageClient interface {
	GetObject(ctx context.Context, bucket, key string) ([]byte, error)
	PutObject(ctx context.Context, bucket, key string, data []byte, contentType string) error
}

// Handler encapsulates the narrator handler dependencies.
type Handler struct {
	Narration NarrationService
	Storage   StorageClient
	Bucket    string
}

// NewHandler creates a Handler with the given dependencies.
func NewHandler(narration NarrationService, s3Client StorageClient, bucket string) *Handler {
	return &Handler{
		Narration: narration,
		Storage:   s3Client,
		Bucket:    bucket,
	}
}

// NewHandlerFromEnv creates a Handler using environment variables and real AWS clients.
func NewHandlerFromEnv(ctx context.Context, pollyClient PollyClient) (*Handler, error) {
	bucket := os.Getenv("S3_BUCKET")
	if bucket == "" {
		return nil, fmt.Errorf("S3_BUCKET environment variable not set")
	}

	s3Client, err := storage.NewS3Client(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize S3 client: %w", err)
	}

	narrationService := NewPollyNarrationService(pollyClient)

	return &Handler{
		Narration: narrationService,
		Storage:   s3Client,
		Bucket:    bucket,
	}, nil
}

// HandleRequest processes a NarratorInput event and returns NarratorOutput.
// It generates two audio files per question:
//   - q{N}.mp3 — reads question text and options (plays during question slide)
//   - q{N}_answer.mp3 — reads "The correct answer is..." (plays during answer reveal slide)
//
// The AudioKeys output is interleaved: [q0.mp3, q0_answer.mp3, q1.mp3, q1_answer.mp3, ...]
func (h *Handler) HandleRequest(ctx context.Context, input models.NarratorInput) (models.NarratorOutput, error) {
	// Read parsed JSON from S3
	data, err := h.Storage.GetObject(ctx, h.Bucket, input.JSONKey)
	if err != nil {
		return models.NarratorOutput{}, fmt.Errorf("failed to read parsed JSON from S3: %w", err)
	}

	// Unmarshal questions
	var questions []models.Question
	if err := json.Unmarshal(data, &questions); err != nil {
		return models.NarratorOutput{}, fmt.Errorf("failed to unmarshal questions JSON: %w", err)
	}

	var audioKeys []string
	var failed []int

	// Generate narration for each question (question audio + answer audio)
	for _, q := range questions {
		// 1. Synthesize question narration (question text + options)
		audioBytes, err := h.Narration.Synthesize(ctx, q, input.Voice)
		if err != nil {
			failed = append(failed, q.Index)
			continue
		}

		// Upload question MP3 to S3
		questionKey := fmt.Sprintf("temp/%s/audio/q%d.mp3", input.ProjectID, q.Index)
		if err := h.Storage.PutObject(ctx, h.Bucket, questionKey, audioBytes, "audio/mpeg"); err != nil {
			failed = append(failed, q.Index)
			continue
		}
		audioKeys = append(audioKeys, questionKey)

		// 2. Synthesize answer narration ("The correct answer is...")
		answerText := BuildAnswerNarrationText(q)
		if answerText == "" {
			// No correct answer marked — use a brief fallback
			answerText = "Let's see the answer."
		}

		answerBytes, err := h.Narration.SynthesizeText(ctx, answerText, input.Voice)
		if err != nil {
			failed = append(failed, q.Index)
			continue
		}

		answerKey := fmt.Sprintf("temp/%s/audio/q%d_answer.mp3", input.ProjectID, q.Index)
		if err := h.Storage.PutObject(ctx, h.Bucket, answerKey, answerBytes, "audio/mpeg"); err != nil {
			failed = append(failed, q.Index)
			continue
		}
		audioKeys = append(audioKeys, answerKey)
	}

	return models.NarratorOutput{
		ProjectID: input.ProjectID,
		AudioKeys: audioKeys,
		Failed:    failed,
	}, nil
}
