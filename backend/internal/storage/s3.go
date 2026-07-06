// Package storage provides S3 client operations for reading and writing objects.
package storage

import (
	"bytes"
	"context"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

const (
	// UploadURLExpiration is the duration for presigned upload (PUT) URLs.
	UploadURLExpiration = 15 * time.Minute

	// DownloadURLExpiration is the duration for presigned download (GET) URLs.
	DownloadURLExpiration = 24 * time.Hour
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

// DeleteObject removes an object from S3 at the specified bucket and key.
func (c *S3Client) DeleteObject(ctx context.Context, bucket, key string) error {
	_, err := c.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	return err
}

// GenerateUploadURL creates a presigned PUT URL for uploading a file to S3.
// The URL expires after the specified duration.
func (c *S3Client) GenerateUploadURL(ctx context.Context, bucket, key, contentType string, expiration time.Duration) (string, error) {
	presignClient := s3.NewPresignClient(c.client)
	request, err := presignClient.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		ContentType: aws.String(contentType),
	}, s3.WithPresignExpires(expiration))
	if err != nil {
		return "", err
	}
	return request.URL, nil
}

// GenerateDownloadURL creates a presigned GET URL for downloading a file from S3.
// The URL expires after the specified duration (24 hours for video downloads).
func (c *S3Client) GenerateDownloadURL(ctx context.Context, bucket, key string, expiration time.Duration) (string, error) {
	presignClient := s3.NewPresignClient(c.client)
	request, err := presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(expiration))
	if err != nil {
		return "", err
	}
	return request.URL, nil
}
