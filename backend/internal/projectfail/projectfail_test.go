package projectfail

import "testing"

func TestClassify(t *testing.T) {
	cases := []struct {
		name   string
		stored string
		want   Code
	}{
		{"detector error as Step Functions stores it", `{"errorMessage":"format detection failed: no detectable question format found","errorType":"wrapError"}`, NoQuestionFormat},
		{"validator error", `{"errorMessage":"question validation failed: no valid questions found in file","errorType":"wrapError"}`, NoValidQuestions},
		{"validator error with detail", `{"errorMessage":"question validation failed: no valid questions found in file: all entries were malformed"}`, NoValidQuestions},
		{"any case", `NO DETECTABLE QUESTION FORMAT`, NoQuestionFormat},
		{"S3 read error is not the person's file", `{"errorMessage":"failed to read TXT file from S3: operation error S3: GetObject"}`, ""},
		{"compose error", `{"errorMessage":"failed to compose video: no slide files provided"}`, ""},
		{"empty", ``, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Classify(c.stored); got != c.want {
				t.Errorf("Classify = %q, want %q", got, c.want)
			}
		})
	}
}

func TestFileProblemAndExplanation(t *testing.T) {
	for _, c := range []Code{NoQuestionFormat, NoValidQuestions} {
		if !c.FileProblem() || c.Explanation() == "" {
			t.Errorf("%q: FileProblem=%v explanation=%q", c, c.FileProblem(), c.Explanation())
		}
	}
	if Code("").FileProblem() || Code("").Explanation() != "" {
		t.Error("an unknown failure must not claim to be a file problem")
	}
}
