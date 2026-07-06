package api

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/sfn"
	"github.com/golang-jwt/jwt/v5"
	"github.com/rahul/indifferent/backend/internal/auth"
	"github.com/rahul/indifferent/backend/internal/models"
)

const testSecret = "test-secret-key-for-testing-only"

// mockDynamoDB is a mock DynamoDB client for testing.
type mockDynamoDB struct {
	putItemFunc    func(ctx context.Context, params *dynamodb.PutItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error)
	getItemFunc    func(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error)
	deleteItemFunc func(ctx context.Context, params *dynamodb.DeleteItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error)
	queryFunc      func(ctx context.Context, params *dynamodb.QueryInput, optFns ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error)
}

func (m *mockDynamoDB) PutItem(ctx context.Context, params *dynamodb.PutItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
	if m.putItemFunc != nil {
		return m.putItemFunc(ctx, params, optFns...)
	}
	return &dynamodb.PutItemOutput{}, nil
}

func (m *mockDynamoDB) GetItem(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
	if m.getItemFunc != nil {
		return m.getItemFunc(ctx, params, optFns...)
	}
	return &dynamodb.GetItemOutput{}, nil
}

func (m *mockDynamoDB) DeleteItem(ctx context.Context, params *dynamodb.DeleteItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error) {
	if m.deleteItemFunc != nil {
		return m.deleteItemFunc(ctx, params, optFns...)
	}
	return &dynamodb.DeleteItemOutput{}, nil
}

func (m *mockDynamoDB) Query(ctx context.Context, params *dynamodb.QueryInput, optFns ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error) {
	if m.queryFunc != nil {
		return m.queryFunc(ctx, params, optFns...)
	}
	return &dynamodb.QueryOutput{Items: []map[string]dbtypes.AttributeValue{}}, nil
}

// mockSFN is a mock Step Functions client.
type mockSFN struct {
	startExecutionFunc func(ctx context.Context, params *sfn.StartExecutionInput, optFns ...func(*sfn.Options)) (*sfn.StartExecutionOutput, error)
}

func (m *mockSFN) StartExecution(ctx context.Context, params *sfn.StartExecutionInput, optFns ...func(*sfn.Options)) (*sfn.StartExecutionOutput, error) {
	if m.startExecutionFunc != nil {
		return m.startExecutionFunc(ctx, params, optFns...)
	}
	return &sfn.StartExecutionOutput{}, nil
}

// mockAuthService is a mock GoogleAuthService.
type mockAuthService struct {
	authenticateFunc func(ctx context.Context, authCode string) (*models.AuthTokens, error)
}

func (m *mockAuthService) Authenticate(ctx context.Context, authCode string) (*models.AuthTokens, error) {
	if m.authenticateFunc != nil {
		return m.authenticateFunc(ctx, authCode)
	}
	return &models.AuthTokens{
		AccessToken:  "mock-access-token",
		RefreshToken: "mock-refresh-token",
		ExpiresIn:    900,
	}, nil
}

// generateTestToken creates a valid JWT token for testing.
func generateTestToken(userID string) string {
	claims := jwt.MapClaims{
		"userId": userID,
		"email":  "test@example.com",
		"iat":    time.Now().Unix(),
		"exp":    time.Now().Add(15 * time.Minute).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, _ := token.SignedString([]byte(testSecret))
	return signed
}

// generateExpiredToken creates an expired JWT token for testing.
func generateExpiredToken(userID string) string {
	claims := jwt.MapClaims{
		"userId": userID,
		"email":  "test@example.com",
		"iat":    time.Now().Add(-30 * time.Minute).Unix(),
		"exp":    time.Now().Add(-15 * time.Minute).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, _ := token.SignedString([]byte(testSecret))
	return signed
}

func newTestHandler() *APIHandler {
	db := &mockDynamoDB{}
	jwtService := &auth.JWTService{
		Secret:       testSecret,
		DB:           db,
		SessionTable: "sessions-table",
	}
	return &APIHandler{
		AuthService:     &mockAuthService{},
		JWTService:      jwtService,
		DB:              db,
		S3:              nil, // S3 operations will be tested separately
		SFN:             &mockSFN{},
		TableName:       "projects-table",
		Bucket:          "test-bucket",
		StateMachineARN: "arn:aws:states:us-east-1:123456789:stateMachine:test",
	}
}

func TestRouteMatching(t *testing.T) {
	handler := newTestHandler()
	ctx := context.Background()

	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
	}{
		{"POST auth callback", "POST", "/auth/google/callback", 400}, // 400 because body is empty
		{"POST auth refresh", "POST", "/auth/refresh", 400},
		{"POST auth logout", "POST", "/auth/logout", 400},
		{"GET projects - no auth", "GET", "/projects", 401},
		{"POST projects - no auth", "POST", "/projects", 401},
		{"GET project by ID - no auth", "GET", "/projects/abc123", 401},
		{"DELETE project - no auth", "DELETE", "/projects/abc123", 401},
		{"POST upload - no auth", "POST", "/projects/abc123/upload", 401},
		{"GET status - no auth", "GET", "/projects/abc123/status", 401},
		{"GET download - no auth", "GET", "/projects/abc123/download", 401},
		{"OPTIONS preflight", "OPTIONS", "/projects", 200},
		{"Unknown route", "GET", "/unknown", 404},
		{"Unknown method", "PATCH", "/projects", 404},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := events.APIGatewayProxyRequest{
				HTTPMethod: tt.method,
				Path:       tt.path,
			}
			resp, err := handler.HandleRequest(ctx, req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if resp.StatusCode != tt.wantStatus {
				t.Errorf("got status %d, want %d (body: %s)", resp.StatusCode, tt.wantStatus, resp.Body)
			}
		})
	}
}

func TestAuthValidation(t *testing.T) {
	handler := newTestHandler()
	ctx := context.Background()

	tests := []struct {
		name       string
		authHeader string
		wantStatus int
	}{
		{"No auth header", "", 401},
		{"Invalid format - no Bearer", "Token abc123", 401},
		{"Invalid token", "Bearer invalid-token", 401},
		{"Expired token", "Bearer " + generateExpiredToken("user1"), 401},
		{"Valid token", "Bearer " + generateTestToken("user1"), 200},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up mock to return empty project list for valid auth
			handler.DB = &mockDynamoDB{
				queryFunc: func(ctx context.Context, params *dynamodb.QueryInput, optFns ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error) {
					return &dynamodb.QueryOutput{Items: []map[string]dbtypes.AttributeValue{}}, nil
				},
			}
			handler.JWTService = &auth.JWTService{
				Secret:       testSecret,
				DB:           handler.DB,
				SessionTable: "sessions-table",
			}

			req := events.APIGatewayProxyRequest{
				HTTPMethod: "GET",
				Path:       "/projects",
				Headers:    map[string]string{},
			}
			if tt.authHeader != "" {
				req.Headers["Authorization"] = tt.authHeader
			}
			resp, err := handler.HandleRequest(ctx, req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if resp.StatusCode != tt.wantStatus {
				t.Errorf("got status %d, want %d (body: %s)", resp.StatusCode, tt.wantStatus, resp.Body)
			}
		})
	}
}

func TestInputValidation(t *testing.T) {
	handler := newTestHandler()
	handler.DB = &mockDynamoDB{
		putItemFunc: func(ctx context.Context, params *dynamodb.PutItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
			return &dynamodb.PutItemOutput{}, nil
		},
	}
	handler.JWTService = &auth.JWTService{
		Secret:       testSecret,
		DB:           handler.DB,
		SessionTable: "sessions-table",
	}
	ctx := context.Background()
	validToken := generateTestToken("user1")

	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantCode   string
	}{
		{
			"Empty body",
			"",
			400,
			"INVALID_BODY",
		},
		{
			"Missing name",
			`{"template":"classic","voice":"Joanna"}`,
			400,
			"VALIDATION_ERROR",
		},
		{
			"Name too long",
			`{"name":"` + strings.Repeat("a", 101) + `","template":"classic","voice":"Joanna"}`,
			400,
			"VALIDATION_ERROR",
		},
		{
			"Invalid template",
			`{"name":"Test Project","template":"invalid","voice":"Joanna"}`,
			400,
			"VALIDATION_ERROR",
		},
		{
			"Invalid voice",
			`{"name":"Test Project","template":"classic","voice":"InvalidVoice"}`,
			400,
			"VALIDATION_ERROR",
		},
		{
			"HTML in name gets stripped",
			`{"name":"<script>alert('xss')</script>Test","template":"classic","voice":"Joanna"}`,
			201,
			"",
		},
		{
			"Valid request",
			`{"name":"My Project","template":"classic","voice":"Joanna"}`,
			201,
			"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := events.APIGatewayProxyRequest{
				HTTPMethod: "POST",
				Path:       "/projects",
				Headers: map[string]string{
					"Authorization": "Bearer " + validToken,
				},
				Body: tt.body,
			}
			resp, err := handler.HandleRequest(ctx, req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if resp.StatusCode != tt.wantStatus {
				t.Errorf("got status %d, want %d (body: %s)", resp.StatusCode, tt.wantStatus, resp.Body)
			}
			if tt.wantCode != "" {
				var apiErr models.APIError
				if err := json.Unmarshal([]byte(resp.Body), &apiErr); err != nil {
					t.Fatalf("failed to unmarshal error response: %v", err)
				}
				if apiErr.Code != tt.wantCode {
					t.Errorf("got error code %q, want %q", apiErr.Code, tt.wantCode)
				}
			}
		})
	}
}

func TestGoogleCallback(t *testing.T) {
	handler := newTestHandler()
	ctx := context.Background()

	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{"Empty body", "", 400},
		{"Empty code", `{"code":""}`, 400},
		{"Whitespace code", `{"code":"   "}`, 400},
		{"Valid code", `{"code":"valid-google-auth-code"}`, 200},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := events.APIGatewayProxyRequest{
				HTTPMethod: "POST",
				Path:       "/auth/google/callback",
				Body:       tt.body,
			}
			resp, err := handler.HandleRequest(ctx, req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if resp.StatusCode != tt.wantStatus {
				t.Errorf("got status %d, want %d (body: %s)", resp.StatusCode, tt.wantStatus, resp.Body)
			}
		})
	}
}

func TestListProjects(t *testing.T) {
	ctx := context.Background()
	validToken := generateTestToken("user1")

	db := &mockDynamoDB{
		queryFunc: func(ctx context.Context, params *dynamodb.QueryInput, optFns ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error) {
			return &dynamodb.QueryOutput{
				Items: []map[string]dbtypes.AttributeValue{
					{
						"PK":        &dbtypes.AttributeValueMemberS{Value: "USER#user1"},
						"SK":        &dbtypes.AttributeValueMemberS{Value: "PROJECT#proj1"},
						"projectId": &dbtypes.AttributeValueMemberS{Value: "proj1"},
						"name":      &dbtypes.AttributeValueMemberS{Value: "Test Project"},
						"template":  &dbtypes.AttributeValueMemberS{Value: "classic"},
						"voice":     &dbtypes.AttributeValueMemberS{Value: "Joanna"},
						"status":    &dbtypes.AttributeValueMemberS{Value: "completed"},
						"createdAt": &dbtypes.AttributeValueMemberS{Value: "2024-01-01T00:00:00Z"},
						"updatedAt": &dbtypes.AttributeValueMemberS{Value: "2024-01-01T01:00:00Z"},
					},
				},
			}, nil
		},
	}

	handler := &APIHandler{
		AuthService:     &mockAuthService{},
		JWTService:      &auth.JWTService{Secret: testSecret, DB: db, SessionTable: "sessions-table"},
		DB:              db,
		SFN:             &mockSFN{},
		TableName:       "projects-table",
		Bucket:          "test-bucket",
		StateMachineARN: "arn:aws:states:us-east-1:123456789:stateMachine:test",
	}

	req := events.APIGatewayProxyRequest{
		HTTPMethod: "GET",
		Path:       "/projects",
		Headers: map[string]string{
			"Authorization": "Bearer " + validToken,
		},
	}
	resp, err := handler.HandleRequest(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("got status %d, want 200 (body: %s)", resp.StatusCode, resp.Body)
	}

	var result struct {
		Projects []models.Project `json:"projects"`
	}
	if err := json.Unmarshal([]byte(resp.Body), &result); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if len(result.Projects) != 1 {
		t.Errorf("got %d projects, want 1", len(result.Projects))
	}
	if result.Projects[0].ProjectID != "proj1" {
		t.Errorf("got project ID %q, want %q", result.Projects[0].ProjectID, "proj1")
	}
}

func TestGetProject_OwnershipVerification(t *testing.T) {
	ctx := context.Background()

	db := &mockDynamoDB{
		getItemFunc: func(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
			// Only return project if the PK matches user1
			pk := params.Key["PK"].(*dbtypes.AttributeValueMemberS).Value
			if pk == "USER#user1" {
				return &dynamodb.GetItemOutput{
					Item: map[string]dbtypes.AttributeValue{
						"PK":        &dbtypes.AttributeValueMemberS{Value: "USER#user1"},
						"SK":        &dbtypes.AttributeValueMemberS{Value: "PROJECT#proj1"},
						"projectId": &dbtypes.AttributeValueMemberS{Value: "proj1"},
						"name":      &dbtypes.AttributeValueMemberS{Value: "Test Project"},
						"template":  &dbtypes.AttributeValueMemberS{Value: "classic"},
						"voice":     &dbtypes.AttributeValueMemberS{Value: "Joanna"},
						"status":    &dbtypes.AttributeValueMemberS{Value: "completed"},
						"createdAt": &dbtypes.AttributeValueMemberS{Value: "2024-01-01T00:00:00Z"},
						"updatedAt": &dbtypes.AttributeValueMemberS{Value: "2024-01-01T01:00:00Z"},
					},
				}, nil
			}
			// user2 trying to access user1's project - DynamoDB returns nothing
			return &dynamodb.GetItemOutput{Item: nil}, nil
		},
	}

	handler := &APIHandler{
		AuthService:     &mockAuthService{},
		JWTService:      &auth.JWTService{Secret: testSecret, DB: db, SessionTable: "sessions-table"},
		DB:              db,
		SFN:             &mockSFN{},
		TableName:       "projects-table",
		Bucket:          "test-bucket",
		StateMachineARN: "arn:aws:states:us-east-1:123456789:stateMachine:test",
	}

	// user1 can access their own project
	t.Run("Owner can access project", func(t *testing.T) {
		token := generateTestToken("user1")
		req := events.APIGatewayProxyRequest{
			HTTPMethod: "GET",
			Path:       "/projects/proj1",
			Headers: map[string]string{
				"Authorization": "Bearer " + token,
			},
		}
		resp, err := handler.HandleRequest(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode != 200 {
			t.Errorf("got status %d, want 200", resp.StatusCode)
		}
	})

	// user2 cannot access user1's project
	t.Run("Non-owner gets 404", func(t *testing.T) {
		token := generateTestToken("user2")
		req := events.APIGatewayProxyRequest{
			HTTPMethod: "GET",
			Path:       "/projects/proj1",
			Headers: map[string]string{
				"Authorization": "Bearer " + token,
			},
		}
		resp, err := handler.HandleRequest(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatusCode != 404 {
			t.Errorf("got status %d, want 404", resp.StatusCode)
		}
	})
}

func TestSanitizeString(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Hello World", "Hello World"},
		{"<script>alert('xss')</script>", "alert('xss')"},
		{"<b>Bold</b> text", "Bold text"},
		{"  spaces  ", "spaces"},
		{"No<br>tags", "Notags"},
		{"<a href='test'>Link</a>", "Link"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := sanitizeString(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeString(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestExtractProjectID(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/projects/abc123", "abc123"},
		{"/projects/uuid-1234-5678", "uuid-1234-5678"},
		{"/projects/", ""},
		{"/projects", ""},
		{"/other/path", ""},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := extractProjectID(tt.path)
			if got != tt.want {
				t.Errorf("extractProjectID(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestExtractProjectIDFromSubpath(t *testing.T) {
	tests := []struct {
		path   string
		suffix string
		want   string
	}{
		{"/projects/abc123/upload", "/upload", "abc123"},
		{"/projects/abc123/status", "/status", "abc123"},
		{"/projects/abc123/download", "/download", "abc123"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := extractProjectIDFromSubpath(tt.path, tt.suffix)
			if got != tt.want {
				t.Errorf("extractProjectIDFromSubpath(%q, %q) = %q, want %q", tt.path, tt.suffix, got, tt.want)
			}
		})
	}
}

func TestValidateCreateProject(t *testing.T) {
	tests := []struct {
		name    string
		req     CreateProjectRequest
		wantErr bool
	}{
		{
			"Valid request",
			CreateProjectRequest{Name: "My Project", Template: "classic", Voice: "Joanna"},
			false,
		},
		{
			"Empty name",
			CreateProjectRequest{Name: "", Template: "classic", Voice: "Joanna"},
			true,
		},
		{
			"Name too long",
			CreateProjectRequest{Name: string(make([]byte, 101)), Template: "classic", Voice: "Joanna"},
			true,
		},
		{
			"Invalid template",
			CreateProjectRequest{Name: "Test", Template: "invalid", Voice: "Joanna"},
			true,
		},
		{
			"Invalid voice",
			CreateProjectRequest{Name: "Test", Template: "classic", Voice: "Unknown"},
			true,
		},
		{
			"All valid templates",
			CreateProjectRequest{Name: "Test", Template: "modern", Voice: "Matthew"},
			false,
		},
		{
			"All valid voices",
			CreateProjectRequest{Name: "Test", Template: "neon", Voice: "Aditi"},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCreateProject(&tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateCreateProject() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestBuildPipelineProgress(t *testing.T) {
	tests := []struct {
		status         string
		wantPercentage int
	}{
		{"created", 0},
		{"parsing", 5},
		{"generating_slides", 25},
		{"narrating", 50},
		{"rendering", 75},
		{"completed", 100},
		{"failed", 0},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			project := &models.Project{Status: tt.status}
			progress := buildPipelineProgress(project)
			if progress.Percentage != tt.wantPercentage {
				t.Errorf("got percentage %d, want %d", progress.Percentage, tt.wantPercentage)
			}
			if progress.Stage != tt.status {
				t.Errorf("got stage %q, want %q", progress.Stage, tt.status)
			}
		})
	}
}

func TestCORSHeaders(t *testing.T) {
	handler := newTestHandler()
	ctx := context.Background()

	req := events.APIGatewayProxyRequest{
		HTTPMethod: "OPTIONS",
		Path:       "/projects",
	}
	resp, err := handler.HandleRequest(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("got status %d, want 200", resp.StatusCode)
	}
	if resp.Headers["Access-Control-Allow-Origin"] != "https://indifferent.fun" {
		t.Error("missing CORS Allow-Origin header")
	}
	if resp.Headers["Access-Control-Allow-Methods"] == "" {
		t.Error("missing CORS Allow-Methods header")
	}
}
