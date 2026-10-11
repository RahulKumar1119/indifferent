package api

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/sfn"
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

func TestListShortsJobs_NewestFirstTrimmed(t *testing.T) {
	oldJob := map[string]dbtypes.AttributeValue{
		"PK": &dbtypes.AttributeValueMemberS{Value: "USER#user1"}, "SK": &dbtypes.AttributeValueMemberS{Value: "SHORTS#old"},
		"jobId": &dbtypes.AttributeValueMemberS{Value: "old"}, "status": &dbtypes.AttributeValueMemberS{Value: "completed"},
		"fileType": &dbtypes.AttributeValueMemberS{Value: "mp4"}, "sourceDuration": &dbtypes.AttributeValueMemberN{Value: "120"},
		"sourceKey": &dbtypes.AttributeValueMemberS{Value: "uploads/user1/old/source.mp4"},
		"clips": &dbtypes.AttributeValueMemberS{Value: `[{"clipId":"clip-1","s3Key":"k","rank":1,"score":0.9,"duration":20}]`},
		"createdAt": &dbtypes.AttributeValueMemberS{Value: "2024-01-01T00:00:00Z"}, "updatedAt": &dbtypes.AttributeValueMemberS{Value: "2024-01-01T00:00:00Z"},
	}
	newJob := map[string]dbtypes.AttributeValue{
		"PK": &dbtypes.AttributeValueMemberS{Value: "USER#user1"}, "SK": &dbtypes.AttributeValueMemberS{Value: "SHORTS#new"},
		"jobId": &dbtypes.AttributeValueMemberS{Value: "new"}, "status": &dbtypes.AttributeValueMemberS{Value: "rendering"},
		"fileType": &dbtypes.AttributeValueMemberS{Value: "mp3"}, "sourceDuration": &dbtypes.AttributeValueMemberN{Value: "300"},
		"sourceKey": &dbtypes.AttributeValueMemberS{Value: "uploads/user1/new/source.mp3"},
		"createdAt": &dbtypes.AttributeValueMemberS{Value: "2024-02-01T00:00:00Z"}, "updatedAt": &dbtypes.AttributeValueMemberS{Value: "2024-02-01T00:00:00Z"},
	}
	db := &mockDynamoDB{
		queryFunc: func(ctx context.Context, params *dynamodb.QueryInput, optFns ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error) {
			return &dynamodb.QueryOutput{Items: []map[string]dbtypes.AttributeValue{oldJob, newJob}}, nil
		},
	}
	h := newShortsTestHandler(t, db)

	resp, _ := h.HandleRequest(context.Background(), events.APIGatewayProxyRequest{
		HTTPMethod: "GET",
		Path:       "/shorts",
		Headers:    map[string]string{"Authorization": "Bearer " + generateTestToken("user1")},
	})
	if resp.StatusCode != 200 {
		t.Fatalf("got status %d, want 200 (body: %s)", resp.StatusCode, resp.Body)
	}
	var body struct {
		Jobs []struct {
			JobID     string `json:"jobId"`
			Status    string `json:"status"`
			ClipCount int    `json:"clipCount"`
		} `json:"jobs"`
	}
	if err := json.Unmarshal([]byte(resp.Body), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	slim := body.Jobs
	if len(slim) != 2 {
		t.Fatalf("got %d jobs, want 2", len(slim))
	}
	if slim[0].JobID != "new" || slim[1].JobID != "old" {
		t.Errorf("expected newest-first order, got %+v", slim)
	}
	if slim[0].ClipCount != 0 || slim[1].ClipCount != 1 {
		t.Errorf("expected clip counts [0 1], got %+v", slim)
	}
	if strings.Contains(resp.Body, "captionWords") {
		t.Errorf("list must not include heavy caption payloads: %s", resp.Body)
	}
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
	// Stay offline: report objects as standard storage so the handler
	// presigns instead of calling HeadObject on a fake bucket.
	h.S3 = &stubDownloader{status: storage.ObjectStatus{StorageClass: "STANDARD"}}
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

func TestStartShorts_RejectsRestart(t *testing.T) {
	// A completed job must not re-enter the pipeline.
	db := &mockDynamoDB{
		getItemFunc: func(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
			return &dynamodb.GetItemOutput{
				Item: map[string]dbtypes.AttributeValue{
					"PK":             &dbtypes.AttributeValueMemberS{Value: "USER#user1"},
					"SK":             &dbtypes.AttributeValueMemberS{Value: "SHORTS#done"},
					"jobId":          &dbtypes.AttributeValueMemberS{Value: "done"},
					"status":         &dbtypes.AttributeValueMemberS{Value: "completed"},
					"fileType":       &dbtypes.AttributeValueMemberS{Value: "mp4"},
					"sourceDuration": &dbtypes.AttributeValueMemberN{Value: "120"},
					"sourceKey":      &dbtypes.AttributeValueMemberS{Value: "uploads/user1/done/source.mp4"},
					"createdAt":      &dbtypes.AttributeValueMemberS{Value: "2024-01-01T00:00:00Z"},
					"updatedAt":      &dbtypes.AttributeValueMemberS{Value: "2024-01-01T00:00:00Z"},
				},
			}, nil
		},
	}
	h := newShortsTestHandler(t, db)

	resp, _ := h.HandleRequest(context.Background(), events.APIGatewayProxyRequest{
		HTTPMethod: "POST",
		Path:       "/shorts/done/start",
		Headers:    map[string]string{"Authorization": "Bearer " + generateTestToken("user1")},
	})
	if resp.StatusCode != 400 {
		t.Errorf("got status %d, want 400 (body: %s)", resp.StatusCode, resp.Body)
	}
}

func TestGetClipURL_NotReady(t *testing.T) {
	// A clip record on a non-completed job must not yield a presigned URL.
	db := &mockDynamoDB{
		getItemFunc: func(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
			return &dynamodb.GetItemOutput{
				Item: map[string]dbtypes.AttributeValue{
					"PK":             &dbtypes.AttributeValueMemberS{Value: "USER#user1"},
					"SK":             &dbtypes.AttributeValueMemberS{Value: "SHORTS#busy"},
					"jobId":          &dbtypes.AttributeValueMemberS{Value: "busy"},
					"status":         &dbtypes.AttributeValueMemberS{Value: "rendering"},
					"fileType":       &dbtypes.AttributeValueMemberS{Value: "mp4"},
					"sourceDuration": &dbtypes.AttributeValueMemberN{Value: "120"},
					"sourceKey":      &dbtypes.AttributeValueMemberS{Value: "uploads/user1/busy/source.mp4"},
					"clips":          &dbtypes.AttributeValueMemberS{Value: `[{"clipId":"clip-1","s3Key":"renders/busy/clip-1.mp4","rank":1,"score":0.9,"duration":20}]`},
					"createdAt":      &dbtypes.AttributeValueMemberS{Value: "2024-01-01T00:00:00Z"},
					"updatedAt":      &dbtypes.AttributeValueMemberS{Value: "2024-01-01T00:00:00Z"},
				},
			}, nil
		},
	}
	h := newShortsTestHandler(t, db)

	resp, _ := h.HandleRequest(context.Background(), events.APIGatewayProxyRequest{
		HTTPMethod: "GET",
		Path:       "/shorts/busy/clips/clip-1/url",
		Headers:    map[string]string{"Authorization": "Bearer " + generateTestToken("user1")},
	})
	if resp.StatusCode != 400 {
		t.Errorf("got status %d, want 400 (body: %s)", resp.StatusCode, resp.Body)
	}
}

func shortsJobItem(jobID, status string) map[string]dbtypes.AttributeValue {
	return map[string]dbtypes.AttributeValue{
		"PK":             &dbtypes.AttributeValueMemberS{Value: "USER#user1"},
		"SK":             &dbtypes.AttributeValueMemberS{Value: "SHORTS#" + jobID},
		"jobId":          &dbtypes.AttributeValueMemberS{Value: jobID},
		"status":         &dbtypes.AttributeValueMemberS{Value: status},
		"fileType":       &dbtypes.AttributeValueMemberS{Value: "mp4"},
		"sourceDuration": &dbtypes.AttributeValueMemberN{Value: "120"},
		"sourceKey":      &dbtypes.AttributeValueMemberS{Value: "uploads/user1/" + jobID + "/source.mp4"},
		"transcriptKey":  &dbtypes.AttributeValueMemberS{Value: "shorts/user1/" + jobID + "/transcript.json"},
		"clips":          &dbtypes.AttributeValueMemberS{Value: `[{"clipId":"clip-1","s3Key":"shorts/user1/` + jobID + `/clips/clip-1.mp4","rank":1,"score":0.9,"duration":20},{"clipId":"clip-2","s3Key":"shorts/user1/` + jobID + `/clips/clip-2.mp4","rank":2,"score":0.8,"duration":18},{"clipId":"clip-x","s3Key":"elsewhere/evil.mp4","rank":3,"score":0.1,"duration":5}]`},
		"createdAt":      &dbtypes.AttributeValueMemberS{Value: "2024-01-01T00:00:00Z"},
		"updatedAt":      &dbtypes.AttributeValueMemberS{Value: "2024-01-01T00:00:00Z"},
	}
}

type deleteProbe struct {
	handler   *APIHandler
	s3        *stubDownloader
	stops     int
	putItems  []map[string]dbtypes.AttributeValue
	deleted   []string
}

func newDeleteProbe(t *testing.T, item map[string]dbtypes.AttributeValue, s3 *stubDownloader) *deleteProbe {
	t.Helper()
	p := &deleteProbe{s3: s3}
	db := &mockDynamoDB{
		getItemFunc: func(_ context.Context, _ *dynamodb.GetItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
			return &dynamodb.GetItemOutput{Item: item}, nil
		},
		putItemFunc: func(_ context.Context, params *dynamodb.PutItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
			p.putItems = append(p.putItems, params.Item)
			return &dynamodb.PutItemOutput{}, nil
		},
		deleteItemFunc: func(_ context.Context, params *dynamodb.DeleteItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error) {
			if v, ok := params.Key["SK"].(*dbtypes.AttributeValueMemberS); ok {
				p.deleted = append(p.deleted, v.Value)
			}
			return &dynamodb.DeleteItemOutput{}, nil
		},
	}
	p.handler = &APIHandler{
		AuthService:           &mockAuthService{},
		JWTService:            &auth.JWTService{Secret: testSecret, DB: db, SessionTable: "sessions-table"},
		DB:                    db,
		S3:                    s3,
		SFN:                   &mockSFN{stopExecutionFunc: func(_ context.Context, _ *sfn.StopExecutionInput, _ ...func(*sfn.Options)) (*sfn.StopExecutionOutput, error) {
			p.stops++
			return &sfn.StopExecutionOutput{}, nil
		}},
		TableName:             "table",
		Bucket:                "test-bucket",
		StateMachineARN:       "arn:aws:states:us-east-1:123456789:stateMachine:test",
		ShortsStateMachineARN: "arn:aws:states:us-east-1:123456789:stateMachine:shorts",
	}
	return p
}

func deleteReq(jobID string) events.APIGatewayProxyRequest {
	return events.APIGatewayProxyRequest{
		HTTPMethod: "DELETE",
		Path:       "/shorts/" + jobID,
		Headers:    map[string]string{"Authorization": "Bearer " + generateTestToken("user1")},
	}
}

func TestDeleteShorts_Success(t *testing.T) {
	s3 := &stubDownloader{
		status: storage.ObjectStatus{StorageClass: "STANDARD"},
		listedKeys: []string{
			"uploads/user1/job1/source.mp4",
			"shorts/user1/job1/transcript.json",
			"shorts/user1/job1/transcribe-raw.json",
			"shorts/user1/job1/.write_access_check_file.temp",
			"shorts/user1/job1/clips/clip-1.mp4",
			"shorts/user1/job1/clips/clip-2.mp4",
		},
	}
	p := newDeleteProbe(t, shortsJobItem("job1", "completed"), s3)

	resp, err := p.handler.HandleRequest(context.Background(), deleteReq("job1"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 204 {
		t.Fatalf("got status %d, want 204 (body: %s)", resp.StatusCode, resp.Body)
	}
	// Every listed object under the job prefixes must go, including files the
	// record never tracks (raw transcript, temp files).
	for _, want := range []string{
		"uploads/user1/job1/source.mp4",
		"shorts/user1/job1/transcript.json",
		"shorts/user1/job1/transcribe-raw.json",
		"shorts/user1/job1/.write_access_check_file.temp",
		"shorts/user1/job1/clips/clip-1.mp4",
		"shorts/user1/job1/clips/clip-2.mp4",
	} {
		found := false
		for _, got := range s3.deletedKeys {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected S3 delete of %q, got %v", want, s3.deletedKeys)
		}
	}
	// A tracked key outside the job prefixes must never be deleted.
	for _, got := range s3.deletedKeys {
		if got == "elsewhere/evil.mp4" {
			t.Errorf("must not delete keys outside the job prefixes, got %v", s3.deletedKeys)
		}
	}
	if len(p.deleted) != 1 || p.deleted[0] != "SHORTS#job1" {
		t.Errorf("expected DB delete of SHORTS#job1, got %v", p.deleted)
	}
	if p.stops != 0 {
		t.Errorf("completed job must not stop an execution, got %d stops", p.stops)
	}
	// Versioned bucket: both job prefixes must be purged of all versions.
	for _, want := range []string{"uploads/user1/job1/", "shorts/user1/job1/"} {
		found := false
		for _, got := range s3.purged {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected version purge of %q, got %v", want, s3.purged)
		}
	}
}

func TestDeleteShorts_ListFailureRetainsRecord(t *testing.T) {
	s3 := &stubDownloader{
		status:  storage.ObjectStatus{StorageClass: "STANDARD"},
		listErr: errors.New("list down"),
	}
	p := newDeleteProbe(t, shortsJobItem("job1", "completed"), s3)

	resp, _ := p.handler.HandleRequest(context.Background(), deleteReq("job1"))
	if resp.StatusCode != 500 {
		t.Fatalf("got status %d, want 500 (body: %s)", resp.StatusCode, resp.Body)
	}
	if len(p.deleted) != 0 {
		t.Error("record must be retained when enumeration fails")
	}
}

func TestDeleteShorts_InFlightStopsExecution(t *testing.T) {
	s3 := &stubDownloader{status: storage.ObjectStatus{StorageClass: "STANDARD"}}
	p := newDeleteProbe(t, shortsJobItem("busy", "rendering"), s3)

	resp, _ := p.handler.HandleRequest(context.Background(), deleteReq("busy"))
	if resp.StatusCode != 204 {
		t.Fatalf("got status %d, want 204 (body: %s)", resp.StatusCode, resp.Body)
	}
	if p.stops != 1 {
		t.Errorf("expected 1 StopExecution for in-flight job, got %d", p.stops)
	}
}

func TestDeleteShorts_NotFoundAndUnauth(t *testing.T) {
	s3 := &stubDownloader{status: storage.ObjectStatus{StorageClass: "STANDARD"}}
	p := newDeleteProbe(t, nil, s3)

	resp, _ := p.handler.HandleRequest(context.Background(), deleteReq("missing"))
	if resp.StatusCode != 404 {
		t.Errorf("got status %d, want 404 (body: %s)", resp.StatusCode, resp.Body)
	}
	if len(s3.deletedKeys) != 0 || len(p.deleted) != 0 {
		t.Error("no S3 or DB writes must happen for unknown jobs")
	}

	resp, _ = p.handler.HandleRequest(context.Background(), events.APIGatewayProxyRequest{
		HTTPMethod: "DELETE",
		Path:       "/shorts/job1",
	})
	if resp.StatusCode != 401 {
		t.Errorf("got status %d, want 401 (body: %s)", resp.StatusCode, resp.Body)
	}
}

func TestDeleteShorts_S3FailureRetainsRecord(t *testing.T) {
	s3 := &stubDownloader{
		status:    storage.ObjectStatus{StorageClass: "STANDARD"},
		deleteErr: errors.New("s3 down"),
	}
	p := newDeleteProbe(t, shortsJobItem("job9", "completed"), s3)

	resp, _ := p.handler.HandleRequest(context.Background(), deleteReq("job9"))
	if resp.StatusCode != 500 {
		t.Fatalf("got status %d, want 500 (body: %s)", resp.StatusCode, resp.Body)
	}
	if len(p.deleted) != 0 {
		t.Error("record must be retained when S3 cleanup fails")
	}
	if len(p.putItems) != 1 {
		t.Fatalf("expected job marked delete_failed, got %d puts", len(p.putItems))
	}
	if v, ok := p.putItems[0]["status"].(*dbtypes.AttributeValueMemberS); !ok || v.Value != "delete_failed" {
		t.Errorf("expected status delete_failed, got %v", p.putItems[0]["status"])
	}
}

func TestDeleteShorts_RetryAfterFailure(t *testing.T) {
	// A delete_failed record deletes cleanly on retry (idempotent path).
	s3 := &stubDownloader{status: storage.ObjectStatus{StorageClass: "STANDARD"}}
	p := newDeleteProbe(t, shortsJobItem("job9", "delete_failed"), s3)

	resp, _ := p.handler.HandleRequest(context.Background(), deleteReq("job9"))
	if resp.StatusCode != 204 {
		t.Fatalf("got status %d, want 204 (body: %s)", resp.StatusCode, resp.Body)
	}
	if len(p.deleted) != 1 {
		t.Error("expected record delete on retry")
	}
}

func TestDeleteShorts_DeepPathIs404(t *testing.T) {
	s3 := &stubDownloader{status: storage.ObjectStatus{StorageClass: "STANDARD"}}
	p := newDeleteProbe(t, shortsJobItem("job1", "completed"), s3)

	resp, _ := p.handler.HandleRequest(context.Background(), events.APIGatewayProxyRequest{
		HTTPMethod: "DELETE",
		Path:       "/shorts/job1/start",
		Headers:    map[string]string{"Authorization": "Bearer " + generateTestToken("user1")},
	})
	if resp.StatusCode != 404 {
		t.Errorf("got status %d, want 404 (body: %s)", resp.StatusCode, resp.Body)
	}
}
