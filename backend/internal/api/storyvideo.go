package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/sfn"
	"github.com/rahul/indifferent/backend/internal/models"
	"github.com/rahul/indifferent/backend/internal/storage"
	"github.com/rahul/indifferent/backend/internal/storyjob"
	"github.com/rahul/indifferent/backend/internal/storyplan"
)

// Story video endpoints (issue #193, slice 3c of #162):
//
//	GET    /story-videos/limits          what the page may offer (no sign-in needed)
//	POST   /story-videos                 make a video from a confirmed plan
//	GET    /story-videos[?planId=]       the user's videos, newest first
//	GET    /story-videos/{id}            one video's status
//	POST   /story-videos/{id}/cancel     stop a video that is being made
//	GET    /story-videos/{id}/download   a short-lived link to the finished file
//	DELETE /story-videos/{id}            remove a finished or cancelled video and its files
//
// The render itself runs elsewhere: this file writes the job record, starts a
// Step Functions execution, and reads the record back. The worker
// (cmd/storyrender) writes progress and the end state on the same record, and
// every write on either side is conditional on the job not having ended, so a
// cancel can never overwrite a finished video and a slow worker can never undo a
// cancel. See backend/internal/storyjob/README.md for the record and the pipeline.
//
// Storage: one item per job in the projects table, PK USER#{userId} and SK
// STORYVIDEO#{jobId}. The job id is a UUIDv7, whose text form sorts by creation
// time, so "the videos of the last day" is one range query and "newest first"
// needs no index (the story plans use the same trick).

const (
	storyVideoMaxBodyBytes = 4 << 10

	// storyVideoDefaultMaxActive is how many videos one user may have in the
	// making at once. Each is a 2 vCPU Fargate task, and the Spot quota in this
	// account was 30 vCPU when it was last checked.
	storyVideoDefaultMaxActive = 2
	// storyVideoDefaultPerDay is how many videos one user may start in 24 hours,
	// cancelled and failed ones included: they used Polly and Fargate too.
	storyVideoDefaultPerDay = 10
	storyVideoMaxPerDay     = 200
	storyVideoWindow        = 24 * time.Hour

	// storyVideoStaleAfter is how long a job that has not ended is believed to be
	// still running. The longest honest run is about 80 minutes (three Spot
	// attempts and one on-demand, each cut off at 20), so a job untouched for
	// two hours lost its execution without being marked failed. It stops
	// counting towards the active limit, so it cannot block its owner for ever.
	storyVideoStaleAfter = 2 * time.Hour

	// storyVideoListRead is how many of the newest records a list reads, and
	// storyVideoListMax how many it returns.
	storyVideoListRead = 100
	storyVideoListMax  = 20

	storyVideoDownloadExpiry = 15 * time.Minute
	storyVideoFileName       = "story-video.mp4"
)

// Messages stored on a job the API ends. They are shown to the user as they are.
const (
	storyVideoCancelledMessage   = "You cancelled this video."
	storyVideoStartFailedMessage = "The video could not be started. Try again."
)

// StoryVideoResponse is a job as the API returns it. It is a view, not the
// record: no storage key, no scene text, no internal error.
type StoryVideoResponse struct {
	JobID        string              `json:"jobId"`
	PlanID       string              `json:"planId"`
	PlanRevision int                 `json:"planRevision"`
	Status       string              `json:"status"`
	Progress     int                 `json:"progress"`
	Language     string              `json:"language"`
	Voice        string              `json:"voice"`
	Caption      models.CaptionStyle `json:"caption"`
	TotalSeconds float64             `json:"totalSeconds,omitempty"`
	// Error, ErrorCode and FailedStage are present only when Status is
	// "failed". ErrorCode "CANCELLED" means the user cancelled it.
	Error       string `json:"error,omitempty"`
	ErrorCode   string `json:"errorCode,omitempty"`
	FailedStage string `json:"failedStage,omitempty"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
	CompletedAt string `json:"completedAt,omitempty"`
}

// CreateStoryVideoRequest is the body of POST /story-videos. Only the plan's id
// and the revision the user was looking at come from the client: the plan itself
// is read from the user's own record, so the client cannot render text it did
// not save.
type CreateStoryVideoRequest struct {
	PlanID   string `json:"planId"`
	Revision int    `json:"revision"`
	// Voice is optional. Empty means the default voice for the plan's language.
	Voice string `json:"voice,omitempty"`
	// Caption is optional. The zero value means the default look.
	Caption models.CaptionStyle `json:"caption,omitempty"`
}

// StoryVideoLimits tells the page what it may offer, so it never drifts from
// the backend.
type StoryVideoLimits struct {
	Enabled    bool                `json:"enabled"`
	MaxActive  int                 `json:"maxActive"`
	PerDay     int                 `json:"perDay"`
	MaxSeconds float64             `json:"maxSeconds"`
	Voices     map[string][]string `json:"voices"`
}

func (h *APIHandler) storyVideoMaxActive() int {
	if h.StoryVideoMaxActive <= 0 {
		return storyVideoDefaultMaxActive
	}
	return h.StoryVideoMaxActive
}

func (h *APIHandler) storyVideosPerDay() int {
	switch {
	case h.StoryVideosPerDay <= 0:
		return storyVideoDefaultPerDay
	case h.StoryVideosPerDay > storyVideoMaxPerDay:
		return storyVideoMaxPerDay
	}
	return h.StoryVideosPerDay
}

func (h *APIHandler) storyVideoNow() time.Time {
	if h.StoryVideoNow != nil {
		return h.StoryVideoNow()
	}
	return time.Now()
}

func (h *APIHandler) storyVideoEnabled() bool {
	return h.StoryVideoStateMachineARN != "" && h.SFN != nil
}

// isStoryVideoPath reports whether path is /story-videos or under it.
func isStoryVideoPath(path string) bool {
	return path == "/story-videos" || strings.HasPrefix(path, "/story-videos/")
}

// storyVideoRoute splits a path into the job id and the action. ok is false for
// any shape that is not one of the routes, including a trailing slash and
// anything deeper than /story-videos/{id}/{cancel|download}.
func storyVideoRoute(path string) (id, action string, ok bool) {
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if parts[0] != "story-videos" {
		return "", "", false
	}
	switch len(parts) {
	case 1:
		return "", "", true
	case 2:
		return parts[1], "", parts[1] != ""
	case 3:
		return parts[1], parts[2], parts[1] != "" && (parts[2] == "cancel" || parts[2] == "download")
	}
	return "", "", false
}

// handleGetStoryVideoLimits answers GET /story-videos/limits.
func (h *APIHandler) handleGetStoryVideoLimits() events.APIGatewayProxyResponse {
	voices := map[string][]string{}
	for _, lang := range storyjob.Languages() {
		voices[lang] = storyjob.Voices(lang)
	}
	return jsonResponse(http.StatusOK, StoryVideoLimits{
		Enabled:    h.storyVideoEnabled(),
		MaxActive:  h.storyVideoMaxActive(),
		PerDay:     h.storyVideosPerDay(),
		MaxSeconds: storyplan.MaxTotalSeconds,
		Voices:     voices,
	})
}

// handleStoryVideos routes the story video endpoints other than the public
// limits one.
func (h *APIHandler) handleStoryVideos(ctx context.Context, req events.APIGatewayProxyRequest, claims *models.JWTClaims) (events.APIGatewayProxyResponse, error) {
	id, action, ok := storyVideoRoute(req.Path)
	if !ok {
		return errorResponse(http.StatusNotFound, "NOT_FOUND", "Route not found"), nil
	}
	// The user id goes into storage keys and database keys. Every real id passes
	// (a Google id is numeric, a password signup's is a UUID); this stops a
	// malformed one before it can build a key that reaches another user's files.
	if !storyjob.ValidUserID(claims.UserID) {
		log.Printf("story_video_bad_user_id")
		return errorResponse(http.StatusBadRequest, "INVALID_INPUT", "Your account cannot use this yet"), nil
	}
	if id != "" {
		if !storyjob.ValidJobID(id) {
			return errorResponse(http.StatusNotFound, "NOT_FOUND", "Video not found"), nil
		}
		id = strings.ToLower(id) // ids are stored in lower case
	}
	// Every case that works on one video requires an id. Delete builds storage
	// prefixes from it, so an empty id must never get that far.
	switch {
	case id == "" && req.HTTPMethod == "POST":
		return h.handleCreateStoryVideo(ctx, req, claims)
	case id == "" && req.HTTPMethod == "GET":
		return h.handleListStoryVideos(ctx, req, claims)
	case id != "" && action == "" && req.HTTPMethod == "GET":
		return h.handleGetStoryVideo(ctx, claims, id)
	case id != "" && action == "" && req.HTTPMethod == "DELETE":
		return h.handleDeleteStoryVideo(ctx, claims, id)
	case id != "" && action == "cancel" && req.HTTPMethod == "POST":
		return h.handleCancelStoryVideo(ctx, claims, id)
	case id != "" && action == "download" && req.HTTPMethod == "GET":
		return h.handleDownloadStoryVideo(ctx, claims, id)
	}
	return errorResponse(http.StatusNotFound, "NOT_FOUND", "Route not found"), nil
}

// storyVideoView turns a job into what the API returns.
func storyVideoView(j storyjob.Job) StoryVideoResponse {
	v := StoryVideoResponse{
		JobID: j.JobID, PlanID: j.PlanID, PlanRevision: j.PlanRevision,
		Status: j.Status, Progress: j.Progress, Language: j.Language, Voice: j.Voice, Caption: j.Caption,
		TotalSeconds: j.TotalSeconds,
		CreatedAt:    j.CreatedAt, UpdatedAt: j.UpdatedAt, CompletedAt: j.CompletedAt,
	}
	// A job that asked to be retried keeps the reason on its record in case the
	// retries run out (see storyjob.Runner). That is not the user's business
	// while the job is still going, and it is cleared when the next attempt
	// starts, so the reason is shown only once the job has failed.
	if j.Status == storyjob.StatusFailed {
		v.Error, v.ErrorCode, v.FailedStage = j.Error, j.ErrorCode, j.FailedStage
	}
	return v
}

// getStoryVideo reads one of the user's jobs with a consistent read, so a page
// that polls never sees progress go backwards. (nil, nil) means there is no such
// job for this user. The read is keyed by the caller's own partition, so another
// user's job is simply not found.
func (h *APIHandler) getStoryVideo(ctx context.Context, userID, jobID string) (*storyjob.Job, error) {
	out, err := h.DB.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(h.TableName), Key: storyjob.Key(userID, jobID), ConsistentRead: aws.Bool(true),
	})
	if err != nil {
		return nil, err
	}
	if len(out.Item) == 0 {
		return nil, nil
	}
	j, err := storyjob.FromItem(out.Item)
	if err != nil {
		return nil, fmt.Errorf("reading the record: %w", err)
	}
	return &j, nil
}

// loadStoryVideo is getStoryVideo with the response for the two failure cases.
func (h *APIHandler) loadStoryVideo(ctx context.Context, userID, jobID string) (*storyjob.Job, *events.APIGatewayProxyResponse) {
	j, err := h.getStoryVideo(ctx, userID, jobID)
	if err != nil {
		log.Printf("story_video_read_failed user=%s job=%s err=%v", userID, jobID, err)
		r := errorResponse(http.StatusInternalServerError, "DB_ERROR", "Could not read the video. Please try again.")
		return nil, &r
	}
	if j == nil {
		r := errorResponse(http.StatusNotFound, "NOT_FOUND", "Video not found")
		return nil, &r
	}
	return j, nil
}

func (h *APIHandler) handleGetStoryVideo(ctx context.Context, claims *models.JWTClaims, id string) (events.APIGatewayProxyResponse, error) {
	j, resp := h.loadStoryVideo(ctx, claims.UserID, id)
	if resp != nil {
		return *resp, nil
	}
	return jsonResponse(http.StatusOK, storyVideoView(*j)), nil
}

// ---------------------------------------------------------------------------
// Create

// storyVideoBrief is the little a limit check needs to know about a job.
type storyVideoBrief struct {
	JobID, PlanID, Status, CreatedAt, UpdatedAt string
}

// active reports whether the job is believed to be in the making: it has not
// ended, and it was touched recently enough that its execution can still be
// alive. A record whose times cannot be read does not count, so a damaged record
// cannot hold its owner at the limit for ever.
func (b storyVideoBrief) active(now time.Time) bool {
	if storyjob.Terminal(b.Status) {
		return false
	}
	t, err := time.Parse(time.RFC3339, b.UpdatedAt)
	if err != nil {
		if t, err = time.Parse(time.RFC3339, b.CreatedAt); err != nil {
			return false
		}
	}
	return now.Sub(t) < storyVideoStaleAfter
}

// storyVideoSortPrefix is the start of every job's sort key.
func storyVideoSortPrefix() string {
	lo, _ := storyjob.SortKeyRange()
	return lo
}

// recentStoryVideos reads the user's jobs from the last 24 hours, oldest first,
// at most limit of them. The window is a key range on the time-ordered job ids,
// so no scan or index is involved.
func (h *APIHandler) recentStoryVideos(ctx context.Context, userID string, now time.Time, limit int) ([]storyVideoBrief, error) {
	// Bounded here as well as in storyVideosPerDay, next to the conversion, so
	// the int32 below is visibly safe.
	if limit < 1 {
		limit = 1
	} else if limit > storyVideoMaxPerDay {
		limit = storyVideoMaxPerDay
	}
	prefix := storyVideoSortPrefix()
	out, err := h.DB.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(h.TableName),
		KeyConditionExpression: aws.String("PK = :pk AND SK BETWEEN :lo AND :hi"),
		ExpressionAttributeNames: map[string]string{
			"#s": "status",
		},
		ExpressionAttributeValues: map[string]dbtypes.AttributeValue{
			":pk": &dbtypes.AttributeValueMemberS{Value: "USER#" + userID},
			":lo": &dbtypes.AttributeValueMemberS{Value: prefix + uuidV7Floor(now.Add(-storyVideoWindow))},
			// '~' sorts after every character a UUID can contain.
			":hi": &dbtypes.AttributeValueMemberS{Value: prefix + "~"},
		},
		ProjectionExpression: aws.String("SK, planId, #s, createdAt, updatedAt"),
		ScanIndexForward:     aws.Bool(true),
		ConsistentRead:       aws.Bool(true),
		Limit:                aws.Int32(int32(limit)),
	})
	if err != nil {
		return nil, err
	}
	str := func(item map[string]dbtypes.AttributeValue, k string) string {
		if v, ok := item[k].(*dbtypes.AttributeValueMemberS); ok {
			return v.Value
		}
		return ""
	}
	briefs := make([]storyVideoBrief, 0, len(out.Items))
	for _, it := range out.Items {
		briefs = append(briefs, storyVideoBrief{
			JobID:  strings.TrimPrefix(str(it, "SK"), prefix),
			PlanID: str(it, "planId"), Status: str(it, "status"),
			CreatedAt: str(it, "createdAt"), UpdatedAt: str(it, "updatedAt"),
		})
	}
	return briefs, nil
}

// storyVideoConflict is a 409 that carries the job the caller should look at.
func storyVideoConflict(code, message, jobID string) events.APIGatewayProxyResponse {
	body, _ := json.Marshal(models.APIError{Code: code, Message: message, Details: map[string]string{"jobId": jobID}})
	return events.APIGatewayProxyResponse{StatusCode: http.StatusConflict, Headers: corsHeaders(), Body: string(body)}
}

// getStoryPlanRecord reads one of the user's plans with a consistent read.
// (nil, nil) means there is no such plan for this user.
func (h *APIHandler) getStoryPlanRecord(ctx context.Context, userID, planID string) (*storyPlanRecord, error) {
	out, err := h.DB.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(h.TableName), ConsistentRead: aws.Bool(true),
		Key: map[string]dbtypes.AttributeValue{
			"PK": &dbtypes.AttributeValueMemberS{Value: "USER#" + userID},
			"SK": &dbtypes.AttributeValueMemberS{Value: storyPlanPrefix + planID},
		},
	})
	if err != nil {
		return nil, err
	}
	if len(out.Item) == 0 {
		return nil, nil
	}
	rec := itemToStoryPlan(out.Item)
	rec.UserID = userID
	return &rec, nil
}

// handleCreateStoryVideo makes a job from a plan and starts the pipeline.
//
// The order matters. Everything the user can fix (the plan, its revision, the
// pictures, the voice) is checked first, since it costs nothing and consumes no
// allowance. The limits come next, then the job is written, and only then is
// the pipeline started. If starting fails, the job is marked failed rather than
// left queued for ever.
func (h *APIHandler) handleCreateStoryVideo(ctx context.Context, req events.APIGatewayProxyRequest, claims *models.JWTClaims) (events.APIGatewayProxyResponse, error) {
	if !h.storyVideoEnabled() {
		return errorResponse(http.StatusServiceUnavailable, "UNAVAILABLE", "Making videos is not available yet"), nil
	}
	if len(req.Body) > storyVideoMaxBodyBytes {
		return errorResponse(http.StatusBadRequest, "VALIDATION_ERROR", "The request is too large"), nil
	}
	var body CreateStoryVideoRequest
	if err := json.Unmarshal([]byte(req.Body), &body); err != nil {
		return errorResponse(http.StatusBadRequest, "INVALID_BODY", "Invalid request body"), nil
	}
	planID := strings.ToLower(strings.TrimSpace(body.PlanID))
	if !validStoryPlanID(planID) {
		return errorResponse(http.StatusNotFound, "NOT_FOUND", "Plan not found"), nil
	}

	rec, err := h.getStoryPlanRecord(ctx, claims.UserID, planID)
	if err != nil {
		log.Printf("story_video_plan_read_failed user=%s plan=%s err=%v", claims.UserID, planID, err)
		return errorResponse(http.StatusInternalServerError, "DB_ERROR", "Could not read the plan. Please try again."), nil
	}
	if rec == nil {
		return errorResponse(http.StatusNotFound, "NOT_FOUND", "Plan not found"), nil
	}
	if rec.Status != storyPlanReady || rec.Plan == nil {
		return errorResponse(http.StatusConflict, "NOT_READY", "This plan has nothing to make a video from"), nil
	}
	// The user confirms the plan they are looking at. If it changed since (another
	// tab), render nothing: the video would not be the one they approved.
	if body.Revision != rec.Revision {
		return errorResponse(http.StatusConflict, "STALE_REVISION", "This plan was changed somewhere else. Reload it and try again."), nil
	}

	now := h.storyVideoNow().UTC()
	jobID, err := newStoryPlanID(now) // a UUIDv7; see the file comment
	if err != nil {
		return errorResponse(http.StatusInternalServerError, "INTERNAL_ERROR", "Could not start the video. Please try again."), nil
	}
	job, err := storyjob.NewFromPlan(claims.UserID, jobID, rec.PlanID, rec.Revision, *rec.Plan, body.Voice, body.Caption, now)
	if err != nil {
		return storyVideoPlanProblem(err), nil
	}

	// Limits. A read that fails closes the check: no job is made.
	perDay := h.storyVideosPerDay()
	recent, err := h.recentStoryVideos(ctx, claims.UserID, now, perDay)
	if err != nil {
		log.Printf("story_video_limit_check_failed user=%s err=%v", claims.UserID, err)
		return errorResponse(http.StatusInternalServerError, "DB_ERROR", "Could not start the video. Please try again."), nil
	}
	active := 0
	for i := len(recent) - 1; i >= 0; i-- { // newest first, so the one reported is the latest
		if !recent[i].active(now) {
			continue
		}
		if recent[i].PlanID == rec.PlanID {
			return storyVideoConflict("ALREADY_RUNNING", "A video is already being made from this plan.", recent[i].JobID), nil
		}
		active++
	}
	if limit := h.storyVideoMaxActive(); active >= limit {
		return errorResponse(http.StatusConflict, "TOO_MANY_ACTIVE",
			fmt.Sprintf("You already have %d videos being made. Wait for one to finish, or cancel one.", limit)), nil
	}
	if len(recent) >= perDay {
		return tooManyRequests("RATE_LIMITED",
			fmt.Sprintf("You can make %d videos a day. Try again later.", perDay),
			maxInt(1, int(storyVideoRetryAfter(recent[0], now).Seconds()))), nil
	}

	item, err := storyjob.ToItem(job)
	if err != nil {
		log.Printf("story_video_encode_failed user=%s job=%s err=%v", claims.UserID, jobID, err)
		return errorResponse(http.StatusInternalServerError, "INTERNAL_ERROR", "Could not start the video. Please try again."), nil
	}
	if _, err := h.DB.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(h.TableName), Item: item,
		ConditionExpression: aws.String("attribute_not_exists(PK)"),
	}); err != nil {
		log.Printf("story_video_put_failed user=%s job=%s err=%v", claims.UserID, jobID, err)
		return errorResponse(http.StatusInternalServerError, "DB_ERROR", "Could not start the video. Please try again."), nil
	}

	// The execution is named after the job, so a repeated start is refused by
	// Step Functions instead of making a second render, and the execution to stop
	// on a cancel is known without storing it.
	input, _ := json.Marshal(map[string]string{"userId": job.UserID, "jobId": job.JobID})
	_, err = h.SFN.StartExecution(ctx, &sfn.StartExecutionInput{
		StateMachineArn: aws.String(h.StoryVideoStateMachineARN),
		Name:            aws.String(job.JobID),
		Input:           aws.String(string(input)),
	})
	if err != nil && !isExecutionAlreadyExists(err) {
		log.Printf("story_video_start_failed user=%s job=%s err=%v", claims.UserID, jobID, err)
		// A start that failed may still have started (a timeout). Marking the job
		// failed also stops that: the worker finds the record ended and stops.
		endCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		if _, eerr := h.endStoryVideo(endCtx, claims.UserID, jobID, storyjob.CodeStartFailed, storyVideoStartFailedMessage, h.storyVideoNow()); eerr != nil {
			log.Printf("story_video_start_mark_failed user=%s job=%s err=%v", claims.UserID, jobID, eerr)
		}
		return errorResponse(http.StatusInternalServerError, "PIPELINE_ERROR", "Could not start the video. Please try again."), nil
	}
	return jsonResponse(http.StatusAccepted, storyVideoView(job)), nil
}

// storyVideoRetryAfter is how long until the oldest video of the window leaves it.
func storyVideoRetryAfter(oldest storyVideoBrief, now time.Time) time.Duration {
	if t, err := time.Parse(time.RFC3339, oldest.CreatedAt); err == nil {
		if wait := t.Add(storyVideoWindow).Sub(now); wait > 0 {
			return wait
		}
	}
	return time.Minute
}

// storyVideoPlanProblem maps a refusal of storyjob.NewFromPlan to a response. The
// messages come from our own code and say what to change.
func storyVideoPlanProblem(err error) events.APIGatewayProxyResponse {
	var missing *storyjob.MissingPicturesError
	var lang *storyjob.UnsupportedLanguageError
	switch {
	case errors.As(err, &missing):
		return errorResponse(http.StatusBadRequest, "MISSING_PICTURES", missing.UserMessage())
	case errors.As(err, &lang):
		return errorResponse(http.StatusBadRequest, "UNSUPPORTED_LANGUAGE", lang.UserMessage())
	}
	return errorResponse(http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
}

// ---------------------------------------------------------------------------
// End (cancel, or a start that failed)

// endStoryVideo marks a job that has not ended as failed with a code and a
// message for the user. It reports whether it did. false with no error means the
// job had already ended (completed, failed, or cancelled) or no longer exists,
// and nothing was changed.
//
// The condition is storyjob.NotEndedCondition, the one the worker's own store
// and the state machine's MarkFailed use, which is the whole safety argument: a job that finished is
// never overwritten, a worker that is slow never undoes this, and a deleted job
// is never recreated.
func (h *APIHandler) endStoryVideo(ctx context.Context, userID, jobID, code, message string, now time.Time) (bool, error) {
	str := func(s string) dbtypes.AttributeValue { return &dbtypes.AttributeValueMemberS{Value: s} }
	_, err := h.DB.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(h.TableName), Key: storyjob.Key(userID, jobID),
		UpdateExpression:    aws.String("SET #status = :failed, #error = :message, #errorCode = :code, #failedStage = #status, #updatedAt = :now"),
		ConditionExpression: aws.String(storyjob.NotEndedCondition),
		ExpressionAttributeNames: map[string]string{
			"#status": "status", "#error": "error", "#errorCode": "errorCode", "#failedStage": "failedStage", "#updatedAt": "updatedAt",
		},
		ExpressionAttributeValues: map[string]dbtypes.AttributeValue{
			":failed": str(storyjob.StatusFailed), ":completed": str(storyjob.StatusCompleted),
			":message": str(message), ":code": str(code), ":now": str(now.UTC().Format(time.RFC3339)),
		},
	})
	var refused *dbtypes.ConditionalCheckFailedException
	switch {
	case errors.As(err, &refused):
		return false, nil
	case err != nil:
		return false, err
	}
	return true, nil
}

// handleCancelStoryVideo stops a video that is being made.
//
// The record is changed first and the execution stopped second. The record is
// what the worker and the page both look at: the worker notices within seconds
// and stops FFmpeg, and the notifier sends nothing for a cancelled job. Stopping
// the execution is a best-effort extra that also stops the task, so a failure to
// stop it is logged and not reported.
//
// Cancelling a job that has already ended is not an error: the response is its
// current state, and nothing is changed. That includes the race where the worker
// finishes between the read and the write.
func (h *APIHandler) handleCancelStoryVideo(ctx context.Context, claims *models.JWTClaims, id string) (events.APIGatewayProxyResponse, error) {
	j, resp := h.loadStoryVideo(ctx, claims.UserID, id)
	if resp != nil {
		return *resp, nil
	}
	if storyjob.Terminal(j.Status) {
		return jsonResponse(http.StatusOK, storyVideoView(*j)), nil
	}

	now := h.storyVideoNow().UTC()
	changed, err := h.endStoryVideo(ctx, claims.UserID, id, storyjob.CodeCancelled, storyVideoCancelledMessage, now)
	if err != nil {
		log.Printf("story_video_cancel_failed user=%s job=%s err=%v", claims.UserID, id, err)
		return errorResponse(http.StatusInternalServerError, "DB_ERROR", "Could not cancel the video. Please try again."), nil
	}
	if !changed {
		// It ended, or was deleted, between the read and the write. Say what it is now.
		j, resp = h.loadStoryVideo(ctx, claims.UserID, id)
		if resp != nil {
			return *resp, nil
		}
		return jsonResponse(http.StatusOK, storyVideoView(*j)), nil
	}

	h.stopStoryVideoExecution(ctx, claims.UserID, id)

	j.FailedStage, j.Status = j.Status, storyjob.StatusFailed
	j.Error, j.ErrorCode, j.UpdatedAt = storyVideoCancelledMessage, storyjob.CodeCancelled, now.Format(time.RFC3339)
	return jsonResponse(http.StatusOK, storyVideoView(*j)), nil
}

// stopStoryVideoExecution asks Step Functions to stop the job's execution, which
// also stops its task. It never fails the caller.
func (h *APIHandler) stopStoryVideoExecution(ctx context.Context, userID, jobID string) {
	if h.StoryVideoStateMachineARN == "" || h.SFN == nil {
		return
	}
	arn, err := stateMachineExecutionARN(h.StoryVideoStateMachineARN, jobID)
	if err != nil {
		return
	}
	stopCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if _, err := h.SFN.StopExecution(stopCtx, &sfn.StopExecutionInput{ExecutionArn: aws.String(arn)}); err != nil {
		log.Printf("story_video_stop_failed user=%s job=%s err=%v", userID, jobID, err)
	}
}

// ---------------------------------------------------------------------------
// Download

func (h *APIHandler) handleDownloadStoryVideo(ctx context.Context, claims *models.JWTClaims, id string) (events.APIGatewayProxyResponse, error) {
	j, resp := h.loadStoryVideo(ctx, claims.UserID, id)
	if resp != nil {
		return *resp, nil
	}
	if j.Status != storyjob.StatusCompleted || j.OutputKey == "" {
		return errorResponse(http.StatusConflict, "NOT_READY", "This video is not finished yet"), nil
	}
	// The key is on the record, but the only key this job can have is its own.
	// Anything else means the record is wrong, and no link is made for it.
	key := storyjob.OutputKey(claims.UserID, id)
	if j.OutputKey != key {
		log.Printf("story_video_output_key_mismatch user=%s job=%s", claims.UserID, id)
		return errorResponse(http.StatusInternalServerError, "INTERNAL_ERROR", "Could not prepare the download"), nil
	}
	// A failed status check does not block a healthy download (as for the other
	// videos); a file that is certainly gone says so instead of handing out a
	// link that returns an error page.
	if _, err := h.S3.ObjectStatus(ctx, h.Bucket, key); err != nil && storage.IsNotFound(err) {
		return errorResponse(http.StatusNotFound, "NOT_FOUND", "This video's file is no longer available. Make the video again."), nil
	}
	url, err := h.S3.GenerateAttachmentURL(ctx, h.Bucket, key, storyVideoFileName, storyVideoDownloadExpiry)
	if err != nil {
		log.Printf("story_video_presign_failed user=%s job=%s err=%v", claims.UserID, id, err)
		return errorResponse(http.StatusInternalServerError, "S3_ERROR", "Could not prepare the download. Please try again."), nil
	}
	return jsonResponse(http.StatusOK, map[string]any{
		"url": url, "filename": storyVideoFileName, "expiresInSeconds": int(storyVideoDownloadExpiry.Seconds()),
	}), nil
}

// ---------------------------------------------------------------------------
// Delete

// handleDeleteStoryVideo removes a video that has ended, with its files.
//
// A video that is still being made must be cancelled first. Cancel changes the
// record at once, and the worker stops within seconds; deleting while it runs
// could leave a file the worker was in the middle of uploading, in a place no
// record points to and no lifecycle rule expires.
//
// The files go first and the record last, so a failed cleanup leaves the record
// and the user can try again, instead of leaving a video nobody can find to
// remove. The bucket is versioned, so a plain delete would only hide the files:
// every version and marker under the job's two prefixes is removed.
func (h *APIHandler) handleDeleteStoryVideo(ctx context.Context, claims *models.JWTClaims, id string) (events.APIGatewayProxyResponse, error) {
	j, resp := h.loadStoryVideo(ctx, claims.UserID, id)
	if resp != nil {
		return *resp, nil
	}
	if !storyjob.Terminal(j.Status) {
		return errorResponse(http.StatusConflict, "STILL_RUNNING", "This video is still being made. Cancel it first, then delete it."), nil
	}
	for _, prefix := range storyjob.JobPrefixes(claims.UserID, id) {
		if err := h.S3.DeleteObjectVersions(ctx, h.Bucket, prefix); err != nil {
			log.Printf("story_video_delete_s3_failed user=%s job=%s prefix=%s err=%v", claims.UserID, id, prefix, err)
			return errorResponse(http.StatusInternalServerError, "S3_CLEANUP_FAILED", "Could not delete the video's files. Please try again."), nil
		}
	}
	if _, err := h.DB.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(h.TableName), Key: storyjob.Key(claims.UserID, id),
	}); err != nil {
		log.Printf("story_video_delete_db_failed user=%s job=%s err=%v", claims.UserID, id, err)
		return errorResponse(http.StatusInternalServerError, "DB_ERROR", "Could not delete the video. Please try again."), nil
	}
	log.Printf("story_video_deleted user=%s job=%s", claims.UserID, id)
	return events.APIGatewayProxyResponse{StatusCode: http.StatusNoContent, Headers: corsHeaders()}, nil
}

// ---------------------------------------------------------------------------
// List

// storyVideoListAttrs are the attributes a list reads. The scenes are left out:
// they are the bulk of a record and the list does not show them.
var storyVideoListAttrs = []string{
	"PK", "SK", "planId", "planRevision", "status", "progress", "language", "voice", "caption", "totalSeconds",
	"error", "errorCode", "failedStage", "createdAt", "updatedAt", "completedAt",
}

// handleListStoryVideos returns the user's videos, newest first. With ?planId=
// it returns only the ones made from that plan, so the editor can show the
// latest video of the plan it is open on.
//
// It reads the newest 100 records and returns at most 20. hasMore says there
// are records it did not return; for a plan older than the newest 100 videos,
// the filter cannot find it.
func (h *APIHandler) handleListStoryVideos(ctx context.Context, req events.APIGatewayProxyRequest, claims *models.JWTClaims) (events.APIGatewayProxyResponse, error) {
	planID := strings.ToLower(strings.TrimSpace(req.QueryStringParameters["planId"]))
	if planID != "" && !validStoryPlanID(planID) {
		return errorResponse(http.StatusBadRequest, "INVALID_INPUT", "That is not a valid plan id"), nil
	}
	names := map[string]string{}
	proj := make([]string, 0, len(storyVideoListAttrs))
	for i, a := range storyVideoListAttrs {
		ph := fmt.Sprintf("#a%d", i)
		names[ph] = a
		proj = append(proj, ph)
	}
	lo, hi := storyjob.SortKeyRange()
	out, err := h.DB.Query(ctx, &dynamodb.QueryInput{
		TableName:                aws.String(h.TableName),
		KeyConditionExpression:   aws.String("PK = :pk AND SK BETWEEN :lo AND :hi"),
		ExpressionAttributeNames: names,
		ExpressionAttributeValues: map[string]dbtypes.AttributeValue{
			":pk": &dbtypes.AttributeValueMemberS{Value: "USER#" + claims.UserID},
			":lo": &dbtypes.AttributeValueMemberS{Value: lo},
			":hi": &dbtypes.AttributeValueMemberS{Value: hi},
		},
		ProjectionExpression: aws.String(strings.Join(proj, ", ")),
		ScanIndexForward:     aws.Bool(false),
		Limit:                aws.Int32(storyVideoListRead),
	})
	if err != nil {
		log.Printf("story_video_list_failed user=%s err=%v", claims.UserID, err)
		return errorResponse(http.StatusInternalServerError, "DB_ERROR", "Could not list your videos. Please try again."), nil
	}
	videos := make([]StoryVideoResponse, 0, storyVideoListMax)
	hasMore := out.LastEvaluatedKey != nil
	for _, it := range out.Items {
		j, err := storyjob.FromItem(it)
		if err != nil {
			log.Printf("story_video_list_skip user=%s err=%v", claims.UserID, err)
			continue
		}
		if planID != "" && j.PlanID != planID {
			continue
		}
		if len(videos) == storyVideoListMax {
			hasMore = true
			break
		}
		videos = append(videos, storyVideoView(j))
	}
	return jsonResponse(http.StatusOK, map[string]any{"videos": videos, "hasMore": hasMore}), nil
}
