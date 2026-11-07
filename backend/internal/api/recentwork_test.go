package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

func rwStr(v string) dbtypes.AttributeValue { return &dbtypes.AttributeValueMemberS{Value: v} }

func listCall(t *testing.T, path string, db *mockDynamoDB, token string) events.APIGatewayProxyResponse {
	t.Helper()
	h := newShortsTestHandler(t, db)
	req := events.APIGatewayProxyRequest{HTTPMethod: "GET", Path: path, Headers: map[string]string{}}
	if token != "" {
		req.Headers["Authorization"] = "Bearer " + token
	}
	resp, _ := h.HandleRequest(context.Background(), req)
	return resp
}

func queryReturning(captured *dynamodb.QueryInput, out *dynamodb.QueryOutput, err error) *mockDynamoDB {
	return &mockDynamoDB{
		queryFunc: func(_ context.Context, p *dynamodb.QueryInput, _ ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error) {
			if captured != nil {
				*captured = *p
			}
			return out, err
		},
	}
}

func planItem(id, status, updated, hook string) map[string]dbtypes.AttributeValue {
	it := map[string]dbtypes.AttributeValue{
		"PK": rwStr("USER#user1"), "SK": rwStr("STORYPLAN#" + id), "planId": rwStr(id), "status": rwStr(status),
		"createdAt": rwStr(updated), "updatedAt": rwStr(updated),
	}
	if hook != "" {
		it["plan"] = s(`{"language":"en","hook":"` + hook + `","scenes":[{},{}],"totalSeconds":42.5}`)
	}
	return it
}

func TestListStoryPlans(t *testing.T) {
	var q dynamodb.QueryInput
	db := queryReturning(&q, &dynamodb.QueryOutput{Items: []map[string]dbtypes.AttributeValue{
		planItem("p2", "ready", "2026-02-01T00:00:00Z", "A net caught light."),
		{"PK": rwStr("USER#user1"), "SK": rwStr("STORYPLAN#bad")}, // malformed: no id or status
		planItem("p1", "failed", "2026-01-01T00:00:00Z", ""),
	}}, nil)
	resp := listCall(t, "/story-plans", db, generateTestToken("user1"))
	if resp.StatusCode != 200 {
		t.Fatalf("status %d: %s", resp.StatusCode, resp.Body)
	}
	var body struct {
		Plans   []StoryPlanSummary `json:"plans"`
		HasMore bool               `json:"hasMore"`
	}
	if err := json.Unmarshal([]byte(resp.Body), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Plans) != 2 || body.Plans[0].PlanID != "p2" || body.Plans[1].PlanID != "p1" {
		t.Fatalf("plans: %+v", body.Plans)
	}
	if p := body.Plans[0]; p.Hook != "A net caught light." || p.SceneCount != 2 || p.TotalSeconds != 42.5 || p.Language != "en" {
		t.Errorf("summary: %+v", p)
	}
	if body.HasMore {
		t.Error("hasMore set for a short list")
	}
	// The key is built from the token, only story plans are read, newest first,
	// and the user's story text is never fetched.
	if got := q.ExpressionAttributeValues[":pk"].(*dbtypes.AttributeValueMemberS).Value; got != "USER#user1" {
		t.Errorf("pk %q", got)
	}
	if got := q.ExpressionAttributeValues[":sk"].(*dbtypes.AttributeValueMemberS).Value; got != "STORYPLAN#" {
		t.Errorf("sk prefix %q", got)
	}
	if q.ScanIndexForward == nil || *q.ScanIndexForward {
		t.Error("not newest first")
	}
	for _, n := range q.ExpressionAttributeNames {
		if n == "script" {
			t.Error("script is projected")
		}
	}
	if strings.Contains(resp.Body, "script") {
		t.Error("response mentions script")
	}
}

func TestListStoryPlans_CapsAtTwentyAndReportsMore(t *testing.T) {
	var items []map[string]dbtypes.AttributeValue
	for i := 0; i < 25; i++ {
		items = append(items, planItem(fmt.Sprintf("p%02d", i), "ready", "2026-01-01T00:00:00Z", "h"))
	}
	resp := listCall(t, "/story-plans", queryReturning(nil, &dynamodb.QueryOutput{Items: items}, nil), generateTestToken("user1"))
	var body struct {
		Plans   []StoryPlanSummary `json:"plans"`
		HasMore bool               `json:"hasMore"`
	}
	_ = json.Unmarshal([]byte(resp.Body), &body)
	if len(body.Plans) != 20 || !body.HasMore {
		t.Fatalf("got %d plans, hasMore=%v", len(body.Plans), body.HasMore)
	}
}

func TestListStoryPlans_LongHookIsCut(t *testing.T) {
	hook := strings.Repeat("é", 500)
	resp := listCall(t, "/story-plans", queryReturning(nil, &dynamodb.QueryOutput{Items: []map[string]dbtypes.AttributeValue{planItem("p", "ready", "x", hook)}}, nil), generateTestToken("user1"))
	var body struct {
		Plans []StoryPlanSummary `json:"plans"`
	}
	_ = json.Unmarshal([]byte(resp.Body), &body)
	if n := len([]rune(body.Plans[0].Hook)); n != planHookMaxRunes {
		t.Errorf("hook has %d runes", n)
	}
}

func thumbItem(job, status, updated, thumbs, selected string) map[string]dbtypes.AttributeValue {
	it := map[string]dbtypes.AttributeValue{
		"SK": rwStr("THUMBS#" + job), "status": rwStr(status), "createdAt": rwStr("2026-01-01T00:00:00Z"), "updatedAt": rwStr(updated),
	}
	if thumbs != "" {
		it["thumbnails"] = rwStr(thumbs)
	}
	if selected != "" {
		it["selectedId"] = rwStr(selected)
	}
	return it
}

func TestListThumbnails(t *testing.T) {
	var q dynamodb.QueryInput
	db := queryReturning(&q, &dynamodb.QueryOutput{Items: []map[string]dbtypes.AttributeValue{
		thumbItem("old", "completed", "2026-01-01T00:00:00Z", `[{"id":"a"},{"id":"b"},{"id":"c"}]`, "a"),
		thumbItem("new", "running", "2026-03-01T00:00:00Z", "", ""),
		{"SK": rwStr("THUMBS#nostatus")},
		{"SK": rwStr("OTHER#x"), "status": rwStr("completed")},
	}}, nil)
	resp := listCall(t, "/thumbnails", db, generateTestToken("user1"))
	if resp.StatusCode != 200 {
		t.Fatalf("status %d: %s", resp.StatusCode, resp.Body)
	}
	var body struct {
		Thumbnails []ThumbnailSetSummary `json:"thumbnails"`
		HasMore    bool                  `json:"hasMore"`
	}
	_ = json.Unmarshal([]byte(resp.Body), &body)
	if len(body.Thumbnails) != 2 || body.Thumbnails[0].JobID != "new" || body.Thumbnails[1].JobID != "old" {
		t.Fatalf("sets: %+v", body.Thumbnails)
	}
	if o := body.Thumbnails[1]; o.ThumbnailCount != 3 || !o.Selected {
		t.Errorf("old: %+v", o)
	}
	if got := q.ExpressionAttributeValues[":sk"].(*dbtypes.AttributeValueMemberS).Value; got != "THUMBS#" {
		t.Errorf("sk prefix %q", got)
	}
	if got := q.ExpressionAttributeValues[":pk"].(*dbtypes.AttributeValueMemberS).Value; got != "USER#user1" {
		t.Errorf("pk %q", got)
	}
}

func TestListThumbnails_CapsAtTwenty(t *testing.T) {
	var items []map[string]dbtypes.AttributeValue
	for i := 0; i < 30; i++ {
		items = append(items, thumbItem(fmt.Sprintf("j%02d", i), "completed", fmt.Sprintf("2026-01-%02dT00:00:00Z", i+1), "", ""))
	}
	resp := listCall(t, "/thumbnails", queryReturning(nil, &dynamodb.QueryOutput{Items: items}, nil), generateTestToken("user1"))
	var body struct {
		Thumbnails []ThumbnailSetSummary `json:"thumbnails"`
		HasMore    bool                  `json:"hasMore"`
	}
	_ = json.Unmarshal([]byte(resp.Body), &body)
	if len(body.Thumbnails) != 20 || !body.HasMore || body.Thumbnails[0].JobID != "j29" {
		t.Fatalf("got %d, hasMore=%v, first=%s", len(body.Thumbnails), body.HasMore, body.Thumbnails[0].JobID)
	}
}

func TestRecentWorkLists_NeedSignInAndSurfaceDBErrors(t *testing.T) {
	for _, path := range []string{"/story-plans", "/thumbnails"} {
		called := false
		db := &mockDynamoDB{queryFunc: func(context.Context, *dynamodb.QueryInput, ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error) {
			called = true
			return &dynamodb.QueryOutput{}, nil
		}}
		if resp := listCall(t, path, db, ""); resp.StatusCode != 401 || called {
			t.Errorf("%s without a token: %d, db called=%v", path, resp.StatusCode, called)
		}
		if resp := listCall(t, path, queryReturning(nil, nil, errors.New("boom")), generateTestToken("user1")); resp.StatusCode != 500 {
			t.Errorf("%s on DB error: %d", path, resp.StatusCode)
		}
	}
}
