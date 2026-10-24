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

	"github.com/aws/aws-lambda-go/events"
	"github.com/rahul/indifferent/backend/internal/models"
	"github.com/rahul/indifferent/backend/internal/shorts"
	"github.com/rahul/indifferent/backend/internal/storage"
)

// Multipart upload of a Shorts source (issue #151). Sources above one part
// size go to S3 in parallel parts that can be retried and resumed one by one.
// The object key always comes from the stored job, never from the request, so
// an upload ID can only ever act on the caller's own source object.
//
//	POST   /shorts/{id}/upload/multipart/initiate  {size}                          -> {uploadId, partSize, partCount}
//	POST   /shorts/{id}/upload/multipart/parts     {uploadId, partNumbers, accelerate?} -> {urls, expiresInSeconds, accelerated}
//	POST   /shorts/{id}/upload/multipart/complete  {uploadId, parts:[{partNumber, etag}]}
//	GET    /shorts/{id}/upload/multipart?uploadId= parts S3 holds for the upload (resume)
//	DELETE /shorts/{id}/upload/multipart?uploadId= abort and discard the parts

var (
	uploadIDPattern = regexp.MustCompile(`^[A-Za-z0-9._~+/=-]{1,512}$`)
	etagPattern     = regexp.MustCompile(`^(?:"[0-9A-Za-z+/=_-]{1,128}"|[0-9A-Za-z+/=_-]{1,128})$`)
)

// multipartRoute is a parsed /shorts/{id}/upload/multipart... path.
type multipartRoute struct {
	jobID  string
	action string // "" (list and abort), "initiate", "parts" or "complete"
}

func parseMultipartPath(path string) (multipartRoute, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 4 || len(parts) > 5 || parts[0] != "shorts" || parts[1] == "" || parts[2] != "upload" || parts[3] != "multipart" {
		return multipartRoute{}, false
	}
	r := multipartRoute{jobID: parts[1]}
	if len(parts) == 5 {
		switch parts[4] {
		case "initiate", "parts", "complete":
			r.action = parts[4]
		default:
			return multipartRoute{}, false
		}
	}
	return r, true
}

func isMultipartPath(path string) bool {
	_, ok := parseMultipartPath(path)
	return ok
}

// handleMultipart routes the multipart upload endpoints.
func (h *APIHandler) handleMultipart(ctx context.Context, req events.APIGatewayProxyRequest, claims *models.JWTClaims) (events.APIGatewayProxyResponse, error) {
	route, _ := parseMultipartPath(req.Path)
	switch {
	case req.HTTPMethod == http.MethodPost && route.action == "initiate":
		return h.handleMultipartInitiate(ctx, req, route, claims)
	case req.HTTPMethod == http.MethodPost && route.action == "parts":
		return h.handleMultipartParts(ctx, req, route, claims)
	case req.HTTPMethod == http.MethodPost && route.action == "complete":
		return h.handleMultipartComplete(ctx, req, route, claims)
	case req.HTTPMethod == http.MethodGet && route.action == "":
		return h.handleMultipartList(ctx, req, route, claims)
	case req.HTTPMethod == http.MethodDelete && route.action == "":
		return h.handleMultipartAbort(ctx, req, route, claims)
	}
	return errorResponse(http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed"), nil
}

// multipartJob loads the caller's job. With waiting set, the job must still be
// waiting for its upload: the pipeline reads the source, so it must not change
// once processing has started. A failure comes back as a ready response.
func (h *APIHandler) multipartJob(ctx context.Context, claims *models.JWTClaims, jobID string, waiting bool) (*models.ShortsJob, *events.APIGatewayProxyResponse) {
	job, err := h.getShortsJobByID(ctx, claims.UserID, jobID)
	if err != nil {
		resp := errorResponse(http.StatusNotFound, "NOT_FOUND", "Shorts job not found")
		return nil, &resp
	}
	if waiting && job.Status != "uploaded" {
		resp := errorResponse(http.StatusBadRequest, "INVALID_STATE", "This job is no longer waiting for an upload.")
		return nil, &resp
	}
	return job, nil
}

// validUploadID returns the upload ID when it has the shape S3 issues.
func validUploadID(id string) (string, bool) {
	id = strings.TrimSpace(id)
	return id, uploadIDPattern.MatchString(id)
}

func normalizeETag(etag string) string {
	return strings.Trim(etag, `"`)
}

func tooLargeProblem(size int64) *uploadProblem {
	return &uploadProblem{
		code: "UPLOAD_TOO_LARGE",
		message: fmt.Sprintf("Uploaded file is %.0f MB; the maximum is %d MB. Pick a smaller file, or compress the source first.",
			float64(size)/1024/1024, shorts.MaxUploadBytes/1024/1024),
	}
}

func sessionGoneResponse() events.APIGatewayProxyResponse {
	return errorResponse(http.StatusNotFound, "UPLOAD_SESSION_NOT_FOUND", "This upload session no longer exists. Start the upload again.")
}

// MultipartInitiateRequest is the body of .../multipart/initiate.
type MultipartInitiateRequest struct {
	Size int64 `json:"size"` // source size in bytes
}

// MultipartInitiateResponse tells the client how to slice the file.
type MultipartInitiateResponse struct {
	UploadID  string `json:"uploadId"`
	PartSize  int64  `json:"partSize"`
	PartCount int    `json:"partCount"`
}

// handleMultipartInitiate opens a multipart upload for a job that is waiting
// for its source.
func (h *APIHandler) handleMultipartInitiate(ctx context.Context, req events.APIGatewayProxyRequest, route multipartRoute, claims *models.JWTClaims) (events.APIGatewayProxyResponse, error) {
	var body MultipartInitiateRequest
	if err := json.Unmarshal([]byte(req.Body), &body); err != nil {
		return errorResponse(http.StatusBadRequest, "INVALID_BODY", "Invalid request body"), nil
	}
	if body.Size <= 0 {
		return errorResponse(http.StatusBadRequest, "VALIDATION_ERROR", "size must be greater than zero"), nil
	}
	if body.Size > shorts.MaxUploadBytes {
		p := tooLargeProblem(body.Size)
		return errorResponse(http.StatusBadRequest, p.code, p.message), nil
	}
	partCount := shorts.MultipartPartCount(body.Size)
	if partCount > shorts.MaxMultipartParts {
		return errorResponse(http.StatusBadRequest, "VALIDATION_ERROR", "size needs more parts than S3 allows"), nil
	}

	job, failed := h.multipartJob(ctx, claims, route.jobID, true)
	if failed != nil {
		return *failed, nil
	}

	// A landed source means the upload is already done; starting another would
	// overwrite what the pipeline is about to read.
	meta, err := h.S3.ObjectMeta(ctx, h.Bucket, job.SourceKey)
	if err != nil {
		return errorResponse(http.StatusInternalServerError, "S3_ERROR", "Failed to check the upload. Please try again."), nil
	}
	if meta.Found {
		return errorResponse(http.StatusConflict, "ALREADY_UPLOADED", "This job already has its file."), nil
	}

	contentType := validShortsFileTypes[strings.ToLower(strings.TrimSpace(job.FileType))]
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	uploadID, err := h.S3.CreateMultipartUpload(ctx, h.Bucket, job.SourceKey, contentType)
	if err != nil {
		log.Printf("shorts_multipart_create_failed user=%s job=%s err=%v", claims.UserID, job.JobID, err)
		return errorResponse(http.StatusInternalServerError, "S3_ERROR", "Failed to start the upload. Please try again."), nil
	}

	log.Printf("shorts_multipart_initiated user=%s job=%s size=%d parts=%d", claims.UserID, job.JobID, body.Size, partCount)
	return jsonResponse(http.StatusCreated, MultipartInitiateResponse{
		UploadID:  uploadID,
		PartSize:  shorts.MultipartPartSize,
		PartCount: partCount,
	}), nil
}

// MultipartPartsRequest asks for presigned URLs for a batch of parts.
type MultipartPartsRequest struct {
	UploadID    string `json:"uploadId"`
	PartNumbers []int  `json:"partNumbers"`
	// Accelerate false asks for regular URLs, for a network that blocks the
	// accelerate host. Omitted or true leaves the choice to the server.
	Accelerate *bool `json:"accelerate,omitempty"`
}

// MultipartPartURL is one presigned UploadPart URL.
type MultipartPartURL struct {
	PartNumber int    `json:"partNumber"`
	URL        string `json:"url"`
}

// MultipartPartsResponse carries the URLs and how long they last.
type MultipartPartsResponse struct {
	URLs             []MultipartPartURL `json:"urls"`
	ExpiresInSeconds int                `json:"expiresInSeconds"`
	// Accelerated is true when the URLs point at the Transfer Acceleration endpoint.
	Accelerated bool `json:"accelerated"`
}

// handleMultipartParts presigns a batch of parts. Presigning is local, so the
// upload ID is not checked here: S3 rejects an unknown one on the first PUT.
func (h *APIHandler) handleMultipartParts(ctx context.Context, req events.APIGatewayProxyRequest, route multipartRoute, claims *models.JWTClaims) (events.APIGatewayProxyResponse, error) {
	var body MultipartPartsRequest
	if err := json.Unmarshal([]byte(req.Body), &body); err != nil {
		return errorResponse(http.StatusBadRequest, "INVALID_BODY", "Invalid request body"), nil
	}
	uploadID, ok := validUploadID(body.UploadID)
	if !ok {
		return errorResponse(http.StatusBadRequest, "VALIDATION_ERROR", "uploadId is missing or malformed"), nil
	}
	if len(body.PartNumbers) == 0 || len(body.PartNumbers) > shorts.MaxPartURLBatch {
		return errorResponse(http.StatusBadRequest, "VALIDATION_ERROR",
			fmt.Sprintf("partNumbers must hold between 1 and %d entries", shorts.MaxPartURLBatch)), nil
	}
	seen := make(map[int]bool, len(body.PartNumbers))
	for _, n := range body.PartNumbers {
		if n < 1 || n > shorts.MaxMultipartParts {
			return errorResponse(http.StatusBadRequest, "VALIDATION_ERROR",
				fmt.Sprintf("part numbers must be between 1 and %d", shorts.MaxMultipartParts)), nil
		}
		if seen[n] {
			return errorResponse(http.StatusBadRequest, "VALIDATION_ERROR", "part numbers must be unique"), nil
		}
		seen[n] = true
	}

	job, failed := h.multipartJob(ctx, claims, route.jobID, true)
	if failed != nil {
		return *failed, nil
	}

	accelerate := h.S3Accelerate && (body.Accelerate == nil || *body.Accelerate)
	urls := make([]MultipartPartURL, 0, len(body.PartNumbers))
	for _, n := range body.PartNumbers {
		url, err := h.S3.GenerateUploadPartURL(ctx, h.Bucket, job.SourceKey, uploadID, int32(n), storage.PartURLExpiration, accelerate)
		if err != nil {
			return errorResponse(http.StatusInternalServerError, "S3_ERROR", "Failed to prepare the upload. Please try again."), nil
		}
		urls = append(urls, MultipartPartURL{PartNumber: n, URL: url})
	}
	return jsonResponse(http.StatusOK, MultipartPartsResponse{
		URLs:             urls,
		ExpiresInSeconds: int(storage.PartURLExpiration.Seconds()),
		Accelerated:      accelerate,
	}), nil
}

// MultipartCompletePart is one stored part named by the client.
type MultipartCompletePart struct {
	PartNumber int    `json:"partNumber"`
	ETag       string `json:"etag"`
}

// MultipartCompleteRequest lists every part of the file, in order.
type MultipartCompleteRequest struct {
	UploadID string                  `json:"uploadId"`
	Parts    []MultipartCompletePart `json:"parts"`
}

// handleMultipartComplete assembles the parts into the source object, then
// runs the same checks the single PUT path gets at start.
func (h *APIHandler) handleMultipartComplete(ctx context.Context, req events.APIGatewayProxyRequest, route multipartRoute, claims *models.JWTClaims) (events.APIGatewayProxyResponse, error) {
	var body MultipartCompleteRequest
	if err := json.Unmarshal([]byte(req.Body), &body); err != nil {
		return errorResponse(http.StatusBadRequest, "INVALID_BODY", "Invalid request body"), nil
	}
	uploadID, ok := validUploadID(body.UploadID)
	if !ok {
		return errorResponse(http.StatusBadRequest, "VALIDATION_ERROR", "uploadId is missing or malformed"), nil
	}
	if len(body.Parts) == 0 || len(body.Parts) > shorts.MaxMultipartParts {
		return errorResponse(http.StatusBadRequest, "VALIDATION_ERROR",
			fmt.Sprintf("parts must hold between 1 and %d entries", shorts.MaxMultipartParts)), nil
	}
	// Numbered 1..N with no gap or repeat also proves the list is sorted.
	for i, p := range body.Parts {
		if p.PartNumber != i+1 {
			return errorResponse(http.StatusBadRequest, "VALIDATION_ERROR", "parts must be numbered 1 to N in order, with none missing"), nil
		}
		if !etagPattern.MatchString(p.ETag) {
			return errorResponse(http.StatusBadRequest, "VALIDATION_ERROR", fmt.Sprintf("part %d has a malformed etag", p.PartNumber)), nil
		}
	}

	job, failed := h.multipartJob(ctx, claims, route.jobID, true)
	if failed != nil {
		return *failed, nil
	}

	stored, err := h.S3.ListUploadedParts(ctx, h.Bucket, job.SourceKey, uploadID)
	if err != nil {
		if storage.IsNoSuchUpload(err) {
			return h.multipartAlreadyCompleted(ctx, job), nil
		}
		log.Printf("shorts_multipart_list_failed user=%s job=%s err=%v", claims.UserID, job.JobID, err)
		return errorResponse(http.StatusInternalServerError, "S3_ERROR", "Failed to check the uploaded parts. Please try again."), nil
	}
	sizes := make(map[int]storage.UploadedPart, len(stored))
	for _, p := range stored {
		sizes[int(p.PartNumber)] = p
	}
	var total int64
	done := make([]storage.CompletedPart, 0, len(body.Parts))
	for _, p := range body.Parts {
		have, found := sizes[p.PartNumber]
		if !found || normalizeETag(have.ETag) != normalizeETag(p.ETag) {
			return errorResponse(http.StatusBadRequest, "INVALID_PARTS",
				fmt.Sprintf("Part %d is missing or changed. Resume the upload to send it again.", p.PartNumber)), nil
		}
		total += have.Size
		done = append(done, storage.CompletedPart{PartNumber: int32(p.PartNumber), ETag: `"` + normalizeETag(p.ETag) + `"`})
	}

	// The size the client declared at initiate is only a claim; the parts S3
	// holds are the truth. Refuse before the object exists.
	if total > shorts.MaxUploadBytes {
		if err := h.S3.AbortMultipartUpload(ctx, h.Bucket, job.SourceKey, uploadID); err != nil && !storage.IsNoSuchUpload(err) {
			log.Printf("shorts_multipart_abort_failed user=%s job=%s err=%v", claims.UserID, job.JobID, err)
		}
		p := tooLargeProblem(total)
		return errorResponse(http.StatusBadRequest, p.code, p.message), nil
	}

	if err := h.S3.CompleteMultipartUpload(ctx, h.Bucket, job.SourceKey, uploadID, done); err != nil {
		switch {
		case storage.IsNoSuchUpload(err):
			return h.multipartAlreadyCompleted(ctx, job), nil
		case storage.IsInvalidParts(err):
			return errorResponse(http.StatusBadRequest, "INVALID_PARTS", "The uploaded parts do not add up to a file. Resume the upload to send the missing ones."), nil
		}
		log.Printf("shorts_multipart_complete_failed user=%s job=%s err=%v", claims.UserID, job.JobID, err)
		return errorResponse(http.StatusInternalServerError, "S3_ERROR", "Failed to finish the upload. Please try again."), nil
	}

	resp := h.multipartVerified(ctx, job)
	if resp.StatusCode == http.StatusOK {
		log.Printf("shorts_multipart_completed user=%s job=%s size=%d parts=%d", claims.UserID, job.JobID, total, len(done))
	}
	return resp, nil
}

// multipartVerified runs verifyShortsUpload on the source and answers like
// the start route would.
func (h *APIHandler) multipartVerified(ctx context.Context, job *models.ShortsJob) events.APIGatewayProxyResponse {
	var problem *uploadProblem
	if err := h.verifyShortsUpload(ctx, job); errors.As(err, &problem) {
		return errorResponse(http.StatusBadRequest, problem.code, problem.message)
	} else if err != nil {
		return errorResponse(http.StatusInternalServerError, "S3_ERROR", "Failed to verify upload. Please try again.")
	}
	return jsonResponse(http.StatusOK, map[string]string{"status": "completed"})
}

// multipartAlreadyCompleted answers a complete call whose upload ID is gone.
// A lost response, or a retry after a timeout, leaves the source in place with
// no open upload: that still counts as done, so the file is not sent again.
func (h *APIHandler) multipartAlreadyCompleted(ctx context.Context, job *models.ShortsJob) events.APIGatewayProxyResponse {
	resp := h.multipartVerified(ctx, job)
	if resp.StatusCode == http.StatusBadRequest {
		var apiErr models.APIError
		if json.Unmarshal([]byte(resp.Body), &apiErr) == nil && apiErr.Code == "UPLOAD_NOT_FOUND" {
			return sessionGoneResponse()
		}
	}
	return resp
}

// MultipartListResponse is the parts S3 holds for an open upload.
type MultipartListResponse struct {
	Parts []MultipartListedPart `json:"parts"`
}

// MultipartListedPart is one stored part.
type MultipartListedPart struct {
	PartNumber int    `json:"partNumber"`
	ETag       string `json:"etag"`
	Size       int64  `json:"size"`
}

// handleMultipartList reports what S3 has stored, so a resumed upload trusts
// S3 over whatever the browser remembers.
func (h *APIHandler) handleMultipartList(ctx context.Context, req events.APIGatewayProxyRequest, route multipartRoute, claims *models.JWTClaims) (events.APIGatewayProxyResponse, error) {
	uploadID, ok := validUploadID(req.QueryStringParameters["uploadId"])
	if !ok {
		return errorResponse(http.StatusBadRequest, "VALIDATION_ERROR", "uploadId is missing or malformed"), nil
	}
	job, failed := h.multipartJob(ctx, claims, route.jobID, true)
	if failed != nil {
		return *failed, nil
	}
	stored, err := h.S3.ListUploadedParts(ctx, h.Bucket, job.SourceKey, uploadID)
	if err != nil {
		if storage.IsNoSuchUpload(err) {
			return sessionGoneResponse(), nil
		}
		return errorResponse(http.StatusInternalServerError, "S3_ERROR", "Failed to check the uploaded parts. Please try again."), nil
	}
	out := MultipartListResponse{Parts: make([]MultipartListedPart, 0, len(stored))}
	for _, p := range stored {
		out.Parts = append(out.Parts, MultipartListedPart{PartNumber: int(p.PartNumber), ETag: p.ETag, Size: p.Size})
	}
	return jsonResponse(http.StatusOK, out), nil
}

// handleMultipartAbort discards an open upload. It works in any job state so
// cleanup is always possible, and an upload that is already gone counts as
// aborted.
func (h *APIHandler) handleMultipartAbort(ctx context.Context, req events.APIGatewayProxyRequest, route multipartRoute, claims *models.JWTClaims) (events.APIGatewayProxyResponse, error) {
	uploadID, ok := validUploadID(req.QueryStringParameters["uploadId"])
	if !ok {
		return errorResponse(http.StatusBadRequest, "VALIDATION_ERROR", "uploadId is missing or malformed"), nil
	}
	job, failed := h.multipartJob(ctx, claims, route.jobID, false)
	if failed != nil {
		return *failed, nil
	}
	if err := h.S3.AbortMultipartUpload(ctx, h.Bucket, job.SourceKey, uploadID); err != nil && !storage.IsNoSuchUpload(err) {
		log.Printf("shorts_multipart_abort_failed user=%s job=%s err=%v", claims.UserID, job.JobID, err)
		return errorResponse(http.StatusInternalServerError, "S3_ERROR", "Failed to cancel the upload. Please try again."), nil
	}
	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusNoContent,
		Headers:    corsHeaders(),
	}, nil
}
