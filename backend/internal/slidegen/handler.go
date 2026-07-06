package slidegen

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

// Handler encapsulates the slide generation handler dependencies.
type Handler struct {
	Renderer SlideRendererInterface
	Storage  StorageClient
	Bucket   string
}

// NewHandler creates a Handler with the given dependencies.
func NewHandler(renderer SlideRendererInterface, s3Client StorageClient, bucket string) *Handler {
	return &Handler{
		Renderer: renderer,
		Storage:  s3Client,
		Bucket:   bucket,
	}
}

// NewHandlerFromEnv creates a Handler using environment variables and the real S3 client.
// The renderer must be provided externally (e.g., Playwright-based).
func NewHandlerFromEnv(ctx context.Context, renderer SlideRendererInterface) (*Handler, error) {
	bucket := os.Getenv("S3_BUCKET")
	if bucket == "" {
		return nil, fmt.Errorf("S3_BUCKET environment variable not set")
	}

	s3Client, err := storage.NewS3Client(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize S3 client: %w", err)
	}

	return &Handler{
		Renderer: renderer,
		Storage:  s3Client,
		Bucket:   bucket,
	}, nil
}

// HandleRequest processes a SlideGenInput event and returns SlideGenOutput.
func (h *Handler) HandleRequest(ctx context.Context, input models.SlideGenInput) (models.SlideGenOutput, error) {
	// Read parsed JSON from S3
	data, err := h.Storage.GetObject(ctx, h.Bucket, input.JSONKey)
	if err != nil {
		return models.SlideGenOutput{}, fmt.Errorf("failed to read parsed JSON from S3: %w", err)
	}

	// Unmarshal questions
	var questions []models.Question
	if err := json.Unmarshal(data, &questions); err != nil {
		return models.SlideGenOutput{}, fmt.Errorf("failed to unmarshal questions JSON: %w", err)
	}

	totalQuestions := len(questions)
	var slideKeys []string
	var failed []int

	// Render slides for each question
	for _, q := range questions {
		questionFailed := false

		// 1. Question slide
		qData := QuestionSlideData{
			QuestionNumber: q.Index + 1,
			TotalQuestions: totalQuestions,
			QuestionText:   q.Text,
			Options:        q.Options,
			CorrectIndex:   q.CorrectIndex,
		}

		questionKey := fmt.Sprintf("temp/%s/slides/q%d_question.png", input.ProjectID, q.Index)
		if err := h.renderAndUpload(ctx, input.Template, "question.html", qData, questionKey); err != nil {
			questionFailed = true
		} else {
			slideKeys = append(slideKeys, questionKey)
		}

		// 2. Countdown slides (5, 4, 3, 2, 1)
		if !questionFailed {
			for sec := 5; sec >= 1; sec-- {
				cData := CountdownSlideData{Seconds: sec}
				countdownKey := fmt.Sprintf("temp/%s/slides/q%d_countdown_%d.png", input.ProjectID, q.Index, sec)
				if err := h.renderAndUpload(ctx, input.Template, "countdown.html", cData, countdownKey); err != nil {
					questionFailed = true
					break
				}
				slideKeys = append(slideKeys, countdownKey)
			}
		}

		// 3. Answer reveal slide
		if !questionFailed {
			answerKey := fmt.Sprintf("temp/%s/slides/q%d_answer.png", input.ProjectID, q.Index)
			if err := h.renderAndUpload(ctx, input.Template, "answer-reveal.html", qData, answerKey); err != nil {
				questionFailed = true
			} else {
				slideKeys = append(slideKeys, answerKey)
			}
		}

		if questionFailed {
			failed = append(failed, q.Index)
		}
	}

	// Render outro slide
	outroData := OutroSlideData{ChannelName: "Quiz Channel"}
	outroKey := fmt.Sprintf("temp/%s/slides/outro.png", input.ProjectID)
	if err := h.renderAndUpload(ctx, input.Template, "outro.html", outroData, outroKey); err != nil {
		// Outro failure is non-fatal; we don't fail the whole job for it
		_ = err
	} else {
		slideKeys = append(slideKeys, outroKey)
	}

	return models.SlideGenOutput{
		ProjectID: input.ProjectID,
		SlideKeys: slideKeys,
		Failed:    failed,
	}, nil
}

// renderAndUpload renders a template and uploads the resulting PNG to S3.
func (h *Handler) renderAndUpload(ctx context.Context, template, templateFile string, data interface{}, s3Key string) error {
	// Construct the full template name: "{template}/{templateFile}"
	templateName := fmt.Sprintf("%s/%s", template, templateFile)

	pngData, err := h.Renderer.RenderTemplate(templateName, data)
	if err != nil {
		return fmt.Errorf("failed to render template %s: %w", templateName, err)
	}

	if err := h.Storage.PutObject(ctx, h.Bucket, s3Key, pngData, "image/png"); err != nil {
		return fmt.Errorf("failed to upload slide to S3 at %s: %w", s3Key, err)
	}

	return nil
}
