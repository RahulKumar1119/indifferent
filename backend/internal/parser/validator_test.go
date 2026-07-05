package parser

import (
	"errors"
	"strings"
	"testing"

	"github.com/rahul/indifferent/backend/internal/models"
)

func TestValidateFileSize(t *testing.T) {
	tests := []struct {
		name    string
		size    int
		wantErr error
	}{
		{"empty file", 0, nil},
		{"small file", 100, nil},
		{"exactly at limit", MaxFileSize, nil},
		{"one byte over limit", MaxFileSize + 1, ErrFileTooLarge},
		{"well over limit", MaxFileSize * 2, ErrFileTooLarge},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content := make([]byte, tt.size)
			err := ValidateFileSize(content)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("ValidateFileSize(%d bytes) = %v, want %v", tt.size, err, tt.wantErr)
			}
		})
	}
}

func TestSanitizeContent(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "plain text unchanged",
			input:    "1. What is Go?\nA) A language\nB) A game",
			expected: "1. What is Go?\nA) A language\nB) A game",
		},
		{
			name:     "removes HTML tags",
			input:    "<script>alert('xss')</script>Hello",
			expected: "alert('xss')Hello",
		},
		{
			name:     "removes nested tags",
			input:    "<div><b>Question</b></div>",
			expected: "Question",
		},
		{
			name:     "removes script injection via javascript:",
			input:    "javascript:alert(1)",
			expected: "alert(1)",
		},
		{
			name:     "removes event handler injection",
			input:    "onerror=alert(1)",
			expected: "alert(1)",
		},
		{
			name:     "case insensitive script removal",
			input:    "JAVASCRIPT:evil()",
			expected: "evil()",
		},
		{
			name:     "preserves newlines",
			input:    "line1\nline2\nline3",
			expected: "line1\nline2\nline3",
		},
		{
			name:     "preserves tabs",
			input:    "question\n\tA) option",
			expected: "question\n\tA) option",
		},
		{
			name:     "removes control characters",
			input:    "hello\x00world\x01test\x1F",
			expected: "helloworldtest",
		},
		{
			name:     "preserves carriage returns",
			input:    "line1\r\nline2",
			expected: "line1\r\nline2",
		},
		{
			name:     "removes self-closing tags",
			input:    "text<br/>more<img src='x'/>end",
			expected: "textmoreend",
		},
		{
			name:     "handles vbscript injection",
			input:    "vbscript:MsgBox",
			expected: "MsgBox",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeContent(tt.input)
			if result != tt.expected {
				t.Errorf("SanitizeContent(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSanitizeContent_PreservesQuestionFormat(t *testing.T) {
	// Ensure sanitization doesn't break valid TXT question formats
	inputs := []string{
		"1. What is 2+2?\nA) 3\nB) 4 *\nC) 5\nD) 6",
		"• What color is the sky?\n\tA) Red\n\tB) Blue *\n\tC) Green",
		"- How many continents?\n\tA) 5\n\tB) 6\n\tC) 7 *",
	}

	for _, input := range inputs {
		result := SanitizeContent(input)
		if result != input {
			t.Errorf("SanitizeContent should not modify valid question format.\nInput:  %q\nOutput: %q", input, result)
		}
	}
}

func TestValidateQuestions(t *testing.T) {
	validQuestion := models.Question{
		Index:        1,
		Text:         "What is Go?",
		Options:      []models.Option{{Label: "A", Text: "Language"}, {Label: "B", Text: "Game"}},
		CorrectIndex: 0,
	}

	tests := []struct {
		name      string
		questions []models.Question
		warnings  []models.Warning
		wantErr   bool
		errIs     error
	}{
		{
			name:      "valid questions no warnings",
			questions: []models.Question{validQuestion},
			warnings:  nil,
			wantErr:   false,
		},
		{
			name:      "valid questions with warnings",
			questions: []models.Question{validQuestion},
			warnings:  []models.Warning{{Line: 5, Reason: "missing options"}},
			wantErr:   false,
		},
		{
			name:      "no questions no warnings",
			questions: nil,
			warnings:  nil,
			wantErr:   true,
			errIs:     ErrNoQuestions,
		},
		{
			name:      "no questions with warnings (all malformed)",
			questions: nil,
			warnings:  []models.Warning{{Line: 1, Reason: "no options"}},
			wantErr:   true,
			errIs:     ErrNoQuestions,
		},
		{
			name:      "empty questions slice no warnings",
			questions: []models.Question{},
			warnings:  nil,
			wantErr:   true,
			errIs:     ErrNoQuestions,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateQuestions(tt.questions, tt.warnings)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateQuestions() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.errIs != nil && !errors.Is(err, tt.errIs) {
				t.Errorf("ValidateQuestions() error = %v, want errors.Is %v", err, tt.errIs)
			}
		})
	}
}

func TestValidateQuestion(t *testing.T) {
	tests := []struct {
		name     string
		question models.Question
		wantMsg  string
	}{
		{
			name: "valid question",
			question: models.Question{
				Index:        1,
				Text:         "What is Go?",
				Options:      []models.Option{{Label: "A", Text: "Language"}, {Label: "B", Text: "Game"}},
				CorrectIndex: 0,
			},
			wantMsg: "",
		},
		{
			name: "empty question text",
			question: models.Question{
				Index:        1,
				Text:         "",
				Options:      []models.Option{{Label: "A", Text: "Language"}},
				CorrectIndex: 0,
			},
			wantMsg: "question has no text",
		},
		{
			name: "whitespace-only question text",
			question: models.Question{
				Index:        1,
				Text:         "   ",
				Options:      []models.Option{{Label: "A", Text: "Language"}},
				CorrectIndex: 0,
			},
			wantMsg: "question has no text",
		},
		{
			name: "no options",
			question: models.Question{
				Index:        1,
				Text:         "What is Go?",
				Options:      nil,
				CorrectIndex: 0,
			},
			wantMsg: "question has no options",
		},
		{
			name: "empty options slice",
			question: models.Question{
				Index:        1,
				Text:         "What is Go?",
				Options:      []models.Option{},
				CorrectIndex: 0,
			},
			wantMsg: "question has no options",
		},
		{
			name: "option with empty text",
			question: models.Question{
				Index:        1,
				Text:         "What is Go?",
				Options:      []models.Option{{Label: "A", Text: "Language"}, {Label: "B", Text: ""}},
				CorrectIndex: 0,
			},
			wantMsg: "option 2 has no text",
		},
		{
			name: "correct index out of range (negative)",
			question: models.Question{
				Index:        1,
				Text:         "What is Go?",
				Options:      []models.Option{{Label: "A", Text: "Language"}, {Label: "B", Text: "Game"}},
				CorrectIndex: -1,
			},
			wantMsg: "no correct answer indicated",
		},
		{
			name: "correct index out of range (too high)",
			question: models.Question{
				Index:        1,
				Text:         "What is Go?",
				Options:      []models.Option{{Label: "A", Text: "Language"}, {Label: "B", Text: "Game"}},
				CorrectIndex: 2,
			},
			wantMsg: "no correct answer indicated",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ValidateQuestion(tt.question)
			if got != tt.wantMsg {
				t.Errorf("ValidateQuestion() = %q, want %q", got, tt.wantMsg)
			}
		})
	}
}

func TestSanitizeContent_LargeInput(t *testing.T) {
	// Verify sanitization handles large inputs without issues
	var sb strings.Builder
	for i := 0; i < 1000; i++ {
		sb.WriteString("1. Question number " + strings.Repeat("x", 100) + "\n")
		sb.WriteString("A) Option A\n")
		sb.WriteString("B) Option B *\n\n")
	}
	input := sb.String()
	result := SanitizeContent(input)
	if result != input {
		t.Error("SanitizeContent should not modify valid content without injection patterns")
	}
}
