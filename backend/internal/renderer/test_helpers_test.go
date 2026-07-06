package renderer

import (
	"context"
	"fmt"
	"strings"
)

// mockStorageClient implements StorageClient for testing across all test files.
type mockStorageClient struct {
	objects     map[string][]byte
	getFunc     func(ctx context.Context, bucket, key string) ([]byte, error)
	putCalls    []putCall
	deleteCalls []string
	putErr      error
	deleteErr   error
	getData     []byte // if set, GetObject returns this for all keys
}

type putCall struct {
	bucket      string
	key         string
	data        []byte
	contentType string
}

func newMockStorageClient() *mockStorageClient {
	return &mockStorageClient{
		objects: make(map[string][]byte),
	}
}

func (m *mockStorageClient) GetObject(ctx context.Context, bucket, key string) ([]byte, error) {
	if m.getFunc != nil {
		return m.getFunc(ctx, bucket, key)
	}
	// If getData is set, return it for all keys
	if m.getData != nil {
		return m.getData, nil
	}
	data, ok := m.objects[key]
	if !ok {
		return nil, fmt.Errorf("object not found: %s", key)
	}
	return data, nil
}

func (m *mockStorageClient) PutObject(ctx context.Context, bucket, key string, data []byte, contentType string) error {
	m.putCalls = append(m.putCalls, putCall{
		bucket:      bucket,
		key:         key,
		data:        data,
		contentType: contentType,
	})
	if m.putErr != nil {
		return m.putErr
	}
	if m.objects == nil {
		m.objects = make(map[string][]byte)
	}
	m.objects[key] = data
	return nil
}

func (m *mockStorageClient) DeleteObject(_ context.Context, bucket, key string) error {
	m.deleteCalls = append(m.deleteCalls, key)
	return m.deleteErr
}

// failingGetStorageClient always fails on GetObject.
type failingGetStorageClient struct{}

func (f *failingGetStorageClient) GetObject(_ context.Context, bucket, key string) ([]byte, error) {
	return nil, fmt.Errorf("S3 download error for %s", key)
}

func (f *failingGetStorageClient) PutObject(_ context.Context, bucket, key string, data []byte, contentType string) error {
	return nil
}

func (f *failingGetStorageClient) DeleteObject(_ context.Context, bucket, key string) error {
	return nil
}

// selectiveFailStorageClient fails on keys starting with failPrefix.
type selectiveFailStorageClient struct {
	failPrefix string
}

func (s *selectiveFailStorageClient) GetObject(_ context.Context, bucket, key string) ([]byte, error) {
	if strings.HasPrefix(key, s.failPrefix) {
		return nil, fmt.Errorf("S3 download error for %s", key)
	}
	return []byte("file-data"), nil
}

func (s *selectiveFailStorageClient) PutObject(_ context.Context, bucket, key string, data []byte, contentType string) error {
	return nil
}

func (s *selectiveFailStorageClient) DeleteObject(_ context.Context, bucket, key string) error {
	return nil
}

// assertContainsStr checks that a slice contains a given string.
func assertContainsStr(t interface{ Helper(); Errorf(string, ...any) }, slice []string, item string) {
	t.Helper()
	for _, s := range slice {
		if s == item {
			return
		}
	}
	t.Errorf("expected slice to contain '%s', got %v", item, slice)
}
