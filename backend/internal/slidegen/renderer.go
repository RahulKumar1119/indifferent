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

// Slide renderer payloads live in models so the API can build a preview
// request without importing this package's Playwright dependency.
type QuestionSlideData = models.QuestionSlideData
type CountdownSlideData = models.CountdownSlideData
type OutroSlideData = models.OutroSlideData
