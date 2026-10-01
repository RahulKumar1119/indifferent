package storage

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

// mockHeadAPI implements headObjectAPI for testing HeadObject's mapping logic.
type mockHeadAPI struct {
	out *s3.HeadObjectOutput
	err error

	gotBucket string
	gotKey    string
}

func (m *mockHeadAPI) HeadObject(ctx context.Context, in *s3.HeadObjectInput, optFns ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
	if in.Bucket != nil {
		m.gotBucket = *in.Bucket
	}
	if in.Key != nil {
		m.gotKey = *in.Key
	}
	return m.out, m.err
}

func TestHeadObject_Exists(t *testing.T) {
	mock := &mockHeadAPI{out: &s3.HeadObjectOutput{}}
	c := &S3Client{headAPI: mock}

	exists, err := c.HeadObject(context.Background(), "test-bucket", "shorts/u1/j1/clips/c1.mp4")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !exists {
		t.Error("expected exists=true when the object is present")
	}
	if mock.gotBucket != "test-bucket" {
		t.Errorf("expected bucket 'test-bucket', got %q", mock.gotBucket)
	}
	if mock.gotKey != "shorts/u1/j1/clips/c1.mp4" {
		t.Errorf("expected key to be passed through, got %q", mock.gotKey)
	}
}

func TestHeadObject_NotFound(t *testing.T) {
	// The S3 SDK returns *types.NotFound for a missing object.
	mock := &mockHeadAPI{err: &types.NotFound{}}
	c := &S3Client{headAPI: mock}

	exists, err := c.HeadObject(context.Background(), "test-bucket", "missing-key")
	if err != nil {
		t.Fatalf("expected no error for not-found, got: %v", err)
	}
	if exists {
		t.Error("expected exists=false when the object is missing")
	}
}

func TestHeadObject_NotFoundWrapped(t *testing.T) {
	// errors.As should unwrap a wrapped NotFound as well.
	wrapped := errors.Join(errors.New("operation error S3: HeadObject"), &types.NotFound{})
	mock := &mockHeadAPI{err: wrapped}
	c := &S3Client{headAPI: mock}

	exists, err := c.HeadObject(context.Background(), "test-bucket", "missing-key")
	if err != nil {
		t.Fatalf("expected no error for wrapped not-found, got: %v", err)
	}
	if exists {
		t.Error("expected exists=false when the object is missing")
	}
}

func TestHeadObject_OtherErrorPassthrough(t *testing.T) {
	sentinel := errors.New("access denied")
	mock := &mockHeadAPI{err: sentinel}
	c := &S3Client{headAPI: mock}

	exists, err := c.HeadObject(context.Background(), "test-bucket", "some-key")
	if err == nil {
		t.Fatal("expected an error to be passed through")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("expected the underlying error to pass through, got: %v", err)
	}
	if exists {
		t.Error("expected exists=false on error")
	}
}

func TestHeadObject_NoSuchKey(t *testing.T) {
	mock := &mockHeadAPI{err: &types.NoSuchKey{}}
	c := &S3Client{headAPI: mock}

	exists, err := c.HeadObject(context.Background(), "test-bucket", "missing-key")
	if err != nil {
		t.Fatalf("expected no error for no-such-key, got: %v", err)
	}
	if exists {
		t.Error("expected exists=false when the object is missing")
	}
}

func TestHeadObject_NotFoundAPIErrorCodes(t *testing.T) {
	// HeadObject on a missing key surfaces as an unmodeled Smithy APIError
	// (no response body), not *types.NotFound. S3 returns 403 Forbidden
	// instead of 404 when the caller lacks s3:ListBucket — the exact failure
	// from the shorts-render Fargate task:
	//   "operation error S3: HeadObject, ... StatusCode: 403, ... api error Forbidden".
	for _, code := range []string{"NotFound", "NoSuchKey", "NoSuchBucket", "Forbidden", "AccessDenied"} {
		t.Run(code, func(t *testing.T) {
			err := fmt.Errorf("operation error S3: HeadObject: %w",
				&smithy.GenericAPIError{Code: code, Message: code})
			mock := &mockHeadAPI{err: err}
			c := &S3Client{headAPI: mock}

			exists, err := c.HeadObject(context.Background(), "test-bucket", "missing-key")
			if err != nil {
				t.Fatalf("expected no error for code %q, got: %v", code, err)
			}
			if exists {
				t.Errorf("expected exists=false for code %q", code)
			}
		})
	}
}

func TestHeadObject_RealErrorPassthrough(t *testing.T) {
	// A genuine service failure (e.g. 500) must still surface as an error.
	err := fmt.Errorf("operation error S3: HeadObject: %w",
		&smithy.GenericAPIError{Code: "InternalError", Message: "boom"})
	mock := &mockHeadAPI{err: err}
	c := &S3Client{headAPI: mock}

	exists, err := c.HeadObject(context.Background(), "test-bucket", "some-key")
	if err == nil {
		t.Fatal("expected an error to be passed through")
	}
	if exists {
		t.Error("expected exists=false on error")
	}
}
