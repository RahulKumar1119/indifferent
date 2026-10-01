// Package storage provides S3 client operations for reading and writing objects.
package storage

import (
	"bytes"
	"context"
	"errors"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

const (
	// UploadURLExpiration is the duration for presigned upload (PUT) URLs.
	UploadURLExpiration = 15 * time.Minute

	// DownloadURLExpiration is the duration for presigned download (GET) URLs.
	DownloadURLExpiration = 24 * time.Hour
)

// headObjectAPI is the subset of the S3 API used by HeadObject. Defining it as
// an interface lets tests exercise the exists/not-found/error mapping without
// network access or valid credentials.
type headObjectAPI interface {
	HeadObject(ctx context.Context, in *s3.HeadObjectInput, optFns ...func(*s3.Options)) (*s3.HeadObjectOutput, error)
}

// S3Client wraps the AWS S3 service client for common operations.
type S3Client struct {
	client *s3.Client
	// headAPI performs HeadObject calls. It defaults to client but can be
	// overridden in tests. When nil, client is used.
	headAPI headObjectAPI
}

// NewS3Client creates a new S3Client using the default AWS config.
func NewS3Client(ctx context.Context) (*S3Client, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}
	c := s3.NewFromConfig(cfg)
	return &S3Client{client: c, headAPI: c}, nil
}

// NewS3ClientWithRegion creates an S3Client bound to the given region without
// requiring live credentials. Presigning is a local operation, so this is
// suitable for tests in other packages that need a usable *S3Client.
func NewS3ClientWithRegion(ctx context.Context, region string) (*S3Client, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, err
	}
	c := s3.NewFromConfig(cfg)
	return &S3Client{client: c, headAPI: c}, nil
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

// HeadObject reports whether an object exists at the specified bucket and key.
// It returns (true, nil) when the object exists, (false, nil) when it does not,
// and (false, err) for any other error.
//
// S3 returns 403 Forbidden (instead of 404 NotFound) for a missing key when the
// caller lacks s3:ListBucket on the bucket — which is the common case for an
// object-scoped role (arn:.../bucket/*). For an existence check that must
// therefore be treated as "not found" so the caller can proceed (a genuine
// permission problem on an existing object surfaces later as a Get/Put
// failure with a clearer message).
func (c *S3Client) HeadObject(ctx context.Context, bucket, key string) (bool, error) {
	api := c.headAPI
	if api == nil {
		api = c.client
	}
	_, err := api.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		if isHeadObjectNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// isHeadObjectNotFound reports whether err from HeadObject means "no such
// object" rather than a real failure. It covers:
//
//   - the modeled *types.NotFound / *types.NoSuchKey shapes,
//   - Smithy APIError codes "NotFound" / "NoSuchKey" / "NoSuchBucket" (the
//     shape the SDK actually returns for a missing key — HeadObject has no
//     response body, so the typed NotFound is rarely produced), and
//   - "Forbidden" / "AccessDenied" / "AccessForbidden", which S3 returns for a
//     missing key when the caller lacks s3:ListBucket (see HeadObject docs).
func isHeadObjectNotFound(err error) bool {
	var notFound *types.NotFound
	if errors.As(err, &notFound) {
		return true
	}
	var noSuchKey *types.NoSuchKey
	if errors.As(err, &noSuchKey) {
		return true
	}
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "NotFound", "NoSuchKey", "NoSuchBucket",
			"Forbidden", "AccessDenied", "AccessForbidden", "403":
			return true
		}
	}
	return false
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
