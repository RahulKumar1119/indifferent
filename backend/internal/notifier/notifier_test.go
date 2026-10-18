package notifier

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

const (
	testProjects = "projects-table"
	testUsers    = "users-table"
	testBase     = "https://indifferent.fun"
)

// ---------------------------------------------------------------- fakes

// fakeDB is an in-memory DynamoDB that understands exactly the three update
// shapes the notifier issues (claim, release, record message id). The shapes
// themselves are pinned by TestUpdateExpressions.
type fakeDB struct {
	items   map[string]map[string]dbtypes.AttributeValue
	updates []*dynamodb.UpdateItemInput
	getErr  error
}

func newFakeDB() *fakeDB { return &fakeDB{items: map[string]map[string]dbtypes.AttributeValue{}} }

func dbKey(table, pk, sk string) string { return table + "|" + pk + "|" + sk }

func (f *fakeDB) put(table, pk, sk string, attrs map[string]dbtypes.AttributeValue) {
	item := map[string]dbtypes.AttributeValue{
		"PK": &dbtypes.AttributeValueMemberS{Value: pk},
		"SK": &dbtypes.AttributeValueMemberS{Value: sk},
	}
	for k, v := range attrs {
		item[k] = v
	}
	f.items[dbKey(table, pk, sk)] = item
}

func (f *fakeDB) get(table, pk, sk string) map[string]dbtypes.AttributeValue {
	return f.items[dbKey(table, pk, sk)]
}

func str(v string) dbtypes.AttributeValue { return &dbtypes.AttributeValueMemberS{Value: v} }
func flag(v bool) dbtypes.AttributeValue  { return &dbtypes.AttributeValueMemberBOOL{Value: v} }
func sv(item map[string]dbtypes.AttributeValue, k string) string {
	if v, ok := item[k].(*dbtypes.AttributeValueMemberS); ok {
		return v.Value
	}
	return ""
}

func keyOf(m map[string]dbtypes.AttributeValue) (string, string) {
	return m["PK"].(*dbtypes.AttributeValueMemberS).Value, m["SK"].(*dbtypes.AttributeValueMemberS).Value
}

func (f *fakeDB) GetItem(_ context.Context, in *dynamodb.GetItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	pk, sk := keyOf(in.Key)
	return &dynamodb.GetItemOutput{Item: f.get(*in.TableName, pk, sk)}, nil
}

func (f *fakeDB) UpdateItem(_ context.Context, in *dynamodb.UpdateItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
	f.updates = append(f.updates, in)
	pk, sk := keyOf(in.Key)
	item := f.get(*in.TableName, pk, sk)
	fail := &dbtypes.ConditionalCheckFailedException{}
	if item == nil {
		return nil, fail
	}
	exec := ""
	if v, ok := in.ExpressionAttributeValues[":exec"].(*dbtypes.AttributeValueMemberS); ok {
		exec = v.Value
	}
	cur := sv(item, "notifiedExecution")
	expr := *in.UpdateExpression
	switch {
	case strings.HasPrefix(expr, "SET notifiedExecution"):
		if cur != "" && cur == exec {
			return nil, fail
		}
		item["notifiedExecution"] = str(exec)
		item["notifiedAt"] = in.ExpressionAttributeValues[":at"]
	case strings.HasPrefix(expr, "REMOVE"):
		if cur != exec {
			return nil, fail
		}
		delete(item, "notifiedExecution")
		delete(item, "notifiedAt")
	case strings.HasPrefix(expr, "SET notifyMessageId"):
		if cur != exec {
			return nil, fail
		}
		item["notifyMessageId"] = in.ExpressionAttributeValues[":mid"]
	default:
		return nil, fmt.Errorf("fakeDB: unexpected update %q", expr)
	}
	return &dynamodb.UpdateItemOutput{}, nil
}

type fakeMailer struct {
	sent []Message
	errs []error // consumed one per call; nil entry or empty queue = success
}

func (m *fakeMailer) Send(_ context.Context, msg Message) (string, error) {
	if len(m.errs) > 0 {
		err := m.errs[0]
		m.errs = m.errs[1:]
		if err != nil {
			return "", err
		}
	}
	m.sent = append(m.sent, msg)
	return fmt.Sprintf("msg-%d", len(m.sent)), nil
}

// ---------------------------------------------------------------- helpers

type fixture struct {
	db *fakeDB
	ml *fakeMailer
	n  *Notifier
}

// newFixture seeds a Google-style (verified) user and one completed quiz
// project and one completed shorts job.
func newFixture() *fixture {
	db := newFakeDB()
	db.put(testUsers, "USER#u1", "PROFILE", map[string]dbtypes.AttributeValue{
		"email": str("person@example.com"),
		"name":  str("Person"),
	})
	db.put(testProjects, "USER#u1", "PROJECT#p1", map[string]dbtypes.AttributeValue{
		"status": str("completed"),
		"name":   str("Week 3 Quiz"),
		"error":  str("AccessDenied: arn:aws:iam::438097524343:role/internal-secret-path"),
	})
	db.put(testProjects, "USER#u1", "SHORTS#j1", map[string]dbtypes.AttributeValue{
		"status": str("completed"),
	})
	ml := &fakeMailer{}
	return &fixture{
		db: db,
		ml: ml,
		n: &Notifier{
			DB: db, Mailer: ml,
			ProjectsTable: testProjects, UsersTable: testUsers, BaseURL: testBase,
			Now: func() time.Time { return time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC) },
		},
	}
}

func quizDone(exec string) Input {
	return Input{Kind: KindQuiz, Outcome: OutcomeCompleted, UserID: "u1", ProjectID: "p1", ExecutionID: exec}
}

func errTypeName(err error) string {
	t := reflect.TypeOf(err)
	if t.Kind() == reflect.Ptr {
		return t.Elem().Name()
	}
	return t.Name()
}

// ---------------------------------------------------------------- tests

func TestSendsOnceForOneExecution(t *testing.T) {
	f := newFixture()
	ctx := context.Background()

	r, err := f.n.Handle(ctx, quizDone("exec-1"))
	if err != nil || r.Status != StatusSent {
		t.Fatalf("first call: %+v, %v", r, err)
	}
	// Step Functions retry inside the same execution.
	r, err = f.n.Handle(ctx, quizDone("exec-1"))
	if err != nil || r.Status != StatusSkipped || r.Reason != "already_notified" {
		t.Fatalf("second call: %+v, %v", r, err)
	}
	if len(f.ml.sent) != 1 {
		t.Fatalf("sent %d emails, want exactly 1", len(f.ml.sent))
	}
	job := f.db.get(testProjects, "USER#u1", "PROJECT#p1")
	if sv(job, "notifiedExecution") != "exec-1" || sv(job, "notifyMessageId") != "msg-1" {
		t.Fatalf("claim/message id not recorded: %+v", job)
	}
}

func TestLaterExecutionOfSameProjectNotifiesAgain(t *testing.T) {
	f := newFixture()
	ctx := context.Background()

	if _, err := f.n.Handle(ctx, quizDone("exec-1")); err != nil {
		t.Fatal(err)
	}
	// The user retried the project; a new execution finishes.
	r, err := f.n.Handle(ctx, quizDone("exec-2"))
	if err != nil || r.Status != StatusSent {
		t.Fatalf("second execution: %+v, %v", r, err)
	}
	if len(f.ml.sent) != 2 {
		t.Fatalf("sent %d emails, want 2 (one per execution)", len(f.ml.sent))
	}
}

func TestPreferences(t *testing.T) {
	tests := []struct {
		name       string
		attrs      map[string]dbtypes.AttributeValue
		outcome    string
		wantSent   bool
		wantReason string
	}{
		{"defaults are on (completed)", nil, OutcomeCompleted, true, ""},
		{"defaults are on (failed)", nil, OutcomeFailed, true, ""},
		{"completed off blocks completed", map[string]dbtypes.AttributeValue{"notifyCompleted": flag(false)}, OutcomeCompleted, false, "preference_off"},
		{"completed off still sends failed", map[string]dbtypes.AttributeValue{"notifyCompleted": flag(false)}, OutcomeFailed, true, ""},
		{"failed off blocks failed", map[string]dbtypes.AttributeValue{"notifyFailed": flag(false)}, OutcomeFailed, false, "preference_off"},
		{"failed off still sends completed", map[string]dbtypes.AttributeValue{"notifyFailed": flag(false)}, OutcomeCompleted, true, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture()
			attrs := map[string]dbtypes.AttributeValue{"email": str("person@example.com")}
			for k, v := range tt.attrs {
				attrs[k] = v
			}
			f.db.put(testUsers, "USER#u1", "PROFILE", attrs)
			f.db.put(testProjects, "USER#u1", "PROJECT#p1", map[string]dbtypes.AttributeValue{"status": str(tt.outcome), "name": str("Quiz")})

			in := quizDone("exec-1")
			in.Outcome = tt.outcome
			r, err := f.n.Handle(context.Background(), in)
			if err != nil {
				t.Fatal(err)
			}
			if got := r.Status == StatusSent; got != tt.wantSent {
				t.Fatalf("sent=%v, want %v (%+v)", got, tt.wantSent, r)
			}
			if !tt.wantSent {
				if r.Reason != tt.wantReason {
					t.Fatalf("reason=%q, want %q", r.Reason, tt.wantReason)
				}
				if len(f.db.updates) != 0 {
					t.Fatal("an opted-out user must cause no writes")
				}
			}
		})
	}
}

func TestUnverifiedRecipientIsSkippedWithoutWrites(t *testing.T) {
	f := newFixture()
	f.db.put(testUsers, "USER#u1", "PROFILE", map[string]dbtypes.AttributeValue{
		"email":        str("stranger-could-have-typed-this@example.com"),
		"passwordHash": str("$2a$10$hash"),
	})
	r, err := f.n.Handle(context.Background(), quizDone("exec-1"))
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != StatusSkipped || r.Reason != "email_not_verified" {
		t.Fatalf("got %+v", r)
	}
	if len(f.ml.sent) != 0 || len(f.db.updates) != 0 {
		t.Fatalf("sent=%d writes=%d, want none", len(f.ml.sent), len(f.db.updates))
	}
}

func TestSkips(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(f *fixture)
		in     Input
		reason string
	}{
		{"no profile", func(f *fixture) { delete(f.db.items, dbKey(testUsers, "USER#u1", "PROFILE")) }, quizDone("e"), "no_profile"},
		{"no email", func(f *fixture) {
			f.db.put(testUsers, "USER#u1", "PROFILE", map[string]dbtypes.AttributeValue{"name": str("x")})
		}, quizDone("e"), "no_email"},
		{"job deleted", func(f *fixture) { delete(f.db.items, dbKey(testProjects, "USER#u1", "PROJECT#p1")) }, quizDone("e"), "job_missing"},
		{"status moved on", func(f *fixture) {
			f.db.put(testProjects, "USER#u1", "PROJECT#p1", map[string]dbtypes.AttributeValue{"status": str("rendering")})
		}, quizDone("e"), "status_mismatch"},
		{"shorts cancelled by user", func(f *fixture) {
			f.db.put(testProjects, "USER#u1", "SHORTS#j1", map[string]dbtypes.AttributeValue{"status": str("failed"), "errorCode": str("CANCELLED")})
		}, Input{Kind: KindShorts, Outcome: OutcomeFailed, UserID: "u1", JobID: "j1", ExecutionID: "e"}, "cancelled_by_user"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture()
			tt.mutate(f)
			r, err := f.n.Handle(context.Background(), tt.in)
			if err != nil {
				t.Fatal(err)
			}
			if r.Status != StatusSkipped || r.Reason != tt.reason {
				t.Fatalf("got %+v, want skipped/%s", r, tt.reason)
			}
			if len(f.ml.sent) != 0 {
				t.Fatal("must not send")
			}
		})
	}
}

func TestThrottleReturnsTypedErrorAndReleasesClaim(t *testing.T) {
	f := newFixture()
	f.ml.errs = []error{fmt.Errorf("%w: TooManyRequests", ErrThrottled)}
	ctx := context.Background()

	_, err := f.n.Handle(ctx, quizDone("exec-1"))
	if err == nil {
		t.Fatal("want an error")
	}
	// Step Functions matches the retry on this exact name.
	if name := errTypeName(err); name != "Throttled" {
		t.Fatalf("error type name %q, want Throttled", name)
	}
	var th *Throttled
	if !errors.As(err, &th) {
		t.Fatal("want *Throttled")
	}
	job := f.db.get(testProjects, "USER#u1", "PROJECT#p1")
	if sv(job, "notifiedExecution") != "" {
		t.Fatal("claim must be released so the retry can send")
	}
	if sv(job, "status") != "completed" {
		t.Fatal("job status must be untouched")
	}

	// The state machine's retry: now SES accepts it.
	r, err := f.n.Handle(ctx, quizDone("exec-1"))
	if err != nil || r.Status != StatusSent {
		t.Fatalf("retry: %+v, %v", r, err)
	}
	if len(f.ml.sent) != 1 {
		t.Fatalf("sent %d, want 1", len(f.ml.sent))
	}
}

func TestRejectedIsNotRetriedButReleasesClaim(t *testing.T) {
	f := newFixture()
	f.ml.errs = []error{fmt.Errorf("%w: MessageRejected", ErrRejected)}

	_, err := f.n.Handle(context.Background(), quizDone("exec-1"))
	if name := errTypeName(err); name != "SendFailed" {
		t.Fatalf("error type name %q, want SendFailed (not retried by the state machine)", name)
	}
	if sv(f.db.get(testProjects, "USER#u1", "PROJECT#p1"), "notifiedExecution") != "" {
		t.Fatal("nothing was sent, so the claim should be released")
	}
}

func TestUnknownSendOutcomeKeepsClaimSoNothingDuplicates(t *testing.T) {
	f := newFixture()
	f.ml.errs = []error{errors.New("context deadline exceeded")}
	ctx := context.Background()

	_, err := f.n.Handle(ctx, quizDone("exec-1"))
	if name := errTypeName(err); name != "SendFailed" {
		t.Fatalf("error type name %q, want SendFailed", name)
	}
	r, err := f.n.Handle(ctx, quizDone("exec-1"))
	if err != nil || r.Reason != "already_notified" {
		t.Fatalf("a retry after an unknown outcome must not send again: %+v, %v", r, err)
	}
	if len(f.ml.sent) != 0 {
		t.Fatal("no second send expected")
	}
}

func TestMailFailureNeverChangesJobOutcome(t *testing.T) {
	for _, mailErr := range []error{
		fmt.Errorf("%w: slow down", ErrThrottled),
		fmt.Errorf("%w: rejected", ErrRejected),
		errors.New("connection reset"),
	} {
		f := newFixture()
		f.ml.errs = []error{mailErr}
		before := fmt.Sprint(f.db.get(testProjects, "USER#u1", "PROJECT#p1")["status"])
		_, _ = f.n.Handle(context.Background(), quizDone("exec-1"))
		after := fmt.Sprint(f.db.get(testProjects, "USER#u1", "PROJECT#p1")["status"])
		if before != after {
			t.Fatalf("status changed from %s to %s on %v", before, after, mailErr)
		}
		for _, u := range f.db.updates {
			if strings.Contains(*u.UpdateExpression, "status") {
				t.Fatalf("notifier must never write status, wrote: %s", *u.UpdateExpression)
			}
		}
	}
}

func TestFailedEmailNeverContainsRawError(t *testing.T) {
	f := newFixture()
	f.db.put(testProjects, "USER#u1", "PROJECT#p1", map[string]dbtypes.AttributeValue{
		"status": str("failed"),
		"name":   str("Quiz"),
		"error":  str("AccessDenied: arn:aws:iam::438097524343:role/internal-secret-path"),
	})
	in := quizDone("exec-1")
	in.Outcome = OutcomeFailed
	if _, err := f.n.Handle(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	m := f.ml.sent[0]
	for _, part := range []string{m.Subject, m.Text, m.HTML} {
		if strings.Contains(part, "AccessDenied") || strings.Contains(part, "438097524343") || strings.Contains(part, "internal-secret-path") {
			t.Fatalf("raw error leaked into email: %q", part)
		}
	}
}

func TestInvalidInput(t *testing.T) {
	tests := []Input{
		{Kind: KindQuiz, Outcome: OutcomeCompleted, ProjectID: "p1", ExecutionID: "e"},              // no user
		{Kind: KindQuiz, Outcome: OutcomeCompleted, UserID: "u1", ExecutionID: "e"},                 // no project
		{Kind: KindShorts, Outcome: OutcomeCompleted, UserID: "u1", ExecutionID: "e"},               // no job
		{Kind: KindQuiz, Outcome: OutcomeCompleted, UserID: "u1", ProjectID: "p1"},                  // no execution
		{Kind: "other", Outcome: OutcomeCompleted, UserID: "u1", ProjectID: "p1", ExecutionID: "e"}, // bad kind
		{Kind: KindQuiz, Outcome: "running", UserID: "u1", ProjectID: "p1", ExecutionID: "e"},       // bad outcome
	}
	for i, in := range tests {
		f := newFixture()
		_, err := f.n.Handle(context.Background(), in)
		if err == nil {
			t.Fatalf("case %d: want error", i)
		}
		if len(f.ml.sent) != 0 || len(f.db.updates) != 0 {
			t.Fatalf("case %d: invalid input must not send or write", i)
		}
	}
}

func TestDatabaseErrorIsSendFailed(t *testing.T) {
	f := newFixture()
	f.db.getErr = errors.New("dynamodb unavailable")
	_, err := f.n.Handle(context.Background(), quizDone("exec-1"))
	if name := errTypeName(err); name != "SendFailed" {
		t.Fatalf("error type name %q, want SendFailed", name)
	}
}

func TestShortsEmailsUseShortsLinks(t *testing.T) {
	f := newFixture()
	ctx := context.Background()
	if _, err := f.n.Handle(ctx, Input{Kind: KindShorts, Outcome: OutcomeCompleted, UserID: "u1", JobID: "j1", ExecutionID: "e"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(f.ml.sent[0].Text, testBase+"/shorts/j1/clips") {
		t.Fatalf("completed shorts link wrong:\n%s", f.ml.sent[0].Text)
	}
}

// TestUpdateExpressions pins the exact DynamoDB expressions. The fake above
// interprets them by prefix, so this guards against the real thing drifting.
func TestUpdateExpressions(t *testing.T) {
	f := newFixture()
	if _, err := f.n.Handle(context.Background(), quizDone("exec-1")); err != nil {
		t.Fatal(err)
	}
	if len(f.db.updates) != 2 {
		t.Fatalf("want claim + record message id, got %d updates", len(f.db.updates))
	}
	claim, record := f.db.updates[0], f.db.updates[1]

	if got, want := *claim.UpdateExpression, "SET notifiedExecution = :exec, notifiedAt = :at"; got != want {
		t.Errorf("claim update:\n got %q\nwant %q", got, want)
	}
	if got, want := *claim.ConditionExpression,
		"attribute_exists(PK) AND (attribute_not_exists(notifiedExecution) OR notifiedExecution <> :exec)"; got != want {
		t.Errorf("claim condition:\n got %q\nwant %q", got, want)
	}
	if got, want := *record.UpdateExpression, "SET notifyMessageId = :mid"; got != want {
		t.Errorf("record update:\n got %q\nwant %q", got, want)
	}
	if got, want := *record.ConditionExpression, "notifiedExecution = :exec"; got != want {
		t.Errorf("record condition:\n got %q\nwant %q", got, want)
	}
	if *claim.TableName != testProjects {
		t.Errorf("claim must target the projects table, got %s", *claim.TableName)
	}
	if sv(f.db.get(testProjects, "USER#u1", "PROJECT#p1"), "notifiedAt") != "2026-10-18T12:00:00Z" {
		t.Error("notifiedAt not recorded in RFC3339 UTC")
	}

	// Release shape (throttle path).
	f2 := newFixture()
	f2.ml.errs = []error{fmt.Errorf("%w: x", ErrThrottled)}
	_, _ = f2.n.Handle(context.Background(), quizDone("exec-1"))
	rel := f2.db.updates[1]
	if got, want := *rel.UpdateExpression, "REMOVE notifiedExecution, notifiedAt"; got != want {
		t.Errorf("release update:\n got %q\nwant %q", got, want)
	}
	if got, want := *rel.ConditionExpression, "notifiedExecution = :exec"; got != want {
		t.Errorf("release condition:\n got %q\nwant %q", got, want)
	}
}
