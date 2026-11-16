package api

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// quota is a per-user daily allowance for an action that costs real money
// (storage, Transcribe, Bedrock, Fargate) or ties up render capacity.
//
// The limits are above what a person does in a day and well below what a
// script does. They exist because sign-up is free and open: without them one
// account can run paid pipelines in a loop (SEC-02).
type quota struct {
	name  string
	limit int64
	// what is shown to the user: "You have reached today's limit for <what>."
	what string
}

var (
	quotaProjectCreate = quota{name: "project-create", limit: 40, what: "creating projects"}
	quotaShortsCreate  = quota{name: "shorts-create", limit: 30, what: "uploading videos for Shorts"}
	quotaShortsRetry   = quota{name: "shorts-retry", limit: 15, what: "retrying Shorts jobs"}
	quotaThumbnails    = quota{name: "thumbnails", limit: 30, what: "generating thumbnails"}
	quotaUploadURL     = quota{name: "upload-url", limit: 120, what: "uploading files"}
	// quotaVoicePreview counts voice previews that had to be made (a Polly call).
	// A repeat of a take already kept costs nothing and is not counted here.
	quotaVoicePreview = quota{name: "story-voice-preview", limit: 60, what: "voice previews"}
	// quotaVoicePreviewReplay bounds replays of kept takes, so S3 reads and
	// signed links cannot be used as a free loop. It is far above what a person does.
	quotaVoicePreviewReplay = quota{name: "story-voice-replay", limit: 1000, what: "playing voice previews"}
	// quotaCopyHelper counts calls to the model that writes titles, a description
	// and chapters (#331). A refine is a new call and counts too.
	quotaCopyHelper = quota{name: "copy-helper", limit: 20, what: "writing titles and descriptions"}
)

// quotaNow is replaced in tests.
var quotaNow = time.Now

// spendQuota counts one use of q for the user and reports whether it is still
// within today's allowance (the UTC day). The count is a single atomic
// DynamoDB ADD, so parallel requests cannot all pass. Counters live in
// QuotaTable (the sessions table, which has TTL on expiresAt) as
// PK=QUOTA#<user>, SK=<name>#<yyyymmdd>.
//
// When QuotaTable is empty the quotas are off. When the write fails the use is
// allowed and the error logged: a DynamoDB fault must not stop paying users.
// A use that is refused is not undone, and a use that later fails is not
// refunded; both keep the counter simple and cannot be gamed to get more.
func (h *APIHandler) spendQuota(ctx context.Context, userID string, q quota) (time.Duration, bool) {
	_, retryAfter, ok := h.spendQuotaCount(ctx, userID, q)
	return retryAfter, ok
}

// spendQuotaCount is spendQuota that also says how many uses the day has now,
// this one included, or -1 when the quotas are off or the count is unknown.
func (h *APIHandler) spendQuotaCount(ctx context.Context, userID string, q quota) (int64, time.Duration, bool) {
	if h.QuotaTable == "" {
		return -1, 0, true
	}
	now := quotaNow().UTC()
	day := now.Format("20060102")
	endOfDay := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.UTC)

	out, err := h.DB.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(h.QuotaTable),
		Key: map[string]dbtypes.AttributeValue{
			"PK": &dbtypes.AttributeValueMemberS{Value: "QUOTA#" + userID},
			"SK": &dbtypes.AttributeValueMemberS{Value: q.name + "#" + day},
		},
		UpdateExpression: aws.String("ADD uses :one SET expiresAt = :exp"),
		ExpressionAttributeValues: map[string]dbtypes.AttributeValue{
			":one": &dbtypes.AttributeValueMemberN{Value: "1"},
			":exp": &dbtypes.AttributeValueMemberN{Value: strconv.FormatInt(endOfDay.Add(48*time.Hour).Unix(), 10)},
		},
		ReturnValues: dbtypes.ReturnValueUpdatedNew,
	})
	if err != nil {
		log.Printf("quota_count_failed quota=%s err=%v", q.name, err)
		return -1, 0, true
	}
	n, ok := out.Attributes["uses"].(*dbtypes.AttributeValueMemberN)
	if !ok {
		return -1, 0, true
	}
	used, err := strconv.ParseInt(n.Value, 10, 64)
	if err != nil {
		return -1, 0, true
	}
	if used <= q.limit {
		return used, 0, true
	}
	log.Printf("quota_exceeded quota=%s user=%s used=%d limit=%d", q.name, userID, used, q.limit)
	return used, endOfDay.Sub(now), false
}

// refundQuota gives back one use of q after work that was counted failed through
// no fault of the user (the voice service was down). Best effort: a failure to
// give it back is logged and the use stays counted, which errs against the user
// by one, never in their favour.
func (h *APIHandler) refundQuota(ctx context.Context, userID string, q quota) {
	if h.QuotaTable == "" {
		return
	}
	day := quotaNow().UTC().Format("20060102")
	_, err := h.DB.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(h.QuotaTable),
		Key: map[string]dbtypes.AttributeValue{
			"PK": &dbtypes.AttributeValueMemberS{Value: "QUOTA#" + userID},
			"SK": &dbtypes.AttributeValueMemberS{Value: q.name + "#" + day},
		},
		UpdateExpression:    aws.String("ADD uses :minus"),
		ConditionExpression: aws.String("attribute_exists(PK) AND uses > :zero"),
		ExpressionAttributeValues: map[string]dbtypes.AttributeValue{
			":minus": &dbtypes.AttributeValueMemberN{Value: "-1"},
			":zero":  &dbtypes.AttributeValueMemberN{Value: "0"},
		},
	})
	if err != nil {
		log.Printf("quota_refund_failed quota=%s err=%v", q.name, err)
	}
}

// quotaExceeded is the 429 for a refused use.
func quotaExceeded(q quota, retryAfter time.Duration) events.APIGatewayProxyResponse {
	seconds := int(retryAfter.Seconds())
	if seconds < 1 {
		seconds = 1
	}
	resp := tooManyRequests("QUOTA_EXCEEDED", "You have reached today's limit for "+q.what+". It resets at midnight UTC.", seconds)
	resp.StatusCode = http.StatusTooManyRequests
	return resp
}

// checkQuota is spendQuota for handlers: it returns the 429 to send when the
// user is over, or nil to carry on.
func (h *APIHandler) checkQuota(ctx context.Context, userID string, q quota) *events.APIGatewayProxyResponse {
	if retryAfter, ok := h.spendQuota(ctx, userID, q); !ok {
		resp := quotaExceeded(q, retryAfter)
		return &resp
	}
	return nil
}
