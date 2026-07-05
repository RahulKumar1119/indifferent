package parser

import (
	"testing"

	"github.com/rahul/indifferent/backend/internal/models"
)

func TestExtractQuestions_Numbered_Basic(t *testing.T) {
	content := `1. What is the capital of France?
A) London
B) Paris *
C) Berlin
D) Madrid

2. What is 2+2?
A) 3
B) 4 (correct)
C) 5
D) 6`

	questions := ExtractQuestions(content, "numbered")

	if len(questions) != 2 {
		t.Fatalf("expected 2 questions, got %d", len(questions))
	}

	// Check first question
	q1 := questions[0]
	if q1.Index != 0 {
		t.Errorf("q1 index: expected 0, got %d", q1.Index)
	}
	if q1.Text != "What is the capital of France?" {
		t.Errorf("q1 text: got %q", q1.Text)
	}
	if len(q1.Options) != 4 {
		t.Fatalf("q1 options: expected 4, got %d", len(q1.Options))
	}
	if q1.Options[0].Label != "A" || q1.Options[0].Text != "London" {
		t.Errorf("q1 option A: got %+v", q1.Options[0])
	}
	if q1.Options[1].Label != "B" || q1.Options[1].Text != "Paris" {
		t.Errorf("q1 option B: got %+v", q1.Options[1])
	}
	if q1.CorrectIndex != 1 {
		t.Errorf("q1 correctIndex: expected 1, got %d", q1.CorrectIndex)
	}

	// Check second question
	q2 := questions[1]
	if q2.Index != 1 {
		t.Errorf("q2 index: expected 1, got %d", q2.Index)
	}
	if q2.CorrectIndex != 1 {
		t.Errorf("q2 correctIndex: expected 1, got %d", q2.CorrectIndex)
	}
}

func TestExtractQuestions_Numbered_QPrefix(t *testing.T) {
	content := `Q1. What is Go?
A. A programming language (correct)
B. A board game
C. A movie

Q2: Who created Linux?
A: Linus Torvalds *
B: Bill Gates`

	questions := ExtractQuestions(content, "numbered")

	if len(questions) != 2 {
		t.Fatalf("expected 2 questions, got %d", len(questions))
	}

	if questions[0].Text != "What is Go?" {
		t.Errorf("q1 text: got %q", questions[0].Text)
	}
	if len(questions[0].Options) != 3 {
		t.Errorf("q1 options: expected 3, got %d", len(questions[0].Options))
	}
	if questions[0].CorrectIndex != 0 {
		t.Errorf("q1 correctIndex: expected 0, got %d", questions[0].CorrectIndex)
	}

	if questions[1].Text != "Who created Linux?" {
		t.Errorf("q2 text: got %q", questions[1].Text)
	}
	if len(questions[1].Options) != 2 {
		t.Errorf("q2 options: expected 2, got %d", len(questions[1].Options))
	}
	if questions[1].CorrectIndex != 0 {
		t.Errorf("q2 correctIndex: expected 0, got %d", questions[1].CorrectIndex)
	}
}

func TestExtractQuestions_Numbered_MultiLine(t *testing.T) {
	content := `1. This is a long question that
spans multiple lines before options start
A) First option
B) Second option *
C) Third option`

	questions := ExtractQuestions(content, "numbered")

	if len(questions) != 1 {
		t.Fatalf("expected 1 question, got %d", len(questions))
	}

	expected := "This is a long question that spans multiple lines before options start"
	if questions[0].Text != expected {
		t.Errorf("text: got %q, want %q", questions[0].Text, expected)
	}
	if len(questions[0].Options) != 3 {
		t.Errorf("options: expected 3, got %d", len(questions[0].Options))
	}
}

func TestExtractQuestions_Bulleted_Basic(t *testing.T) {
	content := `What is the largest planet?
- Jupiter *
- Saturn
- Earth
- Mars

What color is the sky?
• Blue (correct)
• Green
• Red`

	questions := ExtractQuestions(content, "bulleted")

	if len(questions) != 2 {
		t.Fatalf("expected 2 questions, got %d", len(questions))
	}

	q1 := questions[0]
	if q1.Index != 0 {
		t.Errorf("q1 index: expected 0, got %d", q1.Index)
	}
	if q1.Text != "What is the largest planet?" {
		t.Errorf("q1 text: got %q", q1.Text)
	}
	if len(q1.Options) != 4 {
		t.Fatalf("q1 options: expected 4, got %d", len(q1.Options))
	}
	if q1.Options[0].Label != "A" || q1.Options[0].Text != "Jupiter" {
		t.Errorf("q1 option A: got %+v", q1.Options[0])
	}
	if q1.CorrectIndex != 0 {
		t.Errorf("q1 correctIndex: expected 0, got %d", q1.CorrectIndex)
	}

	q2 := questions[1]
	if len(q2.Options) != 3 {
		t.Fatalf("q2 options: expected 3, got %d", len(q2.Options))
	}
	if q2.CorrectIndex != 0 {
		t.Errorf("q2 correctIndex: expected 0, got %d", q2.CorrectIndex)
	}
}

func TestExtractQuestions_Bulleted_MultiLine(t *testing.T) {
	content := `This is a question that
continues on the next line
- Option one
- Option two *`

	questions := ExtractQuestions(content, "bulleted")

	if len(questions) != 1 {
		t.Fatalf("expected 1 question, got %d", len(questions))
	}

	expected := "This is a question that continues on the next line"
	if questions[0].Text != expected {
		t.Errorf("text: got %q, want %q", questions[0].Text, expected)
	}
}

func TestExtractQuestions_Tabbed_Basic(t *testing.T) {
	content := `What is the speed of light?
	A) 300,000 km/s *
	B) 150,000 km/s
	C) 1,000 km/s

What is water made of?
	A) H2O (correct)
	B) CO2
	C) NaCl
	D) O2`

	questions := ExtractQuestions(content, "tabbed")

	if len(questions) != 2 {
		t.Fatalf("expected 2 questions, got %d", len(questions))
	}

	q1 := questions[0]
	if q1.Text != "What is the speed of light?" {
		t.Errorf("q1 text: got %q", q1.Text)
	}
	if len(q1.Options) != 3 {
		t.Fatalf("q1 options: expected 3, got %d", len(q1.Options))
	}
	if q1.CorrectIndex != 0 {
		t.Errorf("q1 correctIndex: expected 0, got %d", q1.CorrectIndex)
	}

	q2 := questions[1]
	if len(q2.Options) != 4 {
		t.Fatalf("q2 options: expected 4, got %d", len(q2.Options))
	}
	if q2.CorrectIndex != 0 {
		t.Errorf("q2 correctIndex: expected 0, got %d", q2.CorrectIndex)
	}
}

func TestExtractQuestions_Tabbed_PlainOptions(t *testing.T) {
	// Tab-indented options without A), B) labels - plain text
	content := `What is Go?
	Fast compiled language *
	Interpreted scripting lang
	Markup language`

	questions := ExtractQuestions(content, "tabbed")

	if len(questions) != 1 {
		t.Fatalf("expected 1 question, got %d", len(questions))
	}

	q := questions[0]
	if len(q.Options) != 3 {
		t.Fatalf("options: expected 3, got %d", len(q.Options))
	}
	if q.Options[0].Label != "A" {
		t.Errorf("option 0 label: expected A, got %q", q.Options[0].Label)
	}
	if q.Options[0].Text != "Fast compiled language" {
		t.Errorf("option 0 text: got %q", q.Options[0].Text)
	}
	if q.CorrectIndex != 0 {
		t.Errorf("correctIndex: expected 0, got %d", q.CorrectIndex)
	}
}

func TestExtractQuestions_Tabbed_SpaceIndented(t *testing.T) {
	// 4-space indented options
	content := `What is 1+1?
    2 (correct)
    3
    4`

	questions := ExtractQuestions(content, "tabbed")

	if len(questions) != 1 {
		t.Fatalf("expected 1 question, got %d", len(questions))
	}

	if questions[0].CorrectIndex != 0 {
		t.Errorf("correctIndex: expected 0, got %d", questions[0].CorrectIndex)
	}
}

func TestExtractQuestions_VariableOptionCounts(t *testing.T) {
	content := `1. Two options
A) Yes *
B) No

2. Six options
A) Alpha (correct)
B) Beta
C) Gamma
D) Delta
E) Epsilon
F) Zeta`

	questions := ExtractQuestions(content, "numbered")

	if len(questions) != 2 {
		t.Fatalf("expected 2 questions, got %d", len(questions))
	}

	if len(questions[0].Options) != 2 {
		t.Errorf("q1 options: expected 2, got %d", len(questions[0].Options))
	}
	if len(questions[1].Options) != 6 {
		t.Errorf("q2 options: expected 6, got %d", len(questions[1].Options))
	}
}

func TestExtractQuestions_EmptyContent(t *testing.T) {
	questions := ExtractQuestions("", "numbered")
	if len(questions) != 0 {
		t.Errorf("expected 0 questions for empty content, got %d", len(questions))
	}
}

func TestExtractQuestions_UnknownFormat(t *testing.T) {
	questions := ExtractQuestions("some content", "unknown")
	if questions != nil {
		t.Errorf("expected nil for unknown format, got %v", questions)
	}
}

func TestExtractQuestions_CorrectIndexDefaultsToNegativeOne(t *testing.T) {
	content := `1. No correct answer marked
A) Option one
B) Option two`

	questions := ExtractQuestions(content, "numbered")

	if len(questions) != 1 {
		t.Fatalf("expected 1 question, got %d", len(questions))
	}

	if questions[0].CorrectIndex != -1 {
		t.Errorf("correctIndex: expected -1 when no marker, got %d", questions[0].CorrectIndex)
	}
}

func TestExtractQuestions_ZeroBasedIndexing(t *testing.T) {
	content := `1. First question
A) A *
B) B

2. Second question
A) A
B) B *

3. Third question
A) A
B) B
C) C *`

	questions := ExtractQuestions(content, "numbered")

	if len(questions) != 3 {
		t.Fatalf("expected 3 questions, got %d", len(questions))
	}

	for i, q := range questions {
		if q.Index != i {
			t.Errorf("question %d: expected index %d, got %d", i, i, q.Index)
		}
	}
}

func TestOptionLabel(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{0, "A"},
		{1, "B"},
		{2, "C"},
		{3, "D"},
		{4, "E"},
		{5, "F"},
		{25, "Z"},
	}

	for _, tt := range tests {
		got := optionLabel(tt.n)
		if got != tt.want {
			t.Errorf("optionLabel(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}

func TestCheckCorrectMarkers(t *testing.T) {
	tests := []struct {
		input       string
		wantText    string
		wantCorrect bool
	}{
		{"Paris *", "Paris", true},
		{"* Paris", "Paris", true},
		{"Paris (correct)", "Paris", true},
		{"Paris", "Paris", false},
		{"*Answer*", "Answer", true}, // prefix and suffix
	}

	for _, tt := range tests {
		text, correct := checkCorrectMarkers(tt.input)
		if text != tt.wantText || correct != tt.wantCorrect {
			t.Errorf("checkCorrectMarkers(%q) = (%q, %v), want (%q, %v)",
				tt.input, text, correct, tt.wantText, tt.wantCorrect)
		}
	}
}

func TestExtractQuestions_ReturnsProperModelTypes(t *testing.T) {
	content := `1. Test question
A) Answer one *
B) Answer two`

	questions := ExtractQuestions(content, "numbered")

	if len(questions) != 1 {
		t.Fatalf("expected 1 question, got %d", len(questions))
	}

	// Verify the types match models.Question and models.Option
	var _ models.Question = questions[0]
	var _ []models.Option = questions[0].Options
}
