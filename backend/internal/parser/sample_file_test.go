package parser

import (
	"os"
	"strings"
	"testing"
)

// The sample file the app offers for download must be one the parser accepts,
// with every question complete. If the parser changes, this fails before a
// first-time visitor downloads a file that does not work.
func TestDownloadableSampleQuizParses(t *testing.T) {
	data, err := os.ReadFile("../../../frontend/public/samples/quiz-sample.txt")
	if err != nil {
		t.Fatalf("read sample: %v", err)
	}
	if err := ValidateFileSize(data); err != nil {
		t.Fatalf("sample too large: %v", err)
	}
	content := SanitizeContent(string(data))
	format, err := DetectFormat(content)
	if err != nil {
		t.Fatalf("sample format not detected: %v", err)
	}
	questions := ExtractQuestions(content, string(format))
	if len(questions) < 5 {
		t.Fatalf("want at least 5 questions in the sample, got %d", len(questions))
	}
	for _, q := range questions {
		if reason := ValidateQuestion(q); reason != "" {
			t.Errorf("question %d is not valid: %s", q.Index+1, reason)
		}
		if reason := MissingAnswerReason(q); reason != "" {
			t.Errorf("question %d has no marked answer: %s", q.Index+1, reason)
		}
	}
	if !strings.Contains(content, "Answer:") {
		t.Error("the sample should show the Answer: line")
	}
}
