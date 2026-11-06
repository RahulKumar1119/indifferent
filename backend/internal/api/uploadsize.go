package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/aws/aws-lambda-go/events"

	"github.com/rahul/indifferent/backend/internal/shorts"
	"github.com/rahul/indifferent/backend/internal/storage"
)

const (
	// maxTextUploadBytes and maxLogoUploadBytes match the limits the web app
	// already enforces before it uploads.
	maxTextUploadBytes = 5 << 20
	maxLogoUploadBytes = 5 << 20
)

// uploadURL presigns a PUT for key. When the client declared the file size
// (size > 0) the size, already checked by checkUploadSize, is signed into the URL, so S3
// itself refuses a body of any other length. When it did not (size == 0) the
// URL is unsized, as before: older web app versions do not send a size yet.
// That fallback is logged so it can be removed once the log goes quiet.
func (h *APIHandler) uploadURL(ctx context.Context, kind, key, contentType string, size int64) (string, error) {
	if size <= 0 {
		log.Printf("upload_url_unsized kind=%s", kind)
		return h.S3.GenerateUploadURL(ctx, h.Bucket, key, contentType, storage.UploadURLExpiration)
	}
	return h.S3.GenerateSizedUploadURL(ctx, h.Bucket, key, contentType, size, storage.UploadURLExpiration)
}

// checkUploadSize validates a declared size: zero means "not declared"
// (accepted for now), negative or over max is refused.
func checkUploadSize(size, max int64, what string) *events.APIGatewayProxyResponse {
	if size == 0 {
		return nil
	}
	if size < 0 {
		resp := errorResponse(http.StatusBadRequest, "VALIDATION_ERROR", "size must be greater than zero")
		return &resp
	}
	if size > max {
		resp := errorResponse(http.StatusBadRequest, "UPLOAD_TOO_LARGE",
			fmt.Sprintf("%s is %.1f MB; the maximum is %d MB.", what, float64(size)/1024/1024, max/1024/1024))
		return &resp
	}
	return nil
}

// shortsMaxUploadBytes is the cap for a shorts source declared at creation.
const shortsMaxUploadBytes = int64(shorts.MaxUploadBytes)

// declaredSize reads an optional {"size": N} from a request body. Missing,
// unparseable or non-numeric bodies give 0, meaning "not declared": the upload
// routes have always accepted empty and unrelated bodies.
func declaredSize(body string) int64 {
	var in struct {
		Size int64 `json:"size"`
	}
	if strings.TrimSpace(body) == "" || json.Unmarshal([]byte(body), &in) != nil {
		return 0
	}
	return in.Size
}
