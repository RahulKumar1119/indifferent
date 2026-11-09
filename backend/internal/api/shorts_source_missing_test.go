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

	"github.com/rahul/indifferent/backend/internal/storage"
)

// metaByKey answers ObjectMeta per key and counts the calls (issue #275).
type metaByKey struct {
	*stubDownloader
	found map[string]bool
	err   map[string]error
	calls []string
}

func (m *metaByKey) ObjectMeta(ctx context.Context, bucket, key string) (storage.ObjectMeta, error) {
	m.calls = append(m.calls, key)
	if e := m.err[key]; e != nil {
		return storage.ObjectMeta{}, e
	}
	if m.found[key] {
		return storage.ObjectMeta{Found: true, Size: 10}, nil
	}
	return storage.ObjectMeta{}, nil
}

func shortsItem(id, status string) map[string]dbtypes.AttributeValue {
	return map[string]dbtypes.AttributeValue{
		"PK": &dbtypes.AttributeValueMemberS{Value: "USER#user1"}, "SK": &dbtypes.AttributeValueMemberS{Value: "SHORTS#" + id},
		"jobId": &dbtypes.AttributeValueMemberS{Value: id}, "status": &dbtypes.AttributeValueMemberS{Value: status},
		"fileType": &dbtypes.AttributeValueMemberS{Value: "mp4"}, "sourceDuration": &dbtypes.AttributeValueMemberN{Value: "36"},
		"sourceKey": &dbtypes.AttributeValueMemberS{Value: "uploads/user1/" + id + "/source.mp4"},
		"createdAt": &dbtypes.AttributeValueMemberS{Value: "2026-11-08T17:26:51Z"}, "updatedAt": &dbtypes.AttributeValueMemberS{Value: "2026-11-08T17:26:51Z"},
	}
}

func listShorts(t *testing.T, items []map[string]dbtypes.AttributeValue, s3 *metaByKey) (map[string]bool, string) {
	t.Helper()
	db := &mockDynamoDB{queryFunc: func(ctx context.Context, params *dynamodb.QueryInput, optFns ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error) {
		return &dynamodb.QueryOutput{Items: items}, nil
	}}
	h := newShortsTestHandler(t, db)
	h.S3 = s3
	resp, _ := h.HandleRequest(context.Background(), events.APIGatewayProxyRequest{
		HTTPMethod: "GET", Path: "/shorts",
		Headers: map[string]string{"Authorization": "Bearer " + generateTestToken("user1")},
	})
	if resp.StatusCode != 200 {
		t.Fatalf("got %d: %s", resp.StatusCode, resp.Body)
	}
	var body struct {
		Jobs []struct {
			JobID         string `json:"jobId"`
			SourceMissing bool   `json:"sourceMissing"`
		} `json:"jobs"`
	}
	if err := json.Unmarshal([]byte(resp.Body), &body); err != nil {
		t.Fatal(err)
	}
	out := map[string]bool{}
	for _, j := range body.Jobs {
		out[j.JobID] = j.SourceMissing
	}
	return out, resp.Body
}

func TestListShorts_FlagsUploadedJobWithoutFile(t *testing.T) {
	s3 := &metaByKey{stubDownloader: &stubDownloader{}, found: map[string]bool{"uploads/user1/landed/source.mp4": true}}
	got, body := listShorts(t, []map[string]dbtypes.AttributeValue{
		shortsItem("lost", "uploaded"),
		shortsItem("landed", "uploaded"),
		shortsItem("done", "completed"),
		shortsItem("running", "rendering"),
	}, s3)
	if !got["lost"] {
		t.Errorf("an uploaded job with no file must be flagged: %s", body)
	}
	if got["landed"] || got["done"] || got["running"] {
		t.Errorf("only the job without a file may be flagged: %v", got)
	}
	// Jobs past "uploaded" already have their file (the pipeline read it): no S3 call for them.
	if len(s3.calls) != 2 {
		t.Errorf("want HeadObject only for the 2 uploaded jobs, got %v", s3.calls)
	}
	if strings.Count(body, "sourceMissing") != 1 {
		t.Errorf("the flag should be omitted when false: %s", body)
	}
}

func TestListShorts_S3ErrorIsNotReportedAsMissing(t *testing.T) {
	s3 := &metaByKey{stubDownloader: &stubDownloader{}, err: map[string]error{"uploads/user1/flaky/source.mp4": errors.New("throttled")}}
	got, _ := listShorts(t, []map[string]dbtypes.AttributeValue{shortsItem("flaky", "uploaded")}, s3)
	if got["flaky"] {
		t.Error("an S3 error must not be shown to the user as a missing upload")
	}
}

func TestListShorts_CapsSourceChecks(t *testing.T) {
	items := make([]map[string]dbtypes.AttributeValue, 0, maxSourceChecks+5)
	for i := 0; i < maxSourceChecks+5; i++ {
		items = append(items, shortsItem(fmt.Sprintf("j%02d", i), "uploaded"))
	}
	s3 := &metaByKey{stubDownloader: &stubDownloader{}}
	listShorts(t, items, s3)
	if len(s3.calls) != maxSourceChecks {
		t.Errorf("want at most %d HeadObject calls per list, got %d", maxSourceChecks, len(s3.calls))
	}
}

func TestUploadNotFoundMessageHasNoEmDash(t *testing.T) {
	h := newShortsTestHandler(t, &mockDynamoDB{})
	h.S3 = &metaByKey{stubDownloader: &stubDownloader{}}
	job := itemToShortsJob(shortsItem("lost", "uploaded"))
	err := h.verifyShortsUpload(context.Background(), &job)
	var problem *uploadProblem
	if !errors.As(err, &problem) || problem.code != "UPLOAD_NOT_FOUND" {
		t.Fatalf("want UPLOAD_NOT_FOUND, got %v", err)
	}
	if strings.Contains(problem.message, "\u2014") {
		t.Errorf("message must not contain an em dash: %q", problem.message)
	}
}
