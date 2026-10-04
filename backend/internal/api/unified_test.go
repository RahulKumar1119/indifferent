package api

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/rahul/indifferent/backend/internal/auth"
	"github.com/rahul/indifferent/backend/internal/models"
	"github.com/rahul/indifferent/backend/internal/storage"
)

// captureDB records PutItems keyed by PK|SK for unified-project assertions.
type captureDB struct {
	mockDynamoDB
	items map[string]map[string]dbtypes.AttributeValue
}

func strVal(item map[string]dbtypes.AttributeValue, key string) string {
	if v, ok := item[key].(*dbtypes.AttributeValueMemberS); ok {
		return v.Value
	}
	return ""
}

func newCaptureDB() *captureDB {
	c := &captureDB{items: map[string]map[string]dbtypes.AttributeValue{}}
	c.putItemFunc = func(ctx context.Context, params *dynamodb.PutItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
		c.items[strVal(params.Item, "PK")+"|"+strVal(params.Item, "SK")] = params.Item
		return &dynamodb.PutItemOutput{}, nil
	}
	c.getItemFunc = func(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
		if item, ok := c.items[strVal(params.Key, "PK")+"|"+strVal(params.Key, "SK")]; ok {
			return &dynamodb.GetItemOutput{Item: item}, nil
		}
		return &dynamodb.GetItemOutput{}, nil
	}
	return c
}

func newUnifiedTestHandler(db *captureDB) *APIHandler {
	return &APIHandler{
		AuthService:     &mockAuthService{},
		JWTService:      &auth.JWTService{Secret: testSecret, DB: db, SessionTable: "sessions-table"},
		DB:              db,
		S3:              &stubDownloader{status: storage.ObjectStatus{StorageClass: "STANDARD"}},
		SFN:             &mockSFN{},
		TableName:       "projects-table",
		Bucket:          "test-bucket",
		StateMachineARN: "arn:aws:states:us-east-1:123456789:stateMachine:test",
	}
}

func TestCreateProject_Unified(t *testing.T) {
	db := newCaptureDB()
	h := newUnifiedTestHandler(db)
	ctx := context.Background()
	token := generateTestToken("user1")

	resp, _ := h.HandleRequest(ctx, events.APIGatewayProxyRequest{
		HTTPMethod: "POST",
		Path:       "/projects",
		Headers:    map[string]string{"Authorization": "Bearer " + token},
		Body:       `{"name":"Launch","template":"classic","voice":"Joanna","watermark":{"text":"© me"},"shorts":{"fileType":"mp4","duration":120}}`,
	})
	if resp.StatusCode != 201 {
		t.Fatalf("got status %d, want 201 (body: %s)", resp.StatusCode, resp.Body)
	}
	var body struct {
		ID           string `json:"id"`
		Watermark    *models.WatermarkSettings `json:"watermark"`
		ShortsJobIDs []string `json:"shortsJobIds"`
		Shorts       *struct {
			JobID     string `json:"jobId"`
			UploadURL string `json:"uploadUrl"`
		} `json:"shorts"`
	}
	if err := json.Unmarshal([]byte(resp.Body), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body.Watermark == nil || body.Watermark.Text != "© me" {
		t.Errorf("expected watermark round-trip, got %+v", body.Watermark)
	}
	if body.Shorts == nil || body.Shorts.JobID == "" || body.Shorts.UploadURL == "" {
		t.Fatalf("expected linked shorts upload, got %+v", body.Shorts)
	}
	if len(body.ShortsJobIDs) != 1 || body.ShortsJobIDs[0] != body.Shorts.JobID {
		t.Errorf("expected project linked to job, got %v", body.ShortsJobIDs)
	}

	// The stored job carries the project link back.
	jobItem, ok := db.items["USER#user1|SHORTS#"+body.Shorts.JobID]
	if !ok {
		t.Fatalf("linked shorts job not stored (keys: %v)", keysOf(db.items))
	}
	if strVal(jobItem, "projectId") != body.ID {
		t.Errorf("expected job.projectId %q, got %q", body.ID, strVal(jobItem, "projectId"))
	}
}

func TestCreateShorts_LinkedToProject(t *testing.T) {
	db := newCaptureDB()
	h := newUnifiedTestHandler(db)
	ctx := context.Background()
	token := generateTestToken("user1")

	// Seed a project owned by user1.
	resp, _ := h.HandleRequest(ctx, events.APIGatewayProxyRequest{
		HTTPMethod: "POST",
		Path:       "/projects",
		Headers:    map[string]string{"Authorization": "Bearer " + token},
		Body:       `{"name":"P","template":"classic","voice":"Joanna"}`,
	})
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal([]byte(resp.Body), &created); err != nil || created.ID == "" {
		t.Fatalf("seed project failed: %s", resp.Body)
	}

	resp, _ = h.HandleRequest(ctx, events.APIGatewayProxyRequest{
		HTTPMethod: "POST",
		Path:       "/shorts",
		Headers:    map[string]string{"Authorization": "Bearer " + token},
		Body:       `{"fileType":"mp4","duration":60,"projectId":"` + created.ID + `"}`,
	})
	if resp.StatusCode != 201 {
		t.Fatalf("got status %d, want 201 (body: %s)", resp.StatusCode, resp.Body)
	}
	var job struct {
		JobID string `json:"jobId"`
	}
	if err := json.Unmarshal([]byte(resp.Body), &job); err != nil || job.JobID == "" {
		t.Fatalf("no job id: %s", resp.Body)
	}

	// Project re-stored with the link.
	found := false
	for key, item := range db.items {
		if strings.HasPrefix(key, "USER#user1|PROJECT#") && strings.Contains(strVal(item, "shortsJobIds"), job.JobID) {
			found = true
		}
	}
	if !found {
		t.Errorf("project was not linked to job %s", job.JobID)
	}
}

func TestCreateShorts_UnknownProject(t *testing.T) {
	db := newCaptureDB()
	h := newUnifiedTestHandler(db)
	ctx := context.Background()
	token := generateTestToken("user1")

	resp, _ := h.HandleRequest(ctx, events.APIGatewayProxyRequest{
		HTTPMethod: "POST",
		Path:       "/shorts",
		Headers:    map[string]string{"Authorization": "Bearer " + token},
		Body:       `{"fileType":"mp4","duration":60,"projectId":"nope"}`,
	})
	if resp.StatusCode != 404 {
		t.Errorf("got status %d, want 404 (body: %s)", resp.StatusCode, resp.Body)
	}
}

func keysOf(items map[string]map[string]dbtypes.AttributeValue) []string {
	out := make([]string, 0, len(items))
	for k := range items {
		out = append(out, k)
	}
	return out
}

func TestProjectBranding_RoundTrip(t *testing.T) {
	db := newCaptureDB()
	h := newUnifiedTestHandler(db)
	ctx := context.Background()
	token := generateTestToken("user1")

	// Create with a channel handle.
	resp, _ := h.HandleRequest(ctx, events.APIGatewayProxyRequest{
		HTTPMethod: "POST",
		Path:       "/projects",
		Headers:    map[string]string{"Authorization": "Bearer " + token},
		Body:       `{"name":"B","template":"classic","voice":"Joanna","branding":{"channelName":"@studio"}}`,
	})
	if resp.StatusCode != 201 {
		t.Fatalf("got status %d, want 201 (body: %s)", resp.StatusCode, resp.Body)
	}
	var created struct {
		ID       string `json:"id"`
		Branding *models.Branding `json:"branding"`
	}
	if err := json.Unmarshal([]byte(resp.Body), &created); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if created.Branding == nil || created.Branding.ChannelName != "@studio" {
		t.Fatalf("expected branding round-trip, got %+v", created.Branding)
	}

	// Logo upload URL records the key on the project.
	resp, _ = h.HandleRequest(ctx, events.APIGatewayProxyRequest{
		HTTPMethod: "POST",
		Path:       "/projects/" + created.ID + "/logo",
		Headers:    map[string]string{"Authorization": "Bearer " + token},
	})
	if resp.StatusCode != 200 {
		t.Fatalf("logo endpoint status %d, want 200 (body: %s)", resp.StatusCode, resp.Body)
	}
	var logo struct {
		UploadURL string `json:"uploadUrl"`
		LogoKey   string `json:"logoKey"`
	}
	if err := json.Unmarshal([]byte(resp.Body), &logo); err != nil || logo.UploadURL == "" || logo.LogoKey == "" {
		t.Fatalf("expected upload URL + key, got %s", resp.Body)
	}

	// Update the handle; overlong names rejected.
	resp, _ = h.HandleRequest(ctx, events.APIGatewayProxyRequest{
		HTTPMethod: "PUT",
		Path:       "/projects/" + created.ID,
		Headers:    map[string]string{"Authorization": "Bearer " + token},
		Body:       `{"channelName":"@newhandle"}`,
	})
	if resp.StatusCode != 200 {
		t.Fatalf("update status %d, want 200 (body: %s)", resp.StatusCode, resp.Body)
	}
	var updated models.Project
	if err := json.Unmarshal([]byte(resp.Body), &updated); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if updated.Branding == nil || updated.Branding.ChannelName != "@newhandle" || updated.Branding.LogoKey != logo.LogoKey {
		t.Errorf("expected handle update with logo preserved, got %+v", updated.Branding)
	}

	resp, _ = h.HandleRequest(ctx, events.APIGatewayProxyRequest{
		HTTPMethod: "PUT",
		Path:       "/projects/" + created.ID,
		Headers:    map[string]string{"Authorization": "Bearer " + token},
		Body:       `{"channelName":"` + strings.Repeat("x", 61) + `"}`,
	})
	if resp.StatusCode != 400 {
		t.Errorf("overlong handle: got %d, want 400 (body: %s)", resp.StatusCode, resp.Body)
	}
}
