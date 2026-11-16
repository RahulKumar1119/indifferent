package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/rahul/indifferent/backend/internal/models"
	"github.com/rahul/indifferent/backend/internal/videocopy"
)

// Titles, a description and chapters for a finished video (#331).
//
//	GET  /shorts/{id}/copy/status   whether it is on, whether the video has a transcript, uses left today
//	POST /shorts/{id}/copy          { topic?, transcript?, useJobTranscript?, refine?, previousTitles? }
//
// Nothing is stored: the result goes back to the page and nowhere else. The
// text sent to the model (a topic, pasted text, or the video's own transcript)
// is never logged.

const (
	// copyMaxBody bounds the request before it is parsed: 20,000 characters of
	// Devanagari are 60 KB in UTF-8, and JSON escaping can double that.
	copyMaxBody = 160 << 10
	// copyMaxTranscriptBytes refuses a stored transcript too large to read into
	// the Lambda for one prompt (about two and a half hours of speech is 1 MB).
	copyMaxTranscriptBytes = 12 << 20
)

// CopyWriter writes the copy for a video. *videocopy.Service implements it.
type CopyWriter interface {
	Generate(ctx context.Context, r videocopy.Request) (videocopy.Result, error)
	Model() string
}

type copyRoute struct {
	jobID  string
	status bool
}

// parseCopyPath recognises /shorts/{id}/copy and /shorts/{id}/copy/status.
func parseCopyPath(path string) (copyRoute, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 3 || parts[0] != "shorts" || parts[1] == "" || parts[2] != "copy" {
		return copyRoute{}, false
	}
	switch len(parts) {
	case 3:
		return copyRoute{jobID: parts[1]}, true
	case 4:
		if parts[3] == "status" {
			return copyRoute{jobID: parts[1], status: true}, true
		}
	}
	return copyRoute{}, false
}

func isCopyPath(path string) bool {
	_, ok := parseCopyPath(path)
	return ok
}

// CopyRequest is the body of POST /shorts/{id}/copy.
type CopyRequest struct {
	Topic string `json:"topic"`
	// Transcript is pasted text. Lines that begin with a time give it timestamps.
	Transcript string `json:"transcript"`
	// UseJobTranscript uses this video's own transcript, with its real times.
	UseJobTranscript bool `json:"useJobTranscript"`
	// Refine and PreviousTitles ask for different titles.
	Refine         string   `json:"refine"`
	PreviousTitles []string `json:"previousTitles"`
}

// CopyChapter is one chapter of the response.
type CopyChapter struct {
	Start float64 `json:"start"`
	Time  string  `json:"time"`
	Title string  `json:"title"`
}

// CopyResponse is the body of a successful POST /shorts/{id}/copy.
type CopyResponse struct {
	Titles                  []string      `json:"titles"`
	Description             string        `json:"description"`
	Chapters                []CopyChapter `json:"chapters"`
	DescriptionWithChapters string        `json:"descriptionWithChapters"`
	HasTimestamps           bool          `json:"hasTimestamps"`
	Notes                   []string      `json:"notes"`
	RemainingToday          *int64        `json:"remainingToday,omitempty"`
	DailyLimit              int64         `json:"dailyLimit"`
}

// CopyStatus is the body of GET /shorts/{id}/copy/status.
type CopyStatus struct {
	Enabled bool `json:"enabled"`
	// TranscriptAvailable says this video has a transcript the helper can read.
	TranscriptAvailable bool   `json:"transcriptAvailable"`
	RemainingToday      *int64 `json:"remainingToday,omitempty"`
	DailyLimit          int64  `json:"dailyLimit"`
	MaxTopicChars       int    `json:"maxTopicChars"`
	MaxTranscriptChars  int    `json:"maxTranscriptChars"`
	MaxRefineChars      int    `json:"maxRefineChars"`
}

func (h *APIHandler) handleCopy(ctx context.Context, req events.APIGatewayProxyRequest, claims *models.JWTClaims) (events.APIGatewayProxyResponse, error) {
	route, _ := parseCopyPath(req.Path)
	switch {
	case req.HTTPMethod == http.MethodGet && route.status:
		return h.handleCopyStatus(ctx, route, claims)
	case req.HTTPMethod == http.MethodPost && !route.status:
		return h.handleCopyWrite(ctx, req, route, claims)
	}
	return errorResponse(http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed"), nil
}

// jobTranscriptKey returns where this job's transcript is, or "" when it has none.
// Older jobs predate transcriptKey, so the conventional place is tried too.
func (h *APIHandler) jobTranscriptKey(ctx context.Context, userID string, job *models.ShortsJob) string {
	if job.TranscriptKey != "" {
		return job.TranscriptKey
	}
	conventional := fmt.Sprintf("shorts/%s/%s/transcript.json", userID, job.JobID)
	if meta, err := h.S3.ObjectMeta(ctx, h.Bucket, conventional); err == nil && meta.Found {
		return conventional
	}
	return ""
}

// quotaUsedToday reads how many uses of q the user has today, or -1 when unknown.
func (h *APIHandler) quotaUsedToday(ctx context.Context, userID string, q quota) int64 {
	if h.QuotaTable == "" {
		return -1
	}
	out, err := h.DB.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(h.QuotaTable),
		Key: map[string]dbtypes.AttributeValue{
			"PK": &dbtypes.AttributeValueMemberS{Value: "QUOTA#" + userID},
			"SK": &dbtypes.AttributeValueMemberS{Value: q.name + "#" + quotaNow().UTC().Format("20060102")},
		},
	})
	if err != nil {
		return -1
	}
	if out == nil || len(out.Item) == 0 {
		return 0
	}
	n, ok := out.Item["uses"].(*dbtypes.AttributeValueMemberN)
	if !ok {
		return -1
	}
	used, err := strconv.ParseInt(n.Value, 10, 64)
	if err != nil {
		return -1
	}
	return used
}

func remainingOf(q quota, used int64) *int64 {
	if used < 0 {
		return nil
	}
	r := q.limit - used
	if r < 0 {
		r = 0
	}
	return &r
}

func (h *APIHandler) handleCopyStatus(ctx context.Context, route copyRoute, claims *models.JWTClaims) (events.APIGatewayProxyResponse, error) {
	job, err := h.getShortsJobByID(ctx, claims.UserID, route.jobID)
	if err != nil {
		return errorResponse(http.StatusNotFound, "NOT_FOUND", "Shorts job not found"), nil
	}
	st := CopyStatus{
		Enabled:            h.CopyWriter != nil,
		DailyLimit:         quotaCopyHelper.limit,
		MaxTopicChars:      videocopy.MaxTopicRunes,
		MaxTranscriptChars: videocopy.MaxTranscriptRunes,
		MaxRefineChars:     videocopy.MaxRefineRunes,
	}
	if st.Enabled {
		st.TranscriptAvailable = h.jobTranscriptKey(ctx, claims.UserID, job) != ""
		st.RemainingToday = remainingOf(quotaCopyHelper, h.quotaUsedToday(ctx, claims.UserID, quotaCopyHelper))
	}
	return jsonResponse(http.StatusOK, st), nil
}

func (h *APIHandler) handleCopyWrite(ctx context.Context, req events.APIGatewayProxyRequest, route copyRoute, claims *models.JWTClaims) (events.APIGatewayProxyResponse, error) {
	if h.CopyWriter == nil {
		return errorResponse(http.StatusServiceUnavailable, "UNAVAILABLE", "Writing titles and descriptions is not available right now."), nil
	}
	if len(req.Body) > copyMaxBody {
		return errorResponse(http.StatusBadRequest, "VALIDATION_ERROR", "The request is too large."), nil
	}
	var body CopyRequest
	if err := json.Unmarshal([]byte(req.Body), &body); err != nil {
		return errorResponse(http.StatusBadRequest, "INVALID_BODY", "Invalid request body"), nil
	}
	job, err := h.getShortsJobByID(ctx, claims.UserID, route.jobID)
	if err != nil {
		return errorResponse(http.StatusNotFound, "NOT_FOUND", "Shorts job not found"), nil
	}

	cr := videocopy.Request{
		Topic:      body.Topic,
		Transcript: body.Transcript,
		Duration:   job.SourceDuration,
		Refine:     body.Refine,
		Previous:   body.PreviousTitles,
	}
	if body.UseJobTranscript {
		words, resp := h.loadCopyWords(ctx, claims.UserID, job)
		if resp != nil {
			return *resp, nil
		}
		cr.Words = words
	}
	// A request that cannot succeed costs nothing.
	if err := videocopy.Check(cr); err != nil {
		return errorResponse(http.StatusBadRequest, "VALIDATION_ERROR", videocopy.InputReason(err)), nil
	}

	used, retryAfter, ok := h.spendQuotaCount(ctx, claims.UserID, quotaCopyHelper)
	if !ok {
		return quotaExceeded(quotaCopyHelper, retryAfter), nil
	}
	started := time.Now()
	res, genErr := h.CopyWriter.Generate(ctx, cr)
	if genErr != nil {
		// A person who closed the page made the call, so the use stays. Any other
		// failure is not the user's doing: give the use back.
		if !errors.Is(genErr, context.Canceled) {
			h.refundQuota(ctx, claims.UserID, quotaCopyHelper)
		}
		log.Printf("copy_failed user=%s job=%s ms=%d err=%v", claims.UserID, route.jobID, time.Since(started).Milliseconds(), genErr)
		return copyErrorResponse(genErr), nil
	}
	log.Printf("copy_ok user=%s job=%s ms=%d titles=%d chapters=%d timed=%t", claims.UserID, route.jobID, time.Since(started).Milliseconds(), len(res.Titles), len(res.Chapters), res.HasTimestamps)

	out := CopyResponse{
		Titles:                  res.Titles,
		Description:             res.Description,
		Chapters:                make([]CopyChapter, 0, len(res.Chapters)),
		DescriptionWithChapters: res.DescriptionWithChapters,
		HasTimestamps:           res.HasTimestamps,
		Notes:                   append([]string{}, res.Notes...),
		RemainingToday:          remainingOf(quotaCopyHelper, used),
		DailyLimit:              quotaCopyHelper.limit,
	}
	for _, c := range res.Chapters {
		out.Chapters = append(out.Chapters, CopyChapter{Start: c.Start, Time: c.Time, Title: c.Title})
	}
	return jsonResponse(http.StatusOK, out), nil
}

// loadCopyWords reads this video's transcript. The second result is a ready
// error response when it cannot be used.
func (h *APIHandler) loadCopyWords(ctx context.Context, userID string, job *models.ShortsJob) ([]videocopy.Word, *events.APIGatewayProxyResponse) {
	fail := func(status int, code, msg string) *events.APIGatewayProxyResponse {
		r := errorResponse(status, code, msg)
		return &r
	}
	key := h.jobTranscriptKey(ctx, userID, job)
	if key == "" {
		return nil, fail(http.StatusBadRequest, "NOT_READY", "This video has no transcript yet. Paste one, or describe the video in a sentence.")
	}
	raw, err := h.S3.GetObject(ctx, h.Bucket, key)
	if err != nil {
		log.Printf("copy_transcript_read_failed user=%s job=%s err=%v", userID, job.JobID, err)
		return nil, fail(http.StatusInternalServerError, "S3_ERROR", "Could not read this video's transcript. Please try again.")
	}
	if len(raw) > copyMaxTranscriptBytes {
		return nil, fail(http.StatusBadRequest, "VALIDATION_ERROR", "This video's transcript is too long to use. Paste the part you want, or describe the video in a sentence.")
	}
	var tr models.Transcript
	if err := json.Unmarshal(raw, &tr); err != nil {
		return nil, fail(http.StatusInternalServerError, "S3_ERROR", "This video's transcript could not be read.")
	}
	words := make([]videocopy.Word, 0, len(tr.Words))
	for _, w := range tr.Words {
		words = append(words, videocopy.Word{Text: w.Text, Start: w.Start, End: w.End})
	}
	if len(words) == 0 {
		return nil, fail(http.StatusBadRequest, "NOT_READY", "This video's transcript has no words in it.")
	}
	return words, nil
}

func copyErrorResponse(err error) events.APIGatewayProxyResponse {
	switch {
	case errors.Is(err, videocopy.ErrInvalidInput):
		return errorResponse(http.StatusBadRequest, "VALIDATION_ERROR", videocopy.InputReason(err))
	case errors.Is(err, videocopy.ErrModelTimeout):
		return errorResponse(http.StatusGatewayTimeout, "COPY_TIMEOUT", "Writing took too long. Please try again. This did not use up one of today's tries.")
	case errors.Is(err, videocopy.ErrModelFailed):
		return errorResponse(http.StatusServiceUnavailable, "COPY_UNAVAILABLE", "The writer is busy right now. Please try again in a minute. This did not use up one of today's tries.")
	case errors.Is(err, videocopy.ErrInvalidAnswer):
		return errorResponse(http.StatusBadGateway, "COPY_FAILED", "The writer did not give a usable answer. Please try again. This did not use up one of today's tries.")
	}
	return errorResponse(http.StatusInternalServerError, "INTERNAL_ERROR", "Could not write the titles. Please try again.")
}
