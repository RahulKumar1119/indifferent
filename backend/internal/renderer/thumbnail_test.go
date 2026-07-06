package renderer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// Note: mockStorageClient and putCall are defined in compositor_test.go

func TestUploadResults_ConstructsCorrectKeys(t *testing.T) {
	// Create temporary files to simulate local video and thumbnail
	tmpDir := t.TempDir()
	videoPath := filepath.Join(tmpDir, "video.mp4")
	thumbnailPath := filepath.Join(tmpDir, "thumbnail.png")

	if err := os.WriteFile(videoPath, []byte("fake-video-data"), 0644); err != nil {
		t.Fatalf("failed to create temp video file: %v", err)
	}
	if err := os.WriteFile(thumbnailPath, []byte("fake-thumbnail-data"), 0644); err != nil {
		t.Fatalf("failed to create temp thumbnail file: %v", err)
	}

	mock := newMockStorageClient()
	ctx := context.Background()

	videoKey, thumbnailKey, err := UploadResults(ctx, mock, "test-bucket", "proj-123", videoPath, thumbnailPath)
	if err != nil {
		t.Fatalf("UploadResults returned error: %v", err)
	}

	expectedVideoKey := "output/proj-123/video.mp4"
	expectedThumbnailKey := "output/proj-123/thumbnail.png"

	if videoKey != expectedVideoKey {
		t.Errorf("expected video key %q, got %q", expectedVideoKey, videoKey)
	}
	if thumbnailKey != expectedThumbnailKey {
		t.Errorf("expected thumbnail key %q, got %q", expectedThumbnailKey, thumbnailKey)
	}

	// Verify PutObject was called with correct parameters
	if len(mock.putCalls) != 2 {
		t.Fatalf("expected 2 PutObject calls, got %d", len(mock.putCalls))
	}

	// First call should be video
	if mock.putCalls[0].bucket != "test-bucket" {
		t.Errorf("expected bucket %q, got %q", "test-bucket", mock.putCalls[0].bucket)
	}
	if mock.putCalls[0].key != expectedVideoKey {
		t.Errorf("expected key %q, got %q", expectedVideoKey, mock.putCalls[0].key)
	}
	if mock.putCalls[0].contentType != "video/mp4" {
		t.Errorf("expected content-type %q, got %q", "video/mp4", mock.putCalls[0].contentType)
	}
	if string(mock.putCalls[0].data) != "fake-video-data" {
		t.Errorf("expected video data %q, got %q", "fake-video-data", string(mock.putCalls[0].data))
	}

	// Second call should be thumbnail
	if mock.putCalls[1].bucket != "test-bucket" {
		t.Errorf("expected bucket %q, got %q", "test-bucket", mock.putCalls[1].bucket)
	}
	if mock.putCalls[1].key != expectedThumbnailKey {
		t.Errorf("expected key %q, got %q", expectedThumbnailKey, mock.putCalls[1].key)
	}
	if mock.putCalls[1].contentType != "image/png" {
		t.Errorf("expected content-type %q, got %q", "image/png", mock.putCalls[1].contentType)
	}
	if string(mock.putCalls[1].data) != "fake-thumbnail-data" {
		t.Errorf("expected thumbnail data %q, got %q", "fake-thumbnail-data", string(mock.putCalls[1].data))
	}
}

func TestUploadResults_VideoFileNotFound(t *testing.T) {
	mock := newMockStorageClient()
	ctx := context.Background()

	_, _, err := UploadResults(ctx, mock, "test-bucket", "proj-123", "/nonexistent/video.mp4", "/nonexistent/thumb.png")
	if err == nil {
		t.Fatal("expected error for missing video file, got nil")
	}
}

func TestUploadResults_ThumbnailFileNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	videoPath := filepath.Join(tmpDir, "video.mp4")
	if err := os.WriteFile(videoPath, []byte("data"), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	mock := newMockStorageClient()
	ctx := context.Background()

	_, _, err := UploadResults(ctx, mock, "test-bucket", "proj-123", videoPath, "/nonexistent/thumb.png")
	if err == nil {
		t.Fatal("expected error for missing thumbnail file, got nil")
	}
}

func TestUploadResults_PutObjectError(t *testing.T) {
	tmpDir := t.TempDir()
	videoPath := filepath.Join(tmpDir, "video.mp4")
	thumbnailPath := filepath.Join(tmpDir, "thumbnail.png")
	os.WriteFile(videoPath, []byte("data"), 0644)
	os.WriteFile(thumbnailPath, []byte("data"), 0644)

	mock := newMockStorageClient()
	mock.putErr = fmt.Errorf("s3 error")
	ctx := context.Background()

	_, _, err := UploadResults(ctx, mock, "test-bucket", "proj-123", videoPath, thumbnailPath)
	if err == nil {
		t.Fatal("expected error from PutObject failure, got nil")
	}
}

func TestCleanupTempFiles_DeletesAllKeys(t *testing.T) {
	mock := newMockStorageClient()
	ctx := context.Background()

	slideKeys := []string{
		"temp/proj-123/slides/slide-001.png",
		"temp/proj-123/slides/slide-002.png",
		"temp/proj-123/slides/slide-003.png",
	}
	audioKeys := []string{
		"temp/proj-123/audio/audio-001.mp3",
		"temp/proj-123/audio/audio-002.mp3",
		"temp/proj-123/audio/audio-003.mp3",
	}

	err := CleanupTempFiles(ctx, mock, "test-bucket", slideKeys, audioKeys)
	if err != nil {
		t.Fatalf("CleanupTempFiles returned error: %v", err)
	}

	expectedCalls := len(slideKeys) + len(audioKeys)
	if len(mock.deleteCalls) != expectedCalls {
		t.Fatalf("expected %d DeleteObject calls, got %d", expectedCalls, len(mock.deleteCalls))
	}

	// Verify all slide keys were deleted
	for i, key := range slideKeys {
		if mock.deleteCalls[i] != key {
			t.Errorf("expected delete call %d to be %q, got %q", i, key, mock.deleteCalls[i])
		}
	}

	// Verify all audio keys were deleted
	for i, key := range audioKeys {
		idx := len(slideKeys) + i
		if mock.deleteCalls[idx] != key {
			t.Errorf("expected delete call %d to be %q, got %q", idx, key, mock.deleteCalls[idx])
		}
	}
}

func TestCleanupTempFiles_EmptyKeys(t *testing.T) {
	mock := newMockStorageClient()
	ctx := context.Background()

	err := CleanupTempFiles(ctx, mock, "test-bucket", nil, nil)
	if err != nil {
		t.Fatalf("CleanupTempFiles returned error for empty keys: %v", err)
	}

	if len(mock.deleteCalls) != 0 {
		t.Errorf("expected 0 delete calls, got %d", len(mock.deleteCalls))
	}
}

func TestCleanupTempFiles_DeleteError(t *testing.T) {
	mock := newMockStorageClient()
	mock.deleteErr = fmt.Errorf("delete failed")
	ctx := context.Background()

	slideKeys := []string{"temp/proj-123/slides/slide-001.png"}
	audioKeys := []string{"temp/proj-123/audio/audio-001.mp3"}

	err := CleanupTempFiles(ctx, mock, "test-bucket", slideKeys, audioKeys)
	if err == nil {
		t.Fatal("expected error from DeleteObject failure, got nil")
	}

	// Should still attempt to delete all keys even if one fails
	expectedCalls := len(slideKeys) + len(audioKeys)
	if len(mock.deleteCalls) != expectedCalls {
		t.Fatalf("expected %d DeleteObject calls even with errors, got %d", expectedCalls, len(mock.deleteCalls))
	}
}

func TestThumbnailGenerator_ImplementsInterface(t *testing.T) {
	// Verify ThumbnailGenerator satisfies ThumbnailInterface
	var _ ThumbnailInterface = &ThumbnailGenerator{}
}
