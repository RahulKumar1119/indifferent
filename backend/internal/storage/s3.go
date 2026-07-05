// Package storage provides S3 client operations for reading and writing objects.
package storage

import (
	"bytes"
	"context"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3Client wraps the AWS S3 service client for common operations.
type S3Client struct {
	client *s3.Client
}

// NewS3Client creates a new S3Client using the default AWS config.
func NewS3Client(ctx context.Context) (*S3Client, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}
	return &S3Client{client: s3.NewFromConfig(cfg)}, nil
}

// GetObject downloads an object from S3 and returns its contents as bytes.
func (c *S3Client) GetObject(ctx context.Context, bucket, key string) ([]byte, error) {
	output, err := c.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}
	defer output.Body.Close()

	return io.ReadAll(output.Body)
}

// PutObject uploads data to S3 at the specified bucket and key.
func (c *S3Client) PutObject(ctx context.Context, bucket, key string, data []byte, contentType string) error {
	_, err := c.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
	})
	return err
}
