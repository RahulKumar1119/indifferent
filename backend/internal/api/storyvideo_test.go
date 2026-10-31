package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/aws-sdk-go-v2/service/sfn"
	sfntypes "github.com/aws/aws-sdk-go-v2/service/sfn/types"
	"github.com/rahul/indifferent/backend/internal/models"
	"github.com/rahul/indifferent/backend/internal/storage"
	"github.com/rahul/indifferent/backend/internal/storyjob"
	"github.com/rahul/indifferent/backend/internal/storyplan"
)

const (
	svMachineARN = "arn:aws:states:ap-south-1:123456789012:stateMachine:indifferent-fun-story-pipeline"
	// svGoogleUser is a real Google user id: numeric, not a UUID. Users who sign
	// in with Google have ids like this, so every route has to work with one.
	svGoogleUser = "114373949285061923844"
	svUUIDUser   = "0b5e4a1c-7d2f-4a8e-9c31-5f6a7b8c9d0e"
	svPlanA      = "01939f6c-8a40-7e21-b2c4-aa11bb22cc33"
	svPlanB      = "01939f6c-8a40-7e21-b2c4-aa11bb22cc44"
	svPlanC      = "01939f6c-8a40-7e21-b2c4-aa11bb22cc55"
	svNarration  = "the fisherman rows quietly across the dark water"
)

// svS3 is the storage a story video test sees: it records what is presigned and
// purged, in order with the other effects.
type svS3 struct {
	stubDownloader
	e         *svEnv
	presigned []string
	filenames []string
	statusErr error
	purgeErr  error
}

func (s *svS3) GenerateAttachmentURL(_ context.Context, _, key, filename string, _ time.Duration) (string, error) {
	s.presigned = append(s.presigned, key)
	s.filenames = append(s.filenames, filename)
	return "https://dl.example/" + key + "?filename=" + filename, nil
}

func (s *svS3) ObjectStatus(context.Context, string, string) (storage.ObjectStatus, error) {
	return storage.ObjectStatus{StorageClass: "STANDARD"}, s.statusErr
}

func (s *svS3) DeleteObjectVersions(_ context.Context, _, prefix string) error {
	s.e.log("purge:" + prefix)
	return s.purgeErr
}

// svEnv is a handler wired to an in-memory table that behaves like DynamoDB where
// the story video code depends on it: projections, key ranges, limits, and above
// all the conditional update. It fails the test on an expression it does not
// understand and on a name or value that is listed and not used (which DynamoDB
// rejects), so a change to the expressions cannot slip through.
type svEnv struct {
	t       *testing.T
	h       *APIHandler
	s3      *svS3
	now     time.Time
	mu      sync.Mutex
	items   map[string]map[string]dbtypes.AttributeValue
	events  []string
	gets    []*dynamodb.GetItemInput
	queries []*dynamodb.QueryInput
	updates []*dynamodb.UpdateItemInput
	puts    int
	starts  []*sfn.StartExecutionInput
	stops   []*sfn.StopExecutionInput

	getErr, qErr, putErr, updErr, delErr, startErr, stopErr error
	// beforeUpdate runs at the start of an UpdateItem, before its condition is
	// evaluated: the moment a worker could finish the job.
	beforeUpdate func()
}

const svWantUpdate = "SET #status = :failed, #error = :message, #errorCode = :code, #failedStage = #status, #updatedAt = :now"

func newSVEnv(t *testing.T) *svEnv {
	t.Helper()
	e := &svEnv{t: t, items: map[string]map[string]dbtypes.AttributeValue{}, now: time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)}
	e.s3 = &svS3{e: e}
	str := func(v dbtypes.AttributeValue) string { return v.(*dbtypes.AttributeValueMemberS).Value }
	key := func(m map[string]dbtypes.AttributeValue) string { return str(m["PK"]) + "|" + str(m["SK"]) }

	db := &mockDynamoDB{
		getItemFunc: func(_ context.Context, in *dynamodb.GetItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
			e.mu.Lock()
			defer e.mu.Unlock()
			e.gets = append(e.gets, in)
			if e.getErr != nil {
				return nil, e.getErr
			}
			return &dynamodb.GetItemOutput{Item: copyItem(e.items[key(in.Key)])}, nil
		},
		putItemFunc: func(_ context.Context, in *dynamodb.PutItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
			e.mu.Lock()
			defer e.mu.Unlock()
			if e.putErr != nil {
				return nil, e.putErr
			}
			k := key(in.Item)
			if aws.ToString(in.ConditionExpression) != "attribute_not_exists(PK)" {
				t.Errorf("a job is created with the condition attribute_not_exists(PK), got %q", aws.ToString(in.ConditionExpression))
			}
			if _, exists := e.items[k]; exists {
				return nil, &dbtypes.ConditionalCheckFailedException{Message: aws.String("The conditional request failed")}
			}
			e.puts++
			e.items[k] = in.Item
			e.events = append(e.events, "put")
			return &dynamodb.PutItemOutput{}, nil
		},
		queryFunc: func(_ context.Context, in *dynamodb.QueryInput, _ ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error) {
			e.mu.Lock()
			defer e.mu.Unlock()
			e.queries = append(e.queries, in)
			if e.qErr != nil {
				return nil, e.qErr
			}
			if aws.ToString(in.KeyConditionExpression) != "PK = :pk AND SK BETWEEN :lo AND :hi" {
				t.Fatalf("the test table does not understand the key condition %q", aws.ToString(in.KeyConditionExpression))
			}
			e.checkUsed(in.ExpressionAttributeNames, in.ExpressionAttributeValues, aws.ToString(in.KeyConditionExpression), aws.ToString(in.ProjectionExpression))
			pk, lo, hi := str(in.ExpressionAttributeValues[":pk"]), str(in.ExpressionAttributeValues[":lo"]), str(in.ExpressionAttributeValues[":hi"])
			var rows []map[string]dbtypes.AttributeValue
			for _, it := range e.items {
				if str(it["PK"]) == pk && str(it["SK"]) >= lo && str(it["SK"]) <= hi {
					rows = append(rows, it)
				}
			}
			sortBySK(rows)
			if in.ScanIndexForward != nil && !*in.ScanIndexForward {
				for i, j := 0, len(rows)-1; i < j; i, j = i+1, j-1 {
					rows[i], rows[j] = rows[j], rows[i]
				}
			}
			outp := &dynamodb.QueryOutput{}
			if in.Limit != nil && len(rows) > int(*in.Limit) {
				rows = rows[:*in.Limit]
				outp.LastEvaluatedKey = map[string]dbtypes.AttributeValue{"SK": rows[len(rows)-1]["SK"]}
			}
			for _, r := range rows {
				outp.Items = append(outp.Items, e.project(r, aws.ToString(in.ProjectionExpression), in.ExpressionAttributeNames))
			}
			return outp, nil
		},
		updateItemFunc: func(_ context.Context, in *dynamodb.UpdateItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
			if e.beforeUpdate != nil {
				e.beforeUpdate()
			}
			e.mu.Lock()
			defer e.mu.Unlock()
			e.updates = append(e.updates, in)
			if e.updErr != nil {
				return nil, e.updErr
			}
			if aws.ToString(in.UpdateExpression) != svWantUpdate {
				t.Fatalf("the test table does not understand the update %q", aws.ToString(in.UpdateExpression))
			}
			// The whole safety argument of a cancel: refuse a record that is gone or ended.
			if aws.ToString(in.ConditionExpression) != storyjob.NotEndedCondition {
				t.Fatalf("an update must use storyjob.NotEndedCondition, got %q", aws.ToString(in.ConditionExpression))
			}
			e.checkUsed(in.ExpressionAttributeNames, in.ExpressionAttributeValues, aws.ToString(in.UpdateExpression), aws.ToString(in.ConditionExpression))
			it := e.items[key(in.Key)]
			val := func(k string) string { return str(in.ExpressionAttributeValues[k]) }
			if it == nil || str(it["status"]) == val(":completed") || str(it["status"]) == val(":failed") {
				return nil, &dbtypes.ConditionalCheckFailedException{Message: aws.String("The conditional request failed")}
			}
			next := copyItem(it)
			next["failedStage"] = it["status"]
			next["status"] = in.ExpressionAttributeValues[":failed"]
			next["error"] = in.ExpressionAttributeValues[":message"]
			next["errorCode"] = in.ExpressionAttributeValues[":code"]
			next["updatedAt"] = in.ExpressionAttributeValues[":now"]
			e.items[key(in.Key)] = next
			e.events = append(e.events, "update")
			return &dynamodb.UpdateItemOutput{}, nil
		},
		deleteItemFunc: func(_ context.Context, in *dynamodb.DeleteItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error) {
			e.mu.Lock()
			defer e.mu.Unlock()
			if e.delErr != nil {
				return nil, e.delErr
			}
			delete(e.items, key(in.Key))
			e.events = append(e.events, "delete")
			return &dynamodb.DeleteItemOutput{}, nil
		},
	}
	h := newShortsTestHandler(t, db)
	h.S3 = e.s3
	h.SFN = &mockSFN{
		startExecutionFunc: func(_ context.Context, in *sfn.StartExecutionInput, _ ...func(*sfn.Options)) (*sfn.StartExecutionOutput, error) {
			e.mu.Lock()
			defer e.mu.Unlock()
			e.starts = append(e.starts, in)
			e.events = append(e.events, "start")
			if e.startErr != nil {
				return nil, e.startErr
			}
			return &sfn.StartExecutionOutput{}, nil
		},
		stopExecutionFunc: func(_ context.Context, in *sfn.StopExecutionInput, _ ...func(*sfn.Options)) (*sfn.StopExecutionOutput, error) {
			e.mu.Lock()
			defer e.mu.Unlock()
			e.stops = append(e.stops, in)
			e.events = append(e.events, "stop")
			if e.stopErr != nil {
				return nil, e.stopErr
			}
			return &sfn.StopExecutionOutput{}, nil
		},
	}
	h.StoryVideoStateMachineARN = svMachineARN
	h.StoryVideoNow = func() time.Time { return e.now }
	h.Bucket = "test-bucket"
	e.h = h
	return e
}

func (e *svEnv) log(ev string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.events = append(e.events, ev)
}

// checkUsed fails the test if a placeholder is defined and not used, or used and
// not defined. DynamoDB refuses both.
func (e *svEnv) checkUsed(names map[string]string, values map[string]dbtypes.AttributeValue, exprs ...string) {
	e.t.Helper()
	text := strings.Join(exprs, " ")
	for n := range names {
		if !strings.Contains(text, n) {
			e.t.Errorf("name %s is listed but not used", n)
		}
	}
	for v := range values {
		if !strings.Contains(text, v) {
			e.t.Errorf("value %s is listed but not used", v)
		}
	}
}

// project keeps the attributes a projection expression names, as DynamoDB does.
func (e *svEnv) project(item map[string]dbtypes.AttributeValue, expr string, names map[string]string) map[string]dbtypes.AttributeValue {
	if expr == "" {
		return copyItem(item)
	}
	out := map[string]dbtypes.AttributeValue{}
	for _, tok := range strings.Split(expr, ",") {
		tok = strings.TrimSpace(tok)
		if strings.HasPrefix(tok, "#") {
			real, ok := names[tok]
			if !ok {
				e.t.Fatalf("projection uses %s, which is not defined", tok)
			}
			tok = real
		}
		if v, ok := item[tok]; ok {
			out[tok] = v
		}
	}
	return out
}

func (e *svEnv) resolvedProjection(in *dynamodb.QueryInput) []string {
	var out []string
	for _, tok := range strings.Split(aws.ToString(in.ProjectionExpression), ",") {
		tok = strings.TrimSpace(tok)
		if strings.HasPrefix(tok, "#") {
			tok = in.ExpressionAttributeNames[tok]
		}
		out = append(out, tok)
	}
	return out
}

func (e *svEnv) do(method, path, body, user string) events.APIGatewayProxyResponse {
	e.t.Helper()
	return e.doQuery(method, path, body, user, nil)
}

func (e *svEnv) doQuery(method, path, body, user string, query map[string]string) events.APIGatewayProxyResponse {
	e.t.Helper()
	headers := map[string]string{}
	if user != "" {
		headers["Authorization"] = "Bearer " + generateTestToken(user)
	}
	resp, err := e.h.HandleRequest(context.Background(), events.APIGatewayProxyRequest{
		HTTPMethod: method, Path: path, Headers: headers, Body: body, QueryStringParameters: query,
	})
	if err != nil {
		e.t.Fatalf("HandleRequest: %v", err)
	}
	return resp
}

// planWithPictures is a plan every scene of which has a picture chosen.
func planWithPictures(lang string) storyplan.Plan {
	p := samplePlan(lang)
	for i := range p.Scenes {
		p.Scenes[i].Narration = svNarration
		if lang == "hi" {
			p.Scenes[i].Narration = "मछुआरा चुपचाप अँधेरे पानी में नाव खेता है"
		}
		p.Scenes[i].Media = &storyplan.MediaRef{Provider: "pixabay", Type: "photo", ID: strconv.Itoa(100 + i)}
	}
	return p
}

func (e *svEnv) seedPlan(user, planID string, rev int, status string, plan *storyplan.Plan) {
	e.t.Helper()
	item, err := storyPlanToItem(storyPlanRecord{
		UserID: user, PlanID: planID, Status: status, Model: "m", Revision: rev, Plan: plan,
		CreatedAt: e.now.Format(time.RFC3339), UpdatedAt: e.now.Format(time.RFC3339),
	})
	if err != nil {
		e.t.Fatal(err)
	}
	e.items[str0(item["PK"])+"|"+str0(item["SK"])] = item
}

func str0(v dbtypes.AttributeValue) string { return v.(*dbtypes.AttributeValueMemberS).Value }

// seedJob stores a job made at time at, in the given status, and returns its id.
func (e *svEnv) seedJob(user, planID, status string, at time.Time, edit ...func(*storyjob.Job)) string {
	e.t.Helper()
	jobID, err := newStoryPlanID(at)
	if err != nil {
		e.t.Fatal(err)
	}
	plan := planWithPictures("en")
	j, err := storyjob.NewFromPlan(user, jobID, planID, 1, plan, "", models.CaptionStyle{}, at)
	if err != nil {
		e.t.Fatal(err)
	}
	j.Status = status
	j.Progress = storyjob.StageProgress[status]
	switch status {
	case storyjob.StatusCompleted:
		j.OutputKey = storyjob.OutputKey(user, jobID)
		j.CompletedAt = at.Format(time.RFC3339)
		j.TotalSeconds = 12.5
	case storyjob.StatusFailed:
		j.Error, j.ErrorCode, j.FailedStage = "Scene 2: the picture is no longer available from Pixabay. Choose another and make the video again.", "MEDIA_UNAVAILABLE", storyjob.StatusPreparing
	}
	for _, f := range edit {
		f(&j)
	}
	item, err := storyjob.ToItem(j)
	if err != nil {
		e.t.Fatal(err)
	}
	e.items[str0(item["PK"])+"|"+str0(item["SK"])] = item
	return jobID
}

func (e *svEnv) stored(user, jobID string) *storyjob.Job {
	e.t.Helper()
	it := e.items["USER#"+user+"|STORYVIDEO#"+jobID]
	if it == nil {
		return nil
	}
	j, err := storyjob.FromItem(it)
	if err != nil {
		e.t.Fatal(err)
	}
	return &j
}

func (e *svEnv) countJobs(user string) int {
	n := 0
	for _, it := range e.items {
		if str0(it["PK"]) == "USER#"+user && strings.HasPrefix(str0(it["SK"]), "STORYVIDEO#") {
			n++
		}
	}
	return n
}

// assertNothingDone checks that a refused request changed and started nothing.
func (e *svEnv) assertNothingDone() {
	e.t.Helper()
	if e.puts != 0 || len(e.starts) != 0 || len(e.stops) != 0 || len(e.updates) != 0 {
		e.t.Errorf("a refused request did something: puts=%d starts=%d stops=%d updates=%d", e.puts, len(e.starts), len(e.stops), len(e.updates))
	}
}

func createBody(planID string, rev int, extra string) string {
	return fmt.Sprintf(`{"planId":%q,"revision":%d%s}`, planID, rev, extra)
}

func decodeVideo(t *testing.T, resp events.APIGatewayProxyResponse) StoryVideoResponse {
	t.Helper()
	var v StoryVideoResponse
	if err := json.Unmarshal([]byte(resp.Body), &v); err != nil {
		t.Fatalf("not a video: %s", resp.Body)
	}
	return v
}

// ---------------------------------------------------------------------------

func TestStoryVideos_EveryRouteNeedsSignIn(t *testing.T) {
	e := newSVEnv(t)
	for _, rt := range [][2]string{
		{"POST", "/story-videos"}, {"GET", "/story-videos"},
		{"GET", "/story-videos/" + svPlanA}, {"DELETE", "/story-videos/" + svPlanA},
		{"POST", "/story-videos/" + svPlanA + "/cancel"}, {"GET", "/story-videos/" + svPlanA + "/download"},
	} {
		if resp := e.do(rt[0], rt[1], "{}", ""); resp.StatusCode != 401 {
			t.Errorf("%s %s without a token: %d", rt[0], rt[1], resp.StatusCode)
		}
	}
	if len(e.events) != 0 || len(e.gets) != 0 {
		t.Error("an unauthenticated request reached the database or the pipeline")
	}
	// Only the limits are public.
	if resp := e.do("GET", "/story-videos/limits", "", ""); resp.StatusCode != 200 {
		t.Errorf("limits: %d", resp.StatusCode)
	}
}

func TestStoryVideos_Limits(t *testing.T) {
	e := newSVEnv(t)
	var l StoryVideoLimits
	if err := json.Unmarshal([]byte(e.do("GET", "/story-videos/limits", "", "").Body), &l); err != nil {
		t.Fatal(err)
	}
	if !l.Enabled || l.MaxActive != 2 || l.PerDay != 10 || l.MaxSeconds != 90 {
		t.Errorf("limits = %+v", l)
	}
	if got := l.Voices["en"]; len(got) != 5 || got[0] != "Joanna" {
		t.Errorf("english voices = %v", got)
	}
	if got := l.Voices["hi"]; len(got) != 1 || got[0] != "Aditi" {
		t.Errorf("hindi voices = %v", got)
	}
	e.h.StoryVideoStateMachineARN = ""
	if err := json.Unmarshal([]byte(e.do("GET", "/story-videos/limits", "", "").Body), &l); err != nil || l.Enabled {
		t.Errorf("without a state machine the page is told it is off: %+v", l)
	}
	e.h.StoryVideosPerDay = 100000
	_ = json.Unmarshal([]byte(e.do("GET", "/story-videos/limits", "", "").Body), &l)
	if l.PerDay != storyVideoMaxPerDay {
		t.Errorf("a huge configured allowance is %d, want it bounded at %d", l.PerDay, storyVideoMaxPerDay)
	}
}

func TestStoryVideos_RoutesThatAreNotRoutesAre404(t *testing.T) {
	e := newSVEnv(t)
	id := svPlanA
	for _, rt := range [][2]string{
		{"GET", "/story-videos/"}, {"PUT", "/story-videos/" + id}, {"PATCH", "/story-videos"},
		{"GET", "/story-videos/" + id + "/cancel"}, {"POST", "/story-videos/" + id + "/download"},
		{"POST", "/story-videos/" + id + "/other"}, {"POST", "/story-videos/" + id + "/cancel/extra"},
		{"GET", "/story-videos/not-an-id"}, {"POST", "/story-videos/limits"}, {"DELETE", "/story-videos"},
		{"GET", "/story-videos/..%2Fx"}, {"GET", "/story-videos/" + id + "x"},
	} {
		before := len(e.gets)
		if resp := e.do(rt[0], rt[1], "{}", svGoogleUser); resp.StatusCode != 404 {
			t.Errorf("%s %s: %d, want 404", rt[0], rt[1], resp.StatusCode)
		}
		if len(e.gets) != before {
			t.Errorf("%s %s reached the database", rt[0], rt[1])
		}
	}
	e.assertNothingDone()
}

func TestStoryVideoRoute(t *testing.T) {
	for path, want := range map[string][3]any{
		"/story-videos":              {"", "", true},
		"/story-videos/abc":          {"abc", "", true},
		"/story-videos/abc/cancel":   {"abc", "cancel", true},
		"/story-videos/abc/download": {"abc", "download", true},
		"/story-videos/":             {"", "", false},
		"/story-videos/abc/x":        {"abc", "x", false},
		"/story-videos/a/cancel/b":   {"", "", false},
		"/story-videos//cancel":      {"", "cancel", false},
		"/story-plans/abc":           {"", "", false},
	} {
		id, action, ok := storyVideoRoute(path)
		if id != want[0] || action != want[1] || ok != want[2] {
			t.Errorf("%s = %q %q %v, want %v", path, id, action, ok, want)
		}
	}
}

func TestStoryVideos_AUserIdThatCouldBuildAnUnsafeKeyIsRefusedBeforeAnything(t *testing.T) {
	e := newSVEnv(t)
	for _, user := range []string{"a/b", "../other", "a.b", "x#y"} {
		for _, rt := range [][2]string{{"GET", "/story-videos"}, {"DELETE", "/story-videos/" + svPlanA}, {"POST", "/story-videos"}} {
			if resp := e.do(rt[0], rt[1], createBody(svPlanA, 1, ""), user); resp.StatusCode != 400 {
				t.Errorf("user %q, %s %s: %d", user, rt[0], rt[1], resp.StatusCode)
			}
		}
	}
	if len(e.gets)+len(e.queries) != 0 || len(e.events) != 0 {
		t.Error("an unsafe user id reached storage")
	}
}

// ---------------------------------------------------------------------------
// Create

func TestStoryVideos_CreateWritesTheJobStartsTheExecutionAndReturns202(t *testing.T) {
	e := newSVEnv(t)
	plan := planWithPictures("en")
	e.seedPlan(svGoogleUser, svPlanA, 3, storyPlanReady, &plan)

	resp := e.do("POST", "/story-videos", createBody(svPlanA, 3, ""), svGoogleUser)
	if resp.StatusCode != 202 {
		t.Fatalf("status %d: %s", resp.StatusCode, resp.Body)
	}
	v := decodeVideo(t, resp)
	if !storyjob.ValidJobID(v.JobID) || v.PlanID != svPlanA || v.PlanRevision != 3 || v.Status != "queued" || v.Progress != 0 || v.Voice != "Joanna" || v.Language != "en" {
		t.Errorf("view = %+v", v)
	}
	if v.CreatedAt != e.now.Format(time.RFC3339) || v.Error != "" {
		t.Errorf("view = %+v", v)
	}

	// The record: the plan is copied, the pictures are in it, nothing else is.
	j := e.stored(svGoogleUser, v.JobID)
	if j == nil || j.Status != "queued" || len(j.Scenes) != 2 || j.Scenes[1].Pick.ID != "101" || j.Scenes[0].Narration != svNarration {
		t.Fatalf("stored = %+v", j)
	}

	// The execution: one, named after the job, with exactly the two ids.
	if len(e.starts) != 1 {
		t.Fatalf("%d executions started", len(e.starts))
	}
	st := e.starts[0]
	if aws.ToString(st.StateMachineArn) != svMachineARN || aws.ToString(st.Name) != v.JobID {
		t.Errorf("execution %q of %q", aws.ToString(st.Name), aws.ToString(st.StateMachineArn))
	}
	var in map[string]string
	if err := json.Unmarshal([]byte(aws.ToString(st.Input)), &in); err != nil || len(in) != 2 || in["userId"] != svGoogleUser || in["jobId"] != v.JobID {
		t.Errorf("input = %s", aws.ToString(st.Input))
	}
	// What the pipeline sends the worker must be accepted by the worker. A Google
	// id is numeric; a worker that wanted a UUID refused every such user.
	if _, err := storyjob.ParseWorkerInput(in["userId"], in["jobId"]); err != nil {
		t.Errorf("the worker would refuse this job: %v", err)
	}
	if got := strings.Join(e.events, ","); got != "put,start" {
		t.Errorf("events = %s: the record must exist before the pipeline starts", got)
	}
	// The response is a view: no storage key, no scene text.
	for _, leak := range []string{"storyvideo", "outputKey", "scenes", "rows quietly", "pixabay", "\"pick\""} {
		if strings.Contains(resp.Body, leak) {
			t.Errorf("the response contains %q: %s", leak, resp.Body)
		}
	}
}

func TestStoryVideos_AUserWithAUUIDIdWorksToo(t *testing.T) {
	e := newSVEnv(t)
	plan := planWithPictures("en")
	e.seedPlan(svUUIDUser, svPlanA, 1, storyPlanReady, &plan)
	if resp := e.do("POST", "/story-videos", createBody(svPlanA, 1, ""), svUUIDUser); resp.StatusCode != 202 {
		t.Fatalf("%d: %s", resp.StatusCode, resp.Body)
	}
}

func TestStoryVideos_VoiceAndCaption(t *testing.T) {
	e := newSVEnv(t)
	en, hi := planWithPictures("en"), planWithPictures("hi")
	e.seedPlan(svGoogleUser, svPlanA, 1, storyPlanReady, &en)
	e.seedPlan(svGoogleUser, svPlanB, 1, storyPlanReady, &hi)

	if v := decodeVideo(t, e.do("POST", "/story-videos", createBody(svPlanB, 1, ""), svGoogleUser)); v.Voice != "Aditi" || v.Language != "hi" {
		t.Errorf("a Hindi story's default voice is %q", v.Voice)
	}
	ok := e.do("POST", "/story-videos", createBody(svPlanA, 1, `,"voice":"Matthew","caption":{"preset":"karaoke","fontSize":40,"color":"#FFFFFF","position":"middle"}`), svGoogleUser)
	if ok.StatusCode != 202 {
		t.Fatalf("%d: %s", ok.StatusCode, ok.Body)
	}
	v := decodeVideo(t, ok)
	if v.Voice != "Matthew" || v.Caption.Preset != "karaoke" || v.Caption.FontSize != 40 || v.Caption.Position != "middle" {
		t.Errorf("view = %+v", v)
	}
	if j := e.stored(svGoogleUser, v.JobID); j.Caption != v.Caption || j.Voice != "Matthew" {
		t.Errorf("stored = %+v", j)
	}
	before := e.puts

	for name, tc := range map[string]struct{ plan, extra, want string }{
		"an english voice for a hindi story": {svPlanB, `,"voice":"Joanna"`, "choose Aditi"},
		"a voice that does not exist":        {svPlanA, `,"voice":"Zoltan"`, "cannot narrate"},
		"an unknown caption preset":          {svPlanA, `,"caption":{"preset":"neon"}`, "unknown caption preset"},
		"a caption colour that is not one":   {svPlanA, `,"caption":{"color":"red"}`, "#RRGGBB"},
		"a font size out of range":           {svPlanA, `,"caption":{"fontSize":500}`, "out of range"},
	} {
		resp := e.do("POST", "/story-videos", createBody(tc.plan, 1, tc.extra), svGoogleUser)
		if resp.StatusCode != 400 || !strings.Contains(decodeErr(t, resp).Message, tc.want) {
			t.Errorf("%s: %d %s", name, resp.StatusCode, resp.Body)
		}
	}
	if e.puts != before {
		t.Error("a refused request made a job")
	}
}

func TestStoryVideos_CreateRefusesWhatIsNotRenderableAndDoesNothing(t *testing.T) {
	missing := planWithPictures("en")
	missing.Scenes[1].Media = nil
	french := planWithPictures("fr")
	cases := map[string]struct {
		seed    func(e *svEnv)
		body    string
		user    string
		status  int
		code    string
		contain string
	}{
		"a plan that does not exist": {func(e *svEnv) {}, createBody(svPlanA, 1, ""), svGoogleUser, 404, "NOT_FOUND", ""},
		"an id that is not a UUID":   {func(e *svEnv) {}, createBody("../x", 1, ""), svGoogleUser, 404, "NOT_FOUND", ""},
		"no plan id":                 {func(e *svEnv) {}, `{"revision":1}`, svGoogleUser, 404, "NOT_FOUND", ""},
		"somebody else's plan": {func(e *svEnv) {
			p := planWithPictures("en")
			e.seedPlan(svUUIDUser, svPlanA, 1, storyPlanReady, &p)
		}, createBody(svPlanA, 1, ""), svGoogleUser, 404, "NOT_FOUND", ""},
		"a plan that is still pending": {func(e *svEnv) { e.seedPlan(svGoogleUser, svPlanA, 0, storyPlanPending, nil) }, createBody(svPlanA, 0, ""), svGoogleUser, 409, "NOT_READY", ""},
		"a plan that failed":           {func(e *svEnv) { e.seedPlan(svGoogleUser, svPlanA, 0, storyPlanFailed, nil) }, createBody(svPlanA, 0, ""), svGoogleUser, 409, "NOT_READY", ""},
		// Only a plan the planner finished is rendered, whatever else the record holds.
		"a failed plan that still has scenes": {func(e *svEnv) {
			p := planWithPictures("en")
			e.seedPlan(svGoogleUser, svPlanA, 1, storyPlanFailed, &p)
		}, createBody(svPlanA, 1, ""), svGoogleUser, 409, "NOT_READY", ""},
		"a pending plan that already has scenes": {func(e *svEnv) {
			p := planWithPictures("en")
			e.seedPlan(svGoogleUser, svPlanA, 1, storyPlanPending, &p)
		}, createBody(svPlanA, 1, ""), svGoogleUser, 409, "NOT_READY", ""},
		"a plan changed elsewhere": {func(e *svEnv) {
			p := planWithPictures("en")
			e.seedPlan(svGoogleUser, svPlanA, 3, storyPlanReady, &p)
		}, createBody(svPlanA, 2, ""), svGoogleUser, 409, "STALE_REVISION", ""},
		"a revision left out": {func(e *svEnv) {
			p := planWithPictures("en")
			e.seedPlan(svGoogleUser, svPlanA, 3, storyPlanReady, &p)
		}, `{"planId":"` + svPlanA + `"}`, svGoogleUser, 409, "STALE_REVISION", ""},
		"a scene without a picture":          {func(e *svEnv) { e.seedPlan(svGoogleUser, svPlanA, 1, storyPlanReady, &missing) }, createBody(svPlanA, 1, ""), svGoogleUser, 400, "MISSING_PICTURES", "Scene 2 has no picture"},
		"a language that cannot be narrated": {func(e *svEnv) { e.seedPlan(svGoogleUser, svPlanA, 1, storyPlanReady, &french) }, createBody(svPlanA, 1, ""), svGoogleUser, 400, "UNSUPPORTED_LANGUAGE", "English and Hindi"},
		"a body that is not JSON":            {func(e *svEnv) {}, `{"planId":`, svGoogleUser, 400, "INVALID_BODY", ""},
		"a body that is too large":           {func(e *svEnv) {}, createBody(svPlanA, 1, `,"voice":"`+strings.Repeat("x", storyVideoMaxBodyBytes)+`"`), svGoogleUser, 400, "VALIDATION_ERROR", "too large"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := newSVEnv(t)
			tc.seed(e)
			resp := e.do("POST", "/story-videos", tc.body, tc.user)
			if resp.StatusCode != tc.status || decodeErr(t, resp).Code != tc.code {
				t.Fatalf("%d %s", resp.StatusCode, resp.Body)
			}
			if tc.contain != "" && !strings.Contains(resp.Body, tc.contain) {
				t.Errorf("body %s lacks %q", resp.Body, tc.contain)
			}
			e.assertNothingDone()
			if n := e.countJobs(tc.user); n != 0 {
				t.Errorf("%d jobs were made", n)
			}
		})
	}
}

func TestStoryVideos_CreateWithoutAStateMachineAnswers503AndTouchesNothing(t *testing.T) {
	e := newSVEnv(t)
	plan := planWithPictures("en")
	e.seedPlan(svGoogleUser, svPlanA, 1, storyPlanReady, &plan)
	e.h.StoryVideoStateMachineARN = ""
	resp := e.do("POST", "/story-videos", createBody(svPlanA, 1, ""), svGoogleUser)
	if resp.StatusCode != 503 || decodeErr(t, resp).Code != "UNAVAILABLE" {
		t.Errorf("%d %s", resp.StatusCode, resp.Body)
	}
	e.assertNothingDone()
	if len(e.gets) != 0 {
		t.Error("it read the database for a feature that is off")
	}
	// The rest of the API does not depend on it.
	if resp := e.do("GET", "/story-videos", "", svGoogleUser); resp.StatusCode != 200 {
		t.Errorf("list with the pipeline off: %d", resp.StatusCode)
	}
}

func TestStoryVideos_AStartThatFailsDoesNotLeaveAJobQueuedForEver(t *testing.T) {
	e := newSVEnv(t)
	plan := planWithPictures("en")
	e.seedPlan(svGoogleUser, svPlanA, 1, storyPlanReady, &plan)
	e.startErr = errors.New("states: throttled")

	resp := e.do("POST", "/story-videos", createBody(svPlanA, 1, ""), svGoogleUser)
	if resp.StatusCode != 500 || decodeErr(t, resp).Code != "PIPELINE_ERROR" {
		t.Fatalf("%d %s", resp.StatusCode, resp.Body)
	}
	if strings.Contains(resp.Body, "throttled") {
		t.Error("the error from Step Functions reached the user")
	}
	if got := strings.Join(e.events, ","); got != "put,start,update" {
		t.Errorf("events = %s", got)
	}
	var jobs []*storyjob.Job
	for k := range e.items {
		if strings.Contains(k, "STORYVIDEO#") {
			id := k[strings.Index(k, "STORYVIDEO#")+len("STORYVIDEO#"):]
			jobs = append(jobs, e.stored(svGoogleUser, id))
		}
	}
	if len(jobs) != 1 || jobs[0].Status != "failed" || jobs[0].ErrorCode != "START_FAILED" || jobs[0].Error != storyVideoStartFailedMessage || jobs[0].FailedStage != "queued" {
		t.Fatalf("jobs = %+v", jobs)
	}
	// A job that did not start is not counted as being made, so the user can try again at once.
	e.startErr = nil
	if resp := e.do("POST", "/story-videos", createBody(svPlanA, 1, ""), svGoogleUser); resp.StatusCode != 202 {
		t.Errorf("the retry: %d %s", resp.StatusCode, resp.Body)
	}
}

func TestStoryVideos_AStartThatFailsAndCannotBeMarkedStillAnswersTheError(t *testing.T) {
	e := newSVEnv(t)
	plan := planWithPictures("en")
	e.seedPlan(svGoogleUser, svPlanA, 1, storyPlanReady, &plan)
	e.startErr, e.updErr = errors.New("states down"), errors.New("dynamo down")
	if resp := e.do("POST", "/story-videos", createBody(svPlanA, 1, ""), svGoogleUser); resp.StatusCode != 500 {
		t.Errorf("%d %s", resp.StatusCode, resp.Body)
	}
}

func TestStoryVideos_AnExecutionThatAlreadyExistsIsNotAnError(t *testing.T) {
	e := newSVEnv(t)
	plan := planWithPictures("en")
	e.seedPlan(svGoogleUser, svPlanA, 1, storyPlanReady, &plan)
	e.startErr = &sfntypes.ExecutionAlreadyExists{}
	if resp := e.do("POST", "/story-videos", createBody(svPlanA, 1, ""), svGoogleUser); resp.StatusCode != 202 {
		t.Errorf("%d %s", resp.StatusCode, resp.Body)
	}
}

func TestStoryVideos_AWriteThatFailsStartsNothing(t *testing.T) {
	e := newSVEnv(t)
	plan := planWithPictures("en")
	e.seedPlan(svGoogleUser, svPlanA, 1, storyPlanReady, &plan)
	e.putErr = errors.New("dynamo down")
	if resp := e.do("POST", "/story-videos", createBody(svPlanA, 1, ""), svGoogleUser); resp.StatusCode != 500 {
		t.Fatalf("%d", resp.StatusCode)
	}
	if len(e.starts) != 0 {
		t.Error("a pipeline was started for a job that was never written")
	}
}

// ---------------------------------------------------------------------------
// Limits

func TestStoryVideos_TheLimitsQueryIsABoundedConsistentKeyRange(t *testing.T) {
	e := newSVEnv(t)
	plan := planWithPictures("en")
	e.seedPlan(svGoogleUser, svPlanA, 1, storyPlanReady, &plan)
	e.do("POST", "/story-videos", createBody(svPlanA, 1, ""), svGoogleUser)
	if len(e.queries) != 1 {
		t.Fatalf("%d queries", len(e.queries))
	}
	q := e.queries[0]
	val := func(k string) string { return str0(q.ExpressionAttributeValues[k]) }
	if val(":pk") != "USER#"+svGoogleUser || val(":lo") != "STORYVIDEO#"+uuidV7Floor(e.now.Add(-24*time.Hour)) || val(":hi") != "STORYVIDEO#~" {
		t.Errorf("range = %s .. %s for %s", val(":lo"), val(":hi"), val(":pk"))
	}
	if q.Limit == nil || *q.Limit != 10 || !aws.ToBool(q.ConsistentRead) || q.ScanIndexForward == nil || !*q.ScanIndexForward {
		t.Errorf("limit %v consistent %v forward %v", q.Limit, q.ConsistentRead, q.ScanIndexForward)
	}
	for _, a := range e.resolvedProjection(q) {
		if a == "scenes" || a == "outputKey" {
			t.Errorf("the limit check reads %s", a)
		}
	}
}

func TestStoryVideos_ACheckThatCannotReadStopsBeforeMakingAnything(t *testing.T) {
	e := newSVEnv(t)
	plan := planWithPictures("en")
	e.seedPlan(svGoogleUser, svPlanA, 1, storyPlanReady, &plan)
	e.qErr = errors.New("dynamo down")
	if resp := e.do("POST", "/story-videos", createBody(svPlanA, 1, ""), svGoogleUser); resp.StatusCode != 500 {
		t.Fatalf("%d", resp.StatusCode)
	}
	e.assertNothingDone()
}

func TestStoryVideos_OnlyOneVideoAtATimePerPlan(t *testing.T) {
	e := newSVEnv(t)
	plan := planWithPictures("en")
	e.seedPlan(svGoogleUser, svPlanA, 1, storyPlanReady, &plan)
	old := e.seedJob(svGoogleUser, svPlanA, "rendering", e.now.Add(-10*time.Minute))
	newer := e.seedJob(svGoogleUser, svPlanA, "queued", e.now.Add(-1*time.Minute))

	resp := e.do("POST", "/story-videos", createBody(svPlanA, 1, ""), svGoogleUser)
	if resp.StatusCode != 409 || decodeErr(t, resp).Code != "ALREADY_RUNNING" {
		t.Fatalf("%d %s", resp.StatusCode, resp.Body)
	}
	var body struct {
		Details struct {
			JobID string `json:"jobId"`
		} `json:"details"`
	}
	_ = json.Unmarshal([]byte(resp.Body), &body)
	if body.Details.JobID != newer || body.Details.JobID == old {
		t.Errorf("it points at %q, want the latest, %q", body.Details.JobID, newer)
	}
	e.assertNothingDone()
}

func TestStoryVideos_AnEndedOrAbandonedVideoDoesNotBlockTheSamePlan(t *testing.T) {
	for name, seed := range map[string]func(e *svEnv){
		"a completed one": func(e *svEnv) { e.seedJob(svGoogleUser, svPlanA, "completed", e.now.Add(-time.Hour)) },
		"a failed one":    func(e *svEnv) { e.seedJob(svGoogleUser, svPlanA, "failed", e.now.Add(-time.Hour)) },
		"one nobody has touched for three hours": func(e *svEnv) {
			e.seedJob(svGoogleUser, svPlanA, "rendering", e.now.Add(-3*time.Hour))
		},
		"one whose times cannot be read": func(e *svEnv) {
			e.seedJob(svGoogleUser, svPlanA, "rendering", e.now.Add(-time.Minute), func(j *storyjob.Job) { j.CreatedAt, j.UpdatedAt = "garbage", "garbage" })
		},
		"somebody else's": func(e *svEnv) { e.seedJob(svUUIDUser, svPlanA, "rendering", e.now.Add(-time.Minute)) },
	} {
		t.Run(name, func(t *testing.T) {
			e := newSVEnv(t)
			plan := planWithPictures("en")
			e.seedPlan(svGoogleUser, svPlanA, 1, storyPlanReady, &plan)
			seed(e)
			if resp := e.do("POST", "/story-videos", createBody(svPlanA, 1, ""), svGoogleUser); resp.StatusCode != 202 {
				t.Errorf("%d %s", resp.StatusCode, resp.Body)
			}
		})
	}
}

func TestStoryVideos_NoMoreThanTwoAtOnce(t *testing.T) {
	e := newSVEnv(t)
	plan := planWithPictures("en")
	e.seedPlan(svGoogleUser, svPlanA, 1, storyPlanReady, &plan)
	e.seedJob(svGoogleUser, svPlanB, "narrating", e.now.Add(-5*time.Minute))
	if resp := e.do("POST", "/story-videos", createBody(svPlanA, 1, ""), svGoogleUser); resp.StatusCode != 202 {
		t.Fatalf("one running, a second is allowed: %d %s", resp.StatusCode, resp.Body)
	}
	// Two now run (the seeded one and the one just made). A third is refused.
	e.seedPlan(svGoogleUser, svPlanC, 1, storyPlanReady, &plan)
	e.puts, e.starts = 0, nil
	resp := e.do("POST", "/story-videos", createBody(svPlanC, 1, ""), svGoogleUser)
	if resp.StatusCode != 409 || decodeErr(t, resp).Code != "TOO_MANY_ACTIVE" || !strings.Contains(resp.Body, "2 videos") {
		t.Fatalf("%d %s", resp.StatusCode, resp.Body)
	}
	e.assertNothingDone()
	// The limit is per user.
	other := planWithPictures("en")
	e.seedPlan(svUUIDUser, svPlanC, 1, storyPlanReady, &other)
	if resp := e.do("POST", "/story-videos", createBody(svPlanC, 1, ""), svUUIDUser); resp.StatusCode != 202 {
		t.Errorf("another user: %d %s", resp.StatusCode, resp.Body)
	}
	// A video that ends frees its place.
	for k, it := range e.items {
		if strings.HasPrefix(k, "USER#"+svGoogleUser) && strings.Contains(k, "STORYVIDEO#") && str0(it["status"]) == "narrating" {
			it["status"] = &dbtypes.AttributeValueMemberS{Value: "completed"}
		}
	}
	if resp := e.do("POST", "/story-videos", createBody(svPlanC, 1, ""), svGoogleUser); resp.StatusCode != 202 {
		t.Errorf("after one ended: %d %s", resp.StatusCode, resp.Body)
	}
}

func TestStoryVideos_ADayAllowanceThatCountsEverythingStartedAndSaysWhenToComeBack(t *testing.T) {
	e := newSVEnv(t)
	e.h.StoryVideosPerDay = 3
	e.h.StoryVideoMaxActive = 10
	plan := planWithPictures("en")
	e.seedPlan(svGoogleUser, svPlanA, 1, storyPlanReady, &plan)
	// Cancelled and failed ones count: they used Polly and a task.
	e.seedJob(svGoogleUser, svPlanB, "failed", e.now.Add(-23*time.Hour), func(j *storyjob.Job) { j.ErrorCode = "CANCELLED" })
	e.seedJob(svGoogleUser, svPlanB, "completed", e.now.Add(-5*time.Hour))
	// Older than a day: not counted.
	e.seedJob(svGoogleUser, svPlanB, "completed", e.now.Add(-25*time.Hour))

	if resp := e.do("POST", "/story-videos", createBody(svPlanA, 1, ""), svGoogleUser); resp.StatusCode != 202 {
		t.Fatalf("two in the window, a third is allowed: %d %s", resp.StatusCode, resp.Body)
	}
	// Three in the window now. End the one just made, so that only the day
	// allowance can refuse: the oldest leaves the window in an hour.
	e.cancelRunning(svGoogleUser)
	e.puts, e.starts = 0, nil
	resp := e.do("POST", "/story-videos", createBody(svPlanA, 1, ""), svGoogleUser)
	if resp.StatusCode != 429 || decodeErr(t, resp).Code != "RATE_LIMITED" {
		t.Fatalf("%d %s", resp.StatusCode, resp.Body)
	}
	if got := bodyRetryAfter(t, resp); got != "3600" {
		t.Errorf("retryAfter in the body = %s, want 3600 (the oldest leaves the window in an hour)", got)
	}
	if resp.Headers["Retry-After"] != "3600" {
		t.Errorf("Retry-After = %q", resp.Headers["Retry-After"])
	}
	e.assertNothingDone()
	// An hour later the oldest has left the window.
	e.now = e.now.Add(61 * time.Minute)
	if resp := e.do("POST", "/story-videos", createBody(svPlanA, 1, ""), svGoogleUser); resp.StatusCode != 202 {
		t.Errorf("an hour later: %d %s", resp.StatusCode, resp.Body)
	}
}

// cancelRunning ends every running job of a user, so that only the day allowance can refuse.
func (e *svEnv) cancelRunning(user string) {
	for k, it := range e.items {
		if strings.HasPrefix(k, "USER#"+user) && strings.Contains(k, "STORYVIDEO#") && !storyjob.Terminal(str0(it["status"])) {
			it["status"] = &dbtypes.AttributeValueMemberS{Value: "failed"}
		}
	}
}

func TestStoryVideoBriefActive(t *testing.T) {
	now := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)
	at := func(d time.Duration) string { return now.Add(d).Format(time.RFC3339) }
	for name, tc := range map[string]struct {
		b    storyVideoBrief
		want bool
	}{
		"just touched":                      {storyVideoBrief{Status: "rendering", UpdatedAt: at(-time.Minute)}, true},
		"queued and new":                    {storyVideoBrief{Status: "queued", UpdatedAt: at(-time.Second)}, true},
		"completed":                         {storyVideoBrief{Status: "completed", UpdatedAt: at(-time.Second)}, false},
		"failed":                            {storyVideoBrief{Status: "failed", UpdatedAt: at(-time.Second)}, false},
		"one second short of stale":         {storyVideoBrief{Status: "rendering", UpdatedAt: at(-storyVideoStaleAfter + time.Second)}, true},
		"exactly stale":                     {storyVideoBrief{Status: "rendering", UpdatedAt: at(-storyVideoStaleAfter)}, false},
		"long abandoned":                    {storyVideoBrief{Status: "rendering", UpdatedAt: at(-48 * time.Hour)}, false},
		"no update time falls back to made": {storyVideoBrief{Status: "queued", CreatedAt: at(-time.Minute)}, true},
		"a bad update time falls back":      {storyVideoBrief{Status: "queued", UpdatedAt: "x", CreatedAt: at(-time.Minute)}, true},
		"no times at all":                   {storyVideoBrief{Status: "queued"}, false},
		"an unknown status is not final":    {storyVideoBrief{Status: "weird", UpdatedAt: at(-time.Minute)}, true},
	} {
		if got := tc.b.active(now); got != tc.want {
			t.Errorf("%s: %v, want %v", name, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Get

func TestStoryVideos_GetShowsOnlyTheViewAndOnlyToItsOwner(t *testing.T) {
	e := newSVEnv(t)
	id := e.seedJob(svGoogleUser, svPlanA, "rendering", e.now.Add(-time.Minute), func(j *storyjob.Job) { j.Progress = 61 })

	resp := e.do("GET", "/story-videos/"+id, "", svGoogleUser)
	if resp.StatusCode != 200 {
		t.Fatalf("%d %s", resp.StatusCode, resp.Body)
	}
	if v := decodeVideo(t, resp); v.JobID != id || v.Status != "rendering" || v.Progress != 61 || v.PlanID != svPlanA {
		t.Errorf("view = %+v", v)
	}
	if g := e.gets[len(e.gets)-1]; !aws.ToBool(g.ConsistentRead) {
		t.Error("a status poll must be a consistent read, or progress can go backwards")
	}
	// The same id for another user is not found, and says nothing else.
	for name, user := range map[string]string{"another user": svUUIDUser} {
		other := e.do("GET", "/story-videos/"+id, "", user)
		if other.StatusCode != 404 || decodeErr(t, other).Code != "NOT_FOUND" || strings.Contains(other.Body, id) {
			t.Errorf("%s: %d %s", name, other.StatusCode, other.Body)
		}
	}
	if resp := e.do("GET", "/story-videos/"+strings.ToUpper(id), "", svGoogleUser); resp.StatusCode != 200 {
		t.Errorf("an upper case id: %d", resp.StatusCode)
	}
	if resp := e.do("GET", "/story-videos/01939f6c-8a40-7e21-b2c4-aa11bb22ffff", "", svGoogleUser); resp.StatusCode != 404 {
		t.Errorf("an id that does not exist: %d", resp.StatusCode)
	}
}

func TestStoryVideos_TheErrorIsShownOnlyOnceTheVideoHasFailed(t *testing.T) {
	e := newSVEnv(t)
	reason := func(j *storyjob.Job) {
		j.Error, j.ErrorCode, j.FailedStage = "Scene 1: Pixabay did not answer. Try again in a few minutes.", "MEDIA_TEMPORARY", "preparing"
	}
	running := e.seedJob(svGoogleUser, svPlanA, "preparing", e.now.Add(-time.Minute), reason)
	failed := e.seedJob(svGoogleUser, svPlanB, "failed", e.now.Add(-2*time.Minute))
	done := e.seedJob(svGoogleUser, svPlanC, "completed", e.now.Add(-3*time.Minute), reason)

	if v := decodeVideo(t, e.do("GET", "/story-videos/"+running, "", svGoogleUser)); v.Error != "" || v.ErrorCode != "" || v.FailedStage != "" {
		t.Errorf("a video still going shows the reason of an attempt that will be retried: %+v", v)
	}
	if v := decodeVideo(t, e.do("GET", "/story-videos/"+failed, "", svGoogleUser)); v.Status != "failed" || v.ErrorCode != "MEDIA_UNAVAILABLE" || v.FailedStage != "preparing" || !strings.Contains(v.Error, "Choose another") {
		t.Errorf("failed view = %+v", v)
	}
	if v := decodeVideo(t, e.do("GET", "/story-videos/"+done, "", svGoogleUser)); v.Error != "" || v.CompletedAt == "" || v.TotalSeconds != 12.5 {
		t.Errorf("completed view = %+v", v)
	}
}

func TestStoryVideos_NoResponseCarriesAStorageKeyOrSceneText(t *testing.T) {
	e := newSVEnv(t)
	var ids []string
	for i, st := range []string{"queued", "preparing", "narrating", "rendering", "completed", "failed"} {
		ids = append(ids, e.seedJob(svGoogleUser, svPlanA, st, e.now.Add(-time.Duration(i+1)*time.Minute)))
	}
	bodies := []string{e.do("GET", "/story-videos", "", svGoogleUser).Body}
	for _, id := range ids {
		bodies = append(bodies, e.do("GET", "/story-videos/"+id, "", svGoogleUser).Body)
	}
	for _, b := range bodies {
		for _, leak := range []string{"storyvideo", "outputKey", "scenes", "rows quietly", "marks", "voice.mp3", "pixabay"} {
			if strings.Contains(b, leak) {
				t.Errorf("a response contains %q: %s", leak, b)
			}
		}
	}
}

func TestStoryVideos_ARecordThatCannotBeReadIsAnErrorWithoutTheDetails(t *testing.T) {
	e := newSVEnv(t)
	id := e.seedJob(svGoogleUser, svPlanA, "rendering", e.now)
	e.items["USER#"+svGoogleUser+"|STORYVIDEO#"+id]["scenes"] = &dbtypes.AttributeValueMemberS{Value: "{not json"}
	resp := e.do("GET", "/story-videos/"+id, "", svGoogleUser)
	if resp.StatusCode != 500 || strings.Contains(resp.Body, "scenes") || strings.Contains(resp.Body, "json") {
		t.Errorf("%d %s", resp.StatusCode, resp.Body)
	}
	e.getErr = errors.New("dynamo down")
	if resp := e.do("GET", "/story-videos/"+id, "", svGoogleUser); resp.StatusCode != 500 || strings.Contains(resp.Body, "dynamo") {
		t.Errorf("%d %s", resp.StatusCode, resp.Body)
	}
}

// ---------------------------------------------------------------------------
// List

func TestStoryVideos_ListIsNewestFirstFilteredByPlanAndOnlyTheUsers(t *testing.T) {
	e := newSVEnv(t)
	a1 := e.seedJob(svGoogleUser, svPlanA, "completed", e.now.Add(-3*time.Hour))
	b1 := e.seedJob(svGoogleUser, svPlanB, "completed", e.now.Add(-2*time.Hour))
	a2 := e.seedJob(svGoogleUser, svPlanA, "failed", e.now.Add(-1*time.Hour))
	e.seedJob(svUUIDUser, svPlanA, "completed", e.now.Add(-30*time.Minute))

	list := func(query map[string]string) (ids []string, more bool, resp events.APIGatewayProxyResponse) {
		resp = e.doQuery("GET", "/story-videos", "", svGoogleUser, query)
		var body struct {
			Videos  []StoryVideoResponse `json:"videos"`
			HasMore bool                 `json:"hasMore"`
		}
		if err := json.Unmarshal([]byte(resp.Body), &body); err != nil {
			t.Fatalf("%s", resp.Body)
		}
		for _, v := range body.Videos {
			ids = append(ids, v.JobID)
		}
		return ids, body.HasMore, resp
	}
	if ids, more, _ := list(nil); fmt.Sprint(ids) != fmt.Sprint([]string{a2, b1, a1}) || more {
		t.Errorf("all = %v more=%v, want %v", ids, more, []string{a2, b1, a1})
	}
	if ids, _, _ := list(map[string]string{"planId": svPlanA}); fmt.Sprint(ids) != fmt.Sprint([]string{a2, a1}) {
		t.Errorf("plan A = %v", ids)
	}
	if ids, _, _ := list(map[string]string{"planId": strings.ToUpper(svPlanB)}); fmt.Sprint(ids) != fmt.Sprint([]string{b1}) {
		t.Errorf("plan B (upper case) = %v", ids)
	}
	if ids, _, _ := list(map[string]string{"planId": svPlanC}); len(ids) != 0 {
		t.Errorf("a plan with no videos = %v", ids)
	}
	// The other user sees only their own video, not the first user's.
	own := e.seedJob(svUUIDUser, svPlanA, "completed", e.now.Add(-30*time.Minute))
	resp := e.doQuery("GET", "/story-videos", "", svUUIDUser, nil)
	if resp.StatusCode != 200 || !strings.Contains(resp.Body, own) || strings.Contains(resp.Body, a1) || strings.Contains(resp.Body, a2) || strings.Contains(resp.Body, b1) {
		t.Errorf("another user's list: %d %s", resp.StatusCode, resp.Body)
	}
	if _, _, resp := list(map[string]string{"planId": "../x"}); resp.StatusCode != 400 {
		t.Errorf("a bad plan id: %d", resp.StatusCode)
	}
	// A user with no videos gets [], not null: the page loops over it.
	if body := newSVEnv(t).do("GET", "/story-videos", "", svGoogleUser).Body; !strings.Contains(body, `"videos":[]`) {
		t.Errorf("an empty list must be [], got %s", body)
	}
}

func TestStoryVideos_ListReadsABoundedPageWithoutTheScenes(t *testing.T) {
	e := newSVEnv(t)
	e.seedJob(svGoogleUser, svPlanA, "completed", e.now.Add(-time.Hour))
	e.do("GET", "/story-videos", "", svGoogleUser)
	q := e.queries[len(e.queries)-1]
	if q.Limit == nil || *q.Limit != storyVideoListRead || q.ScanIndexForward == nil || *q.ScanIndexForward {
		t.Errorf("limit %v forward %v", q.Limit, q.ScanIndexForward)
	}
	attrs := strings.Join(e.resolvedProjection(q), ",")
	for _, banned := range []string{"scenes", "outputKey"} {
		if strings.Contains(attrs, banned) {
			t.Errorf("the list reads %s: %s", banned, attrs)
		}
	}
	for _, need := range []string{"SK", "status", "progress", "caption", "planId"} {
		if !strings.Contains(attrs, need) {
			t.Errorf("the list does not read %s", need)
		}
	}
	// Names are placeholders: "status", "language" and "error" are reserved words.
	if strings.Contains(aws.ToString(q.ProjectionExpression), "status") {
		t.Error("attribute names must be placeholders")
	}
}

func TestStoryVideos_ListReturnsTwentyAndSaysThereAreMore(t *testing.T) {
	e := newSVEnv(t)
	for i := 0; i < 25; i++ {
		e.seedJob(svGoogleUser, svPlanA, "completed", e.now.Add(-time.Duration(i+1)*time.Hour))
	}
	var body struct {
		Videos  []StoryVideoResponse `json:"videos"`
		HasMore bool                 `json:"hasMore"`
	}
	if err := json.Unmarshal([]byte(e.do("GET", "/story-videos", "", svGoogleUser).Body), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Videos) != storyVideoListMax || !body.HasMore {
		t.Errorf("%d videos, hasMore %v", len(body.Videos), body.HasMore)
	}
	// Newest first, strictly.
	for i := 1; i < len(body.Videos); i++ {
		if body.Videos[i-1].JobID <= body.Videos[i].JobID {
			t.Fatalf("not newest first at %d", i)
		}
	}
	// When the page read is itself cut short, that is "more" too, even if the filter matched little.
	e2 := newSVEnv(t)
	for i := 0; i < storyVideoListRead+1; i++ {
		plan := svPlanB
		if i == 0 {
			plan = svPlanA
		}
		e2.seedJob(svGoogleUser, plan, "completed", e2.now.Add(-time.Duration(i+1)*time.Minute))
	}
	resp := e2.doQuery("GET", "/story-videos", "", svGoogleUser, map[string]string{"planId": svPlanA})
	if err := json.Unmarshal([]byte(resp.Body), &body); err != nil || len(body.Videos) != 1 || !body.HasMore {
		t.Errorf("%d videos, hasMore %v: the read stopped at %d records, so the filter may have missed some", len(body.Videos), body.HasMore, storyVideoListRead)
	}
}

func TestStoryVideos_AListSkipsARecordItCannotReadInsteadOfFailing(t *testing.T) {
	e := newSVEnv(t)
	good := e.seedJob(svGoogleUser, svPlanA, "completed", e.now.Add(-time.Hour))
	bad := e.seedJob(svGoogleUser, svPlanA, "completed", e.now.Add(-2*time.Hour))
	e.items["USER#"+svGoogleUser+"|STORYVIDEO#"+bad]["planRevision"] = &dbtypes.AttributeValueMemberN{Value: "x"}
	resp := e.do("GET", "/story-videos", "", svGoogleUser)
	if resp.StatusCode != 200 || !strings.Contains(resp.Body, good) || strings.Contains(resp.Body, bad) {
		t.Errorf("%d %s", resp.StatusCode, resp.Body)
	}
	e.qErr = errors.New("dynamo down")
	if resp := e.do("GET", "/story-videos", "", svGoogleUser); resp.StatusCode != 500 || strings.Contains(resp.Body, "dynamo") {
		t.Errorf("%d %s", resp.StatusCode, resp.Body)
	}
}

// ---------------------------------------------------------------------------
// Cancel

func TestStoryVideos_CancelEndsTheRecordFirstThenStopsTheExecution(t *testing.T) {
	e := newSVEnv(t)
	id := e.seedJob(svGoogleUser, svPlanA, "rendering", e.now.Add(-time.Minute), func(j *storyjob.Job) {
		j.Progress = 52
		j.Error, j.ErrorCode = "Scene 1: Pixabay did not answer. Try again in a few minutes.", "MEDIA_TEMPORARY" // a retry's saved reason
	})
	resp := e.do("POST", "/story-videos/"+id+"/cancel", "", svGoogleUser)
	if resp.StatusCode != 200 {
		t.Fatalf("%d %s", resp.StatusCode, resp.Body)
	}
	v := decodeVideo(t, resp)
	if v.Status != "failed" || v.ErrorCode != "CANCELLED" || v.Error != storyVideoCancelledMessage || v.FailedStage != "rendering" {
		t.Errorf("view = %+v", v)
	}
	j := e.stored(svGoogleUser, id)
	if j.Status != "failed" || j.ErrorCode != "CANCELLED" || j.Error != storyVideoCancelledMessage || j.FailedStage != "rendering" || j.Progress != 52 {
		t.Errorf("stored = %+v", j)
	}
	// The notifier skips errorCode "CANCELLED" by this exact text.
	if j.ErrorCode != "CANCELLED" {
		t.Error("the notifier would email a cancel")
	}
	if got := strings.Join(e.events, ","); got != "update,stop" {
		t.Errorf("events = %s: the record must end before the execution is stopped", got)
	}
	if len(e.stops) != 1 || aws.ToString(e.stops[0].ExecutionArn) != "arn:aws:states:ap-south-1:123456789012:execution:indifferent-fun-story-pipeline:"+id {
		t.Errorf("stopped %v", e.stops)
	}
}

func TestStoryVideos_CancellingAVideoThatHasEndedChangesNothing(t *testing.T) {
	for _, st := range []string{"completed", "failed"} {
		t.Run(st, func(t *testing.T) {
			e := newSVEnv(t)
			id := e.seedJob(svGoogleUser, svPlanA, st, e.now.Add(-time.Hour))
			before := *e.stored(svGoogleUser, id)
			resp := e.do("POST", "/story-videos/"+id+"/cancel", "", svGoogleUser)
			if resp.StatusCode != 200 || decodeVideo(t, resp).Status != st {
				t.Fatalf("%d %s", resp.StatusCode, resp.Body)
			}
			e.assertNothingDone()
			if after := *e.stored(svGoogleUser, id); fmt.Sprint(after) != fmt.Sprint(before) {
				t.Errorf("the record changed:\n%+v\n%+v", before, after)
			}
		})
	}
}

func TestStoryVideos_ACancelNeverOverwritesAVideoThatFinishesFirst(t *testing.T) {
	e := newSVEnv(t)
	id := e.seedJob(svGoogleUser, svPlanA, "rendering", e.now.Add(-time.Minute))
	// The worker finishes after the handler read the record and before it wrote.
	e.beforeUpdate = func() {
		e.mu.Lock()
		defer e.mu.Unlock()
		it := e.items["USER#"+svGoogleUser+"|STORYVIDEO#"+id]
		it["status"] = &dbtypes.AttributeValueMemberS{Value: "completed"}
		it["outputKey"] = &dbtypes.AttributeValueMemberS{Value: storyjob.OutputKey(svGoogleUser, id)}
	}
	resp := e.do("POST", "/story-videos/"+id+"/cancel", "", svGoogleUser)
	if resp.StatusCode != 200 {
		t.Fatalf("%d %s", resp.StatusCode, resp.Body)
	}
	if v := decodeVideo(t, resp); v.Status != "completed" || v.ErrorCode != "" {
		t.Errorf("the answer must be what the video is now: %+v", v)
	}
	j := e.stored(svGoogleUser, id)
	if j.Status != "completed" || j.ErrorCode != "" || j.OutputKey == "" {
		t.Errorf("a finished video was overwritten by a cancel: %+v", j)
	}
	if len(e.stops) != 0 {
		t.Error("it stopped the execution of a video that had finished")
	}
}

func TestStoryVideos_CancellingTwiceIsFine(t *testing.T) {
	e := newSVEnv(t)
	id := e.seedJob(svGoogleUser, svPlanA, "narrating", e.now.Add(-time.Minute))
	e.do("POST", "/story-videos/"+id+"/cancel", "", svGoogleUser)
	e.now = e.now.Add(time.Minute)
	resp := e.do("POST", "/story-videos/"+id+"/cancel", "", svGoogleUser)
	if resp.StatusCode != 200 || decodeVideo(t, resp).ErrorCode != "CANCELLED" {
		t.Errorf("%d %s", resp.StatusCode, resp.Body)
	}
	if len(e.updates) != 1 || len(e.stops) != 1 {
		t.Errorf("%d updates and %d stops: the second cancel did something", len(e.updates), len(e.stops))
	}
}

func TestStoryVideos_CancelIsForTheOwnerOnly(t *testing.T) {
	e := newSVEnv(t)
	id := e.seedJob(svGoogleUser, svPlanA, "rendering", e.now.Add(-time.Minute))
	if resp := e.do("POST", "/story-videos/"+id+"/cancel", "", svUUIDUser); resp.StatusCode != 404 {
		t.Errorf("another user's cancel: %d", resp.StatusCode)
	}
	if resp := e.do("POST", "/story-videos/01939f6c-8a40-7e21-b2c4-aa11bb22ffff/cancel", "", svGoogleUser); resp.StatusCode != 404 {
		t.Errorf("a video that does not exist: %d", resp.StatusCode)
	}
	e.assertNothingDone()
	if e.stored(svGoogleUser, id).Status != "rendering" {
		t.Error("somebody else's cancel changed the video")
	}
}

func TestStoryVideos_AStopThatFailsDoesNotFailTheCancel(t *testing.T) {
	e := newSVEnv(t)
	id := e.seedJob(svGoogleUser, svPlanA, "rendering", e.now.Add(-time.Minute))
	e.stopErr = errors.New("AccessDeniedException: not authorized to perform states:StopExecution")
	resp := e.do("POST", "/story-videos/"+id+"/cancel", "", svGoogleUser)
	if resp.StatusCode != 200 || strings.Contains(resp.Body, "Denied") || e.stored(svGoogleUser, id).ErrorCode != "CANCELLED" {
		t.Errorf("%d %s", resp.StatusCode, resp.Body)
	}
	// And with no state machine configured the record is still ended.
	id2 := e.seedJob(svGoogleUser, svPlanB, "rendering", e.now.Add(-time.Minute))
	e.h.StoryVideoStateMachineARN = ""
	e.stops = nil
	if resp := e.do("POST", "/story-videos/"+id2+"/cancel", "", svGoogleUser); resp.StatusCode != 200 || len(e.stops) != 0 || e.stored(svGoogleUser, id2).ErrorCode != "CANCELLED" {
		t.Errorf("%d %s", resp.StatusCode, resp.Body)
	}
}

func TestStoryVideos_ACancelThatCannotBeWrittenIsAnErrorAndStopsNothing(t *testing.T) {
	e := newSVEnv(t)
	id := e.seedJob(svGoogleUser, svPlanA, "rendering", e.now.Add(-time.Minute))
	e.updErr = errors.New("dynamo down")
	resp := e.do("POST", "/story-videos/"+id+"/cancel", "", svGoogleUser)
	if resp.StatusCode != 500 || strings.Contains(resp.Body, "dynamo") {
		t.Errorf("%d %s", resp.StatusCode, resp.Body)
	}
	if len(e.stops) != 0 {
		t.Error("it stopped the execution although the cancel was not recorded: the worker would stop with the job still saying it runs")
	}
}

// ---------------------------------------------------------------------------
// Download

func TestStoryVideos_DownloadOfACompletedVideoIsAShortLinkToItsOwnFile(t *testing.T) {
	e := newSVEnv(t)
	id := e.seedJob(svGoogleUser, svPlanA, "completed", e.now.Add(-time.Hour))
	resp := e.do("GET", "/story-videos/"+id+"/download", "", svGoogleUser)
	if resp.StatusCode != 200 {
		t.Fatalf("%d %s", resp.StatusCode, resp.Body)
	}
	var body struct {
		URL              string `json:"url"`
		Filename         string `json:"filename"`
		ExpiresInSeconds int    `json:"expiresInSeconds"`
	}
	_ = json.Unmarshal([]byte(resp.Body), &body)
	want := "storyvideo/" + svGoogleUser + "/" + id + "/video.mp4"
	if len(e.s3.presigned) != 1 || e.s3.presigned[0] != want || e.s3.filenames[0] != "story-video.mp4" {
		t.Errorf("presigned %v as %v, want %s", e.s3.presigned, e.s3.filenames, want)
	}
	if !strings.Contains(body.URL, want) || body.Filename != "story-video.mp4" || body.ExpiresInSeconds != 900 {
		t.Errorf("body = %+v", body)
	}
}

func TestStoryVideos_DownloadBeforeTheVideoIsFinishedIsRefused(t *testing.T) {
	e := newSVEnv(t)
	for _, st := range []string{"queued", "preparing", "narrating", "rendering", "failed"} {
		id := e.seedJob(svGoogleUser, svPlanA, st, e.now.Add(-time.Minute), func(j *storyjob.Job) {
			j.OutputKey = storyjob.OutputKey(svGoogleUser, j.JobID) // even with a key on the record
		})
		resp := e.do("GET", "/story-videos/"+id+"/download", "", svGoogleUser)
		if resp.StatusCode != 409 || decodeErr(t, resp).Code != "NOT_READY" {
			t.Errorf("%s: %d %s", st, resp.StatusCode, resp.Body)
		}
	}
	// A completed record with no file named is not a download either.
	id := e.seedJob(svGoogleUser, svPlanB, "completed", e.now.Add(-time.Minute), func(j *storyjob.Job) { j.OutputKey = "" })
	if resp := e.do("GET", "/story-videos/"+id+"/download", "", svGoogleUser); resp.StatusCode != 409 {
		t.Errorf("no output key: %d", resp.StatusCode)
	}
	if len(e.s3.presigned) != 0 {
		t.Errorf("links were made for %v", e.s3.presigned)
	}
}

func TestStoryVideos_DownloadNeverLinksToAFileThatIsNotTheJobsOwn(t *testing.T) {
	e := newSVEnv(t)
	id := e.seedJob(svGoogleUser, svPlanA, "completed", e.now.Add(-time.Hour), func(j *storyjob.Job) {
		j.OutputKey = storyjob.OutputKey(svUUIDUser, "01939f6c-8a40-7e21-b2c4-aa11bb22ffff") // another user's video
	})
	resp := e.do("GET", "/story-videos/"+id+"/download", "", svGoogleUser)
	if resp.StatusCode != 500 || len(e.s3.presigned) != 0 || strings.Contains(resp.Body, svUUIDUser) {
		t.Errorf("%d %s presigned=%v", resp.StatusCode, resp.Body, e.s3.presigned)
	}
	own := e.seedJob(svGoogleUser, svPlanB, "completed", e.now.Add(-time.Hour))
	if resp := e.do("GET", "/story-videos/"+own+"/download", "", svUUIDUser); resp.StatusCode != 404 {
		t.Errorf("another user's download: %d", resp.StatusCode)
	}
}

func TestStoryVideos_DownloadSaysSoWhenTheFileIsGone(t *testing.T) {
	e := newSVEnv(t)
	id := e.seedJob(svGoogleUser, svPlanA, "completed", e.now.Add(-time.Hour))
	e.s3.statusErr = &s3types.NotFound{}
	resp := e.do("GET", "/story-videos/"+id+"/download", "", svGoogleUser)
	if resp.StatusCode != 404 || len(e.s3.presigned) != 0 {
		t.Errorf("%d %s", resp.StatusCode, resp.Body)
	}
	// A check that fails for another reason does not block a healthy download.
	e.s3.statusErr = errors.New("timeout")
	if resp := e.do("GET", "/story-videos/"+id+"/download", "", svGoogleUser); resp.StatusCode != 200 {
		t.Errorf("a failed status check blocked the download: %d", resp.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// Delete

func TestStoryVideos_DeleteRemovesTheFilesThenTheRecord(t *testing.T) {
	for _, st := range []string{"completed", "failed"} {
		t.Run(st, func(t *testing.T) {
			e := newSVEnv(t)
			id := e.seedJob(svGoogleUser, svPlanA, st, e.now.Add(-time.Hour))
			other := e.seedJob(svGoogleUser, svPlanA, "completed", e.now.Add(-2*time.Hour))
			resp := e.do("DELETE", "/story-videos/"+id, "", svGoogleUser)
			if resp.StatusCode != 204 || resp.Body != "" {
				t.Fatalf("%d %s", resp.StatusCode, resp.Body)
			}
			want := []string{
				"purge:storyvideo/" + svGoogleUser + "/" + id + "/",
				"purge:storyvideo-work/" + svGoogleUser + "/" + id + "/",
				"delete",
			}
			if got := strings.Join(e.events, ","); got != strings.Join(want, ",") {
				t.Errorf("events = %s\nwant     %s", got, strings.Join(want, ","))
			}
			if e.stored(svGoogleUser, id) != nil || e.stored(svGoogleUser, other) == nil {
				t.Error("the wrong record was removed")
			}
			if resp.Headers["Access-Control-Allow-Origin"] == "" {
				t.Error("no CORS headers on a 204")
			}
		})
	}
}

func TestStoryVideos_AVideoThatIsStillBeingMadeMustBeCancelledFirst(t *testing.T) {
	e := newSVEnv(t)
	for _, st := range []string{"queued", "preparing", "narrating", "rendering"} {
		id := e.seedJob(svGoogleUser, svPlanA, st, e.now.Add(-time.Minute))
		resp := e.do("DELETE", "/story-videos/"+id, "", svGoogleUser)
		if resp.StatusCode != 409 || decodeErr(t, resp).Code != "STILL_RUNNING" {
			t.Errorf("%s: %d %s", st, resp.StatusCode, resp.Body)
		}
		if e.stored(svGoogleUser, id) == nil {
			t.Errorf("%s: the record was removed", st)
		}
	}
	if len(e.events) != 0 {
		t.Errorf("effects: %v", e.events)
	}
	// Cancel, then delete.
	id := e.seedJob(svGoogleUser, svPlanB, "rendering", e.now.Add(-time.Minute))
	e.do("POST", "/story-videos/"+id+"/cancel", "", svGoogleUser)
	if resp := e.do("DELETE", "/story-videos/"+id, "", svGoogleUser); resp.StatusCode != 204 {
		t.Errorf("after the cancel: %d %s", resp.StatusCode, resp.Body)
	}
}

func TestStoryVideos_AFailedCleanupKeepsTheRecordSoTheUserCanTryAgain(t *testing.T) {
	e := newSVEnv(t)
	id := e.seedJob(svGoogleUser, svPlanA, "completed", e.now.Add(-time.Hour))
	e.s3.purgeErr = errors.New("s3 down")
	resp := e.do("DELETE", "/story-videos/"+id, "", svGoogleUser)
	if resp.StatusCode != 500 || decodeErr(t, resp).Code != "S3_CLEANUP_FAILED" || strings.Contains(resp.Body, "s3 down") {
		t.Fatalf("%d %s", resp.StatusCode, resp.Body)
	}
	if e.stored(svGoogleUser, id) == nil {
		t.Fatal("the record went although the files did not: nobody could find the video to remove it")
	}
	for _, ev := range e.events {
		if ev == "delete" {
			t.Error("the record was deleted")
		}
	}
	e.s3.purgeErr = nil
	if resp := e.do("DELETE", "/story-videos/"+id, "", svGoogleUser); resp.StatusCode != 204 || e.stored(svGoogleUser, id) != nil {
		t.Errorf("the retry: %d %s", resp.StatusCode, resp.Body)
	}
}

func TestStoryVideos_ARecordThatCannotBeDeletedIsAnError(t *testing.T) {
	e := newSVEnv(t)
	id := e.seedJob(svGoogleUser, svPlanA, "completed", e.now.Add(-time.Hour))
	e.delErr = errors.New("dynamo down")
	if resp := e.do("DELETE", "/story-videos/"+id, "", svGoogleUser); resp.StatusCode != 500 || strings.Contains(resp.Body, "dynamo") {
		t.Errorf("%d %s", resp.StatusCode, resp.Body)
	}
}

func TestStoryVideos_DeleteIsForTheOwnerOnlyAndCannotReachAnotherUsersFiles(t *testing.T) {
	e := newSVEnv(t)
	id := e.seedJob(svGoogleUser, svPlanA, "completed", e.now.Add(-time.Hour))
	if resp := e.do("DELETE", "/story-videos/"+id, "", svUUIDUser); resp.StatusCode != 404 {
		t.Errorf("another user's delete: %d", resp.StatusCode)
	}
	if resp := e.do("DELETE", "/story-videos/01939f6c-8a40-7e21-b2c4-aa11bb22ffff", "", svGoogleUser); resp.StatusCode != 404 {
		t.Errorf("a video that does not exist: %d", resp.StatusCode)
	}
	if len(e.events) != 0 || e.stored(svGoogleUser, id) == nil {
		t.Errorf("a refused delete did something: %v", e.events)
	}
}

// ---------------------------------------------------------------------------

func TestStoryVideos_ResponsesAreJSONWithCORS(t *testing.T) {
	e := newSVEnv(t)
	id := e.seedJob(svGoogleUser, svPlanA, "completed", e.now.Add(-time.Hour))
	for _, resp := range []events.APIGatewayProxyResponse{
		e.do("GET", "/story-videos", "", svGoogleUser),
		e.do("GET", "/story-videos/"+id, "", svGoogleUser),
		e.do("GET", "/story-videos/limits", "", ""),
		e.do("GET", "/story-videos/nope", "", svGoogleUser),
		e.do("POST", "/story-videos", "{", svGoogleUser),
	} {
		if resp.Headers["Content-Type"] != "application/json" || resp.Headers["Access-Control-Allow-Origin"] != "https://indifferent.fun" {
			t.Errorf("headers = %v", resp.Headers)
		}
	}
}
