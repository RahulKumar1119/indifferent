package shorts

// Multipart upload limits for source media (issue #151).
const (
	// MultipartPartSize is the size of every part except the last. Sources up
	// to this size keep the single presigned PUT.
	MultipartPartSize = 16 * 1024 * 1024
	// MaxMultipartParts is the S3 limit on parts in one upload.
	MaxMultipartParts = 10000
	// MaxPartURLBatch caps how many part URLs one request may ask for.
	MaxPartURLBatch = 50
)

// MultipartPartCount is the number of parts a source of the given size needs.
func MultipartPartCount(size int64) int {
	return int((size + MultipartPartSize - 1) / MultipartPartSize)
}
