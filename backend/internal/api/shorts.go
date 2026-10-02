package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/sfn"
	"github.com/google/uuid"
	"github.com/rahul/indifferent/backend/internal/models"
	"github.com/rahul/indifferent/backend/internal/shorts"
	"github.com/rahul/indifferent/backend/internal/storage"
)

// CreateShortsRequest is the POST /shorts body.
type CreateShortsRequest struct {
	FileType  string  `json:"fileType"`            // "mp4" | "mov" | "mp3" | "wav"
	Duration  float64 `json:"duration"`            // client-probed source duration in seconds
	ProjectID string  `json:"projectId,omitempty"` // optional unified project to link
}

// CreateShortsResponse returns the created job and a presigned upload URL.
type CreateShortsResponse struct {
	JobID     string `json:"jobId"`
	UploadURL string `json:"uploadUrl"`
	SourceKey string `json:"sourceKey"`
}

// validShortsFileTypes maps a supported source extension to its content type
// (Requirements 1.3, 9.3).
var validShortsFileTypes = map[string]string{
	"mp4": "video/mp4",
	"mov": "video/quicktime",
	"mp3": "audio/mpeg",
	"wav": "audio/wav",
}

// validateCreateShorts enforces file-type and duration limits before any job
// is created (Requirements 1.3, 1.4, 9.1, 9.3).
func validateCreateShorts(req *CreateShortsRequest) error {
	ft := strings.ToLower(strings.TrimSpace(req.FileType))
	if _, ok := validShortsFileTypes[ft]; !ok {
		return fmt.Errorf("unsupported file type %q: must be one of mp4, mov, mp3, wav", req.FileType)
	}
	if req.Duration <= 0 {
		return fmt.Errorf("duration must be greater than zero")
	}
	if req.Duration > shorts.MaxSourceDuration {
		return fmt.Errorf("source duration %.0fs exceeds maximum of %.0fs", req.Duration, shorts.MaxSourceDuration)
	}
	return nil
}

// handleCreateShorts creates a new shorts job with status "uploaded" and
// returns a presigned PUT URL scoped to the requesting user's prefix.
func (h *APIHandler) handleCreateShorts(ctx context.Context, req events.APIGatewayProxyRequest, claims *models.JWTClaims) (events.APIGatewayProxyResponse, error) {
	var body CreateShortsRequest
	if err := json.Unmarshal([]byte(req.Body), &body); err != nil {
		return errorResponse(http.StatusBadRequest, "INVALID_BODY", "Invalid request body"), nil
	}

	if err := validateCreateShorts(&body); err != nil {
		return errorResponse(http.StatusBadRequest, "VALIDATION_ERROR", err.Error()), nil
	}

	// Optional link into a unified project (ownership verified inside).
	projectID := strings.TrimSpace(body.ProjectID)
	if projectID != "" {
		if _, err := h.getProjectByID(ctx, claims.UserID, projectID); err != nil {
			return errorResponse(http.StatusNotFound, "NOT_FOUND", "Project not found"), nil
		}
	}

	job, uploadURL, err := h.createShortsJob(ctx, claims.UserID, body.FileType, body.Duration, projectID)
	if err != nil {
		return errorResponse(http.StatusInternalServerError, "DB_ERROR", err.Error()), nil
	}

	return jsonResponse(http.StatusCreated, CreateShortsResponse{
		JobID:     job.JobID,
		UploadURL: uploadURL,
		SourceKey: job.SourceKey,
	}), nil
}

// createShortsJob validates, stores and presigns a new shorts job, linking it
// into the given unified project when projectID is set.
func (h *APIHandler) createShortsJob(ctx context.Context, userID, fileType string, duration float64, projectID string) (models.ShortsJob, string, error) {
	ext := strings.ToLower(strings.TrimSpace(fileType))
	contentType := validShortsFileTypes[ext]

	jobID := uuid.New().String()
	now := time.Now().UTC().Format(time.RFC3339)

	job := models.ShortsJob{
		UserID:         userID,
		JobID:          jobID,
		ProjectID:      projectID,
		Status:         "uploaded",
		FileType:       ext,
		SourceDuration: duration,
		SourceKey:      fmt.Sprintf("uploads/%s/%s/source.%s", userID, jobID, ext),
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if _, err := h.DB.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(h.TableName),
		Item:      shortsJobToItem(job),
	}); err != nil {
		return models.ShortsJob{}, "", fmt.Errorf("failed to create shorts job")
	}

	uploadURL, err := h.S3.GenerateUploadURL(ctx, h.Bucket, job.SourceKey, contentType, storage.UploadURLExpiration)
	if err != nil {
		return models.ShortsJob{}, "", fmt.Errorf("failed to generate upload URL")
	}

	if projectID != "" {
		if err := h.linkShortsJob(ctx, userID, projectID, jobID); err != nil {
			return models.ShortsJob{}, "", err
		}
	}

	return job, uploadURL, nil
}

// linkShortsJob appends a job ID to its unified project (idempotent).
func (h *APIHandler) linkShortsJob(ctx context.Context, userID, projectID, jobID string) error {
	project, err := h.getProjectByID(ctx, userID, projectID)
	if err != nil {
		return fmt.Errorf("project not found")
	}
	for _, id := range project.ShortsJobIDs {
		if id == jobID {
			return nil
		}
	}
	project.ShortsJobIDs = append(project.ShortsJobIDs, jobID)
	project.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if _, err := h.DB.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(h.TableName),
		Item:      projectToItem(*project),
	}); err != nil {
		return fmt.Errorf("failed to link shorts job")
	}
	return nil
}

// handleStartShorts starts the shorts Step Functions pipeline for a job.
func (h *APIHandler) handleStartShorts(ctx context.Context, req events.APIGatewayProxyRequest, claims *models.JWTClaims) (events.APIGatewayProxyResponse, error) {
	jobID := extractShortsIDFromSubpath(req.Path, "/start")
	if jobID == "" {
		return errorResponse(http.StatusBadRequest, "INVALID_INPUT", "Job ID is required"), nil
	}

	job, err := h.getShortsJobByID(ctx, claims.UserID, jobID)
	if err != nil {
		return errorResponse(http.StatusNotFound, "NOT_FOUND", "Shorts job not found"), nil
	}

	sfnInput, _ := json.Marshal(map[string]interface{}{
		"jobId":          job.JobID,
		"userId":         claims.UserID,
		"sourceKey":      job.SourceKey,
		"sourceDuration": job.SourceDuration,
	})

	_, err = h.SFN.StartExecution(ctx, &sfn.StartExecutionInput{
		StateMachineArn: aws.String(h.ShortsStateMachineARN),
		Name:            aws.String(fmt.Sprintf("shorts-%s", jobID)),
		Input:           aws.String(string(sfnInput)),
	})
	if err != nil {
		return errorResponse(http.StatusInternalServerError, "PIPELINE_ERROR", "Failed to start processing pipeline"), nil
	}

	return jsonResponse(http.StatusOK, map[string]string{
		"status": "started",
	}), nil
}

// handleGetShorts returns the current shorts job (status and metadata).
func (h *APIHandler) handleGetShorts(ctx context.Context, req events.APIGatewayProxyRequest, claims *models.JWTClaims) (events.APIGatewayProxyResponse, error) {
	jobID := extractShortsID(req.Path)
	if jobID == "" {
		return errorResponse(http.StatusBadRequest, "INVALID_INPUT", "Job ID is required"), nil
	}

	job, err := h.getShortsJobByID(ctx, claims.UserID, jobID)
	if err != nil {
		return errorResponse(http.StatusNotFound, "NOT_FOUND", "Shorts job not found"), nil
	}

	return jsonResponse(http.StatusOK, job), nil
}

// handleListClips returns the job's clips ordered by rank ascending.
func (h *APIHandler) handleListClips(ctx context.Context, req events.APIGatewayProxyRequest, claims *models.JWTClaims) (events.APIGatewayProxyResponse, error) {
	jobID := extractShortsIDFromSubpath(req.Path, "/clips")
	if jobID == "" {
		return errorResponse(http.StatusBadRequest, "INVALID_INPUT", "Job ID is required"), nil
	}

	job, err := h.getShortsJobByID(ctx, claims.UserID, jobID)
	if err != nil {
		return errorResponse(http.StatusNotFound, "NOT_FOUND", "Shorts job not found"), nil
	}

	clips := make([]models.Clip, len(job.Clips))
	copy(clips, job.Clips)
	sort.SliceStable(clips, func(i, j int) bool {
		return clips[i].Rank < clips[j].Rank
	})

	return jsonResponse(http.StatusOK, map[string]interface{}{
		"clips": clips,
	}), nil
}

// handleGetClipURL returns a presigned GET URL for a specific clip in a job.
func (h *APIHandler) handleGetClipURL(ctx context.Context, req events.APIGatewayProxyRequest, claims *models.JWTClaims) (events.APIGatewayProxyResponse, error) {
	jobID, clipID := extractShortsClipIDs(req.Path)
	if jobID == "" || clipID == "" {
		return errorResponse(http.StatusBadRequest, "INVALID_INPUT", "Job ID and clip ID are required"), nil
	}

	job, err := h.getShortsJobByID(ctx, claims.UserID, jobID)
	if err != nil {
		return errorResponse(http.StatusNotFound, "NOT_FOUND", "Shorts job not found"), nil
	}

	var clip *models.Clip
	for i := range job.Clips {
		if job.Clips[i].ClipID == clipID {
			clip = &job.Clips[i]
			break
		}
	}
	if clip == nil {
		return errorResponse(http.StatusNotFound, "NOT_FOUND", "Clip not found"), nil
	}

	url, restoring, err := h.downloadURLOrRestore(ctx, clip.S3Key)
	if err != nil {
		if errors.Is(err, errObjectNotFound) {
			return errorResponse(http.StatusNotFound, "NOT_FOUND", "Clip not found"), nil
		}
		return errorResponse(http.StatusInternalServerError, "S3_ERROR", "Failed to generate clip URL"), nil
	}
	if restoring {
		return restoringResponse(), nil
	}

	return jsonResponse(http.StatusOK, map[string]string{
		"url": url,
	}), nil
}

// getShortsJobByID retrieves a shorts job from DynamoDB scoped to the owner.
// Because the read is keyed by PK = USER#{userId}, another user's job is not
// present in the caller's partition and is reported as not found — enforcing
// owner scoping (Requirements 8.1, 8.2).
func (h *APIHandler) getShortsJobByID(ctx context.Context, userID, jobID string) (*models.ShortsJob, error) {
	result, err := h.DB.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(h.TableName),
		Key: map[string]dbtypes.AttributeValue{
			"PK": &dbtypes.AttributeValueMemberS{Value: "USER#" + userID},
			"SK": &dbtypes.AttributeValueMemberS{Value: "SHORTS#" + jobID},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("get item: %w", err)
	}
	if result.Item == nil {
		return nil, fmt.Errorf("shorts job not found")
	}

	job := itemToShortsJob(result.Item)
	return &job, nil
}

// extractShortsID extracts the job ID from a path like /shorts/{id}.
func extractShortsID(path string) string {
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(parts) >= 2 && parts[0] == "shorts" {
		return parts[1]
	}
	return ""
}

// extractShortsIDFromSubpath extracts the job ID from a path like
// /shorts/{id}/subpath.
func extractShortsIDFromSubpath(path, suffix string) string {
	path = strings.TrimSuffix(path, suffix)
	return extractShortsID(path)
}

// extractShortsClipIDs extracts (jobID, clipID) from a path like
// /shorts/{id}/clips/{clipId}/url.
func extractShortsClipIDs(path string) (string, string) {
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	// shorts / {id} / clips / {clipId} / url
	if len(parts) >= 5 && parts[0] == "shorts" && parts[2] == "clips" && parts[4] == "url" {
		return parts[1], parts[3]
	}
	return "", ""
}

// shortsJobToItem converts a ShortsJob model to a DynamoDB item, following the
// projectToItem pattern; slices are stored as JSON strings.
func shortsJobToItem(j models.ShortsJob) map[string]dbtypes.AttributeValue {
	item := map[string]dbtypes.AttributeValue{
		"PK":             &dbtypes.AttributeValueMemberS{Value: "USER#" + j.UserID},
		"SK":             &dbtypes.AttributeValueMemberS{Value: "SHORTS#" + j.JobID},
		"jobId":          &dbtypes.AttributeValueMemberS{Value: j.JobID},
		"status":         &dbtypes.AttributeValueMemberS{Value: j.Status},
		"fileType":       &dbtypes.AttributeValueMemberS{Value: j.FileType},
		"sourceDuration": &dbtypes.AttributeValueMemberN{Value: fmt.Sprintf("%g", j.SourceDuration)},
		"sourceKey":      &dbtypes.AttributeValueMemberS{Value: j.SourceKey},
		"createdAt":      &dbtypes.AttributeValueMemberS{Value: j.CreatedAt},
		"updatedAt":      &dbtypes.AttributeValueMemberS{Value: j.UpdatedAt},
	}
	if j.TranscriptKey != "" {
		item["transcriptKey"] = &dbtypes.AttributeValueMemberS{Value: j.TranscriptKey}
	}
	if j.ProjectID != "" {
		item["projectId"] = &dbtypes.AttributeValueMemberS{Value: j.ProjectID}
	}
	if len(j.Segments) > 0 {
		if b, err := json.Marshal(j.Segments); err == nil {
			item["segments"] = &dbtypes.AttributeValueMemberS{Value: string(b)}
		}
	}
	if len(j.Clips) > 0 {
		if b, err := json.Marshal(j.Clips); err == nil {
			item["clips"] = &dbtypes.AttributeValueMemberS{Value: string(b)}
		}
	}
	if j.Error != "" {
		item["error"] = &dbtypes.AttributeValueMemberS{Value: j.Error}
	}
	if j.CompletedAt != "" {
		item["completedAt"] = &dbtypes.AttributeValueMemberS{Value: j.CompletedAt}
	}
	return item
}

// itemToShortsJob converts a DynamoDB item to a ShortsJob model.
func itemToShortsJob(item map[string]dbtypes.AttributeValue) models.ShortsJob {
	j := models.ShortsJob{}
	if v, ok := item["jobId"].(*dbtypes.AttributeValueMemberS); ok {
		j.JobID = v.Value
	}
	if v, ok := item["PK"].(*dbtypes.AttributeValueMemberS); ok && len(v.Value) > 5 {
		j.UserID = v.Value[5:] // Strip "USER#" prefix
	}
	if v, ok := item["status"].(*dbtypes.AttributeValueMemberS); ok {
		j.Status = v.Value
	}
	if v, ok := item["fileType"].(*dbtypes.AttributeValueMemberS); ok {
		j.FileType = v.Value
	}
	if v, ok := item["sourceDuration"].(*dbtypes.AttributeValueMemberN); ok {
		fmt.Sscanf(v.Value, "%g", &j.SourceDuration)
	}
	if v, ok := item["sourceKey"].(*dbtypes.AttributeValueMemberS); ok {
		j.SourceKey = v.Value
	}
	if v, ok := item["transcriptKey"].(*dbtypes.AttributeValueMemberS); ok {
		j.TranscriptKey = v.Value
	}
	if v, ok := item["projectId"].(*dbtypes.AttributeValueMemberS); ok {
		j.ProjectID = v.Value
	}
	if v, ok := item["segments"].(*dbtypes.AttributeValueMemberS); ok && v.Value != "" {
		_ = json.Unmarshal([]byte(v.Value), &j.Segments)
	}
	if v, ok := item["clips"].(*dbtypes.AttributeValueMemberS); ok && v.Value != "" {
		_ = json.Unmarshal([]byte(v.Value), &j.Clips)
	}
	if v, ok := item["error"].(*dbtypes.AttributeValueMemberS); ok {
		j.Error = v.Value
	}
	if v, ok := item["createdAt"].(*dbtypes.AttributeValueMemberS); ok {
		j.CreatedAt = v.Value
	}
	if v, ok := item["updatedAt"].(*dbtypes.AttributeValueMemberS); ok {
		j.UpdatedAt = v.Value
	}
	if v, ok := item["completedAt"].(*dbtypes.AttributeValueMemberS); ok {
		j.CompletedAt = v.Value
	}
	return j
}
