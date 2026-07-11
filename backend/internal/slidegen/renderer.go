// Package slidegen provides slide rendering functionality for the
// TXT-to-Video pipeline. It renders HTML templates into PNG images.
package slidegen

import (
	"github.com/rahul/indifferent/backend/internal/models"
)

// SlideRendererInterface defines the contract for rendering slides.
// Implementations may use Playwright, headless Chrome, native Go rendering, or mocks for testing.
type SlideRendererInterface interface {
	// RenderTemplate renders the named template with the given data and returns PNG bytes.
	RenderTemplate(templateName string, data interface{}) ([]byte, error)
}

// QuestionSlideData holds the template data for a question or answer-reveal slide.
type QuestionSlideData struct {
	QuestionNumber int             `json:"questionNumber"`
	TotalQuestions int             `json:"totalQuestions"`
	QuestionText   string          `json:"questionText"`
	Options        []models.Option `json:"options"`
	CorrectIndex   int             `json:"correctIndex"`
	CorrectIndices []int           `json:"correctIndices"`
}

// CountdownSlideData holds the template data for a countdown slide.
type CountdownSlideData struct {
	Seconds int `json:"seconds"`
}

// OutroSlideData holds the template data for the outro slide.
type OutroSlideData struct {
	ChannelName string `json:"channelName"`
}
