package api

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/rahul/indifferent/backend/internal/models"
)

func TestPipelineProgress_FailureCode(t *testing.T) {
	raw := `{"errorMessage":"format detection failed: no detectable question format found","errorType":"wrapError"}`
	cases := []struct {
		name    string
		project models.Project
		want    string
	}{
		{"failed with a file problem", models.Project{Status: "failed", Error: raw}, "NO_QUESTION_FORMAT"},
		{"failed, no valid questions", models.Project{Status: "failed", Error: `{"errorMessage":"question validation failed: no valid questions found in file"}`}, "NO_VALID_QUESTIONS"},
		{"failed for another reason", models.Project{Status: "failed", Error: `{"errorMessage":"failed to read TXT file from S3: boom"}`}, ""},
		{"failed with no stored error", models.Project{Status: "failed"}, ""},
		{"not failed: the code is never set", models.Project{Status: "rendering", Error: raw}, ""},
		{"completed: the code is never set", models.Project{Status: "completed", Error: raw}, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := c.project
			if got := buildPipelineProgress(&p).FailureCode; got != c.want {
				t.Errorf("FailureCode = %q, want %q", got, c.want)
			}
		})
	}
}

func TestPipelineProgress_NeverLeaksTheStoredError(t *testing.T) {
	p := &models.Project{Status: "failed", Error: `{"errorMessage":"format detection failed: no detectable question format found","errorType":"wrapError"}`}
	b, err := json.Marshal(buildPipelineProgress(p))
	if err != nil {
		t.Fatal(err)
	}
	body := string(b)
	for _, leak := range []string{"errorMessage", "errorType", "wrapError", "detection failed", "no detectable"} {
		if strings.Contains(body, leak) {
			t.Errorf("response %s leaks %q", body, leak)
		}
	}
	if !strings.Contains(body, `"failureCode":"NO_QUESTION_FORMAT"`) {
		t.Errorf("response %s has no failure code", body)
	}
}

func TestPipelineProgress_OmitsTheCodeWhenThereIsNone(t *testing.T) {
	b, _ := json.Marshal(buildPipelineProgress(&models.Project{Status: "rendering"}))
	if strings.Contains(string(b), "failureCode") {
		t.Errorf("%s should not mention failureCode", b)
	}
}
