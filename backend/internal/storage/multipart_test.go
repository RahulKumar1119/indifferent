package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

func TestIsNoSuchUpload(t *testing.T) {
	wrapped := fmt.Errorf("operation error S3: ListParts: %w", &types.NoSuchUpload{})
	coded := fmt.Errorf("operation error S3: UploadPart: %w", &smithy.GenericAPIError{Code: "NoSuchUpload"})
	for name, err := range map[string]error{"modeled": &types.NoSuchUpload{}, "wrapped": wrapped, "by code": coded} {
		if !IsNoSuchUpload(err) {
			t.Errorf("%s: expected NoSuchUpload to be recognized", name)
		}
	}
	for name, err := range map[string]error{
		"nil":        nil,
		"plain":      errors.New("boom"),
		"other code": &smithy.GenericAPIError{Code: "InternalError"},
		"no key":     &types.NoSuchKey{},
	} {
		if IsNoSuchUpload(err) {
			t.Errorf("%s: must not be treated as a missing upload", name)
		}
	}
}

func TestIsInvalidParts(t *testing.T) {
	for _, code := range []string{"InvalidPart", "InvalidPartOrder", "EntityTooSmall"} {
		err := fmt.Errorf("operation error S3: CompleteMultipartUpload: %w", &smithy.GenericAPIError{Code: code})
		if !IsInvalidParts(err) {
			t.Errorf("%s: expected to be recognized", code)
		}
	}
	for _, err := range []error{nil, errors.New("boom"), &smithy.GenericAPIError{Code: "NoSuchUpload"}, &smithy.GenericAPIError{Code: "InternalError"}} {
		if IsInvalidParts(err) {
			t.Errorf("%v: must not be treated as invalid parts", err)
		}
	}
}

// recordingHTTP answers every S3 request with a bare 200 and remembers the
// host it was sent to, so a test can see which endpoint a call would use.
type recordingHTTP struct {
	mu    sync.Mutex
	hosts []string
}

func (r *recordingHTTP) Do(req *http.Request) (*http.Response, error) {
	r.mu.Lock()
	r.hosts = append(r.hosts, req.URL.Host)
	r.mu.Unlock()
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{},
		Body:       io.NopCloser(strings.NewReader("<Result></Result>")),
		Request:    req,
	}, nil
}

// newRecordedClient builds an S3Client for the bucket's region with fixed
// credentials and no network, so the tests do not depend on the machine's AWS
// setup.
func newRecordedClient() (*S3Client, *recordingHTTP) {
	rec := &recordingHTTP{}
	c := s3.New(s3.Options{
		Region: "ap-south-1",
		Credentials: aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
			return aws.Credentials{AccessKeyID: "AKIAEXAMPLE", SecretAccessKey: "secret"}, nil
		}),
		HTTPClient: rec,
	})
	return &S3Client{client: c, headAPI: c}, rec
}

func TestGenerateUploadPartURL_Host(t *testing.T) {
	const key = "uploads/u1/j1/source.mp4"
	tests := []struct {
		name       string
		accelerate bool
		wantHost   string
	}{
		{"regional", false, "test-bucket.s3.ap-south-1.amazonaws.com"},
		{"accelerated", true, "test-bucket.s3-accelerate.amazonaws.com"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, rec := newRecordedClient()
			raw, err := c.GenerateUploadPartURL(context.Background(), "test-bucket", key, "up-123", 7, PartURLExpiration, tt.accelerate)
			if err != nil {
				t.Fatalf("GenerateUploadPartURL returned error: %v", err)
			}
			u, err := url.Parse(raw)
			if err != nil {
				t.Fatalf("not a URL: %s", raw)
			}
			if u.Scheme != "https" || u.Host != tt.wantHost || u.Path != "/"+key {
				t.Errorf("got %s://%s%s, want https://%s/%s", u.Scheme, u.Host, u.Path, tt.wantHost, key)
			}
			q := u.Query()
			if q.Get("partNumber") != "7" || q.Get("uploadId") != "up-123" || q.Get("X-Amz-Expires") != "3600" {
				t.Errorf("part query lost in %s", raw)
			}
			// Same signature scope as the regional URL, and still nothing but
			// host signed: the browser sends the part with no extra headers.
			if got := q.Get("X-Amz-SignedHeaders"); got != "host" {
				t.Errorf("X-Amz-SignedHeaders = %q, want host", got)
			}
			if !strings.Contains(q.Get("X-Amz-Credential"), "/ap-south-1/s3/aws4_request") {
				t.Errorf("credential scope = %q, want region ap-south-1", q.Get("X-Amz-Credential"))
			}
			if strings.Contains(strings.ToLower(raw), "checksum") {
				t.Errorf("part URL must not demand a checksum header the browser cannot send: %s", raw)
			}
			if len(rec.hosts) != 0 {
				t.Errorf("presigning must not touch the network, saw %v", rec.hosts)
			}
		})
	}
}

// Acceleration only helps the browser. Every call the Lambda makes itself must
// keep the regional endpoint, also after accelerated URLs were signed from the
// same client.
func TestServerCallsStayRegional(t *testing.T) {
	ctx := context.Background()
	c, rec := newRecordedClient()
	if _, err := c.GenerateUploadPartURL(ctx, "test-bucket", "k", "up-1", 1, PartURLExpiration, true); err != nil {
		t.Fatal(err)
	}
	calls := map[string]func() error{
		"CreateMultipartUpload": func() error { _, err := c.CreateMultipartUpload(ctx, "test-bucket", "k", "video/mp4"); return err },
		"ListParts":             func() error { _, err := c.ListUploadedParts(ctx, "test-bucket", "k", "up-1"); return err },
		"CompleteMultipart": func() error {
			return c.CompleteMultipartUpload(ctx, "test-bucket", "k", "up-1", []CompletedPart{{PartNumber: 1, ETag: `"e"`}})
		},
		"AbortMultipartUpload": func() error { return c.AbortMultipartUpload(ctx, "test-bucket", "k", "up-1") },
		"HeadObject":           func() error { _, err := c.HeadObject(ctx, "test-bucket", "k"); return err },
		"ObjectMeta":           func() error { _, err := c.ObjectMeta(ctx, "test-bucket", "k"); return err },
	}
	for name, call := range calls {
		before := len(rec.hosts)
		if err := call(); err != nil {
			t.Errorf("%s: %v", name, err)
		}
		if len(rec.hosts)-before != 1 {
			t.Errorf("%s: %d requests, want 1", name, len(rec.hosts)-before)
		}
	}
	for _, h := range rec.hosts {
		if h != "test-bucket.s3.ap-south-1.amazonaws.com" {
			t.Errorf("server call went to %s, want the regional endpoint", h)
		}
	}
	// The single PUT for small files is untouched as well.
	single, err := c.GenerateUploadURL(ctx, "test-bucket", "k", "video/mp4", UploadURLExpiration)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(single, "https://test-bucket.s3.ap-south-1.amazonaws.com/") {
		t.Errorf("single PUT URL = %s, want the regional endpoint", single)
	}
	if len(rec.hosts) != 6 {
		t.Errorf("saw %d requests, want 6: %v", len(rec.hosts), rec.hosts)
	}
}
