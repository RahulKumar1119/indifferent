package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	"github.com/rahul/indifferent/backend/internal/models"
	"github.com/rahul/indifferent/backend/internal/shorts"
	"github.com/rahul/indifferent/backend/internal/storage"
)

const (
	mpJobKey = "uploads/user1/job1/source.mp4"
	mpMB     = 1 << 20
)

// multipartHarness serves one job owned by user1 and records S3 calls.
type multipartHarness struct {
	h  *APIHandler
	s3 *stubDownloader
}

func newMultipartHarness(t *testing.T, status string, s3 *stubDownloader) *multipartHarness {
	t.Helper()
	item := shortsJobItem("job1", status)
	db := &mockDynamoDB{
		getItemFunc: func(_ context.Context, in *dynamodb.GetItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
			// Rows are keyed by owner: another user's lookup finds nothing.
			if in.Key["PK"].(*dbtypes.AttributeValueMemberS).Value != "USER#user1" ||
				in.Key["SK"].(*dbtypes.AttributeValueMemberS).Value != "SHORTS#job1" {
				return &dynamodb.GetItemOutput{}, nil
			}
			return &dynamodb.GetItemOutput{Item: item}, nil
		},
	}
	h := newShortsTestHandler(t, db)
	h.S3 = s3
	return &multipartHarness{h: h, s3: s3}
}

func (m *multipartHarness) call(method, path, body, user string, query map[string]string) events.APIGatewayProxyResponse {
	resp, _ := m.h.HandleRequest(context.Background(), events.APIGatewayProxyRequest{
		HTTPMethod:            method,
		Path:                  path,
		Headers:               map[string]string{"Authorization": "Bearer " + generateTestToken(user)},
		Body:                  body,
		QueryStringParameters: query,
	})
	return resp
}

func errCode(t *testing.T, resp events.APIGatewayProxyResponse) string {
	t.Helper()
	var apiErr models.APIError
	if err := json.Unmarshal([]byte(resp.Body), &apiErr); err != nil {
		t.Fatalf("response is not an API error: %s", resp.Body)
	}
	return apiErr.Code
}

func uploaded(n int, size int64) []storage.UploadedPart {
	parts := make([]storage.UploadedPart, n)
	for i := range parts {
		parts[i] = storage.UploadedPart{PartNumber: int32(i + 1), ETag: fmt.Sprintf(`"etag%d"`, i+1), Size: size}
	}
	return parts
}

func completeBody(uploadID string, n int) string {
	parts := make([]MultipartCompletePart, n)
	for i := range parts {
		parts[i] = MultipartCompletePart{PartNumber: i + 1, ETag: fmt.Sprintf(`"etag%d"`, i+1)}
	}
	b, _ := json.Marshal(MultipartCompleteRequest{UploadID: uploadID, Parts: parts})
	return string(b)
}

func TestParseMultipartPath(t *testing.T) {
	tests := []struct {
		path   string
		want   multipartRoute
		wantOK bool
	}{
		{"/shorts/j1/upload/multipart/initiate", multipartRoute{"j1", "initiate"}, true},
		{"/shorts/j1/upload/multipart/parts", multipartRoute{"j1", "parts"}, true},
		{"/shorts/j1/upload/multipart/complete", multipartRoute{"j1", "complete"}, true},
		{"/shorts/j1/upload/multipart", multipartRoute{"j1", ""}, true},
		{"/shorts/j1/upload/multipart/", multipartRoute{"j1", ""}, true},
		{"/shorts/j1/upload/multipart/other", multipartRoute{}, false},
		{"/shorts/j1/upload/multipart/parts/x", multipartRoute{}, false},
		{"/shorts//upload/multipart", multipartRoute{}, false},
		{"/shorts/j1/upload", multipartRoute{}, false},
		{"/projects/j1/upload/multipart", multipartRoute{}, false},
		{"/shorts/j1/start", multipartRoute{}, false},
	}
	for _, tt := range tests {
		got, ok := parseMultipartPath(tt.path)
		if ok != tt.wantOK || got != tt.want {
			t.Errorf("parseMultipartPath(%q) = %+v, %v; want %+v, %v", tt.path, got, ok, tt.want, tt.wantOK)
		}
	}
}

func TestMultipart_RequiresAuth(t *testing.T) {
	m := newMultipartHarness(t, "uploaded", &stubDownloader{})
	resp, _ := m.h.HandleRequest(context.Background(), events.APIGatewayProxyRequest{
		HTTPMethod: "POST",
		Path:       "/shorts/job1/upload/multipart/initiate",
		Body:       `{"size":1}`,
	})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("got %d, want 401", resp.StatusCode)
	}
	if len(m.s3.mpCalls) != 0 {
		t.Errorf("S3 touched without auth: %v", m.s3.mpCalls)
	}
}

func TestMultipart_WrongMethod(t *testing.T) {
	m := newMultipartHarness(t, "uploaded", &stubDownloader{})
	for _, tt := range []struct{ method, path string }{
		{"GET", "/shorts/job1/upload/multipart/initiate"},
		{"PUT", "/shorts/job1/upload/multipart"},
		{"POST", "/shorts/job1/upload/multipart"},
		{"DELETE", "/shorts/job1/upload/multipart/complete"},
	} {
		if got := m.call(tt.method, tt.path, "{}", "user1", nil).StatusCode; got != http.StatusMethodNotAllowed {
			t.Errorf("%s %s = %d, want 405", tt.method, tt.path, got)
		}
	}
}

func TestMultipartInitiate(t *testing.T) {
	m := newMultipartHarness(t, "uploaded", &stubDownloader{mpUploadID: "up-123"})
	resp := m.call("POST", "/shorts/job1/upload/multipart/initiate", fmt.Sprintf(`{"size":%d}`, 100*mpMB), "user1", nil)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("got %d: %s", resp.StatusCode, resp.Body)
	}
	var out MultipartInitiateResponse
	_ = json.Unmarshal([]byte(resp.Body), &out)
	if out.UploadID != "up-123" || out.PartSize != shorts.MultipartPartSize || out.PartCount != 7 {
		t.Errorf("response = %+v, want upload up-123, part size 16 MiB, 7 parts", out)
	}
	// The key and content type come from the stored job.
	if got := m.s3.mpCalls; len(got) != 1 || got[0] != "create|"+mpJobKey+"|video/mp4" {
		t.Errorf("S3 calls = %v", got)
	}
}

func TestMultipartInitiate_PartCounts(t *testing.T) {
	for _, tt := range []struct {
		size int64
		want int
	}{
		{1, 1},
		{shorts.MultipartPartSize, 1},
		{shorts.MultipartPartSize + 1, 2},
		{shorts.MaxUploadBytes, 128},
	} {
		m := newMultipartHarness(t, "uploaded", &stubDownloader{})
		resp := m.call("POST", "/shorts/job1/upload/multipart/initiate", fmt.Sprintf(`{"size":%d}`, tt.size), "user1", nil)
		var out MultipartInitiateResponse
		_ = json.Unmarshal([]byte(resp.Body), &out)
		if resp.StatusCode != http.StatusCreated || out.PartCount != tt.want {
			t.Errorf("size %d: status %d, parts %d, want 201 and %d", tt.size, resp.StatusCode, out.PartCount, tt.want)
		}
	}
}

func TestMultipartInitiate_Rejects(t *testing.T) {
	tests := []struct {
		name     string
		status   string
		s3       *stubDownloader
		user     string
		body     string
		wantHTTP int
		wantCode string
	}{
		{"bad json", "uploaded", &stubDownloader{}, "user1", `nope`, 400, "INVALID_BODY"},
		{"zero size", "uploaded", &stubDownloader{}, "user1", `{"size":0}`, 400, "VALIDATION_ERROR"},
		{"negative size", "uploaded", &stubDownloader{}, "user1", `{"size":-5}`, 400, "VALIDATION_ERROR"},
		{"missing size", "uploaded", &stubDownloader{}, "user1", `{}`, 400, "VALIDATION_ERROR"},
		{"over the cap", "uploaded", &stubDownloader{}, "user1", fmt.Sprintf(`{"size":%d}`, int64(shorts.MaxUploadBytes)+1), 400, "UPLOAD_TOO_LARGE"},
		{"other user's job", "uploaded", &stubDownloader{}, "user2", `{"size":1000}`, 404, "NOT_FOUND"},
		{"pipeline running", "transcribing", &stubDownloader{}, "user1", `{"size":1000}`, 400, "INVALID_STATE"},
		{"completed job", "completed", &stubDownloader{}, "user1", `{"size":1000}`, 400, "INVALID_STATE"},
		{"source already landed", "uploaded", &stubDownloader{meta: storage.ObjectMeta{Found: true, Size: 5}}, "user1", `{"size":1000}`, 409, "ALREADY_UPLOADED"},
		{"head fails", "uploaded", &stubDownloader{metaErr: fmt.Errorf("boom")}, "user1", `{"size":1000}`, 500, "S3_ERROR"},
		{"create fails", "uploaded", &stubDownloader{mpCreateErr: fmt.Errorf("boom")}, "user1", `{"size":1000}`, 500, "S3_ERROR"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newMultipartHarness(t, tt.status, tt.s3)
			resp := m.call("POST", "/shorts/job1/upload/multipart/initiate", tt.body, tt.user, nil)
			if resp.StatusCode != tt.wantHTTP || errCode(t, resp) != tt.wantCode {
				t.Fatalf("got %d %s, want %d %s", resp.StatusCode, resp.Body, tt.wantHTTP, tt.wantCode)
			}
			for _, c := range m.s3.mpCalls {
				if strings.HasPrefix(c, "create|") && tt.name != "create fails" {
					t.Errorf("upload created despite rejection: %v", m.s3.mpCalls)
				}
			}
		})
	}
}

func TestMultipartParts(t *testing.T) {
	m := newMultipartHarness(t, "uploaded", &stubDownloader{})
	resp := m.call("POST", "/shorts/job1/upload/multipart/parts", `{"uploadId":"up-1","partNumbers":[3,4,10000]}`, "user1", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("got %d: %s", resp.StatusCode, resp.Body)
	}
	var out MultipartPartsResponse
	_ = json.Unmarshal([]byte(resp.Body), &out)
	if len(out.URLs) != 3 || out.URLs[0].PartNumber != 3 || out.URLs[2].PartNumber != 10000 {
		t.Fatalf("urls = %+v", out.URLs)
	}
	if out.ExpiresInSeconds != 3600 {
		t.Errorf("expiresInSeconds = %d, want 3600", out.ExpiresInSeconds)
	}
	// Every URL is signed for the job's own key with the one hour expiry, not
	// the 15 minute window of the single PUT.
	want := fmt.Sprintf("partURL|%s|up-1|3|1h0m0s", mpJobKey)
	if m.s3.mpCalls[0] != want {
		t.Errorf("first call = %q, want %q", m.s3.mpCalls[0], want)
	}
}

// The server flag decides whether part URLs are accelerated. A client can opt
// out with accelerate:false but can never switch acceleration on.
func TestMultipartParts_Accelerate(t *testing.T) {
	tests := []struct {
		name  string
		flag  bool
		body  string
		accel bool
	}{
		{"flag off, field omitted", false, `{"uploadId":"u","partNumbers":[1,2]}`, false},
		{"flag off, client asks for it", false, `{"uploadId":"u","partNumbers":[1,2],"accelerate":true}`, false},
		{"flag on, field omitted", true, `{"uploadId":"u","partNumbers":[1,2]}`, true},
		{"flag on, accelerate true", true, `{"uploadId":"u","partNumbers":[1,2],"accelerate":true}`, true},
		{"flag on, client opts out", true, `{"uploadId":"u","partNumbers":[1,2],"accelerate":false}`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newMultipartHarness(t, "uploaded", &stubDownloader{})
			m.h.S3Accelerate = tt.flag
			resp := m.call("POST", "/shorts/job1/upload/multipart/parts", tt.body, "user1", nil)
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("got %d: %s", resp.StatusCode, resp.Body)
			}
			var raw map[string]json.RawMessage
			_ = json.Unmarshal([]byte(resp.Body), &raw)
			if got := string(raw["accelerated"]); got != fmt.Sprint(tt.accel) {
				t.Errorf("accelerated = %s, want %v", got, tt.accel)
			}
			if len(m.s3.mpAccel) != 2 {
				t.Fatalf("signed %d URLs, want 2", len(m.s3.mpAccel))
			}
			for i, got := range m.s3.mpAccel {
				if got != tt.accel {
					t.Errorf("URL %d signed with accelerate=%v, want %v", i, got, tt.accel)
				}
			}
			// The old fields keep their shape.
			if _, ok := raw["urls"]; !ok {
				t.Errorf("urls missing from %s", resp.Body)
			}
			if got := string(raw["expiresInSeconds"]); got != "3600" {
				t.Errorf("expiresInSeconds = %s, want 3600", got)
			}
		})
	}
}

func TestMultipartParts_Rejects(t *testing.T) {
	many := make([]string, shorts.MaxPartURLBatch+1)
	for i := range many {
		many[i] = fmt.Sprint(i + 1)
	}
	tests := []struct {
		name     string
		status   string
		user     string
		body     string
		wantHTTP int
		wantCode string
	}{
		{"bad json", "uploaded", "user1", `x`, 400, "INVALID_BODY"},
		{"no upload id", "uploaded", "user1", `{"partNumbers":[1]}`, 400, "VALIDATION_ERROR"},
		{"upload id with spaces", "uploaded", "user1", `{"uploadId":"a b","partNumbers":[1]}`, 400, "VALIDATION_ERROR"},
		{"upload id with a slash path", "uploaded", "user1", `{"uploadId":"a?x=1&y","partNumbers":[1]}`, 400, "VALIDATION_ERROR"},
		{"empty list", "uploaded", "user1", `{"uploadId":"u","partNumbers":[]}`, 400, "VALIDATION_ERROR"},
		{"part zero", "uploaded", "user1", `{"uploadId":"u","partNumbers":[0]}`, 400, "VALIDATION_ERROR"},
		{"part negative", "uploaded", "user1", `{"uploadId":"u","partNumbers":[-1]}`, 400, "VALIDATION_ERROR"},
		{"part 10001", "uploaded", "user1", `{"uploadId":"u","partNumbers":[10001]}`, 400, "VALIDATION_ERROR"},
		{"duplicate", "uploaded", "user1", `{"uploadId":"u","partNumbers":[2,2]}`, 400, "VALIDATION_ERROR"},
		{"batch too big", "uploaded", "user1", `{"uploadId":"u","partNumbers":[` + strings.Join(many, ",") + `]}`, 400, "VALIDATION_ERROR"},
		{"other user's job", "uploaded", "user2", `{"uploadId":"u","partNumbers":[1]}`, 404, "NOT_FOUND"},
		{"pipeline running", "ranking", "user1", `{"uploadId":"u","partNumbers":[1]}`, 400, "INVALID_STATE"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newMultipartHarness(t, tt.status, &stubDownloader{})
			resp := m.call("POST", "/shorts/job1/upload/multipart/parts", tt.body, tt.user, nil)
			if resp.StatusCode != tt.wantHTTP || errCode(t, resp) != tt.wantCode {
				t.Fatalf("got %d %s, want %d %s", resp.StatusCode, resp.Body, tt.wantHTTP, tt.wantCode)
			}
			if len(m.s3.mpCalls) != 0 {
				t.Errorf("URLs signed despite rejection: %v", m.s3.mpCalls)
			}
		})
	}
}

func TestMultipartParts_BatchAtCapIsAccepted(t *testing.T) {
	nums := make([]string, shorts.MaxPartURLBatch)
	for i := range nums {
		nums[i] = fmt.Sprint(i + 1)
	}
	m := newMultipartHarness(t, "uploaded", &stubDownloader{})
	resp := m.call("POST", "/shorts/job1/upload/multipart/parts", `{"uploadId":"u","partNumbers":[`+strings.Join(nums, ",")+`]}`, "user1", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("got %d: %s", resp.StatusCode, resp.Body)
	}
}

func TestMultipartComplete(t *testing.T) {
	s3 := &stubDownloader{
		mpParts: uploaded(3, 16*mpMB),
		meta:    storage.ObjectMeta{Found: true, Size: 48 * mpMB, ContentType: "video/mp4"},
	}
	m := newMultipartHarness(t, "uploaded", s3)
	resp := m.call("POST", "/shorts/job1/upload/multipart/complete", completeBody("up-1", 3), "user1", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("got %d: %s", resp.StatusCode, resp.Body)
	}
	want := []string{"list|" + mpJobKey + "|up-1", "complete|" + mpJobKey + "|up-1|3"}
	if fmt.Sprint(s3.mpCalls) != fmt.Sprint(want) {
		t.Errorf("S3 calls = %v, want %v", s3.mpCalls, want)
	}
}

func TestMultipartComplete_AcceptsUnquotedETags(t *testing.T) {
	s3 := &stubDownloader{mpParts: uploaded(1, 5*mpMB), meta: storage.ObjectMeta{Found: true, Size: 5 * mpMB}}
	m := newMultipartHarness(t, "uploaded", s3)
	resp := m.call("POST", "/shorts/job1/upload/multipart/complete", `{"uploadId":"u","parts":[{"partNumber":1,"etag":"etag1"}]}`, "user1", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("got %d: %s", resp.StatusCode, resp.Body)
	}
}

func TestMultipartComplete_RunsExistingVerification(t *testing.T) {
	tests := []struct {
		name     string
		meta     storage.ObjectMeta
		wantCode string
	}{
		{"empty object", storage.ObjectMeta{Found: true, Size: 0}, "EMPTY_UPLOAD"},
		{"wrong content type", storage.ObjectMeta{Found: true, Size: 5, ContentType: "audio/mpeg"}, "UPLOAD_TYPE_MISMATCH"},
		{"object missing", storage.ObjectMeta{}, "UPLOAD_NOT_FOUND"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newMultipartHarness(t, "uploaded", &stubDownloader{mpParts: uploaded(1, 5*mpMB), meta: tt.meta})
			resp := m.call("POST", "/shorts/job1/upload/multipart/complete", completeBody("u", 1), "user1", nil)
			if resp.StatusCode != 400 || errCode(t, resp) != tt.wantCode {
				t.Fatalf("got %d %s, want 400 %s", resp.StatusCode, resp.Body, tt.wantCode)
			}
		})
	}
}

func TestMultipartComplete_RejectsBadLists(t *testing.T) {
	tooMany := make([]MultipartCompletePart, shorts.MaxMultipartParts+1)
	for i := range tooMany {
		tooMany[i] = MultipartCompletePart{PartNumber: i + 1, ETag: "e"}
	}
	tooManyBody, _ := json.Marshal(MultipartCompleteRequest{UploadID: "u", Parts: tooMany})
	tests := []struct {
		name string
		body string
	}{
		{"bad json", `x`},
		{"no upload id", `{"parts":[{"partNumber":1,"etag":"e"}]}`},
		{"no parts", `{"uploadId":"u","parts":[]}`},
		{"more than 10000 parts", string(tooManyBody)},
		{"unsorted", `{"uploadId":"u","parts":[{"partNumber":2,"etag":"e"},{"partNumber":1,"etag":"e"}]}`},
		{"gap", `{"uploadId":"u","parts":[{"partNumber":1,"etag":"e"},{"partNumber":3,"etag":"e"}]}`},
		{"duplicate", `{"uploadId":"u","parts":[{"partNumber":1,"etag":"e"},{"partNumber":1,"etag":"e"}]}`},
		{"starts at zero", `{"uploadId":"u","parts":[{"partNumber":0,"etag":"e"}]}`},
		{"empty etag", `{"uploadId":"u","parts":[{"partNumber":1,"etag":""}]}`},
		{"etag with markup", `{"uploadId":"u","parts":[{"partNumber":1,"etag":"<x>"}]}`},
		{"etag too long", `{"uploadId":"u","parts":[{"partNumber":1,"etag":"` + strings.Repeat("a", 129) + `"}]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newMultipartHarness(t, "uploaded", &stubDownloader{})
			resp := m.call("POST", "/shorts/job1/upload/multipart/complete", tt.body, "user1", nil)
			if resp.StatusCode != 400 {
				t.Fatalf("got %d: %s", resp.StatusCode, resp.Body)
			}
			if len(m.s3.mpCalls) != 0 {
				t.Errorf("S3 called despite a bad list: %v", m.s3.mpCalls)
			}
		})
	}
}

func TestMultipartComplete_RejectsWrongJob(t *testing.T) {
	m := newMultipartHarness(t, "uploaded", &stubDownloader{})
	if got := m.call("POST", "/shorts/job1/upload/multipart/complete", completeBody("u", 1), "user2", nil).StatusCode; got != 404 {
		t.Errorf("other user: got %d, want 404", got)
	}
	m = newMultipartHarness(t, "rendering", &stubDownloader{})
	resp := m.call("POST", "/shorts/job1/upload/multipart/complete", completeBody("u", 1), "user1", nil)
	if resp.StatusCode != 400 || errCode(t, resp) != "INVALID_STATE" {
		t.Errorf("running job: got %d %s", resp.StatusCode, resp.Body)
	}
	if len(m.s3.mpCalls) != 0 {
		t.Errorf("S3 called: %v", m.s3.mpCalls)
	}
}

func TestMultipartComplete_PartsMustMatchS3(t *testing.T) {
	tests := []struct {
		name   string
		stored []storage.UploadedPart
		n      int
	}{
		{"part never uploaded", uploaded(2, 16*mpMB), 3},
		{"etag differs", append(uploaded(1, 16*mpMB), storage.UploadedPart{PartNumber: 2, ETag: `"other"`, Size: 5}), 2},
		{"nothing stored", nil, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s3 := &stubDownloader{mpParts: tt.stored}
			m := newMultipartHarness(t, "uploaded", s3)
			resp := m.call("POST", "/shorts/job1/upload/multipart/complete", completeBody("u", tt.n), "user1", nil)
			if resp.StatusCode != 400 || errCode(t, resp) != "INVALID_PARTS" {
				t.Fatalf("got %d %s, want 400 INVALID_PARTS", resp.StatusCode, resp.Body)
			}
			for _, c := range s3.mpCalls {
				if strings.HasPrefix(c, "complete|") {
					t.Errorf("completed despite a mismatch: %v", s3.mpCalls)
				}
			}
		})
	}
}

func TestMultipartComplete_OversizeIsRefusedBeforeTheObjectExists(t *testing.T) {
	// 129 parts of 16 MiB is one part over 2 GiB; the client's declared size
	// at initiate cannot be trusted, so the stored parts decide.
	s3 := &stubDownloader{mpParts: uploaded(129, 16*mpMB)}
	m := newMultipartHarness(t, "uploaded", s3)
	resp := m.call("POST", "/shorts/job1/upload/multipart/complete", completeBody("up-9", 129), "user1", nil)
	if resp.StatusCode != 400 || errCode(t, resp) != "UPLOAD_TOO_LARGE" {
		t.Fatalf("got %d %s, want 400 UPLOAD_TOO_LARGE", resp.StatusCode, resp.Body)
	}
	last := s3.mpCalls[len(s3.mpCalls)-1]
	if last != "abort|"+mpJobKey+"|up-9" {
		t.Errorf("upload not aborted: %v", s3.mpCalls)
	}
	for _, c := range s3.mpCalls {
		if strings.HasPrefix(c, "complete|") {
			t.Errorf("completed an oversize upload: %v", s3.mpCalls)
		}
	}
}

func TestMultipartComplete_AtTheCapIsAccepted(t *testing.T) {
	s3 := &stubDownloader{
		mpParts: uploaded(128, 16*mpMB),
		meta:    storage.ObjectMeta{Found: true, Size: shorts.MaxUploadBytes},
	}
	m := newMultipartHarness(t, "uploaded", s3)
	resp := m.call("POST", "/shorts/job1/upload/multipart/complete", completeBody("u", 128), "user1", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("got %d: %s", resp.StatusCode, resp.Body)
	}
}

func TestMultipartComplete_S3Failures(t *testing.T) {
	tests := []struct {
		name     string
		s3       *stubDownloader
		wantHTTP int
		wantCode string
	}{
		{"list fails", &stubDownloader{mpListErr: fmt.Errorf("boom")}, 500, "S3_ERROR"},
		{"complete fails", &stubDownloader{mpParts: uploaded(1, 5*mpMB), mpCompleteErr: fmt.Errorf("boom")}, 500, "S3_ERROR"},
		{"s3 rejects the parts", &stubDownloader{mpParts: uploaded(1, 5*mpMB), mpCompleteErr: &stubAPIError{code: "InvalidPart"}}, 400, "INVALID_PARTS"},
		{"s3 says part too small", &stubDownloader{mpParts: uploaded(1, 5*mpMB), mpCompleteErr: &stubAPIError{code: "EntityTooSmall"}}, 400, "INVALID_PARTS"},
		{"upload gone and no object", &stubDownloader{mpListErr: &types.NoSuchUpload{}}, 404, "UPLOAD_SESSION_NOT_FOUND"},
		{"upload gone during complete, no object", &stubDownloader{mpParts: uploaded(1, 5*mpMB), mpCompleteErr: &types.NoSuchUpload{}}, 404, "UPLOAD_SESSION_NOT_FOUND"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newMultipartHarness(t, "uploaded", tt.s3)
			resp := m.call("POST", "/shorts/job1/upload/multipart/complete", completeBody("u", 1), "user1", nil)
			if resp.StatusCode != tt.wantHTTP || errCode(t, resp) != tt.wantCode {
				t.Fatalf("got %d %s, want %d %s", resp.StatusCode, resp.Body, tt.wantHTTP, tt.wantCode)
			}
		})
	}
}

// A lost response to a successful complete, retried, finds no open upload but
// a verified source: that is success, not a reason to send the file again.
func TestMultipartComplete_RetryAfterSuccessIsIdempotent(t *testing.T) {
	s3 := &stubDownloader{
		mpListErr: &types.NoSuchUpload{},
		meta:      storage.ObjectMeta{Found: true, Size: 48 * mpMB, ContentType: "video/mp4"},
	}
	m := newMultipartHarness(t, "uploaded", s3)
	resp := m.call("POST", "/shorts/job1/upload/multipart/complete", completeBody("u", 3), "user1", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("got %d: %s", resp.StatusCode, resp.Body)
	}
}

func TestMultipartList(t *testing.T) {
	s3 := &stubDownloader{mpParts: uploaded(2, 16*mpMB)}
	m := newMultipartHarness(t, "uploaded", s3)
	resp := m.call("GET", "/shorts/job1/upload/multipart", "", "user1", map[string]string{"uploadId": "up-1"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("got %d: %s", resp.StatusCode, resp.Body)
	}
	var out MultipartListResponse
	_ = json.Unmarshal([]byte(resp.Body), &out)
	if len(out.Parts) != 2 || out.Parts[1].PartNumber != 2 || out.Parts[1].Size != 16*mpMB || out.Parts[1].ETag != `"etag2"` {
		t.Errorf("parts = %+v", out.Parts)
	}
	if s3.mpCalls[0] != "list|"+mpJobKey+"|up-1" {
		t.Errorf("calls = %v", s3.mpCalls)
	}
}

func TestMultipartList_EmptyIsAnArray(t *testing.T) {
	m := newMultipartHarness(t, "uploaded", &stubDownloader{})
	resp := m.call("GET", "/shorts/job1/upload/multipart", "", "user1", map[string]string{"uploadId": "up-1"})
	if !strings.Contains(resp.Body, `"parts":[]`) {
		t.Errorf("body = %s, want an empty array", resp.Body)
	}
}

func TestMultipartList_Rejects(t *testing.T) {
	q := map[string]string{"uploadId": "up-1"}
	m := newMultipartHarness(t, "uploaded", &stubDownloader{})
	if got := m.call("GET", "/shorts/job1/upload/multipart", "", "user1", nil).StatusCode; got != 400 {
		t.Errorf("no uploadId: got %d, want 400", got)
	}
	if got := m.call("GET", "/shorts/job1/upload/multipart", "", "user2", q).StatusCode; got != 404 {
		t.Errorf("other user: got %d, want 404", got)
	}
	gone := newMultipartHarness(t, "uploaded", &stubDownloader{mpListErr: &types.NoSuchUpload{}})
	resp := gone.call("GET", "/shorts/job1/upload/multipart", "", "user1", q)
	if resp.StatusCode != 404 || errCode(t, resp) != "UPLOAD_SESSION_NOT_FOUND" {
		t.Errorf("gone upload: got %d %s", resp.StatusCode, resp.Body)
	}
	running := newMultipartHarness(t, "rendering", &stubDownloader{})
	if got := running.call("GET", "/shorts/job1/upload/multipart", "", "user1", q).StatusCode; got != 400 {
		t.Errorf("running job: got %d, want 400", got)
	}
}

func TestMultipartAbort(t *testing.T) {
	s3 := &stubDownloader{}
	m := newMultipartHarness(t, "uploaded", s3)
	resp := m.call("DELETE", "/shorts/job1/upload/multipart", "", "user1", map[string]string{"uploadId": "up-1"})
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("got %d: %s", resp.StatusCode, resp.Body)
	}
	if len(s3.mpCalls) != 1 || s3.mpCalls[0] != "abort|"+mpJobKey+"|up-1" {
		t.Errorf("calls = %v", s3.mpCalls)
	}
}

func TestMultipartAbort_Cases(t *testing.T) {
	q := map[string]string{"uploadId": "up-1"}
	t.Run("already gone is fine", func(t *testing.T) {
		m := newMultipartHarness(t, "uploaded", &stubDownloader{mpAbortErr: &types.NoSuchUpload{}})
		if got := m.call("DELETE", "/shorts/job1/upload/multipart", "", "user1", q).StatusCode; got != 204 {
			t.Errorf("got %d, want 204", got)
		}
	})
	t.Run("works whatever the job state", func(t *testing.T) {
		m := newMultipartHarness(t, "failed", &stubDownloader{})
		if got := m.call("DELETE", "/shorts/job1/upload/multipart", "", "user1", q).StatusCode; got != 204 {
			t.Errorf("got %d, want 204", got)
		}
	})
	t.Run("other user's job", func(t *testing.T) {
		m := newMultipartHarness(t, "uploaded", &stubDownloader{})
		if got := m.call("DELETE", "/shorts/job1/upload/multipart", "", "user2", q).StatusCode; got != 404 {
			t.Errorf("got %d, want 404", got)
		}
		if len(m.s3.mpCalls) != 0 {
			t.Errorf("S3 touched: %v", m.s3.mpCalls)
		}
	})
	t.Run("no upload id", func(t *testing.T) {
		m := newMultipartHarness(t, "uploaded", &stubDownloader{})
		if got := m.call("DELETE", "/shorts/job1/upload/multipart", "", "user1", nil).StatusCode; got != 400 {
			t.Errorf("got %d, want 400", got)
		}
	})
	t.Run("s3 fails", func(t *testing.T) {
		m := newMultipartHarness(t, "uploaded", &stubDownloader{mpAbortErr: fmt.Errorf("boom")})
		if got := m.call("DELETE", "/shorts/job1/upload/multipart", "", "user1", q).StatusCode; got != 500 {
			t.Errorf("got %d, want 500", got)
		}
	})
}

// The single PUT route and the job routes keep working next to the new ones.
func TestMultipart_ExistingShortsRoutesUnchanged(t *testing.T) {
	m := newMultipartHarness(t, "uploaded", &stubDownloader{})
	if got := m.call("GET", "/shorts/job1", "", "user1", nil).StatusCode; got != 200 {
		t.Errorf("GET /shorts/job1 = %d, want 200", got)
	}
	resp := m.call("POST", "/shorts", `{"fileType":"mp4","duration":90}`, "user1", nil)
	var created CreateShortsResponse
	_ = json.Unmarshal([]byte(resp.Body), &created)
	if resp.StatusCode != 201 || created.UploadURL == "" {
		t.Errorf("POST /shorts = %d %s, want 201 with an upload URL", resp.StatusCode, resp.Body)
	}
}

// stubAPIError is an S3 error identified by its Smithy code.
type stubAPIError struct{ code string }

func (e *stubAPIError) Error() string                 { return e.code }
func (e *stubAPIError) ErrorCode() string             { return e.code }
func (e *stubAPIError) ErrorMessage() string          { return e.code }
func (e *stubAPIError) ErrorFault() smithy.ErrorFault { return smithy.FaultUnknown }
