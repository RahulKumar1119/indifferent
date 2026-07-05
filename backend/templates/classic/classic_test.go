package classic_test

import (
	"bytes"
	"html/template"
	"os"
	"path/filepath"
	"testing"
)

// Option matches the models.Option struct used for template rendering.
type Option struct {
	Label string
	Text  string
}

// QuestionData represents the data passed to question.html and answer-reveal.html.
type QuestionData struct {
	QuestionNumber int
	QuestionText   string
	Options        []Option
	TotalQuestions  int
	CorrectIndex   int // used only in answer-reveal
}

// CountdownData represents the data passed to countdown.html.
type CountdownData struct {
	Seconds int
}

// OutroData represents the data passed to outro.html.
type OutroData struct {
	ChannelName string
}

func templateDir() string {
	// When running tests, the working directory is the package directory.
	return "."
}

func TestQuestionTemplate(t *testing.T) {
	tmplPath := filepath.Join(templateDir(), "question.html")
	content, err := os.ReadFile(tmplPath)
	if err != nil {
		t.Fatalf("failed to read question.html: %v", err)
	}

	tmpl, err := template.New("question").Parse(string(content))
	if err != nil {
		t.Fatalf("failed to parse question.html: %v", err)
	}

	data := QuestionData{
		QuestionNumber: 1,
		QuestionText:   "What is the capital of France?",
		Options: []Option{
			{Label: "A", Text: "London"},
			{Label: "B", Text: "Paris"},
			{Label: "C", Text: "Berlin"},
			{Label: "D", Text: "Madrid"},
		},
		TotalQuestions: 10,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		t.Fatalf("failed to execute question.html: %v", err)
	}

	output := buf.String()
	if len(output) == 0 {
		t.Fatal("question.html produced empty output")
	}

	// Verify key content is present
	assertContains(t, output, "Question 1")
	assertContains(t, output, "What is the capital of France?")
	assertContains(t, output, "Question 1 of 10")
	assertContains(t, output, "Paris")
	assertContains(t, output, "1920px")
	assertContains(t, output, "1080px")
}

func TestAnswerRevealTemplate(t *testing.T) {
	tmplPath := filepath.Join(templateDir(), "answer-reveal.html")
	content, err := os.ReadFile(tmplPath)
	if err != nil {
		t.Fatalf("failed to read answer-reveal.html: %v", err)
	}

	tmpl, err := template.New("answer-reveal").Parse(string(content))
	if err != nil {
		t.Fatalf("failed to parse answer-reveal.html: %v", err)
	}

	data := QuestionData{
		QuestionNumber: 3,
		QuestionText:   "Which planet is closest to the Sun?",
		Options: []Option{
			{Label: "A", Text: "Venus"},
			{Label: "B", Text: "Mercury"},
			{Label: "C", Text: "Earth"},
			{Label: "D", Text: "Mars"},
		},
		TotalQuestions:  5,
		CorrectIndex: 1, // Mercury (0-based)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		t.Fatalf("failed to execute answer-reveal.html: %v", err)
	}

	output := buf.String()
	if len(output) == 0 {
		t.Fatal("answer-reveal.html produced empty output")
	}

	// Verify the correct answer is highlighted
	assertContains(t, output, "correct")
	assertContains(t, output, "✓ Correct")
	assertContains(t, output, "Which planet is closest to the Sun?")
	assertContains(t, output, "#4ecca3") // green highlight color
}

func TestCountdownTemplate(t *testing.T) {
	tmplPath := filepath.Join(templateDir(), "countdown.html")
	content, err := os.ReadFile(tmplPath)
	if err != nil {
		t.Fatalf("failed to read countdown.html: %v", err)
	}

	tmpl, err := template.New("countdown").Parse(string(content))
	if err != nil {
		t.Fatalf("failed to parse countdown.html: %v", err)
	}

	data := CountdownData{Seconds: 5}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		t.Fatalf("failed to execute countdown.html: %v", err)
	}

	output := buf.String()
	if len(output) == 0 {
		t.Fatal("countdown.html produced empty output")
	}

	assertContains(t, output, "5")
	assertContains(t, output, "1920px")
	assertContains(t, output, "1080px")
}

func TestOutroTemplate(t *testing.T) {
	tmplPath := filepath.Join(templateDir(), "outro.html")
	content, err := os.ReadFile(tmplPath)
	if err != nil {
		t.Fatalf("failed to read outro.html: %v", err)
	}

	tmpl, err := template.New("outro").Parse(string(content))
	if err != nil {
		t.Fatalf("failed to parse outro.html: %v", err)
	}

	data := OutroData{ChannelName: "Quiz Master"}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		t.Fatalf("failed to execute outro.html: %v", err)
	}

	output := buf.String()
	if len(output) == 0 {
		t.Fatal("outro.html produced empty output")
	}

	assertContains(t, output, "Quiz Master")
	assertContains(t, output, "Thanks for watching!")
	assertContains(t, output, "Subscribe")
	assertContains(t, output, "Like")
}

func assertContains(t *testing.T, haystack, needle string) {
	t.Helper()
	if !containsStr(haystack, needle) {
		t.Errorf("expected output to contain %q, but it did not", needle)
	}
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && contains(s, substr))
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
