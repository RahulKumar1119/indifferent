package api

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/sfn"
	"github.com/rahul/indifferent/backend/internal/auth"
	"github.com/rahul/indifferent/backend/internal/storage"
	"github.com/rahul/indifferent/backend/internal/thumbnail"
)

const thumbSM = "arn:aws:states:us-east-1:123456789:stateMachine:thumbnails"

func sv(v string) dbtypes.AttributeValue { return &dbtypes.AttributeValueMemberS{Value: v} }

// thumbEnv is a handler wired to in-memory fakes, with a Shorts job and an
// optional thumbnail record.
type thumbEnv struct {
	h       *APIHandler
	updates []*dynamodb.UpdateItemInput
	started []*sfn.StartExecutionInput
	updErr  error
	sfnErr  error
	record  map[string]dbtypes.AttributeValue // THUMBS# item, nil for none
	deleted []string
}

func jobItemFor(status, fileType string) map[string]dbtypes.AttributeValue {
	return map[string]dbtypes.AttributeValue{
		"PK": sv("USER#user1"), "SK": sv("SHORTS#job1"), "jobId": sv("job1"),
		"status": s(status), "fileType": s(fileType),
		"sourceDuration": &dbtypes.AttributeValueMemberN{Value: "120"},
		"sourceKey":      sv("uploads/user1/job1/source." + fileType),
		"transcriptKey":  sv("shorts/user1/job1/transcript.json"),
		"createdAt":      sv("2026-10-01T00:00:00Z"), "updatedAt": sv("2026-10-01T00:00:00Z"),
	}
}

func newThumbEnv(t *testing.T, jobStatus, fileType string, record map[string]dbtypes.AttributeValue) *thumbEnv {
	t.Helper()
	e := &thumbEnv{record: record}
	db := &mockDynamoDB{
		getItemFunc: func(_ context.Context, in *dynamodb.GetItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
			pk := in.Key["PK"].(*dbtypes.AttributeValueMemberS).Value
			sk := in.Key["SK"].(*dbtypes.AttributeValueMemberS).Value
			if pk != "USER#user1" {
				return &dynamodb.GetItemOutput{}, nil
			}
			switch sk {
			case "SHORTS#job1":
				return &dynamodb.GetItemOutput{Item: jobItemFor(jobStatus, fileType)}, nil
			case "THUMBS#job1":
				return &dynamodb.GetItemOutput{Item: e.record}, nil
			}
			return &dynamodb.GetItemOutput{}, nil
		},
		updateItemFunc: func(_ context.Context, in *dynamodb.UpdateItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
			e.updates = append(e.updates, in)
			if e.updErr != nil && len(e.updates) == 1 {
				return nil, e.updErr
			}
			return &dynamodb.UpdateItemOutput{}, nil
		},
		deleteItemFunc: func(_ context.Context, in *dynamodb.DeleteItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error) {
			e.deleted = append(e.deleted, in.Key["SK"].(*dbtypes.AttributeValueMemberS).Value)
			return &dynamodb.DeleteItemOutput{}, nil
		},
	}
	e.h = &APIHandler{
		AuthService: &mockAuthService{},
		JWTService:  &auth.JWTService{Secret: testSecret, DB: db, SessionTable: "sessions-table"},
		DB:          db,
		S3:          &stubDownloader{status: storage.ObjectStatus{StorageClass: "STANDARD"}},
		SFN: &mockSFN{startExecutionFunc: func(_ context.Context, in *sfn.StartExecutionInput, _ ...func(*sfn.Options)) (*sfn.StartExecutionOutput, error) {
			e.started = append(e.started, in)
			return &sfn.StartExecutionOutput{}, e.sfnErr
		}},
		TableName:                "table",
		Bucket:                   "test-bucket",
		ShortsStateMachineARN:    "arn:aws:states:us-east-1:123456789:stateMachine:shorts",
		ThumbnailStateMachineARN: thumbSM,
	}
	return e
}

func (e *thumbEnv) do(method, path, body, user string) events.APIGatewayProxyResponse {
	headers := map[string]string{}
	if user != "" {
		headers["Authorization"] = "Bearer " + generateTestToken(user)
	}
	resp, _ := e.h.HandleRequest(context.Background(), events.APIGatewayProxyRequest{HTTPMethod: method, Path: path, Body: body, Headers: headers})
	return resp
}

func completedRecord(selected string) map[string]dbtypes.AttributeValue {
	rec := map[string]dbtypes.AttributeValue{
		"status": sv("completed"), "style": sv("technology"), "format": sv("16:9"),
		"generation": &dbtypes.AttributeValueMemberN{Value: "2"},
		"updatedAt":  s(time.Now().UTC().Format(time.RFC3339)),
		"thumbnails": s(`[
		 {"id":"thumb-01","concept":"face","style":"technology","format":"16:9","key":"shorts/user1/job1/thumbnails/g2/thumbnail-01.jpg","headline":"5 AI TOOLS","secondaryText":"YOU NEED","frameTime":12.5,"width":1280,"height":720},
		 {"id":"thumb-02","concept":"subject","style":"technology","format":"16:9","key":"shorts/user1/job1/thumbnails/g2/thumbnail-02.jpg","headline":"5 AI TOOLS","frameTime":40,"width":1280,"height":720},
		 {"id":"thumb-03","concept":"creative","style":"technology","format":"16:9","key":"shorts/user1/job1/thumbnails/g2/thumbnail-03.jpg","headline":"5 AI TOOLS","frameTime":80,"width":1280,"height":720}]`),
		"analysis":       s(`{"topic":"AI tools","headline":"5 AI TOOLS","bestFrameTime":12.5}`),
		"funnel":         s(`{"extracted":24,"usable":20,"unique":12,"candidates":8}`),
		"transcriptUsed": &dbtypes.AttributeValueMemberBOOL{Value: true},
		"detectionUsed":  &dbtypes.AttributeValueMemberBOOL{Value: true},
	}
	if selected != "" {
		rec["selectedId"] = s(selected)
	}
	return rec
}

func decodeResp(t *testing.T, body string) thumbnailsResponse {
	t.Helper()
	var r thumbnailsResponse
	if err := json.Unmarshal([]byte(body), &r); err != nil {
		t.Fatalf("bad JSON %q: %v", body, err)
	}
	return r
}

func TestParseThumbnailPath(t *testing.T) {
	tests := []struct {
		path   string
		ok     bool
		action string
		thumb  string
	}{
		{"/shorts/job1/thumbnails", true, "", ""},
		{"/shorts/job1/thumbnails/select", true, "select", ""},
		{"/shorts/job1/thumbnails/thumb-02/download", true, "download", "thumb-02"},
		{"/shorts/job1/thumbnails/", true, "", ""},
		{"/shorts/job1/clips", false, "", ""},
		{"/shorts//thumbnails", false, "", ""},
		{"/shorts/job1/thumbnails/other", false, "", ""},
		{"/shorts/job1/thumbnails/../download", false, "", ""},
		{"/shorts/job1/thumbnails/thumb-2/download", false, "", ""},
		{"/shorts/job1/thumbnails/thumb-02/download/x", false, "", ""},
		{"/projects/job1/thumbnails", false, "", ""},
	}
	for _, tt := range tests {
		r, ok := parseThumbnailPath(tt.path)
		if ok != tt.ok || (ok && (r.action != tt.action || r.thumbID != tt.thumb)) {
			t.Errorf("%q => %+v %v, want ok=%v action=%q thumb=%q", tt.path, r, ok, tt.ok, tt.action, tt.thumb)
		}
		if ok && r.jobID != "job1" {
			t.Errorf("%q jobID = %q", tt.path, r.jobID)
		}
	}
}

func TestThumbnailEndpointsRequireAuth(t *testing.T) {
	e := newThumbEnv(t, "completed", "mp4", nil)
	for _, c := range []struct{ method, path string }{
		{"GET", "/shorts/job1/thumbnails"},
		{"POST", "/shorts/job1/thumbnails"},
		{"POST", "/shorts/job1/thumbnails/select"},
		{"GET", "/shorts/job1/thumbnails/thumb-01/download"},
	} {
		if resp := e.do(c.method, c.path, "{}", ""); resp.StatusCode != 401 {
			t.Errorf("%s %s without a token = %d, want 401", c.method, c.path, resp.StatusCode)
		}
	}
}

func TestThumbnailEndpointsAreOwnerOnly(t *testing.T) {
	e := newThumbEnv(t, "completed", "mp4", completedRecord(""))
	for _, c := range []struct{ method, path, body string }{
		{"GET", "/shorts/job1/thumbnails", ""},
		{"POST", "/shorts/job1/thumbnails", "{}"},
		{"POST", "/shorts/job1/thumbnails/select", `{"id":"thumb-01"}`},
		{"GET", "/shorts/job1/thumbnails/thumb-01/download", ""},
	} {
		if resp := e.do(c.method, c.path, c.body, "intruder"); resp.StatusCode != 404 {
			t.Errorf("%s %s as another user = %d, want 404", c.method, c.path, resp.StatusCode)
		}
	}
	if len(e.started) != 0 || len(e.updates) != 0 {
		t.Error("another user's request must change nothing")
	}
}

func TestGetThumbnailsBeforeAnythingExists(t *testing.T) {
	e := newThumbEnv(t, "transcribing", "mp4", nil)
	resp := e.do("GET", "/shorts/job1/thumbnails", "", "user1")
	if resp.StatusCode != 200 {
		t.Fatalf("status %d: %s", resp.StatusCode, resp.Body)
	}
	r := decodeResp(t, resp.Body)
	if r.Status != "none" || !r.Supported || r.JobStatus != "transcribing" || r.Thumbnails == nil || len(r.Thumbnails) != 0 {
		t.Errorf("response = %+v", r)
	}
	if !strings.Contains(resp.Body, `"thumbnails":[]`) {
		t.Errorf("thumbnails must be an empty array, not null: %s", resp.Body)
	}
}

func TestGetThumbnailsForAnAudioFile(t *testing.T) {
	e := newThumbEnv(t, "completed", "mp3", nil)
	r := decodeResp(t, e.do("GET", "/shorts/job1/thumbnails", "", "user1").Body)
	if r.Supported {
		t.Error("an audio upload has no picture to make thumbnails from")
	}
}

func TestGetThumbnailsCompleted(t *testing.T) {
	e := newThumbEnv(t, "completed", "mp4", completedRecord("thumb-02"))
	resp := e.do("GET", "/shorts/job1/thumbnails", "", "user1")
	r := decodeResp(t, resp.Body)
	if r.Status != "completed" || len(r.Thumbnails) != 3 || r.Style != "technology" || r.Format != "16:9" || r.Generation != 2 {
		t.Fatalf("response = %+v", r)
	}
	for _, th := range r.Thumbnails {
		if !strings.HasPrefix(th.URL, "https://downloads.example/shorts/user1/job1/thumbnails/g2/") {
			t.Errorf("thumbnail %s url = %q", th.ID, th.URL)
		}
		if th.Selected != (th.ID == "thumb-02") {
			t.Errorf("thumbnail %s selected = %v", th.ID, th.Selected)
		}
	}
	if r.Thumbnail == nil || r.Thumbnail.ID != "thumb-02" || r.Thumbnail.Type != "ai_generated" || !r.Thumbnail.Selected || r.Thumbnail.Style != "technology" {
		t.Errorf("selected thumbnail = %+v", r.Thumbnail)
	}
	if r.Funnel == nil || r.Funnel.Candidates != 8 || r.Analysis == nil || r.Analysis.Topic != "AI tools" || !r.TranscriptUsed {
		t.Errorf("analysis/funnel missing: %+v", r)
	}
	// S3 keys are internal; they must not be sent to the browser.
	if strings.Contains(resp.Body, `"key"`) || strings.Contains(resp.Body, "thumbnails/g2/thumbnail-01.jpg\"") && strings.Contains(resp.Body, `"key":`) {
		t.Errorf("response leaks storage keys: %s", resp.Body)
	}
}

func TestGetThumbnailsNoSelection(t *testing.T) {
	e := newThumbEnv(t, "completed", "mp4", completedRecord(""))
	r := decodeResp(t, e.do("GET", "/shorts/job1/thumbnails", "", "user1").Body)
	if r.Thumbnail != nil {
		t.Errorf("nothing selected, but thumbnail = %+v", r.Thumbnail)
	}
}

func TestGetThumbnailsReportsAbandonedRunsAsFailed(t *testing.T) {
	rec := map[string]dbtypes.AttributeValue{
		"status": sv("running"), "stage": sv("analyzing"),
		"updatedAt": s(time.Now().Add(-2 * thumbnail.StaleAfter).UTC().Format(time.RFC3339)),
	}
	e := newThumbEnv(t, "completed", "mp4", rec)
	r := decodeResp(t, e.do("GET", "/shorts/job1/thumbnails", "", "user1").Body)
	if r.Status != "failed" || r.ErrorCode != thumbnail.CodeTaskFailed || r.Error == "" || r.Stage != "" {
		t.Errorf("a run that stopped updating should read as failed with retry: %+v", r)
	}

	rec["updatedAt"] = s(time.Now().UTC().Format(time.RFC3339))
	r = decodeResp(t, e.do("GET", "/shorts/job1/thumbnails", "", "user1").Body)
	if r.Status != "running" || r.Stage != "analyzing" {
		t.Errorf("a live run keeps its status and stage: %+v", r)
	}
}

func TestGetThumbnailsFailedKeepsEarlierThumbnails(t *testing.T) {
	rec := completedRecord("")
	rec["status"] = sv("failed")
	rec["errorCode"] = sv("AI_TIMEOUT")
	rec["error"] = sv("The AI took too long to respond. Please try again.")
	e := newThumbEnv(t, "completed", "mp4", rec)
	r := decodeResp(t, e.do("GET", "/shorts/job1/thumbnails", "", "user1").Body)
	if r.Status != "failed" || r.ErrorCode != "AI_TIMEOUT" || len(r.Thumbnails) != 3 {
		t.Errorf("a failed regenerate must still show the previous set: %+v", r)
	}
}

func TestStartThumbnails(t *testing.T) {
	e := newThumbEnv(t, "completed", "mp4", nil)
	resp := e.do("POST", "/shorts/job1/thumbnails", `{"style":"news","format":"9:16"}`, "user1")
	if resp.StatusCode != 202 {
		t.Fatalf("status %d: %s", resp.StatusCode, resp.Body)
	}
	if len(e.started) != 1 {
		t.Fatalf("executions started = %d", len(e.started))
	}
	in := e.started[0]
	if *in.StateMachineArn != thumbSM || !strings.HasPrefix(*in.Name, "thumbnails-job1-") {
		t.Errorf("arn=%q name=%q", *in.StateMachineArn, *in.Name)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(*in.Input), &payload); err != nil {
		t.Fatal(err)
	}
	for k, want := range map[string]string{
		"jobId": "job1", "userId": "user1",
		"sourceKey":     "uploads/user1/job1/source.mp4",
		"transcriptKey": "shorts/user1/job1/transcript.json",
	} {
		if payload[k] != want {
			t.Errorf("input[%s] = %v, want %q", k, payload[k], want)
		}
	}
	if payload["sourceDuration"] != float64(120) {
		t.Errorf("sourceDuration = %v", payload["sourceDuration"])
	}
	// The request is recorded for the worker to read.
	vals := e.updates[0].ExpressionAttributeValues
	if vals[":style"].(*dbtypes.AttributeValueMemberS).Value != "news" || vals[":format"].(*dbtypes.AttributeValueMemberS).Value != "9:16" {
		t.Errorf("style/format not recorded: %v", vals)
	}
}

func TestStartThumbnailsWithNoBodyIsAutomatic(t *testing.T) {
	e := newThumbEnv(t, "ranking", "mov", nil)
	if resp := e.do("POST", "/shorts/job1/thumbnails", "", "user1"); resp.StatusCode != 202 {
		t.Fatalf("status %d: %s", resp.StatusCode, resp.Body)
	}
	if _, set := e.updates[0].ExpressionAttributeValues[":style"]; set {
		t.Error("no style was requested, so none should be recorded")
	}
}

func TestStartThumbnailsValidation(t *testing.T) {
	tests := []struct {
		name, body, job, ftype string
		want                   int
		code                   string
	}{
		{"bad json", `{`, "completed", "mp4", 400, "INVALID_BODY"},
		{"unknown style", `{"style":"retro"}`, "completed", "mp4", 400, "VALIDATION_ERROR"},
		{"unknown format", `{"format":"4:3"}`, "completed", "mp4", 400, "VALIDATION_ERROR"},
		{"audio file", `{}`, "completed", "mp3", 400, "NO_VIDEO_TRACK"},
		{"wav file", `{}`, "completed", "wav", 400, "NO_VIDEO_TRACK"},
		{"not started yet", `{}`, "uploaded", "mp4", 400, "INVALID_STATE"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newThumbEnv(t, tt.job, tt.ftype, nil)
			resp := e.do("POST", "/shorts/job1/thumbnails", tt.body, "user1")
			if resp.StatusCode != tt.want || !strings.Contains(resp.Body, tt.code) {
				t.Errorf("got %d %s, want %d %s", resp.StatusCode, resp.Body, tt.want, tt.code)
			}
			if len(e.started) != 0 || len(e.updates) != 0 {
				t.Error("a rejected request must not queue or start anything")
			}
		})
	}
}

func TestStartThumbnailsWhileOneIsRunning(t *testing.T) {
	e := newThumbEnv(t, "completed", "mp4", nil)
	e.updErr = &dbtypes.ConditionalCheckFailedException{}
	resp := e.do("POST", "/shorts/job1/thumbnails", `{}`, "user1")
	if resp.StatusCode != 409 || !strings.Contains(resp.Body, "IN_PROGRESS") {
		t.Errorf("got %d %s", resp.StatusCode, resp.Body)
	}
	if len(e.started) != 0 {
		t.Error("a second execution must not be started")
	}
}

func TestStartThumbnailsWhenThePipelineWillNotStart(t *testing.T) {
	e := newThumbEnv(t, "completed", "mp4", nil)
	e.sfnErr = errors.New("StateMachineDoesNotExist")
	resp := e.do("POST", "/shorts/job1/thumbnails", `{}`, "user1")
	if resp.StatusCode != 500 || !strings.Contains(resp.Body, "PIPELINE_ERROR") {
		t.Errorf("got %d %s", resp.StatusCode, resp.Body)
	}
	if strings.Contains(resp.Body, "StateMachineDoesNotExist") {
		t.Error("internal error text must not reach the client")
	}
	// The queued record is marked failed, so the page shows a retry instead
	// of spinning.
	if len(e.updates) != 2 || !strings.Contains(*e.updates[1].ConditionExpression, ":queued") ||
		e.updates[1].ExpressionAttributeValues[":code"].(*dbtypes.AttributeValueMemberS).Value != thumbnail.CodePipelineError {
		t.Errorf("expected the queued record to be aborted, updates = %d", len(e.updates))
	}
}

func TestStartThumbnailsDisabledWithoutAStateMachine(t *testing.T) {
	e := newThumbEnv(t, "completed", "mp4", nil)
	e.h.ThumbnailStateMachineARN = ""
	if resp := e.do("POST", "/shorts/job1/thumbnails", `{}`, "user1"); resp.StatusCode != 503 {
		t.Errorf("status %d, want 503", resp.StatusCode)
	}
	// Reading still works, so the page can say there is nothing yet.
	if resp := e.do("GET", "/shorts/job1/thumbnails", "", "user1"); resp.StatusCode != 200 {
		t.Errorf("GET status %d, want 200", resp.StatusCode)
	}
}

func TestSelectThumbnail(t *testing.T) {
	e := newThumbEnv(t, "completed", "mp4", completedRecord(""))
	resp := e.do("POST", "/shorts/job1/thumbnails/select", `{"id":"thumb-03"}`, "user1")
	if resp.StatusCode != 200 {
		t.Fatalf("status %d: %s", resp.StatusCode, resp.Body)
	}
	r := decodeResp(t, resp.Body)
	if r.Thumbnail == nil || r.Thumbnail.ID != "thumb-03" {
		t.Errorf("selected = %+v", r.Thumbnail)
	}
	if got := e.updates[0].ExpressionAttributeValues[":id"].(*dbtypes.AttributeValueMemberS).Value; got != "thumb-03" {
		t.Errorf("saved id = %q", got)
	}
}

func TestSelectThumbnailRejections(t *testing.T) {
	tests := []struct {
		name   string
		record map[string]dbtypes.AttributeValue
		body   string
		want   int
	}{
		{"bad body", completedRecord(""), `nope`, 400},
		{"malformed id", completedRecord(""), `{"id":"../../x"}`, 400},
		{"missing id", completedRecord(""), `{}`, 400},
		{"no thumbnails yet", nil, `{"id":"thumb-01"}`, 409},
		{"unknown thumbnail", completedRecord(""), `{"id":"thumb-09"}`, 404},
		{"still running", map[string]dbtypes.AttributeValue{"status": sv("running"), "updatedAt": s(time.Now().UTC().Format(time.RFC3339))}, `{"id":"thumb-01"}`, 409},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newThumbEnv(t, "completed", "mp4", tt.record)
			resp := e.do("POST", "/shorts/job1/thumbnails/select", tt.body, "user1")
			if resp.StatusCode != tt.want {
				t.Errorf("got %d %s, want %d", resp.StatusCode, resp.Body, tt.want)
			}
			if len(e.updates) != 0 {
				t.Error("nothing may be saved for a rejected selection")
			}
		})
	}
}

func TestSelectLosesRaceWithRegenerate(t *testing.T) {
	e := newThumbEnv(t, "completed", "mp4", completedRecord(""))
	e.updErr = &dbtypes.ConditionalCheckFailedException{}
	if resp := e.do("POST", "/shorts/job1/thumbnails/select", `{"id":"thumb-01"}`, "user1"); resp.StatusCode != 409 {
		t.Errorf("status %d, want 409", resp.StatusCode)
	}
}

func TestDownloadThumbnail(t *testing.T) {
	e := newThumbEnv(t, "completed", "mp4", completedRecord(""))
	resp := e.do("GET", "/shorts/job1/thumbnails/thumb-02/download", "", "user1")
	if resp.StatusCode != 200 {
		t.Fatalf("status %d: %s", resp.StatusCode, resp.Body)
	}
	var body map[string]string
	_ = json.Unmarshal([]byte(resp.Body), &body)
	if !strings.Contains(body["url"], "thumbnails/g2/thumbnail-02.jpg") || body["filename"] != "thumbnail-02.jpg" {
		t.Errorf("body = %v", body)
	}
	if !strings.Contains(body["url"], "filename=thumbnail-02.jpg") {
		t.Errorf("url should ask for a download named thumbnail-02.jpg: %q", body["url"])
	}

	if resp := e.do("GET", "/shorts/job1/thumbnails/thumb-09/download", "", "user1"); resp.StatusCode != 404 {
		t.Errorf("unknown thumbnail = %d, want 404", resp.StatusCode)
	}
	e.record = nil
	if resp := e.do("GET", "/shorts/job1/thumbnails/thumb-01/download", "", "user1"); resp.StatusCode != 404 {
		t.Errorf("no record = %d, want 404", resp.StatusCode)
	}
}

func TestThumbnailRoutesDoNotShadowOtherShortsRoutes(t *testing.T) {
	e := newThumbEnv(t, "completed", "mp4", nil)
	// The generic GET /shorts/{id} still returns the job itself.
	resp := e.do("GET", "/shorts/job1", "", "user1")
	if resp.StatusCode != 200 || !strings.Contains(resp.Body, `"jobId":"job1"`) {
		t.Errorf("GET /shorts/job1 = %d %s", resp.StatusCode, resp.Body)
	}
	if resp := e.do("DELETE", "/shorts/job1/thumbnails", "", "user1"); resp.StatusCode != 405 {
		t.Errorf("DELETE on thumbnails = %d, want 405", resp.StatusCode)
	}
}

func TestDeletingAJobDeletesItsThumbnailRecord(t *testing.T) {
	e := newThumbEnv(t, "completed", "mp4", completedRecord(""))
	e.h.S3 = &stubDownloader{status: storage.ObjectStatus{StorageClass: "STANDARD"}, listedKeys: []string{"shorts/user1/job1/thumbnails/g2/thumbnail-01.jpg"}}
	resp := e.do("DELETE", "/shorts/job1", "", "user1")
	if resp.StatusCode != 204 {
		t.Fatalf("status %d: %s", resp.StatusCode, resp.Body)
	}
	if len(e.deleted) != 2 || e.deleted[0] != "SHORTS#job1" || e.deleted[1] != "THUMBS#job1" {
		t.Errorf("deleted records = %v, want the job and its thumbnails", e.deleted)
	}
	swept := false
	for _, k := range e.h.S3.(*stubDownloader).deletedKeys {
		if strings.HasPrefix(k, "shorts/user1/job1/thumbnails/") {
			swept = true
		}
	}
	if !swept {
		t.Errorf("thumbnail images should be swept with the job's other assets, got %v", e.h.S3.(*stubDownloader).deletedKeys)
	}
}
