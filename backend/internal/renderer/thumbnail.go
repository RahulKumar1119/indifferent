package renderer

import (
	"context"
	"fmt"
	"os"
	"os/exec"
)

// ThumbnailGenerator implements ThumbnailInterface using FFmpeg.
type ThumbnailGenerator struct{}

// GenerateThumbnail creates a thumbnail PNG from the first slide,
// scaled to a YouTube-friendly resolution (1280x720).
func (t *ThumbnailGenerator) GenerateThumbnail(firstSlide string, outputPath string) error {
	cmd := exec.Command("ffmpeg",
		"-i", firstSlide,
		"-vf", "scale=1280:720",
		"-frames:v", "1",
		"-y", outputPath,
	)

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ffmpeg thumbnail generation failed: %w\noutput: %s", err, string(output))
	}

	return nil
}

// UploadResults uploads the final video and thumbnail to S3 and returns their keys.
func UploadResults(ctx context.Context, storage StorageClient, bucket, projectID, videoPath, thumbnailPath string) (string, string, error) {
	videoKey := fmt.Sprintf("output/%s/video.mp4", projectID)
	thumbnailKey := fmt.Sprintf("output/%s/thumbnail.png", projectID)

	// Read and upload video
	videoData, err := os.ReadFile(videoPath)
	if err != nil {
		return "", "", fmt.Errorf("failed to read video file: %w", err)
	}
	if err := storage.PutObject(ctx, bucket, videoKey, videoData, "video/mp4"); err != nil {
		return "", "", fmt.Errorf("failed to upload video: %w", err)
	}

	// Read and upload thumbnail
	thumbnailData, err := os.ReadFile(thumbnailPath)
	if err != nil {
		return "", "", fmt.Errorf("failed to read thumbnail file: %w", err)
	}
	if err := storage.PutObject(ctx, bucket, thumbnailKey, thumbnailData, "image/png"); err != nil {
		return "", "", fmt.Errorf("failed to upload thumbnail: %w", err)
	}

	return videoKey, thumbnailKey, nil
}

// CleanupTempFiles deletes all temporary S3 objects (slides and audio) for a project.
func CleanupTempFiles(ctx context.Context, storage StorageClient, bucket string, slideKeys, audioKeys []string) error {
	allKeys := append(slideKeys, audioKeys...)
	var firstErr error

	for _, key := range allKeys {
		if err := storage.DeleteObject(ctx, bucket, key); err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("failed to delete temp file (key=%s): %w", key, err)
			}
		}
	}

	return firstErr
}
