package slidegen

import (
	"bytes"
	"image/png"
	"testing"

	"github.com/rahul/indifferent/backend/internal/models"
)

func TestNativeRenderer_QuestionSlide(t *testing.T) {
	renderer := NewNativeRenderer()

	data := QuestionSlideData{
		QuestionNumber: 1,
		TotalQuestions: 5,
		QuestionText:   "What is the capital of France?",
		Options: []models.Option{
			{Label: "A", Text: "London"},
			{Label: "B", Text: "Paris"},
			{Label: "C", Text: "Berlin"},
			{Label: "D", Text: "Madrid"},
		},
		CorrectIndex: 1,
	}

	pngBytes, err := renderer.RenderTemplate("classic/question.html", data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify it's a valid PNG
	img, err := png.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		t.Fatalf("output is not a valid PNG: %v", err)
	}

	bounds := img.Bounds()
	if bounds.Dx() != 1920 || bounds.Dy() != 1080 {
		t.Errorf("expected 1920x1080, got %dx%d", bounds.Dx(), bounds.Dy())
	}
}

func TestNativeRenderer_AnswerRevealSlide(t *testing.T) {
	renderer := NewNativeRenderer()

	data := QuestionSlideData{
		QuestionNumber: 2,
		TotalQuestions: 5,
		QuestionText:   "Which planet is closest to the Sun?",
		Options: []models.Option{
			{Label: "A", Text: "Venus"},
			{Label: "B", Text: "Mercury"},
			{Label: "C", Text: "Mars"},
			{Label: "D", Text: "Earth"},
		},
		CorrectIndex: 1,
	}

	pngBytes, err := renderer.RenderTemplate("classic/answer-reveal.html", data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	img, err := png.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		t.Fatalf("output is not a valid PNG: %v", err)
	}

	bounds := img.Bounds()
	if bounds.Dx() != 1920 || bounds.Dy() != 1080 {
		t.Errorf("expected 1920x1080, got %dx%d", bounds.Dx(), bounds.Dy())
	}
}

func TestNativeRenderer_CountdownSlide(t *testing.T) {
	renderer := NewNativeRenderer()

	for sec := 1; sec <= 5; sec++ {
		data := CountdownSlideData{Seconds: sec}
		pngBytes, err := renderer.RenderTemplate("classic/countdown.html", data)
		if err != nil {
			t.Fatalf("unexpected error for seconds=%d: %v", sec, err)
		}

		img, err := png.Decode(bytes.NewReader(pngBytes))
		if err != nil {
			t.Fatalf("output is not a valid PNG for seconds=%d: %v", sec, err)
		}

		bounds := img.Bounds()
		if bounds.Dx() != 1920 || bounds.Dy() != 1080 {
			t.Errorf("seconds=%d: expected 1920x1080, got %dx%d", sec, bounds.Dx(), bounds.Dy())
		}
	}
}

func TestNativeRenderer_OutroSlide(t *testing.T) {
	renderer := NewNativeRenderer()

	data := OutroSlideData{ChannelName: "Quiz Channel"}
	pngBytes, err := renderer.RenderTemplate("classic/outro.html", data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	img, err := png.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		t.Fatalf("output is not a valid PNG: %v", err)
	}

	bounds := img.Bounds()
	if bounds.Dx() != 1920 || bounds.Dy() != 1080 {
		t.Errorf("expected 1920x1080, got %dx%d", bounds.Dx(), bounds.Dy())
	}
}

func TestNativeRenderer_UnsupportedDataType(t *testing.T) {
	renderer := NewNativeRenderer()

	_, err := renderer.RenderTemplate("classic/question.html", "invalid data")
	if err == nil {
		t.Fatal("expected error for unsupported data type, got nil")
	}
}

func TestNativeRenderer_LongQuestionTextWraps(t *testing.T) {
	renderer := NewNativeRenderer()

	data := QuestionSlideData{
		QuestionNumber: 1,
		TotalQuestions: 1,
		QuestionText:   "This is a very long question text that should be wrapped across multiple lines when rendered on the slide because it exceeds the maximum line width defined in the wrapText function which is set to approximately eighty characters per line for readability.",
		Options: []models.Option{
			{Label: "A", Text: "Option A"},
			{Label: "B", Text: "Option B"},
		},
		CorrectIndex: 0,
	}

	pngBytes, err := renderer.RenderTemplate("classic/question.html", data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(pngBytes) == 0 {
		t.Fatal("expected non-empty PNG output")
	}
}
