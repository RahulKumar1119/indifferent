package thumbnail

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	db "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// These tests run the store against a real DynamoDB engine, because the
// update and condition expressions are strings that only DynamoDB can check
// (reserved words, syntax, condition semantics). Start DynamoDB Local and set
// the endpoint to run them:
//
//	docker run -d --rm -p 127.0.0.1:8765:8000 amazon/dynamodb-local
//	DYNAMODB_LOCAL_ENDPOINT=http://127.0.0.1:8765 go test ./internal/thumbnail -run Integration
func integrationStore(t *testing.T) (*Store, *dynamodb.Client) {
	t.Helper()
	endpoint := os.Getenv("DYNAMODB_LOCAL_ENDPOINT")
	if endpoint == "" {
		t.Skip("DYNAMODB_LOCAL_ENDPOINT not set")
	}
	client := dynamodb.New(dynamodb.Options{
		Region:       "ap-south-1",
		BaseEndpoint: aws.String(endpoint),
		Credentials:  credentials.NewStaticCredentialsProvider("local", "local", ""),
	})
	table := fmt.Sprintf("thumbs-test-%d", time.Now().UnixNano())
	_, err := client.CreateTable(context.Background(), &dynamodb.CreateTableInput{
		TableName: aws.String(table),
		KeySchema: []db.KeySchemaElement{
			{AttributeName: aws.String("PK"), KeyType: db.KeyTypeHash},
			{AttributeName: aws.String("SK"), KeyType: db.KeyTypeRange},
		},
		AttributeDefinitions: []db.AttributeDefinition{
			{AttributeName: aws.String("PK"), AttributeType: db.ScalarAttributeTypeS},
			{AttributeName: aws.String("SK"), AttributeType: db.ScalarAttributeTypeS},
		},
		BillingMode: db.BillingModePayPerRequest,
	})
	if err != nil {
		t.Fatalf("create table: %v", err)
	}
	t.Cleanup(func() {
		_, _ = client.DeleteTable(context.Background(), &dynamodb.DeleteTableInput{TableName: aws.String(table)})
	})
	return &Store{DB: client, Table: table}, client
}

func TestIntegrationLifecycle(t *testing.T) {
	ctx := context.Background()
	s, _ := integrationStore(t)

	// Nothing yet.
	if set, err := s.Get(ctx, "u1", "j1"); err != nil || set != nil {
		t.Fatalf("empty Get = %v %v", set, err)
	}

	// Queue (first time creates the record).
	if err := s.Queue(ctx, "u1", "j1", StyleNews, FormatPortrait); err != nil {
		t.Fatalf("Queue: %v", err)
	}
	set, _ := s.Get(ctx, "u1", "j1")
	if set.Status != StatusQueued {
		t.Fatalf("status after Queue = %q", set.Status)
	}
	// A second Queue while queued is refused.
	if err := s.Queue(ctx, "u1", "j1", "", ""); !errors.Is(err, ErrInFlight) {
		t.Fatalf("second Queue = %v, want ErrInFlight", err)
	}

	// The worker claims it and learns what was asked for.
	gen, style, format, err := s.Begin(ctx, "u1", "j1")
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}
	if gen != 1 || style != StyleNews || format != FormatPortrait {
		t.Fatalf("Begin = %d %q %q", gen, style, format)
	}
	// A second worker cannot claim a fresh running record.
	if _, _, _, err := s.Begin(ctx, "u1", "j1"); !errors.Is(err, ErrInFlight) {
		t.Fatalf("second Begin = %v, want ErrInFlight", err)
	}
	// Queue is refused while running too.
	if err := s.Queue(ctx, "u1", "j1", "", ""); !errors.Is(err, ErrInFlight) {
		t.Fatalf("Queue while running = %v", err)
	}

	if err := s.SetStage(ctx, "u1", "j1", gen, StageAnalyzing); err != nil {
		t.Fatalf("SetStage: %v", err)
	}
	set, _ = s.Get(ctx, "u1", "j1")
	if set.Status != StatusRunning || set.Stage != StageAnalyzing {
		t.Fatalf("running state = %q %q", set.Status, set.Stage)
	}
	// A worker from another generation is shut out.
	if err := s.SetStage(ctx, "u1", "j1", gen+1, StageRendering); !errors.Is(err, ErrGone) {
		t.Fatalf("stale-generation write = %v, want ErrGone", err)
	}

	// Complete.
	res := Result{
		Thumbnails: []Asset{
			{ID: "thumb-01", Concept: ConceptFace, Key: "k1", Headline: "HI", Width: 720, Height: 1280},
			{ID: "thumb-02", Concept: ConceptSubject, Key: "k2", Headline: "HI", Width: 720, Height: 1280},
		},
		Analysis: Analysis{Topic: "t", Headline: "HI", BestFrameTime: 3.5},
		Funnel:   Funnel{Extracted: 10, Usable: 8, Unique: 6, Candidates: 4},
		Style:    StyleNews, Format: FormatPortrait, TranscriptUsed: true, DetectionUsed: true,
	}
	if err := s.Complete(ctx, "u1", "j1", gen, res); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	set, _ = s.Get(ctx, "u1", "j1")
	if set.Status != StatusCompleted || set.Stage != "" || len(set.Thumbnails) != 2 || set.Style != StyleNews || set.Format != FormatPortrait ||
		!set.TranscriptUsed || set.Funnel == nil || set.Funnel.Candidates != 4 || set.Analysis == nil || set.Analysis.BestFrameTime != 3.5 || set.Generation != 1 {
		t.Fatalf("completed set = %+v", set)
	}

	// Select.
	if err := s.Select(ctx, "u1", "j1", "thumb-02"); err != nil {
		t.Fatalf("Select: %v", err)
	}
	set, _ = s.Get(ctx, "u1", "j1")
	if sel := set.Selected(); sel == nil || sel.ID != "thumb-02" {
		t.Fatalf("selected = %+v", sel)
	}

	// Regenerate: allowed from completed, keeps the old thumbnails while it runs.
	if err := s.Queue(ctx, "u1", "j1", "", ""); err != nil {
		t.Fatalf("Queue after completion: %v", err)
	}
	set, _ = s.Get(ctx, "u1", "j1")
	if set.Status != StatusQueued || len(set.Thumbnails) != 2 || set.SelectedID != "thumb-02" {
		t.Fatalf("regenerate should keep the previous set until it is replaced: %+v", set)
	}
	gen2, style2, format2, err := s.Begin(ctx, "u1", "j1")
	if err != nil || gen2 != 2 || style2 != "" || format2 != "" {
		t.Fatalf("Begin 2 = %d %q %q %v (the earlier request must be cleared)", gen2, style2, format2, err)
	}
	// Selecting is refused while a run is in progress.
	if err := s.Select(ctx, "u1", "j1", "thumb-01"); !errors.Is(err, ErrNotReady) {
		t.Fatalf("Select while running = %v, want ErrNotReady", err)
	}

	// Fail keeps the earlier thumbnails and records the class.
	if err := s.Fail(ctx, "u1", "j1", gen2, CodeAITimeout, "The AI took too long to respond. Please try again."); err != nil {
		t.Fatalf("Fail: %v", err)
	}
	set, _ = s.Get(ctx, "u1", "j1")
	if set.Status != StatusFailed || set.ErrorCode != CodeAITimeout || set.Error == "" || len(set.Thumbnails) != 2 || set.Stage != "" {
		t.Fatalf("failed set = %+v", set)
	}
	// And a retry is allowed from failed.
	if err := s.Queue(ctx, "u1", "j1", StyleViral, ""); err != nil {
		t.Fatalf("Queue after failure: %v", err)
	}
	set, _ = s.Get(ctx, "u1", "j1")
	if set.ErrorCode != "" || set.Error != "" {
		t.Fatalf("queueing should clear the old error: %+v", set)
	}

	// A queued record that never starts can be aborted; a claimed one cannot.
	if err := s.AbortQueued(ctx, "u1", "j1", CodePipelineError, "We couldn't start thumbnail generation. Please try again."); err != nil {
		t.Fatalf("AbortQueued: %v", err)
	}
	set, _ = s.Get(ctx, "u1", "j1")
	if set.Status != StatusFailed || set.ErrorCode != CodePipelineError {
		t.Fatalf("aborted set = %+v", set)
	}
	if err := s.Queue(ctx, "u1", "j1", "", ""); err != nil {
		t.Fatal(err)
	}
	g3, _, _, err := s.Begin(ctx, "u1", "j1")
	if err != nil || g3 != 3 {
		t.Fatalf("Begin 3 = %d %v", g3, err)
	}
	if err := s.AbortQueued(ctx, "u1", "j1", CodePipelineError, "x"); err != nil {
		t.Fatal(err)
	}
	set, _ = s.Get(ctx, "u1", "j1")
	if set.Status != StatusRunning {
		t.Fatalf("AbortQueued must not touch a running record, status = %q", set.Status)
	}

	// Another user's record is separate.
	if set, _ := s.Get(ctx, "u2", "j1"); set != nil {
		t.Fatal("records must be per user")
	}

	// Delete, after which late writes from the old worker are refused.
	if err := s.Delete(ctx, "u1", "j1"); err != nil {
		t.Fatal(err)
	}
	if err := s.Complete(ctx, "u1", "j1", g3, res); !errors.Is(err, ErrGone) {
		t.Fatalf("write after delete = %v, want ErrGone (it must not recreate the record)", err)
	}
	if set, _ := s.Get(ctx, "u1", "j1"); set != nil {
		t.Fatalf("deleted record came back: %+v", set)
	}
	if err := s.Delete(ctx, "u1", "j1"); err != nil {
		t.Fatalf("deleting twice should be fine: %v", err)
	}
}

func TestIntegrationStaleRunsCanBeRestarted(t *testing.T) {
	ctx := context.Background()
	s, client := integrationStore(t)

	// A task that died long ago left a running record.
	old := time.Now().Add(-2 * StaleAfter).UTC().Format(time.RFC3339)
	_, err := client.PutItem(ctx, &dynamodb.PutItemInput{TableName: aws.String(s.Table), Item: map[string]db.AttributeValue{
		"PK": str("USER#u1"), "SK": str("THUMBS#j1"), "status": str("running"), "stage": str("analyzing"),
		"generation": &db.AttributeValueMemberN{Value: "4"}, "updatedAt": str(old),
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Queue(ctx, "u1", "j1", "", ""); err != nil {
		t.Fatalf("a stale running record should not block a new request: %v", err)
	}
	gen, _, _, err := s.Begin(ctx, "u1", "j1")
	if err != nil || gen != 5 {
		t.Fatalf("Begin = %d %v, want generation 5", gen, err)
	}
}

// The thumbnails state machine writes to DynamoDB itself when the task dies
// without recording anything. This runs the exact parameters from the machine's
// definition against DynamoDB, so a typo there fails here and not in production.
func TestIntegrationStateMachineMarkFailed(t *testing.T) {
	ctx := context.Background()
	s, client := integrationStore(t)

	raw, err := os.ReadFile("../shorts/pipeline/thumbnails.json")
	if err != nil {
		t.Fatal(err)
	}
	var def struct {
		States map[string]struct {
			Parameters struct {
				UpdateExpression          string                       `json:"UpdateExpression"`
				ConditionExpression       string                       `json:"ConditionExpression"`
				ExpressionAttributeNames  map[string]string            `json:"ExpressionAttributeNames"`
				ExpressionAttributeValues map[string]map[string]string `json:"ExpressionAttributeValues"`
			} `json:"Parameters"`
		} `json:"States"`
	}
	if err := json.Unmarshal(raw, &def); err != nil {
		t.Fatal(err)
	}
	p := def.States["MarkFailed"].Parameters
	if p.UpdateExpression == "" {
		t.Fatal("MarkFailed has no UpdateExpression")
	}
	vals := map[string]db.AttributeValue{}
	for k, v := range p.ExpressionAttributeValues {
		if lit, ok := v["S"]; ok {
			vals[k] = str(lit)
		} else if _, ok := v["S.$"]; ok {
			vals[k] = str(time.Now().UTC().Format("2006-01-02T15:04:05.000Z")) // $$.State.EnteredTime
		} else {
			t.Fatalf("unexpected value shape for %s: %v", k, v)
		}
	}
	markFailed := func() error {
		_, err := client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
			TableName: aws.String(s.Table), Key: Key("u1", "j1"),
			UpdateExpression: aws.String(p.UpdateExpression), ConditionExpression: aws.String(p.ConditionExpression),
			ExpressionAttributeNames: p.ExpressionAttributeNames, ExpressionAttributeValues: vals,
		})
		return err
	}

	// A task that died while running is marked failed.
	if err := s.Queue(ctx, "u1", "j1", "", ""); err != nil {
		t.Fatal(err)
	}
	gen, _, _, err := s.Begin(ctx, "u1", "j1")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetStage(ctx, "u1", "j1", gen, StageAnalyzing); err != nil {
		t.Fatal(err)
	}
	if err := markFailed(); err != nil {
		t.Fatalf("MarkFailed on a running record: %v", err)
	}
	set, _ := s.Get(ctx, "u1", "j1")
	if set.Status != StatusFailed || set.ErrorCode != CodeTaskFailed || set.Error == "" || set.Stage != "" {
		t.Fatalf("after MarkFailed: %+v", set)
	}
	// The page reads that as a failed run with a Retry.
	if err := s.Queue(ctx, "u1", "j1", "", ""); err != nil {
		t.Fatalf("a failed record must be retryable: %v", err)
	}

	// A failure the task already recorded (or a finished run) is left alone:
	// the condition fails and the machine's Catch ends cleanly.
	if _, _, _, err := s.Begin(ctx, "u1", "j1"); err != nil {
		t.Fatal(err)
	}
	g, _ := s.Get(ctx, "u1", "j1")
	if err := s.Fail(ctx, "u1", "j1", g.Generation, CodeAITimeout, "The AI took too long to respond. Please try again."); err != nil {
		t.Fatal(err)
	}
	err = markFailed()
	var ccf *db.ConditionalCheckFailedException
	if !errors.As(err, &ccf) {
		t.Fatalf("MarkFailed over a recorded failure = %v, want ConditionalCheckFailedException", err)
	}
	set, _ = s.Get(ctx, "u1", "j1")
	if set.ErrorCode != CodeAITimeout {
		t.Fatalf("the task's own error code was overwritten: %q", set.ErrorCode)
	}
	// And it never creates a record for a job that has none.
	if err := s.Delete(ctx, "u1", "j1"); err != nil {
		t.Fatal(err)
	}
	if err := markFailed(); !errors.As(err, &ccf) {
		t.Fatalf("MarkFailed on a missing record = %v, want ConditionalCheckFailedException", err)
	}
}

func TestIntegrationBeginCreatesARecordForAutomaticRuns(t *testing.T) {
	// The Shorts pipeline starts a run with no earlier Queue call.
	ctx := context.Background()
	s, _ := integrationStore(t)
	gen, style, format, err := s.Begin(ctx, "u1", "auto1")
	if err != nil || gen != 1 || style != "" || format != "" {
		t.Fatalf("Begin = %d %q %q %v", gen, style, format, err)
	}
	set, _ := s.Get(ctx, "u1", "auto1")
	if set == nil || set.Status != StatusRunning || set.Stage != StageExtracting || set.CreatedAt == "" {
		t.Fatalf("record = %+v", set)
	}
}
