package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/google/uuid"
	"github.com/rahul/indifferent/backend/internal/models"
	"github.com/rahul/indifferent/backend/internal/storyplan"
)

const (
	spEnglish = "A fisherman in Kerala wakes before dawn, rows out into the backwaters, and finds a glowing lantern in his net."
	spHindi   = "केरल का एक मछुआरा भोर से पहले उठता है, पीछे के पानी में नाव खेता है, और अपने जाल में एक चमकती लालटेन पाता है।"
)

// fakePlanner is a StoryPlanner whose result the test controls.
type fakePlanner struct {
	mu      sync.Mutex
	res     storyplan.Result
	err     error
	stories []string
	hook    func()
}

func (f *fakePlanner) Generate(ctx context.Context, story string) (storyplan.Result, error) {
	f.mu.Lock()
	f.stories = append(f.stories, story)
	hook := f.hook
	res, err := f.res, f.err
	f.mu.Unlock()
	if hook != nil {
		hook()
	}
	return res, err
}

func (f *fakePlanner) Model() string { return "test-model" }

func (f *fakePlanner) calls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.stories)
}

func samplePlan(lang string) storyplan.Plan {
	narr := "the fisherman rows quietly across the dark water"
	if lang == "hi" {
		narr = "मछुआरा चुपचाप अँधेरे पानी में नाव खेता है"
	}
	return storyplan.Plan{
		Language: lang,
		Hook:     "Before dawn.",
		Scenes: []storyplan.Scene{
			{Narration: narr, Description: "A boat on calm water", Keywords: []string{"fishing boat", "sunrise"}, MediaType: "video", Duration: 5},
			{Narration: narr, Description: "A glowing lantern", Keywords: []string{"lantern"}, MediaType: "photo", Duration: 6},
		},
		TotalSeconds: 11,
	}
}

// spEnv is a handler wired to an in-memory table.
type spEnv struct {
	h       *APIHandler
	planner *fakePlanner
	mu      sync.Mutex
	items   map[string]map[string]dbtypes.AttributeValue // "PK|SK" -> item
	puts    []map[string]dbtypes.AttributeValue
	queries []*dynamodb.QueryInput
	putErr  func(n int) error // returns an error for the nth PutItem (1-based), nil to allow
	getErr  error
	qErr    error
	now     time.Time
}

func newSPEnv(t *testing.T) *spEnv {
	t.Helper()
	e := &spEnv{
		items:   map[string]map[string]dbtypes.AttributeValue{},
		planner: &fakePlanner{res: storyplan.Result{Plan: samplePlan("en"), Script: storyplan.ScriptLatin, StoryRunes: 100, Latency: 4200 * time.Millisecond}},
		now:     time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC),
	}
	key := func(pk, sk string) string { return pk + "|" + sk }
	str := func(v dbtypes.AttributeValue) string { return v.(*dbtypes.AttributeValueMemberS).Value }
	db := &mockDynamoDB{
		putItemFunc: func(_ context.Context, in *dynamodb.PutItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
			e.mu.Lock()
			defer e.mu.Unlock()
			e.puts = append(e.puts, in.Item)
			if e.putErr != nil {
				if err := e.putErr(len(e.puts)); err != nil {
					return nil, err
				}
			}
			e.items[key(str(in.Item["PK"]), str(in.Item["SK"]))] = in.Item
			return &dynamodb.PutItemOutput{}, nil
		},
		getItemFunc: func(_ context.Context, in *dynamodb.GetItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
			e.mu.Lock()
			defer e.mu.Unlock()
			if e.getErr != nil {
				return nil, e.getErr
			}
			return &dynamodb.GetItemOutput{Item: e.items[key(str(in.Key["PK"]), str(in.Key["SK"]))]}, nil
		},
		queryFunc: func(_ context.Context, in *dynamodb.QueryInput, _ ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error) {
			e.mu.Lock()
			defer e.mu.Unlock()
			e.queries = append(e.queries, in)
			if e.qErr != nil {
				return nil, e.qErr
			}
			pk := str(in.ExpressionAttributeValues[":pk"])
			lo, hi := str(in.ExpressionAttributeValues[":lo"]), str(in.ExpressionAttributeValues[":hi"])
			var out []map[string]dbtypes.AttributeValue
			for _, it := range e.items {
				if str(it["PK"]) == pk && str(it["SK"]) >= lo && str(it["SK"]) <= hi {
					out = append(out, it)
				}
			}
			// DynamoDB returns the range in sort-key order.
			sortBySK(out)
			if in.Limit != nil && len(out) > int(*in.Limit) {
				out = out[:*in.Limit]
			}
			return &dynamodb.QueryOutput{Items: out}, nil
		},
	}
	h := newShortsTestHandler(t, db)
	h.StoryPlanner = e.planner
	h.StoryPlanNow = func() time.Time { return e.now }
	e.h = h
	return e
}

func sortBySK(items []map[string]dbtypes.AttributeValue) {
	for i := 1; i < len(items); i++ {
		for j := i; j > 0 && items[j]["SK"].(*dbtypes.AttributeValueMemberS).Value < items[j-1]["SK"].(*dbtypes.AttributeValueMemberS).Value; j-- {
			items[j], items[j-1] = items[j-1], items[j]
		}
	}
}

func (e *spEnv) do(t *testing.T, method, path, body, user string) events.APIGatewayProxyResponse {
	t.Helper()
	headers := map[string]string{}
	if user != "" {
		headers["Authorization"] = "Bearer " + generateTestToken(user)
	}
	resp, err := e.h.HandleRequest(context.Background(), events.APIGatewayProxyRequest{HTTPMethod: method, Path: path, Headers: headers, Body: body})
	if err != nil {
		t.Fatalf("HandleRequest: %v", err)
	}
	return resp
}

func storyBody(story string) string {
	b, _ := json.Marshal(CreateStoryPlanRequest{Story: story})
	return string(b)
}

func decodeErr(t *testing.T, resp events.APIGatewayProxyResponse) models.APIError {
	t.Helper()
	var e models.APIError
	if err := json.Unmarshal([]byte(resp.Body), &e); err != nil {
		t.Fatalf("not an API error: %s", resp.Body)
	}
	return e
}

func decodePlan(t *testing.T, resp events.APIGatewayProxyResponse) StoryPlanResponse {
	t.Helper()
	var p StoryPlanResponse
	if err := json.Unmarshal([]byte(resp.Body), &p); err != nil {
		t.Fatalf("not a plan response: %s", resp.Body)
	}
	return p
}

func TestStoryPlans_RequireSignIn(t *testing.T) {
	e := newSPEnv(t)
	for _, tc := range [][2]string{
		{"POST", "/story-plans"},
		{"GET", "/story-plans/" + uuid.NewString()},
		{"GET", "/story-plans"},
	} {
		resp := e.do(t, tc[0], tc[1], storyBody(spEnglish), "")
		if resp.StatusCode != 401 {
			t.Errorf("%s %s without a token: status %d, want 401", tc[0], tc[1], resp.StatusCode)
		}
	}
	if e.planner.calls() != 0 {
		t.Error("the planner ran for an unauthenticated request")
	}
}

func TestStoryPlans_LimitsArePublicAndAccurate(t *testing.T) {
	e := newSPEnv(t)
	resp := e.do(t, "GET", "/story-plans/limits", "", "") // no sign-in
	if resp.StatusCode != 200 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	var l StoryPlanLimits
	if err := json.Unmarshal([]byte(resp.Body), &l); err != nil {
		t.Fatal(err)
	}
	want := StoryPlanLimits{Enabled: true, MinStoryChars: storyplan.MinStoryRunes, MaxStoryChars: storyplan.MaxStoryRunes,
		MinScenes: storyplan.MinScenes, MaxScenes: storyplan.MaxScenes, MaxTotalSeconds: storyplan.MaxTotalSeconds, PerHour: 20}
	if l != want {
		t.Errorf("limits = %+v, want %+v", l, want)
	}
	e.h.StoryPlanner = nil
	var off StoryPlanLimits
	_ = json.Unmarshal([]byte(e.do(t, "GET", "/story-plans/limits", "", "").Body), &off)
	if off.Enabled {
		t.Error("limits should report enabled=false when the planner is off")
	}
}

func TestStoryPlans_SwitchedOffAnswers503WithoutTouchingTheModelOrTheDatabase(t *testing.T) {
	e := newSPEnv(t)
	e.h.StoryPlanner = nil
	resp := e.do(t, "POST", "/story-plans", storyBody(spEnglish), "user1")
	if resp.StatusCode != 503 || decodeErr(t, resp).Code != "UNAVAILABLE" {
		t.Errorf("status %d body %s", resp.StatusCode, resp.Body)
	}
	if len(e.puts) != 0 || len(e.queries) != 0 {
		t.Error("a switched-off feature touched the database")
	}
}

func TestStoryPlans_Validation(t *testing.T) {
	e := newSPEnv(t)
	for _, tc := range []struct {
		name, body, code string
	}{
		{"not json", "{nope", "INVALID_BODY"},
		{"empty body", "", "INVALID_BODY"},
		{"missing story", `{}`, "VALIDATION_ERROR"},
		{"empty story", `{"story":""}`, "VALIDATION_ERROR"},
		{"blank story", `{"story":"   \n  "}`, "VALIDATION_ERROR"},
		{"too short", `{"story":"hi"}`, "VALIDATION_ERROR"},
		{"too long", storyBody(strings.Repeat("a", storyplan.MaxStoryRunes+1)), "VALIDATION_ERROR"},
		{"story is a number", `{"story":123}`, "INVALID_BODY"},
		{"story is a list", `{"story":["a"]}`, "INVALID_BODY"},
		{"request body far too large", `{"story":"` + strings.Repeat("a", storyPlanMaxBodyBytes) + `"}`, "VALIDATION_ERROR"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resp := e.do(t, "POST", "/story-plans", tc.body, "user1")
			if resp.StatusCode != 400 || decodeErr(t, resp).Code != tc.code {
				t.Errorf("status %d body %s, want 400 %s", resp.StatusCode, resp.Body, tc.code)
			}
		})
	}
	if e.planner.calls() != 0 || len(e.puts) != 0 || len(e.queries) != 0 {
		t.Errorf("invalid input reached the model or the database: calls=%d puts=%d queries=%d", e.planner.calls(), len(e.puts), len(e.queries))
	}
}

func TestStoryPlans_ValidationMessagesNameTheLimit(t *testing.T) {
	e := newSPEnv(t)
	msg := decodeErr(t, e.do(t, "POST", "/story-plans", storyBody(strings.Repeat("a", storyplan.MaxStoryRunes+1)), "user1")).Message
	if !strings.Contains(msg, fmt.Sprint(storyplan.MaxStoryRunes)) {
		t.Errorf("message %q should name the limit", msg)
	}
}

func TestStoryPlans_CreateStoresAndReturnsThePlan(t *testing.T) {
	e := newSPEnv(t)
	resp := e.do(t, "POST", "/story-plans", storyBody(spEnglish), "user1")
	if resp.StatusCode != 201 {
		t.Fatalf("status %d body %s", resp.StatusCode, resp.Body)
	}
	got := decodePlan(t, resp)
	if got.Status != "ready" || got.Plan == nil || len(got.Plan.Scenes) != 2 || got.Model != "test-model" {
		t.Fatalf("response = %+v", got)
	}
	if _, err := uuid.Parse(got.PlanID); err != nil {
		t.Errorf("plan id %q is not a UUID", got.PlanID)
	}
	if u, _ := uuid.Parse(got.PlanID); u.Version() != 7 {
		t.Errorf("plan id has version %d, want 7 (time ordered)", u.Version())
	}
	// Two writes: the pending record, then the finished one under the same key.
	if len(e.puts) != 2 {
		t.Fatalf("%d writes, want 2", len(e.puts))
	}
	first, last := e.puts[0], e.puts[1]
	if first["status"].(*dbtypes.AttributeValueMemberS).Value != "pending" || last["status"].(*dbtypes.AttributeValueMemberS).Value != "ready" {
		t.Errorf("statuses = %v then %v", first["status"], last["status"])
	}
	if first["SK"].(*dbtypes.AttributeValueMemberS).Value != last["SK"].(*dbtypes.AttributeValueMemberS).Value {
		t.Error("the finished record must replace the pending one")
	}
	if pk := last["PK"].(*dbtypes.AttributeValueMemberS).Value; pk != "USER#user1" {
		t.Errorf("PK = %q", pk)
	}
	if sk := last["SK"].(*dbtypes.AttributeValueMemberS).Value; sk != "STORYPLAN#"+got.PlanID {
		t.Errorf("SK = %q", sk)
	}
}

// The story is private text. It must reach the model and nothing else.
func TestStoryPlans_TheStoryIsNeverStored(t *testing.T) {
	e := newSPEnv(t)
	const marker = "ZEBRA-MARKER-STORY-TEXT"
	story := marker + " " + spEnglish
	if resp := e.do(t, "POST", "/story-plans", storyBody(story), "user1"); resp.StatusCode != 201 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	if e.planner.stories[0] != story {
		t.Error("the planner did not receive the story")
	}
	for i, item := range e.puts {
		for k, v := range item {
			if s, ok := v.(*dbtypes.AttributeValueMemberS); ok && strings.Contains(s.Value, marker) {
				t.Errorf("write %d stores the story text in %q", i+1, k)
			}
		}
	}
	// What is stored is the length, not the text.
	if n := e.puts[0]["storyRunes"].(*dbtypes.AttributeValueMemberN).Value; n != fmt.Sprint(len([]rune(story))) {
		t.Errorf("storyRunes = %s, want %d", n, len([]rune(story)))
	}
}

func TestStoryPlans_HindiStoryRoundTrip(t *testing.T) {
	e := newSPEnv(t)
	e.planner.res = storyplan.Result{Plan: samplePlan("hi"), Script: storyplan.ScriptDevanagari, StoryRunes: 90}
	resp := e.do(t, "POST", "/story-plans", storyBody(spHindi), "user1")
	if resp.StatusCode != 201 {
		t.Fatalf("status %d body %s", resp.StatusCode, resp.Body)
	}
	id := decodePlan(t, resp).PlanID
	got := decodePlan(t, e.do(t, "GET", "/story-plans/"+id, "", "user1"))
	if got.Plan == nil || got.Plan.Language != "hi" {
		t.Fatalf("plan = %+v", got.Plan)
	}
	if storyplan.ShareInScript(got.Plan.Scenes[0].Narration, storyplan.ScriptDevanagari) < 0.9 {
		t.Errorf("Hindi narration did not survive storage: %q", got.Plan.Scenes[0].Narration)
	}
	if storyplan.HasIndianScript(strings.Join(got.Plan.Scenes[0].Keywords, " ")) {
		t.Errorf("keywords: %v", got.Plan.Scenes[0].Keywords)
	}
	if e.puts[1]["script"].(*dbtypes.AttributeValueMemberS).Value != "devanagari" {
		t.Error("the detected script should be recorded")
	}
}

func TestStoryPlans_ModelFailuresMapToStatusCodesAndAreRecorded(t *testing.T) {
	for _, tc := range []struct {
		name       string
		err        error
		status     int
		code       string
		storedCode string
	}{
		{"timeout", fmt.Errorf("%w: %w", storyplan.ErrModelTimeout, storyplan.ErrModelFailed), 504, "PLAN_TIMEOUT", "timeout"},
		{"model busy", fmt.Errorf("%w: the model is busy", storyplan.ErrModelFailed), 503, "PLAN_UNAVAILABLE", "model_failed"},
		{"unusable answer", fmt.Errorf("%w: only 1 usable scene", storyplan.ErrInvalidPlan), 502, "PLAN_FAILED", "invalid_plan"},
		{"unexpected", errors.New("boom"), 500, "INTERNAL_ERROR", "error"},
		{"cancelled", context.Canceled, 500, "INTERNAL_ERROR", "canceled"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newSPEnv(t)
			e.planner.err = tc.err
			resp := e.do(t, "POST", "/story-plans", storyBody(spEnglish), "user1")
			if resp.StatusCode != tc.status || decodeErr(t, resp).Code != tc.code {
				t.Errorf("status %d body %s, want %d %s", resp.StatusCode, resp.Body, tc.status, tc.code)
			}
			if strings.Contains(resp.Body, "usable scene") || strings.Contains(resp.Body, "boom") {
				t.Errorf("internal detail leaked to the client: %s", resp.Body)
			}
			last := e.puts[len(e.puts)-1]
			if last["status"].(*dbtypes.AttributeValueMemberS).Value != "failed" || last["errorCode"].(*dbtypes.AttributeValueMemberS).Value != tc.storedCode {
				t.Errorf("stored %v %v, want failed %s", last["status"], last["errorCode"], tc.storedCode)
			}
			if _, has := last["plan"]; has {
				t.Error("a failed attempt must not carry a plan")
			}
		})
	}
}

// A failed attempt must still count against the hourly allowance: it cost
// model tokens, and the allowance is what stops a loop of failures from being
// free.
func TestStoryPlans_FailedAttemptsCountTowardsTheAllowance(t *testing.T) {
	e := newSPEnv(t)
	e.h.StoryPlansPerHour = 3
	e.planner.err = fmt.Errorf("%w: busy", storyplan.ErrModelFailed)
	for i := 0; i < 3; i++ {
		e.now = e.now.Add(time.Second) // plan ids are time ordered, so each attempt gets a later id
		if resp := e.do(t, "POST", "/story-plans", storyBody(spEnglish), "user1"); resp.StatusCode != 503 {
			t.Fatalf("attempt %d: status %d", i+1, resp.StatusCode)
		}
	}
	before := e.planner.calls()
	resp := e.do(t, "POST", "/story-plans", storyBody(spEnglish), "user1")
	if resp.StatusCode != 429 || e.planner.calls() != before {
		t.Errorf("status %d, planner calls %d -> %d: the fourth attempt must be refused before the model", resp.StatusCode, before, e.planner.calls())
	}
}

func TestStoryPlans_PendingRecordIsWrittenBeforeTheModelRuns(t *testing.T) {
	e := newSPEnv(t)
	var seenWhileRunning []string
	e.planner.hook = func() {
		e.mu.Lock()
		defer e.mu.Unlock()
		for _, it := range e.items {
			seenWhileRunning = append(seenWhileRunning, it["status"].(*dbtypes.AttributeValueMemberS).Value)
		}
	}
	e.do(t, "POST", "/story-plans", storyBody(spEnglish), "user1")
	if fmt.Sprint(seenWhileRunning) != "[pending]" {
		t.Errorf("while the model ran the table held %v, want [pending]", seenWhileRunning)
	}
}

func TestStoryPlans_RateLimit(t *testing.T) {
	create := func(e *spEnv, user string) events.APIGatewayProxyResponse {
		e.now = e.now.Add(time.Second)
		return e.do(t, "POST", "/story-plans", storyBody(spEnglish), user)
	}
	t.Run("the allowance, then 429 with Retry-After", func(t *testing.T) {
		e := newSPEnv(t)
		e.h.StoryPlansPerHour = 3
		for i := 0; i < 3; i++ {
			if resp := create(e, "user1"); resp.StatusCode != 201 {
				t.Fatalf("plan %d: status %d %s", i+1, resp.StatusCode, resp.Body)
			}
		}
		resp := create(e, "user1")
		if resp.StatusCode != 429 || decodeErr(t, resp).Code != "RATE_LIMITED" {
			t.Fatalf("status %d body %s", resp.StatusCode, resp.Body)
		}
		if resp.Headers["Retry-After"] == "" || resp.Headers["Retry-After"] == "0" {
			t.Errorf("Retry-After = %q", resp.Headers["Retry-After"])
		}
		if e.planner.calls() != 3 {
			t.Errorf("the model ran %d times, want 3", e.planner.calls())
		}
	})
	t.Run("the allowance is per user", func(t *testing.T) {
		e := newSPEnv(t)
		e.h.StoryPlansPerHour = 2
		create(e, "user1")
		create(e, "user1")
		if resp := create(e, "user1"); resp.StatusCode != 429 {
			t.Fatalf("user1: status %d", resp.StatusCode)
		}
		if resp := create(e, "user2"); resp.StatusCode != 201 {
			t.Errorf("another user was blocked: status %d", resp.StatusCode)
		}
	})
	t.Run("plans leave the window after an hour", func(t *testing.T) {
		e := newSPEnv(t)
		e.h.StoryPlansPerHour = 2
		create(e, "user1")
		create(e, "user1")
		if resp := create(e, "user1"); resp.StatusCode != 429 {
			t.Fatalf("status %d", resp.StatusCode)
		}
		e.now = e.now.Add(61 * time.Minute)
		if resp := e.do(t, "POST", "/story-plans", storyBody(spEnglish), "user1"); resp.StatusCode != 201 {
			t.Errorf("after the window: status %d %s", resp.StatusCode, resp.Body)
		}
	})
	t.Run("Retry-After points at when the oldest plan leaves the window", func(t *testing.T) {
		e := newSPEnv(t)
		e.h.StoryPlansPerHour = 2
		create(e, "user1") // created at T+1s
		e.now = e.now.Add(20 * time.Minute)
		create(e, "user1")
		e.now = e.now.Add(10 * time.Minute)
		resp := e.do(t, "POST", "/story-plans", storyBody(spEnglish), "user1")
		// The oldest plan is ~31 minutes old, so about 29 minutes remain.
		var secs int
		fmt.Sscan(resp.Headers["Retry-After"], &secs)
		if resp.StatusCode != 429 || secs < 28*60 || secs > 30*60 {
			t.Errorf("status %d Retry-After %ds, want about 29 minutes", resp.StatusCode, secs)
		}
	})
	t.Run("the default is twenty, and the setting is bounded", func(t *testing.T) {
		e := newSPEnv(t)
		for set, want := range map[int]int{0: 20, -5: 20, 1: 1, 50: 50, 1000: 1000, 1001: 1000, 1 << 30: 1000} {
			e.h.StoryPlansPerHour = set
			if got := e.h.storyPlansPerHour(); got != want {
				t.Errorf("StoryPlansPerHour=%d gives %d, want %d", set, got, want)
			}
		}
	})
}

func TestStoryPlans_RateLimitQueryIsABoundedKeyRange(t *testing.T) {
	e := newSPEnv(t)
	e.h.StoryPlansPerHour = 5
	e.do(t, "POST", "/story-plans", storyBody(spEnglish), "user1")
	q := e.queries[0]
	if aws.ToString(q.KeyConditionExpression) != "PK = :pk AND SK BETWEEN :lo AND :hi" {
		t.Errorf("key condition = %q", aws.ToString(q.KeyConditionExpression))
	}
	if aws.ToInt32(q.Limit) != 5 {
		t.Errorf("Limit = %d: the check must read no more than the allowance", aws.ToInt32(q.Limit))
	}
	if q.IndexName != nil || q.FilterExpression != nil {
		t.Error("the check must be a plain key-range query: no index, no filter")
	}
	lo := q.ExpressionAttributeValues[":lo"].(*dbtypes.AttributeValueMemberS).Value
	if !strings.HasPrefix(lo, "STORYPLAN#") || len(lo) != len("STORYPLAN#")+13 {
		t.Errorf("lower bound = %q", lo)
	}
}

func TestStoryPlans_FailsClosedWhenTheAllowanceCannotBeChecked(t *testing.T) {
	e := newSPEnv(t)
	e.qErr = errors.New("dynamodb is down")
	resp := e.do(t, "POST", "/story-plans", storyBody(spEnglish), "user1")
	if resp.StatusCode != 500 || decodeErr(t, resp).Code != "DB_ERROR" {
		t.Errorf("status %d body %s", resp.StatusCode, resp.Body)
	}
	if e.planner.calls() != 0 {
		t.Error("the model ran although the allowance could not be checked")
	}
	if strings.Contains(resp.Body, "dynamodb is down") {
		t.Error("the database error leaked to the client")
	}
}

func TestStoryPlans_PendingWriteFailureStopsBeforeTheModel(t *testing.T) {
	e := newSPEnv(t)
	e.putErr = func(n int) error { return errors.New("write failed") }
	resp := e.do(t, "POST", "/story-plans", storyBody(spEnglish), "user1")
	if resp.StatusCode != 500 || e.planner.calls() != 0 {
		t.Errorf("status %d, planner calls %d: with no record the model must not run", resp.StatusCode, e.planner.calls())
	}
}

func TestStoryPlans_SaveFailureAfterGenerationIsReported(t *testing.T) {
	e := newSPEnv(t)
	e.putErr = func(n int) error {
		if n == 2 {
			return errors.New("write failed")
		}
		return nil
	}
	resp := e.do(t, "POST", "/story-plans", storyBody(spEnglish), "user1")
	if resp.StatusCode != 500 || !strings.Contains(decodeErr(t, resp).Message, "could not be saved") {
		t.Errorf("status %d body %s", resp.StatusCode, resp.Body)
	}
}

func TestStoryPlans_FailureRecordIsWrittenEvenIfTheRequestWasCancelled(t *testing.T) {
	e := newSPEnv(t)
	ctx, cancel := context.WithCancel(context.Background())
	e.planner.err = context.Canceled
	e.planner.hook = cancel // the client goes away while the model runs
	var finalCtxErr error
	e.h.DB.(*mockDynamoDB).putItemFunc = func(c context.Context, in *dynamodb.PutItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
		finalCtxErr = c.Err()
		e.mu.Lock()
		e.puts = append(e.puts, in.Item)
		e.mu.Unlock()
		return &dynamodb.PutItemOutput{}, nil
	}
	_, _ = e.h.HandleRequest(ctx, events.APIGatewayProxyRequest{
		HTTPMethod: "POST", Path: "/story-plans", Body: storyBody(spEnglish),
		Headers: map[string]string{"Authorization": "Bearer " + generateTestToken("user1")},
	})
	if len(e.puts) != 2 {
		t.Fatalf("%d writes, want pending + failed", len(e.puts))
	}
	if finalCtxErr != nil {
		t.Errorf("the failure record was written with a cancelled context: %v", finalCtxErr)
	}
	if e.puts[1]["status"].(*dbtypes.AttributeValueMemberS).Value != "failed" {
		t.Error("the attempt was left looking pending")
	}
}

func TestStoryPlans_GetPlan(t *testing.T) {
	e := newSPEnv(t)
	id := decodePlan(t, e.do(t, "POST", "/story-plans", storyBody(spEnglish), "user1")).PlanID

	t.Run("owner reads it back", func(t *testing.T) {
		resp := e.do(t, "GET", "/story-plans/"+id, "", "user1")
		got := decodePlan(t, resp)
		if resp.StatusCode != 200 || got.PlanID != id || got.Status != "ready" || got.Plan == nil || got.Plan.TotalSeconds != 11 {
			t.Errorf("status %d body %s", resp.StatusCode, resp.Body)
		}
	})
	t.Run("upper case id finds the same plan", func(t *testing.T) {
		if resp := e.do(t, "GET", "/story-plans/"+strings.ToUpper(id), "", "user1"); resp.StatusCode != 200 {
			t.Errorf("status %d", resp.StatusCode)
		}
	})
	t.Run("another user gets 404, never the plan and never a 403", func(t *testing.T) {
		resp := e.do(t, "GET", "/story-plans/"+id, "", "user2")
		if resp.StatusCode != 404 || decodeErr(t, resp).Code != "NOT_FOUND" {
			t.Errorf("status %d body %s: reading another user's plan must look like a missing one", resp.StatusCode, resp.Body)
		}
	})
	t.Run("unknown id", func(t *testing.T) {
		if resp := e.do(t, "GET", "/story-plans/"+uuid.NewString(), "", "user1"); resp.StatusCode != 404 {
			t.Errorf("status %d", resp.StatusCode)
		}
	})
	t.Run("malformed ids never reach the database", func(t *testing.T) {
		before := len(e.queries)
		for _, bad := range []string{"abc", "1", "../../etc", "{" + id + "}", "urn:uuid:" + id, strings.ReplaceAll(id, "-", ""), id + "x", "%00", "SHORTS%23" + id} {
			if resp := e.do(t, "GET", "/story-plans/"+bad, "", "user1"); resp.StatusCode != 404 {
				t.Errorf("id %q: status %d, want 404", bad, resp.StatusCode)
			}
		}
		if len(e.queries) != before {
			t.Error("unexpected queries")
		}
	})
	t.Run("database error is a 500 without detail", func(t *testing.T) {
		e.getErr = errors.New("table exploded")
		defer func() { e.getErr = nil }()
		resp := e.do(t, "GET", "/story-plans/"+id, "", "user1")
		if resp.StatusCode != 500 || strings.Contains(resp.Body, "exploded") {
			t.Errorf("status %d body %s", resp.StatusCode, resp.Body)
		}
	})
	t.Run("only story plans are readable through this route", func(t *testing.T) {
		// A shorts job lives under the same partition but a different prefix.
		e.items["USER#user1|SHORTS#"+id] = map[string]dbtypes.AttributeValue{"PK": sv("USER#user1"), "SK": sv("SHORTS#" + id)}
		resp := e.do(t, "GET", "/story-plans/"+id, "", "user1")
		if resp.StatusCode != 200 || decodePlan(t, resp).Status != "ready" {
			t.Errorf("status %d", resp.StatusCode)
		}
	})
}

func TestStoryPlans_OtherRoutesAndMethods(t *testing.T) {
	e := newSPEnv(t)
	for _, tc := range []struct {
		method, path string
		want         int
	}{
		{"DELETE", "/story-plans/" + uuid.NewString(), 404},
		{"PUT", "/story-plans/" + uuid.NewString(), 404},
		{"GET", "/story-plans", 404},
		{"GET", "/story-plans/", 404},
		{"POST", "/story-plans/" + uuid.NewString(), 404},
		{"GET", "/story-plans/" + uuid.NewString() + "/extra", 404},
		{"GET", "/story-plansX", 404},
	} {
		if resp := e.do(t, tc.method, tc.path, "", "user1"); resp.StatusCode != tc.want {
			t.Errorf("%s %s: status %d, want %d", tc.method, tc.path, resp.StatusCode, tc.want)
		}
	}
	if e.planner.calls() != 0 {
		t.Error("the planner ran for a route that should not exist")
	}
}

func TestStoryPlans_ResponsesAreJSONWithCORS(t *testing.T) {
	e := newSPEnv(t)
	resp := e.do(t, "POST", "/story-plans", storyBody(spEnglish), "user1")
	if resp.Headers["Content-Type"] != "application/json" || resp.Headers["Access-Control-Allow-Origin"] != "https://indifferent.fun" {
		t.Errorf("headers = %v", resp.Headers)
	}
}

func TestNewStoryPlanID(t *testing.T) {
	now := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	id, err := newStoryPlanID(now)
	if err != nil {
		t.Fatal(err)
	}
	u, err := uuid.Parse(id)
	if err != nil || u.Version() != 7 || u.Variant() != uuid.RFC4122 || len(id) != 36 || id != strings.ToLower(id) {
		t.Fatalf("id %q: err=%v version=%v variant=%v", id, err, u.Version(), u.Variant())
	}
	// The first 12 hex digits are the millisecond timestamp.
	var ms int64
	fmt.Sscanf(id[:8]+id[9:13], "%x", &ms)
	if ms != now.UnixMilli() {
		t.Errorf("embedded time %d, want %d", ms, now.UnixMilli())
	}
	// Distinct even within one millisecond.
	seen := map[string]bool{}
	for i := 0; i < 500; i++ {
		x, _ := newStoryPlanID(now)
		if seen[x] {
			t.Fatalf("duplicate id %s within one millisecond", x)
		}
		seen[x] = true
	}
	if !validStoryPlanID(id) {
		t.Error("a generated id must pass the handler's own id check")
	}
}

func TestUUIDv7Floor(t *testing.T) {
	// Every id made at or after t sorts at or after the floor of t, and every id
	// made before sorts before it. The allowance query depends on both.
	base := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	floor := uuidV7Floor(base)
	if len(floor) != 13 || floor[8] != '-' {
		t.Fatalf("floor = %q", floor)
	}
	for i := 0; i < 300; i++ {
		after, _ := newStoryPlanID(base.Add(time.Duration(i) * time.Millisecond))
		before, _ := newStoryPlanID(base.Add(-time.Duration(i+1) * time.Millisecond))
		if after[:13] < floor {
			t.Fatalf("id %s made at/after the boundary sorts below %s", after, floor)
		}
		if before[:13] >= floor {
			t.Fatalf("id %s made before the boundary does not sort below %s", before, floor)
		}
	}
	// The key-range upper bound ("~") sorts after every possible id.
	top, _ := newStoryPlanID(time.Date(2200, 1, 1, 0, 0, 0, 0, time.UTC))
	if !(top < "~") {
		t.Error("the range's upper bound must be above every id")
	}
	if uuidV7Floor(time.Unix(-5, 0)) != "00000000-0000" {
		t.Error("a time before 1970 clamps to zero")
	}
	if uuidV7Floor(base.Add(time.Millisecond)) <= uuidV7Floor(base) {
		t.Error("later times must have later floors")
	}
}

func TestStoryPlanRecordRoundTrip(t *testing.T) {
	plan := samplePlan("hi")
	in := storyPlanRecord{UserID: "u", PlanID: "p", Status: "ready", Model: "m", StoryRunes: 42, Script: "devanagari",
		LatencyMs: 4200, Plan: &plan, CreatedAt: "2026-10-30T12:00:00Z", UpdatedAt: "2026-10-30T12:00:05Z"}
	item, err := storyPlanToItem(in)
	if err != nil {
		t.Fatal(err)
	}
	out := itemToStoryPlan(item)
	out.UserID = "u"
	b1, _ := json.Marshal(in)
	b2, _ := json.Marshal(out)
	if string(b1) != string(b2) {
		t.Errorf("round trip changed the record:\n in  %s\n out %s", b1, b2)
	}
	// A corrupt stored plan is left out rather than failing the read.
	item["plan"] = &dbtypes.AttributeValueMemberS{Value: "{not json"}
	if got := itemToStoryPlan(item); got.Plan != nil || got.Status != "ready" {
		t.Errorf("corrupt plan: %+v", got)
	}
	// Hostile stored numbers are ignored, not wrapped or negated.
	for _, v := range []string{"-5", "99999999999999999999999", "1.5", "abc"} {
		item["storyRunes"] = &dbtypes.AttributeValueMemberN{Value: v}
		item["latencyMs"] = &dbtypes.AttributeValueMemberN{Value: v}
		if got := itemToStoryPlan(item); got.StoryRunes != 0 || got.LatencyMs != 0 {
			t.Errorf("stored %q read back as %d / %d, want 0", v, got.StoryRunes, got.LatencyMs)
		}
	}
	// Wrong attribute types are ignored.
	item["storyRunes"] = &dbtypes.AttributeValueMemberS{Value: "x"}
	item["status"] = &dbtypes.AttributeValueMemberN{Value: "1"}
	if got := itemToStoryPlan(item); got.StoryRunes != 0 || got.Status != "" {
		t.Errorf("wrong types: %+v", got)
	}
}

func TestStoryPlans_ConcurrentCreatesAreSafe(t *testing.T) {
	// Run with -race.
	e := newSPEnv(t)
	e.h.StoryPlansPerHour = 1000
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if resp := e.do(t, "POST", "/story-plans", storyBody(spEnglish), "user1"); resp.StatusCode != 201 {
				t.Errorf("status %d", resp.StatusCode)
			}
		}()
	}
	wg.Wait()
}
