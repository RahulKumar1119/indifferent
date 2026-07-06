package renderer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/rahul/indifferent/backend/internal/models"
)

// mockHandlerStorage implements StorageClient for handler testing.
type mockHandlerStorage struct {
	objects     map[string][]byte
	putKeys     []string
	deletedKeys []string
	getFunc     func(ctx context.Context, bucket, key string) ([]byte, error)
	putFunc     func(ctx context.Context, bucket, key string, data []byte, contentType string) error
	deleteFunc  func(ctx context.Context, bucket, key string) error
}

func newMockHandlerStorage() *mockHandlerStorage {
	return &mockHandlerStorage{
		objects: make(map[string][]byte),
	}
}

func (m *mockHandlerStorage) GetObject(ctx context.Context, bucket, key string) ([]byte, error) {
	if m.getFunc != nil {
		return m.getFunc(ctx, bucket, key)
	}
	data, ok := m.objects[key]
	if !ok {
		return nil, fmt.Errorf("object not found: %s", key)
	}
	return data, nil
}

func (m *mockHandlerStorage) PutObject(ctx context.Context, bucket, key string, data []byte, contentType string) error {
	if m.putFunc != nil {
		return m.putFunc(ctx, bucket, key, data, contentType)
	}
	m.objects[key] = data
	m.putKeys = append(m.putKeys, key)
	return nil
}

func (m *mockHandlerStorage) DeleteObject(ctx context.Context, bucket, key string) error {
	if m.deleteFunc != nil {
		return m.deleteFunc(ctx, bucket, key)
	}
	delete(m.objects, key)
	m.deletedKeys = append(m.deletedKeys, key)
	return nil
}

// mockHandlerCompositor implements CompositorInterface for handler testing.
type mockHandlerCompositor struct {
	composeFunc func(ctx context.Context, workDir string, slideFiles, audioFiles []string) (string, error)
	callCount   int
}

func (m *mockHandlerCompositor) ComposeVideo(ctx context.Context, workDir string, slideFiles, audioFiles []string) (string, error) {
	m.callCount++
	if m.composeFunc != nil {
		return m.composeFunc(ctx, workDir, slideFiles, audioFiles)
	}
	// Create a fake output video file
	outputPath := filepath.Join(workDir, "output.mp4")
	if err := os.WriteFile(outputPath, []byte("fake-video-data"), 0o644); err != nil {
		return "", err
	}
	return outputPath, nil
}

// mockHandlerThumbnail implements ThumbnailInterface for handler testing.
type mockHandlerThumbnail struct {
	generateFunc func(firstSlide string, outputPath string) error
	callCount    int
}

func (m *mockHandlerThumbnail) GenerateThumbnail(firstSlide string, outputPath string) error {
	m.callCount++
	if m.generateFunc != nil {
		return m.generateFunc(firstSlide, outputPath)
	}
	// Create a fake thumbnail file
	return os.WriteFile(outputPath, []byte("fake-thumbnail-data"), 0o644)
}

func TestHandleRequest_SuccessfulRender(t *testing.T) {
	store := newMockHandlerStorage()
	store.objects["temp/proj1/slides/slide-001.png"] = []byte("slide1-png-data")
	store.objects["temp/proj1/slides/slide-002.png"] = []byte("slide2-png-data")
	store.objects["temp/proj1/audio/audio-001.mp3"] = []byte("audio1-mp3-data")
	store.objects["temp/proj1/audio/audio-002.mp3"] = []byte("audio2-mp3-data")

	compositor := &mockHandlerCompositor{}
	thumbnail := &mockHandlerThumbnail{}

	handler := NewHandler(compositor, thumbnail, store, "test-bucket")

	input := models.RendererInput{
		ProjectID: "proj1",
		SlideKeys: []string{"temp/proj1/slides/slide-001.png", "temp/proj1/slides/slide-002.png"},
		AudioKeys: []string{"temp/proj1/audio/audio-001.mp3", "temp/proj1/audio/audio-002.mp3"},
		JSONKey:   "parsed/proj1/questions.json",
	}

	output, err := handler.HandleRequest(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify output keys
	expectedVideoKey := "output/proj1/video.mp4"
	if output.VideoKey != expectedVideoKey {
		t.Errorf("expected video key '%s', got '%s'", expectedVideoKey, output.VideoKey)
	}

	expectedThumbnailKey := "output/proj1/thumbnail.png"
	if output.ThumbnailKey != expectedThumbnailKey {
		t.Errorf("expected thumbnail key '%s', got '%s'", expectedThumbnailKey, output.ThumbnailKey)
	}

	if output.ProjectID != "proj1" {
		t.Errorf("expected project ID 'proj1', got '%s'", output.ProjectID)
	}

	// Verify compositor was called
	if compositor.callCount != 1 {
		t.Errorf("expected 1 compose call, got %d", compositor.callCount)
	}

	// Verify thumbnail was generated
	if thumbnail.callCount != 1 {
		t.Errorf("expected 1 thumbnail call, got %d", thumbnail.callCount)
	}

	// Verify video and thumbnail were uploaded
	if len(store.putKeys) != 2 {
		t.Fatalf("expected 2 put operations, got %d: %v", len(store.putKeys), store.putKeys)
	}
	if store.putKeys[0] != expectedVideoKey {
		t.Errorf("expected first put key '%s', got '%s'", expectedVideoKey, store.putKeys[0])
	}
	if store.putKeys[1] != expectedThumbnailKey {
		t.Errorf("expected second put key '%s', got '%s'", expectedThumbnailKey, store.putKeys[1])
	}

	// Verify temp S3 objects were cleaned up
	expectedDeleted := 4 // 2 slides + 2 audio
	if len(store.deletedKeys) != expectedDeleted {
		t.Errorf("expected %d deleted keys, got %d: %v", expectedDeleted, len(store.deletedKeys), store.deletedKeys)
	}
}

func TestHandleRequest_S3DownloadFailure(t *testing.T) {
	store := newMockHandlerStorage()
	// No objects stored — download will fail

	compositor := &mockHandlerCompositor{}
	thumbnail := &mockHandlerThumbnail{}

	handler := NewHandler(compositor, thumbnail, store, "test-bucket")

	input := models.RendererInput{
		ProjectID: "proj2",
		SlideKeys: []string{"temp/proj2/slides/slide-001.png"},
		AudioKeys: []string{"temp/proj2/audio/audio-001.mp3"},
		JSONKey:   "parsed/proj2/questions.json",
	}

	_, err := handler.HandleRequest(context.Background(), input)
	if err == nil {
		t.Fatal("expected error for S3 download failure, got nil")
	}

	// Verify compositor was NOT called
	if compositor.callCount != 0 {
		t.Errorf("expected 0 compose calls on download failure, got %d", compositor.callCount)
	}
}

func TestHandleRequest_CompositionFailure(t *testing.T) {
	store := newMockHandlerStorage()
	store.objects["temp/proj3/slides/slide-001.png"] = []byte("slide1-data")
	store.objects["temp/proj3/audio/audio-001.mp3"] = []byte("audio1-data")

	compositor := &mockHandlerCompositor{
		composeFunc: func(ctx context.Context, workDir string, slideFiles, audioFiles []string) (string, error) {
			return "", fmt.Errorf("ffmpeg composition failed")
		},
	}
	thumbnail := &mockHandlerThumbnail{}

	handler := NewHandler(compositor, thumbnail, store, "test-bucket")

	input := models.RendererInput{
		ProjectID: "proj3",
		SlideKeys: []string{"temp/proj3/slides/slide-001.png"},
		AudioKeys: []string{"temp/proj3/audio/audio-001.mp3"},
		JSONKey:   "parsed/proj3/questions.json",
	}

	_, err := handler.HandleRequest(context.Background(), input)
	if err == nil {
		t.Fatal("expected error for composition failure, got nil")
	}

	// Verify thumbnail was NOT generated
	if thumbnail.callCount != 0 {
		t.Errorf("expected 0 thumbnail calls on composition failure, got %d", thumbnail.callCount)
	}

	// Verify no uploads happened
	if len(store.putKeys) != 0 {
		t.Errorf("expected 0 uploads on failure, got %d: %v", len(store.putKeys), store.putKeys)
	}
}

func TestHandleRequest_ThumbnailFailure(t *testing.T) {
	store := newMockHandlerStorage()
	store.objects["temp/proj4/slides/slide-001.png"] = []byte("slide1-data")
	store.objects["temp/proj4/audio/audio-001.mp3"] = []byte("audio1-data")

	compositor := &mockHandlerCompositor{}
	thumbnail := &mockHandlerThumbnail{
		generateFunc: func(firstSlide string, outputPath string) error {
			return fmt.Errorf("ffmpeg thumbnail failed")
		},
	}

	handler := NewHandler(compositor, thumbnail, store, "test-bucket")

	input := models.RendererInput{
		ProjectID: "proj4",
		SlideKeys: []string{"temp/proj4/slides/slide-001.png"},
		AudioKeys: []string{"temp/proj4/audio/audio-001.mp3"},
		JSONKey:   "parsed/proj4/questions.json",
	}

	_, err := handler.HandleRequest(context.Background(), input)
	if err == nil {
		t.Fatal("expected error for thumbnail failure, got nil")
	}

	// Verify compositor was called (it succeeds before thumbnail)
	if compositor.callCount != 1 {
		t.Errorf("expected 1 compose call, got %d", compositor.callCount)
	}
}

func TestHandleRequest_CleanupCalledOnSuccess(t *testing.T) {
	store := newMockHandlerStorage()
	store.objects["temp/proj5/slides/s1.png"] = []byte("s1")
	store.objects["temp/proj5/slides/s2.png"] = []byte("s2")
	store.objects["temp/proj5/slides/s3.png"] = []byte("s3")
	store.objects["temp/proj5/audio/a1.mp3"] = []byte("a1")
	store.objects["temp/proj5/audio/a2.mp3"] = []byte("a2")
	store.objects["temp/proj5/audio/a3.mp3"] = []byte("a3")

	compositor := &mockHandlerCompositor{}
	thumbnail := &mockHandlerThumbnail{}

	handler := NewHandler(compositor, thumbnail, store, "test-bucket")

	input := models.RendererInput{
		ProjectID: "proj5",
		SlideKeys: []string{"temp/proj5/slides/s1.png", "temp/proj5/slides/s2.png", "temp/proj5/slides/s3.png"},
		AudioKeys: []string{"temp/proj5/audio/a1.mp3", "temp/proj5/audio/a2.mp3", "temp/proj5/audio/a3.mp3"},
		JSONKey:   "parsed/proj5/questions.json",
	}

	output, err := handler.HandleRequest(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if output.ProjectID != "proj5" {
		t.Errorf("expected project ID 'proj5', got '%s'", output.ProjectID)
	}

	// Verify all 6 temp objects (3 slides + 3 audio) were deleted
	if len(store.deletedKeys) != 6 {
		t.Errorf("expected 6 deleted keys for cleanup, got %d: %v", len(store.deletedKeys), store.deletedKeys)
	}

	// Verify the correct keys were deleted
	expectedDeleted := map[string]bool{
		"temp/proj5/slides/s1.png": true,
		"temp/proj5/slides/s2.png": true,
		"temp/proj5/slides/s3.png": true,
		"temp/proj5/audio/a1.mp3":  true,
		"temp/proj5/audio/a2.mp3":  true,
		"temp/proj5/audio/a3.mp3":  true,
	}
	for _, key := range store.deletedKeys {
		if !expectedDeleted[key] {
			t.Errorf("unexpected deleted key: %s", key)
		}
	}
}

func TestHandleRequest_CleanupNotCalledOnFailure(t *testing.T) {
	store := newMockHandlerStorage()
	store.objects["temp/proj6/slides/s1.png"] = []byte("s1")
	store.objects["temp/proj6/audio/a1.mp3"] = []byte("a1")

	compositor := &mockHandlerCompositor{
		composeFunc: func(ctx context.Context, workDir string, slideFiles, audioFiles []string) (string, error) {
			return "", fmt.Errorf("composition error")
		},
	}
	thumbnail := &mockHandlerThumbnail{}

	handler := NewHandler(compositor, thumbnail, store, "test-bucket")

	input := models.RendererInput{
		ProjectID: "proj6",
		SlideKeys: []string{"temp/proj6/slides/s1.png"},
		AudioKeys: []string{"temp/proj6/audio/a1.mp3"},
		JSONKey:   "parsed/proj6/questions.json",
	}

	_, err := handler.HandleRequest(context.Background(), input)
	if err == nil {
		t.Fatal("expected error")
	}

	// Verify cleanup was NOT called on failure (error returned before cleanup)
	if len(store.deletedKeys) != 0 {
		t.Errorf("expected no cleanup on failure, got %d deleted keys: %v", len(store.deletedKeys), store.deletedKeys)
	}
}
