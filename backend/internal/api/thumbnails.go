package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sfn"
	"github.com/rahul/indifferent/backend/internal/models"
	"github.com/rahul/indifferent/backend/internal/thumbnail"
)

// AI thumbnails for an uploaded video (issue #77).
//
//	GET  /shorts/{id}/thumbnails                    current state and the thumbnails
//	POST /shorts/{id}/thumbnails                    start (or regenerate) {style?, format?}
//	POST /shorts/{id}/thumbnails/select             choose one {id}
//	GET  /shorts/{id}/thumbnails/{thumbId}/download a link that saves the JPEG

// thumbnailURLExpiry is how long a thumbnail link stays valid. The page asks
// again whenever it reloads, so an hour is plenty.
const thumbnailURLExpiry = time.Hour

var thumbIDPattern = regexp.MustCompile(`^thumb-\d{2}$`)

// thumbnailRoute is a parsed /shorts/{id}/thumbnails... path.
type thumbnailRoute struct {
	jobID   string
	action  string // "", "select" or "download"
	thumbID string
}

// parseThumbnailPath recognises the thumbnail routes and rejects anything else.
func parseThumbnailPath(path string) (thumbnailRoute, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 3 || parts[0] != "shorts" || parts[1] == "" || parts[2] != "thumbnails" {
		return thumbnailRoute{}, false
	}
	r := thumbnailRoute{jobID: parts[1]}
	switch len(parts) {
	case 3:
		return r, true
	case 4:
		if parts[3] == "select" {
			r.action = "select"
			return r, true
		}
	case 5:
		if parts[4] == "download" && thumbIDPattern.MatchString(parts[3]) {
			r.action, r.thumbID = "download", parts[3]
			return r, true
		}
	}
	return thumbnailRoute{}, false
}

func isThumbnailPath(path string) bool {
	_, ok := parseThumbnailPath(path)
	return ok
}

func (h *APIHandler) thumbnailStore() *thumbnail.Store {
	return &thumbnail.Store{DB: h.DB, Table: h.TableName}
}

// handleThumbnails routes the thumbnail endpoints.
func (h *APIHandler) handleThumbnails(ctx context.Context, req events.APIGatewayProxyRequest, claims *models.JWTClaims) (events.APIGatewayProxyResponse, error) {
	route, _ := parseThumbnailPath(req.Path)
	switch {
	case req.HTTPMethod == http.MethodGet && route.action == "":
		return h.handleGetThumbnails(ctx, route, claims)
	case req.HTTPMethod == http.MethodPost && route.action == "":
		return h.handleStartThumbnails(ctx, req, route, claims)
	case req.HTTPMethod == http.MethodPost && route.action == "select":
		return h.handleSelectThumbnail(ctx, req, route, claims)
	case req.HTTPMethod == http.MethodGet && route.action == "download":
		return h.handleDownloadThumbnail(ctx, route, claims)
	}
	return errorResponse(http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed"), nil
}

// isVideoFileType reports whether an upload type has a picture to take frames from.
func isVideoFileType(fileType string) bool {
	switch strings.ToLower(fileType) {
	case "mp4", "mov":
		return true
	}
	return false
}

// thumbnailJSON is one thumbnail as the client sees it.
type thumbnailJSON struct {
	ID            string  `json:"id"`
	Concept       string  `json:"concept"`
	Style         string  `json:"style"`
	Format        string  `json:"format"`
	Headline      string  `json:"headline"`
	SecondaryText string  `json:"secondaryText,omitempty"`
	FrameTime     float64 `json:"frameTime"`
	Width         int     `json:"width"`
	Height        int     `json:"height"`
	URL           string  `json:"url,omitempty"`
	Selected      bool    `json:"selected"`
}

// selectedJSON is the video's chosen thumbnail, in the shape the data model in
// the issue describes.
type selectedJSON struct {
	ID       string `json:"id"`
	URL      string `json:"url"`
	Type     string `json:"type"` // always "ai_generated"
	Style    string `json:"style"`
	Selected bool   `json:"selected"`
}

// thumbnailsResponse is the body of GET /shorts/{id}/thumbnails.
type thumbnailsResponse struct {
	// Status is none, queued, running, completed or failed.
	Status string `json:"status"`
	Stage  string `json:"stage,omitempty"`
	// Supported is false when the upload has no video (an audio file).
	Supported      bool                `json:"supported"`
	JobStatus      string              `json:"jobStatus"`
	ErrorCode      string              `json:"errorCode,omitempty"`
	Error          string              `json:"error,omitempty"`
	Style          string              `json:"style,omitempty"`
	Format         string              `json:"format,omitempty"`
	Generation     int                 `json:"generation,omitempty"`
	Thumbnails     []thumbnailJSON     `json:"thumbnails"`
	Thumbnail      *selectedJSON       `json:"thumbnail"`
	Analysis       *thumbnail.Analysis `json:"analysis,omitempty"`
	Funnel         *thumbnail.Funnel   `json:"funnel,omitempty"`
	TranscriptUsed bool                `json:"transcriptUsed"`
	DetectionUsed  bool                `json:"detectionUsed"`
	UpdatedAt      string              `json:"updatedAt,omitempty"`
}

// effectiveStatus reports a run that stopped updating as failed, so the page
// stops spinning on a task that crashed without recording anything.
func effectiveStatus(set *thumbnail.Set, now time.Time) (thumbnail.Status, string, string) {
	if set.Status.InFlight() {
		if t, err := time.Parse(time.RFC3339, set.UpdatedAt); err == nil && now.Sub(t) > thumbnail.StaleAfter {
			return thumbnail.StatusFailed, thumbnail.CodeTaskFailed, "Thumbnail generation stopped unexpectedly. Please try again."
		}
	}
	return set.Status, set.ErrorCode, set.Error
}

func (h *APIHandler) buildThumbnailsResponse(ctx context.Context, job *models.ShortsJob, set *thumbnail.Set) thumbnailsResponse {
	resp := thumbnailsResponse{
		Status:     "none",
		Supported:  isVideoFileType(job.FileType),
		JobStatus:  job.Status,
		Thumbnails: []thumbnailJSON{},
	}
	if set == nil {
		return resp
	}
	status, code, msg := effectiveStatus(set, time.Now())
	resp.Status = string(status)
	resp.ErrorCode, resp.Error = code, msg
	if status.InFlight() {
		resp.Stage = string(set.Stage)
	}
	resp.Style, resp.Format = string(set.Style), string(set.Format)
	resp.Generation = set.Generation
	resp.Analysis, resp.Funnel = set.Analysis, set.Funnel
	resp.TranscriptUsed, resp.DetectionUsed = set.TranscriptUsed, set.DetectionUsed
	resp.UpdatedAt = set.UpdatedAt

	for _, a := range set.Thumbnails {
		item := thumbnailJSON{
			ID: a.ID, Concept: string(a.Concept), Style: string(a.Style), Format: string(a.Format),
			Headline: a.Headline, SecondaryText: a.SecondaryText, FrameTime: a.FrameTime,
			Width: a.Width, Height: a.Height, Selected: a.ID == set.SelectedID,
		}
		url, err := h.S3.GenerateDownloadURL(ctx, h.Bucket, a.Key, thumbnailURLExpiry)
		if err != nil {
			log.Printf("thumbnail_url_failed job=%s thumb=%s err=%v", job.JobID, a.ID, err)
		} else {
			item.URL = url
		}
		resp.Thumbnails = append(resp.Thumbnails, item)
		if item.Selected && item.URL != "" {
			resp.Thumbnail = &selectedJSON{ID: a.ID, URL: item.URL, Type: "ai_generated", Style: string(a.Style), Selected: true}
		}
	}
	return resp
}

func (h *APIHandler) handleGetThumbnails(ctx context.Context, route thumbnailRoute, claims *models.JWTClaims) (events.APIGatewayProxyResponse, error) {
	job, err := h.getShortsJobByID(ctx, claims.UserID, route.jobID)
	if err != nil {
		return errorResponse(http.StatusNotFound, "NOT_FOUND", "Shorts job not found"), nil
	}
	set, err := h.thumbnailStore().Get(ctx, claims.UserID, route.jobID)
	if err != nil {
		log.Printf("thumbnails_get_failed user=%s job=%s err=%v", claims.UserID, route.jobID, err)
		return errorResponse(http.StatusInternalServerError, "DB_ERROR", "Failed to load thumbnails"), nil
	}
	return jsonResponse(http.StatusOK, h.buildThumbnailsResponse(ctx, job, set)), nil
}

// StartThumbnailsRequest is the optional body of POST /shorts/{id}/thumbnails.
type StartThumbnailsRequest struct {
	// Style is one of the seven presets. Empty lets the AI suggest one.
	Style string `json:"style"`
	// Format is "16:9" or "9:16". Empty follows the video's shape.
	Format string `json:"format"`
}

func (h *APIHandler) handleStartThumbnails(ctx context.Context, req events.APIGatewayProxyRequest, route thumbnailRoute, claims *models.JWTClaims) (events.APIGatewayProxyResponse, error) {
	if h.ThumbnailStateMachineARN == "" {
		return errorResponse(http.StatusServiceUnavailable, "THUMBNAILS_UNAVAILABLE", "AI thumbnails are not available right now."), nil
	}
	var body StartThumbnailsRequest
	if strings.TrimSpace(req.Body) != "" {
		if err := json.Unmarshal([]byte(req.Body), &body); err != nil {
			return errorResponse(http.StatusBadRequest, "INVALID_BODY", "Invalid request body"), nil
		}
	}
	style, err := thumbnail.ParseStyle(body.Style)
	if err != nil {
		return errorResponse(http.StatusBadRequest, "VALIDATION_ERROR", err.Error()), nil
	}
	format, err := thumbnail.ParseFormat(body.Format)
	if err != nil {
		return errorResponse(http.StatusBadRequest, "VALIDATION_ERROR", err.Error()), nil
	}

	job, err := h.getShortsJobByID(ctx, claims.UserID, route.jobID)
	if err != nil {
		return errorResponse(http.StatusNotFound, "NOT_FOUND", "Shorts job not found"), nil
	}
	if !isVideoFileType(job.FileType) {
		return errorResponse(http.StatusBadRequest, thumbnail.CodeNoVideoTrack, "This file has no video to make a thumbnail from."), nil
	}
	// The source is only known to be in place once the pipeline has begun.
	if job.Status == "uploaded" || job.Status == "delete_failed" {
		return errorResponse(http.StatusBadRequest, "INVALID_STATE", "Thumbnails can be made once the video has been processed."), nil
	}

	store := h.thumbnailStore()
	if err := store.Queue(ctx, claims.UserID, route.jobID, style, format); err != nil {
		if errors.Is(err, thumbnail.ErrInFlight) {
			return errorResponse(http.StatusConflict, "IN_PROGRESS", "Thumbnails are already being generated."), nil
		}
		log.Printf("thumbnails_queue_failed user=%s job=%s err=%v", claims.UserID, route.jobID, err)
		return errorResponse(http.StatusInternalServerError, "DB_ERROR", "Failed to start thumbnail generation"), nil
	}

	input, _ := json.Marshal(map[string]interface{}{
		"jobId":          job.JobID,
		"userId":         claims.UserID,
		"sourceKey":      job.SourceKey,
		"transcriptKey":  job.TranscriptKey,
		"sourceDuration": job.SourceDuration,
	})
	_, err = h.SFN.StartExecution(ctx, &sfn.StartExecutionInput{
		StateMachineArn: aws.String(h.ThumbnailStateMachineARN),
		Name:            aws.String(fmt.Sprintf("thumbnails-%s-%d", job.JobID, time.Now().Unix())),
		Input:           aws.String(string(input)),
	})
	if err != nil {
		log.Printf("thumbnails_start_failed user=%s job=%s err=%v", claims.UserID, route.jobID, err)
		if aerr := store.AbortQueued(ctx, claims.UserID, route.jobID, thumbnail.CodePipelineError, "We couldn't start thumbnail generation. Please try again."); aerr != nil {
			log.Printf("thumbnails_abort_failed user=%s job=%s err=%v", claims.UserID, route.jobID, aerr)
		}
		return errorResponse(http.StatusInternalServerError, "PIPELINE_ERROR", "Failed to start thumbnail generation"), nil
	}
	return jsonResponse(http.StatusAccepted, map[string]string{"status": string(thumbnail.StatusQueued)}), nil
}

func (h *APIHandler) handleSelectThumbnail(ctx context.Context, req events.APIGatewayProxyRequest, route thumbnailRoute, claims *models.JWTClaims) (events.APIGatewayProxyResponse, error) {
	var body struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal([]byte(req.Body), &body); err != nil || !thumbIDPattern.MatchString(body.ID) {
		return errorResponse(http.StatusBadRequest, "VALIDATION_ERROR", "A thumbnail id like thumb-01 is required"), nil
	}
	job, err := h.getShortsJobByID(ctx, claims.UserID, route.jobID)
	if err != nil {
		return errorResponse(http.StatusNotFound, "NOT_FOUND", "Shorts job not found"), nil
	}
	store := h.thumbnailStore()
	set, err := store.Get(ctx, claims.UserID, route.jobID)
	if err != nil {
		return errorResponse(http.StatusInternalServerError, "DB_ERROR", "Failed to load thumbnails"), nil
	}
	if set == nil || set.Status != thumbnail.StatusCompleted {
		return errorResponse(http.StatusConflict, "NOT_READY", "Thumbnails are not ready yet."), nil
	}
	if set.Find(body.ID) == nil {
		return errorResponse(http.StatusNotFound, "NOT_FOUND", "Thumbnail not found"), nil
	}
	if err := store.Select(ctx, claims.UserID, route.jobID, body.ID); err != nil {
		if errors.Is(err, thumbnail.ErrNotReady) {
			return errorResponse(http.StatusConflict, "NOT_READY", "Thumbnails are not ready yet."), nil
		}
		log.Printf("thumbnails_select_failed user=%s job=%s err=%v", claims.UserID, route.jobID, err)
		return errorResponse(http.StatusInternalServerError, "DB_ERROR", "Failed to save your choice"), nil
	}
	set.SelectedID = body.ID
	return jsonResponse(http.StatusOK, h.buildThumbnailsResponse(ctx, job, set)), nil
}

func (h *APIHandler) handleDownloadThumbnail(ctx context.Context, route thumbnailRoute, claims *models.JWTClaims) (events.APIGatewayProxyResponse, error) {
	if _, err := h.getShortsJobByID(ctx, claims.UserID, route.jobID); err != nil {
		return errorResponse(http.StatusNotFound, "NOT_FOUND", "Shorts job not found"), nil
	}
	set, err := h.thumbnailStore().Get(ctx, claims.UserID, route.jobID)
	if err != nil {
		return errorResponse(http.StatusInternalServerError, "DB_ERROR", "Failed to load thumbnails"), nil
	}
	asset := set.Find(route.thumbID)
	if asset == nil {
		return errorResponse(http.StatusNotFound, "NOT_FOUND", "Thumbnail not found"), nil
	}
	name := fmt.Sprintf("thumbnail-%s.jpg", strings.TrimPrefix(route.thumbID, "thumb-"))
	url, err := h.S3.GenerateAttachmentURL(ctx, h.Bucket, asset.Key, name, thumbnailURLExpiry)
	if err != nil {
		log.Printf("thumbnail_download_url_failed user=%s job=%s thumb=%s err=%v", claims.UserID, route.jobID, route.thumbID, err)
		return errorResponse(http.StatusInternalServerError, "S3_ERROR", "Failed to prepare the download"), nil
	}
	return jsonResponse(http.StatusOK, map[string]string{"url": url, "filename": name}), nil
}

// removeThumbnails drops a deleted job's thumbnail record. The images live
// under the job's S3 prefix and go with the rest of its assets.
func (h *APIHandler) removeThumbnails(ctx context.Context, userID, jobID string) {
	if err := h.thumbnailStore().Delete(ctx, userID, jobID); err != nil {
		log.Printf("thumbnails_delete_failed user=%s job=%s err=%v", userID, jobID, err)
	}
}
