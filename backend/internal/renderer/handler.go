// Package renderer implements the video rendering Lambda handler that
// orchestrates downloading assets, composing video, generating thumbnails,
// uploading results, and cleaning up temporary files.
package renderer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rahul/indifferent/backend/internal/models"
)

// CompositorInterface defines the video composition contract.
type CompositorInterface interface {
	ComposeVideo(ctx context.Context, workDir string, slideFiles, audioFiles []string) (string, error)
	ApplyLogoOverlay(workDir, videoPath, logoPath string) (string, error)
}

// ThumbnailInterface defines thumbnail generation operations.
type ThumbnailInterface interface {
	GenerateThumbnail(firstSlide string, outputPath string) error
}

// SimpleCompositor implements CompositorInterface using the full Compositor internally.
// It provides the simplified interface expected by the Handler.
type SimpleCompositor struct{}

// ComposeVideo implements CompositorInterface by creating a Compositor and driving
// the full FFmpeg composition pipeline.
func (s *SimpleCompositor) ComposeVideo(ctx context.Context, workDir string, slideFiles, audioFiles []string) (string, error) {
	cfg := DefaultConfig()
	compositor := NewCompositor(cfg, workDir)

	// Audio files are interleaved: [q0.mp3, q0_answer.mp3, q1.mp3, q1_answer.mp3, ...]
	// So numQuestions = len(audioFiles) / 2
	if len(audioFiles) == 0 {
		return "", fmt.Errorf("no audio files provided")
	}
	numQuestions := len(audioFiles) / 2
	if numQuestions == 0 {
		// Fallback: if odd number, treat as legacy (1 audio per question)
		numQuestions = len(audioFiles)
	}

	segments, outroSlide, err := BuildSegments(slideFiles, audioFiles, numQuestions)
	if err != nil {
		return "", fmt.Errorf("failed to build segments: %w (slides=%d, audio=%d, questions=%d)", err, len(slideFiles), len(audioFiles), numQuestions)
	}

	return compositor.ComposeVideo(ctx, segments, outroSlide)
}

// ApplyLogoOverlay implements CompositorInterface by delegating to a
// Compositor bound to the caller's work directory.
func (s *SimpleCompositor) ApplyLogoOverlay(workDir, videoPath, logoPath string) (string, error) {
	compositor := NewCompositor(DefaultConfig(), workDir)
	return compositor.applyLogoOverlay(workDir, videoPath, logoPath)
}

// StorageClient defines S3 operations needed by the renderer.
type StorageClient interface {
	GetObject(ctx context.Context, bucket, key string) ([]byte, error)
	PutObject(ctx context.Context, bucket, key string, data []byte, contentType string) error
	DeleteObject(ctx context.Context, bucket, key string) error
}

// Handler encapsulates the renderer handler dependencies.
type Handler struct {
	Compositor CompositorInterface
	Thumbnail  ThumbnailInterface
	Storage    StorageClient
	Bucket     string
}

// NewHandler creates a Handler with the given dependencies.
func NewHandler(compositor CompositorInterface, thumbnail ThumbnailInterface, storage StorageClient, bucket string) *Handler {
	return &Handler{
		Compositor: compositor,
		Thumbnail:  thumbnail,
		Storage:    storage,
		Bucket:     bucket,
	}
}

// HandleRequest processes a RendererInput event and returns RendererOutput.
// It orchestrates: download assets → compose video → generate thumbnail → upload → cleanup.
func (h *Handler) HandleRequest(ctx context.Context, input models.RendererInput) (models.RendererOutput, error) {
	// Create temp working directory
	workDir, err := os.MkdirTemp("", "renderer-*")
	if err != nil {
		return models.RendererOutput{}, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(workDir)

	// Download all slide PNGs from S3 to local workDir
	slideFiles, err := h.downloadAssets(ctx, workDir, "slides", input.SlideKeys)
	if err != nil {
		return models.RendererOutput{}, fmt.Errorf("failed to download slides: %w", err)
	}

	// Download all audio MP3s from S3 to local workDir
	audioFiles, err := h.downloadAssets(ctx, workDir, "audio", input.AudioKeys)
	if err != nil {
		return models.RendererOutput{}, fmt.Errorf("failed to download audio: %w", err)
	}

	// Compose video using FFmpeg via Compositor
	videoPath, err := h.Compositor.ComposeVideo(ctx, workDir, slideFiles, audioFiles)
	if err != nil {
		return models.RendererOutput{}, fmt.Errorf("failed to compose video: %w", err)
	}

	// Optional brand logo overlay (top-right). Fail-soft: a missing logo
	// must never block the finished video.
	if input.LogoKey != "" {
		if branded, berr := h.applyBranding(ctx, workDir, videoPath, input.LogoKey); berr != nil {
			fmt.Printf("branding skipped: %v\n", berr)
		} else {
			videoPath = branded
		}
	}

	// Generate thumbnail from first slide
	thumbnailPath := filepath.Join(workDir, "thumbnail.png")
	if len(slideFiles) > 0 {
		if err := h.Thumbnail.GenerateThumbnail(slideFiles[0], thumbnailPath); err != nil {
			return models.RendererOutput{}, fmt.Errorf("failed to generate thumbnail: %w", err)
		}
	}

	// Read final video and upload to S3
	videoKey := fmt.Sprintf("output/%s/video.mp4", input.ProjectID)
	videoData, err := os.ReadFile(videoPath)
	if err != nil {
		return models.RendererOutput{}, fmt.Errorf("failed to read video file: %w", err)
	}
	if err := h.Storage.PutObject(ctx, h.Bucket, videoKey, videoData, "video/mp4"); err != nil {
		return models.RendererOutput{}, fmt.Errorf("failed to upload video to S3: %w", err)
	}

	// Read thumbnail and upload to S3
	thumbnailKey := fmt.Sprintf("output/%s/thumbnail.png", input.ProjectID)
	thumbnailData, err := os.ReadFile(thumbnailPath)
	if err != nil {
		return models.RendererOutput{}, fmt.Errorf("failed to read thumbnail file: %w", err)
	}
	if err := h.Storage.PutObject(ctx, h.Bucket, thumbnailKey, thumbnailData, "image/png"); err != nil {
		return models.RendererOutput{}, fmt.Errorf("failed to upload thumbnail to S3: %w", err)
	}

	// Cleanup: delete all temporary S3 objects (slideKeys + audioKeys)
	h.cleanupTempS3Objects(ctx, input.SlideKeys, input.AudioKeys)

	return models.RendererOutput{
		ProjectID:    input.ProjectID,
		VideoKey:     videoKey,
		ThumbnailKey: thumbnailKey,
	}, nil
}

// validateS3Key rejects object keys that could escape an intended prefix or a
// local directory once joined to a path. Legitimate pipeline keys are plain
// relative keys such as "temp/<projectId>/slides/q0_question.png": forward
// slashes only, no leading slash, no "." or ".." segment, no NUL or backslash.
// Keys reach this handler from the job record, which is built from
// user-supplied uploads, so they are treated as untrusted (go/path-injection).
func validateS3Key(key string) error {
	if key == "" {
		return fmt.Errorf("empty S3 key")
	}
	if strings.ContainsAny(key, "\x00\\") || strings.HasPrefix(key, "/") {
		return fmt.Errorf("invalid S3 key: %q", key)
	}
	for _, seg := range strings.Split(key, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return fmt.Errorf("invalid S3 key: %q", key)
		}
	}
	return nil
}

// applyBranding downloads the project logo and burns it onto the finished
// video, returning the branded file path.
func (h *Handler) applyBranding(ctx context.Context, workDir, videoPath, logoKey string) (string, error) {
	if err := validateS3Key(logoKey); err != nil {
		return "", err
	}
	logoData, err := h.Storage.GetObject(ctx, h.Bucket, logoKey)
	if err != nil {
		return "", fmt.Errorf("failed to download logo: %w", err)
	}
	logoPath := filepath.Join(workDir, "logo.png")
	if err := os.WriteFile(logoPath, logoData, 0o644); err != nil {
		return "", fmt.Errorf("failed to write logo: %w", err)
	}
	return h.Compositor.ApplyLogoOverlay(workDir, videoPath, logoPath)
}

// safeFileExt extracts a safe file extension from a key, stripping any path
// traversal characters. Only alphanumeric characters and dots are kept.
func safeFileExt(key string) string {
	ext := filepath.Ext(filepath.Base(key))
	// Only allow alphanumeric extensions with a leading dot
	cleaned := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' {
			return r
		}
		return -1
	}, ext)
	if cleaned == "" || cleaned == "." {
		return ".bin"
	}
	if cleaned[0] != '.' {
		cleaned = "." + cleaned
	}
	return cleaned
}

// downloadAssets downloads files from S3 to a subdirectory within workDir.
func (h *Handler) downloadAssets(ctx context.Context, workDir, subDir string, keys []string) ([]string, error) {
	dir := filepath.Join(workDir, subDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create %s directory: %w", subDir, err)
	}

	// Resolve the absolute path of the safe directory for containment checks
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve directory path: %w", err)
	}

	var localFiles []string
	for i, key := range keys {
		// Reject traversal before the key is used as a storage path or joined locally.
		if err := validateS3Key(key); err != nil {
			return nil, err
		}
		data, err := h.Storage.GetObject(ctx, h.Bucket, key)
		if err != nil {
			return nil, fmt.Errorf("failed to download %s (key=%s): %w", subDir, key, err)
		}

		// Local filename is generated, not taken from the key: only its sanitized
		// extension is reused. filepath.Clean collapses the result and the prefix
		// check below proves containment, which is what go/path-injection needs.
		filename := filepath.Clean(fmt.Sprintf("%s-%03d%s", subDir, i, safeFileExt(key)))
		localPath := filepath.Join(dir, filename)

		// Verify the resolved path is within the safe directory
		absPath, err := filepath.Abs(localPath)
		if err != nil || !strings.HasPrefix(absPath, absDir+string(os.PathSeparator)) && absPath != absDir {
			return nil, fmt.Errorf("unsafe file path detected for key: %s", key)
		}

		if err := os.WriteFile(absPath, data, 0o644); err != nil {
			return nil, fmt.Errorf("failed to write %s to disk: %w", absPath, err)
		}

		localFiles = append(localFiles, absPath)
	}

	return localFiles, nil
}

// cleanupTempS3Objects deletes all temporary S3 objects created by previous pipeline stages.
func (h *Handler) cleanupTempS3Objects(ctx context.Context, slideKeys, audioKeys []string) {
	allKeys := append(slideKeys, audioKeys...)
	for _, key := range allKeys {
		// Best-effort cleanup; errors are logged but don't fail the operation
		_ = h.Storage.DeleteObject(ctx, h.Bucket, key)
	}
}
