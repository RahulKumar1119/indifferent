// Package api provides the API Gateway Lambda handler for the TXT-to-Video SaaS.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/sfn"
	"github.com/google/uuid"
	"github.com/rahul/indifferent/backend/internal/auth"
	"github.com/rahul/indifferent/backend/internal/models"
	"github.com/rahul/indifferent/backend/internal/storage"
)

// Valid templates for project creation.
var validTemplates = map[string]bool{
	"classic":   true,
	"modern":    true,
	"education": true,
	"dark":      true,
	"minimal":   true,
	"neon":      true,
}

// Valid voices for project creation (standard engine compatible).
var validVoices = map[string]bool{
	"Joanna":  true,
	"Matthew": true,
	"Amy":     true,
	"Brian":   true,
	"Aditi":   true,
}

// DynamoDBAPI defines the DynamoDB operations needed by the API handler.
type DynamoDBAPI interface {
	PutItem(ctx context.Context, params *dynamodb.PutItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error)
	GetItem(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error)
	DeleteItem(ctx context.Context, params *dynamodb.DeleteItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error)
	Query(ctx context.Context, params *dynamodb.QueryInput, optFns ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error)
}

// SFNClient defines the Step Functions operations needed by the API handler.
type SFNClient interface {
	StartExecution(ctx context.Context, params *sfn.StartExecutionInput, optFns ...func(*sfn.Options)) (*sfn.StartExecutionOutput, error)
}

// APIHandler handles all API Gateway proxy requests.
type APIHandler struct {
	AuthService     auth.GoogleAuthService
	JWTService      *auth.JWTService
	DB              DynamoDBAPI
	S3              *storage.S3Client
	SFN             SFNClient
	TableName       string
	Bucket          string
	StateMachineARN string
}

// corsHeaders returns the standard CORS headers for all responses.
func corsHeaders() map[string]string {
	return map[string]string{
		"Content-Type":                     "application/json",
		"Access-Control-Allow-Origin":      "https://indifferent.fun",
		"Access-Control-Allow-Headers":     "Content-Type,Authorization",
		"Access-Control-Allow-Methods":     "GET,POST,DELETE,OPTIONS",
		"Access-Control-Allow-Credentials": "true",
	}
}

// HandleRequest is the main router that dispatches requests to endpoint handlers.
func (h *APIHandler) HandleRequest(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// Handle CORS preflight
	if req.HTTPMethod == "OPTIONS" {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusOK,
			Headers:    corsHeaders(),
		}, nil
	}

	switch {
	case req.HTTPMethod == "POST" && req.Path == "/auth/google/callback":
		return h.handleGoogleCallback(ctx, req)
	case req.HTTPMethod == "POST" && req.Path == "/auth/refresh":
		return h.handleRefresh(ctx, req)
	case req.HTTPMethod == "POST" && req.Path == "/auth/logout":
		return h.handleLogout(ctx, req)
	case req.HTTPMethod == "GET" && req.Path == "/projects":
		return h.handleWithAuth(ctx, req, h.handleListProjects)
	case req.HTTPMethod == "POST" && req.Path == "/projects":
		return h.handleWithAuth(ctx, req, h.handleCreateProject)
	case req.HTTPMethod == "GET" && strings.HasPrefix(req.Path, "/projects/") && strings.HasSuffix(req.Path, "/status"):
		return h.handleWithAuth(ctx, req, h.handleGetProjectStatus)
	case req.HTTPMethod == "GET" && strings.HasPrefix(req.Path, "/projects/") && strings.HasSuffix(req.Path, "/download"):
		return h.handleWithAuth(ctx, req, h.handleGetDownloadURL)
	case req.HTTPMethod == "POST" && strings.HasPrefix(req.Path, "/projects/") && strings.HasSuffix(req.Path, "/start"):
		return h.handleWithAuth(ctx, req, h.handleStartPipeline)
	case req.HTTPMethod == "POST" && strings.HasPrefix(req.Path, "/projects/") && strings.HasSuffix(req.Path, "/upload"):
		return h.handleWithAuth(ctx, req, h.handleUpload)
	case req.HTTPMethod == "GET" && strings.HasPrefix(req.Path, "/projects/"):
		return h.handleWithAuth(ctx, req, h.handleGetProject)
	case req.HTTPMethod == "DELETE" && strings.HasPrefix(req.Path, "/projects/"):
		return h.handleWithAuth(ctx, req, h.handleDeleteProject)
	default:
		return errorResponse(http.StatusNotFound, "NOT_FOUND", "Route not found"), nil
	}
}

// authenticatedHandler is a handler that receives the authenticated user's claims.
type authenticatedHandler func(ctx context.Context, req events.APIGatewayProxyRequest, claims *models.JWTClaims) (events.APIGatewayProxyResponse, error)

// handleWithAuth extracts and validates the JWT token, then calls the handler with claims.
func (h *APIHandler) handleWithAuth(ctx context.Context, req events.APIGatewayProxyRequest, handler authenticatedHandler) (events.APIGatewayProxyResponse, error) {
	claims, err := h.extractAndValidateToken(req)
	if err != nil {
		return errorResponse(http.StatusUnauthorized, "UNAUTHORIZED", "Invalid or missing authentication token"), nil
	}
	return handler(ctx, req, claims)
}

// extractAndValidateToken extracts the Bearer token from the Authorization header and validates it.
func (h *APIHandler) extractAndValidateToken(req events.APIGatewayProxyRequest) (*models.JWTClaims, error) {
	authHeader := req.Headers["Authorization"]
	if authHeader == "" {
		authHeader = req.Headers["authorization"]
	}
	if authHeader == "" {
		return nil, fmt.Errorf("missing authorization header")
	}

	if !strings.HasPrefix(authHeader, "Bearer ") {
		return nil, fmt.Errorf("invalid authorization format")
	}

	token := strings.TrimPrefix(authHeader, "Bearer ")
	return h.JWTService.ValidateToken(token)
}

// handleGoogleCallback exchanges a Google auth code for tokens.
func (h *APIHandler) handleGoogleCallback(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	var body struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal([]byte(req.Body), &body); err != nil {
		return errorResponse(http.StatusBadRequest, "INVALID_BODY", "Invalid request body"), nil
	}

	if strings.TrimSpace(body.Code) == "" {
		return errorResponse(http.StatusBadRequest, "INVALID_INPUT", "Authorization code is required"), nil
	}

	tokens, err := h.AuthService.Authenticate(ctx, body.Code)
	if err != nil {
		return errorResponse(http.StatusUnauthorized, "AUTH_FAILED", "Authentication failed"), nil
	}

	return jsonResponse(http.StatusOK, tokens), nil
}

// handleRefresh refreshes an access token using a refresh token.
func (h *APIHandler) handleRefresh(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	var body struct {
		RefreshToken string `json:"refreshToken"`
	}
	if err := json.Unmarshal([]byte(req.Body), &body); err != nil {
		return errorResponse(http.StatusBadRequest, "INVALID_BODY", "Invalid request body"), nil
	}

	if strings.TrimSpace(body.RefreshToken) == "" {
		return errorResponse(http.StatusBadRequest, "INVALID_INPUT", "Refresh token is required"), nil
	}

	tokens, err := h.JWTService.RefreshToken(ctx, body.RefreshToken)
	if err != nil {
		return errorResponse(http.StatusUnauthorized, "REFRESH_FAILED", "Token refresh failed"), nil
	}

	return jsonResponse(http.StatusOK, tokens), nil
}

// handleLogout invalidates a session.
func (h *APIHandler) handleLogout(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	var body struct {
		RefreshToken string `json:"refreshToken"`
	}
	if err := json.Unmarshal([]byte(req.Body), &body); err != nil {
		return errorResponse(http.StatusBadRequest, "INVALID_BODY", "Invalid request body"), nil
	}

	if strings.TrimSpace(body.RefreshToken) == "" {
		return errorResponse(http.StatusBadRequest, "INVALID_INPUT", "Refresh token is required"), nil
	}

	if err := h.JWTService.Logout(ctx, body.RefreshToken); err != nil {
		return errorResponse(http.StatusInternalServerError, "LOGOUT_FAILED", "Logout failed"), nil
	}

	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusNoContent,
		Headers:    corsHeaders(),
	}, nil
}

// handleListProjects returns all projects belonging to the authenticated user.
func (h *APIHandler) handleListProjects(ctx context.Context, req events.APIGatewayProxyRequest, claims *models.JWTClaims) (events.APIGatewayProxyResponse, error) {
	result, err := h.DB.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(h.TableName),
		KeyConditionExpression: aws.String("PK = :pk AND begins_with(SK, :sk)"),
		ExpressionAttributeValues: map[string]dbtypes.AttributeValue{
			":pk": &dbtypes.AttributeValueMemberS{Value: "USER#" + claims.UserID},
			":sk": &dbtypes.AttributeValueMemberS{Value: "PROJECT#"},
		},
	})
	if err != nil {
		return errorResponse(http.StatusInternalServerError, "DB_ERROR", "Failed to list projects"), nil
	}

	projects := make([]models.Project, 0, len(result.Items))
	for _, item := range result.Items {
		projects = append(projects, itemToProject(item))
	}

	return jsonResponse(http.StatusOK, map[string]interface{}{
		"projects": projects,
	}), nil
}

// CreateProjectRequest represents the request body for creating a project.
type CreateProjectRequest struct {
	Name     string `json:"name"`
	Template string `json:"template"`
	Voice    string `json:"voice"`
}

// handleCreateProject creates a new project. The pipeline is started later via POST /projects/:id/start
// after the user uploads their TXT file.
func (h *APIHandler) handleCreateProject(ctx context.Context, req events.APIGatewayProxyRequest, claims *models.JWTClaims) (events.APIGatewayProxyResponse, error) {
	var body CreateProjectRequest
	if err := json.Unmarshal([]byte(req.Body), &body); err != nil {
		return errorResponse(http.StatusBadRequest, "INVALID_BODY", "Invalid request body"), nil
	}

	// Validate and sanitize inputs
	if err := validateCreateProject(&body); err != nil {
		return errorResponse(http.StatusBadRequest, "VALIDATION_ERROR", err.Error()), nil
	}

	projectID := uuid.New().String()
	now := time.Now().UTC().Format(time.RFC3339)

	project := models.Project{
		UserID:    claims.UserID,
		ProjectID: projectID,
		Name:      body.Name,
		Template:  body.Template,
		Voice:     body.Voice,
		Status:    "created",
		CreatedAt: now,
		UpdatedAt: now,
	}

	// Store project in DynamoDB
	item := projectToItem(project)
	_, err := h.DB.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(h.TableName),
		Item:      item,
	})
	if err != nil {
		return errorResponse(http.StatusInternalServerError, "DB_ERROR", "Failed to create project"), nil
	}

	return jsonResponse(http.StatusCreated, project), nil
}

// handleGetProject returns a specific project, verifying ownership.
func (h *APIHandler) handleGetProject(ctx context.Context, req events.APIGatewayProxyRequest, claims *models.JWTClaims) (events.APIGatewayProxyResponse, error) {
	projectID := extractProjectID(req.Path)
	if projectID == "" {
		return errorResponse(http.StatusBadRequest, "INVALID_INPUT", "Project ID is required"), nil
	}

	project, err := h.getProjectByID(ctx, claims.UserID, projectID)
	if err != nil {
		return errorResponse(http.StatusNotFound, "NOT_FOUND", "Project not found"), nil
	}

	return jsonResponse(http.StatusOK, project), nil
}

// handleDeleteProject deletes a project, verifying ownership.
func (h *APIHandler) handleDeleteProject(ctx context.Context, req events.APIGatewayProxyRequest, claims *models.JWTClaims) (events.APIGatewayProxyResponse, error) {
	projectID := extractProjectID(req.Path)
	if projectID == "" {
		return errorResponse(http.StatusBadRequest, "INVALID_INPUT", "Project ID is required"), nil
	}

	// Verify ownership by trying to get the project first
	_, err := h.getProjectByID(ctx, claims.UserID, projectID)
	if err != nil {
		return errorResponse(http.StatusNotFound, "NOT_FOUND", "Project not found"), nil
	}

	_, err = h.DB.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(h.TableName),
		Key: map[string]dbtypes.AttributeValue{
			"PK": &dbtypes.AttributeValueMemberS{Value: "USER#" + claims.UserID},
			"SK": &dbtypes.AttributeValueMemberS{Value: "PROJECT#" + projectID},
		},
	})
	if err != nil {
		return errorResponse(http.StatusInternalServerError, "DB_ERROR", "Failed to delete project"), nil
	}

	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusNoContent,
		Headers:    corsHeaders(),
	}, nil
}

// handleUpload generates a signed upload URL for a TXT file.
func (h *APIHandler) handleUpload(ctx context.Context, req events.APIGatewayProxyRequest, claims *models.JWTClaims) (events.APIGatewayProxyResponse, error) {
	projectID := extractProjectIDFromSubpath(req.Path, "/upload")
	if projectID == "" {
		return errorResponse(http.StatusBadRequest, "INVALID_INPUT", "Project ID is required"), nil
	}

	// Verify ownership
	_, err := h.getProjectByID(ctx, claims.UserID, projectID)
	if err != nil {
		return errorResponse(http.StatusNotFound, "NOT_FOUND", "Project not found"), nil
	}

	s3Key := fmt.Sprintf("uploads/%s/%s/input.txt", claims.UserID, projectID)
	uploadURL, err := h.S3.GenerateUploadURL(ctx, h.Bucket, s3Key, "text/plain", storage.UploadURLExpiration)
	if err != nil {
		return errorResponse(http.StatusInternalServerError, "S3_ERROR", "Failed to generate upload URL"), nil
	}

	return jsonResponse(http.StatusOK, map[string]string{
		"uploadUrl": uploadURL,
	}), nil
}

// handleStartPipeline starts the Step Functions pipeline for a project after the TXT file has been uploaded.
func (h *APIHandler) handleStartPipeline(ctx context.Context, req events.APIGatewayProxyRequest, claims *models.JWTClaims) (events.APIGatewayProxyResponse, error) {
	projectID := extractProjectIDFromSubpath(req.Path, "/start")
	if projectID == "" {
		return errorResponse(http.StatusBadRequest, "INVALID_INPUT", "Project ID is required"), nil
	}

	// Verify ownership
	project, err := h.getProjectByID(ctx, claims.UserID, projectID)
	if err != nil {
		return errorResponse(http.StatusNotFound, "NOT_FOUND", "Project not found"), nil
	}

	// Only start pipeline if project is in "created" status
	if project.Status != "created" {
		return errorResponse(http.StatusBadRequest, "INVALID_STATE", "Pipeline has already been started"), nil
	}

	// The TXT file key follows the convention: uploads/{userId}/{projectId}/input.txt
	txtKey := fmt.Sprintf("uploads/%s/%s/input.txt", claims.UserID, projectID)

	// Start Step Functions execution with the s3Key
	sfnInput, _ := json.Marshal(map[string]string{
		"projectId": projectID,
		"userId":    claims.UserID,
		"template":  project.Template,
		"voice":     project.Voice,
		"s3Key":     txtKey,
	})

	_, err = h.SFN.StartExecution(ctx, &sfn.StartExecutionInput{
		StateMachineArn: aws.String(h.StateMachineARN),
		Name:            aws.String(fmt.Sprintf("project-%s", projectID)),
		Input:           aws.String(string(sfnInput)),
	})
	if err != nil {
		return errorResponse(http.StatusInternalServerError, "PIPELINE_ERROR", "Failed to start processing pipeline"), nil
	}

	return jsonResponse(http.StatusOK, map[string]string{
		"status": "started",
	}), nil
}

// handleGetProjectStatus returns the current pipeline status and progress.
func (h *APIHandler) handleGetProjectStatus(ctx context.Context, req events.APIGatewayProxyRequest, claims *models.JWTClaims) (events.APIGatewayProxyResponse, error) {
	projectID := extractProjectIDFromSubpath(req.Path, "/status")
	if projectID == "" {
		return errorResponse(http.StatusBadRequest, "INVALID_INPUT", "Project ID is required"), nil
	}

	project, err := h.getProjectByID(ctx, claims.UserID, projectID)
	if err != nil {
		return errorResponse(http.StatusNotFound, "NOT_FOUND", "Project not found"), nil
	}

	progress := buildPipelineProgress(project)
	return jsonResponse(http.StatusOK, progress), nil
}

// handleGetDownloadURL generates a signed download URL for the completed video.
func (h *APIHandler) handleGetDownloadURL(ctx context.Context, req events.APIGatewayProxyRequest, claims *models.JWTClaims) (events.APIGatewayProxyResponse, error) {
	projectID := extractProjectIDFromSubpath(req.Path, "/download")
	if projectID == "" {
		return errorResponse(http.StatusBadRequest, "INVALID_INPUT", "Project ID is required"), nil
	}

	project, err := h.getProjectByID(ctx, claims.UserID, projectID)
	if err != nil {
		return errorResponse(http.StatusNotFound, "NOT_FOUND", "Project not found"), nil
	}

	if project.Status != "completed" {
		return errorResponse(http.StatusBadRequest, "NOT_READY", "Video is not yet completed"), nil
	}

	if project.VideoKey == "" {
		return errorResponse(http.StatusNotFound, "NOT_FOUND", "Video file not found"), nil
	}

	downloadURL, err := h.S3.GenerateDownloadURL(ctx, h.Bucket, project.VideoKey, storage.DownloadURLExpiration)
	if err != nil {
		return errorResponse(http.StatusInternalServerError, "S3_ERROR", "Failed to generate download URL"), nil
	}

	return jsonResponse(http.StatusOK, map[string]string{
		"downloadUrl": downloadURL,
	}), nil
}

// getProjectByID retrieves a project from DynamoDB and verifies ownership.
func (h *APIHandler) getProjectByID(ctx context.Context, userID, projectID string) (*models.Project, error) {
	result, err := h.DB.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(h.TableName),
		Key: map[string]dbtypes.AttributeValue{
			"PK": &dbtypes.AttributeValueMemberS{Value: "USER#" + userID},
			"SK": &dbtypes.AttributeValueMemberS{Value: "PROJECT#" + projectID},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("get item: %w", err)
	}
	if result.Item == nil {
		return nil, fmt.Errorf("project not found")
	}

	project := itemToProject(result.Item)
	return &project, nil
}

// validateCreateProject validates and sanitizes the create project request.
func validateCreateProject(req *CreateProjectRequest) error {
	// Sanitize name: strip HTML tags
	req.Name = sanitizeString(req.Name)

	if strings.TrimSpace(req.Name) == "" {
		return fmt.Errorf("project name is required")
	}
	if len(req.Name) > 100 {
		return fmt.Errorf("project name must be 100 characters or less")
	}

	if !validTemplates[req.Template] {
		return fmt.Errorf("invalid template: must be one of classic, modern, education, dark, minimal, neon")
	}

	if !validVoices[req.Voice] {
		return fmt.Errorf("invalid voice: must be one of Joanna, Matthew, Amy, Brian, Aditi")
	}

	return nil
}

// sanitizeString strips HTML tags and escapes special characters.
func sanitizeString(s string) string {
	// Remove HTML tags
	s = stripHTMLTags(s)
	// Unescape any HTML entities that resulted from stripping
	s = html.UnescapeString(s)
	// Trim whitespace
	s = strings.TrimSpace(s)
	return s
}

// stripHTMLTags removes all HTML tags from a string.
func stripHTMLTags(s string) string {
	var result strings.Builder
	inTag := false
	for _, r := range s {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			continue
		}
		if !inTag {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// extractProjectID extracts the project ID from a path like /projects/{id}.
func extractProjectID(path string) string {
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(parts) >= 2 && parts[0] == "projects" {
		return parts[1]
	}
	return ""
}

// extractProjectIDFromSubpath extracts the project ID from a path like /projects/{id}/subpath.
func extractProjectIDFromSubpath(path, suffix string) string {
	path = strings.TrimSuffix(path, suffix)
	return extractProjectID(path)
}

// PipelineProgress represents the current progress of a project's pipeline.
type PipelineProgress struct {
	Stage           string `json:"stage"`
	Percentage      int    `json:"percentage"`
	SlidesProcessed *int   `json:"slidesProcessed,omitempty"`
	SlidesTotal     *int   `json:"slidesTotal,omitempty"`
}

// buildPipelineProgress constructs a progress object from the project state.
func buildPipelineProgress(project *models.Project) PipelineProgress {
	progress := PipelineProgress{
		Stage: project.Status,
	}

	switch project.Status {
	case "created":
		progress.Percentage = 0
	case "parsing":
		progress.Percentage = 5
	case "generating_slides":
		progress.Percentage = 25
	case "narrating":
		progress.Percentage = 50
	case "rendering":
		progress.Percentage = 75
	case "completed":
		progress.Percentage = 100
	case "failed":
		progress.Percentage = 0
	}

	return progress
}

// projectToItem converts a Project model to a DynamoDB item.
func projectToItem(p models.Project) map[string]dbtypes.AttributeValue {
	item := map[string]dbtypes.AttributeValue{
		"PK":        &dbtypes.AttributeValueMemberS{Value: "USER#" + p.UserID},
		"SK":        &dbtypes.AttributeValueMemberS{Value: "PROJECT#" + p.ProjectID},
		"projectId": &dbtypes.AttributeValueMemberS{Value: p.ProjectID},
		"name":      &dbtypes.AttributeValueMemberS{Value: p.Name},
		"template":  &dbtypes.AttributeValueMemberS{Value: p.Template},
		"voice":     &dbtypes.AttributeValueMemberS{Value: p.Voice},
		"status":    &dbtypes.AttributeValueMemberS{Value: p.Status},
		"createdAt": &dbtypes.AttributeValueMemberS{Value: p.CreatedAt},
		"updatedAt": &dbtypes.AttributeValueMemberS{Value: p.UpdatedAt},
	}
	if p.TxtKey != "" {
		item["txtKey"] = &dbtypes.AttributeValueMemberS{Value: p.TxtKey}
	}
	if p.JSONKey != "" {
		item["jsonKey"] = &dbtypes.AttributeValueMemberS{Value: p.JSONKey}
	}
	if p.VideoKey != "" {
		item["videoKey"] = &dbtypes.AttributeValueMemberS{Value: p.VideoKey}
	}
	if p.ThumbnailKey != "" {
		item["thumbnailKey"] = &dbtypes.AttributeValueMemberS{Value: p.ThumbnailKey}
	}
	if p.CompletedAt != "" {
		item["completedAt"] = &dbtypes.AttributeValueMemberS{Value: p.CompletedAt}
	}
	if p.Error != "" {
		item["error"] = &dbtypes.AttributeValueMemberS{Value: p.Error}
	}
	return item
}

// itemToProject converts a DynamoDB item to a Project model.
func itemToProject(item map[string]dbtypes.AttributeValue) models.Project {
	p := models.Project{}
	if v, ok := item["projectId"].(*dbtypes.AttributeValueMemberS); ok {
		p.ProjectID = v.Value
	}
	if v, ok := item["PK"].(*dbtypes.AttributeValueMemberS); ok && len(v.Value) > 5 {
		p.UserID = v.Value[5:] // Strip "USER#" prefix
	}
	if v, ok := item["name"].(*dbtypes.AttributeValueMemberS); ok {
		p.Name = v.Value
	}
	if v, ok := item["template"].(*dbtypes.AttributeValueMemberS); ok {
		p.Template = v.Value
	}
	if v, ok := item["voice"].(*dbtypes.AttributeValueMemberS); ok {
		p.Voice = v.Value
	}
	if v, ok := item["status"].(*dbtypes.AttributeValueMemberS); ok {
		p.Status = v.Value
	}
	if v, ok := item["txtKey"].(*dbtypes.AttributeValueMemberS); ok {
		p.TxtKey = v.Value
	}
	if v, ok := item["jsonKey"].(*dbtypes.AttributeValueMemberS); ok {
		p.JSONKey = v.Value
	}
	if v, ok := item["videoKey"].(*dbtypes.AttributeValueMemberS); ok {
		p.VideoKey = v.Value
	}
	if v, ok := item["thumbnailKey"].(*dbtypes.AttributeValueMemberS); ok {
		p.ThumbnailKey = v.Value
	}
	if v, ok := item["error"].(*dbtypes.AttributeValueMemberS); ok {
		p.Error = v.Value
	}
	if v, ok := item["createdAt"].(*dbtypes.AttributeValueMemberS); ok {
		p.CreatedAt = v.Value
	}
	if v, ok := item["updatedAt"].(*dbtypes.AttributeValueMemberS); ok {
		p.UpdatedAt = v.Value
	}
	if v, ok := item["completedAt"].(*dbtypes.AttributeValueMemberS); ok {
		p.CompletedAt = v.Value
	}
	return p
}

// jsonResponse creates a JSON success response.
func jsonResponse(statusCode int, body interface{}) events.APIGatewayProxyResponse {
	data, _ := json.Marshal(body)
	return events.APIGatewayProxyResponse{
		StatusCode: statusCode,
		Headers:    corsHeaders(),
		Body:       string(data),
	}
}

// errorResponse creates a JSON error response.
func errorResponse(statusCode int, code, message string) events.APIGatewayProxyResponse {
	apiErr := models.APIError{
		Code:    code,
		Message: message,
	}
	data, _ := json.Marshal(apiErr)
	return events.APIGatewayProxyResponse{
		StatusCode: statusCode,
		Headers:    corsHeaders(),
		Body:       string(data),
	}
}
