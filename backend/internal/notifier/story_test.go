package notifier

import (
	"context"
	"strings"
	"testing"

	dbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// storyFixture adds a completed story video (job j9, made from plan p9) to the
// standard fixture, and a failed one.
func storyFixture() *fixture {
	f := newFixture()
	f.db.put(testProjects, "USER#u1", "STORYVIDEO#j9", map[string]dbtypes.AttributeValue{
		"status": str("completed"),
		"planId": str("plan-9"),
	})
	f.db.put(testProjects, "USER#u1", "STORYVIDEO#jf", map[string]dbtypes.AttributeValue{
		"status":    str("failed"),
		"planId":    str("plan-f"),
		"error":     str("Scene 2: the picture is no longer available from Pixabay. Choose another and make the video again."),
		"errorCode": str("MEDIA_UNAVAILABLE"),
	})
	return f
}

func storyIn(job, outcome string) Input {
	return Input{Kind: KindStory, Outcome: outcome, UserID: "u1", JobID: job, ExecutionID: "exec-1"}
}

func TestStory_CompletedEmailLinksToThePlanNotTheJob(t *testing.T) {
	f := storyFixture()
	r, err := f.n.Handle(context.Background(), storyIn("j9", OutcomeCompleted))
	if err != nil || r.Status != StatusSent {
		t.Fatalf("%+v, %v", r, err)
	}
	m := f.ml.sent[0]
	if !strings.Contains(m.Text, testBase+"/story/plan-9") {
		t.Errorf("the link is not the plan's page:\n%s", m.Text)
	}
	for _, part := range []string{m.Text, m.HTML} {
		if strings.Contains(part, "/story/j9") || strings.Contains(part, "j9") {
			t.Errorf("the job id is in the email: %q", part)
		}
	}
	if m.Subject != "Your story video is ready" {
		t.Errorf("subject = %q", m.Subject)
	}
}

func TestStory_FailedEmailIsGenericAndNeverRepeatsTheStoredReason(t *testing.T) {
	f := storyFixture()
	if _, err := f.n.Handle(context.Background(), storyIn("jf", OutcomeFailed)); err != nil {
		t.Fatal(err)
	}
	m := f.ml.sent[0]
	for _, part := range []string{m.Subject, m.Text, m.HTML} {
		if strings.Contains(part, "Pixabay") || strings.Contains(part, "Scene 2") || strings.Contains(part, "MEDIA_UNAVAILABLE") {
			t.Errorf("the stored failure reached the email: %q", part)
		}
	}
	if !strings.Contains(m.Text, testBase+"/story/plan-f") || !strings.Contains(m.Subject, "could not be finished") {
		t.Errorf("subject %q text:\n%s", m.Subject, m.Text)
	}
}

func TestStory_ACancelledJobSendsNothing(t *testing.T) {
	f := storyFixture()
	f.db.put(testProjects, "USER#u1", "STORYVIDEO#jc", map[string]dbtypes.AttributeValue{
		"status": str("failed"), "errorCode": str("CANCELLED"), "planId": str("plan-c"),
	})
	r, err := f.n.Handle(context.Background(), storyIn("jc", OutcomeFailed))
	if err != nil || r.Status != StatusSkipped || r.Reason != "cancelled_by_user" || len(f.ml.sent) != 0 {
		t.Errorf("%+v, %v, %d emails", r, err, len(f.ml.sent))
	}
}

func TestStory_AJobWithNoPlanIsSkippedNotSentABrokenLink(t *testing.T) {
	f := storyFixture()
	f.db.put(testProjects, "USER#u1", "STORYVIDEO#jn", map[string]dbtypes.AttributeValue{"status": str("completed")})
	r, err := f.n.Handle(context.Background(), storyIn("jn", OutcomeCompleted))
	if err != nil || r.Status != StatusSkipped || r.Reason != "no_plan" || len(f.ml.sent) != 0 {
		t.Errorf("%+v, %v, %d emails", r, err, len(f.ml.sent))
	}
}

func TestStory_FollowsTheSameRulesAsTheOtherPipelines(t *testing.T) {
	t.Run("once per execution", func(t *testing.T) {
		f := storyFixture()
		for i := 0; i < 3; i++ {
			if _, err := f.n.Handle(context.Background(), storyIn("j9", OutcomeCompleted)); err != nil {
				t.Fatal(err)
			}
		}
		if len(f.ml.sent) != 1 {
			t.Errorf("%d emails for one execution", len(f.ml.sent))
		}
		if sv(f.db.get(testProjects, "USER#u1", "STORYVIDEO#j9"), "notifiedExecution") != "exec-1" {
			t.Error("the claim was not recorded on the story job")
		}
	})
	t.Run("a job that is not in the reported state sends nothing", func(t *testing.T) {
		f := storyFixture()
		r, _ := f.n.Handle(context.Background(), storyIn("j9", OutcomeFailed)) // it is completed
		if r.Reason != "status_mismatch" || len(f.ml.sent) != 0 {
			t.Errorf("%+v, %d emails", r, len(f.ml.sent))
		}
	})
	t.Run("a missing job sends nothing", func(t *testing.T) {
		f := storyFixture()
		r, _ := f.n.Handle(context.Background(), storyIn("nope", OutcomeCompleted))
		if r.Reason != "job_missing" || len(f.ml.sent) != 0 {
			t.Errorf("%+v", r)
		}
	})
	t.Run("preferences are respected", func(t *testing.T) {
		f := storyFixture()
		f.db.put(testUsers, "USER#u1", "PROFILE", map[string]dbtypes.AttributeValue{
			"email": str("person@example.com"), "notifyCompleted": flag(false),
		})
		r, _ := f.n.Handle(context.Background(), storyIn("j9", OutcomeCompleted))
		if r.Reason != "preference_off" || len(f.ml.sent) != 0 {
			t.Errorf("%+v", r)
		}
	})
	t.Run("a story job needs a job id", func(t *testing.T) {
		f := storyFixture()
		if _, err := f.n.Handle(context.Background(), Input{Kind: KindStory, Outcome: OutcomeCompleted, UserID: "u1", ExecutionID: "e"}); err == nil {
			t.Error("no error for a missing jobId")
		}
	})
	t.Run("a story job is looked up under its own sort key, not a shorts job's", func(t *testing.T) {
		f := storyFixture()
		// A shorts job with the same id must not be mistaken for the story job.
		f.db.put(testProjects, "USER#u1", "SHORTS#j9", map[string]dbtypes.AttributeValue{"status": str("completed")})
		f.db.items[dbKey(testProjects, "USER#u1", "STORYVIDEO#j9")] = nil
		delete(f.db.items, dbKey(testProjects, "USER#u1", "STORYVIDEO#j9"))
		r, _ := f.n.Handle(context.Background(), storyIn("j9", OutcomeCompleted))
		if r.Reason != "job_missing" {
			t.Errorf("%+v: the shorts job was used", r)
		}
	})
}

func TestStory_QuizAndShortsEmailsAreUnchanged(t *testing.T) {
	f := newFixture()
	if _, err := f.n.Handle(context.Background(), quizDone("e1")); err != nil {
		t.Fatal(err)
	}
	if _, err := f.n.Handle(context.Background(), Input{Kind: KindShorts, Outcome: OutcomeCompleted, UserID: "u1", JobID: "j1", ExecutionID: "e2"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(f.ml.sent[0].Text, "/projects/p1/preview") || !strings.Contains(f.ml.sent[1].Text, "/shorts/j1/clips") {
		t.Errorf("links changed:\n%s\n%s", f.ml.sent[0].Text, f.ml.sent[1].Text)
	}
}
