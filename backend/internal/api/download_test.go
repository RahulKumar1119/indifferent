package api

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/aws/smithy-go"
	"github.com/rahul/indifferent/backend/internal/storage"
)

// stubDownloader implements storage.Downloader with scripted archival status,
// letting download tests exercise the Glacier restore flow without S3.
type stubDownloader struct {
	status       storage.ObjectStatus
	statusErr    error
	restoreCalls int
	restoreErr   error
	deletedKeys  []string
	deleteErr    error
	listedKeys   []string
	listErr      error
	purged       []string
	purgeErr     error
	meta         storage.ObjectMeta
	metaErr      error
}

func (s *stubDownloader) GenerateUploadURL(ctx context.Context, bucket, key, contentType string, expiration time.Duration) (string, error) {
	return "https://uploads.example/" + key, nil
}

func (s *stubDownloader) GenerateDownloadURL(ctx context.Context, bucket, key string, expiration time.Duration) (string, error) {
	return "https://downloads.example/" + key, nil
}

func (s *stubDownloader) ObjectStatus(ctx context.Context, bucket, key string) (storage.ObjectStatus, error) {
	if s.statusErr != nil {
		return storage.ObjectStatus{}, s.statusErr
	}
	return s.status, nil
}

func (s *stubDownloader) RestoreObject(ctx context.Context, bucket, key string) error {
	s.restoreCalls++
	return s.restoreErr
}

func (s *stubDownloader) DeleteObjects(ctx context.Context, bucket string, keys []string) error {
	s.deletedKeys = append(s.deletedKeys, keys...)
	return s.deleteErr
}

func (s *stubDownloader) ListKeys(ctx context.Context, bucket, prefix string) ([]string, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	var out []string
	for _, k := range s.listedKeys {
		if strings.HasPrefix(k, prefix) {
			out = append(out, k)
		}
	}
	return out, nil
}

func (s *stubDownloader) DeleteObjectVersions(ctx context.Context, bucket, prefix string) error {
	s.purged = append(s.purged, prefix)
	return s.purgeErr
}

func (s *stubDownloader) ObjectMeta(ctx context.Context, bucket, key string) (storage.ObjectMeta, error) {
	if s.metaErr != nil {
		return storage.ObjectMeta{}, s.metaErr
	}
	return s.meta, nil
}

func TestDownloadURLOrRestore_Standard(t *testing.T) {
	h := newTestHandler()
	stub := &stubDownloader{status: storage.ObjectStatus{StorageClass: "STANDARD"}}
	h.S3 = stub

	url, restoring, err := h.downloadURLOrRestore(context.Background(), "output/p1/video.mp4")
	if err != nil || restoring {
		t.Fatalf("expected direct URL, got url=%q restoring=%v err=%v", url, restoring, err)
	}
	if url == "" {
		t.Fatal("expected non-empty presigned URL")
	}
	if stub.restoreCalls != 0 {
		t.Errorf("expected no restore for standard object, got %d calls", stub.restoreCalls)
	}
}

func TestDownloadURLOrRestore_ArchivedTriggersRestore(t *testing.T) {
	h := newTestHandler()
	stub := &stubDownloader{status: storage.ObjectStatus{StorageClass: "GLACIER"}}
	h.S3 = stub

	url, restoring, err := h.downloadURLOrRestore(context.Background(), "uploads/u/j/source.mp4")
	if err != nil || !restoring || url != "" {
		t.Fatalf("expected restoring=true, got url=%q restoring=%v err=%v", url, restoring, err)
	}
	if stub.restoreCalls != 1 {
		t.Errorf("expected exactly 1 restore request, got %d", stub.restoreCalls)
	}
}

func TestDownloadURLOrRestore_RestoreInProgress(t *testing.T) {
	h := newTestHandler()
	stub := &stubDownloader{status: storage.ObjectStatus{
		StorageClass: "GLACIER",
		Restore:      `ongoing-request="true"`,
	}}
	h.S3 = stub

	_, restoring, err := h.downloadURLOrRestore(context.Background(), "uploads/u/j/source.mp4")
	if err != nil || !restoring {
		t.Fatalf("expected restoring=true, got restoring=%v err=%v", restoring, err)
	}
	if stub.restoreCalls != 0 {
		t.Errorf("must not re-request a restore already in progress, got %d calls", stub.restoreCalls)
	}
}

func TestDownloadURLOrRestore_RestoredCopyDownloads(t *testing.T) {
	h := newTestHandler()
	stub := &stubDownloader{status: storage.ObjectStatus{
		StorageClass: "GLACIER",
		Restore:      `ongoing-request="false", expiry-date="Fri, 09 Oct 2026 00:00:00 GMT"`,
	}}
	h.S3 = stub

	url, restoring, err := h.downloadURLOrRestore(context.Background(), "uploads/u/j/source.mp4")
	if err != nil || restoring || url == "" {
		t.Fatalf("expected direct URL for restored copy, got url=%q restoring=%v err=%v", url, restoring, err)
	}
	if stub.restoreCalls != 0 {
		t.Errorf("expected no restore for usable copy, got %d calls", stub.restoreCalls)
	}
}

func TestDownloadURLOrRestore_MissingObject(t *testing.T) {
	h := newTestHandler()
	stub := &stubDownloader{statusErr: &stubNotFoundError{}}
	h.S3 = stub

	_, _, err := h.downloadURLOrRestore(context.Background(), "output/p1/video.mp4")
	if !errors.Is(err, errObjectNotFound) {
		t.Fatalf("expected errObjectNotFound, got %v", err)
	}
}

// stubNotFoundError simulates an S3 not-found failure. It is recognized by
// storage.IsNotFound through the Smithy APIError code.
type stubNotFoundError struct{}

func (e *stubNotFoundError) Error() string         { return "not found" }
func (e *stubNotFoundError) ErrorCode() string      { return "NoSuchKey" }
func (e *stubNotFoundError) ErrorMessage() string   { return "not found" }
func (e *stubNotFoundError) ErrorFault() smithy.ErrorFault { return smithy.FaultUnknown }
