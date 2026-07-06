//go:build playwright

// This file contains the Playwright-based renderer implementation.
// It is excluded from normal builds since Playwright is only available
// in the Lambda container runtime. Use the "playwright" build tag to include.
package slidegen

import (
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"time"

	playwright "github.com/mxschmitt/playwright-go"
	"github.com/rahul/indifferent/backend/internal/models"
)

// SlideRenderer renders HTML templates as PNG screenshots using Playwright.
// It implements SlideRendererInterface.
type SlideRenderer struct {
	templateDir    string
	viewportWidth  int
	viewportHeight int
}

// NewSlideRenderer creates a new SlideRenderer with the given template directory.
// The viewport is fixed at 1920x1080.
func NewSlideRenderer(templateDir string) *SlideRenderer {
	return &SlideRenderer{
		templateDir:    templateDir,
		viewportWidth:  1920,
		viewportHeight: 1080,
	}
}

// RenderTemplate renders the named template (e.g., "classic/question.html") with the
// given data and returns a PNG screenshot captured by Playwright at 1920x1080 resolution.
// It retries up to 2 additional times on failure (3 total attempts).
func (r *SlideRenderer) RenderTemplate(templateName string, data interface{}) ([]byte, error) {
	htmlContent, err := r.renderHTMLTemplate(templateName, data)
	if err != nil {
		return nil, fmt.Errorf("render template %q: %w", templateName, err)
	}

	tmpFile, err := os.CreateTemp("", "slide-*.html")
	if err != nil {
		return nil, fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.WriteString(htmlContent); err != nil {
		tmpFile.Close()
		return nil, fmt.Errorf("write temp file: %w", err)
	}
	tmpFile.Close()

	var pngBytes []byte
	var lastErr error
	const maxAttempts = 3

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		pngBytes, lastErr = r.captureScreenshot(tmpPath)
		if lastErr == nil {
			return pngBytes, nil
		}
		if attempt < maxAttempts {
			time.Sleep(time.Duration(attempt) * 500 * time.Millisecond)
		}
	}

	return nil, fmt.Errorf("screenshot failed after %d attempts: %w", maxAttempts, lastErr)
}

// renderHTMLTemplate loads and executes the named HTML template with the provided data.
func (r *SlideRenderer) renderHTMLTemplate(templateName string, data interface{}) (string, error) {
	templateFile := filepath.Join(r.templateDir, templateName)

	tmpl, err := template.ParseFiles(templateFile)
	if err != nil {
		return "", fmt.Errorf("parse template file %q: %w", templateFile, err)
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template %q: %w", templateName, err)
	}

	return buf.String(), nil
}

// captureScreenshot launches a Playwright headless browser, navigates to the
// given HTML file, and captures a full-page PNG screenshot at 1920x1080.
func (r *SlideRenderer) captureScreenshot(htmlFilePath string) ([]byte, error) {
	pw, err := playwright.Run()
	if err != nil {
		return nil, fmt.Errorf("start playwright: %w", err)
	}
	defer pw.Stop()

	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(true),
	})
	if err != nil {
		return nil, fmt.Errorf("launch chromium: %w", err)
	}
	defer browser.Close()

	page, err := browser.NewPage(playwright.BrowserNewPageOptions{
		Viewport: &playwright.BrowserNewPageOptionsViewport{
			Width:  playwright.Int(r.viewportWidth),
			Height: playwright.Int(r.viewportHeight),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("create page: %w", err)
	}

	fileURL := "file://" + htmlFilePath
	if _, err := page.Goto(fileURL, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateNetworkidle,
	}); err != nil {
		return nil, fmt.Errorf("navigate to %q: %w", fileURL, err)
	}

	screenshot, err := page.Screenshot(playwright.PageScreenshotOptions{
		FullPage: playwright.Bool(true),
		Type:     playwright.ScreenshotTypePng,
	})
	if err != nil {
		return nil, fmt.Errorf("take screenshot: %w", err)
	}

	return screenshot, nil
}

// RenderQuestionSlides renders all question-related slides for a set of questions.
// For each question it renders: question slide, countdown (5 seconds), and answer-reveal slide.
func RenderQuestionSlides(questions []models.Question, templateDir string) ([][]byte, []int, error) {
	renderer := NewSlideRenderer(templateDir)
	totalQuestions := len(questions)

	var slides [][]byte
	var failed []int

	for i, q := range questions {
		questionFailed := false

		questionData := QuestionSlideData{
			QuestionNumber: q.Index + 1,
			QuestionText:   q.Text,
			Options:        q.Options,
			TotalQuestions:  totalQuestions,
		}
		qSlide, err := renderer.RenderTemplate("question.html", questionData)
		if err != nil {
			questionFailed = true
		} else {
			slides = append(slides, qSlide)
		}

		countdownData := CountdownSlideData{Seconds: 5}
		cSlide, err := renderer.RenderTemplate("countdown.html", countdownData)
		if err != nil {
			questionFailed = true
		} else {
			if !questionFailed {
				slides = append(slides, cSlide)
			}
		}

		answerData := QuestionSlideData{
			QuestionNumber: q.Index + 1,
			QuestionText:   q.Text,
			Options:        q.Options,
			TotalQuestions:  totalQuestions,
			CorrectIndex:   q.CorrectIndex,
		}
		aSlide, err := renderer.RenderTemplate("answer-reveal.html", answerData)
		if err != nil {
			questionFailed = true
		} else {
			if !questionFailed {
				slides = append(slides, aSlide)
			}
		}

		if questionFailed {
			failed = append(failed, i)
		}
	}

	return slides, failed, nil
}
