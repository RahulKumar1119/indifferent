package parser

import (
	"testing"

	"github.com/rahul/indifferent/backend/internal/models"
)

func TestExtractQuestions_Numbered_AnswerLine(t *testing.T) {
	content := `1. What is the capital of France?
A. London
B. Paris
C. Berlin
D. Madrid
Answer: B

2. Which planet is the Red Planet?
A. Venus
B. Jupiter
C. Mars
D. Saturn
Answer: C`

	qs := ExtractQuestions(content, "numbered")
	if len(qs) != 2 {
		t.Fatalf("expected 2 questions, got %d", len(qs))
	}
	if len(qs[0].CorrectIndices) != 1 || qs[0].CorrectIndices[0] != 1 {
		t.Errorf("Q1: expected correct index [1], got %v", qs[0].CorrectIndices)
	}
	if qs[0].CorrectIndex != 1 {
		t.Errorf("Q1: expected CorrectIndex 1, got %d", qs[0].CorrectIndex)
	}
	if len(qs[1].CorrectIndices) != 1 || qs[1].CorrectIndices[0] != 2 {
		t.Errorf("Q2: expected correct index [2], got %v", qs[1].CorrectIndices)
	}
}

func TestExtractQuestions_AnswerLineVariants(t *testing.T) {
	cases := []struct {
		name   string
		answer string
		want   []int
	}{
		{"uppercase", "Answer: B", []int{1}},
		{"lowercase", "Answer: b", []int{1}},
		{"spaced", "Answer:   B", []int{1}},
		{"no space after colon", "Answer:B", []int{1}},
		{"case insensitive word", "ANSWER: B", []int{1}},
		{"comma separated", "Answer: B, D", []int{1, 3}},
		{"and separated", "Answer: B and D", []int{1, 3}},
		{"slash separated", "Answer: B/D", []int{1, 3}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			content := "1. Q?\nA. one\nB. two\nC. three\nD. four\n" + tc.answer
			qs := ExtractQuestions(content, "numbered")
			if len(qs) != 1 {
				t.Fatalf("expected 1 question, got %d", len(qs))
			}
			got := qs[0].CorrectIndices
			if len(got) != len(tc.want) {
				t.Fatalf("expected correct indices %v, got %v", tc.want, got)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("expected correct indices %v, got %v", tc.want, got)
				}
			}
		})
	}
}

// The bulleted and tabbed formats name the answer by its full text, which is
// what the format guide teaches.
func TestExtractQuestions_AnswerLineByOptionText(t *testing.T) {
	cases := []struct {
		name    string
		content string
		format  string
		want    int
	}{
		{
			name:    "bulleted",
			content: "Largest planet?\n- Mercury\n- Jupiter\n- Saturn\nAnswer: Jupiter",
			format:  "bulleted",
			want:    1,
		},
		{
			name:    "tabbed",
			content: "Chemical symbol Au?\n\tSilver\n\tGold\n\tCopper\nAnswer: Gold",
			format:  "tabbed",
			want:    1,
		},
		{
			name:    "numbered by text",
			content: "1. Q?\nA. London\nB. Paris\nAnswer: Paris",
			format:  "numbered",
			want:    1,
		},
		{
			name:    "case insensitive text",
			content: "1. Q?\nA. London\nB. Paris\nAnswer: paris",
			format:  "numbered",
			want:    1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			qs := ExtractQuestions(tc.content, tc.format)
			if len(qs) != 1 {
				t.Fatalf("expected 1 question, got %d", len(qs))
			}
			if len(qs[0].CorrectIndices) != 1 || qs[0].CorrectIndices[0] != tc.want {
				t.Errorf("expected correct index [%d], got %v", tc.want, qs[0].CorrectIndices)
			}
		})
	}
}

// An option whose text contains a separator word must still match as a whole.
func TestExtractQuestions_AnswerLineOptionTextWithAnd(t *testing.T) {
	content := "1. Q?\nA. Salt and Pepper\nB. Ketchup\nAnswer: Salt and Pepper"
	qs := ExtractQuestions(content, "numbered")
	if len(qs) != 1 {
		t.Fatalf("expected 1 question, got %d", len(qs))
	}
	if len(qs[0].CorrectIndices) != 1 || qs[0].CorrectIndices[0] != 0 {
		t.Errorf("expected correct index [0], got %v", qs[0].CorrectIndices)
	}
}

func TestExtractQuestions_AnswerLineBeforeOptions(t *testing.T) {
	content := "1. Q?\nAnswer: B\nA. one\nB. two\nC. three"
	qs := ExtractQuestions(content, "numbered")
	if len(qs) != 1 {
		t.Fatalf("expected 1 question, got %d", len(qs))
	}
	if len(qs[0].CorrectIndices) != 1 || qs[0].CorrectIndices[0] != 1 {
		t.Errorf("expected correct index [1], got %v", qs[0].CorrectIndices)
	}
}

// An answer that names no option is ignored rather than guessed.
func TestExtractQuestions_AnswerLineUnknownIgnored(t *testing.T) {
	content := "1. Q?\nA. one\nB. two\nC. three\nAnswer: None of these"
	qs := ExtractQuestions(content, "numbered")
	if len(qs) != 1 {
		t.Fatalf("expected 1 question, got %d", len(qs))
	}
	if len(qs[0].CorrectIndices) != 0 {
		t.Errorf("expected no correct answer, got %v", qs[0].CorrectIndices)
	}
}

// The asterisk prefix marks an option as correct in the numbered format.
func TestExtractQuestions_AsteriskPrefixedOption(t *testing.T) {
	content := `1. What is the capital of France?
A. London
*B. Paris
C. Berlin
D. Madrid`
	qs := ExtractQuestions(content, "numbered")
	if len(qs) != 1 {
		t.Fatalf("expected 1 question, got %d", len(qs))
	}
	if len(qs[0].CorrectIndices) != 1 || qs[0].CorrectIndices[0] != 1 {
		t.Errorf("expected correct index [1], got %v", qs[0].CorrectIndices)
	}
	if len(qs[0].Options) != 4 {
		t.Errorf("expected 4 options, got %d", len(qs[0].Options))
	}
	if qs[0].Options[1].Text != "Paris" {
		t.Errorf("expected option text %q, got %q", "Paris", qs[0].Options[1].Text)
	}
}

func TestExtractQuestions_AsteriskPrefixedOptionTabbed(t *testing.T) {
	content := "Chemical symbol Au?\n\tSilver\n\t*Gold\n\tCopper"
	qs := ExtractQuestions(content, "tabbed")
	if len(qs) != 1 {
		t.Fatalf("expected 1 question, got %d", len(qs))
	}
	if len(qs[0].CorrectIndices) != 1 || qs[0].CorrectIndices[0] != 1 {
		t.Errorf("expected correct index [1], got %v", qs[0].CorrectIndices)
	}
}

// A question with no marked answer is reported so the user can be told.
func TestMissingAnswerReason(t *testing.T) {
	marked := models.Question{
		Options:        []models.Option{{Label: "A", Text: "one"}},
		CorrectIndex:   0,
		CorrectIndices: []int{0},
	}
	if r := MissingAnswerReason(marked); r != "" {
		t.Errorf("expected no reason for a marked answer, got %q", r)
	}

	unmarked := models.Question{
		Options:        []models.Option{{Label: "A", Text: "one"}},
		CorrectIndex:   -1,
		CorrectIndices: []int{},
	}
	if r := MissingAnswerReason(unmarked); r == "" {
		t.Error("expected a reason for an unmarked answer, got empty")
	}

	noOptions := models.Question{CorrectIndex: -1}
	if r := MissingAnswerReason(noOptions); r != "" {
		t.Errorf("expected no reason when there are no options, got %q", r)
	}
}
