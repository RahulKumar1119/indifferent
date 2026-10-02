// Package storage provides S3 client operations for reading and writing objects.
package storage

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
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

// ObjectStatus describes where an S3 object physically lives, used to decide
// whether a presigned download URL will work or the object must first be
// restored from archival storage.
type ObjectStatus struct {
	// StorageClass is the S3 storage class (e.g. "STANDARD", "GLACIER",
	// "DEEP_ARCHIVE"). Empty means unreported — treat as standard.
	StorageClass string
	// Restore is the raw x-amz-restore header value ("" when absent), e.g.
	// `ongoing-request="true"` while a restore is in progress or
	// `ongoing-request="false", expiry-date="..."` once a restored copy
	// is available for download.
	Restore string
}

// Archived reports whether the object sits in archival storage and therefore
// cannot be downloaded directly. Glacier Instant Retrieval is intentionally
// excluded: it serves GETs without restoration.
func (s ObjectStatus) Archived() bool {
	switch s.StorageClass {
	case string(types.ObjectStorageClassGlacier), string(types.ObjectStorageClassDeepArchive):
		return true
	}
	return false
}

// RestoreInProgress reports whether a previously requested restore is still
// running (the object becomes downloadable once it completes).
func (s ObjectStatus) RestoreInProgress() bool {
	return strings.Contains(s.Restore, `ongoing-request="true"`)
}

// Downloadable reports whether a presigned GET URL will work right now: any
// non-archived object, or an archived one with a completed restore.
func (s ObjectStatus) Downloadable() bool {
	if s.RestoreInProgress() {
		return false
	}
	if s.Archived() {
		// Archived with no restore header at all: no usable copy.
		// A completed restore carries ongoing-request="false".
		return strings.Contains(s.Restore, `ongoing-request="false"`)
	}
	return true
}

// ObjectStatus returns the storage status of the object at bucket/key.
// A missing object yields a not-found error (use errors.Is NotFound checks
// at the call site as needed).
func (c *S3Client) ObjectStatus(ctx context.Context, bucket, key string) (ObjectStatus, error) {
	out, err := c.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return ObjectStatus{}, err
	}
	st := ObjectStatus{StorageClass: string(out.StorageClass)}
	if out.Restore != nil {
		st.Restore = *out.Restore
	}
	return st, nil
}

// IsNotFound reports whether err from an S3 read means the object does not
// exist (as opposed to a real failure).
func IsNotFound(err error) bool {
	return isHeadObjectNotFound(err)
}

// restoreCopyDays is how long a restored (downloadable) copy of an archived
// object is kept before S3 drops it back to archival storage.
const restoreCopyDays = 7

// RestoreObject requests restoration of an archived object with Standard
// retrieval (typically minutes to hours). It is a no-op for objects that
// already have a usable copy; S3 returns RestoreAlreadyInProgress in that
// case, which is swallowed here.
func (c *S3Client) RestoreObject(ctx context.Context, bucket, key string) error {
	days := int32(restoreCopyDays)
	_, err := c.client.RestoreObject(ctx, &s3.RestoreObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		RestoreRequest: &types.RestoreRequest{
			Days: &days,
			GlacierJobParameters: &types.GlacierJobParameters{
				Tier: types.TierStandard,
			},
		},
	})
	if err != nil {
		// S3 reports a concurrent restore as the unmodeled
		// RestoreAlreadyInProgress API error: just as good as success.
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) && apiErr.ErrorCode() == "RestoreAlreadyInProgress" {
			return nil
		}
		return err
	}
	return nil
}
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
