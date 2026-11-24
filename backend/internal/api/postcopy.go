package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/rahul/indifferent/backend/internal/models"
	"github.com/rahul/indifferent/backend/internal/storyjob"
	"github.com/rahul/indifferent/backend/internal/videocopy"
)

// Copy for each platform, for one clip or one story video (#330).
//
//	GET  /shorts/{id}/clips/{clipId}/post-copy   the saved copy, the platform limits, uses left
//	POST /shorts/{id}/clips/{clipId}/post-copy   { requestId }   write it (one use of the shared daily quota)
//	PUT  /shorts/{id}/clips/{clipId}/post-copy   { copy }        save what the person edited
//	GET|POST|PUT /story-videos/{id}/post-copy                    the same, for a finished story video
//
// The latest copy, generated or edited, is one small record beside the clip or
// the video, and is deleted with it. The text sent to the model (the clip's
// words or the story's narration) is never logged, and nothing is posted
// anywhere.
const (
	// postCopyMaxBody bounds a request before it is parsed. Three platforms of
	// 5,000 bytes each, JSON-escaped, are well under it.
	postCopyMaxBody = 96 << 10
	// postCopyPendingStale is how long a request may stay "being written" before
	// a retry with the same id is allowed to start over: longer than the model
	// call can take, so a crashed attempt does not block the person for good.
	postCopyPendingStale = 90 * time.Second
	// postCopyReplayKeep is how long a finished request's answer is kept for a
	// retry that did not see it.
	postCopyReplayKeep = 24 * time.Hour
)

var requestIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{16,64}$`)

// PostCopyWriter writes the per-platform copy. *videocopy.Service implements it.
type PostCopyWriter interface {
	GeneratePost(ctx context.Context, r videocopy.Request, kind videocopy.Kind, specs []videocopy.PlatformSpec) (videocopy.PostResult, error)
}

func (h *APIHandler) postCopySpecs() []videocopy.PlatformSpec {
	if len(h.PostCopySpecs) > 0 {
		return h.PostCopySpecs
	}
	return videocopy.Platforms()
}

// postCopyTarget is the one thing a copy belongs to.
type postCopyTarget struct {
	kind   videocopy.Kind
	jobID  string
	clipID string
}

// sk is the copy's sort key. It sits beside the job in the user's partition.
func (t postCopyTarget) sk() string {
	if t.kind == videocopy.KindStory {
		return "POSTCOPY#STORY#" + t.jobID
	}
	return "POSTCOPY#CLIP#" + t.jobID + "#" + t.clipID
}

// parsePostCopyPath recognises the clip and story routes. An id with a '#' is
// refused: it would build a key that points at another record.
func parsePostCopyPath(path string) (postCopyTarget, bool) {
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	switch {
	case len(parts) == 5 && parts[0] == "shorts" && parts[2] == "clips" && parts[4] == "post-copy":
		if parts[1] == "" || parts[3] == "" || strings.Contains(parts[1]+parts[3], "#") {
			return postCopyTarget{}, false
		}
		return postCopyTarget{kind: videocopy.KindClip, jobID: parts[1], clipID: parts[3]}, true
	case len(parts) == 3 && parts[0] == "story-videos" && parts[2] == "post-copy":
		if !storyjob.ValidJobID(parts[1]) {
			return postCopyTarget{}, false
		}
		return postCopyTarget{kind: videocopy.KindStory, jobID: strings.ToLower(parts[1])}, true
	}
	return postCopyTarget{}, false
}

func isPostCopyPath(path string) bool {
	_, ok := parsePostCopyPath(path)
	return ok
}

// PostCopySaved is a copy as stored and as returned.
type PostCopySaved struct {
	Copy  map[videocopy.Platform]videocopy.PlatformCopy `json:"copy"`
	Notes []string                                      `json:"notes"`
	// Source is "generated" until the person saves an edit, then "edited".
	Source      string `json:"source"`
	GeneratedAt string `json:"generatedAt,omitempty"`
	UpdatedAt   string `json:"updatedAt"`
}

// PostCopyView is the body of GET.
type PostCopyView struct {
	Enabled        bool                     `json:"enabled"`
	Platforms      []videocopy.PlatformSpec `json:"platforms"`
	Saved          *PostCopySaved           `json:"saved"`
	RemainingToday *int64                   `json:"remainingToday,omitempty"`
	DailyLimit     int64                    `json:"dailyLimit"`
}

// PostCopyRequest is the body of POST: only an id for the attempt. Everything
// that is written about comes from the server's own record of the clip or video.
type PostCopyRequest struct {
	RequestID string `json:"requestId"`
}

// PostCopySaveRequest is the body of PUT.
type PostCopySaveRequest struct {
	Copy map[videocopy.Platform]videocopy.PlatformCopy `json:"copy"`
}

// PostCopyResponse is the body of a successful POST.
type PostCopyResponse struct {
	PostCopySaved
	RemainingToday *int64 `json:"remainingToday,omitempty"`
	DailyLimit     int64  `json:"dailyLimit"`
	// Replayed says this is the answer to an earlier try with the same id, and
	// that this try used nothing.
	Replayed bool `json:"replayed"`
}

func (h *APIHandler) handlePostCopy(ctx context.Context, req events.APIGatewayProxyRequest, claims *models.JWTClaims) (events.APIGatewayProxyResponse, error) {
	t, ok := parsePostCopyPath(req.Path)
	if !ok {
		return errorResponse(http.StatusNotFound, "NOT_FOUND", "Route not found"), nil
	}
	if t.kind == videocopy.KindStory && !storyjob.ValidUserID(claims.UserID) {
		return errorResponse(http.StatusBadRequest, "INVALID_INPUT", "Your account cannot use this yet"), nil
	}
	switch req.HTTPMethod {
	case http.MethodGet:
		return h.postCopyGet(ctx, t, claims)
	case http.MethodPost:
		return h.postCopyWrite(ctx, req, t, claims)
	case http.MethodPut:
		return h.postCopySave(ctx, req, t, claims)
	}
	return errorResponse(http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed"), nil
}

func postCopyOff() events.APIGatewayProxyResponse {
	return errorResponse(http.StatusServiceUnavailable, "UNAVAILABLE", "Writing for each platform is not available right now.")
}

// --- reading the thing a copy belongs to -----------------------------------

// postCopySource finds the clip or video, checks it is the caller's, and
// returns what the model reads. A target that is not the caller's is simply not
// found: every read is keyed by the caller's own partition.
func (h *APIHandler) postCopySource(ctx context.Context, userID string, t postCopyTarget) (videocopy.Request, *events.APIGatewayProxyResponse) {
	fail := func(status int, code, msg string) (videocopy.Request, *events.APIGatewayProxyResponse) {
		r := errorResponse(status, code, msg)
		return videocopy.Request{}, &r
	}
	if t.kind == videocopy.KindStory {
		j, resp := h.loadStoryVideo(ctx, userID, t.jobID)
		if resp != nil {
			return videocopy.Request{}, resp
		}
		if j.Status != storyjob.StatusCompleted {
			return fail(http.StatusConflict, "NOT_READY", "Write the post copy once the video is finished.")
		}
		parts := make([]string, 0, len(j.Scenes))
		for _, sc := range j.Scenes {
			if n := strings.TrimSpace(sc.Narration); n != "" {
				parts = append(parts, n)
			}
		}
		return videocopy.Request{Transcript: strings.Join(parts, " "), Duration: j.TotalSeconds}, nil
	}
	job, err := h.getShortsJobByID(ctx, userID, t.jobID)
	if err != nil {
		return fail(http.StatusNotFound, "NOT_FOUND", "Shorts job not found")
	}
	if !clipKnown(job, t.clipID) {
		return fail(http.StatusNotFound, "NOT_FOUND", "Clip not found")
	}
	start, end := clipWindow(job, t.clipID)
	// The words the person corrected are the words the clip shows; use them when
	// there are any, so the copy matches what the viewer reads.
	if edit, ok := job.ClipEdits[t.clipID]; ok && len(edit.Words) > 0 {
		words := make([]videocopy.Word, 0, len(edit.Words))
		for _, w := range edit.Words {
			words = append(words, videocopy.Word{Text: w.Text, Start: w.Start, End: w.End})
		}
		return videocopy.Request{Words: words, Duration: end - start}, nil
	}
	all, resp := h.loadCopyWords(ctx, userID, job)
	if resp != nil {
		return videocopy.Request{}, resp
	}
	var words []videocopy.Word
	for _, w := range all {
		if w.End < start || w.Start > end {
			continue
		}
		words = append(words, w)
	}
	if len(words) == 0 {
		return fail(http.StatusBadRequest, "NOT_READY", "This clip has no spoken words, so there is nothing to write from.")
	}
	return videocopy.Request{Words: words, Duration: end - start}, nil
}

// --- the stored copy -------------------------------------------------------

func (h *APIHandler) postCopyKey(userID string, t postCopyTarget) map[string]dbtypes.AttributeValue {
	return map[string]dbtypes.AttributeValue{
		"PK": &dbtypes.AttributeValueMemberS{Value: "USER#" + userID},
		"SK": &dbtypes.AttributeValueMemberS{Value: t.sk()},
	}
}

func (h *APIHandler) loadPostCopy(ctx context.Context, userID string, t postCopyTarget) (*PostCopySaved, error) {
	out, err := h.DB.GetItem(ctx, &dynamodb.GetItemInput{TableName: aws.String(h.TableName), Key: h.postCopyKey(userID, t), ConsistentRead: aws.Bool(true)})
	if err != nil {
		return nil, err
	}
	if len(out.Item) == 0 {
		return nil, nil
	}
	str := func(k string) string {
		v, _ := out.Item[k].(*dbtypes.AttributeValueMemberS)
		if v == nil {
			return ""
		}
		return v.Value
	}
	s := &PostCopySaved{Source: str("source"), GeneratedAt: str("generatedAt"), UpdatedAt: str("updatedAt"), Notes: []string{}}
	if err := json.Unmarshal([]byte(str("copy")), &s.Copy); err != nil || len(s.Copy) == 0 {
		return nil, fmt.Errorf("stored copy is unreadable")
	}
	_ = json.Unmarshal([]byte(str("notes")), &s.Notes)
	if s.Notes == nil {
		s.Notes = []string{}
	}
	return s, nil
}

func (h *APIHandler) storePostCopy(ctx context.Context, userID string, t postCopyTarget, s PostCopySaved, model string) error {
	cp, err := json.Marshal(s.Copy)
	if err != nil {
		return err
	}
	notes, _ := json.Marshal(s.Notes)
	item := h.postCopyKey(userID, t)
	for k, v := range map[string]string{
		"type": "POSTCOPY", "copy": string(cp), "notes": string(notes), "source": s.Source, "model": model,
		"generatedAt": s.GeneratedAt, "updatedAt": s.UpdatedAt, "kind": string(t.kind), "jobId": t.jobID, "clipId": t.clipID,
	} {
		if v != "" {
			item[k] = &dbtypes.AttributeValueMemberS{Value: v}
		}
	}
	_, err = h.DB.PutItem(ctx, &dynamodb.PutItemInput{TableName: aws.String(h.TableName), Item: item})
	return err
}

// deleteClipPostCopies removes every clip copy of a Shorts job. It is called
// before the job's own record goes, and a failure stops the delete so it can be
// tried again: a copy must never outlive the project it belongs to.
func (h *APIHandler) deleteClipPostCopies(ctx context.Context, userID, jobID string) error {
	prefix := "POSTCOPY#CLIP#" + jobID + "#"
	var start map[string]dbtypes.AttributeValue
	for {
		out, err := h.DB.Query(ctx, &dynamodb.QueryInput{
			TableName:              aws.String(h.TableName),
			KeyConditionExpression: aws.String("PK = :pk AND begins_with(SK, :sk)"),
			ExpressionAttributeValues: map[string]dbtypes.AttributeValue{
				":pk": &dbtypes.AttributeValueMemberS{Value: "USER#" + userID},
				":sk": &dbtypes.AttributeValueMemberS{Value: prefix},
			},
			ProjectionExpression: aws.String("PK, SK"),
			ExclusiveStartKey:    start,
		})
		if err != nil {
			return fmt.Errorf("list post copy: %w", err)
		}
		for _, it := range out.Items {
			if _, err := h.DB.DeleteItem(ctx, &dynamodb.DeleteItemInput{
				TableName: aws.String(h.TableName),
				Key:       map[string]dbtypes.AttributeValue{"PK": it["PK"], "SK": it["SK"]},
			}); err != nil {
				return fmt.Errorf("delete post copy: %w", err)
			}
		}
		if len(out.LastEvaluatedKey) == 0 {
			return nil
		}
		start = out.LastEvaluatedKey
	}
}

// deleteStoryPostCopy removes a story video's copy. Deleting an item that is not
// there is not an error.
func (h *APIHandler) deleteStoryPostCopy(ctx context.Context, userID, jobID string) error {
	_, err := h.DB.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(h.TableName),
		Key:       h.postCopyKey(userID, postCopyTarget{kind: videocopy.KindStory, jobID: jobID}),
	})
	if err != nil {
		return fmt.Errorf("delete post copy: %w", err)
	}
	return nil
}

// --- GET -------------------------------------------------------------------

func (h *APIHandler) postCopyGet(ctx context.Context, t postCopyTarget, claims *models.JWTClaims) (events.APIGatewayProxyResponse, error) {
	view := PostCopyView{Enabled: h.PostCopyWriter != nil, Platforms: h.postCopySpecs(), DailyLimit: quotaCopyHelper.limit}
	if !view.Enabled {
		return jsonResponse(http.StatusOK, view), nil
	}
	if _, resp := h.postCopyExists(ctx, claims.UserID, t); resp != nil {
		return *resp, nil
	}
	saved, err := h.loadPostCopy(ctx, claims.UserID, t)
	if err != nil {
		log.Printf("postcopy_read_failed user=%s kind=%s job=%s err=%v", claims.UserID, t.kind, t.jobID, err)
		return errorResponse(http.StatusInternalServerError, "DB_ERROR", "Could not read the saved copy. Please try again."), nil
	}
	view.Saved = saved
	view.RemainingToday = remainingOf(quotaCopyHelper, h.quotaUsedToday(ctx, claims.UserID, quotaCopyHelper))
	return jsonResponse(http.StatusOK, view), nil
}

// postCopyExists answers 404 unless the clip or video is the caller's.
func (h *APIHandler) postCopyExists(ctx context.Context, userID string, t postCopyTarget) (bool, *events.APIGatewayProxyResponse) {
	if t.kind == videocopy.KindStory {
		_, resp := h.loadStoryVideo(ctx, userID, t.jobID)
		return resp == nil, resp
	}
	job, err := h.getShortsJobByID(ctx, userID, t.jobID)
	if err != nil || !clipKnown(job, t.clipID) {
		r := errorResponse(http.StatusNotFound, "NOT_FOUND", "Clip not found")
		return false, &r
	}
	return true, nil
}

// --- PUT -------------------------------------------------------------------

func (h *APIHandler) postCopySave(ctx context.Context, req events.APIGatewayProxyRequest, t postCopyTarget, claims *models.JWTClaims) (events.APIGatewayProxyResponse, error) {
	if h.PostCopyWriter == nil {
		return postCopyOff(), nil
	}
	if len(req.Body) > postCopyMaxBody {
		return errorResponse(http.StatusBadRequest, "VALIDATION_ERROR", "The request is too large."), nil
	}
	var body PostCopySaveRequest
	if err := json.Unmarshal([]byte(req.Body), &body); err != nil || body.Copy == nil {
		return errorResponse(http.StatusBadRequest, "INVALID_BODY", "Invalid request body"), nil
	}
	if _, resp := h.postCopyExists(ctx, claims.UserID, t); resp != nil {
		return *resp, nil
	}
	for id, c := range body.Copy {
		if c.Hashtags == nil {
			c.Hashtags = []string{}
			body.Copy[id] = c
		}
	}
	// The same limits as generation, counted the same way. A copy that is too
	// long is refused whole and never shortened.
	if err := videocopy.ValidateSaved(h.postCopySpecs(), body.Copy); err != nil {
		return errorResponse(http.StatusBadRequest, "VALIDATION_ERROR", err.Error()), nil
	}
	prev, err := h.loadPostCopy(ctx, claims.UserID, t)
	if err != nil {
		log.Printf("postcopy_read_failed user=%s kind=%s job=%s err=%v", claims.UserID, t.kind, t.jobID, err)
		return errorResponse(http.StatusInternalServerError, "DB_ERROR", "Could not save. Please try again."), nil
	}
	saved := PostCopySaved{Copy: body.Copy, Notes: []string{}, Source: "edited", UpdatedAt: time.Now().UTC().Format(time.RFC3339)}
	if prev != nil {
		saved.GeneratedAt = prev.GeneratedAt
	}
	if err := h.storePostCopy(ctx, claims.UserID, t, saved, ""); err != nil {
		log.Printf("postcopy_save_failed user=%s kind=%s job=%s err=%v", claims.UserID, t.kind, t.jobID, err)
		return errorResponse(http.StatusInternalServerError, "DB_ERROR", "Could not save. Please try again."), nil
	}
	return jsonResponse(http.StatusOK, saved), nil
}

// --- POST ------------------------------------------------------------------

// idemKey is the record that makes a retry safe. It lives in the quota table
// (the sessions table, which expires rows by TTL on expiresAt).
func (h *APIHandler) idemKey(userID, requestID string) map[string]dbtypes.AttributeValue {
	return map[string]dbtypes.AttributeValue{
		"PK": &dbtypes.AttributeValueMemberS{Value: "COPYREQ#" + userID},
		"SK": &dbtypes.AttributeValueMemberS{Value: requestID},
	}
}

type idemRecord struct {
	found    bool
	status   string
	target   string
	started  int64
	expires  int64
	response string
}

func (h *APIHandler) readIdem(ctx context.Context, userID, requestID string) (idemRecord, error) {
	out, err := h.DB.GetItem(ctx, &dynamodb.GetItemInput{TableName: aws.String(h.QuotaTable), Key: h.idemKey(userID, requestID), ConsistentRead: aws.Bool(true)})
	if err != nil {
		return idemRecord{}, err
	}
	if len(out.Item) == 0 {
		return idemRecord{}, nil
	}
	s := func(k string) string {
		v, _ := out.Item[k].(*dbtypes.AttributeValueMemberS)
		if v == nil {
			return ""
		}
		return v.Value
	}
	n := func(k string) int64 {
		v, _ := out.Item[k].(*dbtypes.AttributeValueMemberN)
		if v == nil {
			return 0
		}
		x, _ := strconv.ParseInt(v.Value, 10, 64)
		return x
	}
	return idemRecord{found: true, status: s("status"), target: s("target"), started: n("startedAt"), expires: n("expiresAt"), response: s("response")}, nil
}

// claimIdem marks the request as being written. It fails with false when a
// live record for this id exists: finished, or still being written.
func (h *APIHandler) claimIdem(ctx context.Context, userID, requestID, target string, now time.Time) (bool, error) {
	item := h.idemKey(userID, requestID)
	item["status"] = &dbtypes.AttributeValueMemberS{Value: "pending"}
	item["target"] = &dbtypes.AttributeValueMemberS{Value: target}
	item["startedAt"] = &dbtypes.AttributeValueMemberN{Value: strconv.FormatInt(now.Unix(), 10)}
	item["expiresAt"] = &dbtypes.AttributeValueMemberN{Value: strconv.FormatInt(now.Add(postCopyReplayKeep).Unix(), 10)}
	_, err := h.DB.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:                aws.String(h.QuotaTable),
		Item:                     item,
		ConditionExpression:      aws.String("attribute_not_exists(PK) OR expiresAt < :now OR (#st = :pending AND startedAt < :stale)"),
		ExpressionAttributeNames: map[string]string{"#st": "status"},
		ExpressionAttributeValues: map[string]dbtypes.AttributeValue{
			":now":     &dbtypes.AttributeValueMemberN{Value: strconv.FormatInt(now.Unix(), 10)},
			":pending": &dbtypes.AttributeValueMemberS{Value: "pending"},
			":stale":   &dbtypes.AttributeValueMemberN{Value: strconv.FormatInt(now.Add(-postCopyPendingStale).Unix(), 10)},
		},
	})
	var ccf *dbtypes.ConditionalCheckFailedException
	if errors.As(err, &ccf) {
		return false, nil
	}
	return err == nil, err
}

func (h *APIHandler) releaseIdem(ctx context.Context, userID, requestID string) {
	if _, err := h.DB.DeleteItem(ctx, &dynamodb.DeleteItemInput{TableName: aws.String(h.QuotaTable), Key: h.idemKey(userID, requestID)}); err != nil {
		log.Printf("postcopy_idem_release_failed user=%s err=%v", userID, err)
	}
}

func (h *APIHandler) finishIdem(ctx context.Context, userID, requestID, response string, now time.Time) {
	_, err := h.DB.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:                aws.String(h.QuotaTable),
		Key:                      h.idemKey(userID, requestID),
		UpdateExpression:         aws.String("SET #st = :done, #r = :r, expiresAt = :exp"),
		ExpressionAttributeNames: map[string]string{"#st": "status", "#r": "response"},
		ExpressionAttributeValues: map[string]dbtypes.AttributeValue{
			":done": &dbtypes.AttributeValueMemberS{Value: "done"},
			":r":    &dbtypes.AttributeValueMemberS{Value: response},
			":exp":  &dbtypes.AttributeValueMemberN{Value: strconv.FormatInt(now.Add(postCopyReplayKeep).Unix(), 10)},
		},
	})
	if err != nil {
		log.Printf("postcopy_idem_finish_failed user=%s err=%v", userID, err)
	}
}

func (h *APIHandler) postCopyWrite(ctx context.Context, req events.APIGatewayProxyRequest, t postCopyTarget, claims *models.JWTClaims) (events.APIGatewayProxyResponse, error) {
	if h.PostCopyWriter == nil {
		return postCopyOff(), nil
	}
	if len(req.Body) > postCopyMaxBody {
		return errorResponse(http.StatusBadRequest, "VALIDATION_ERROR", "The request is too large."), nil
	}
	var body PostCopyRequest
	if err := json.Unmarshal([]byte(req.Body), &body); err != nil {
		return errorResponse(http.StatusBadRequest, "INVALID_BODY", "Invalid request body"), nil
	}
	if !requestIDPattern.MatchString(body.RequestID) {
		return errorResponse(http.StatusBadRequest, "VALIDATION_ERROR", "requestId must be 16 to 64 letters, digits, hyphens or underscores."), nil
	}
	src, resp := h.postCopySource(ctx, claims.UserID, t)
	if resp != nil {
		return *resp, nil
	}
	// A request that cannot succeed costs nothing.
	if err := videocopy.Check(src); err != nil {
		return errorResponse(http.StatusBadRequest, "VALIDATION_ERROR", tooLittleToWrite(t)), nil
	}

	now := quotaNow().UTC()
	if h.QuotaTable != "" {
		claimed, err := h.claimIdem(ctx, claims.UserID, body.RequestID, t.sk(), now)
		if err != nil {
			log.Printf("postcopy_idem_claim_failed user=%s err=%v", claims.UserID, err)
			return errorResponse(http.StatusInternalServerError, "DB_ERROR", "Could not start. Please try again."), nil
		}
		if !claimed {
			return h.postCopyReplay(ctx, claims.UserID, body.RequestID, t, now), nil
		}
	}

	used, retryAfter, ok := h.spendQuotaCount(ctx, claims.UserID, quotaCopyHelper)
	if !ok {
		h.releaseIdem(ctx, claims.UserID, body.RequestID)
		return quotaExceeded(quotaCopyHelper, retryAfter), nil
	}
	giveBack := func() {
		h.refundQuota(ctx, claims.UserID, quotaCopyHelper)
		h.releaseIdem(ctx, claims.UserID, body.RequestID)
	}

	started := time.Now()
	res, genErr := h.PostCopyWriter.GeneratePost(ctx, src, t.kind, h.postCopySpecs())
	if genErr != nil {
		// A person who closed the page made the call, so the use stays and the
		// attempt stays claimed. Any other failure is not the user's doing.
		if errors.Is(genErr, context.Canceled) {
			h.releaseIdem(ctx, claims.UserID, body.RequestID)
		} else {
			giveBack()
		}
		log.Printf("postcopy_failed user=%s kind=%s job=%s ms=%d err=%v", claims.UserID, t.kind, t.jobID, time.Since(started).Milliseconds(), genErr)
		return copyErrorResponse(genErr), nil
	}

	saved := PostCopySaved{Copy: res.Platforms, Notes: append([]string{}, res.Notes...), Source: "generated",
		GeneratedAt: now.Format(time.RFC3339), UpdatedAt: now.Format(time.RFC3339)}
	if err := h.storePostCopy(ctx, claims.UserID, t, saved, res.Model); err != nil {
		// Written but not kept is not a success: give the use back.
		log.Printf("postcopy_store_failed user=%s kind=%s job=%s err=%v", claims.UserID, t.kind, t.jobID, err)
		giveBack()
		return errorResponse(http.StatusInternalServerError, "DB_ERROR", "Could not save the copy. Please try again. This did not use up one of today's tries."), nil
	}
	log.Printf("postcopy_ok user=%s kind=%s job=%s ms=%d platforms=%d notes=%d", claims.UserID, t.kind, t.jobID, time.Since(started).Milliseconds(), len(res.Platforms), len(res.Notes))
	out := PostCopyResponse{PostCopySaved: saved, RemainingToday: remainingOf(quotaCopyHelper, used), DailyLimit: quotaCopyHelper.limit}
	if h.QuotaTable != "" {
		stored := out
		stored.RemainingToday = nil
		if b, err := json.Marshal(stored); err == nil {
			h.finishIdem(ctx, claims.UserID, body.RequestID, string(b), now)
		}
	}
	return jsonResponse(http.StatusOK, out), nil
}

// postCopyReplay answers a try whose id has been seen: the finished answer,
// or a request to wait while the first try is still being written. It never
// spends a use.
func (h *APIHandler) postCopyReplay(ctx context.Context, userID, requestID string, t postCopyTarget, now time.Time) events.APIGatewayProxyResponse {
	rec, err := h.readIdem(ctx, userID, requestID)
	if err != nil || !rec.found {
		return errorResponse(http.StatusConflict, "IN_PROGRESS", "That request is being handled. Wait a moment, then try again.")
	}
	if rec.target != t.sk() {
		return errorResponse(http.StatusConflict, "REQUEST_ID_REUSED", "That request id was used for something else. Start a new request.")
	}
	if rec.status != "done" {
		return errorResponse(http.StatusConflict, "IN_PROGRESS", "Still writing. Wait a moment, then try again with the same request.")
	}
	var out PostCopyResponse
	if json.Unmarshal([]byte(rec.response), &out) != nil || len(out.Copy) == 0 {
		return errorResponse(http.StatusConflict, "IN_PROGRESS", "That request is being handled. Wait a moment, then try again.")
	}
	out.Replayed = true
	out.DailyLimit = quotaCopyHelper.limit
	out.RemainingToday = remainingOf(quotaCopyHelper, h.quotaUsedToday(ctx, userID, quotaCopyHelper))
	return jsonResponse(http.StatusOK, out)
}

func tooLittleToWrite(t postCopyTarget) string {
	if t.kind == videocopy.KindStory {
		return "This story has too little text to write from."
	}
	return "There is too little spoken text in this clip to write from."
}
