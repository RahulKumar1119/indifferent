// Package api provides the API Gateway Lambda handler for the TXT-to-Video SaaS.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/sfn"
	sfntypes "github.com/aws/aws-sdk-go-v2/service/sfn/types"
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
	UpdateItem(ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error)
}

// SFNClient defines the Step Functions operations needed by the API handler.
type SFNClient interface {
	StartExecution(ctx context.Context, params *sfn.StartExecutionInput, optFns ...func(*sfn.Options)) (*sfn.StartExecutionOutput, error)
	StopExecution(ctx context.Context, params *sfn.StopExecutionInput, optFns ...func(*sfn.Options)) (*sfn.StopExecutionOutput, error)
}

// APIHandler handles all API Gateway proxy requests.
type APIHandler struct {
	AuthService           auth.GoogleAuthService
	JWTService            *auth.JWTService
	PasswordAuth          *auth.PasswordAuthService
	DB                    DynamoDBAPI
	S3                    storage.Downloader
	SFN                   SFNClient
	TableName             string
	UsersTable            string
	Bucket                string
	StateMachineARN       string
	ShortsStateMachineARN string
	// ThumbnailStateMachineARN starts AI thumbnail generation. Empty disables
	// the feature (the endpoints answer 503) so the API can deploy first.
	ThumbnailStateMachineARN string
	// S3Accelerate makes the multipart part URLs point at the S3 Transfer
	// Acceleration endpoint (env S3_ACCELERATE). Off by default, and the kill
	// switch: a client cannot turn it on when it is off here.
	S3Accelerate bool

	// PreviewRenderer and the preview limiter/cache back /slide-preview.
	// Keeping the renderer as an interface makes the endpoint testable without
	// pulling Playwright into the API binary.
	PreviewRenderer SlidePreviewRenderer
	PreviewLimiter  PreviewLimiter
	PreviewCache    PreviewCache

	// StoryPlanner backs the /story-plans endpoints (issue #191). Nil disables
	// them (they answer 503 and /story-plans/limits reports enabled=false), so
	// the API can be deployed before the feature is switched on.
	StoryPlanner StoryPlanner
	// StoryPlansPerHour overrides the per-user hourly allowance of plans. Zero
	// uses the default (20).
	StoryPlansPerHour int
	// StoryPlanNow overrides the clock for the allowance window, for tests.
	StoryPlanNow func() time.Time

	// StockMedia backs the /stock-media endpoints (issue #192). Nil disables
	// them (503), which is the case when neither PEXELS_API_KEY nor
	// PIXABAY_API_KEY is set.
	StockMedia StockMedia
	// StockLimiter caps searches per user; nil takes the default (120 an hour).
	StockLimiter     *PreviewLimiter
	stockLimiterOnce sync.Once
}

// corsHeaders returns the standard CORS headers for all responses.
func corsHeaders() map[string]string {
	return map[string]string{
		"Content-Type":                     "application/json",
		"Access-Control-Allow-Origin":      "https://indifferent.fun",
		"Access-Control-Allow-Headers":     "Content-Type,Authorization",
		"Access-Control-Allow-Methods":     "GET,POST,PUT,DELETE,OPTIONS",
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
	case req.HTTPMethod == "POST" && req.Path == "/auth/signup":
		return h.handleSignup(ctx, req)
	case req.HTTPMethod == "POST" && req.Path == "/auth/login":
		return h.handleLogin(ctx, req)
	case req.HTTPMethod == "POST" && req.Path == "/auth/refresh":
		return h.handleRefresh(ctx, req)
	case req.HTTPMethod == "POST" && req.Path == "/auth/logout":
		return h.handleLogout(ctx, req)
	case req.HTTPMethod == "GET" && req.Path == "/auth/me":
		return h.handleWithAuth(ctx, req, h.handleGetMe)
	case req.HTTPMethod == "GET" && req.Path == "/settings/notifications":
		return h.handleWithAuth(ctx, req, h.handleGetNotificationSettings)
	case req.HTTPMethod == "PUT" && req.Path == "/settings/notifications":
		return h.handleWithAuth(ctx, req, h.handleUpdateNotificationSettings)
	case req.HTTPMethod == "GET" && req.Path == "/projects":
		return h.handleWithAuth(ctx, req, h.handleListProjects)
	case req.HTTPMethod == "POST" && req.Path == "/projects":
		return h.handleWithAuth(ctx, req, h.handleCreateProject)
	case req.HTTPMethod == "POST" && strings.HasPrefix(req.Path, "/projects/") && strings.HasSuffix(req.Path, "/slide-preview"):
		return h.handleWithAuth(ctx, req, h.handleSlidePreview)
	case req.HTTPMethod == "GET" && strings.HasPrefix(req.Path, "/projects/") && strings.HasSuffix(req.Path, "/status"):
		return h.handleWithAuth(ctx, req, h.handleGetProjectStatus)
	case req.HTTPMethod == "GET" && strings.HasPrefix(req.Path, "/projects/") && strings.HasSuffix(req.Path, "/download"):
		return h.handleWithAuth(ctx, req, h.handleGetDownloadURL)
	case req.HTTPMethod == "GET" && strings.HasPrefix(req.Path, "/projects/") && strings.HasSuffix(req.Path, "/logo"):
		return h.handleWithAuth(ctx, req, h.handleGetLogoURL)
	case req.HTTPMethod == "POST" && strings.HasPrefix(req.Path, "/projects/") && strings.HasSuffix(req.Path, "/start"):
		return h.handleWithAuth(ctx, req, h.handleStartPipeline)
	case req.HTTPMethod == "POST" && strings.HasPrefix(req.Path, "/projects/") && strings.HasSuffix(req.Path, "/upload"):
		return h.handleWithAuth(ctx, req, h.handleUpload)
	case req.HTTPMethod == "POST" && strings.HasPrefix(req.Path, "/projects/") && strings.HasSuffix(req.Path, "/logo"):
		return h.handleWithAuth(ctx, req, h.handleLogoUpload)
	case req.HTTPMethod == "PUT" && isExactProjectPath(req.Path):
		return h.handleWithAuth(ctx, req, h.handleUpdateProject)
	case req.HTTPMethod == "GET" && strings.HasPrefix(req.Path, "/projects/"):
		return h.handleWithAuth(ctx, req, h.handleGetProject)
	case req.HTTPMethod == "DELETE" && isExactProjectPath(req.Path):
		return h.handleWithAuth(ctx, req, h.handleDeleteProject)
	case req.HTTPMethod == "POST" && req.Path == "/shorts":
		return h.handleWithAuth(ctx, req, h.handleCreateShorts)
	case req.HTTPMethod == "GET" && req.Path == "/shorts":
		return h.handleWithAuth(ctx, req, h.handleListShortsJobs)
	case req.HTTPMethod == "GET" && req.Path == "/shorts/limits":
		return h.handleGetShortsLimits(ctx, req)
	case req.HTTPMethod == "GET" && req.Path == "/story-plans/limits":
		return h.handleGetStoryPlanLimits(), nil
	case isStoryPlanPath(req.Path):
		return h.handleWithAuth(ctx, req, h.handleStoryPlans)
	case isStockMediaPath(req.Path):
		return h.handleWithAuth(ctx, req, h.handleStockMedia)
	case isThumbnailPath(req.Path):
		return h.handleWithAuth(ctx, req, h.handleThumbnails)
	case isMultipartPath(req.Path):
		return h.handleWithAuth(ctx, req, h.handleMultipart)
	case req.HTTPMethod == "POST" && strings.HasPrefix(req.Path, "/shorts/") && strings.HasSuffix(req.Path, "/start"):
		return h.handleWithAuth(ctx, req, h.handleStartShorts)
	case req.HTTPMethod == "POST" && strings.HasPrefix(req.Path, "/shorts/") && strings.HasSuffix(req.Path, "/cancel"):
		return h.handleWithAuth(ctx, req, h.handleCancelShorts)
	case req.HTTPMethod == "POST" && strings.HasPrefix(req.Path, "/shorts/") && strings.HasSuffix(req.Path, "/retry"):
		return h.handleWithAuth(ctx, req, h.handleRetryShorts)
	case req.HTTPMethod == "PUT" && strings.HasPrefix(req.Path, "/shorts/") && strings.HasSuffix(req.Path, "/style"):
		return h.handleWithAuth(ctx, req, h.handleUpdateCaption)
	case req.HTTPMethod == "PUT" && strings.HasPrefix(req.Path, "/shorts/") && strings.HasSuffix(req.Path, "/effects"):
		return h.handleWithAuth(ctx, req, h.handleUpdateEffects)
	case req.HTTPMethod == "GET" && strings.HasPrefix(req.Path, "/shorts/") && strings.HasSuffix(req.Path, "/url"):
		return h.handleWithAuth(ctx, req, h.handleGetClipURL)
	case req.HTTPMethod == "GET" && strings.HasPrefix(req.Path, "/shorts/") && strings.HasSuffix(req.Path, "/clips"):
		return h.handleWithAuth(ctx, req, h.handleListClips)
	case req.HTTPMethod == "GET" && strings.HasPrefix(req.Path, "/shorts/") && strings.HasSuffix(req.Path, "/captions"):
		return h.handleWithAuth(ctx, req, h.handleGetClipCaptions)
	case req.HTTPMethod == "PUT" && strings.HasPrefix(req.Path, "/shorts/") && strings.Contains(req.Path, "/clips/"):
		return h.handleWithAuth(ctx, req, h.handleUpdateClipEdit)
	case req.HTTPMethod == "GET" && strings.HasPrefix(req.Path, "/shorts/"):
		return h.handleWithAuth(ctx, req, h.handleGetShorts)
	case req.HTTPMethod == "DELETE" && isExactShortsPath(req.Path):
		return h.handleWithAuth(ctx, req, h.handleDeleteShorts)
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
		if errors.Is(err, auth.ErrEmailTaken) {
			return errorResponse(http.StatusConflict, "EMAIL_TAKEN", "An account with this email already exists"), nil
		}
		return errorResponse(http.StatusUnauthorized, "AUTH_FAILED", "Authentication failed"), nil
	}

	return jsonResponse(http.StatusOK, tokens), nil
}

// handleSignup registers an email/password user and returns tokens.
func (h *APIHandler) handleSignup(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	if h.PasswordAuth == nil {
		return errorResponse(http.StatusInternalServerError, "NOT_CONFIGURED", "Password signup is not enabled"), nil
	}
	var body struct {
		Email    string `json:"email"`
		Name     string `json:"name"`
		Password string `json:"password"`
	}
	if err := json.Unmarshal([]byte(req.Body), &body); err != nil {
		return errorResponse(http.StatusBadRequest, "INVALID_BODY", "Invalid request body"), nil
	}
	if strings.TrimSpace(body.Email) == "" || body.Password == "" {
		return errorResponse(http.StatusBadRequest, "INVALID_INPUT", "Email and password are required"), nil
	}

	tokens, err := h.PasswordAuth.SignUp(ctx, body.Email, strings.TrimSpace(body.Name), body.Password)
	if err != nil {
		if errors.Is(err, auth.ErrEmailTaken) {
			return errorResponse(http.StatusConflict, "EMAIL_TAKEN", "An account with this email already exists"), nil
		}
		if isValidationError(err) {
			return errorResponse(http.StatusBadRequest, "VALIDATION_ERROR", err.Error()), nil
		}
		return errorResponse(http.StatusInternalServerError, "SIGNUP_FAILED", "Signup failed"), nil
	}

	return jsonResponse(http.StatusCreated, tokens), nil
}

// handleLogin verifies an email/password pair and returns tokens. Failures
// always report the same generic message to avoid account enumeration.
func (h *APIHandler) handleLogin(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	if h.PasswordAuth == nil {
		return errorResponse(http.StatusInternalServerError, "NOT_CONFIGURED", "Password login is not enabled"), nil
	}
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.Unmarshal([]byte(req.Body), &body); err != nil {
		return errorResponse(http.StatusBadRequest, "INVALID_BODY", "Invalid request body"), nil
	}
	if strings.TrimSpace(body.Email) == "" || body.Password == "" {
		return errorResponse(http.StatusBadRequest, "INVALID_INPUT", "Email and password are required"), nil
	}

	tokens, err := h.PasswordAuth.Login(ctx, body.Email, body.Password)
	if err != nil {
		if errors.Is(err, auth.ErrInvalidCredentials) {
			return errorResponse(http.StatusUnauthorized, "AUTH_FAILED", "Invalid email or password"), nil
		}
		if isValidationError(err) {
			return errorResponse(http.StatusBadRequest, "VALIDATION_ERROR", err.Error()), nil
		}
		return errorResponse(http.StatusUnauthorized, "AUTH_FAILED", "Invalid email or password"), nil
	}

	return jsonResponse(http.StatusOK, tokens), nil
}

// isValidationError reports whether err is a client input problem rather than
// an auth or server failure.
func isValidationError(err error) bool {
	msg := err.Error()
	return strings.HasPrefix(msg, "invalid email") || strings.HasPrefix(msg, "password must be")
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
		if errors.Is(err, auth.ErrTokenReuse) {
			return errorResponse(http.StatusUnauthorized, "SESSION_REVOKED", "Your session has been revoked for security. Please sign in again."), nil
		}
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

// UserProfile is the safe public view of a user record returned by
// GET /auth/me. It never includes passwordHash or other secrets.
type UserProfile struct {
	Email     string `json:"email"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatarUrl"`
	CreatedAt string `json:"createdAt"`
}

// handleGetMe returns the authenticated user's profile from the users
// table (PK=USER#<id>, SK=PROFILE). Unknown users get 404 so clients can
// fall back to local defaults.
func (h *APIHandler) handleGetMe(ctx context.Context, req events.APIGatewayProxyRequest, claims *models.JWTClaims) (events.APIGatewayProxyResponse, error) {
	if h.UsersTable == "" {
		return errorResponse(http.StatusInternalServerError, "NOT_CONFIGURED", "User profiles are not enabled"), nil
	}
	result, err := h.DB.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(h.UsersTable),
		Key: map[string]dbtypes.AttributeValue{
			"PK": &dbtypes.AttributeValueMemberS{Value: "USER#" + claims.UserID},
			"SK": &dbtypes.AttributeValueMemberS{Value: "PROFILE"},
		},
		ConsistentRead: aws.Bool(true),
	})
	if err != nil {
		return errorResponse(http.StatusInternalServerError, "DB_ERROR", "Failed to load profile"), nil
	}
	if result.Item == nil {
		return errorResponse(http.StatusNotFound, "USER_NOT_FOUND", "User profile not found"), nil
	}
	return jsonResponse(http.StatusOK, UserProfile{
		Email:     stringAttr(result.Item, "email"),
		Name:      stringAttr(result.Item, "name"),
		AvatarURL: stringAttr(result.Item, "avatarUrl"),
		CreatedAt: stringAttr(result.Item, "createdAt"),
	}), nil
}

// stringAttr reads a string attribute from a DynamoDB item ("") when absent.
func stringAttr(item map[string]dbtypes.AttributeValue, key string) string {
	if v, ok := item[key].(*dbtypes.AttributeValueMemberS); ok {
		return v.Value
	}
	return ""
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
		project := itemToProject(item)
		if project.ProjectID == "" {
			// Defensive: pipeline status writers once created ID-less
			// PROJECT# records; never surface them as projects.
			continue
		}
		projects = append(projects, project)
	}

	return jsonResponse(http.StatusOK, map[string]interface{}{
		"projects": projects,
	}), nil
}

// CreateProjectRequest represents the request body for creating a project.
// Watermark and Shorts are optional: a unified project may configure all
// three tools in one call.
type CreateProjectRequest struct {
	Name          string                      `json:"name"`
	Template      string                      `json:"template"`
	DesignOptions models.DesignOptions        `json:"designOptions,omitempty"`
	Voice         string                      `json:"voice"`
	Watermark     *models.WatermarkSettings   `json:"watermark,omitempty"`
	Branding      *models.Branding            `json:"branding,omitempty"`
	Shorts        *CreateProjectShortsRequest `json:"shorts,omitempty"`
}

// CreateProjectShortsRequest asks project creation to also create a linked
// shorts job (upload URL returned alongside the project).
type CreateProjectShortsRequest struct {
	FileType string  `json:"fileType"`
	Duration float64 `json:"duration"`
	Language string  `json:"language,omitempty"`
}

// CreateProjectResponse is the POST /projects body: the project plus, when
// requested, the linked shorts upload. Warning carries a non-fatal shorts-leg
// failure: the project itself was created and the client should proceed
// without the linked upload.
type CreateProjectResponse struct {
	models.Project
	Shorts  *CreateShortsResponse `json:"shorts,omitempty"`
	Warning string                `json:"warning,omitempty"`
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

	// Optional shorts leg validated up front so creation is all-or-nothing.
	if body.Shorts != nil {
		if err := validateCreateShorts(&CreateShortsRequest{FileType: body.Shorts.FileType, Duration: body.Shorts.Duration, Language: body.Shorts.Language}); err != nil {
			return errorResponse(http.StatusBadRequest, "VALIDATION_ERROR", err.Error()), nil
		}
	}

	projectID := uuid.New().String()
	now := time.Now().UTC().Format(time.RFC3339)

	project := models.Project{
		UserID:        claims.UserID,
		ProjectID:     projectID,
		Name:          body.Name,
		Template:      body.Template,
		DesignOptions: body.DesignOptions,
		Voice:         body.Voice,
		Watermark:     body.Watermark,
		Branding:      body.Branding,
		Status:        "created",
		CreatedAt:     now,
		UpdatedAt:     now,
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

	resp := CreateProjectResponse{Project: project}

	// Optional linked shorts job; the upload URL lets the client PUT the
	// source immediately after project creation. The project itself is always
	// valid, so a shorts-leg failure degrades to "project without linked
	// upload" (201 + warning) instead of stranding half-written rows: the job
	// row is compensated away and the client proceeds quiz-only.
	if body.Shorts != nil {
		job, uploadURL, err := h.createShortsJob(ctx, claims.UserID, body.Shorts.FileType, body.Shorts.Duration, projectID, body.Shorts.Language)
		if err != nil {
			log.Printf("create_project_shorts_leg_failed user=%s project=%s err=%v", claims.UserID, projectID, err)
			resp.Warning = "Project created, but the linked Shorts upload could not be prepared. You can still create shorts separately from Shorts history."
		} else {
			resp.Shorts = &CreateShortsResponse{JobID: job.JobID, UploadURL: uploadURL, SourceKey: job.SourceKey}
			resp.Project.ShortsJobIDs = []string{job.JobID}
		}
	}

	return jsonResponse(http.StatusCreated, resp), nil
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

	// Resolve playable URLs for completed projects so detail/preview pages
	// can play inline without a separate download round-trip. Glacier-archived
	// files resolve on their next request once restored (no error surfaced).
	if project.Status == "completed" {
		if project.VideoKey != "" {
			if url, restoring, err := h.downloadURLOrRestore(ctx, project.VideoKey); err == nil && !restoring {
				project.VideoURL = url
			}
		}
		if project.ThumbnailKey != "" {
			if url, restoring, err := h.downloadURLOrRestore(ctx, project.ThumbnailKey); err == nil && !restoring {
				project.ThumbnailURL = url
			}
		}
	}

	return jsonResponse(http.StatusOK, project), nil
}

// handleDeleteProject deletes a project, verifying ownership. Linked shorts
// jobs are removed first (each fully: pipeline, S3, record), then every
// project S3 asset (uploads, temp slides/audio, rendered output), then the
// record itself. Linked-job failures are logged but don't block the project
// delete: the job stays reachable from Shorts history for a standalone retry.
func (h *APIHandler) handleDeleteProject(ctx context.Context, req events.APIGatewayProxyRequest, claims *models.JWTClaims) (events.APIGatewayProxyResponse, error) {
	projectID := extractProjectID(req.Path)
	if projectID == "" {
		return errorResponse(http.StatusBadRequest, "INVALID_INPUT", "Project ID is required"), nil
	}

	// Verify ownership by trying to get the project first
	project, err := h.getProjectByID(ctx, claims.UserID, projectID)
	if err != nil {
		return errorResponse(http.StatusNotFound, "NOT_FOUND", "Project not found"), nil
	}

	// Best effort: stop a running quiz pipeline (deterministic execution
	// name, mirroring the start handler).
	if quizJobInFlight(project.Status) {
		if arn, aerr := quizExecutionARN(h.StateMachineARN, projectID); aerr == nil {
			_, _ = h.SFN.StopExecution(ctx, &sfn.StopExecutionInput{
				ExecutionArn: aws.String(arn),
			})
		}
	}

	for _, jobID := range project.ShortsJobIDs {
		if err := h.removeShortsJob(ctx, claims.UserID, jobID); err != nil &&
			!errors.Is(err, ErrShortsJobNotFound) {
			log.Printf("project_delete_shorts_failed user=%s project=%s job=%s err=%v", claims.UserID, projectID, jobID, err)
		}
	}

	keys, err := h.projectSweepKeys(ctx, claims.UserID, project)
	if err != nil {
		log.Printf("project_delete_list_failed user=%s project=%s err=%v (falling back to tracked keys)", claims.UserID, projectID, err)
		keys = constrainToPrefixes(projectAssetKeys(project), projectAssetPrefixes(claims.UserID, projectID))
	}
	if err := h.S3.DeleteObjects(ctx, h.Bucket, keys); err != nil {
		log.Printf("project_delete_s3_failed user=%s project=%s keys=%d err=%v", claims.UserID, projectID, len(keys), err)
		return errorResponse(http.StatusInternalServerError, "S3_CLEANUP_FAILED", "Failed to delete project assets. Please retry."), nil
	}

	_, err = h.DB.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(h.TableName),
		Key: map[string]dbtypes.AttributeValue{
			"PK": &dbtypes.AttributeValueMemberS{Value: "USER#" + claims.UserID},
			"SK": &dbtypes.AttributeValueMemberS{Value: "PROJECT#" + projectID},
		},
	})
	if err != nil {
		log.Printf("project_delete_db_failed user=%s project=%s err=%v", claims.UserID, projectID, err)
		return errorResponse(http.StatusInternalServerError, "DB_ERROR", "Failed to delete project"), nil
	}

	for _, p := range projectAssetPrefixes(claims.UserID, projectID) {
		if err := h.S3.DeleteObjectVersions(ctx, h.Bucket, p); err != nil {
			log.Printf("project_delete_purge_failed user=%s project=%s prefix=%s err=%v", claims.UserID, projectID, p, err)
		}
	}

	log.Printf("project_deleted user=%s project=%s keys=%d s3=success", claims.UserID, projectID, len(keys))
	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusNoContent,
		Headers:    corsHeaders(),
	}, nil
}

// quizJobInFlight reports whether a quiz project may still have a running
// Step Functions execution worth stopping before deletion.
func quizJobInFlight(status string) bool {
	switch status {
	case "parsing", "generating_slides", "narrating", "rendering":
		return true
	}
	return false
}

// quizExecutionARN derives the quiz pipeline execution ARN from the state
// machine ARN and the deterministic execution name used at start.
func quizExecutionARN(stateMachineARN, projectID string) (string, error) {
	return stateMachineExecutionARN(stateMachineARN, "project-"+projectID)
}

// projectAssetPrefixes are the only S3 locations a quiz project may own:
// user uploads + logo, worker temp slides/audio, rendered output.
func projectAssetPrefixes(userID, projectID string) []string {
	return []string{
		"uploads/" + userID + "/" + projectID + "/",
		"temp/" + projectID + "/",
		"output/" + projectID + "/",
	}
}

// projectAssetKeys collects the record's tracked keys: source text,
// rendered video + thumbnail, and brand logo.
func projectAssetKeys(project *models.Project) []string {
	seen := map[string]bool{}
	var keys []string
	add := func(k string) {
		if k == "" || seen[k] {
			return
		}
		seen[k] = true
		keys = append(keys, k)
	}
	add(project.TxtKey)
	add(project.JSONKey)
	add(project.VideoKey)
	add(project.ThumbnailKey)
	if project.Branding != nil {
		add(project.Branding.LogoKey)
	}
	return keys
}

// projectSweepKeys lists every object under the project's prefixes plus the
// record's tracked keys constrained to those prefixes.
func (h *APIHandler) projectSweepKeys(ctx context.Context, userID string, project *models.Project) ([]string, error) {
	prefixes := projectAssetPrefixes(userID, project.ProjectID)
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
	for _, k := range constrainToPrefixes(projectAssetKeys(project), prefixes) {
		add(k)
	}
	return keys, nil
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

// handleLogoUpload generates a signed upload URL for a project's logo image
// (PNG) and records its key on the project for branded renders.
func (h *APIHandler) handleLogoUpload(ctx context.Context, req events.APIGatewayProxyRequest, claims *models.JWTClaims) (events.APIGatewayProxyResponse, error) {
	projectID := extractProjectIDFromSubpath(req.Path, "/logo")
	if projectID == "" {
		return errorResponse(http.StatusBadRequest, "INVALID_INPUT", "Project ID is required"), nil
	}

	logoKey := fmt.Sprintf("uploads/%s/%s/logo.png", claims.UserID, projectID)
	uploadURL, err := h.S3.GenerateUploadURL(ctx, h.Bucket, logoKey, "image/png", storage.UploadURLExpiration)
	if err != nil {
		return errorResponse(http.StatusInternalServerError, "S3_ERROR", "Failed to generate upload URL"), nil
	}

	// Optimistically locked write (see handleUpdateProject). Ownership is
	// verified by the update itself.
	if _, err := h.updateProjectFields(ctx, claims.UserID, projectID, func(p *models.Project) {
		if p.Branding == nil {
			p.Branding = &models.Branding{}
		}
		p.Branding.LogoKey = logoKey
	}); err != nil {
		switch {
		case err.Error() == "project not found":
			return errorResponse(http.StatusNotFound, "NOT_FOUND", "Project not found"), nil
		case strings.Contains(err.Error(), "concurrently"):
			return errorResponse(http.StatusConflict, "CONFLICT", "Project changed concurrently, please retry."), nil
		default:
			return errorResponse(http.StatusInternalServerError, "DB_ERROR", "Failed to save logo"), nil
		}
	}

	return jsonResponse(http.StatusOK, map[string]string{
		"uploadUrl": uploadURL,
		"logoKey":   logoKey,
	}), nil
}

// UpdateProjectRequest carries the mutable branding fields of a project.
type UpdateProjectRequest struct {
	ChannelName *string                   `json:"channelName,omitempty"`
	Watermark   *models.WatermarkSettings `json:"watermark,omitempty"`
}

// handleUpdateProject updates a project's branding (channel handle and
// watermark preferences), verifying ownership.
func (h *APIHandler) handleUpdateProject(ctx context.Context, req events.APIGatewayProxyRequest, claims *models.JWTClaims) (events.APIGatewayProxyResponse, error) {
	projectID := extractProjectID(req.Path)
	if projectID == "" {
		return errorResponse(http.StatusBadRequest, "INVALID_INPUT", "Project ID is required"), nil
	}

	var body UpdateProjectRequest
	if err := json.Unmarshal([]byte(req.Body), &body); err != nil {
		return errorResponse(http.StatusBadRequest, "INVALID_BODY", "Invalid request body"), nil
	}

	if body.ChannelName != nil {
		name := strings.TrimSpace(*body.ChannelName)
		if len(name) > 60 {
			return errorResponse(http.StatusBadRequest, "VALIDATION_ERROR", "Channel name must be 60 characters or less"), nil
		}
	}
	if body.Watermark != nil {
		if len(body.Watermark.Text) > 100 {
			return errorResponse(http.StatusBadRequest, "VALIDATION_ERROR", "Watermark text must be 100 characters or less"), nil
		}
	}

	// Optimistically locked write: a concurrent status update is retried
	// instead of clobbered. Ownership is verified by the update itself.
	updated, err := h.updateProjectFields(ctx, claims.UserID, projectID, func(p *models.Project) {
		if body.ChannelName != nil {
			if p.Branding == nil {
				p.Branding = &models.Branding{}
			}
			p.Branding.ChannelName = strings.TrimSpace(*body.ChannelName)
		}
		if body.Watermark != nil {
			p.Watermark = body.Watermark
		}
	})
	if err != nil {
		switch {
		case err.Error() == "project not found":
			return errorResponse(http.StatusNotFound, "NOT_FOUND", "Project not found"), nil
		case strings.Contains(err.Error(), "concurrently"):
			return errorResponse(http.StatusConflict, "CONFLICT", "Project changed concurrently, please retry."), nil
		default:
			return errorResponse(http.StatusInternalServerError, "DB_ERROR", "Failed to update project"), nil
		}
	}

	return jsonResponse(http.StatusOK, updated), nil
}

// handleGetLogoURL returns a presigned GET URL for a project's brand logo.
func (h *APIHandler) handleGetLogoURL(ctx context.Context, req events.APIGatewayProxyRequest, claims *models.JWTClaims) (events.APIGatewayProxyResponse, error) {
	projectID := extractProjectIDFromSubpath(req.Path, "/logo")
	if projectID == "" {
		return errorResponse(http.StatusBadRequest, "INVALID_INPUT", "Project ID is required"), nil
	}

	project, err := h.getProjectByID(ctx, claims.UserID, projectID)
	if err != nil {
		return errorResponse(http.StatusNotFound, "NOT_FOUND", "Project not found"), nil
	}
	if project.Branding == nil || project.Branding.LogoKey == "" {
		return errorResponse(http.StatusNotFound, "NOT_FOUND", "No logo uploaded"), nil
	}

	url, restoring, err := h.downloadURLOrRestore(ctx, project.Branding.LogoKey)
	if err != nil {
		if errors.Is(err, errObjectNotFound) {
			return errorResponse(http.StatusNotFound, "NOT_FOUND", "No logo uploaded"), nil
		}
		return errorResponse(http.StatusInternalServerError, "S3_ERROR", "Failed to generate logo URL"), nil
	}
	if restoring {
		return restoringResponse(), nil
	}

	return jsonResponse(http.StatusOK, map[string]string{
		"url": url,
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
	sfnInput, _ := json.Marshal(map[string]any{
		"projectId":     projectID,
		"userId":        claims.UserID,
		"template":      project.Template,
		"designOptions": project.DesignOptions,
		"voice":         project.Voice,
		"s3Key":         txtKey,
		"logoKey":       projectLogoKey(project),
	})

	_, err = h.SFN.StartExecution(ctx, &sfn.StartExecutionInput{
		StateMachineArn: aws.String(h.StateMachineARN),
		Name:            aws.String(fmt.Sprintf("project-%s", projectID)),
		Input:           aws.String(string(sfnInput)),
	})
	if err != nil {
		// Idempotent restart: the deterministic execution name means a
		// duplicate start races an already-running execution.
		if isExecutionAlreadyExists(err) {
			log.Printf("start_pipeline_already_running project=%s", projectID)
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

// errObjectNotFound marks a download whose S3 object no longer exists.
var errObjectNotFound = fmt.Errorf("video file not found")

// downloadURLOrRestore resolves a presigned GET URL for key, Glacier-aware:
// archived objects without a usable copy trigger a restore and report
// restoring=true (caller answers 202); missing objects yield
// errObjectNotFound. A failed status check falls back to presigning so a
// transient HeadObject problem never blocks a healthy download.
func (h *APIHandler) downloadURLOrRestore(ctx context.Context, key string) (url string, restoring bool, err error) {
	if st, serr := h.S3.ObjectStatus(ctx, h.Bucket, key); serr == nil {
		switch {
		case st.RestoreInProgress():
			return "", true, nil
		case !st.Downloadable():
			if rerr := h.S3.RestoreObject(ctx, h.Bucket, key); rerr != nil {
				return "", false, rerr
			}
			return "", true, nil
		}
	} else if storage.IsNotFound(serr) {
		return "", false, errObjectNotFound
	}

	url, err = h.S3.GenerateDownloadURL(ctx, h.Bucket, key, storage.DownloadURLExpiration)
	if err != nil {
		return "", false, err
	}
	return url, false, nil
}

// restoringResponse is the 202 body returned while an archived video is being
// restored to a downloadable copy (Standard retrieval: minutes to hours).
func restoringResponse() events.APIGatewayProxyResponse {
	return jsonResponse(http.StatusAccepted, map[string]string{
		"code":    "RESTORING",
		"message": "This video is in cold storage and is being restored. Please check back in a few hours.",
	})
}

// projectLogoKey returns the project's logo key for pipeline input (""
// when unbranded; Step Functions drops empty-string parameters).
func projectLogoKey(project *models.Project) string {
	if project.Branding == nil {
		return ""
	}
	return project.Branding.LogoKey
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

	downloadURL, restoring, err := h.downloadURLOrRestore(ctx, project.VideoKey)
	if err != nil {
		if errors.Is(err, errObjectNotFound) {
			return errorResponse(http.StatusNotFound, "NOT_FOUND", "Video file not found"), nil
		}
		return errorResponse(http.StatusInternalServerError, "S3_ERROR", "Failed to generate download URL"), nil
	}
	if restoring {
		return restoringResponse(), nil
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

	if err := models.ValidateDesignOptions(req.Template, &req.DesignOptions); err != nil {
		return err
	}

	if !validVoices[req.Voice] {
		return fmt.Errorf("invalid voice: must be one of Joanna, Matthew, Amy, Brian, Aditi")
	}

	if req.Watermark != nil && len(req.Watermark.Text) > 100 {
		return fmt.Errorf("watermark text must be 100 characters or less")
	}

	if req.Branding != nil && len(strings.TrimSpace(req.Branding.ChannelName)) > 60 {
		return fmt.Errorf("channel name must be 60 characters or less")
	}

	return nil
}

// sanitizeString strips HTML tags and escapes special characters. Entities
// are decoded FIRST so encoded payloads (&lt;script&gt;) can't slip past
// tag stripping only to be decoded later at render time.
func sanitizeString(s string) string {
	// Decode entities first so encoded tags become visible to the stripper.
	s = html.UnescapeString(s)
	// Remove HTML tags
	s = stripHTMLTags(s)
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

// isExactProjectPath reports whether path is exactly /projects/{id} (no
// deeper sub-resource): only exact updates are mutable via PUT, deeper PUT
// paths fall through to 404 instead of mutating the parent project.
func isExactProjectPath(path string) bool {
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	return len(parts) == 2 && parts[0] == "projects" && parts[1] != ""
}

// isExecutionAlreadyExists reports a duplicate Step Functions start: with
// deterministic execution names ("project-<id>") a double-click, timeout
// retry, or restart races the first execution. It is already running, so
// callers treat it as success instead of a 500.
func isExecutionAlreadyExists(err error) bool {
	var exists *sfntypes.ExecutionAlreadyExists
	return errors.As(err, &exists)
}

// updateProjectFields reloads the project, applies mutate, and writes it back
// only if untouched since the read (optimistic locking on updatedAt). A
// concurrent status-worker write between read and write fails the condition
// instead of being silently clobbered; the operation reloads once and
// retries. Records without updatedAt (legacy) fall back to an unconditional
// write. Returns the stored project.
func (h *APIHandler) updateProjectFields(ctx context.Context, userID, projectID string, mutate func(*models.Project)) (*models.Project, error) {
	for attempt := 0; attempt < 2; attempt++ {
		project, err := h.getProjectByID(ctx, userID, projectID)
		if err != nil {
			return nil, err
		}
		stamp := project.UpdatedAt
		mutate(project)
		project.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

		input := &dynamodb.PutItemInput{
			TableName: aws.String(h.TableName),
			Item:      projectToItem(*project),
		}
		if stamp != "" {
			input.ConditionExpression = aws.String("updatedAt = :old")
			input.ExpressionAttributeValues = map[string]dbtypes.AttributeValue{
				":old": &dbtypes.AttributeValueMemberS{Value: stamp},
			}
		}
		if _, err := h.DB.PutItem(ctx, input); err != nil {
			var condFailed *dbtypes.ConditionalCheckFailedException
			if stamp != "" && errors.As(err, &condFailed) {
				continue
			}
			return nil, err
		}
		return project, nil
	}
	return nil, fmt.Errorf("project changed concurrently, please retry")
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
	if !p.DesignOptions.IsZero() {
		if b, err := json.Marshal(p.DesignOptions); err == nil {
			item["designOptions"] = &dbtypes.AttributeValueMemberS{Value: string(b)}
		}
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
	if len(p.ShortsJobIDs) > 0 {
		if b, err := json.Marshal(p.ShortsJobIDs); err == nil {
			item["shortsJobIds"] = &dbtypes.AttributeValueMemberS{Value: string(b)}
		}
	}
	if p.Watermark != nil {
		if b, err := json.Marshal(p.Watermark); err == nil {
			item["watermark"] = &dbtypes.AttributeValueMemberS{Value: string(b)}
		}
	}
	if p.Branding != nil {
		if b, err := json.Marshal(p.Branding); err == nil {
			item["branding"] = &dbtypes.AttributeValueMemberS{Value: string(b)}
		}
	}
	if p.CompletedAt != "" {
		item["completedAt"] = &dbtypes.AttributeValueMemberS{Value: p.CompletedAt}
	}
	if p.Error != "" {
		item["error"] = &dbtypes.AttributeValueMemberS{Value: p.Error}
	}
	if len(p.Warnings) > 0 {
		if b, err := json.Marshal(p.Warnings); err == nil {
			item["warnings"] = &dbtypes.AttributeValueMemberS{Value: string(b)}
		}
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
	if v, ok := item["designOptions"].(*dbtypes.AttributeValueMemberS); ok && v.Value != "" {
		_ = json.Unmarshal([]byte(v.Value), &p.DesignOptions)
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
	if v, ok := item["shortsJobIds"].(*dbtypes.AttributeValueMemberS); ok && v.Value != "" {
		_ = json.Unmarshal([]byte(v.Value), &p.ShortsJobIDs)
	}
	if v, ok := item["watermark"].(*dbtypes.AttributeValueMemberS); ok && v.Value != "" {
		var w models.WatermarkSettings
		if err := json.Unmarshal([]byte(v.Value), &w); err == nil {
			p.Watermark = &w
		}
	}
	if v, ok := item["branding"].(*dbtypes.AttributeValueMemberS); ok && v.Value != "" {
		var b models.Branding
		if err := json.Unmarshal([]byte(v.Value), &b); err == nil {
			p.Branding = &b
		}
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
	if v, ok := item["warnings"].(*dbtypes.AttributeValueMemberS); ok && v.Value != "" {
		var w []models.Warning
		if err := json.Unmarshal([]byte(v.Value), &w); err == nil {
			p.Warnings = w
		}
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
