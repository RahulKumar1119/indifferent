package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/rahul/indifferent/backend/internal/models"
)

type fakePreviewRenderer struct {
	calls    int
	template string
	payload  models.QuestionSlideData
	err      error
}

func (f *fakePreviewRenderer) RenderTemplate(template string, data interface{}) ([]byte, error) {
	f.calls++
	f.template = template
	f.payload = data.(models.QuestionSlideData)
	if f.err != nil {
		return nil, f.err
	}
	return []byte("fake-png"), nil
}

func previewTestHandler(t *testing.T, project models.Project, s3 *stubDownloader, renderer *fakePreviewRenderer) *APIHandler {
	t.Helper()
	h := newTestHandler()
	item := projectToItem(project)
	h.DB = &mockDynamoDB{
		getItemFunc: func(context.Context, *dynamodb.GetItemInput, ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
			return &dynamodb.GetItemOutput{Item: item}, nil
		},
	}
	h.S3 = s3
	h.PreviewRenderer = renderer
	h.PreviewLimiter = PreviewLimiter{Limit: 100, Period: time.Minute}
	return h
}

func previewProject() models.Project {
	show := true
	return models.Project{
		UserID:    "user1",
		ProjectID: "p1",
		Name:      "Quiz",
		Template:  "classic",
		Voice:     "Joanna",
		Status:    "created",
		DesignOptions: models.DesignOptions{
			Accent:             "#7C3AED",
			TextSize:           "m",
			ShowQuestionNumber: &show,
			AnswerLabels:       "letters",
		},
		CreatedAt: "2026-01-01T00:00:00Z",
		UpdatedAt: "2026-01-01T00:00:00Z",
	}
}

func previewRequest(userID, body string) events.APIGatewayProxyRequest {
	return events.APIGatewayProxyRequest{
		HTTPMethod: "POST",
		Path:       "/projects/p1/slide-preview",
		Headers:    map[string]string{"Authorization": "Bearer " + generateTestToken(userID)},
		Body:       body,
	}
}

func previewBody(question string, options []models.Option) string {
	b, _ := json.Marshal(SlidePreviewRequest{
		Design:   "classic",
		Question: question,
		Options:  options,
	})
	return string(b)
}

func samplePreviewOptions() []models.Option {
	return []models.Option{
		{Label: "A", Text: "First"},
		{Label: "B", Text: "Second"},
		{Label: "C", Text: "Third"},
		{Label: "D", Text: "Fourth"},
	}
}

func TestSlidePreviewRendersPNG(t *testing.T) {
	renderer := &fakePreviewRenderer{}
	h := previewTestHandler(t, previewProject(), &stubDownloader{}, renderer)

	resp, err := h.HandleRequest(context.Background(), previewRequest("user1", previewBody("What is 2+2?", samplePreviewOptions())))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, resp.Body)
	}
	if resp.Headers["Content-Type"] != "image/png" || !resp.IsBase64Encoded {
		t.Errorf("headers = %v, base64 = %v", resp.Headers, resp.IsBase64Encoded)
	}
	png, err := base64.StdEncoding.DecodeString(resp.Body)
	if err != nil || string(png) != "fake-png" {
		t.Errorf("body = %q, err = %v", png, err)
	}
	if renderer.template != "classic/question.html" {
		t.Errorf("template = %q", renderer.template)
	}
	if renderer.payload.QuestionText != "What is 2+2?" || len(renderer.payload.Options) != 4 {
		t.Errorf("payload = %+v", renderer.payload)
	}
	if renderer.payload.DesignOptions.Accent != "#7C3AED" || renderer.payload.DesignOptions.TextSize != "m" {
		t.Errorf("options = %+v", renderer.payload.DesignOptions)
	}
}

func TestSlidePreviewRequiresAuth(t *testing.T) {
	h := previewTestHandler(t, previewProject(), &stubDownloader{}, &fakePreviewRenderer{})
	req := previewRequest("user1", previewBody("Q?", samplePreviewOptions()))
	delete(req.Headers, "Authorization")
	resp, err := h.HandleRequest(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 401 {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}

func TestSlidePreviewRejectsBadInput(t *testing.T) {
	tooLongQuestion := strings.Repeat("q", previewQuestionMaxRunes+1)
	tooLongOption := strings.Repeat("o", previewOptionMaxRunes+1)
	tests := []struct {
		name string
		body string
	}{
		{"invalid json", "{"},
		{"unknown design", `{"design":"fancy","question":"Q?","options":[{"label":"A","text":"A"}]}`},
		{"accent not allowed", `{"design":"classic","designOptions":{"accent":"#FF3DCB"},"question":"Q?","options":[{"label":"A","text":"A"}]}`},
		{"bad text size", `{"designOptions":{"textSize":"xl"},"question":"Q?","options":[{"label":"A","text":"A"}]}`},
		{"question too long", previewBody(tooLongQuestion, samplePreviewOptions())},
		{"too many options", previewBody("Q?", []models.Option{{Text: "1"}, {Text: "2"}, {Text: "3"}, {Text: "4"}, {Text: "5"}, {Text: "6"}, {Text: "7"}})},
		{"option too long", previewBody("Q?", []models.Option{{Text: tooLongOption}})},
		{"empty option", previewBody("Q?", []models.Option{{Text: ""}})},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := previewTestHandler(t, previewProject(), &stubDownloader{}, &fakePreviewRenderer{})
			resp, err := h.HandleRequest(context.Background(), previewRequest("user1", tt.body))
			if err != nil {
				t.Fatal(err)
			}
			if resp.StatusCode != 400 {
				t.Errorf("status = %d, want 400 (body %s)", resp.StatusCode, resp.Body)
			}
		})
	}
}

func TestSlidePreviewRateLimitedPerUser(t *testing.T) {
	h := previewTestHandler(t, previewProject(), &stubDownloader{}, &fakePreviewRenderer{})
	h.PreviewLimiter = PreviewLimiter{Limit: 2, Period: time.Minute}

	body := previewBody("Q?", samplePreviewOptions())
	for i := 0; i < 2; i++ {
		resp, _ := h.HandleRequest(context.Background(), previewRequest("user1", body))
		if resp.StatusCode != 200 {
			t.Fatalf("request %d: status = %d", i+1, resp.StatusCode)
		}
	}
	resp, _ := h.HandleRequest(context.Background(), previewRequest("user1", body))
	if resp.StatusCode != 429 {
		t.Fatalf("third request: status = %d, want 429", resp.StatusCode)
	}
	if resp.Headers["Retry-After"] == "" {
		t.Error("429 response has no Retry-After")
	}
	// Another user has a separate window (the mock DB deliberately ignores
	// ownership, so the second user sees the same project).
	resp, _ = h.HandleRequest(context.Background(), previewRequest("user2", body))
	if resp.StatusCode != 200 {
		t.Errorf("other user status = %d, want 200", resp.StatusCode)
	}
}

func TestSlidePreviewCacheAvoidsRerender(t *testing.T) {
	renderer := &fakePreviewRenderer{}
	h := previewTestHandler(t, previewProject(), &stubDownloader{}, renderer)
	body := previewBody("Q?", samplePreviewOptions())
	for i := 0; i < 2; i++ {
		resp, _ := h.HandleRequest(context.Background(), previewRequest("user1", body))
		if resp.StatusCode != 200 {
			t.Fatalf("request %d: status = %d", i+1, resp.StatusCode)
		}
	}
	if renderer.calls != 1 {
		t.Errorf("renderer calls = %d, want 1 (cached)", renderer.calls)
	}
}

func TestSlidePreviewUsesFirstQuestionFromTXT(t *testing.T) {
	txt := `1. What is the capital of France?
A. London
B. Paris
C. Berlin
D. Madrid
Answer: B`
	project := previewProject()
	project.TxtKey = "uploads/user1/p1/input.txt"
	s3 := &stubDownloader{objects: map[string][]byte{project.TxtKey: []byte(txt)}}
	renderer := &fakePreviewRenderer{}
	h := previewTestHandler(t, project, s3, renderer)

	resp, _ := h.HandleRequest(context.Background(), previewRequest("user1", `{"design":"modern"}`))
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, resp.Body)
	}
	if renderer.payload.QuestionText != "What is the capital of France?" {
		t.Errorf("question = %q", renderer.payload.QuestionText)
	}
	if renderer.payload.CorrectIndex != 1 || renderer.template != "modern/question.html" {
		t.Errorf("payload = %+v, template = %q", renderer.payload, renderer.template)
	}
}

func TestSlidePreviewWithoutTXT(t *testing.T) {
	h := previewTestHandler(t, previewProject(), &stubDownloader{}, &fakePreviewRenderer{})
	resp, _ := h.HandleRequest(context.Background(), previewRequest("user1", `{}`))
	if resp.StatusCode != 422 {
		t.Fatalf("status = %d, want 422", resp.StatusCode)
	}
}

func TestPreviewCacheEvictsOldest(t *testing.T) {
	var c PreviewCache
	c.Max = 2
	c.set("a", []byte("1"))
	c.set("b", []byte("2"))
	c.set("c", []byte("3"))
	if _, ok := c.get("a"); ok {
		t.Error("oldest cache entry should be evicted")
	}
	if _, ok := c.get("c"); !ok {
		t.Error("newest cache entry missing")
	}
}

func TestSlidePreviewKeepsLegacyProjectsAtBaseType(t *testing.T) {
	project := previewProject()
	project.DesignOptions = models.DesignOptions{}
	renderer := &fakePreviewRenderer{}
	h := previewTestHandler(t, project, &stubDownloader{}, renderer)

	resp, err := h.HandleRequest(context.Background(), previewRequest("user1", previewBody("What is 2+2?", samplePreviewOptions())))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, resp.Body)
	}
	payload := renderer.payload
	if payload.DesignOptions.TextSize != "" || payload.DesignOptions.AnswerLabels != "" || payload.DesignOptions.ShowQuestionNumber != nil {
		t.Errorf("legacy preview options = %+v, want empty fields at base type", payload.DesignOptions)
	}
}

func TestSlidePreviewRejectsOutOfRangeCorrectIndex(t *testing.T) {
	renderer := &fakePreviewRenderer{}
	h := previewTestHandler(t, previewProject(), &stubDownloader{}, renderer)
	body := previewBody("What is 2+2?", samplePreviewOptions())
	var req SlidePreviewRequest
	if err := json.Unmarshal([]byte(body), &req); err != nil {
		t.Fatal(err)
	}
	bad := 9
	req.CorrectIndex = &bad
	badBody, _ := json.Marshal(req)

	resp, err := h.HandleRequest(context.Background(), previewRequest("user1", string(badBody)))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 400 {
		t.Fatalf("status = %d, want 400; body = %s", resp.StatusCode, resp.Body)
	}
	if renderer.calls != 0 {
		t.Errorf("renderer called %d times for invalid input", renderer.calls)
	}
}

func TestSlidePreviewParsesTextBody(t *testing.T) {
	renderer := &fakePreviewRenderer{}
	h := previewTestHandler(t, previewProject(), &stubDownloader{}, renderer)
	text := `1. What colour is the sky?

A. Green
B. Blue
C. Red
D. Yellow

Answer: B`
	body, _ := json.Marshal(SlidePreviewRequest{Text: text})

	resp, err := h.HandleRequest(context.Background(), previewRequest("user1", string(body)))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, resp.Body)
	}
	if renderer.payload.QuestionText != "What colour is the sky?" {
		t.Errorf("question = %q", renderer.payload.QuestionText)
	}
	if len(renderer.payload.Options) != 4 || renderer.payload.CorrectIndex != 1 {
		t.Errorf("options = %+v, correct = %d", renderer.payload.Options, renderer.payload.CorrectIndex)
	}
}
