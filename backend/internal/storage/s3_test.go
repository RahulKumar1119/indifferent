package storage

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// newTestClient creates an S3Client with a real (but unconfigured) s3.Client.
// Presigning does not make network calls—it only builds a signed URL locally—
// so this works without valid AWS credentials or endpoint connectivity.
func newTestClient(t *testing.T) *S3Client {
	t.Helper()
	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion("us-east-1"),
	)
	if err != nil {
		t.Fatalf("failed to load AWS config: %v", err)
	}
	client := s3.NewFromConfig(cfg)
	return &S3Client{client: client}
}

func TestGenerateUploadURL(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()

	url, err := c.GenerateUploadURL(ctx, "test-bucket", "uploads/user1/project1/input.txt", "text/plain", UploadURLExpiration)
	if err != nil {
		t.Fatalf("GenerateUploadURL returned error: %v", err)
	}
	if url == "" {
		t.Fatal("GenerateUploadURL returned empty URL")
	}
	// Verify the URL contains expected components
	if !contains(url, "test-bucket") {
		t.Errorf("expected URL to reference bucket, got: %s", url)
	}
	if !contains(url, "uploads/user1/project1/input.txt") {
		t.Errorf("expected URL to reference key, got: %s", url)
	}
	if !contains(url, "X-Amz-Expires") {
		t.Errorf("expected URL to contain X-Amz-Expires parameter, got: %s", url)
	}
}

func TestGenerateDownloadURL(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()

	url, err := c.GenerateDownloadURL(ctx, "test-bucket", "output/user1/project1/video.mp4", DownloadURLExpiration)
	if err != nil {
		t.Fatalf("GenerateDownloadURL returned error: %v", err)
	}
	if url == "" {
		t.Fatal("GenerateDownloadURL returned empty URL")
	}
	// Verify the URL contains expected components
	if !contains(url, "test-bucket") {
		t.Errorf("expected URL to reference bucket, got: %s", url)
	}
	if !contains(url, "output/user1/project1/video.mp4") {
		t.Errorf("expected URL to reference key, got: %s", url)
	}
	if !contains(url, "X-Amz-Expires") {
		t.Errorf("expected URL to contain X-Amz-Expires parameter, got: %s", url)
	}
}

func TestUploadURLExpiration(t *testing.T) {
	// Verify the upload URL expiration constant is 15 minutes.
	if UploadURLExpiration != 15*time.Minute {
		t.Errorf("expected UploadURLExpiration to be 15m, got %v", UploadURLExpiration)
	}
}

func TestDownloadURLExpiration(t *testing.T) {
	// Verify the download URL expiration constant is 24 hours.
	if DownloadURLExpiration != 24*time.Hour {
		t.Errorf("expected DownloadURLExpiration to be 24h, got %v", DownloadURLExpiration)
	}
}

func TestGenerateUploadURL_DifferentContentTypes(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()

	contentTypes := []string{"text/plain", "application/json", "video/mp4", "image/png"}
	for _, ct := range contentTypes {
		url, err := c.GenerateUploadURL(ctx, "test-bucket", "test-key", ct, UploadURLExpiration)
		if err != nil {
			t.Fatalf("GenerateUploadURL(%s) returned error: %v", ct, err)
		}
		if url == "" {
			t.Errorf("GenerateUploadURL(%s) returned empty URL", ct)
		}
	}
}

func TestGenerateDownloadURL_CustomExpiration(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()

	// Test with a custom 1-hour expiration
	url, err := c.GenerateDownloadURL(ctx, "test-bucket", "some/key.mp4", 1*time.Hour)
	if err != nil {
		t.Fatalf("GenerateDownloadURL with custom expiration returned error: %v", err)
	}
	if url == "" {
		t.Fatal("GenerateDownloadURL with custom expiration returned empty URL")
	}
}

// contains checks if substr is present in s.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
