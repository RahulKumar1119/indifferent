// Package pipeline provides the Step Functions pipeline orchestration utilities
// including status updates and progress tracking.
package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
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
		updateExpr += ", #error = :error"
		exprNames["#error"] = "error"
		exprValues[":error"] = &types.AttributeValueMemberS{Value: extractErrorMessage(input.Error)}
	}

	_, err := u.DB.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:                 aws.String(u.TableName),
		Key:                       key,
		UpdateExpression:          aws.String(updateExpr),
		ExpressionAttributeNames:  exprNames,
		ExpressionAttributeValues: exprValues,
	})
	if err != nil {
		return fmt.Errorf("failed to update shorts status: %w", err)
	}

	return nil
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
		})
	}
	return clips
}

// GenerateVideoURL generates a presigned download URL for the video with 24-hour expiration.
func (u *StatusUpdater) GenerateVideoURL(ctx context.Context, videoKey string) (string, error) {
	return u.S3Storage.GenerateDownloadURL(ctx, u.Bucket, videoKey, storage.DownloadURLExpiration)
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
