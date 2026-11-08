package notifier

import (
	"context"
	"strings"
	"testing"

	dbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

func sendFailedQuiz(t *testing.T, stored string) (subject, text, html string) {
	t.Helper()
	f := newFixture()
	f.db.put(testProjects, "USER#u1", "PROJECT#p1", map[string]dbtypes.AttributeValue{
		"status": str("failed"),
		"name":   str("My quiz"),
		"error":  str(stored),
	})
	in := quizDone("exec-1")
	in.Outcome = OutcomeFailed
	if _, err := f.n.Handle(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	if len(f.ml.sent) != 1 {
		t.Fatalf("want 1 email, got %d", len(f.ml.sent))
	}
	m := f.ml.sent[0]
	return m.Subject, m.Text, m.HTML
}

func TestFailedQuizEmailSaysWhenTheFileHadNoQuestions(t *testing.T) {
	_, text, html := sendFailedQuiz(t, `{"errorMessage":"format detection failed: no detectable question format found","errorType":"wrapError"}`)
	for _, body := range []string{text, html} {
		if !strings.Contains(body, "We could not find any multiple-choice questions in your file.") {
			t.Errorf("email does not explain the file problem: %q", body)
		}
		if !strings.Contains(body, "choose another file") {
			t.Errorf("email does not say what to do: %q", body)
		}
		// The raw error stays private, only the fixed sentence goes out.
		for _, leak := range []string{"errorMessage", "wrapError", "detection failed", "no detectable"} {
			if strings.Contains(body, leak) {
				t.Errorf("raw error text %q leaked into the email", leak)
			}
		}
	}
}

func TestFailedQuizEmailIsGenericForOtherFailures(t *testing.T) {
	_, text, _ := sendFailedQuiz(t, `{"errorMessage":"failed to read TXT file from S3: boom"}`)
	if !strings.Contains(text, "Something went wrong while making") {
		t.Errorf("an unrecognised failure should keep the generic wording: %q", text)
	}
	if strings.Contains(text, "multiple-choice") {
		t.Errorf("an unrecognised failure must not blame the file: %q", text)
	}
}

func TestReasonIsOnlyUsedForAFailedQuizVideo(t *testing.T) {
	reason := "We could not find any multiple-choice questions in your file."
	done, err := RenderWithReason(KindQuiz, OutcomeCompleted, "Q", "p1", testBase, reason)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(done.Text, reason) {
		t.Error("a completed video must not carry a failure reason")
	}
	shorts, err := RenderWithReason(KindShorts, OutcomeFailed, "", "j1", testBase, reason)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(shorts.Text, reason) {
		t.Error("a Shorts failure must not carry a quiz file reason")
	}
	same, _ := Render(KindQuiz, OutcomeFailed, "Q", "p1", testBase)
	none, _ := RenderWithReason(KindQuiz, OutcomeFailed, "Q", "p1", testBase, "")
	if same.Text != none.Text || same.Subject != none.Subject {
		t.Error("Render and RenderWithReason with no reason must be identical")
	}
}
