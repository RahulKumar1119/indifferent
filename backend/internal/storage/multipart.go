package storage

import (
	"context"
	"errors"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

// PartURLExpiration is how long a presigned UploadPart URL stays valid. A
// part that is slow to send, or retried with backoff on the same URL, outlives
// the 15 minute window of the single-PUT URL. The signing credentials can end
// the URL sooner, so clients still refresh on a 403.
const PartURLExpiration = time.Hour

// UploadedPart is a part S3 has stored for an open multipart upload.
type UploadedPart struct {
	PartNumber int32
	ETag       string
	Size       int64
}

// CompletedPart names a stored part when completing an upload.
type CompletedPart struct {
	PartNumber int32
	ETag       string
}

// CreateMultipartUpload opens a multipart upload and returns its upload ID.
// The content type is fixed here, so UploadPart requests carry no headers.
func (c *S3Client) CreateMultipartUpload(ctx context.Context, bucket, key, contentType string) (string, error) {
	out, err := c.client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return "", err
	}
	return aws.ToString(out.UploadId), nil
}

// GenerateUploadPartURL presigns a PUT for one part of an open upload. With
// accelerate set, the URL is signed for <bucket>.s3-accelerate.amazonaws.com.
// Only this presign client is switched. The shared client keeps the regional
// endpoint for every call the Lambda makes itself, since acceleration only
// helps the browser.
func (c *S3Client) GenerateUploadPartURL(ctx context.Context, bucket, key, uploadID string, partNumber int32, expiration time.Duration, accelerate bool) (string, error) {
	var presignOpts []func(*s3.PresignOptions)
	if accelerate {
		presignOpts = append(presignOpts, s3.WithPresignClientFromClientOptions(func(o *s3.Options) { o.UseAccelerate = true }))
	}
	presignClient := s3.NewPresignClient(c.client, presignOpts...)
	request, err := presignClient.PresignUploadPart(ctx, &s3.UploadPartInput{
		Bucket:     aws.String(bucket),
		Key:        aws.String(key),
		UploadId:   aws.String(uploadID),
		PartNumber: aws.Int32(partNumber),
	}, s3.WithPresignExpires(expiration))
	if err != nil {
		return "", err
	}
	return request.URL, nil
}

// ListUploadedParts returns every part stored for an open upload, following
// pagination.
func (c *S3Client) ListUploadedParts(ctx context.Context, bucket, key, uploadID string) ([]UploadedPart, error) {
	paginator := s3.NewListPartsPaginator(c.client, &s3.ListPartsInput{
		Bucket:   aws.String(bucket),
		Key:      aws.String(key),
		UploadId: aws.String(uploadID),
	})
	var parts []UploadedPart
	for paginator.HasMorePages() {
		out, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, p := range out.Parts {
			parts = append(parts, UploadedPart{
				PartNumber: aws.ToInt32(p.PartNumber),
				ETag:       aws.ToString(p.ETag),
				Size:       aws.ToInt64(p.Size),
			})
		}
	}
	return parts, nil
}

// CompleteMultipartUpload assembles the stored parts, in the order given, into
// the final object.
func (c *S3Client) CompleteMultipartUpload(ctx context.Context, bucket, key, uploadID string, parts []CompletedPart) error {
	done := make([]types.CompletedPart, 0, len(parts))
	for _, p := range parts {
		done = append(done, types.CompletedPart{
			PartNumber: aws.Int32(p.PartNumber),
			ETag:       aws.String(p.ETag),
		})
	}
	_, err := c.client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:          aws.String(bucket),
		Key:             aws.String(key),
		UploadId:        aws.String(uploadID),
		MultipartUpload: &types.CompletedMultipartUpload{Parts: done},
	})
	return err
}

// AbortMultipartUpload discards an open upload and the parts stored for it.
func (c *S3Client) AbortMultipartUpload(ctx context.Context, bucket, key, uploadID string) error {
	_, err := c.client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
		Bucket:   aws.String(bucket),
		Key:      aws.String(key),
		UploadId: aws.String(uploadID),
	})
	return err
}

// IsNoSuchUpload reports whether err means the upload ID is not open: never
// created, aborted, expired by the lifecycle rule, or already completed.
func IsNoSuchUpload(err error) bool {
	var noSuch *types.NoSuchUpload
	if errors.As(err, &noSuch) {
		return true
	}
	var apiErr smithy.APIError
	return errors.As(err, &apiErr) && apiErr.ErrorCode() == "NoSuchUpload"
}

// IsInvalidParts reports whether S3 refused to complete an upload because a
// listed part is missing, has a different ETag, is out of order, or is too
// small.
func IsInvalidParts(err error) bool {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "InvalidPart", "InvalidPartOrder", "EntityTooSmall":
			return true
		}
	}
	return false
}
