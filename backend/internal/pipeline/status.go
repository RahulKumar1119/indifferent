// Package pipeline provides the Step Functions pipeline orchestration utilities
// including status updates and progress tracking.
package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/rahul/indifferent/backend/internal/models"
	"github.com/rahul/indifferent/backend/internal/shorts/render"
	"github.com/rahul/indifferent/backend/internal/storage"
)

// StatusInput represents the event received by the status updater Lambda.
type StatusInput struct {
	ProjectID    string `json:"projectId"`
	UserID       string `json:"userId"`
	Status       string `json:"status"` // "parsing", "generating_slides", "narrating", "rendering", "completed", "failed"
	VideoKey     string `json:"videoKey,omitempty"`
	ThumbnailKey string `json:"thumbnailKey,omitempty"`
	Error        any    `json:"error,omitempty"` // error details from Step Functions catch
}

// DynamoDBClient defines the interface for DynamoDB operations needed by the status updater.
type DynamoDBClient interface {
	UpdateItem(ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error)
	GetItem(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error)
}

// StatusUpdater handles updating project status in DynamoDB after each pipeline stage.
type StatusUpdater struct {
	DB        DynamoDBClient
	S3Storage *storage.S3Client
	TableName string
	Bucket    string
}

// UpdateStatus updates the project status in DynamoDB based on the pipeline stage result.
// For all statuses: updates status and updatedAt.
// For "completed": also sets videoKey, thumbnailKey, and completedAt.
// For "failed": also sets the error field with the failure reason.
func (u *StatusUpdater) UpdateStatus(ctx context.Context, input StatusInput) error {
	now := time.Now().UTC().Format(time.RFC3339)

	key := map[string]types.AttributeValue{
		"PK": &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s", input.UserID)},
		"SK": &types.AttributeValueMemberS{Value: fmt.Sprintf("PROJECT#%s", input.ProjectID)},
	}

	// Base expression and values for all status updates
	updateExpr := "SET #status = :status, #updatedAt = :updatedAt"
	exprNames := map[string]string{
		"#status":    "status",
		"#updatedAt": "updatedAt",
	}
	exprValues := map[string]types.AttributeValue{
		":status":    &types.AttributeValueMemberS{Value: input.Status},
		":updatedAt": &types.AttributeValueMemberS{Value: now},
	}

	switch input.Status {
	case "completed":
		updateExpr += ", #videoKey = :videoKey, #thumbnailKey = :thumbnailKey, #completedAt = :completedAt"
		exprNames["#videoKey"] = "videoKey"
		exprNames["#thumbnailKey"] = "thumbnailKey"
		exprNames["#completedAt"] = "completedAt"
		exprValues[":videoKey"] = &types.AttributeValueMemberS{Value: input.VideoKey}
		exprValues[":thumbnailKey"] = &types.AttributeValueMemberS{Value: input.ThumbnailKey}
		exprValues[":completedAt"] = &types.AttributeValueMemberS{Value: now}

	case "failed":
		updateExpr += ", #error = :error"
		exprNames["#error"] = "error"
		errorMsg := extractErrorMessage(input.Error)
		exprValues[":error"] = &types.AttributeValueMemberS{Value: errorMsg}
	}

	_, err := u.DB.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:                 aws.String(u.TableName),
		Key:                       key,
		UpdateExpression:          aws.String(updateExpr),
		ExpressionAttributeNames:  exprNames,
		ExpressionAttributeValues: exprValues,
		// Never resurrect deleted projects as ghost records.
		ConditionExpression: aws.String("attribute_exists(PK)"),
	})
	if err != nil {
		return fmt.Errorf("failed to update project status: %w", err)
	}

	return nil
}

// Shorts pipeline status values (Requirement 6.2).
const (
	ShortsStatusUploaded     = "uploaded"
	ShortsStatusTranscribing = "transcribing"
	ShortsStatusRanking      = "ranking"
	ShortsStatusRendering    = "rendering"
	ShortsStatusCompleted    = "completed"
	ShortsStatusFailed       = "failed"
)

// ShortsStatusInput represents a status update for a shorts job. Shorts jobs
// live under the same table with SK = "SHORTS#{JobID}".
type ShortsStatusInput struct {
	JobID  string `json:"jobId"`
	UserID string `json:"userId"`
	Status string `json:"status"` // uploaded|transcribing|ranking|rendering|completed|failed
	// Clips arrives from the state machine as ranked segments (the Rank
	// lambda's output, including clipId). On completion they are converted
	// to Clip records with deterministic S3 keys before storage.
	Clips []models.RankedSegment `json:"clips,omitempty"`
	Error any                    `json:"error,omitempty"`
}

// UpdateShortsStatus updates a shorts job's status in DynamoDB. It mirrors
// UpdateStatus but keys the SHORTS# record. On "completed" it stores the clip
// list (marshaled to JSON) and completedAt; on "failed" it stores the error.
// (Requirements 6.1, 6.2, 6.4)
func (u *StatusUpdater) UpdateShortsStatus(ctx context.Context, input ShortsStatusInput) error {
	now := time.Now().UTC().Format(time.RFC3339)

	key := map[string]types.AttributeValue{
		"PK": &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s", input.UserID)},
		"SK": &types.AttributeValueMemberS{Value: fmt.Sprintf("SHORTS#%s", input.JobID)},
	}

	updateExpr := "SET #status = :status, #updatedAt = :updatedAt, #progress = :progress"
	exprNames := map[string]string{
		"#status":    "status",
		"#updatedAt": "updatedAt",
		"#progress":  "progress",
	}
	exprValues := map[string]types.AttributeValue{
		":status":    &types.AttributeValueMemberS{Value: input.Status},
		":updatedAt": &types.AttributeValueMemberS{Value: now},
		":progress":  &types.AttributeValueMemberN{Value: strconv.Itoa(stageBaseProgress(input.Status))},
	}

	switch input.Status {
	case ShortsStatusCompleted:
		clipsJSON, err := json.Marshal(segmentsToClips(input.UserID, input.JobID, input.Clips))
		if err != nil {
			return fmt.Errorf("failed to marshal clips: %w", err)
		}
		updateExpr += ", #clips = :clips, #completedAt = :completedAt"
		exprNames["#clips"] = "clips"
		exprNames["#completedAt"] = "completedAt"
		exprValues[":clips"] = &types.AttributeValueMemberS{Value: string(clipsJSON)}
		exprValues[":completedAt"] = &types.AttributeValueMemberS{Value: now}

	case ShortsStatusFailed:
		errMsg := extractErrorMessage(input.Error)
		// A render with video effects records its own reason. That reason
		// says what failed, which the state machine's generic cause does not.
		if own := u.renderOwnError(ctx, input); own != "" {
			errMsg = own
		}
		updateExpr += ", #error = :error"
		exprNames["#error"] = "error"
		exprValues[":error"] = &types.AttributeValueMemberS{Value: errMsg}
		// Record where it broke and what kind of failure it was, so retries
		// and the UI can distinguish "try again" from "fix the input".
		// The previous stage is read from the record itself (no ASL change).
		if stage, code := classifyShortsFailure(ctx, u, input, errMsg); stage != "" {
			updateExpr += ", #failedStage = :failedStage, #errorCode = :errorCode"
			exprNames["#failedStage"] = "failedStage"
			exprNames["#errorCode"] = "errorCode"
			exprValues[":failedStage"] = &types.AttributeValueMemberS{Value: stage}
			exprValues[":errorCode"] = &types.AttributeValueMemberS{Value: code}
		}
	}

	_, err := u.DB.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:                 aws.String(u.TableName),
		Key:                       key,
		UpdateExpression:          aws.String(updateExpr),
		ExpressionAttributeNames:  exprNames,
		ExpressionAttributeValues: exprValues,
		// Never resurrect deleted jobs as ghost records.
		ConditionExpression: aws.String("attribute_exists(PK)"),
	})
	if err != nil {
		return fmt.Errorf("failed to update shorts status: %w", err)
	}

	return nil
}

// stageBaseProgress is the determinate progress baseline each pipeline stage
// starts at. The render stage refines 78→99 live from ffmpeg output; the rest
// advance on transitions so no stage ever shows an indeterminate state.
func stageBaseProgress(status string) int {
	switch status {
	case ShortsStatusCompleted:
		return 100
	case ShortsStatusFailed:
		return 0
	case ShortsStatusRendering:
		return 78
	case ShortsStatusRanking:
		return 55
	case ShortsStatusTranscribing:
		return 25
	default: // uploaded and anything unrecognized
		return 5
	}
}

// segmentsToClips converts ranked segments into gallery-ready Clip records.
// S3Key uses the same deterministic pattern the renderer writes to, so the
// clips gallery can presign download URLs without the render tasks having to
// report back. Duration is derived from the segment window.
func segmentsToClips(userID, jobID string, segments []models.RankedSegment) []models.Clip {
	clips := make([]models.Clip, 0, len(segments))
	for _, s := range segments {
		if s.ClipID == "" {
			continue
		}
		clips = append(clips, models.Clip{
			ClipID:   s.ClipID,
			S3Key:    render.ClipKey(userID, jobID, s.ClipID),
			Rank:     s.Rank,
			Score:    s.Score,
			Duration: s.End - s.Start,
			HookText: s.HookText,
			Start:    s.Start,
			End:      s.End,
		})
	}
	return clips
}

// GenerateVideoURL generates a presigned download URL for the video with 24-hour expiration.
func (u *StatusUpdater) GenerateVideoURL(ctx context.Context, videoKey string) (string, error) {
	return u.S3Storage.GenerateDownloadURL(ctx, u.Bucket, videoKey, storage.DownloadURLExpiration)
}

// renderOwnError returns the reason a render recorded on the job, or "" when
// none was recorded. Only renders with video effects record one, and a retry
// rewrites the whole record, which clears it.
func (u *StatusUpdater) renderOwnError(ctx context.Context, input ShortsStatusInput) string {
	out, err := u.DB.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(u.TableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s", input.UserID)},
			"SK": &types.AttributeValueMemberS{Value: fmt.Sprintf("SHORTS#%s", input.JobID)},
		},
		ConsistentRead: aws.Bool(true),
	})
	if err != nil || out == nil {
		return ""
	}
	if v, ok := out.Item["renderError"].(*types.AttributeValueMemberS); ok {
		return v.Value
	}
	return ""
}

// classifyShortsFailure determines the failed stage (from the record's
// current status) and a machine-readable error code (from the message) for a
// failed shorts job. A previously recorded stage is preserved on re-marks.
func classifyShortsFailure(ctx context.Context, u *StatusUpdater, input ShortsStatusInput, errMsg string) (stage, code string) {
	if prev, perr := u.DB.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(u.TableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s", input.UserID)},
			"SK": &types.AttributeValueMemberS{Value: fmt.Sprintf("SHORTS#%s", input.JobID)},
		},
		ConsistentRead: aws.Bool(true),
	}); perr == nil && prev.Item != nil {
		if s, ok := prev.Item["status"].(*types.AttributeValueMemberS); ok {
			switch s.Value {
			case "transcribing", "ranking", "rendering", "uploaded":
				stage = s.Value
			case "failed", "delete_failed":
				if fs, ok := prev.Item["failedStage"].(*types.AttributeValueMemberS); ok && fs.Value != "" {
					stage = fs.Value
				}
			}
		}
		if stage == "" {
			if ec, ok := prev.Item["errorCode"].(*types.AttributeValueMemberS); ok {
				code = ec.Value
			}
		}
	}
	if code == "" {
		code = classifyShortsErrorCode(errMsg)
	}
	return stage, code
}

// classifyShortsErrorCode maps a failure message to a stable error code.
// Validation codes mean retrying the same input is futile (fix the file);
// anything else is safe to retry.
func classifyShortsErrorCode(msg string) string {
	lower := strings.ToLower(msg)
	switch {
	case strings.Contains(lower, "cancelled by user"):
		return "CANCELLED"
	case strings.Contains(lower, "no speech detected"):
		return "VALIDATION_NO_SPEECH"
	case strings.Contains(lower, "over the") && strings.Contains(lower, "limit"):
		return "VALIDATION_DURATION"
	case strings.Contains(lower, "upload not found") ||
		strings.Contains(lower, "empty") && strings.Contains(lower, "upload") ||
		strings.Contains(lower, "too large") ||
		strings.Contains(lower, "looks like"):
		return "VALIDATION_UPLOAD"
	case strings.Contains(lower, "bedrock") || strings.Contains(lower, "invokemodel") || strings.Contains(lower, "inference profile"):
		return "RANKING_BACKEND"
	case strings.Contains(lower, "conflictexception") || strings.Contains(lower, "already exists"):
		return "TRANSIENT_CONFLICT"
	default:
		return "INFRASTRUCTURE"
	}
}

// extractErrorMessage extracts a human-readable error message from the Step Functions error object.
// The error field can be a string, a map with "Error" and "Cause" keys, or any other type.
func extractErrorMessage(err any) string {
	if err == nil {
		return "unknown error"
	}

	switch v := err.(type) {
	case string:
		return v
	case map[string]any:
		// Step Functions error format: {"Error": "...", "Cause": "..."}
		if cause, ok := v["Cause"]; ok {
			if causeStr, ok := cause.(string); ok {
				return causeStr
			}
		}
		if errMsg, ok := v["Error"]; ok {
			if errStr, ok := errMsg.(string); ok {
				return errStr
			}
		}
		return fmt.Sprintf("%v", v)
	default:
		return fmt.Sprintf("%v", v)
	}
}
