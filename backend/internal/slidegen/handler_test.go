package slidegen

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/rahul/indifferent/backend/internal/models"
)

// mockRenderer implements SlideRendererInterface for testing.
type mockRenderer struct {
	renderFunc func(templateName string, data interface{}) ([]byte, error)
	callCount  int
}

func (m *mockRenderer) RenderTemplate(templateName string, data interface{}) ([]byte, error) {
	m.callCount++
	if m.renderFunc != nil {
		return m.renderFunc(templateName, data)
	}
	return []byte("fake-png-data"), nil
}

// mockStorage implements StorageClient for testing.
type mockStorage struct {
	objects    map[string][]byte
	putKeys    []string
	getFunc    func(ctx context.Context, bucket, key string) ([]byte, error)
	putFunc    func(ctx context.Context, bucket, key string, data []byte, contentType string) error
}

func newMockStorage() *mockStorage {
	return &mockStorage{
		objects: make(map[string][]byte),
	}
}

func (m *mockStorage) GetObject(ctx context.Context, bucket, key string) ([]byte, error) {
	if m.getFunc != nil {
		return m.getFunc(ctx, bucket, key)
	}
	data, ok := m.objects[key]
	if !ok {
		return nil, fmt.Errorf("object not found: %s", key)
	}
	return data, nil
}

func (m *mockStorage) PutObject(ctx context.Context, bucket, key string, data []byte, contentType string) error {
	if m.putFunc != nil {
		return m.putFunc(ctx, bucket, key, data, contentType)
	}
	m.objects[key] = data
	m.putKeys = append(m.putKeys, key)
	return nil
}

func TestHandleRequest_SingleQuestion(t *testing.T) {
	questions := []models.Question{
		{
			Index:        0,
			Text:         "What is the capital of France?",
			Options:      []models.Option{{Label: "A", Text: "London"}, {Label: "B", Text: "Paris"}, {Label: "C", Text: "Berlin"}, {Label: "D", Text: "Madrid"}},
			CorrectIndex: 1,
		},
	}

	jsonData, _ := json.Marshal(questions)

	store := newMockStorage()
	store.objects["parsed/proj1/questions.json"] = jsonData

	renderer := &mockRenderer{}

	handler := NewHandler(renderer, store, "test-bucket")

	input := models.SlideGenInput{
		ProjectID: "proj1",
		JSONKey:   "parsed/proj1/questions.json",
		Template:  "classic",
	}

	output, err := handler.HandleRequest(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if output.ProjectID != "proj1" {
		t.Errorf("expected ProjectID 'proj1', got '%s'", output.ProjectID)
	}

	// Expected slides: 1 question + 1 answer = 2
	expectedSlideCount := 2
	if len(output.SlideKeys) != expectedSlideCount {
		t.Errorf("expected %d slide keys, got %d: %v", expectedSlideCount, len(output.SlideKeys), output.SlideKeys)
	}

	if len(output.Failed) != 0 {
		t.Errorf("expected no failed indices, got %v", output.Failed)
	}

	// Verify slide key patterns
	expectedKeys := []string{
		"temp/proj1/slides/q0_question.png",
		"temp/proj1/slides/q0_answer.png",
	}

	for i, expected := range expectedKeys {
		if i >= len(output.SlideKeys) {
			t.Errorf("missing slide key at index %d: expected %s", i, expected)
			continue
		}
		if output.SlideKeys[i] != expected {
			t.Errorf("slide key[%d]: expected %s, got %s", i, expected, output.SlideKeys[i])
		}
	}
}

func TestHandleRequest_MultipleQuestions(t *testing.T) {
	questions := []models.Question{
		{Index: 0, Text: "Q1?", Options: []models.Option{{Label: "A", Text: "A1"}}, CorrectIndex: 0},
		{Index: 1, Text: "Q2?", Options: []models.Option{{Label: "A", Text: "A2"}}, CorrectIndex: 0},
		{Index: 2, Text: "Q3?", Options: []models.Option{{Label: "A", Text: "A3"}}, CorrectIndex: 0},
	}

	jsonData, _ := json.Marshal(questions)

	store := newMockStorage()
	store.objects["parsed/proj2/questions.json"] = jsonData

	renderer := &mockRenderer{}
	handler := NewHandler(renderer, store, "test-bucket")

	input := models.SlideGenInput{
		ProjectID: "proj2",
		JSONKey:   "parsed/proj2/questions.json",
		Template:  "classic",
	}

	output, err := handler.HandleRequest(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 3 questions × (1 question + 1 answer) = 6
	expectedSlideCount := 6
	if len(output.SlideKeys) != expectedSlideCount {
		t.Errorf("expected %d slide keys, got %d", expectedSlideCount, len(output.SlideKeys))
	}

	if len(output.Failed) != 0 {
		t.Errorf("expected no failed indices, got %v", output.Failed)
	}
}

func TestHandleRequest_RenderFailure(t *testing.T) {
	questions := []models.Question{
		{Index: 0, Text: "Q1?", Options: []models.Option{{Label: "A", Text: "A1"}}, CorrectIndex: 0},
		{Index: 1, Text: "Q2?", Options: []models.Option{{Label: "A", Text: "A2"}}, CorrectIndex: 0},
	}

	jsonData, _ := json.Marshal(questions)

	store := newMockStorage()
	store.objects["parsed/proj3/questions.json"] = jsonData

	// Fail on question index 1's question slide
	renderer := &mockRenderer{
		renderFunc: func(templateName string, data interface{}) ([]byte, error) {
			if qData, ok := data.(QuestionSlideData); ok && qData.QuestionNumber == 2 {
				return nil, fmt.Errorf("simulated render failure")
			}
			return []byte("fake-png"), nil
		},
	}

	handler := NewHandler(renderer, store, "test-bucket")

	input := models.SlideGenInput{
		ProjectID: "proj3",
		JSONKey:   "parsed/proj3/questions.json",
		Template:  "classic",
	}

	output, err := handler.HandleRequest(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Question 0 should succeed (2 slides), question 1 should fail, no outro
	// q0: 1 question + 1 answer = 2
	expectedSlideCount := 2
	if len(output.SlideKeys) != expectedSlideCount {
		t.Errorf("expected %d slide keys, got %d: %v", expectedSlideCount, len(output.SlideKeys), output.SlideKeys)
	}

	if len(output.Failed) != 1 || output.Failed[0] != 1 {
		t.Errorf("expected failed=[1], got %v", output.Failed)
	}
}

func TestHandleRequest_InvalidJSON(t *testing.T) {
	store := newMockStorage()
	store.objects["parsed/proj4/questions.json"] = []byte("not valid json")

	renderer := &mockRenderer{}
	handler := NewHandler(renderer, store, "test-bucket")

	input := models.SlideGenInput{
		ProjectID: "proj4",
		JSONKey:   "parsed/proj4/questions.json",
		Template:  "classic",
	}

	_, err := handler.HandleRequest(context.Background(), input)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestHandleRequest_S3ReadFailure(t *testing.T) {
	store := newMockStorage()
	// Don't add any object — GetObject will fail

	renderer := &mockRenderer{}
	handler := NewHandler(renderer, store, "test-bucket")

	input := models.SlideGenInput{
		ProjectID: "proj5",
		JSONKey:   "parsed/proj5/questions.json",
		Template:  "classic",
	}

	_, err := handler.HandleRequest(context.Background(), input)
	if err == nil {
		t.Fatal("expected error for S3 read failure, got nil")
	}
}

func TestHandleRequest_AnswerFailureMarksQuestionFailed(t *testing.T) {
	questions := []models.Question{
		{Index: 0, Text: "Q1?", Options: []models.Option{{Label: "A", Text: "A1"}}, CorrectIndex: 0},
	}

	jsonData, _ := json.Marshal(questions)

	store := newMockStorage()
	store.objects["parsed/proj6/questions.json"] = jsonData

	callCount := 0
	renderer := &mockRenderer{
		renderFunc: func(templateName string, data interface{}) ([]byte, error) {
			callCount++
			// Fail on the 2nd render call (answer slide)
			if callCount == 2 {
				return nil, fmt.Errorf("answer render failure")
			}
			return []byte("fake-png"), nil
		},
	}

	handler := NewHandler(renderer, store, "test-bucket")

	input := models.SlideGenInput{
		ProjectID: "proj6",
		JSONKey:   "parsed/proj6/questions.json",
		Template:  "classic",
	}

	output, err := handler.HandleRequest(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Question 0 should be marked as failed due to answer render failure
	if len(output.Failed) != 1 || output.Failed[0] != 0 {
		t.Errorf("expected failed=[0], got %v", output.Failed)
	}
}

