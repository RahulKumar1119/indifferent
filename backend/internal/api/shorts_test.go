package api

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/rahul/indifferent/backend/internal/auth"
	"github.com/rahul/indifferent/backend/internal/models"
	"github.com/rahul/indifferent/backend/internal/storage"
)

func newShortsTestHandler(t *testing.T, db *mockDynamoDB) *APIHandler {
	t.Helper()
	s3Client, err := storage.NewS3ClientWithRegion(context.Background(), "us-east-1")
	if err != nil {
		t.Fatalf("failed to create S3 client: %v", err)
	}
	return &APIHandler{
		AuthService:           &mockAuthService{},
		JWTService:            &auth.JWTService{Secret: testSecret, DB: db, SessionTable: "sessions-table"},
		DB:                    db,
		S3:                    s3Client,
		SFN:                   &mockSFN{},
		TableName:             "table",
		Bucket:                "test-bucket",
		StateMachineARN:       "arn:aws:states:us-east-1:123456789:stateMachine:test",
		ShortsStateMachineARN: "arn:aws:states:us-east-1:123456789:stateMachine:shorts",
	}
}

func TestValidateCreateShorts(t *testing.T) {
	tests := []struct {
		name    string
		req     CreateShortsRequest
		wantErr bool
	}{
		{"valid mp4", CreateShortsRequest{FileType: "mp4", Duration: 120}, false},
		{"valid mov uppercase", CreateShortsRequest{FileType: "MOV", Duration: 60}, false},
		{"valid mp3", CreateShortsRequest{FileType: "mp3", Duration: 300}, false},
		{"valid wav", CreateShortsRequest{FileType: "wav", Duration: 599}, false},
		{"unsupported type", CreateShortsRequest{FileType: "avi", Duration: 60}, true},
		{"empty type", CreateShortsRequest{FileType: "", Duration: 60}, true},
		{"zero duration", CreateShortsRequest{FileType: "mp4", Duration: 0}, true},
		{"negative duration", CreateShortsRequest{FileType: "mp4", Duration: -5}, true},
		{"duration over max", CreateShortsRequest{FileType: "mp4", Duration: 601}, true},
		{"duration at max", CreateShortsRequest{FileType: "mp4", Duration: 600}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCreateShorts(&tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateCreateShorts() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCreateShorts_ValidationResponse(t *testing.T) {
	db := &mockDynamoDB{}
	h := newShortsTestHandler(t, db)
	ctx := context.Background()
	token := generateTestToken("user1")

	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantCode   string
	}{
		{"invalid body", "", 400, "INVALID_BODY"},
		{"unsupported type", `{"fileType":"avi","duration":60}`, 400, "VALIDATION_ERROR"},
		{"over duration", `{"fileType":"mp4","duration":700}`, 400, "VALIDATION_ERROR"},
		{"valid", `{"fileType":"mp4","duration":60}`, 201, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := h.HandleRequest(ctx, events.APIGatewayProxyRequest{
				HTTPMethod: "POST",
				Path:       "/shorts",
				Headers:    map[string]string{"Authorization": "Bearer " + token},
				Body:       tt.body,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if resp.StatusCode != tt.wantStatus {
				t.Errorf("got status %d, want %d (body: %s)", resp.StatusCode, tt.wantStatus, resp.Body)
			}
			if tt.wantCode != "" {
				var apiErr models.APIError
				_ = json.Unmarshal([]byte(resp.Body), &apiErr)
				if apiErr.Code != tt.wantCode {
					t.Errorf("got error code %q, want %q", apiErr.Code, tt.wantCode)
				}
			}
		})
	}
}

func TestCreateShorts_WritesUploadedJob(t *testing.T) {
	var putItem map[string]dbtypes.AttributeValue
	db := &mockDynamoDB{
		putItemFunc: func(ctx context.Context, params *dynamodb.PutItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
			putItem = params.Item
			return &dynamodb.PutItemOutput{}, nil
		},
	}
	h := newShortsTestHandler(t, db)
	ctx := context.Background()
	token := generateTestToken("user1")

	resp, err := h.HandleRequest(ctx, events.APIGatewayProxyRequest{
		HTTPMethod: "POST",
		Path:       "/shorts",
		Headers:    map[string]string{"Authorization": "Bearer " + token},
		Body:       `{"fileType":"mp4","duration":90}`,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 201 {
		t.Fatalf("got status %d, want 201 (body: %s)", resp.StatusCode, resp.Body)
	}
	if putItem == nil {
		t.Fatal("expected PutItem to be called")
	}
	if got := putItem["status"].(*dbtypes.AttributeValueMemberS).Value; got != "uploaded" {
		t.Errorf("stored status %q, want uploaded", got)
	}
	pk := putItem["PK"].(*dbtypes.AttributeValueMemberS).Value
	if pk != "USER#user1" {
		t.Errorf("stored PK %q, want USER#user1", pk)
	}

	var created CreateShortsResponse
	if err := json.Unmarshal([]byte(resp.Body), &created); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if created.JobID == "" || created.UploadURL == "" || created.SourceKey == "" {
		t.Errorf("incomplete response: %+v", created)
	}
}

func TestGetShorts_NotFound(t *testing.T) {
	db := &mockDynamoDB{
		getItemFunc: func(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
			return &dynamodb.GetItemOutput{Item: nil}, nil
		},
	}
	h := newShortsTestHandler(t, db)
	ctx := context.Background()
	token := generateTestToken("user1")

	resp, _ := h.HandleRequest(ctx, events.APIGatewayProxyRequest{
		HTTPMethod: "GET",
		Path:       "/shorts/missing",
		Headers:    map[string]string{"Authorization": "Bearer " + token},
	})
	if resp.StatusCode != 404 {
		t.Errorf("got status %d, want 404", resp.StatusCode)
	}
}

func TestStartShorts_OwnershipAndStart(t *testing.T) {
	// Owner exists; non-owner partition returns nothing.
	db := &mockDynamoDB{
		getItemFunc: func(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
			pk := params.Key["PK"].(*dbtypes.AttributeValueMemberS).Value
			if pk == "USER#user1" {
				return &dynamodb.GetItemOutput{
					Item: map[string]dbtypes.AttributeValue{
						"PK":             &dbtypes.AttributeValueMemberS{Value: "USER#user1"},
						"SK":             &dbtypes.AttributeValueMemberS{Value: "SHORTS#job1"},
						"jobId":          &dbtypes.AttributeValueMemberS{Value: "job1"},
						"status":         &dbtypes.AttributeValueMemberS{Value: "uploaded"},
						"fileType":       &dbtypes.AttributeValueMemberS{Value: "mp4"},
						"sourceDuration": &dbtypes.AttributeValueMemberN{Value: "120"},
						"sourceKey":      &dbtypes.AttributeValueMemberS{Value: "uploads/user1/job1/source.mp4"},
						"createdAt":      &dbtypes.AttributeValueMemberS{Value: "2024-01-01T00:00:00Z"},
						"updatedAt":      &dbtypes.AttributeValueMemberS{Value: "2024-01-01T00:00:00Z"},
					},
				}, nil
			}
			return &dynamodb.GetItemOutput{Item: nil}, nil
		},
	}
	h := newShortsTestHandler(t, db)
	ctx := context.Background()

	t.Run("owner starts pipeline", func(t *testing.T) {
		resp, _ := h.HandleRequest(ctx, events.APIGatewayProxyRequest{
			HTTPMethod: "POST",
			Path:       "/shorts/job1/start",
			Headers:    map[string]string{"Authorization": "Bearer " + generateTestToken("user1")},
		})
		if resp.StatusCode != 200 {
			t.Errorf("got status %d, want 200 (body: %s)", resp.StatusCode, resp.Body)
		}
	})

	t.Run("non-owner gets 404", func(t *testing.T) {
		resp, _ := h.HandleRequest(ctx, events.APIGatewayProxyRequest{
			HTTPMethod: "POST",
			Path:       "/shorts/job1/start",
			Headers:    map[string]string{"Authorization": "Bearer " + generateTestToken("user2")},
		})
		if resp.StatusCode != 404 {
			t.Errorf("got status %d, want 404", resp.StatusCode)
		}
	})
}

func TestListClips_OrderedByRank(t *testing.T) {
	clips := []models.Clip{
		{ClipID: "c3", S3Key: "shorts/user1/job1/clips/c3.mp4", Rank: 3, Score: 0.5},
		{ClipID: "c1", S3Key: "shorts/user1/job1/clips/c1.mp4", Rank: 1, Score: 0.9},
		{ClipID: "c2", S3Key: "shorts/user1/job1/clips/c2.mp4", Rank: 2, Score: 0.7},
	}
	clipsJSON, _ := json.Marshal(clips)
	db := &mockDynamoDB{
		getItemFunc: func(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
			return &dynamodb.GetItemOutput{
				Item: map[string]dbtypes.AttributeValue{
					"PK":             &dbtypes.AttributeValueMemberS{Value: "USER#user1"},
					"SK":             &dbtypes.AttributeValueMemberS{Value: "SHORTS#job1"},
					"jobId":          &dbtypes.AttributeValueMemberS{Value: "job1"},
					"status":         &dbtypes.AttributeValueMemberS{Value: "completed"},
					"fileType":       &dbtypes.AttributeValueMemberS{Value: "mp4"},
					"sourceDuration": &dbtypes.AttributeValueMemberN{Value: "120"},
					"sourceKey":      &dbtypes.AttributeValueMemberS{Value: "uploads/user1/job1/source.mp4"},
					"clips":          &dbtypes.AttributeValueMemberS{Value: string(clipsJSON)},
					"createdAt":      &dbtypes.AttributeValueMemberS{Value: "2024-01-01T00:00:00Z"},
					"updatedAt":      &dbtypes.AttributeValueMemberS{Value: "2024-01-01T00:00:00Z"},
				},
			}, nil
		},
	}
	h := newShortsTestHandler(t, db)
	ctx := context.Background()

	resp, _ := h.HandleRequest(ctx, events.APIGatewayProxyRequest{
		HTTPMethod: "GET",
		Path:       "/shorts/job1/clips",
		Headers:    map[string]string{"Authorization": "Bearer " + generateTestToken("user1")},
	})
	if resp.StatusCode != 200 {
		t.Fatalf("got status %d, want 200 (body: %s)", resp.StatusCode, resp.Body)
	}
	var body struct {
		Clips []models.Clip `json:"clips"`
	}
	if err := json.Unmarshal([]byte(resp.Body), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(body.Clips) != 3 {
		t.Fatalf("got %d clips, want 3", len(body.Clips))
	}
	for i, c := range body.Clips {
		if c.Rank != i+1 {
			t.Errorf("clip %d has rank %d, want %d", i, c.Rank, i+1)
		}
	}
}

func TestGetClipURL_ClipNotInJob(t *testing.T) {
	clips := []models.Clip{
		{ClipID: "c1", S3Key: "shorts/user1/job1/clips/c1.mp4", Rank: 1},
	}
	clipsJSON, _ := json.Marshal(clips)
	db := &mockDynamoDB{
		getItemFunc: func(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
			return &dynamodb.GetItemOutput{
				Item: map[string]dbtypes.AttributeValue{
					"PK":             &dbtypes.AttributeValueMemberS{Value: "USER#user1"},
					"SK":             &dbtypes.AttributeValueMemberS{Value: "SHORTS#job1"},
					"jobId":          &dbtypes.AttributeValueMemberS{Value: "job1"},
					"status":         &dbtypes.AttributeValueMemberS{Value: "completed"},
					"fileType":       &dbtypes.AttributeValueMemberS{Value: "mp4"},
					"sourceDuration": &dbtypes.AttributeValueMemberN{Value: "120"},
					"sourceKey":      &dbtypes.AttributeValueMemberS{Value: "uploads/user1/job1/source.mp4"},
					"clips":          &dbtypes.AttributeValueMemberS{Value: string(clipsJSON)},
					"createdAt":      &dbtypes.AttributeValueMemberS{Value: "2024-01-01T00:00:00Z"},
					"updatedAt":      &dbtypes.AttributeValueMemberS{Value: "2024-01-01T00:00:00Z"},
				},
			}, nil
		},
	}
	h := newShortsTestHandler(t, db)
	ctx := context.Background()
	token := generateTestToken("user1")

	t.Run("existing clip returns url", func(t *testing.T) {
		resp, _ := h.HandleRequest(ctx, events.APIGatewayProxyRequest{
			HTTPMethod: "GET",
			Path:       "/shorts/job1/clips/c1/url",
			Headers:    map[string]string{"Authorization": "Bearer " + token},
		})
		if resp.StatusCode != 200 {
			t.Errorf("got status %d, want 200 (body: %s)", resp.StatusCode, resp.Body)
		}
	})

	t.Run("unknown clip returns 404", func(t *testing.T) {
		resp, _ := h.HandleRequest(ctx, events.APIGatewayProxyRequest{
			HTTPMethod: "GET",
			Path:       "/shorts/job1/clips/nope/url",
			Headers:    map[string]string{"Authorization": "Bearer " + token},
		})
		if resp.StatusCode != 404 {
			t.Errorf("got status %d, want 404", resp.StatusCode)
		}
	})
}

func TestExtractShortsClipIDs(t *testing.T) {
	tests := []struct {
		path    string
		wantJob string
		wantCli string
	}{
		{"/shorts/j1/clips/c1/url", "j1", "c1"},
		{"/shorts/j1/clips", "", ""},
		{"/shorts/j1", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			job, clip := extractShortsClipIDs(tt.path)
			if job != tt.wantJob || clip != tt.wantCli {
				t.Errorf("extractShortsClipIDs(%q) = (%q,%q), want (%q,%q)", tt.path, job, clip, tt.wantJob, tt.wantCli)
			}
		})
	}
}
