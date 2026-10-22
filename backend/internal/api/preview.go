package api

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/aws/aws-lambda-go/events"
	"github.com/rahul/indifferent/backend/internal/models"
	"github.com/rahul/indifferent/backend/internal/parser"
)

const (
	previewQuestionMaxRunes = 500
	previewOptionMaxRunes   = 200
	previewMaxOptions       = 6
	previewDefaultLimit     = 10
	previewDefaultPeriod    = time.Minute
	previewCacheEntries     = 64
)

// SlidePreviewRenderer is the renderer surface the API needs. The production
// implementation is slidegen.NativeRenderer, but keeping it abstract means the
// API does not import the Playwright-backed slidegen package.
type SlidePreviewRenderer interface {
	RenderTemplate(templateName string, data interface{}) ([]byte, error)
}

type SlidePreviewRequest struct {
	Design        string                `json:"design,omitempty"`
	DesignOptions *models.DesignOptions `json:"designOptions,omitempty"`
	Question      string                `json:"question,omitempty"`
	Options       []models.Option       `json:"options,omitempty"`
	Text          string                `json:"text,omitempty"`
	CorrectIndex  *int                  `json:"correctIndex,omitempty"`
}

type previewWindow struct {
	start time.Time
	count int
}

// PreviewLimiter is a per-user fixed window. The zero value uses 10 requests
// per minute. It lives on the APIHandler so tests can shorten the window.
type PreviewLimiter struct {
	mu      sync.Mutex
	windows map[string]previewWindow
	Limit   int
	Period  time.Duration
	Now     func() time.Time
}

func (l *PreviewLimiter) allow(userID string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	limit := l.Limit
	if limit <= 0 {
		limit = previewDefaultLimit
	}
	period := l.Period
	if period <= 0 {
		period = previewDefaultPeriod
	}
	now := time.Now()
	if l.Now != nil {
		now = l.Now()
	}
	if l.windows == nil {
		l.windows = make(map[string]previewWindow)
	}
	w := l.windows[userID]
	if w.start.IsZero() || now.Sub(w.start) >= period {
		w = previewWindow{start: now}
	}
	if w.count >= limit {
		return false, period - now.Sub(w.start)
	}
	w.count++
	l.windows[userID] = w
	return true, 0
}

// PreviewCache is a tiny in-memory LRU-ish cache keyed by the content hash of
// a preview request. The Lambda instance is short-lived, so a bounded map is
// enough to absorb picker clicks while a user compares designs.
type PreviewCache struct {
	mu      sync.Mutex
	entries map[string][]byte
	order   []string
	Max     int
}

func (c *PreviewCache) get(key string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	png, ok := c.entries[key]
	return png, ok
}

func (c *PreviewCache) set(key string, png []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.entries == nil {
		c.entries = make(map[string][]byte)
	}
	max := c.Max
	if max <= 0 {
		max = previewCacheEntries
	}
	if _, exists := c.entries[key]; !exists {
		c.order = append(c.order, key)
	}
	c.entries[key] = png
	for len(c.order) > max {
		delete(c.entries, c.order[0])
		c.order = c.order[1:]
	}
}

// handleSlidePreview renders the first question of the project's TXT in the
// requested design. It is authenticated by the router, validates the design
// and options against the allow-lists, caps text, rate-limits per user, and
// caches identical renders by content hash.
func (h *APIHandler) handleSlidePreview(ctx context.Context, req events.APIGatewayProxyRequest, claims *models.JWTClaims) (events.APIGatewayProxyResponse, error) {
	projectID := extractProjectIDFromSubpath(req.Path, "/slide-preview")
	if projectID == "" {
		return errorResponse(http.StatusBadRequest, "INVALID_INPUT", "Project ID is required"), nil
	}

	project, err := h.getProjectByID(ctx, claims.UserID, projectID)
	if err != nil {
		return errorResponse(http.StatusNotFound, "NOT_FOUND", "Project not found"), nil
	}

	allowed, retryAfter := h.PreviewLimiter.allow(claims.UserID)
	if !allowed {
		resp := errorResponse(http.StatusTooManyRequests, "RATE_LIMITED", "Too many slide previews. Try again shortly.")
		resp.Headers["Retry-After"] = fmt.Sprintf("%d", maxInt(1, int(retryAfter.Seconds())))
		return resp, nil
	}

	var body SlidePreviewRequest
	if strings.TrimSpace(req.Body) != "" {
		if err := json.Unmarshal([]byte(req.Body), &body); err != nil {
			return errorResponse(http.StatusBadRequest, "INVALID_BODY", "Invalid request body"), nil
		}
	}

	design := strings.ToLower(strings.TrimSpace(body.Design))
	if design == "" {
		design = project.Template
	}
	if !validTemplates[design] {
		return errorResponse(http.StatusBadRequest, "VALIDATION_ERROR", "Invalid design"), nil
	}

	legacyOptions := project.DesignOptions.IsZero()
	opts := project.DesignOptions
	if body.DesignOptions != nil {
		mergeDesignOptions(&opts, *body.DesignOptions)
	}
	// Accents are curated per design. Previewing another design without
	// choosing its accent should use that design's default, not fail because
	// the project's accent belongs to the stored design.
	if design != project.Template && (body.DesignOptions == nil || body.DesignOptions.Accent == "") {
		opts.Accent = ""
	}
	if err := models.ValidateDesignOptions(design, &opts); err != nil {
		return errorResponse(http.StatusBadRequest, "VALIDATION_ERROR", err.Error()), nil
	}
	// Projects created before design options existed must preview at their
	// real base size, not at the new M default. Explicit body overrides still
	// win.
	if legacyOptions {
		if body.DesignOptions == nil || body.DesignOptions.TextSize == "" {
			opts.TextSize = ""
		}
		if body.DesignOptions == nil || body.DesignOptions.AnswerLabels == "" {
			opts.AnswerLabels = ""
		}
		if body.DesignOptions == nil || body.DesignOptions.ShowQuestionNumber == nil {
			opts.ShowQuestionNumber = nil
		}
	}

	question, options, correctIndex, err := h.previewQuestion(ctx, project, body, claims.UserID)
	if err != nil {
		return errorResponse(http.StatusUnprocessableEntity, "PREVIEW_SOURCE", err.Error()), nil
	}
	if err := validatePreviewContent(question, options); err != nil {
		return errorResponse(http.StatusBadRequest, "VALIDATION_ERROR", err.Error()), nil
	}
	if correctIndex < 0 || correctIndex >= len(options) {
		return errorResponse(http.StatusBadRequest, "VALIDATION_ERROR", "correctIndex is out of range"), nil
	}
	if len(body.Text) > 5*1024*1024 {
		return errorResponse(http.StatusBadRequest, "VALIDATION_ERROR", "TXT file must be 5MB or less"), nil
	}

	payload := models.QuestionSlideData{
		QuestionNumber: 1,
		TotalQuestions: 1,
		QuestionText:   question,
		Options:        options,
		CorrectIndex:   correctIndex,
		CorrectIndices: []int{correctIndex},
		DesignOptions:  opts,
	}
	key := previewCacheKey(design, payload)
	if png, ok := h.PreviewCache.get(key); ok {
		return pngResponse(png), nil
	}
	if h.PreviewRenderer == nil {
		return errorResponse(http.StatusServiceUnavailable, "PREVIEW_UNAVAILABLE", "Slide preview is not configured"), nil
	}

	png, err := h.PreviewRenderer.RenderTemplate(design+"/question.html", payload)
	if err != nil {
		return errorResponse(http.StatusInternalServerError, "RENDER_ERROR", "Failed to render slide preview"), nil
	}
	h.PreviewCache.set(key, png)
	return pngResponse(png), nil
}

func (h *APIHandler) previewQuestion(ctx context.Context, project *models.Project, body SlidePreviewRequest, userID string) (string, []models.Option, int, error) {
	if strings.TrimSpace(body.Question) != "" {
		correct := 0
		if body.CorrectIndex != nil {
			correct = *body.CorrectIndex
		}
		return strings.TrimSpace(body.Question), body.Options, correct, nil
	}
	if strings.TrimSpace(body.Text) != "" {
		return parseFirstQuestion(body.Text)
	}

	key := project.TxtKey
	if key == "" {
		key = fmt.Sprintf("uploads/%s/%s/input.txt", userID, project.ProjectID)
	}
	data, err := h.S3.GetObject(ctx, h.Bucket, key)
	if err != nil {
		return "", nil, 0, fmt.Errorf("upload a TXT file before previewing a slide")
	}
	return parseFirstQuestion(string(data))
}

// parseFirstQuestion runs the production parser over a TXT payload, so the
// upload screen can preview a file before starting the pipeline.
func parseFirstQuestion(content string) (string, []models.Option, int, error) {
	format, err := parser.DetectFormat(content)
	if err != nil {
		return "", nil, 0, fmt.Errorf("could not detect the question format in this TXT file")
	}
	questions := parser.ExtractQuestions(content, string(format))
	if len(questions) == 0 {
		return "", nil, 0, fmt.Errorf("no questions could be read from this TXT file")
	}
	q := questions[0]
	return q.Text, q.Options, q.CorrectIndex, nil
}

func validatePreviewContent(question string, options []models.Option) error {
	if strings.TrimSpace(question) == "" {
		return fmt.Errorf("question text is required")
	}
	if utf8.RuneCountInString(question) > previewQuestionMaxRunes {
		return fmt.Errorf("question must be %d characters or less", previewQuestionMaxRunes)
	}
	if len(options) == 0 {
		return fmt.Errorf("at least one option is required")
	}
	if len(options) > previewMaxOptions {
		return fmt.Errorf("at most %d options are allowed", previewMaxOptions)
	}
	for i, opt := range options {
		if strings.TrimSpace(opt.Text) == "" {
			return fmt.Errorf("option %d text is required", i+1)
		}
		if utf8.RuneCountInString(opt.Text) > previewOptionMaxRunes {
			return fmt.Errorf("option %d must be %d characters or less", i+1, previewOptionMaxRunes)
		}
	}
	return nil
}

func mergeDesignOptions(dst *models.DesignOptions, src models.DesignOptions) {
	if src.Accent != "" {
		dst.Accent = src.Accent
	}
	if src.TextSize != "" {
		dst.TextSize = src.TextSize
	}
	if src.ShowQuestionNumber != nil {
		dst.ShowQuestionNumber = src.ShowQuestionNumber
	}
	if src.AnswerLabels != "" {
		dst.AnswerLabels = src.AnswerLabels
	}
}

func previewCacheKey(design string, payload models.QuestionSlideData) string {
	raw, _ := json.Marshal(struct {
		Design  string
		Payload models.QuestionSlideData
	}{design, payload})
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func pngResponse(png []byte) events.APIGatewayProxyResponse {
	headers := corsHeaders()
	headers["Content-Type"] = "image/png"
	headers["Cache-Control"] = "private, max-age=60"
	return events.APIGatewayProxyResponse{
		StatusCode:      http.StatusOK,
		Headers:         headers,
		Body:            base64.StdEncoding.EncodeToString(png),
		IsBase64Encoded: true,
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
