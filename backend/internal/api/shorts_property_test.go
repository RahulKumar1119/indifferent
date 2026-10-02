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
	"pgregory.net/rapid"
)

// newShortsPropertyHandler builds a handler whose DynamoDB mock only returns a
// job when the GetItem PK matches the owner's USER# partition, mirroring how
// DynamoDB scopes reads by partition key. The stub downloader reports every
// object as standard storage so presigning stays offline and deterministic.
func newShortsPropertyHandler(s3store storage.Downloader, ownerID, jobID, clipID, clipKey string) *APIHandler {
	ownerPK := "USER#" + ownerID
	jobSK := "SHORTS#" + jobID

	clips := []models.Clip{{
		ClipID: clipID,
		S3Key:  clipKey,
		Rank:   1,
		Score:  0.9,
	}}
	clipsJSON, _ := json.Marshal(clips)

	db := &mockDynamoDB{
		getItemFunc: func(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
			pk := params.Key["PK"].(*dbtypes.AttributeValueMemberS).Value
			sk := params.Key["SK"].(*dbtypes.AttributeValueMemberS).Value
			if pk == ownerPK && sk == jobSK {
				return &dynamodb.GetItemOutput{
					Item: map[string]dbtypes.AttributeValue{
						"PK":             &dbtypes.AttributeValueMemberS{Value: ownerPK},
						"SK":             &dbtypes.AttributeValueMemberS{Value: jobSK},
						"jobId":          &dbtypes.AttributeValueMemberS{Value: jobID},
						"status":         &dbtypes.AttributeValueMemberS{Value: "completed"},
						"fileType":       &dbtypes.AttributeValueMemberS{Value: "mp4"},
						"sourceDuration": &dbtypes.AttributeValueMemberN{Value: "120"},
						"sourceKey":      &dbtypes.AttributeValueMemberS{Value: "uploads/" + ownerID + "/" + jobID + "/source.mp4"},
						"clips":          &dbtypes.AttributeValueMemberS{Value: string(clipsJSON)},
						"createdAt":      &dbtypes.AttributeValueMemberS{Value: "2024-01-01T00:00:00Z"},
						"updatedAt":      &dbtypes.AttributeValueMemberS{Value: "2024-01-01T00:00:00Z"},
					},
				}, nil
			}
			// Not in this user's partition.
			return &dynamodb.GetItemOutput{Item: nil}, nil
		},
	}

	return &APIHandler{
		AuthService:           &mockAuthService{},
		JWTService:            &auth.JWTService{Secret: testSecret, DB: db, SessionTable: "sessions-table"},
		DB:                    db,
		S3:                    s3store,
		SFN:                   &mockSFN{},
		TableName:             "table",
		Bucket:                "test-bucket",
		StateMachineARN:       "arn:aws:states:us-east-1:123456789:stateMachine:test",
		ShortsStateMachineARN: "arn:aws:states:us-east-1:123456789:stateMachine:shorts",
	}
}

// genIdentifier produces short, non-empty, path-safe identifiers.
func genIdentifier(t *rapid.T, label string) string {
	return rapid.StringMatching(`[a-z0-9]{1,12}`).Draw(t, label)
}

// Feature: ai-shorts-generator, Property 8
// Owner-scoped access: a non-owner request is rejected (not found), and every
// presigned URL references a key under the requesting user's prefix.
// Validates: Requirements 8.1, 8.2, 8.3, 8.4
func TestProperty8_OwnerScopedAccess(t *testing.T) {
	s3store := &stubDownloader{status: storage.ObjectStatus{StorageClass: "STANDARD"}}
	rapid.Check(t, func(t *rapid.T) {
		owner := genIdentifier(t, "owner")
		other := genIdentifier(t, "other")
		if other == owner {
			other = owner + "x"
		}
		jobID := genIdentifier(t, "job")
		clipID := genIdentifier(t, "clip")
		clipKey := "shorts/" + owner + "/" + jobID + "/clips/" + clipID + ".mp4"

		h := newShortsPropertyHandler(s3store, owner, jobID, clipID, clipKey)
		ctx := context.Background()

		ownerToken := generateTestToken(owner)
		otherToken := generateTestToken(other)

		// 1. Non-owner GET /shorts/{id} is not found.
		resp, _ := h.HandleRequest(ctx, events.APIGatewayProxyRequest{
			HTTPMethod: "GET",
			Path:       "/shorts/" + jobID,
			Headers:    map[string]string{"Authorization": "Bearer " + otherToken},
		})
		if resp.StatusCode != 404 {
			t.Fatalf("non-owner GET job: got %d, want 404", resp.StatusCode)
		}

		// 2. Non-owner clip URL request is not found.
		resp, _ = h.HandleRequest(ctx, events.APIGatewayProxyRequest{
			HTTPMethod: "GET",
			Path:       "/shorts/" + jobID + "/clips/" + clipID + "/url",
			Headers:    map[string]string{"Authorization": "Bearer " + otherToken},
		})
		if resp.StatusCode != 404 {
			t.Fatalf("non-owner clip URL: got %d, want 404", resp.StatusCode)
		}

		// 3. Owner clip URL request succeeds and references a key under the
		//    owner's prefix.
		resp, _ = h.HandleRequest(ctx, events.APIGatewayProxyRequest{
			HTTPMethod: "GET",
			Path:       "/shorts/" + jobID + "/clips/" + clipID + "/url",
			Headers:    map[string]string{"Authorization": "Bearer " + ownerToken},
		})
		if resp.StatusCode != 200 {
			t.Fatalf("owner clip URL: got %d, want 200 (body: %s)", resp.StatusCode, resp.Body)
		}
		var body struct {
			URL string `json:"url"`
		}
		if err := json.Unmarshal([]byte(resp.Body), &body); err != nil {
			t.Fatalf("failed to unmarshal clip URL response: %v", err)
		}
		ownerPrefix := "shorts/" + owner + "/"
		if !strings.Contains(body.URL, ownerPrefix) {
			t.Fatalf("presigned URL %q does not reference owner prefix %q", body.URL, ownerPrefix)
		}

		// 4. Owner create-shorts presigned upload URL is under the owner prefix.
		resp, _ = h.HandleRequest(ctx, events.APIGatewayProxyRequest{
			HTTPMethod: "POST",
			Path:       "/shorts",
			Headers:    map[string]string{"Authorization": "Bearer " + ownerToken},
			Body:       `{"fileType":"mp4","duration":60}`,
		})
		if resp.StatusCode != 201 {
			t.Fatalf("owner create shorts: got %d, want 201 (body: %s)", resp.StatusCode, resp.Body)
		}
		var created CreateShortsResponse
		if err := json.Unmarshal([]byte(resp.Body), &created); err != nil {
			t.Fatalf("failed to unmarshal create response: %v", err)
		}
		uploadPrefix := "uploads/" + owner + "/"
		if !strings.HasPrefix(created.SourceKey, uploadPrefix) {
			t.Fatalf("source key %q not under owner prefix %q", created.SourceKey, uploadPrefix)
		}
		if !strings.Contains(created.UploadURL, uploadPrefix) {
			t.Fatalf("upload URL %q does not reference owner prefix %q", created.UploadURL, uploadPrefix)
		}
	})
}
