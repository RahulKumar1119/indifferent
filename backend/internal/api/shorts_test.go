package api

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/sfn"
	"github.com/rahul/indifferent/backend/internal/auth"
	"github.com/rahul/indifferent/backend/internal/models"
	"github.com/rahul/indifferent/backend/internal/shorts"
	"github.com/rahul/indifferent/backend/internal/storage"
)

func newShortsTestHandler(t *testing.T, db *mockDynamoDB) *APIHandler {
	t.Helper()
	return &APIHandler{
		AuthService:           &mockAuthService{},
		JWTService:            &auth.JWTService{Secret: testSecret, DB: db, SessionTable: "sessions-table"},
		DB:                    db,
		S3:                    &stubDownloader{status: storage.ObjectStatus{StorageClass: "STANDARD"}, meta: storage.ObjectMeta{Found: true, Size: 50 << 20}},
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
		{"duration over max", CreateShortsRequest{FileType: "mp4", Duration: 3601}, true},
		{"duration at max", CreateShortsRequest{FileType: "mp4", Duration: 3600}, false},
		{"duration one over max", CreateShortsRequest{FileType: "mp4", Duration: shorts.MaxSourceDuration + 1}, true},
		{"duration exactly at max", CreateShortsRequest{FileType: "mp4", Duration: shorts.MaxSourceDuration}, false},
		{"valid auto language", CreateShortsRequest{FileType: "mp4", Duration: 60, Language: shorts.LanguageAuto}, false},
		{"valid Hindi alias", CreateShortsRequest{FileType: "mp4", Duration: 60, Language: "hi"}, false},
		{"valid Hindi code", CreateShortsRequest{FileType: "mp4", Duration: 60, Language: shorts.LanguageHindi}, false},
		{"valid English India", CreateShortsRequest{FileType: "mp4", Duration: 60, Language: shorts.LanguageEnglishIN}, false},
		{"unsupported language", CreateShortsRequest{FileType: "mp4", Duration: 60, Language: "klingon"}, true},
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
		{"over duration", `{"fileType":"mp4","duration":3601}`, 400, "VALIDATION_ERROR"},
		{"at duration limit", `{"fileType":"mp4","duration":3600}`, 201, ""},
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

func TestCreateShorts_PersistsCanonicalLanguage(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{"default auto", `{"fileType":"mp4","duration":60}`, shorts.LanguageAuto},
		{"hindi alias", `{"fileType":"mp4","duration":60,"language":"hi"}`, shorts.LanguageHindi},
		{"english india", `{"fileType":"mp4","duration":60,"language":"en-IN"}`, shorts.LanguageEnglishIN},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var putItem map[string]dbtypes.AttributeValue
			db := &mockDynamoDB{
				putItemFunc: func(ctx context.Context, params *dynamodb.PutItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
					putItem = params.Item
					return &dynamodb.PutItemOutput{}, nil
				},
			}
			h := newShortsTestHandler(t, db)

			resp, err := h.HandleRequest(context.Background(), events.APIGatewayProxyRequest{
				HTTPMethod: "POST",
				Path:       "/shorts",
				Headers:    map[string]string{"Authorization": "Bearer " + generateTestToken("user1")},
				Body:       tt.body,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if resp.StatusCode != 201 {
				t.Fatalf("got status %d, want 201 (body: %s)", resp.StatusCode, resp.Body)
			}
			got, ok := putItem["language"].(*dbtypes.AttributeValueMemberS)
			if !ok {
				t.Fatalf("stored item has no language attribute: %+v", putItem)
			}
			if got.Value != tt.want {
				t.Errorf("stored language = %q, want %q", got.Value, tt.want)
			}
		})
	}
}

func TestStartShorts_PassesLanguageToPipeline(t *testing.T) {
	var payload map[string]interface{}
	db := &mockDynamoDB{
		getItemFunc: func(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
			return &dynamodb.GetItemOutput{
				Item: map[string]dbtypes.AttributeValue{
					"PK":             &dbtypes.AttributeValueMemberS{Value: "USER#user1"},
					"SK":             &dbtypes.AttributeValueMemberS{Value: "SHORTS#job1"},
					"jobId":          &dbtypes.AttributeValueMemberS{Value: "job1"},
					"status":         &dbtypes.AttributeValueMemberS{Value: "uploaded"},
					"fileType":       &dbtypes.AttributeValueMemberS{Value: "mp4"},
					"language":       &dbtypes.AttributeValueMemberS{Value: shorts.LanguageHindi},
					"sourceDuration": &dbtypes.AttributeValueMemberN{Value: "120"},
					"sourceKey":      &dbtypes.AttributeValueMemberS{Value: "uploads/user1/job1/source.mp4"},
					"createdAt":      &dbtypes.AttributeValueMemberS{Value: "2024-01-01T00:00:00Z"},
					"updatedAt":      &dbtypes.AttributeValueMemberS{Value: "2024-01-01T00:00:00Z"},
				},
			}, nil
		},
	}
	h := newShortsTestHandler(t, db)
	h.SFN = &mockSFN{startExecutionFunc: func(_ context.Context, in *sfn.StartExecutionInput, _ ...func(*sfn.Options)) (*sfn.StartExecutionOutput, error) {
		if err := json.Unmarshal([]byte(aws.ToString(in.Input)), &payload); err != nil {
			t.Fatalf("unmarshal SFN input: %v", err)
		}
		return &sfn.StartExecutionOutput{}, nil
	}}

	resp, _ := h.HandleRequest(context.Background(), events.APIGatewayProxyRequest{
		HTTPMethod: "POST",
		Path:       "/shorts/job1/start",
		Headers:    map[string]string{"Authorization": "Bearer " + generateTestToken("user1")},
	})
	if resp.StatusCode != 200 {
		t.Fatalf("got status %d, want 200 (body: %s)", resp.StatusCode, resp.Body)
	}
	if payload["language"] != shorts.LanguageHindi {
		t.Errorf("pipeline language = %v, want %q", payload["language"], shorts.LanguageHindi)
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
		"clips":     &dbtypes.AttributeValueMemberS{Value: `[{"clipId":"clip-1","s3Key":"k","rank":1,"score":0.9,"duration":20}]`},
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
	handler  *APIHandler
	s3       *stubDownloader
	stops    int
	putItems []map[string]dbtypes.AttributeValue
	deleted  []string
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
		AuthService: &mockAuthService{},
		JWTService:  &auth.JWTService{Secret: testSecret, DB: db, SessionTable: "sessions-table"},
		DB:          db,
		S3:          s3,
		SFN: &mockSFN{stopExecutionFunc: func(_ context.Context, _ *sfn.StopExecutionInput, _ ...func(*sfn.Options)) (*sfn.StopExecutionOutput, error) {
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
	// The job record goes first, then its thumbnail record (issue #77).
	if len(p.deleted) != 2 || p.deleted[0] != "SHORTS#job1" || p.deleted[1] != "THUMBS#job1" {
		t.Errorf("expected DB deletes of SHORTS#job1 and THUMBS#job1, got %v", p.deleted)
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

func TestDeleteShorts_ListFailureFallsBackToTrackedKeys(t *testing.T) {
	// Without s3:ListBucket the sweep degrades to tracked keys instead of
	// failing: the large assets still go and the record is removed.
	s3 := &stubDownloader{
		status:  storage.ObjectStatus{StorageClass: "STANDARD"},
		listErr: errors.New("list down"),
	}
	p := newDeleteProbe(t, shortsJobItem("job1", "completed"), s3)

	resp, _ := p.handler.HandleRequest(context.Background(), deleteReq("job1"))
	if resp.StatusCode != 204 {
		t.Fatalf("got status %d, want 204 (body: %s)", resp.StatusCode, resp.Body)
	}
	for _, want := range []string{
		"uploads/user1/job1/source.mp4",
		"shorts/user1/job1/transcript.json",
		"shorts/user1/job1/clips/clip-1.mp4",
	} {
		found := false
		for _, got := range s3.deletedKeys {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected fallback delete of %q, got %v", want, s3.deletedKeys)
		}
	}
	if len(p.deleted) != 2 {
		t.Error("expected record delete on fallback path")
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
	if len(p.deleted) != 2 {
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

func TestStartShorts_VerifiesUpload(t *testing.T) {
	base := map[string]dbtypes.AttributeValue{
		"PK":             &dbtypes.AttributeValueMemberS{Value: "USER#user1"},
		"SK":             &dbtypes.AttributeValueMemberS{Value: "SHORTS#job1"},
		"jobId":          &dbtypes.AttributeValueMemberS{Value: "job1"},
		"status":         &dbtypes.AttributeValueMemberS{Value: "uploaded"},
		"fileType":       &dbtypes.AttributeValueMemberS{Value: "mp4"},
		"sourceDuration": &dbtypes.AttributeValueMemberN{Value: "120"},
		"sourceKey":      &dbtypes.AttributeValueMemberS{Value: "uploads/user1/job1/source.mp4"},
		"createdAt":      &dbtypes.AttributeValueMemberS{Value: "2024-01-01T00:00:00Z"},
		"updatedAt":      &dbtypes.AttributeValueMemberS{Value: "2024-01-01T00:00:00Z"},
	}
	start := func(meta storage.ObjectMeta) (int, string) {
		db := &mockDynamoDB{
			getItemFunc: func(_ context.Context, _ *dynamodb.GetItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
				return &dynamodb.GetItemOutput{Item: base}, nil
			},
		}
		h := newShortsTestHandler(t, db)
		h.S3 = &stubDownloader{status: storage.ObjectStatus{StorageClass: "STANDARD"}, meta: meta}
		resp, _ := h.HandleRequest(context.Background(), events.APIGatewayProxyRequest{
			HTTPMethod: "POST",
			Path:       "/shorts/job1/start",
			Headers:    map[string]string{"Authorization": "Bearer " + generateTestToken("user1")},
		})
		return resp.StatusCode, resp.Body
	}

	t.Run("missing upload rejected", func(t *testing.T) {
		status, body := start(storage.ObjectMeta{Found: false})
		if status != 400 || !strings.Contains(body, "UPLOAD_NOT_FOUND") {
			t.Errorf("got %d %s, want 400 UPLOAD_NOT_FOUND", status, body)
		}
	})

	t.Run("empty upload rejected", func(t *testing.T) {
		status, body := start(storage.ObjectMeta{Found: true, Size: 0, ContentType: "video/mp4"})
		if status != 400 || !strings.Contains(body, "EMPTY_UPLOAD") {
			t.Errorf("got %d %s, want 400 EMPTY_UPLOAD", status, body)
		}
	})

	t.Run("oversize upload rejected", func(t *testing.T) {
		// One byte over the 2 GB cap.
		status, body := start(storage.ObjectMeta{Found: true, Size: shorts.MaxUploadBytes + 1, ContentType: "video/mp4"})
		if status != 400 || !strings.Contains(body, "UPLOAD_TOO_LARGE") {
			t.Errorf("got %d %s, want 400 UPLOAD_TOO_LARGE", status, body)
		}
	})

	t.Run("upload at the size cap accepted", func(t *testing.T) {
		status, body := start(storage.ObjectMeta{Found: true, Size: shorts.MaxUploadBytes, ContentType: "video/mp4"})
		if status != 200 {
			t.Errorf("got %d %s, want 200", status, body)
		}
	})

	t.Run("type mismatch rejected", func(t *testing.T) {
		status, body := start(storage.ObjectMeta{Found: true, Size: 10 << 20, ContentType: "application/pdf"})
		if status != 400 || !strings.Contains(body, "UPLOAD_TYPE_MISMATCH") {
			t.Errorf("got %d %s, want 400 UPLOAD_TYPE_MISMATCH", status, body)
		}
	})

	t.Run("valid upload starts", func(t *testing.T) {
		status, _ := start(storage.ObjectMeta{Found: true, Size: 10 << 20, ContentType: "video/mp4"})
		if status != 200 {
			t.Errorf("got status %d, want 200", status)
		}
	})
}

func failedJobItem(jobID string) map[string]dbtypes.AttributeValue {
	item := shortsJobItem(jobID, "failed")
	item["error"] = &dbtypes.AttributeValueMemberS{Value: "transcode exploded"}
	item["errorCode"] = &dbtypes.AttributeValueMemberS{Value: "INFRASTRUCTURE"}
	item["failedStage"] = &dbtypes.AttributeValueMemberS{Value: "rendering"}
	item["clips"] = &dbtypes.AttributeValueMemberS{Value: `[{"clipId":"clip-1","s3Key":"shorts/user1/` + jobID + `/clips/clip-1.mp4","rank":1,"score":0.9,"duration":20}]`}
	return item
}

func TestRetryShorts_RestartsFailedJob(t *testing.T) {
	var puts []map[string]dbtypes.AttributeValue
	db := &mockDynamoDB{
		getItemFunc: func(_ context.Context, _ *dynamodb.GetItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
			return &dynamodb.GetItemOutput{Item: failedJobItem("job1")}, nil
		},
		putItemFunc: func(_ context.Context, params *dynamodb.PutItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
			puts = append(puts, params.Item)
			return &dynamodb.PutItemOutput{}, nil
		},
	}
	h := newShortsTestHandler(t, db)

	resp, _ := h.HandleRequest(context.Background(), events.APIGatewayProxyRequest{
		HTTPMethod: "POST",
		Path:       "/shorts/job1/retry",
		Headers:    map[string]string{"Authorization": "Bearer " + generateTestToken("user1")},
	})
	if resp.StatusCode != 200 {
		t.Fatalf("got status %d, want 200 (body: %s)", resp.StatusCode, resp.Body)
	}
	if len(puts) != 1 {
		t.Fatalf("expected 1 reset write, got %d", len(puts))
	}
	reset := puts[0]
	if v, _ := reset["status"].(*dbtypes.AttributeValueMemberS); v.Value != "uploaded" {
		t.Errorf("expected reset to uploaded, got %v", reset["status"])
	}
	for _, k := range []string{"error", "errorCode", "failedStage", "clips"} {
		if _, ok := reset[k]; ok {
			t.Errorf("expected %s cleared on reset, still present", k)
		}
	}
}

func TestRetryShorts_RejectsNonFailed(t *testing.T) {
	db := &mockDynamoDB{
		getItemFunc: func(_ context.Context, _ *dynamodb.GetItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
			return &dynamodb.GetItemOutput{Item: shortsJobItem("job1", "rendering")}, nil
		},
	}
	h := newShortsTestHandler(t, db)

	resp, _ := h.HandleRequest(context.Background(), events.APIGatewayProxyRequest{
		HTTPMethod: "POST",
		Path:       "/shorts/job1/retry",
		Headers:    map[string]string{"Authorization": "Bearer " + generateTestToken("user1")},
	})
	if resp.StatusCode != 400 {
		t.Errorf("got status %d, want 400 (body: %s)", resp.StatusCode, resp.Body)
	}
}

func TestRetryShorts_ConflictOnDoubleRestart(t *testing.T) {
	// A concurrent retry already reset the row: the conditional write fails.
	db := &mockDynamoDB{
		getItemFunc: func(_ context.Context, _ *dynamodb.GetItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
			return &dynamodb.GetItemOutput{Item: failedJobItem("job1")}, nil
		},
		putItemFunc: func(_ context.Context, _ *dynamodb.PutItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
			return nil, &dbtypes.ConditionalCheckFailedException{}
		},
	}
	h := newShortsTestHandler(t, db)

	resp, _ := h.HandleRequest(context.Background(), events.APIGatewayProxyRequest{
		HTTPMethod: "POST",
		Path:       "/shorts/job1/retry",
		Headers:    map[string]string{"Authorization": "Bearer " + generateTestToken("user1")},
	})
	if resp.StatusCode != 409 {
		t.Errorf("got status %d, want 409 (body: %s)", resp.StatusCode, resp.Body)
	}
}

func styleReq(jobID, body string) events.APIGatewayProxyRequest {
	return events.APIGatewayProxyRequest{
		HTTPMethod: "PUT",
		Path:       "/shorts/" + jobID + "/style",
		Body:       body,
		Headers:    map[string]string{"Authorization": "Bearer " + generateTestToken("user1")},
	}
}

func TestUpdateCaption_SavesValidatedStyle(t *testing.T) {
	var puts []map[string]dbtypes.AttributeValue
	db := &mockDynamoDB{
		getItemFunc: func(_ context.Context, _ *dynamodb.GetItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
			return &dynamodb.GetItemOutput{Item: shortsJobItem("job1", "uploaded")}, nil
		},
		putItemFunc: func(_ context.Context, params *dynamodb.PutItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
			puts = append(puts, params.Item)
			return &dynamodb.PutItemOutput{}, nil
		},
	}
	h := newShortsTestHandler(t, db)

	resp, _ := h.HandleRequest(context.Background(), styleReq("job1",
		`{"caption":{"preset":"karaoke","fontSize":24,"color":"#FFD60A","highlight":"#9AA0A6","position":"top"}}`))
	if resp.StatusCode != 200 {
		t.Fatalf("got status %d, want 200 (body: %s)", resp.StatusCode, resp.Body)
	}
	if len(puts) != 1 {
		t.Fatalf("expected 1 persisted write, got %d", len(puts))
	}
	raw, ok := puts[0]["caption"].(*dbtypes.AttributeValueMemberS)
	if !ok || !strings.Contains(raw.Value, `"preset":"karaoke"`) {
		t.Errorf("expected persisted karaoke caption, got %v", puts[0]["caption"])
	}
	var body map[string]interface{}
	if err := json.Unmarshal([]byte(resp.Body), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	cap, ok := body["caption"].(map[string]interface{})
	if !ok || cap["preset"] != "karaoke" || cap["position"] != "top" {
		t.Errorf("expected echoed caption in response, got %v", body["caption"])
	}
}

func TestUpdateCaption_RejectsBadInput(t *testing.T) {
	db := &mockDynamoDB{
		getItemFunc: func(_ context.Context, _ *dynamodb.GetItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
			return &dynamodb.GetItemOutput{Item: shortsJobItem("job1", "uploaded")}, nil
		},
	}
	h := newShortsTestHandler(t, db)

	for _, body := range []string{
		`{"caption":{"preset":"comic"}}`,
		`{"caption":{"fontSize":200}}`,
		`{"caption":{"color":"red"}}`,
		`{"caption":{"position":"left"}}`,
		`not-json`,
	} {
		resp, _ := h.HandleRequest(context.Background(), styleReq("job1", body))
		if resp.StatusCode != 400 {
			t.Errorf("body %q: got status %d, want 400 (body: %s)", body, resp.StatusCode, resp.Body)
		}
	}
}

func TestUpdateCaption_RejectsWrongState(t *testing.T) {
	for _, status := range []string{"transcribing", "ranking", "rendering", "completed", "delete_failed"} {
		db := &mockDynamoDB{
			getItemFunc: func(_ context.Context, _ *dynamodb.GetItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
				return &dynamodb.GetItemOutput{Item: shortsJobItem("job1", status)}, nil
			},
		}
		h := newShortsTestHandler(t, db)
		resp, _ := h.HandleRequest(context.Background(), styleReq("job1", `{"caption":{"preset":"minimal"}}`))
		if resp.StatusCode != 400 {
			t.Errorf("status %s: got %d, want 400 (body: %s)", status, resp.StatusCode, resp.Body)
		}
	}
}

func TestUpdateCaption_UnknownJob(t *testing.T) {
	db := &mockDynamoDB{
		getItemFunc: func(_ context.Context, _ *dynamodb.GetItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
			return &dynamodb.GetItemOutput{Item: nil}, nil
		},
	}
	h := newShortsTestHandler(t, db)
	resp, _ := h.HandleRequest(context.Background(), styleReq("job1", `{"caption":{"preset":"minimal"}}`))
	if resp.StatusCode != 404 {
		t.Errorf("got status %d, want 404 (body: %s)", resp.StatusCode, resp.Body)
	}
}

func editedJobItem(jobID, status string) map[string]dbtypes.AttributeValue {
	item := shortsJobItem(jobID, status)
	item["segments"] = &dbtypes.AttributeValueMemberS{Value: `[{"clipId":"clip-9","start":10,"end":40,"score":0.9,"rank":1,"hookText":"h"}]`}
	return item
}

func clipEditReq(jobID, clipID, body string) events.APIGatewayProxyRequest {
	return events.APIGatewayProxyRequest{
		HTTPMethod: "PUT",
		Path:       "/shorts/" + jobID + "/clips/" + clipID,
		Body:       body,
		Headers:    map[string]string{"Authorization": "Bearer " + generateTestToken("user1")},
	}
}

func TestUpdateClipEdit_SavesAndReportsDropped(t *testing.T) {
	var puts []map[string]dbtypes.AttributeValue
	db := &mockDynamoDB{
		getItemFunc: func(_ context.Context, _ *dynamodb.GetItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
			return &dynamodb.GetItemOutput{Item: editedJobItem("job1", "completed")}, nil
		},
		putItemFunc: func(_ context.Context, params *dynamodb.PutItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
			puts = append(puts, params.Item)
			return &dynamodb.PutItemOutput{}, nil
		},
	}
	h := newShortsTestHandler(t, db)

	resp, _ := h.HandleRequest(context.Background(), clipEditReq("job1", "clip-9",
		`{"start":12,"end":38,"words":[{"text":"fixed","start":15,"end":15.5},{"text":"late","start":50,"end":50.5}]}`))
	if resp.StatusCode != 200 {
		t.Fatalf("got status %d, want 200 (body: %s)", resp.StatusCode, resp.Body)
	}
	var body struct {
		DroppedWords int `json:"droppedWords"`
		Edit         struct {
			Start float64 `json:"start"`
			End   float64 `json:"end"`
		} `json:"edit"`
	}
	if err := json.Unmarshal([]byte(resp.Body), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body.DroppedWords != 1 {
		t.Errorf("expected 1 dropped word, got %d", body.DroppedWords)
	}
	if len(puts) != 1 {
		t.Fatalf("expected 1 persisted write, got %d", len(puts))
	}
	raw, ok := puts[0]["clipEdits"].(*dbtypes.AttributeValueMemberS)
	if !ok || !strings.Contains(raw.Value, "clip-9") {
		t.Errorf("expected persisted clipEdits, got %v", puts[0]["clipEdits"])
	}
}

func TestUpdateClipEdit_RejectsBadInput(t *testing.T) {
	db := &mockDynamoDB{
		getItemFunc: func(_ context.Context, _ *dynamodb.GetItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
			return &dynamodb.GetItemOutput{Item: editedJobItem("job1", "failed")}, nil
		},
	}
	h := newShortsTestHandler(t, db)

	for _, tc := range []struct{ name, body string }{
		{"end-before-start", `{"start":30,"end":20}`},
		{"negative", `{"start":-5,"end":20}`},
		{"too-short", `{"start":10,"end":10.5}`},
		{"over-duration", `{"start":10,"end":700}`},
		{"empty-word", `{"start":10,"end":40,"words":[{"text":" ","start":15,"end":15.5}]}`},
		{"bad-word-timing", `{"start":10,"end":40,"words":[{"text":"x","start":16,"end":15}]}`},
		{"not-json", `oops`},
	} {
		resp, _ := h.HandleRequest(context.Background(), clipEditReq("job1", "clip-9", tc.body))
		if resp.StatusCode != 400 {
			t.Errorf("%s: got status %d, want 400 (body: %s)", tc.name, resp.StatusCode, resp.Body)
		}
	}
	// Unknown clip ID is 404, not 400.
	resp, _ := h.HandleRequest(context.Background(), clipEditReq("job1", "nope", `{"start":10,"end":40}`))
	if resp.StatusCode != 404 {
		t.Errorf("unknown clip: got status %d, want 404 (body: %s)", resp.StatusCode, resp.Body)
	}
}

func TestUpdateClipEdit_ResetAndStateGuard(t *testing.T) {
	var puts []map[string]dbtypes.AttributeValue
	newHandler := func(status string) *APIHandler {
		db := &mockDynamoDB{
			getItemFunc: func(_ context.Context, _ *dynamodb.GetItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
				item := editedJobItem("job1", status)
				item["clipEdits"] = &dbtypes.AttributeValueMemberS{Value: `{"clip-9":{"start":12,"end":38}}`}
				return &dynamodb.GetItemOutput{Item: item}, nil
			},
			putItemFunc: func(_ context.Context, params *dynamodb.PutItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
				puts = append(puts, params.Item)
				return &dynamodb.PutItemOutput{}, nil
			},
		}
		return newShortsTestHandler(t, db)
	}

	// Reset clears the override (reset-to-AI-suggestion).
	puts = nil
	h := newHandler("completed")
	resp, _ := h.HandleRequest(context.Background(), clipEditReq("job1", "clip-9", `{"reset":true}`))
	if resp.StatusCode != 200 || !strings.Contains(resp.Body, `"reset":true`) {
		t.Fatalf("reset: got status %d (body: %s)", resp.StatusCode, resp.Body)
	}
	if _, ok := puts[0]["clipEdits"]; ok {
		t.Error("reset must drop the clipEdits attribute")
	}

	// In-flight jobs reject edits.
	for _, status := range []string{"transcribing", "ranking", "rendering"} {
		h := newHandler(status)
		resp, _ := h.HandleRequest(context.Background(), clipEditReq("job1", "clip-9", `{"start":10,"end":20}`))
		if resp.StatusCode != 400 {
			t.Errorf("status %s: got %d, want 400", status, resp.StatusCode)
		}
	}
}

func TestGetClipCaptions_WindowedWords(t *testing.T) {
	tr, _ := json.Marshal(map[string]interface{}{
		"text": "all",
		"words": []map[string]interface{}{
			{"text": "early", "start": 1.0, "end": 1.5},
			{"text": "kept", "start": 12.0, "end": 12.5},
			{"text": "late", "start": 50.0, "end": 50.5},
		},
	})
	db := &mockDynamoDB{
		getItemFunc: func(_ context.Context, _ *dynamodb.GetItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
			return &dynamodb.GetItemOutput{Item: editedJobItem("job1", "completed")}, nil
		},
	}
	h := newShortsTestHandler(t, db)
	h.S3 = &stubDownloader{
		status:  storage.ObjectStatus{StorageClass: "STANDARD"},
		objects: map[string][]byte{"shorts/user1/job1/transcript.json": tr},
	}

	resp, _ := h.HandleRequest(context.Background(), events.APIGatewayProxyRequest{
		HTTPMethod: "GET",
		Path:       "/shorts/job1/clips/clip-9/captions",
		Headers:    map[string]string{"Authorization": "Bearer " + generateTestToken("user1")},
	})
	if resp.StatusCode != 200 {
		t.Fatalf("got status %d, want 200 (body: %s)", resp.StatusCode, resp.Body)
	}
	var body struct {
		Start float64 `json:"start"`
		End   float64 `json:"end"`
		Words []struct {
			Text string `json:"text"`
		} `json:"words"`
	}
	if err := json.Unmarshal([]byte(resp.Body), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	// Segment clip-9 spans [10,40]: only "kept" survives windowing.
	if body.Start != 10 || body.End != 40 {
		t.Errorf("expected window [10,40], got [%v,%v]", body.Start, body.End)
	}
	if len(body.Words) != 1 || body.Words[0].Text != "kept" {
		t.Errorf("expected only the in-window word, got %+v", body.Words)
	}
}

func TestGetClipCaptions_FallsBackToConventionalKey(t *testing.T) {
	// Old jobs predate transcriptKey persistence: the conventional location
	// is used when present instead of 400ing.
	tr, _ := json.Marshal(map[string]interface{}{
		"text":  "hi",
		"words": []map[string]interface{}{{"text": "hi", "start": 12.0, "end": 12.5}},
	})
	item := editedJobItem("job1", "completed")
	delete(item, "transcriptKey")
	db := &mockDynamoDB{
		getItemFunc: func(_ context.Context, _ *dynamodb.GetItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
			return &dynamodb.GetItemOutput{Item: item}, nil
		},
	}
	h := newShortsTestHandler(t, db)
	h.S3 = &stubDownloader{
		status:  storage.ObjectStatus{StorageClass: "STANDARD"},
		meta:    storage.ObjectMeta{Found: true, Size: 100, ContentType: "application/json"},
		objects: map[string][]byte{"shorts/user1/job1/transcript.json": tr},
	}

	resp, _ := h.HandleRequest(context.Background(), events.APIGatewayProxyRequest{
		HTTPMethod: "GET",
		Path:       "/shorts/job1/clips/clip-9/captions",
		Headers:    map[string]string{"Authorization": "Bearer " + generateTestToken("user1")},
	})
	if resp.StatusCode != 200 {
		t.Fatalf("got status %d, want 200 (body: %s)", resp.StatusCode, resp.Body)
	}
	if !strings.Contains(resp.Body, `"hi"`) {
		t.Errorf("expected captioned word in response, got %s", resp.Body)
	}
}
