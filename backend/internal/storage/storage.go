// Package storage provides interfaces and implementations for
// interacting with S3 and DynamoDB.
package storage

import "context"

// StorageClient defines the interface for object storage operations.
type StorageClient interface {
	GetObject(ctx context.Context, bucket, key string) ([]byte, error)
	PutObject(ctx context.Context, bucket, key string, data []byte, contentType string) error
	DeleteObject(ctx context.Context, bucket, key string) error
}
