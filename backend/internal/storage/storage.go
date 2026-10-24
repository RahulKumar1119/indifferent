// Package storage provides interfaces and implementations for
// interacting with S3 and DynamoDB.
package storage

import (
	"context"
	"time"
)

// StorageClient defines the interface for object storage operations.
type StorageClient interface {
	GetObject(ctx context.Context, bucket, key string) ([]byte, error)
	PutObject(ctx context.Context, bucket, key string, data []byte, contentType string) error
	DeleteObject(ctx context.Context, bucket, key string) error
	HeadObject(ctx context.Context, bucket, key string) (bool, error)
}

// Downloader is the S3 surface the API layer needs for user downloads:
// presigned URLs plus archival-status awareness so Glacier-stored videos
// degrade to a restore flow instead of a broken link. DeleteObjects lets
// delete flows remove a job's whole asset set in one call.
type Downloader interface {
	GenerateUploadURL(ctx context.Context, bucket, key, contentType string, expiration time.Duration) (string, error)
	GenerateDownloadURL(ctx context.Context, bucket, key string, expiration time.Duration) (string, error)
	// GenerateAttachmentURL is like GenerateDownloadURL but makes the browser
	// save the file under filename rather than display it.
	GenerateAttachmentURL(ctx context.Context, bucket, key, filename string, expiration time.Duration) (string, error)
	ObjectStatus(ctx context.Context, bucket, key string) (ObjectStatus, error)
	RestoreObject(ctx context.Context, bucket, key string) error
	DeleteObjects(ctx context.Context, bucket string, keys []string) error
	// ListKeys returns all keys under prefix (paginated internally).
	ListKeys(ctx context.Context, bucket, prefix string) ([]string, error)
	// ObjectMeta reads size + content type for upload validation.
	ObjectMeta(ctx context.Context, bucket, key string) (ObjectMeta, error)
	// GetObject reads a whole object (transcripts, small JSON).
	GetObject(ctx context.Context, bucket, key string) ([]byte, error)
	// DeleteObjectVersions permanently removes every version and delete
	// marker under prefix. Needed because plain deletes on a versioned
	// bucket only hide objects (noncurrent versions keep costing).
	DeleteObjectVersions(ctx context.Context, bucket, prefix string) error

	// Multipart upload for large sources (issue #151).
	CreateMultipartUpload(ctx context.Context, bucket, key, contentType string) (uploadID string, err error)
	// GenerateUploadPartURL signs for the Transfer Acceleration endpoint when
	// accelerate is set. Every other call stays on the regional endpoint.
	GenerateUploadPartURL(ctx context.Context, bucket, key, uploadID string, partNumber int32, expiration time.Duration, accelerate bool) (string, error)
	ListUploadedParts(ctx context.Context, bucket, key, uploadID string) ([]UploadedPart, error)
	CompleteMultipartUpload(ctx context.Context, bucket, key, uploadID string, parts []CompletedPart) error
	AbortMultipartUpload(ctx context.Context, bucket, key, uploadID string) error
}
