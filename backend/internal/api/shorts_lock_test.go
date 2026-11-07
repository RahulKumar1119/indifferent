package api

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// The edit handlers read a shorts job, change one field and write the whole
// record back. These tests pin that the write is conditional on the job not
// having changed since it was read, so a render worker's progress is not
// overwritten with a stale copy.

type lockCase struct {
	name    string
	status  string
	request events.APIGatewayProxyRequest
}

func lockCases() []lockCase {
	auth := map[string]string{"Authorization": "Bearer " + generateTestToken("user1")}
	return []lockCase{
		{"caption style", "uploaded", events.APIGatewayProxyRequest{HTTPMethod: "PUT", Path: "/shorts/job1/style", Headers: auth, Body: `{"caption":{"preset":"bold"}}`}},
		{"video effects", "uploaded", events.APIGatewayProxyRequest{HTTPMethod: "PUT", Path: "/shorts/job1/effects", Headers: auth, Body: `{"effects":null}`}},
		{"clip edit", "completed", clipEditReq("job1", "clip-9", `{"start":12,"end":38}`)},
		{"clip edit reset", "completed", clipEditReq("job1", "clip-9", `{"reset":true}`)},
	}
}

func lockHandler(t *testing.T, status string, updatedAt *string, putErr error, seen *[]*dynamodb.PutItemInput) *APIHandler {
	t.Helper()
	db := &mockDynamoDB{
		getItemFunc: func(_ context.Context, _ *dynamodb.GetItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
			item := editedJobItem("job1", status)
			if updatedAt == nil || *updatedAt == "" {
				delete(item, "updatedAt")
			} else {
				item["updatedAt"] = &dbtypes.AttributeValueMemberS{Value: *updatedAt}
			}
			return &dynamodb.GetItemOutput{Item: item}, nil
		},
		putItemFunc: func(_ context.Context, p *dynamodb.PutItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
			if seen != nil {
				*seen = append(*seen, p)
			}
			return &dynamodb.PutItemOutput{}, putErr
		},
	}
	return newShortsTestHandler(t, db)
}

func TestShortsEdits_WriteOnlyIfTheJobIsUnchanged(t *testing.T) {
	for _, tc := range lockCases() {
		t.Run(tc.name, func(t *testing.T) {
			read := "2026-11-07T10:00:00Z"
			var puts []*dynamodb.PutItemInput
			h := lockHandler(t, tc.status, &read, nil, &puts)
			resp, _ := h.HandleRequest(context.Background(), tc.request)
			if resp.StatusCode != 200 {
				t.Fatalf("status %d: %s", resp.StatusCode, resp.Body)
			}
			if len(puts) != 1 {
				t.Fatalf("%d writes, want 1", len(puts))
			}
			p := puts[0]
			if aws.ToString(p.ConditionExpression) != "updatedAt = :read" {
				t.Errorf("condition = %q, want updatedAt = :read", aws.ToString(p.ConditionExpression))
			}
			got, _ := p.ExpressionAttributeValues[":read"].(*dbtypes.AttributeValueMemberS)
			if got == nil || got.Value != read {
				t.Errorf(":read = %v, want the value that was read (%s)", p.ExpressionAttributeValues[":read"], read)
			}
		})
	}
}

func TestShortsEdits_AJobWithNoUpdatedAtIsWrittenOnlyWhileItStillHasNone(t *testing.T) {
	for _, tc := range lockCases() {
		t.Run(tc.name, func(t *testing.T) {
			var puts []*dynamodb.PutItemInput
			h := lockHandler(t, tc.status, nil, nil, &puts)
			if resp, _ := h.HandleRequest(context.Background(), tc.request); resp.StatusCode != 200 {
				t.Fatalf("status %d: %s", resp.StatusCode, resp.Body)
			}
			if got := aws.ToString(puts[0].ConditionExpression); got != "attribute_not_exists(updatedAt)" {
				t.Errorf("condition = %q, want attribute_not_exists(updatedAt)", got)
			}
		})
	}
}

func TestShortsEdits_ReportAConflictInsteadOfOverwriting(t *testing.T) {
	for _, tc := range lockCases() {
		t.Run(tc.name, func(t *testing.T) {
			read := "2026-11-07T10:00:00Z"
			h := lockHandler(t, tc.status, &read, &dbtypes.ConditionalCheckFailedException{Message: aws.String("condition failed")}, nil)
			resp, _ := h.HandleRequest(context.Background(), tc.request)
			if resp.StatusCode != 409 || quotaErrCode(t, resp.Body) != "CONFLICT" {
				t.Fatalf("%d %s, want 409 CONFLICT", resp.StatusCode, resp.Body)
			}
			if !strings.Contains(resp.Body, "Reload") {
				t.Errorf("the message does not say what to do: %s", resp.Body)
			}
		})
	}
}

func TestShortsEdits_OtherWriteFailuresStayServerErrors(t *testing.T) {
	for _, tc := range lockCases() {
		t.Run(tc.name, func(t *testing.T) {
			read := "2026-11-07T10:00:00Z"
			h := lockHandler(t, tc.status, &read, errors.New("dynamodb is down"), nil)
			if resp, _ := h.HandleRequest(context.Background(), tc.request); resp.StatusCode != 500 {
				t.Fatalf("%d %s, want 500", resp.StatusCode, resp.Body)
			}
		})
	}
}
