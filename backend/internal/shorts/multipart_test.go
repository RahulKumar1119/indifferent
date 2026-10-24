package shorts

import "testing"

func TestMultipartPartCount(t *testing.T) {
	tests := []struct {
		size int64
		want int
	}{
		{1, 1},
		{MultipartPartSize, 1},
		{MultipartPartSize + 1, 2},
		{2 * MultipartPartSize, 2},
		{MaxUploadBytes, 128},
	}
	for _, tt := range tests {
		if got := MultipartPartCount(tt.size); got != tt.want {
			t.Errorf("MultipartPartCount(%d) = %d, want %d", tt.size, got, tt.want)
		}
	}
}

func TestMaxUploadFitsInS3PartLimit(t *testing.T) {
	if got := MultipartPartCount(MaxUploadBytes); got > MaxMultipartParts {
		t.Errorf("a maximum upload needs %d parts, S3 allows %d", got, MaxMultipartParts)
	}
	if MultipartPartSize < 5*1024*1024 {
		t.Errorf("S3 requires parts of at least 5 MiB, got %d", MultipartPartSize)
	}
}
