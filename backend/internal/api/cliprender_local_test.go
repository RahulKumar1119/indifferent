package api

// The single-clip render route (#329) against DynamoDB Local, so the claim,
// the quota counter and the idempotent replay are checked by DynamoDB's own
// conditional writes. Skipped unless DYNAMODB_LOCAL_ENDPOINT is set:
//
//	scripts/ops/cliprender_dynamo_check.sh

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/sfn"
	"github.com/rahul/indifferent/backend/internal/auth"
	"github.com/rahul/indifferent/backend/internal/models"
	"github.com/rahul/indifferent/backend/internal/shorts/cliprender"
	"github.com/rahul/indifferent/backend/internal/storage"
)

type recordingSFN struct {
	mu      sync.Mutex
	starts  []*sfn.StartExecutionInput
	stops   []string
	failErr error
}

func (r *recordingSFN) StartExecution(_ context.Context, in *sfn.StartExecutionInput, _ ...func(*sfn.Options)) (*sfn.StartExecutionOutput, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.failErr != nil {
		return nil, r.failErr
	}
	r.starts = append(r.starts, in)
	return &sfn.StartExecutionOutput{}, nil
}

func (r *recordingSFN) StopExecution(_ context.Context, in *sfn.StopExecutionInput, _ ...func(*sfn.Options)) (*sfn.StopExecutionOutput, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stops = append(r.stops, *in.ExecutionArn)
	return &sfn.StopExecutionOutput{}, nil
}

func (r *recordingSFN) startCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.starts)
}

type localEnv struct {
	t        *testing.T
	h        *APIHandler
	db       *dynamodb.Client
	table    string
	sessions string
	sfn      *recordingSFN
	s3       *stubDownloader
}

func createLocalTable(t *testing.T, db *dynamodb.Client, name string) {
	t.Helper()
	_, err := db.CreateTable(context.Background(), &dynamodb.CreateTableInput{
		TableName: aws.String(name),
		AttributeDefinitions: []dbtypes.AttributeDefinition{
			{AttributeName: aws.String("PK"), AttributeType: dbtypes.ScalarAttributeTypeS},
			{AttributeName: aws.String("SK"), AttributeType: dbtypes.ScalarAttributeTypeS},
		},
		KeySchema: []dbtypes.KeySchemaElement{
			{AttributeName: aws.String("PK"), KeyType: dbtypes.KeyTypeHash},
			{AttributeName: aws.String("SK"), KeyType: dbtypes.KeyTypeRange},
		},
		BillingMode: dbtypes.BillingModePayPerRequest,
	})
	if err != nil {
		t.Fatalf("create table: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.DeleteTable(context.Background(), &dynamodb.DeleteTableInput{TableName: aws.String(name)})
	})
}

const localSource = "uploads/" + pcUser + "/job1/source.mp4"

func newLocalEnv(t *testing.T) *localEnv {
	t.Helper()
	endpoint := os.Getenv("DYNAMODB_LOCAL_ENDPOINT")
	if endpoint == "" {
		t.Skip("DYNAMODB_LOCAL_ENDPOINT not set (see scripts/ops/cliprender_dynamo_check.sh)")
	}
	fixedQuotaClock(t, time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC))
	db := dynamodb.New(dynamodb.Options{Region: "ap-south-1", BaseEndpoint: aws.String(endpoint),
		Credentials: credentials.NewStaticCredentialsProvider("local", "local", "")})
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	e := &localEnv{t: t, db: db, table: "projects-" + suffix, sessions: "sessions-" + suffix, sfn: &recordingSFN{}}
	createLocalTable(t, db, e.table)
	createLocalTable(t, db, e.sessions)
	e.s3 = &stubDownloader{status: storage.ObjectStatus{StorageClass: "STANDARD"}, meta: storage.ObjectMeta{Found: true}}
	e.h = &APIHandler{
		AuthService: &mockAuthService{}, JWTService: &auth.JWTService{Secret: testSecret, DB: db, SessionTable: e.sessions},
		DB: db, S3: e.s3, SFN: e.sfn, TableName: e.table, QuotaTable: e.sessions, Bucket: "bucket",
		ClipRenderStateMachineARN: "arn:aws:states:ap-south-1:123456789012:stateMachine:clip-render",
	}
	job := models.ShortsJob{
		UserID: pcUser, JobID: pcJob, Status: "completed", FileType: "mp4", SourceDuration: 120,
		SourceKey: localSource, TranscriptKey: "shorts/" + pcUser + "/job1/transcript.json",
		Clips: []models.Clip{
			{ClipID: "clip-1", S3Key: "shorts/" + pcUser + "/job1/clips/clip-1.mp4", Rank: 1, Score: 0.9, Duration: 30, Start: 5, End: 35, HookText: "hook one"},
			{ClipID: "clip-2", S3Key: "shorts/" + pcUser + "/job1/clips/clip-2.mp4", Rank: 2, Duration: 30, Start: 60, End: 90},
		},
		CreatedAt: "2026-11-20T09:00:00Z", UpdatedAt: "2026-11-20T09:30:00Z", CompletedAt: "2026-11-20T09:30:00Z",
	}
	if _, err := db.PutItem(context.Background(), &dynamodb.PutItemInput{TableName: aws.String(e.table), Item: shortsJobToItem(job)}); err != nil {
		t.Fatal(err)
	}
	return e
}

func (e *localEnv) do(method, path, body, user string) events.APIGatewayProxyResponse {
	headers := map[string]string{}
	if user != "" {
		headers["Authorization"] = "Bearer " + generateTestToken(user)
	}
	resp, _ := e.h.HandleRequest(context.Background(), events.APIGatewayProxyRequest{HTTPMethod: method, Path: path, Body: body, Headers: headers})
	return resp
}

func (e *localEnv) render(clip string, n int) events.APIGatewayProxyResponse {
	return e.do("POST", "/shorts/job1/clips/"+clip+"/render", `{"requestId":"`+rid(n)+`"}`, pcUser)
}

func (e *localEnv) used() int64 {
	out, err := e.db.GetItem(context.Background(), &dynamodb.GetItemInput{TableName: aws.String(e.sessions), Key: map[string]dbtypes.AttributeValue{
		"PK": &dbtypes.AttributeValueMemberS{Value: "QUOTA#" + pcUser},
		"SK": &dbtypes.AttributeValueMemberS{Value: quotaShortsRetry.name + "#20261120"},
	}, ConsistentRead: aws.Bool(true)})
	if err != nil || out.Item == nil {
		return 0
	}
	return pcN(out.Item["uses"])
}

func (e *localEnv) record(clip string) *cliprender.Record {
	r, err := cliprender.Get(context.Background(), e.db, e.table, pcUser, pcJob, clip)
	if err != nil {
		e.t.Fatal(err)
	}
	return r
}

func (e *localEnv) job() models.ShortsJob {
	out, _ := e.db.GetItem(context.Background(), &dynamodb.GetItemInput{TableName: aws.String(e.table), Key: map[string]dbtypes.AttributeValue{
		"PK": &dbtypes.AttributeValueMemberS{Value: "USER#" + pcUser}, "SK": &dbtypes.AttributeValueMemberS{Value: "SHORTS#" + pcJob}}, ConsistentRead: aws.Bool(true)})
	return itemToShortsJob(out.Item)
}

func TestLocalAPI_RenderIsAcceptedOnce_ChargedOnce_AndRepeatsAreReplayed(t *testing.T) {
	e := newLocalEnv(t)
	before := e.job()
	resp := e.render("clip-1", 1)
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("status %d: %s", resp.StatusCode, resp.Body)
	}
	var out ClipRenderResponse
	_ = json.Unmarshal([]byte(resp.Body), &out)
	if out.Render.State != models.ClipRenderRendering || out.Replayed || out.RemainingToday == nil || *out.RemainingToday != 14 {
		t.Fatalf("response %+v", out)
	}
	if e.used() != 1 || e.sfn.startCount() != 1 {
		t.Fatalf("used %d starts %d", e.used(), e.sfn.startCount())
	}
	rec := e.record("clip-1")
	start := e.sfn.starts[0]
	var input map[string]any
	_ = json.Unmarshal([]byte(*start.Input), &input)
	if *start.Name != cliprender.ExecutionName(rec.Token) || input["token"] != rec.Token || input["clipId"] != "clip-1" ||
		input["start"] != 5.0 || input["end"] != 35.0 || input["sourceKey"] != localSource || input["userId"] != pcUser {
		t.Fatalf("execution %s input %v record %+v", *start.Name, input, rec)
	}

	// The same request again, as a retry after a lost response: answered, not
	// charged, nothing new started.
	again := e.render("clip-1", 1)
	var replay ClipRenderResponse
	_ = json.Unmarshal([]byte(again.Body), &replay)
	if again.StatusCode != http.StatusAccepted || !replay.Replayed || e.used() != 1 || e.sfn.startCount() != 1 {
		t.Fatalf("replay %d %+v used %d starts %d", again.StatusCode, replay, e.used(), e.sfn.startCount())
	}
	// Another request while it renders is refused without a charge.
	busy := e.render("clip-1", 2)
	if busy.StatusCode != http.StatusConflict || !strings.Contains(busy.Body, "RENDER_IN_PROGRESS") || e.used() != 1 {
		t.Fatalf("busy %d %s used %d", busy.StatusCode, busy.Body, e.used())
	}
	// The job record and its clip are untouched.
	after := e.job()
	if after.UpdatedAt != before.UpdatedAt || after.Clips[0] != before.Clips[0] || after.Status != "completed" {
		t.Fatalf("job changed: %+v", after)
	}
	// The page sees the render.
	get := e.do("GET", "/shorts/job1", "", pcUser)
	if !strings.Contains(get.Body, `"clipRenders":{"clip-1":{"state":"rendering"`) {
		t.Fatalf("GET job: %s", get.Body)
	}
	if strings.Contains(get.Body, rec.Token) {
		t.Fatal("the token must not reach the page")
	}
}

func TestLocalAPI_ConcurrentRequestsStartOneRender(t *testing.T) {
	e := newLocalEnv(t)
	var wg sync.WaitGroup
	codes := make([]int, 12)
	for i := range codes {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			// Half of them repeat the same id, half are distinct.
			n := 100
			if i%2 == 1 {
				n = 200 + i
			}
			codes[i] = e.render("clip-1", n).StatusCode
		}(i)
	}
	wg.Wait()
	if e.sfn.startCount() != 1 || e.used() != 1 {
		t.Fatalf("starts %d used %d codes %v", e.sfn.startCount(), e.used(), codes)
	}
	for i, c := range codes {
		if c != http.StatusAccepted && c != http.StatusConflict {
			t.Fatalf("request %d: %d", i, c)
		}
	}
}

func TestLocalAPI_QuotaExhaustedLeavesTheClipAsItWas(t *testing.T) {
	e := newLocalEnv(t)
	withLimit(t, &quotaShortsRetry, 1)
	if r := e.render("clip-2", 1); r.StatusCode != http.StatusAccepted {
		t.Fatalf("first: %d %s", r.StatusCode, r.Body)
	}
	resp := e.render("clip-1", 2)
	if resp.StatusCode != http.StatusTooManyRequests || !strings.Contains(resp.Body, "rendering clips again") {
		t.Fatalf("status %d: %s", resp.StatusCode, resp.Body)
	}
	if e.record("clip-1") != nil || e.sfn.startCount() != 1 {
		t.Fatalf("a refused render must leave no claim and start nothing: %+v", e.record("clip-1"))
	}
}

func TestLocalAPI_StartFailureGivesTheUseBackAndRestoresTheRecord(t *testing.T) {
	e := newLocalEnv(t)
	// A finished earlier render, so there is a record to put back.
	tok := strings.Repeat("a", 32)
	now := time.Now()
	first := cliprender.Record{UserID: pcUser, JobID: pcJob, ClipID: "clip-1", Token: tok, State: models.ClipRenderRendering, RequestIDs: []string{rid(9)}, RequestedAt: now}
	if err := cliprender.Claim(context.Background(), e.db, e.table, nil, first, now); err != nil {
		t.Fatal(err)
	}
	active := cliprender.OutputKey(pcUser, pcJob, "clip-1", tok)
	if err := cliprender.Promote(context.Background(), e.db, e.table, pcUser, pcJob, "clip-1", tok, active, now); err != nil {
		t.Fatal(err)
	}
	e.sfn.failErr = fmt.Errorf("throttled")
	resp := e.render("clip-1", 1)
	if resp.StatusCode != http.StatusServiceUnavailable || !strings.Contains(resp.Body, "did not use up") {
		t.Fatalf("status %d: %s", resp.StatusCode, resp.Body)
	}
	rec := e.record("clip-1")
	if e.used() != 0 || rec.Token != tok || rec.State != models.ClipRenderDone || rec.ActiveKey != active || rec.HasRequest(rid(1)) {
		t.Fatalf("used %d record %+v", e.used(), rec)
	}
	// The same id can be tried again once the service is back: it was never accepted.
	e.sfn.failErr = nil
	if r := e.render("clip-1", 1); r.StatusCode != http.StatusAccepted || e.used() != 1 {
		t.Fatalf("retry: %d used %d", r.StatusCode, e.used())
	}
}

func TestLocalAPI_FinishedRenderIsWhatTheClipPlays_AndTheOldFileGoesNextTime(t *testing.T) {
	e := newLocalEnv(t)
	ctx := context.Background()
	// Render 1 finishes (what the worker does).
	if r := e.render("clip-1", 1); r.StatusCode != http.StatusAccepted {
		t.Fatal(r.Body)
	}
	r1 := e.record("clip-1")
	key1 := cliprender.OutputKey(pcUser, pcJob, "clip-1", r1.Token)
	if err := cliprender.Promote(ctx, e.db, e.table, pcUser, pcJob, "clip-1", r1.Token, key1, time.Now()); err != nil {
		t.Fatal(err)
	}
	url := e.do("GET", "/shorts/job1/clips/clip-1/url", "", pcUser)
	if !strings.Contains(url.Body, key1) {
		t.Fatalf("clip URL should play the new file: %s", url.Body)
	}
	other := e.do("GET", "/shorts/job1/clips/clip-2/url", "", pcUser)
	if !strings.Contains(other.Body, "clips/clip-2.mp4") {
		t.Fatalf("another clip keeps its file: %s", other.Body)
	}
	// Render 2 finishes; the clip plays it and keeps render 1 as the previous file.
	if r := e.render("clip-1", 2); r.StatusCode != http.StatusAccepted {
		t.Fatal(r.Body)
	}
	r2 := e.record("clip-1")
	key2 := cliprender.OutputKey(pcUser, pcJob, "clip-1", r2.Token)
	if err := cliprender.Promote(ctx, e.db, e.table, pcUser, pcJob, "clip-1", r2.Token, key2, time.Now()); err != nil {
		t.Fatal(err)
	}
	if rec := e.record("clip-1"); rec.ActiveKey != key2 || rec.PreviousKey != key1 || rec.Renders != 2 {
		t.Fatalf("record %+v", rec)
	}
	// Render 3 starts: render 1's file is deleted; render 2's still plays.
	e.s3.deletedKeys = nil
	if r := e.render("clip-1", 3); r.StatusCode != http.StatusAccepted {
		t.Fatal(r.Body)
	}
	if len(e.s3.deletedKeys) != 1 || e.s3.deletedKeys[0] != key1 {
		t.Fatalf("deleted %v", e.s3.deletedKeys)
	}
	if url := e.do("GET", "/shorts/job1/clips/clip-1/url", "", pcUser); !strings.Contains(url.Body, key2) {
		t.Fatalf("while render 3 runs the clip plays render 2: %s", url.Body)
	}
	if e.used() != 3 {
		t.Fatalf("used %d, want one per accepted request", e.used())
	}
}

func TestLocalAPI_HookSaveIsStoredOnTheJob_AndMeasured(t *testing.T) {
	e := newLocalEnv(t)
	resp := e.do("PUT", "/shorts/job1/clips/clip-1/hook", `{"hook":{"enabled":true,"text":"  यह गलती मत करना  ","style":"band","position":"middle","seconds":3}}`, pcUser)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d: %s", resp.StatusCode, resp.Body)
	}
	j := e.job()
	h := j.ClipHooks["clip-1"]
	if h.Text != "यह गलती मत करना" || h.Style != "band" || h.Position != "middle" || h.Seconds != 3 || h.UpdatedAt == "" || len(j.Clips) != 2 {
		t.Fatalf("stored %+v", j.ClipHooks)
	}
	for _, body := range []string{
		`{"hook":{"enabled":true,"text":"` + strings.Repeat("W", 60) + `"}}`,
		`{"hook":{"enabled":true,"text":"` + strings.Repeat("a", 81) + `"}}`,
		`{"hook":{"enabled":true,"text":"x","position":"bottom"}}`,
		`{"hook":{"enabled":true,"text":"x","extra":1}}`,
		`{}`,
	} {
		if r := e.do("PUT", "/shorts/job1/clips/clip-1/hook", body, pcUser); r.StatusCode != http.StatusBadRequest {
			t.Errorf("%s: %d %s", body, r.StatusCode, r.Body)
		}
	}
	if r := e.do("PUT", "/shorts/job1/clips/clip-1/hook", `{"reset":true}`, pcUser); r.StatusCode != http.StatusOK {
		t.Fatalf("reset: %d", r.StatusCode)
	}
	if _, ok := e.job().ClipHooks["clip-1"]; ok {
		t.Fatal("reset left the hook")
	}
}

func TestLocalAPI_DeletingTheJobStopsRendersAndRemovesTheirRecords(t *testing.T) {
	e := newLocalEnv(t)
	if r := e.render("clip-1", 1); r.StatusCode != http.StatusAccepted {
		t.Fatal(r.Body)
	}
	rec := e.record("clip-1")
	if r := e.do("DELETE", "/shorts/job1", "", pcUser); r.StatusCode != http.StatusNoContent {
		t.Fatalf("delete: %d %s", r.StatusCode, r.Body)
	}
	if e.record("clip-1") != nil {
		t.Fatal("render record left after the job was deleted")
	}
	if len(e.sfn.stops) != 1 || !strings.HasSuffix(e.sfn.stops[0], ":clip-render:"+rec.ExecutionName) {
		t.Fatalf("stops %v", e.sfn.stops)
	}
	// The worker can no longer point the clip at a file.
	if err := cliprender.Promote(context.Background(), e.db, e.table, pcUser, pcJob, "clip-1", rec.Token, "k", time.Now()); err == nil {
		t.Fatal("promote after delete must fail")
	}
}
