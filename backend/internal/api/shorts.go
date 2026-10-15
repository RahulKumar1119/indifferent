package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
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
	job, contentType := newShortsJob(userID, fileType, duration, projectID)

	// Presign before any write: S3 problems then fail with nothing persisted.
	uploadURL, err := h.S3.GenerateUploadURL(ctx, h.Bucket, job.SourceKey, contentType, storage.UploadURLExpiration)
	if err != nil {
		return models.ShortsJob{}, "", fmt.Errorf("failed to generate upload URL")
	}

	if _, err := h.DB.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(h.TableName),
		Item:      shortsJobToItem(job),
	}); err != nil {
		return models.ShortsJob{}, "", fmt.Errorf("failed to create shorts job")
	}

	if projectID != "" {
		if err := h.linkShortsJob(ctx, userID, projectID, job.JobID); err != nil {
			// Compensate: don't leave an unlinked job row behind.
			_, _ = h.DB.DeleteItem(ctx, &dynamodb.DeleteItemInput{
				TableName: aws.String(h.TableName),
				Key: map[string]dbtypes.AttributeValue{
					"PK": &dbtypes.AttributeValueMemberS{Value: "USER#" + userID},
					"SK": &dbtypes.AttributeValueMemberS{Value: "SHORTS#" + job.JobID},
				},
			})
			return models.ShortsJob{}, "", err
		}
	}

	return job, uploadURL, nil
}

// newShortsJob builds the job record (and its upload content type) without
// touching storage: callers presign first so failures happen before any row
// is written.
func newShortsJob(userID, fileType string, duration float64, projectID string) (models.ShortsJob, string) {
	ext := strings.ToLower(strings.TrimSpace(fileType))
	contentType := validShortsFileTypes[ext]
	now := time.Now().UTC().Format(time.RFC3339)

	jobID := uuid.New().String()
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
	return job, contentType
}

// linkShortsJob appends a job ID to its unified project (idempotent). The
// write is optimistically locked so a concurrent status update is retried
// instead of clobbered.
func (h *APIHandler) linkShortsJob(ctx context.Context, userID, projectID, jobID string) error {
	_, err := h.updateProjectFields(ctx, userID, projectID, func(project *models.Project) {
		for _, id := range project.ShortsJobIDs {
			if id == jobID {
				return
			}
		}
		project.ShortsJobIDs = append(project.ShortsJobIDs, jobID)
	})
	if err != nil {
		return fmt.Errorf("failed to link shorts job: %w", err)
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

	// Only a freshly uploaded job may start the pipeline: re-running a
	// completed/failed/in-flight job would duplicate expensive transcribe,
	// rank and render work (or collide on the deterministic execution name).
	if job.Status != "uploaded" {
		return errorResponse(http.StatusBadRequest, "INVALID_STATE", "Shorts pipeline has already been started"), nil
	}

	// Verify the upload actually landed: presence, non-emptiness, size cap,
	// and a sane content type. The presigned URL alone guarantees none of
	// these (no size/type conditions), and the client-supplied duration is
	// advisory only.
	var problem *uploadProblem
	if err := h.verifyShortsUpload(ctx, job); errors.As(err, &problem) {
		return errorResponse(http.StatusBadRequest, problem.code, problem.message), nil
	} else if err != nil {
		return errorResponse(http.StatusInternalServerError, "S3_ERROR", "Failed to verify upload. Please try again."), nil
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
		// Idempotent restart: the deterministic execution name means a
		// duplicate start races an already-running execution.
		if isExecutionAlreadyExists(err) {
			log.Printf("start_shorts_already_running job=%s", jobID)
			return jsonResponse(http.StatusOK, map[string]string{
				"status": "started",
			}), nil
		}
		return errorResponse(http.StatusInternalServerError, "PIPELINE_ERROR", "Failed to start processing pipeline"), nil
	}

	return jsonResponse(http.StatusOK, map[string]string{
		"status": "started",
	}), nil
}

// handleCancelShorts stops a running shorts pipeline execution and marks the
// job failed. Terminal jobs are rejected; stopping an already-finished
// execution is treated as success (the job record is the source of truth).
func (h *APIHandler) handleCancelShorts(ctx context.Context, req events.APIGatewayProxyRequest, claims *models.JWTClaims) (events.APIGatewayProxyResponse, error) {
	jobID := extractShortsIDFromSubpath(req.Path, "/cancel")
	if jobID == "" {
		return errorResponse(http.StatusBadRequest, "INVALID_INPUT", "Job ID is required"), nil
	}

	job, err := h.getShortsJobByID(ctx, claims.UserID, jobID)
	if err != nil {
		return errorResponse(http.StatusNotFound, "NOT_FOUND", "Shorts job not found"), nil
	}
	if job.Status == "completed" || job.Status == "failed" {
		return errorResponse(http.StatusBadRequest, "INVALID_STATE", "Job is already finished"), nil
	}

	executionARN, err := shortsExecutionARN(h.ShortsStateMachineARN, jobID)
	if err != nil {
		return errorResponse(http.StatusInternalServerError, "PIPELINE_ERROR", "Failed to stop processing pipeline"), nil
	}
	// Best effort: the execution may already be gone; the record update below
	// is what the UI observes.
	_, _ = h.SFN.StopExecution(ctx, &sfn.StopExecutionInput{
		ExecutionArn: aws.String(executionARN),
	})

	job.Status = "failed"
	job.Error = "Cancelled by user."
	job.Progress = 0
	if _, err := h.DB.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(h.TableName),
		Item:      shortsJobToItem(*job),
	}); err != nil {
		return errorResponse(http.StatusInternalServerError, "DB_ERROR", "Failed to cancel job"), nil
	}

	return jsonResponse(http.StatusOK, map[string]string{
		"status": "cancelled",
	}), nil
}

// uploadProblem is an actionable client-facing upload rejection.
type uploadProblem struct {
	code    string
	message string
}

func (e *uploadProblem) Error() string { return e.code + ": " + e.message }

// verifyShortsUpload enforces server-side what the browser only claims:
// the source object exists, is non-empty, fits the size cap, and carries a
// content type matching its extension. S3 lookup failures (non-missing) are
// returned raw for a 500.
func (h *APIHandler) verifyShortsUpload(ctx context.Context, job *models.ShortsJob) error {
	meta, err := h.S3.ObjectMeta(ctx, h.Bucket, job.SourceKey)
	if err != nil {
		return err
	}
	if !meta.Found {
		return &uploadProblem{
			code:    "UPLOAD_NOT_FOUND",
			message: "Upload not found. Your file may not have finished uploading — please upload it again.",
		}
	}
	if meta.Size == 0 {
		return &uploadProblem{
			code:    "EMPTY_UPLOAD",
			message: "Uploaded file is empty. Please select a file with content.",
		}
	}
	if meta.Size > shorts.MaxUploadBytes {
		return &uploadProblem{
			code:    "UPLOAD_TOO_LARGE",
			message: fmt.Sprintf("Uploaded file is %.0f MB; maximum is %d MB. Please use a smaller file.", float64(meta.Size)/1024/1024, shorts.MaxUploadBytes/1024/1024),
		}
	}
	if want, ok := validShortsFileTypes[strings.ToLower(strings.TrimSpace(job.FileType))]; ok && meta.ContentType != "" {
		if !strings.EqualFold(strings.TrimSpace(strings.Split(meta.ContentType, ";")[0]), want) {
			return &uploadProblem{
				code:    "UPLOAD_TYPE_MISMATCH",
				message: fmt.Sprintf("Uploaded file looks like %s, not %s. Please re-upload the correct file.", meta.ContentType, strings.ToUpper(job.FileType)),
			}
		}
	}
	return nil
}

// handleRetryShorts restarts a failed job from the source media: the record
// is conditionally reset to uploaded (concurrent retries collide on the
// condition instead of double-running the pipeline) and a fresh execution is
// started under a unique name. Only failed/delete_failed jobs may restart;
// stale clips are cleared so the gallery never shows previous-run renders
// mid-rerun. Validation failures (VALIDATION_*) restart the same way — the
// pipeline will fail fast again unless the input changed — so the UI steers
// those toward a different file instead.
func (h *APIHandler) handleRetryShorts(ctx context.Context, req events.APIGatewayProxyRequest, claims *models.JWTClaims) (events.APIGatewayProxyResponse, error) {
	jobID := extractShortsIDFromSubpath(req.Path, "/retry")
	if jobID == "" {
		return errorResponse(http.StatusBadRequest, "INVALID_INPUT", "Job ID is required"), nil
	}

	job, err := h.getShortsJobByID(ctx, claims.UserID, jobID)
	if err != nil {
		return errorResponse(http.StatusNotFound, "NOT_FOUND", "Shorts job not found"), nil
	}
	if job.Status != "failed" && job.Status != "delete_failed" {
		return errorResponse(http.StatusBadRequest, "INVALID_STATE", "Only failed jobs can be retried"), nil
	}

	reset := *job
	reset.Status = "uploaded"
	reset.Error = ""
	reset.ErrorCode = ""
	reset.FailedStage = ""
	reset.Progress = 0
	reset.ProgressDetail = ""
	reset.Clips = nil
	reset.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	_, err = h.DB.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:           aws.String(h.TableName),
		Item:                shortsJobToItem(reset),
		ConditionExpression: aws.String("#status IN (:failed, :deleteFailed)"),
		ExpressionAttributeNames: map[string]string{
			"#status": "status",
		},
		ExpressionAttributeValues: map[string]dbtypes.AttributeValue{
			":failed":       &dbtypes.AttributeValueMemberS{Value: "failed"},
			":deleteFailed": &dbtypes.AttributeValueMemberS{Value: "delete_failed"},
		},
	})
	if err != nil {
		var condFailed *dbtypes.ConditionalCheckFailedException
		if errors.As(err, &condFailed) {
			return errorResponse(http.StatusConflict, "CONFLICT", "Job was already restarted."), nil
		}
		return errorResponse(http.StatusInternalServerError, "DB_ERROR", "Failed to restart job."), nil
	}

	sfnInput, _ := json.Marshal(map[string]interface{}{
		"jobId":          reset.JobID,
		"userId":         claims.UserID,
		"sourceKey":      reset.SourceKey,
		"sourceDuration": reset.SourceDuration,
	})
	_, err = h.SFN.StartExecution(ctx, &sfn.StartExecutionInput{
		StateMachineArn: aws.String(h.ShortsStateMachineARN),
		Name:            aws.String(fmt.Sprintf("shorts-%s-r%d", jobID, time.Now().UnixNano())),
		Input:           aws.String(string(sfnInput)),
	})
	if err != nil {
		if isExecutionAlreadyExists(err) {
			log.Printf("retry_shorts_already_running job=%s", jobID)
			return jsonResponse(http.StatusOK, map[string]string{
				"status": "restarted",
			}), nil
		}
		return errorResponse(http.StatusInternalServerError, "PIPELINE_ERROR", "Failed to restart processing pipeline"), nil
	}

	log.Printf("shorts_retried user=%s job=%s", claims.UserID, jobID)
	return jsonResponse(http.StatusOK, map[string]string{
		"status": "restarted",
	}), nil
}

// UpdateCaptionRequest carries the reusable caption style for a job.
type UpdateCaptionRequest struct {
	Caption models.CaptionStyle `json:"caption"`
}

// handleUpdateCaption stores a validated caption style on a shorts job for
// future renders (including retries, which preserve it). Styles apply only
// before rendering starts: changing mid-render would race the worker, and
// changing a finished job would mislead (nothing left to render).
func (h *APIHandler) handleUpdateCaption(ctx context.Context, req events.APIGatewayProxyRequest, claims *models.JWTClaims) (events.APIGatewayProxyResponse, error) {
	jobID := extractShortsIDFromSubpath(req.Path, "/style")
	if jobID == "" {
		return errorResponse(http.StatusBadRequest, "INVALID_INPUT", "Job ID is required"), nil
	}

	var body UpdateCaptionRequest
	if err := json.Unmarshal([]byte(req.Body), &body); err != nil {
		return errorResponse(http.StatusBadRequest, "INVALID_BODY", "Invalid request body"), nil
	}
	if err := body.Caption.Validate(); err != nil {
		return errorResponse(http.StatusBadRequest, "VALIDATION_ERROR", err.Error()), nil
	}

	job, err := h.getShortsJobByID(ctx, claims.UserID, jobID)
	if err != nil {
		return errorResponse(http.StatusNotFound, "NOT_FOUND", "Shorts job not found"), nil
	}
	if job.Status != "uploaded" && job.Status != "failed" {
		return errorResponse(http.StatusBadRequest, "INVALID_STATE", "Caption style can only be changed before rendering starts."), nil
	}

	job.Caption = body.Caption
	job.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if _, err := h.DB.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(h.TableName),
		Item:      shortsJobToItem(*job),
	}); err != nil {
		return errorResponse(http.StatusInternalServerError, "DB_ERROR", "Failed to save caption style."), nil
	}

	return jsonResponse(http.StatusOK, job), nil
}

// extractClipPath splits /shorts/{jobId}/clips/{clipId} exactly (len 4).
func extractClipPath(path string) (jobID, clipID string, ok bool) {
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(parts) == 4 && parts[0] == "shorts" && parts[1] != "" && parts[2] == "clips" && parts[3] != "" {
		return parts[1], parts[3], true
	}
	return "", "", false
}

// UpdateClipEditRequest saves (or, with reset:true, clears) a clip override.
type UpdateClipEditRequest struct {
	Start float64       `json:"start"`
	End   float64       `json:"end"`
	Words []models.Word `json:"words,omitempty"`
	Reset bool          `json:"reset,omitempty"`
}

// handleUpdateClipEdit stores a validated boundary/caption override for one
// clip, or clears it back to the AI suggestion. Edits apply on the next
// render or retry. In-flight jobs are rejected (moving target); completed
// and failed jobs accept edits.
func (h *APIHandler) handleUpdateClipEdit(ctx context.Context, req events.APIGatewayProxyRequest, claims *models.JWTClaims) (events.APIGatewayProxyResponse, error) {
	jobID, clipID, ok := extractClipPath(req.Path)
	if !ok {
		return errorResponse(http.StatusNotFound, "NOT_FOUND", "Route not found"), nil
	}

	var body UpdateClipEditRequest
	if err := json.Unmarshal([]byte(req.Body), &body); err != nil {
		return errorResponse(http.StatusBadRequest, "INVALID_BODY", "Invalid request body"), nil
	}

	job, err := h.getShortsJobByID(ctx, claims.UserID, jobID)
	if err != nil {
		return errorResponse(http.StatusNotFound, "NOT_FOUND", "Shorts job not found"), nil
	}
	switch job.Status {
	case "completed", "failed", "uploaded":
	default:
		return errorResponse(http.StatusBadRequest, "INVALID_STATE", "Clip edits are accepted before rendering starts and after it finishes, not mid-flight."), nil
	}
	if !clipKnown(job, clipID) {
		return errorResponse(http.StatusNotFound, "NOT_FOUND", "Clip not found"), nil
	}

	if job.ClipEdits == nil {
		job.ClipEdits = map[string]models.ClipEdit{}
	}
	if body.Reset {
		delete(job.ClipEdits, clipID)
	} else {
		edit := models.ClipEdit{Start: body.Start, End: body.End, Words: body.Words}
		if err := edit.Validate(job.SourceDuration); err != nil {
			return errorResponse(http.StatusBadRequest, "VALIDATION_ERROR", err.Error()), nil
		}
		dropped := 0
		for _, w := range edit.Words {
			if w.End < edit.Start || w.Start > edit.End {
				dropped++
			}
		}
		edit.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		job.ClipEdits[clipID] = edit
		job.UpdatedAt = edit.UpdatedAt
		if _, err := h.DB.PutItem(ctx, &dynamodb.PutItemInput{
			TableName: aws.String(h.TableName),
			Item:      shortsJobToItem(*job),
		}); err != nil {
			return errorResponse(http.StatusInternalServerError, "DB_ERROR", "Failed to save clip edit."), nil
		}
		return jsonResponse(http.StatusOK, map[string]interface{}{
			"edit":         edit,
			"droppedWords": dropped,
		}), nil
	}
	job.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if _, err := h.DB.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(h.TableName),
		Item:      shortsJobToItem(*job),
	}); err != nil {
		return errorResponse(http.StatusInternalServerError, "DB_ERROR", "Failed to reset clip edit."), nil
	}
	return jsonResponse(http.StatusOK, map[string]interface{}{
		"reset": true,
	}), nil
}

// clipKnown reports whether clipId matches a rendered clip or a ranked
// segment of the job.
func clipKnown(job *models.ShortsJob, clipID string) bool {
	for _, c := range job.Clips {
		if c.ClipID == clipID {
			return true
		}
	}
	for _, s := range job.Segments {
		if s.ClipID == clipID {
			return true
		}
	}
	return false
}

// handleGetClipCaptions returns the transcript words inside one clip's window
// (absolute timings) so the editor can offer text corrections that preserve
// timing. Applies the saved boundary override when one exists.
func (h *APIHandler) handleGetClipCaptions(ctx context.Context, req events.APIGatewayProxyRequest, claims *models.JWTClaims) (events.APIGatewayProxyResponse, error) {
	trimmed := strings.TrimSuffix(req.Path, "/captions")
	jobID, clipID, ok := extractClipPath(trimmed)
	if !ok {
		return errorResponse(http.StatusNotFound, "NOT_FOUND", "Route not found"), nil
	}

	job, err := h.getShortsJobByID(ctx, claims.UserID, jobID)
	if err != nil {
		return errorResponse(http.StatusNotFound, "NOT_FOUND", "Shorts job not found"), nil
	}
	if !clipKnown(job, clipID) {
		return errorResponse(http.StatusNotFound, "NOT_FOUND", "Clip not found"), nil
	}
	if job.TranscriptKey == "" {
		return errorResponse(http.StatusBadRequest, "NOT_READY", "Transcript is not available yet."), nil
	}

	raw, err := h.S3.GetObject(ctx, h.Bucket, job.TranscriptKey)
	if err != nil {
		return errorResponse(http.StatusInternalServerError, "S3_ERROR", "Failed to read transcript."), nil
	}
	var transcript models.Transcript
	if err := json.Unmarshal(raw, &transcript); err != nil {
		return errorResponse(http.StatusInternalServerError, "S3_ERROR", "Transcript is corrupt."), nil
	}

	start, end := clipWindow(job, clipID)
	var words []models.Word
	for _, w := range transcript.Words {
		if w.End < start || w.Start > end {
			continue
		}
		words = append(words, w)
	}
	return jsonResponse(http.StatusOK, map[string]interface{}{
		"start": start,
		"end":   end,
		"words": words,
	}), nil
}

// clipWindow resolves a clip's [start, end]: the saved override wins,
// otherwise the rendered/ranked bounds, otherwise the full source.
func clipWindow(job *models.ShortsJob, clipID string) (float64, float64) {
	if edit, ok := job.ClipEdits[clipID]; ok && edit.End > edit.Start {
		return edit.Start, edit.End
	}
	for _, c := range job.Clips {
		if c.ClipID == clipID && c.End > c.Start {
			return c.Start, c.End
		}
	}
	for _, s := range job.Segments {
		if s.ClipID == clipID && s.End > s.Start {
			return s.Start, s.End
		}
	}
	return 0, job.SourceDuration
}

// isExactShortsPath reports whether path is exactly /shorts/{id} (no deeper
// sub-resource): only the job itself is deletable via DELETE, deeper DELETE
// paths fall through to 404.
func isExactShortsPath(path string) bool {
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	return len(parts) == 2 && parts[0] == "shorts" && parts[1] != ""
}

// shortsJobInFlight reports whether a job may still have a running Step
// Functions execution that must be stopped before deletion.
func shortsJobInFlight(status string) bool {
	switch status {
	case "uploaded", "transcribing", "ranking", "rendering":
		return true
	}
	return false
}

// handleDeleteShorts permanently deletes a shorts job: it stops any running
// pipeline, removes every S3 asset, deletes the record, and unlinks the job
// from the user's projects.
//
// Ownership comes from the JWT partition key (PK=USER#<id>), so another
// user's job reads as not found: 404 either way, no existence leak. The
// client sends only the job ID; asset keys always come from the stored
// record, never from the request.
//
// The operation is idempotent: S3 treats missing keys as success, so a retry
// after a failed cleanup resumes from the retained record (status
// delete_failed) instead of needing new infrastructure.
// ErrShortsJobNotFound marks a delete/remove of a missing (or foreign) job:
// callers report 404 without leaking existence.
var ErrShortsJobNotFound = errors.New("shorts job not found")

// ErrShortsCleanupFailed marks S3 cleanup failure with the record retained
// for retry: callers report a retryable 500.
var ErrShortsCleanupFailed = errors.New("shorts s3 cleanup failed")

func (h *APIHandler) handleDeleteShorts(ctx context.Context, req events.APIGatewayProxyRequest, claims *models.JWTClaims) (events.APIGatewayProxyResponse, error) {
	jobID := extractShortsID(req.Path)
	if jobID == "" {
		return errorResponse(http.StatusBadRequest, "INVALID_INPUT", "Job ID is required"), nil
	}
	if !isExactShortsPath(req.Path) {
		return errorResponse(http.StatusNotFound, "NOT_FOUND", "Route not found"), nil
	}
	switch err := h.removeShortsJob(ctx, claims.UserID, jobID); {
	case err == nil:
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusNoContent,
			Headers:    corsHeaders(),
		}, nil
	case errors.Is(err, ErrShortsJobNotFound):
		return errorResponse(http.StatusNotFound, "NOT_FOUND", "Shorts job not found"), nil
	case errors.Is(err, ErrShortsCleanupFailed):
		return errorResponse(http.StatusInternalServerError, "S3_CLEANUP_FAILED", "Failed to delete Short assets. Please retry."), nil
	default:
		return errorResponse(http.StatusInternalServerError, "DB_ERROR", "Failed to delete Short."), nil
	}
}

// removeShortsJob executes the deletion shared by the DELETE route and
// project cleanup: stop pipeline, sweep S3, delete record, unlink, purge.
func (h *APIHandler) removeShortsJob(ctx context.Context, userID, jobID string) error {
	job, err := h.getShortsJobByID(ctx, userID, jobID)
	if err != nil {
		return ErrShortsJobNotFound
	}

	// Best effort: a finished execution may already be gone; the record and
	// S3 cleanup below are what the UI observes.
	if shortsJobInFlight(job.Status) {
		if arn, aerr := shortsExecutionARN(h.ShortsStateMachineARN, jobID); aerr == nil {
			_, _ = h.SFN.StopExecution(ctx, &sfn.StopExecutionInput{
				ExecutionArn: aws.String(arn),
			})
		}
	}

	keys, err := h.shortsSweepKeys(ctx, userID, job)
	if err != nil {
		// Enumeration needs s3:ListBucket; if it is unavailable (or any
		// listing fails), fall back to the record's tracked keys rather
		// than failing the whole delete — the large assets still go.
		log.Printf("shorts_delete_list_failed user=%s job=%s err=%v (falling back to tracked keys)", userID, jobID, err)
		keys = constrainToPrefixes(shortsAssetKeys(job), shortsJobPrefixes(userID, jobID))
	}
	if err := h.S3.DeleteObjects(ctx, h.Bucket, keys); err != nil {
		log.Printf("shorts_delete_s3_failed user=%s job=%s keys=%d err=%v", userID, jobID, len(keys), err)
		job.Status = "delete_failed"
		job.Error = "S3 cleanup failed; retry deletion."
		job.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		if _, perr := h.DB.PutItem(ctx, &dynamodb.PutItemInput{
			TableName: aws.String(h.TableName),
			Item:      shortsJobToItem(*job),
		}); perr != nil {
			log.Printf("shorts_delete_mark_failed user=%s job=%s err=%v", userID, jobID, perr)
		}
		return ErrShortsCleanupFailed
	}

	if _, err := h.DB.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(h.TableName),
		Key: map[string]dbtypes.AttributeValue{
			"PK": &dbtypes.AttributeValueMemberS{Value: "USER#" + userID},
			"SK": &dbtypes.AttributeValueMemberS{Value: "SHORTS#" + jobID},
		},
	}); err != nil {
		log.Printf("shorts_delete_db_failed user=%s job=%s err=%v", userID, jobID, err)
		return fmt.Errorf("delete record: %w", err)
	}

	// Best effort: drop dangling references from the user's projects.
	h.unlinkShortsJob(ctx, userID, jobID)

	// The bucket is versioned: plain deletes only hide objects, so purge all
	// versions/markers or storage costs linger. A purge failure is logged but
	// not fatal — the assets are already unreachable.
	for _, p := range shortsJobPrefixes(userID, jobID) {
		if err := h.S3.DeleteObjectVersions(ctx, h.Bucket, p); err != nil {
			log.Printf("shorts_delete_purge_failed user=%s job=%s prefix=%s err=%v", userID, jobID, p, err)
		}
	}

	log.Printf("short_deleted user=%s job=%s keys=%d s3=success", userID, jobID, len(keys))
	return nil
}

// shortsJobPrefixes are the only S3 locations a job may own. Cleanup stays
// scoped to these exact prefixes, so sibling jobs and projects are untouched.
func shortsJobPrefixes(userID, jobID string) []string {
	return []string{
		"uploads/" + userID + "/" + jobID + "/",
		"shorts/" + userID + "/" + jobID + "/",
	}
}

// shortsSweepKeys lists every object under the job's prefixes — catching
// untracked intermediates the record never references (transcribe-raw.json,
// access-check temp files) — plus the record's tracked keys, but only when
// they fall under those prefixes. A record can therefore never cause deletion
// outside the job's own directories.
func (h *APIHandler) shortsSweepKeys(ctx context.Context, userID string, job *models.ShortsJob) ([]string, error) {
	prefixes := shortsJobPrefixes(userID, job.JobID)
	seen := map[string]bool{}
	var keys []string
	add := func(k string) {
		if k == "" || seen[k] {
			return
		}
		seen[k] = true
		keys = append(keys, k)
	}
	for _, p := range prefixes {
		listed, err := h.S3.ListKeys(ctx, h.Bucket, p)
		if err != nil {
			return nil, err
		}
		for _, k := range listed {
			add(k)
		}
	}
	for _, k := range shortsAssetKeys(job) {
		for _, p := range prefixes {
			if strings.HasPrefix(k, p) {
				add(k)
				break
			}
		}
		if !seen[k] && k != "" {
			log.Printf("shorts_sweep_skip_outside_prefix user=%s job=%s key=%s", userID, job.JobID, k)
		}
	}
	return keys, nil
}

// constrainToPrefixes keeps only keys under one of the prefixes (deduped).
// Used when prefix enumeration is unavailable: the record's tracked keys
// still clean the large assets without ever escaping the job directories.
func constrainToPrefixes(keys, prefixes []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, k := range keys {
		if k == "" || seen[k] {
			continue
		}
		for _, p := range prefixes {
			if strings.HasPrefix(k, p) {
				seen[k] = true
				out = append(out, k)
				break
			}
		}
	}
	return out
}

// shortsAssetKeys collects every S3 key a job may own: source media,
// transcript, and rendered clips. Deduped with empties dropped.
func shortsAssetKeys(job *models.ShortsJob) []string {
	seen := map[string]bool{}
	var keys []string
	add := func(k string) {
		if k == "" || seen[k] {
			return
		}
		seen[k] = true
		keys = append(keys, k)
	}
	add(job.SourceKey)
	add(job.TranscriptKey)
	for _, c := range job.Clips {
		add(c.S3Key)
	}
	return keys
}

// unlinkShortsJob removes jobID from any of the caller's projects that link
// it. Errors are logged, never fatal: a dangling link reads as "not found"
// wherever jobs are resolved.
func (h *APIHandler) unlinkShortsJob(ctx context.Context, userID, jobID string) {
	result, err := h.DB.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(h.TableName),
		KeyConditionExpression: aws.String("PK = :pk AND begins_with(SK, :sk)"),
		ExpressionAttributeValues: map[string]dbtypes.AttributeValue{
			":pk": &dbtypes.AttributeValueMemberS{Value: "USER#" + userID},
			":sk": &dbtypes.AttributeValueMemberS{Value: "PROJECT#"},
		},
	})
	if err != nil {
		log.Printf("shorts_unlink_query_failed user=%s job=%s err=%v", userID, jobID, err)
		return
	}
	for _, item := range result.Items {
		project := itemToProject(item)
		kept := project.ShortsJobIDs[:0]
		removed := false
		for _, id := range project.ShortsJobIDs {
			if id == jobID {
				removed = true
				continue
			}
			kept = append(kept, id)
		}
		if !removed {
			continue
		}
		project.ShortsJobIDs = kept
		project.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		if _, err := h.DB.PutItem(ctx, &dynamodb.PutItemInput{
			TableName: aws.String(h.TableName),
			Item:      projectToItem(project),
		}); err != nil {
			log.Printf("shorts_unlink_put_failed user=%s job=%s project=%s err=%v", userID, jobID, project.ProjectID, err)
		}
	}
}

// shortsExecutionARN derives the execution ARN from the state machine ARN and
// the deterministic execution name used at start ("shorts-<jobID>").
func shortsExecutionARN(stateMachineARN, jobID string) (string, error) {
	if jobID == "" {
		return "", fmt.Errorf("invalid state machine ARN")
	}
	return stateMachineExecutionARN(stateMachineARN, "shorts-"+jobID)
}

// stateMachineExecutionARN joins a state machine ARN with a deterministic
// execution name into a stoppable/describable execution ARN.
func stateMachineExecutionARN(stateMachineARN, executionName string) (string, error) {
	const marker = ":stateMachine:"
	i := strings.Index(stateMachineARN, marker)
	if i < 0 || executionName == "" {
		return "", fmt.Errorf("invalid state machine ARN")
	}
	return stateMachineARN[:i] + ":execution:" + stateMachineARN[i+len(marker):] + ":" + executionName, nil
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

// ShortsJobSummary is the list view of a shorts job: identity, progress and
// output counts without the heavy transcript/caption payloads.
type ShortsJobSummary struct {
	JobID          string  `json:"jobId"`
	ProjectID      string  `json:"projectId,omitempty"`
	Status         string  `json:"status"`
	FileType       string  `json:"fileType"`
	SourceDuration float64 `json:"sourceDuration"`
	ClipCount      int     `json:"clipCount"`
	CreatedAt      string  `json:"createdAt"`
	UpdatedAt      string  `json:"updatedAt"`
}

// handleListShortsJobs returns the caller's shorts jobs, newest first.
func (h *APIHandler) handleListShortsJobs(ctx context.Context, req events.APIGatewayProxyRequest, claims *models.JWTClaims) (events.APIGatewayProxyResponse, error) {
	result, err := h.DB.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(h.TableName),
		KeyConditionExpression: aws.String("PK = :pk AND begins_with(SK, :sk)"),
		ExpressionAttributeValues: map[string]dbtypes.AttributeValue{
			":pk": &dbtypes.AttributeValueMemberS{Value: "USER#" + claims.UserID},
			":sk": &dbtypes.AttributeValueMemberS{Value: "SHORTS#"},
		},
		ScanIndexForward: aws.Bool(false),
	})
	if err != nil {
		return errorResponse(http.StatusInternalServerError, "DB_ERROR", "Failed to list shorts jobs"), nil
	}

	jobs := make([]ShortsJobSummary, 0, len(result.Items))
	for _, item := range result.Items {
		job := itemToShortsJob(item)
		jobs = append(jobs, ShortsJobSummary{
			JobID:          job.JobID,
			ProjectID:      job.ProjectID,
			Status:         job.Status,
			FileType:       job.FileType,
			SourceDuration: job.SourceDuration,
			ClipCount:      len(job.Clips),
			CreatedAt:      job.CreatedAt,
			UpdatedAt:      job.UpdatedAt,
		})
	}
	sort.SliceStable(jobs, func(i, j int) bool {
		return jobs[i].CreatedAt > jobs[j].CreatedAt
	})

	return jsonResponse(http.StatusOK, map[string]interface{}{
		"jobs": jobs,
	}), nil
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

	// Clips exist only on completed renders; anything else means the caller is
	// early (or the record is corrupt) — never presign a missing key.
	if job.Status != "completed" {
		return errorResponse(http.StatusBadRequest, "NOT_READY", "Clips are not yet rendered"), nil
	}
	if clip.S3Key == "" {
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
	if j.Progress > 0 {
		item["progress"] = &dbtypes.AttributeValueMemberN{Value: strconv.Itoa(j.Progress)}
	}
	if j.ProgressDetail != "" {
		item["progressDetail"] = &dbtypes.AttributeValueMemberS{Value: j.ProgressDetail}
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
	if j.FailedStage != "" {
		item["failedStage"] = &dbtypes.AttributeValueMemberS{Value: j.FailedStage}
	}
	if j.ErrorCode != "" {
		item["errorCode"] = &dbtypes.AttributeValueMemberS{Value: j.ErrorCode}
	}
	if j.Caption.Preset != "" || j.Caption.FontSize != 0 || j.Caption.Color != "" || j.Caption.Highlight != "" || j.Caption.Position != "" {
		if b, err := json.Marshal(j.Caption); err == nil {
			item["caption"] = &dbtypes.AttributeValueMemberS{Value: string(b)}
		}
	}
	if len(j.ClipEdits) > 0 {
		if b, err := json.Marshal(j.ClipEdits); err == nil {
			item["clipEdits"] = &dbtypes.AttributeValueMemberS{Value: string(b)}
		}
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
	if v, ok := item["progress"].(*dbtypes.AttributeValueMemberN); ok {
		if n, err := strconv.Atoi(v.Value); err == nil {
			j.Progress = n
		}
	}
	if v, ok := item["progressDetail"].(*dbtypes.AttributeValueMemberS); ok {
		j.ProgressDetail = v.Value
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
	if v, ok := item["failedStage"].(*dbtypes.AttributeValueMemberS); ok {
		j.FailedStage = v.Value
	}
	if v, ok := item["errorCode"].(*dbtypes.AttributeValueMemberS); ok {
		j.ErrorCode = v.Value
	}
	if v, ok := item["caption"].(*dbtypes.AttributeValueMemberS); ok && v.Value != "" {
		_ = json.Unmarshal([]byte(v.Value), &j.Caption)
	}
	if v, ok := item["clipEdits"].(*dbtypes.AttributeValueMemberS); ok && v.Value != "" {
		_ = json.Unmarshal([]byte(v.Value), &j.ClipEdits)
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
