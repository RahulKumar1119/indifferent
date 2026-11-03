package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	dbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/rahul/indifferent/backend/internal/storyjob"
	"github.com/rahul/indifferent/backend/internal/storymetrics"
)

type sink struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *sink) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

// counts returns the total of each metric written, and the Code of each failure.
func (s *sink) counts(t *testing.T) (map[string]float64, []string) {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	totals := map[string]float64{}
	var codes []string
	for _, l := range strings.Split(strings.TrimSpace(s.buf.String()), "\n") {
		if l == "" {
			continue
		}
		var root map[string]any
		if err := json.Unmarshal([]byte(l), &root); err != nil {
			t.Fatalf("not JSON: %q", l)
		}
		for _, d := range root["_aws"].(map[string]any)["CloudWatchMetrics"].([]any)[0].(map[string]any)["Metrics"].([]any) {
			name := d.(map[string]any)["Name"].(string)
			totals[name] += root[name].(float64)
		}
		if c, ok := root[storymetrics.CodeDimension].(string); ok {
			codes = append(codes, c)
		}
	}
	return totals, codes
}

func (s *sink) equal(t *testing.T, want map[string]float64) {
	t.Helper()
	got, _ := s.counts(t)
	if fmt.Sprint(got) != fmt.Sprint(want) { // fmt prints maps in key order
		t.Errorf("metrics = %v, want %v", got, want)
	}
}

func withSink(f *fixture) *sink {
	s := &sink{}
	f.n.Metrics = storymetrics.New(s)
	return s
}

func put(f *fixture, sk string, attrs map[string]string) {
	m := map[string]dbtypes.AttributeValue{}
	for k, v := range attrs {
		m[k] = str(v)
	}
	f.db.put(testProjects, "USER#u1", sk, m)
}

func TestStoryMetrics_ACompletedVideoIsCountedWithItsGenerationTime(t *testing.T) {
	f := storyFixture()
	s := withSink(f)
	put(f, "STORYVIDEO#jt", map[string]string{
		"status": "completed", "planId": "p", "createdAt": "2026-10-31T12:00:00Z", "completedAt": "2026-10-31T12:03:07Z",
	})
	if _, err := f.n.Handle(context.Background(), storyIn("jt", OutcomeCompleted)); err != nil {
		t.Fatal(err)
	}
	s.equal(t, map[string]float64{storymetrics.VideosCompleted: 1, storymetrics.VideoSeconds: 187})
}

func TestStoryMetrics_ACompletedVideoWithNoUsableTimesIsStillCounted(t *testing.T) {
	for name, attrs := range map[string]map[string]string{
		"no times":            {"status": "completed", "planId": "p"},
		"completed too early": {"status": "completed", "planId": "p", "createdAt": "2026-10-31T12:00:00Z", "completedAt": "2026-10-31T11:00:00Z"},
		"not dates":           {"status": "completed", "planId": "p", "createdAt": "x", "completedAt": "y"},
		"over a day":          {"status": "completed", "planId": "p", "createdAt": "2026-10-01T12:00:00Z", "completedAt": "2026-10-31T12:00:00Z"},
	} {
		t.Run(name, func(t *testing.T) {
			f := storyFixture()
			s := withSink(f)
			put(f, "STORYVIDEO#jt", attrs)
			f.n.Handle(context.Background(), storyIn("jt", OutcomeCompleted))
			s.equal(t, map[string]float64{storymetrics.VideosCompleted: 1})
		})
	}
}

func TestStoryMetrics_AFailedVideoIsCountedWithItsCode(t *testing.T) {
	for _, code := range []string{"MEDIA_UNAVAILABLE", "MEDIA_TEMPORARY", "NARRATION_FAILED", "INTERNAL", "TIMED_OUT"} {
		t.Run(code, func(t *testing.T) {
			f := storyFixture()
			s := withSink(f)
			put(f, "STORYVIDEO#jx", map[string]string{"status": "failed", "planId": "p", "errorCode": code})
			f.n.Handle(context.Background(), storyIn("jx", OutcomeFailed))
			got, codes := s.counts(t)
			if got[storymetrics.VideosFailed] != 1 || len(got) != 1 || len(codes) != 1 || codes[0] != code {
				t.Errorf("metrics = %v, codes = %v", got, codes)
			}
		})
	}
}

func TestStoryMetrics_AFailureTheWorkerNeverSaidAnythingAboutIsCountedToo(t *testing.T) {
	// The state machine's MarkFailed gives a job that ended without a reason of its
	// own the code INTERNAL; a record with no code at all is still a failure.
	f := storyFixture()
	s := withSink(f)
	put(f, "STORYVIDEO#jx", map[string]string{"status": "failed", "planId": "p"})
	f.n.Handle(context.Background(), storyIn("jx", OutcomeFailed))
	got, codes := s.counts(t)
	if got[storymetrics.VideosFailed] != 1 || len(codes) != 1 || codes[0] != storymetrics.OtherCode {
		t.Errorf("metrics = %v, codes = %v", got, codes)
	}
}

// The API counts these when they happen, because the notifier may never run for
// them. If it does run, it must not count them again.
func TestStoryMetrics_ACancelOrAFailedStartIsLeftToTheAPI(t *testing.T) {
	for _, code := range []string{"CANCELLED", "START_FAILED"} {
		t.Run(code, func(t *testing.T) {
			f := storyFixture()
			s := withSink(f)
			put(f, "STORYVIDEO#jx", map[string]string{"status": "failed", "planId": "p", "errorCode": code})
			f.n.Handle(context.Background(), storyIn("jx", OutcomeFailed))
			s.equal(t, map[string]float64{})
		})
	}
}

// Step Functions retries the notifier when the email is throttled, and that
// retry reads the same record again.
func TestStoryMetrics_ARetryOfTheSameExecutionIsNotCountedTwice(t *testing.T) {
	f := storyFixture()
	s := withSink(f)
	f.ml.errs = []error{fmt.Errorf("%w: slow down", ErrThrottled)} // the first send is throttled
	in := storyIn("j9", OutcomeCompleted)
	if _, err := f.n.Handle(context.Background(), in); err == nil {
		t.Fatal("setup: the first run should have been throttled")
	}
	if _, err := f.n.Handle(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	s.equal(t, map[string]float64{storymetrics.VideosCompleted: 1})
	if len(f.ml.sent) != 1 {
		t.Errorf("setup: the email should have been sent once, was %d", len(f.ml.sent))
	}
}

func TestStoryMetrics_ADifferentExecutionOfTheSameJobIsCountedAgain(t *testing.T) {
	// A job is reset and run again only by making a new video, which is a new job;
	// but two executions are two events and the claim is per execution.
	f := storyFixture()
	s := withSink(f)
	a, b := storyIn("j9", OutcomeCompleted), storyIn("j9", OutcomeCompleted)
	b.ExecutionID = "exec-2"
	f.n.Handle(context.Background(), a)
	f.n.Handle(context.Background(), b)
	s.equal(t, map[string]float64{storymetrics.VideosCompleted: 2})
}

func TestStoryMetrics_AreCountedWhateverHappensToTheEmail(t *testing.T) {
	cases := map[string]func(f *fixture){
		"the user has email off": func(f *fixture) {
			f.db.put(testUsers, "USER#u1", "PROFILE", map[string]dbtypes.AttributeValue{"email": str("p@example.com"), "name": str("P"), "notifyCompleted": flag(false)})
		},
		"the user has no profile": func(f *fixture) { delete(f.db.items, dbKey(testUsers, "USER#u1", "PROFILE")) },
		"the user has no email": func(f *fixture) {
			f.db.put(testUsers, "USER#u1", "PROFILE", map[string]dbtypes.AttributeValue{"name": str("P")})
		},
		"the job has no plan": func(f *fixture) { put(f, "STORYVIDEO#j9", map[string]string{"status": "completed"}) },
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			f := storyFixture()
			s := withSink(f)
			setup(f)
			f.n.Handle(context.Background(), storyIn("j9", OutcomeCompleted))
			got, _ := s.counts(t)
			if got[storymetrics.VideosCompleted] != 1 {
				t.Errorf("metrics = %v: a video that finished was not counted because of the email", got)
			}
		})
	}
}

func TestStoryMetrics_NothingIsCountedForARecordThatIsNotTheOutcomeAnnounced(t *testing.T) {
	t.Run("the record has moved on", func(t *testing.T) {
		f := storyFixture()
		s := withSink(f)
		put(f, "STORYVIDEO#jr", map[string]string{"status": "rendering", "planId": "p"})
		f.n.Handle(context.Background(), storyIn("jr", OutcomeCompleted))
		s.equal(t, map[string]float64{})
	})
	t.Run("the job is gone", func(t *testing.T) {
		f := storyFixture()
		s := withSink(f)
		f.n.Handle(context.Background(), storyIn("nope", OutcomeCompleted))
		s.equal(t, map[string]float64{})
	})
	t.Run("announced failed but the record says completed", func(t *testing.T) {
		f := storyFixture()
		s := withSink(f)
		f.n.Handle(context.Background(), storyIn("j9", OutcomeFailed))
		s.equal(t, map[string]float64{})
	})
}

// A problem with the metrics must never cost anyone their email.
func TestStoryMetrics_AProblemRecordingNeverStopsTheEmail(t *testing.T) {
	t.Run("the claim fails", func(t *testing.T) {
		f := storyFixture()
		s := withSink(f)
		f.db.metricsClaimErr = errors.New("dynamodb throttled")
		r, err := f.n.Handle(context.Background(), storyIn("j9", OutcomeCompleted))
		if err != nil || r.Status != StatusSent || len(f.ml.sent) != 1 {
			t.Errorf("%+v, %v, %d emails", r, err, len(f.ml.sent))
		}
		s.equal(t, map[string]float64{})
	})
	t.Run("the job cannot be read", func(t *testing.T) {
		// Handle reads the profile first, so the profile read must work and the job read fail.
		f := storyFixture()
		s := withSink(f)
		f.db.getErr = errors.New("down")
		_, err := f.n.Handle(context.Background(), storyIn("j9", OutcomeCompleted))
		if err == nil {
			t.Fatal("setup: the email path reads the profile and should fail with the database down")
		}
		s.equal(t, map[string]float64{})
	})
	t.Run("the emitter writes nowhere", func(t *testing.T) {
		f := storyFixture()
		f.n.Metrics = storymetrics.New(nil)
		if r, err := f.n.Handle(context.Background(), storyIn("j9", OutcomeCompleted)); err != nil || r.Status != StatusSent {
			t.Errorf("%+v, %v", r, err)
		}
	})
}

func TestStoryMetrics_ANotifierWithNoEmitterBehavesAsBefore(t *testing.T) {
	f := storyFixture()
	if f.n.Metrics != nil {
		t.Fatal("setup")
	}
	before := len(f.db.updates)
	r, err := f.n.Handle(context.Background(), storyIn("j9", OutcomeCompleted))
	if err != nil || r.Status != StatusSent {
		t.Fatalf("%+v, %v", r, err)
	}
	// The claim and the message id, and no metrics claim.
	if n := len(f.db.updates) - before; n != 2 {
		t.Errorf("%d updates, want 2: the metrics claim must not be made without an emitter", n)
	}
}

func TestStoryMetrics_QuizAndShortsAreNeverCountedOrTouched(t *testing.T) {
	f := newFixture()
	s := withSink(f)
	if _, err := f.n.Handle(context.Background(), quizDone("exec-1")); err != nil {
		t.Fatal(err)
	}
	if _, err := f.n.Handle(context.Background(), Input{Kind: KindShorts, Outcome: OutcomeCompleted, UserID: "u1", JobID: "j1", ExecutionID: "exec-2"}); err != nil {
		t.Fatal(err)
	}
	s.equal(t, map[string]float64{})
	for _, u := range f.db.updates {
		if strings.Contains(*u.UpdateExpression, "metricsExecution") {
			t.Errorf("a quiz or shorts job was given a metrics claim: %s", *u.UpdateExpression)
		}
	}
}

func TestStoryMetrics_TheClaimIsExactlyThisExpression(t *testing.T) {
	f := storyFixture()
	withSink(f)
	f.n.Handle(context.Background(), storyIn("j9", OutcomeCompleted))
	var claim = f.db.updates[0]
	if got, want := *claim.UpdateExpression, "SET metricsExecution = :exec"; got != want {
		t.Errorf("update:\n got %q\nwant %q", got, want)
	}
	if got, want := *claim.ConditionExpression,
		"attribute_exists(PK) AND (attribute_not_exists(metricsExecution) OR metricsExecution <> :exec)"; got != want {
		t.Errorf("condition:\n got %q\nwant %q", got, want)
	}
	if *claim.TableName != testProjects {
		t.Errorf("table = %s", *claim.TableName)
	}
	// It must not collide with the email's claim, which a throttled run releases.
	if strings.Contains(*claim.UpdateExpression, "notified") {
		t.Error("the metrics claim shares an attribute with the email claim")
	}
}

// The notifier spells out two failure codes rather than import storyjob.
func TestStoryMetrics_TheCodesLeftToTheAPIAreStoryJobsOwn(t *testing.T) {
	if codeCancelled != storyjob.CodeCancelled || codeStartFailed != storyjob.CodeStartFailed {
		t.Errorf("notifier has %q and %q, storyjob has %q and %q", codeCancelled, codeStartFailed, storyjob.CodeCancelled, storyjob.CodeStartFailed)
	}
}

// The notifier reads these names from the job record; storyjob writes them.
func TestStoryMetrics_TheRecordFieldsAreTheOnesStoryJobWrites(t *testing.T) {
	at := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)
	j := storyjob.Job{UserID: "u1", JobID: "0194f8a2-7c3d-7e11-8b2a-5d6e7f8a9b0c", Status: storyjob.StatusCompleted, ErrorCode: "X",
		CreatedAt: at.Format(time.RFC3339), CompletedAt: at.Add(time.Minute).Format(time.RFC3339)}
	item, err := storyjob.ToItem(j)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"status", "errorCode", "createdAt", "completedAt"} {
		if _, ok := item[name].(*dbtypes.AttributeValueMemberS); !ok {
			t.Errorf("storyjob does not write %q as a string: the notifier reads it by that name", name)
		}
	}
	if got := stringAttr(item, "createdAt"); got != j.CreatedAt {
		t.Errorf("createdAt = %q", got)
	}
}
