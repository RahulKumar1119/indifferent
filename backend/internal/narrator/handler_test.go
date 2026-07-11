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
	synthesizeFunc     func(ctx context.Context, question models.Question, voiceID string) ([]byte, error)
	synthesizeTextFunc func(ctx context.Context, text string, voiceID string) ([]byte, error)
	callCount          int
	textCallCount      int
}

func (m *mockNarrationService) Synthesize(ctx context.Context, question models.Question, voiceID string) ([]byte, error) {
	m.callCount++
	if m.synthesizeFunc != nil {
		return m.synthesizeFunc(ctx, question, voiceID)
	}
	return []byte("fake-mp3-audio-data"), nil
}

func (m *mockNarrationService) SynthesizeText(ctx context.Context, text string, voiceID string) ([]byte, error) {
	m.textCallCount++
	if m.synthesizeTextFunc != nil {
		return m.synthesizeTextFunc(ctx, text, voiceID)
	}
	return []byte("fake-answer-audio-data"), nil
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

	// Now produces 2 audio keys per question: question + answer
	if len(output.AudioKeys) != 2 {
		t.Fatalf("expected 2 audio keys, got %d: %v", len(output.AudioKeys), output.AudioKeys)
	}

	expectedQuestionKey := "temp/proj1/audio/q0.mp3"
	if output.AudioKeys[0] != expectedQuestionKey {
		t.Errorf("expected question audio key '%s', got '%s'", expectedQuestionKey, output.AudioKeys[0])
	}

	expectedAnswerKey := "temp/proj1/audio/q0_answer.mp3"
	if output.AudioKeys[1] != expectedAnswerKey {
		t.Errorf("expected answer audio key '%s', got '%s'", expectedAnswerKey, output.AudioKeys[1])
	}

	if len(output.Failed) != 0 {
		t.Errorf("expected no failed indices, got %v", output.Failed)
	}

	if narration.callCount != 1 {
		t.Errorf("expected 1 synthesize call, got %d", narration.callCount)
	}
	if narration.textCallCount != 1 {
		t.Errorf("expected 1 synthesizeText call, got %d", narration.textCallCount)
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

	// 3 questions × 2 audio files = 6 audio keys (interleaved)
	if len(output.AudioKeys) != 6 {
		t.Fatalf("expected 6 audio keys, got %d: %v", len(output.AudioKeys), output.AudioKeys)
	}

	expectedKeys := []string{
		"temp/proj2/audio/q0.mp3",
		"temp/proj2/audio/q0_answer.mp3",
		"temp/proj2/audio/q1.mp3",
		"temp/proj2/audio/q1_answer.mp3",
		"temp/proj2/audio/q2.mp3",
		"temp/proj2/audio/q2_answer.mp3",
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
	if narration.textCallCount != 3 {
		t.Errorf("expected 3 synthesizeText calls, got %d", narration.textCallCount)
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

	// Questions 0 and 2 succeed (2 audio keys each = 4 keys), question 1 fails
	if len(output.AudioKeys) != 4 {
		t.Errorf("expected 4 audio keys, got %d: %v", len(output.AudioKeys), output.AudioKeys)
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

	// Fail upload for the first PutObject call (question 0's audio)
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

	// Question 0 fails on first upload, question 1 succeeds (2 keys for q1)
	if len(output.AudioKeys) != 2 {
		t.Errorf("expected 2 audio keys, got %d: %v", len(output.AudioKeys), output.AudioKeys)
	}

	if len(output.Failed) != 1 || output.Failed[0] != 0 {
		t.Errorf("expected failed=[0], got %v", output.Failed)
	}
}
