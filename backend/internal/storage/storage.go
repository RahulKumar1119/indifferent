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
// degrade to a restore flow instead of a broken link.
type Downloader interface {
	GenerateUploadURL(ctx context.Context, bucket, key, contentType string, expiration time.Duration) (string, error)
	GenerateDownloadURL(ctx context.Context, bucket, key string, expiration time.Duration) (string, error)
	ObjectStatus(ctx context.Context, bucket, key string) (ObjectStatus, error)
	RestoreObject(ctx context.Context, bucket, key string) error
}
