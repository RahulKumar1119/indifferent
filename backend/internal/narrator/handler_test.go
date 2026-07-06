package narrator

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/rahul/indifferent/backend/internal/models"
)

// mockStorage implements StorageClient for testing.
type mockStorage struct {
	objects map[string][]byte
	putKeys []string
	getFunc func(ctx context.Context, bucket, key string) ([]byte, error)
	putFunc func(ctx context.Context, bucket, key string, data []byte, contentType string) error
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

// mockNarrationService implements NarrationService for testing.
type mockNarrationService struct {
	synthesizeFunc func(ctx context.Context, question models.Question, voiceID string) ([]byte, error)
	callCount      int
}

func (m *mockNarrationService) Synthesize(ctx context.Context, question models.Question, voiceID string) ([]byte, error) {
	m.callCount++
	if m.synthesizeFunc != nil {
		return m.synthesizeFunc(ctx, question, voiceID)
	}
	return []byte("fake-mp3-audio-data"), nil
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

	narration := &mockNarrationService{}
	handler := NewHandler(narration, store, "test-bucket")

	input := models.NarratorInput{
		ProjectID: "proj1",
		JSONKey:   "parsed/proj1/questions.json",
		Voice:     "Joanna",
	}

	output, err := handler.HandleRequest(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if output.ProjectID != "proj1" {
		t.Errorf("expected ProjectID 'proj1', got '%s'", output.ProjectID)
	}

	if len(output.AudioKeys) != 1 {
		t.Fatalf("expected 1 audio key, got %d", len(output.AudioKeys))
	}

	expectedKey := "temp/proj1/audio/q0.mp3"
	if output.AudioKeys[0] != expectedKey {
		t.Errorf("expected audio key '%s', got '%s'", expectedKey, output.AudioKeys[0])
	}

	if len(output.Failed) != 0 {
		t.Errorf("expected no failed indices, got %v", output.Failed)
	}

	if narration.callCount != 1 {
		t.Errorf("expected 1 synthesize call, got %d", narration.callCount)
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

	narration := &mockNarrationService{}
	handler := NewHandler(narration, store, "test-bucket")

	input := models.NarratorInput{
		ProjectID: "proj2",
		JSONKey:   "parsed/proj2/questions.json",
		Voice:     "Matthew",
	}

	output, err := handler.HandleRequest(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(output.AudioKeys) != 3 {
		t.Fatalf("expected 3 audio keys, got %d: %v", len(output.AudioKeys), output.AudioKeys)
	}

	expectedKeys := []string{
		"temp/proj2/audio/q0.mp3",
		"temp/proj2/audio/q1.mp3",
		"temp/proj2/audio/q2.mp3",
	}
	for i, expected := range expectedKeys {
		if output.AudioKeys[i] != expected {
			t.Errorf("audio key[%d]: expected '%s', got '%s'", i, expected, output.AudioKeys[i])
		}
	}

	if len(output.Failed) != 0 {
		t.Errorf("expected no failed indices, got %v", output.Failed)
	}

	if narration.callCount != 3 {
		t.Errorf("expected 3 synthesize calls, got %d", narration.callCount)
	}
}

func TestHandleRequest_SynthesisFailure(t *testing.T) {
	questions := []models.Question{
		{Index: 0, Text: "Q1?", Options: []models.Option{{Label: "A", Text: "A1"}}, CorrectIndex: 0},
		{Index: 1, Text: "Q2?", Options: []models.Option{{Label: "A", Text: "A2"}}, CorrectIndex: 0},
		{Index: 2, Text: "Q3?", Options: []models.Option{{Label: "A", Text: "A3"}}, CorrectIndex: 0},
	}

	jsonData, _ := json.Marshal(questions)

	store := newMockStorage()
	store.objects["parsed/proj3/questions.json"] = jsonData

	// Fail on question index 1
	narration := &mockNarrationService{
		synthesizeFunc: func(ctx context.Context, question models.Question, voiceID string) ([]byte, error) {
			if question.Index == 1 {
				return nil, fmt.Errorf("polly synthesis error")
			}
			return []byte("fake-mp3-audio-data"), nil
		},
	}

	handler := NewHandler(narration, store, "test-bucket")

	input := models.NarratorInput{
		ProjectID: "proj3",
		JSONKey:   "parsed/proj3/questions.json",
		Voice:     "Joanna",
	}

	output, err := handler.HandleRequest(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Questions 0 and 2 should succeed
	if len(output.AudioKeys) != 2 {
		t.Errorf("expected 2 audio keys, got %d: %v", len(output.AudioKeys), output.AudioKeys)
	}

	// Question 1 should be in failed
	if len(output.Failed) != 1 || output.Failed[0] != 1 {
		t.Errorf("expected failed=[1], got %v", output.Failed)
	}
}

func TestHandleRequest_S3ReadFailure(t *testing.T) {
	store := newMockStorage()
	// Don't add any object — GetObject will fail

	narration := &mockNarrationService{}
	handler := NewHandler(narration, store, "test-bucket")

	input := models.NarratorInput{
		ProjectID: "proj4",
		JSONKey:   "parsed/proj4/questions.json",
		Voice:     "Joanna",
	}

	_, err := handler.HandleRequest(context.Background(), input)
	if err == nil {
		t.Fatal("expected error for S3 read failure, got nil")
	}
}

func TestHandleRequest_InvalidJSON(t *testing.T) {
	store := newMockStorage()
	store.objects["parsed/proj5/questions.json"] = []byte("not valid json")

	narration := &mockNarrationService{}
	handler := NewHandler(narration, store, "test-bucket")

	input := models.NarratorInput{
		ProjectID: "proj5",
		JSONKey:   "parsed/proj5/questions.json",
		Voice:     "Joanna",
	}

	_, err := handler.HandleRequest(context.Background(), input)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestHandleRequest_S3UploadFailure(t *testing.T) {
	questions := []models.Question{
		{Index: 0, Text: "Q1?", Options: []models.Option{{Label: "A", Text: "A1"}}, CorrectIndex: 0},
		{Index: 1, Text: "Q2?", Options: []models.Option{{Label: "A", Text: "A2"}}, CorrectIndex: 0},
	}

	jsonData, _ := json.Marshal(questions)

	store := newMockStorage()
	store.objects["parsed/proj6/questions.json"] = jsonData

	// Fail upload for question 0
	uploadCount := 0
	store.putFunc = func(ctx context.Context, bucket, key string, data []byte, contentType string) error {
		uploadCount++
		if uploadCount == 1 {
			return fmt.Errorf("S3 upload error")
		}
		return nil
	}

	narration := &mockNarrationService{}
	handler := NewHandler(narration, store, "test-bucket")

	input := models.NarratorInput{
		ProjectID: "proj6",
		JSONKey:   "parsed/proj6/questions.json",
		Voice:     "Joanna",
	}

	output, err := handler.HandleRequest(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Question 0 should fail due to upload error, question 1 should succeed
	if len(output.AudioKeys) != 1 {
		t.Errorf("expected 1 audio key, got %d: %v", len(output.AudioKeys), output.AudioKeys)
	}

	if len(output.Failed) != 1 || output.Failed[0] != 0 {
		t.Errorf("expected failed=[0], got %v", output.Failed)
	}
}
