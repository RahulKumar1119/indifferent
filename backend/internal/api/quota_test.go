package api

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/rahul/indifferent/backend/internal/models"
)

// quotaDB is a mockDynamoDB whose UpdateItem is an atomic counter upsert like
// the real "ADD uses :one", recording every key it was asked to count.
func quotaDB() (*mockDynamoDB, map[string]int64) {
	counts := map[string]int64{}
	db := &mockDynamoDB{
		updateItemFunc: func(ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
			if _, ok := params.ExpressionAttributeValues[":one"]; !ok {
				return &dynamodb.UpdateItemOutput{}, nil
			}
			k := params.Key["PK"].(*dbtypes.AttributeValueMemberS).Value + "|" + params.Key["SK"].(*dbtypes.AttributeValueMemberS).Value
			counts[k]++
			return &dynamodb.UpdateItemOutput{Attributes: map[string]dbtypes.AttributeValue{
				"uses": &dbtypes.AttributeValueMemberN{Value: strconv.FormatInt(counts[k], 10)},
			}}, nil
		},
	}
	return db, counts
}

func fixedQuotaClock(t *testing.T, at time.Time) {
	t.Helper()
	old := quotaNow
	quotaNow = func() time.Time { return at }
	t.Cleanup(func() { quotaNow = old })
}

func withLimit(t *testing.T, q *quota, limit int64) {
	t.Helper()
	old := q.limit
	q.limit = limit
	t.Cleanup(func() { q.limit = old })
}

func createShortsReq(token, body string) events.APIGatewayProxyRequest {
	return events.APIGatewayProxyRequest{
		HTTPMethod: "POST",
		Path:       "/shorts",
		Headers:    map[string]string{"Authorization": "Bearer " + token},
		Body:       body,
	}
}

func quotaErrCode(t *testing.T, body string) string {
	t.Helper()
	var e models.APIError
	_ = json.Unmarshal([]byte(body), &e)
	return e.Code
}

func TestQuota_ShortsCreateStopsAtTheDailyLimit(t *testing.T) {
	fixedQuotaClock(t, time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC))
	withLimit(t, &quotaShortsCreate, 3)
	db, _ := quotaDB()
	h := newShortsTestHandler(t, db)
	h.QuotaTable = "sessions-table"
	ctx := context.Background()
	tok := generateTestToken("user1")
	body := `{"fileType":"mp4","duration":60}`

	for i := 1; i <= 3; i++ {
		if resp, _ := h.HandleRequest(ctx, createShortsReq(tok, body)); resp.StatusCode != 201 {
			t.Fatalf("use %d: status %d, want 201 (%s)", i, resp.StatusCode, resp.Body)
		}
	}
	resp, _ := h.HandleRequest(ctx, createShortsReq(tok, body))
	if resp.StatusCode != 429 || quotaErrCode(t, resp.Body) != "QUOTA_EXCEEDED" {
		t.Fatalf("4th use: status %d body %s, want 429 QUOTA_EXCEEDED", resp.StatusCode, resp.Body)
	}
	// 14:00 left in the UTC day.
	if got := resp.Headers["Retry-After"]; got != strconv.Itoa(14*3600) {
		t.Errorf("Retry-After = %q, want %d", got, 14*3600)
	}
	if !strings.Contains(resp.Body, "uploading videos for Shorts") {
		t.Errorf("message does not say what was limited: %s", resp.Body)
	}
}

func TestQuota_IsPerUserAndResetsEachDay(t *testing.T) {
	withLimit(t, &quotaShortsCreate, 1)
	db, counts := quotaDB()
	h := newShortsTestHandler(t, db)
	h.QuotaTable = "sessions-table"
	ctx := context.Background()
	body := `{"fileType":"mp4","duration":60}`

	day1 := time.Date(2026, 11, 6, 23, 59, 0, 0, time.UTC)
	fixedQuotaClock(t, day1)
	if resp, _ := h.HandleRequest(ctx, createShortsReq(generateTestToken("alice"), body)); resp.StatusCode != 201 {
		t.Fatalf("alice first: %d", resp.StatusCode)
	}
	if resp, _ := h.HandleRequest(ctx, createShortsReq(generateTestToken("alice"), body)); resp.StatusCode != 429 {
		t.Fatalf("alice second: %d, want 429", resp.StatusCode)
	}
	if resp, _ := h.HandleRequest(ctx, createShortsReq(generateTestToken("bob"), body)); resp.StatusCode != 201 {
		t.Fatalf("bob must have his own allowance: %d", resp.StatusCode)
	}

	fixedQuotaClock(t, day1.Add(2*time.Minute)) // after UTC midnight
	if resp, _ := h.HandleRequest(ctx, createShortsReq(generateTestToken("alice"), body)); resp.StatusCode != 201 {
		t.Fatalf("alice next day: %d, want 201", resp.StatusCode)
	}

	for _, want := range []string{"QUOTA#alice|shorts-create#20261106", "QUOTA#alice|shorts-create#20261107", "QUOTA#bob|shorts-create#20261106"} {
		if counts[want] == 0 {
			t.Errorf("no counter %q; have %v", want, counts)
		}
	}
}

func TestQuota_OffWithoutATable(t *testing.T) {
	withLimit(t, &quotaShortsCreate, 1)
	db, counts := quotaDB()
	h := newShortsTestHandler(t, db) // QuotaTable empty
	for i := 0; i < 3; i++ {
		if resp, _ := h.HandleRequest(context.Background(), createShortsReq(generateTestToken("u"), `{"fileType":"mp4","duration":60}`)); resp.StatusCode != 201 {
			t.Fatalf("use %d: %d", i, resp.StatusCode)
		}
	}
	if len(counts) != 0 {
		t.Errorf("counters written with no table configured: %v", counts)
	}
}

func TestQuota_AllowsWhenTheCounterCannotBeWritten(t *testing.T) {
	withLimit(t, &quotaShortsCreate, 1)
	db := &mockDynamoDB{
		updateItemFunc: func(ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
			return nil, errors.New("dynamodb is down")
		},
	}
	h := newShortsTestHandler(t, db)
	h.QuotaTable = "sessions-table"
	for i := 0; i < 3; i++ {
		if resp, _ := h.HandleRequest(context.Background(), createShortsReq(generateTestToken("u"), `{"fileType":"mp4","duration":60}`)); resp.StatusCode != 201 {
			t.Fatalf("a counter fault must not block users: use %d got %d", i, resp.StatusCode)
		}
	}
}

func TestQuota_RefusedBeforeAnythingIsCreated(t *testing.T) {
	withLimit(t, &quotaShortsCreate, 1)
	db, _ := quotaDB()
	puts := 0
	db.putItemFunc = func(ctx context.Context, params *dynamodb.PutItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
		puts++
		return &dynamodb.PutItemOutput{}, nil
	}
	h := newShortsTestHandler(t, db)
	h.QuotaTable = "sessions-table"
	body := `{"fileType":"mp4","duration":60}`
	h.HandleRequest(context.Background(), createShortsReq(generateTestToken("u"), body))
	before := puts
	h.HandleRequest(context.Background(), createShortsReq(generateTestToken("u"), body))
	if puts != before {
		t.Error("a refused create still wrote a job row")
	}
}

func TestQuota_RetryAndThumbnailsAreLimited(t *testing.T) {
	withLimit(t, &quotaShortsRetry, 1)
	withLimit(t, &quotaThumbnails, 1)
	db, _ := quotaDB()
	failedJob := models.ShortsJob{UserID: "u", JobID: "j1", Status: "failed", SourceKey: "uploads/u/j1/source.mp4", FileType: "mp4"}
	db.getItemFunc = func(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
		return &dynamodb.GetItemOutput{Item: shortsJobToItem(failedJob)}, nil
	}
	h := newShortsTestHandler(t, db)
	h.QuotaTable = "sessions-table"
	h.ThumbnailStateMachineARN = "arn:aws:states:us-east-1:123456789:stateMachine:thumbs"
	tok := generateTestToken("u")
	post := func(path string) events.APIGatewayProxyResponse {
		resp, _ := h.HandleRequest(context.Background(), events.APIGatewayProxyRequest{
			HTTPMethod: "POST", Path: path, Body: "{}", Headers: map[string]string{"Authorization": "Bearer " + tok},
		})
		return resp
	}

	post("/shorts/j1/retry") // first use is allowed whatever the rest of the handler does
	if resp := post("/shorts/j1/retry"); resp.StatusCode != 429 || quotaErrCode(t, resp.Body) != "QUOTA_EXCEEDED" {
		t.Errorf("second retry: %d %s, want 429 QUOTA_EXCEEDED", resp.StatusCode, resp.Body)
	}

	done := models.ShortsJob{UserID: "u", JobID: "j1", Status: "completed", SourceKey: "uploads/u/j1/source.mp4", FileType: "mp4"}
	db.getItemFunc = func(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
		return &dynamodb.GetItemOutput{Item: shortsJobToItem(done)}, nil
	}
	post("/shorts/j1/thumbnails")
	if resp := post("/shorts/j1/thumbnails"); resp.StatusCode != 429 || quotaErrCode(t, resp.Body) != "QUOTA_EXCEEDED" {
		t.Errorf("second thumbnails start: %d %s, want 429 QUOTA_EXCEEDED", resp.StatusCode, resp.Body)
	}
}

// Issue #286: a VALIDATION failure cannot succeed on the same file, so its
// retry is refused with 409 before the quota is checked, and costs nothing.
func TestRetryShorts_ValidationFailuresAreRefusedWithoutSpendingARetry(t *testing.T) {
	withLimit(t, &quotaShortsRetry, 1)
	db, _ := quotaDB()
	job := models.ShortsJob{UserID: "u", JobID: "j1", Status: "failed", SourceKey: "uploads/u/j1/source.mp4", FileType: "mp4", FailedStage: "ranking"}
	db.getItemFunc = func(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
		return &dynamodb.GetItemOutput{Item: shortsJobToItem(job)}, nil
	}
	var resets int
	db.putItemFunc = func(_ context.Context, params *dynamodb.PutItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
		if v, ok := params.Item["status"].(*dbtypes.AttributeValueMemberS); ok && v.Value == "uploaded" {
			resets++
		}
		return &dynamodb.PutItemOutput{}, nil
	}
	h := newShortsTestHandler(t, db)
	h.QuotaTable = "sessions-table"
	tok := generateTestToken("u")
	retry := func() events.APIGatewayProxyResponse {
		resp, _ := h.HandleRequest(context.Background(), events.APIGatewayProxyRequest{
			HTTPMethod: "POST", Path: "/shorts/j1/retry", Body: "{}", Headers: map[string]string{"Authorization": "Bearer " + tok},
		})
		return resp
	}

	for _, code := range []string{"VALIDATION_NO_SEGMENTS", "VALIDATION_NO_SPEECH", "VALIDATION_DURATION", "VALIDATION_UPLOAD"} {
		job.ErrorCode = code
		for i := 0; i < 2; i++ {
			resp := retry()
			if resp.StatusCode != 409 || quotaErrCode(t, resp.Body) != "NOT_RETRYABLE" {
				t.Fatalf("%s retry %d: %d %s, want 409 NOT_RETRYABLE", code, i+1, resp.StatusCode, resp.Body)
			}
			if strings.Contains(resp.Body, code) {
				t.Errorf("%s: the refusal shows the internal code to the user: %s", code, resp.Body)
			}
		}
	}
	if resets != 0 {
		t.Fatalf("a refused retry reset the job %d times", resets)
	}

	// None of those spent the one allowed retry: a retryable failure still gets it.
	job.ErrorCode = "INFRASTRUCTURE"
	if resp := retry(); resp.StatusCode != 200 {
		t.Fatalf("retryable failure after refused retries: %d %s, want 200", resp.StatusCode, resp.Body)
	}
	if resets != 1 {
		t.Errorf("resets = %d, want 1", resets)
	}
}

func TestQuota_ProjectCreateAndUploadURLAreLimited(t *testing.T) {
	withLimit(t, &quotaProjectCreate, 1)
	withLimit(t, &quotaUploadURL, 1)
	db, _ := quotaDB()
	proj := models.Project{UserID: "u", ProjectID: "p1", Name: "n", Status: "created"}
	db.getItemFunc = func(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
		return &dynamodb.GetItemOutput{Item: projectToItem(proj)}, nil
	}
	h := newShortsTestHandler(t, db)
	h.QuotaTable = "sessions-table"
	tok := generateTestToken("u")
	post := func(path, body string) events.APIGatewayProxyResponse {
		resp, _ := h.HandleRequest(context.Background(), events.APIGatewayProxyRequest{
			HTTPMethod: "POST", Path: path, Body: body, Headers: map[string]string{"Authorization": "Bearer " + tok},
		})
		return resp
	}

	if resp := post("/projects/p1/upload", ""); resp.StatusCode != 200 {
		t.Fatalf("first upload url: %d %s", resp.StatusCode, resp.Body)
	}
	if resp := post("/projects/p1/upload", ""); resp.StatusCode != 429 {
		t.Errorf("second upload url: %d, want 429", resp.StatusCode)
	}
	if resp := post("/projects/p1/logo", ""); resp.StatusCode != 429 {
		t.Errorf("logo url shares the upload allowance: %d, want 429", resp.StatusCode)
	}

	createBody := `{"name":"My video","template":"classic","voice":"Joanna"}`
	if first := post("/projects", createBody); first.StatusCode != 201 {
		t.Fatalf("first project create: %d %s", first.StatusCode, first.Body)
	}
	if resp := post("/projects", createBody); resp.StatusCode != 429 {
		t.Errorf("second project create: %d %s, want 429", resp.StatusCode, resp.Body)
	}
}

func TestUploadSize_SignedWhenDeclaredAndRefusedWhenTooBig(t *testing.T) {
	db := &mockDynamoDB{}
	h := newShortsTestHandler(t, db)
	stub := h.S3.(*stubDownloader)
	tok := generateTestToken("u")

	resp, _ := h.HandleRequest(context.Background(), createShortsReq(tok, `{"fileType":"mp4","duration":60,"size":1048576}`))
	if resp.StatusCode != 201 {
		t.Fatalf("status %d: %s", resp.StatusCode, resp.Body)
	}
	if len(stub.sizedUploads) != 1 || stub.sizedUploads[0].size != 1048576 {
		t.Fatalf("sized uploads = %+v, want one of 1048576 bytes", stub.sizedUploads)
	}
	if !strings.HasPrefix(stub.sizedUploads[0].key, "uploads/u/") {
		t.Errorf("signed key = %q, want it under the caller's prefix", stub.sizedUploads[0].key)
	}

	// No size: unsized URL, as older web app versions expect.
	stub.sizedUploads = nil
	if resp, _ := h.HandleRequest(context.Background(), createShortsReq(tok, `{"fileType":"mp4","duration":60}`)); resp.StatusCode != 201 {
		t.Fatalf("unsized create: %d", resp.StatusCode)
	}
	if len(stub.sizedUploads) != 0 {
		t.Error("an undeclared size must not produce a sized URL")
	}

	// Over the 2 GiB cap and negative: refused with nothing signed.
	for name, body := range map[string]string{
		"over cap": `{"fileType":"mp4","duration":60,"size":2147483649}`,
		"negative": `{"fileType":"mp4","duration":60,"size":-1}`,
	} {
		resp, _ := h.HandleRequest(context.Background(), createShortsReq(tok, body))
		if resp.StatusCode != 400 {
			t.Errorf("%s: status %d, want 400 (%s)", name, resp.StatusCode, resp.Body)
		}
	}
	if len(stub.sizedUploads) != 0 {
		t.Error("a refused size was still signed")
	}
	at := `{"fileType":"mp4","duration":60,"size":2147483648}`
	if resp, _ := h.HandleRequest(context.Background(), createShortsReq(tok, at)); resp.StatusCode != 201 {
		t.Errorf("exactly 2 GiB must be accepted: %d", resp.StatusCode)
	}
}

func TestUploadSize_TextAndLogo(t *testing.T) {
	db, _ := quotaDB()
	proj := models.Project{UserID: "u", ProjectID: "p1", Name: "n", Status: "created"}
	db.getItemFunc = func(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
		return &dynamodb.GetItemOutput{Item: projectToItem(proj)}, nil
	}
	h := newShortsTestHandler(t, db)
	stub := h.S3.(*stubDownloader)
	tok := generateTestToken("u")
	post := func(path, body string) events.APIGatewayProxyResponse {
		resp, _ := h.HandleRequest(context.Background(), events.APIGatewayProxyRequest{
			HTTPMethod: "POST", Path: path, Body: body, Headers: map[string]string{"Authorization": "Bearer " + tok},
		})
		return resp
	}

	if resp := post("/projects/p1/upload", `{"fileName":"a.txt","contentType":"text/plain","size":4096}`); resp.StatusCode != 200 {
		t.Fatalf("text upload: %d %s", resp.StatusCode, resp.Body)
	}
	if resp := post("/projects/p1/upload", `{"size":5242881}`); resp.StatusCode != 400 || quotaErrCode(t, resp.Body) != "UPLOAD_TOO_LARGE" {
		t.Errorf("6 MB text: %d %s, want 400 UPLOAD_TOO_LARGE", resp.StatusCode, resp.Body)
	}
	if resp := post("/projects/p1/logo", `{"size":5242881}`); resp.StatusCode != 400 {
		t.Errorf("6 MB logo: %d, want 400", resp.StatusCode)
	}
	if resp := post("/projects/p1/logo", `{"size":2048}`); resp.StatusCode != 200 {
		t.Errorf("2 KB logo: %d %s", resp.StatusCode, resp.Body)
	}
	got := map[string]int64{}
	for _, s := range stub.sizedUploads {
		got[s.key] = s.size
	}
	if got["uploads/u/p1/input.txt"] != 4096 || got["uploads/u/p1/logo.png"] != 2048 || len(got) != 2 {
		t.Errorf("signed uploads = %v", got)
	}
}
