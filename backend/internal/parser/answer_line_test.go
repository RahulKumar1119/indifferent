package parser

import "testing"

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
		{"packed", "Answer: BD", []int{1, 3}},
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

func TestExtractQuestions_AnswerLineFreeTextIgnored(t *testing.T) {
	// A free-text answer must not be read as an option label.
	content := "1. Q?\nA. one\nB. two\nC. three\nAnswer: London"
	qs := ExtractQuestions(content, "numbered")
	if len(qs) != 1 {
		t.Fatalf("expected 1 question, got %d", len(qs))
	}
	if len(qs[0].CorrectIndices) != 0 {
		t.Errorf("expected no correct answer, got %v", qs[0].CorrectIndices)
	}
}

func TestExtractQuestions_Tabbed_AnswerLine(t *testing.T) {
	content := "What is the capital of France?\n\tLondon\n\tParis\n\tBerlin\n\tMadrid\nAnswer: B"
	qs := ExtractQuestions(content, "tabbed")
	if len(qs) != 1 {
		t.Fatalf("expected 1 question, got %d", len(qs))
	}
	if len(qs[0].CorrectIndices) != 1 || qs[0].CorrectIndices[0] != 1 {
		t.Errorf("expected correct index [1], got %v", qs[0].CorrectIndices)
	}
}

func TestExtractQuestions_Bulleted_AnswerLine(t *testing.T) {
	content := "What is the capital of France?\n- London\n- Paris\n- Berlin\n- Madrid\nAnswer: B"
	qs := ExtractQuestions(content, "bulleted")
	if len(qs) != 1 {
		t.Fatalf("expected 1 question, got %d", len(qs))
	}
	if len(qs[0].CorrectIndices) != 1 || qs[0].CorrectIndices[0] != 1 {
		t.Errorf("expected correct index [1], got %v", qs[0].CorrectIndices)
	}
}
