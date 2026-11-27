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
	"github.com/aws/aws-sdk-go-v2/service/sfn"
	"github.com/rahul/indifferent/backend/internal/models"
	"github.com/rahul/indifferent/backend/internal/shorts/cliprender"
	"github.com/rahul/indifferent/backend/internal/shorts/render"
	"github.com/rahul/indifferent/backend/internal/videocopy"
)

// The hook line for a clip, and rendering one finished clip again (#329).
//
//	GET  /shorts/{id}/clips/{clipId}/hook              the saved hook, the ranker's line, allowances
//	PUT  /shorts/{id}/clips/{clipId}/hook              { hook } or { reset: true }
//	POST /shorts/{id}/clips/{clipId}/hook-suggestions  { requestId }  three lines from the clip's words
//	                                                    (one use of the shared 20-a-day writing allowance)
//	POST /shorts/{id}/clips/{clipId}/render            { requestId }  render the clip again with its saved
//	                                                    hook, trim and captions (one use of the 15-a-day
//	                                                    Shorts retry allowance)
//
// Every read is keyed by the caller's own partition, so another person's job or
// clip is simply not found.

// HookWriter writes hook lines. *videocopy.Service implements it.
type HookWriter interface {
	SuggestHooks(ctx context.Context, r videocopy.Request, fits func(string) bool) (videocopy.HookResult, error)
}

// hookMaxBody bounds a hook request before it is parsed.
const hookMaxBody = 8 << 10

// clipAction is a /shorts/{id}/clips/{clipId}/{action} route.
type clipAction struct {
	jobID, clipID, action string
}

// parseClipAction recognises the hook and render routes. An id with a '#' is
// refused: it would build a key that points at another record.
func parseClipAction(path string) (clipAction, bool) {
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(parts) != 5 || parts[0] != "shorts" || parts[2] != "clips" || parts[1] == "" || parts[3] == "" {
		return clipAction{}, false
	}
	if strings.Contains(parts[1]+parts[3], "#") {
		return clipAction{}, false
	}
	switch parts[4] {
	case "hook", "hook-suggestions", "render":
		return clipAction{jobID: parts[1], clipID: parts[3], action: parts[4]}, true
	}
	return clipAction{}, false
}

func isClipActionPath(path string) bool {
	_, ok := parseClipAction(path)
	return ok
}

func (h *APIHandler) handleClipAction(ctx context.Context, req events.APIGatewayProxyRequest, claims *models.JWTClaims) (events.APIGatewayProxyResponse, error) {
	a, ok := parseClipAction(req.Path)
	if !ok {
		return errorResponse(http.StatusNotFound, "NOT_FOUND", "Route not found"), nil
	}
	switch {
	case a.action == "hook" && req.HTTPMethod == http.MethodGet:
		return h.hookGet(ctx, a, claims)
	case a.action == "hook" && req.HTTPMethod == http.MethodPut:
		return h.hookSave(ctx, req, a, claims)
	case a.action == "hook-suggestions" && req.HTTPMethod == http.MethodPost:
		return h.hookSuggest(ctx, req, a, claims)
	case a.action == "render" && req.HTTPMethod == http.MethodPost:
		return h.clipRender(ctx, req, a, claims)
	}
	return errorResponse(http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed"), nil
}

// renderedClip returns the job's rendered clip with this id.
func renderedClip(job *models.ShortsJob, clipID string) *models.Clip {
	for i := range job.Clips {
		if job.Clips[i].ClipID == clipID {
			return &job.Clips[i]
		}
	}
	return nil
}

// rankerHook is the line the ranking model wrote for the clip, when it can be
// used as a hook as it is. It costs nothing to offer.
func rankerHook(job *models.ShortsJob, clipID string) string {
	text := ""
	if c := renderedClip(job, clipID); c != nil {
		text = c.HookText
	}
	if text == "" {
		for _, s := range job.Segments {
			if s.ClipID == clipID {
				text = s.HookText
			}
		}
	}
	text = strings.TrimSpace(text)
	if text == "" || (models.ClipHook{Enabled: true, Text: text, Style: models.HookStyleBox, Position: models.HookTop, Seconds: models.DefaultHookSeconds}).Validate() != nil {
		return ""
	}
	return text
}

// HookView is the body of GET .../hook.
type HookView struct {
	Hook models.ClipHook `json:"hook"`
	// Saved is false when no hook was ever saved and Hook holds the defaults.
	Saved bool `json:"saved"`
	// Ranker is the ranking model's line for the clip, or "".
	Ranker string `json:"ranker,omitempty"`
	// SuggestionsEnabled is false when the writing model is switched off.
	SuggestionsEnabled bool `json:"suggestionsEnabled"`
	// RenderEnabled is false when rendering one clip again is switched off.
	RenderEnabled     bool                   `json:"renderEnabled"`
	Render            *models.ClipRenderView `json:"render,omitempty"`
	SuggestRemaining  *int64                 `json:"suggestRemainingToday,omitempty"`
	SuggestDailyLimit int64                  `json:"suggestDailyLimit"`
	RenderRemaining   *int64                 `json:"renderRemainingToday,omitempty"`
	RenderDailyLimit  int64                  `json:"renderDailyLimit"`
	MaxChars          int                    `json:"maxChars"`
}

func (h *APIHandler) hookGet(ctx context.Context, a clipAction, claims *models.JWTClaims) (events.APIGatewayProxyResponse, error) {
	job, err := h.getShortsJobByID(ctx, claims.UserID, a.jobID)
	if err != nil || !clipKnown(job, a.clipID) {
		return errorResponse(http.StatusNotFound, "NOT_FOUND", "Clip not found"), nil
	}
	view := HookView{
		Hook:               models.ClipHook{}.Normalized(),
		Ranker:             rankerHook(job, a.clipID),
		SuggestionsEnabled: h.HookWriter != nil,
		RenderEnabled:      h.ClipRenderStateMachineARN != "",
		SuggestDailyLimit:  quotaCopyHelper.limit,
		RenderDailyLimit:   quotaShortsRetry.limit,
		MaxChars:           models.MaxHookChars,
	}
	if saved, ok := job.ClipHooks[a.clipID]; ok {
		view.Hook, view.Saved = saved.Normalized(), true
	}
	rec, err := cliprender.Get(ctx, h.DB, h.TableName, claims.UserID, a.jobID, a.clipID)
	if err != nil {
		log.Printf("clip_render_read_failed user=%s job=%s err=%v", claims.UserID, a.jobID, err)
		return errorResponse(http.StatusInternalServerError, "DB_ERROR", "Could not read the clip. Please try again."), nil
	}
	if rec != nil {
		v := rec.View(time.Now())
		view.Render = &v
	}
	if view.SuggestionsEnabled {
		view.SuggestRemaining = remainingOf(quotaCopyHelper, h.quotaUsedToday(ctx, claims.UserID, quotaCopyHelper))
	}
	if view.RenderEnabled {
		view.RenderRemaining = remainingOf(quotaShortsRetry, h.quotaUsedToday(ctx, claims.UserID, quotaShortsRetry))
	}
	return jsonResponse(http.StatusOK, view), nil
}

// HookSaveRequest is the body of PUT .../hook.
type HookSaveRequest struct {
	Hook  *models.ClipHook `json:"hook"`
	Reset bool             `json:"reset,omitempty"`
}

// hookSave stores a validated hook for the clip, or removes it. It applies to the
// next render: a single-clip render, or a retry of a failed job. Like clip
// edits, it is refused while the pipeline is working on the job.
func (h *APIHandler) hookSave(ctx context.Context, req events.APIGatewayProxyRequest, a clipAction, claims *models.JWTClaims) (events.APIGatewayProxyResponse, error) {
	if len(req.Body) > hookMaxBody {
		return errorResponse(http.StatusBadRequest, "VALIDATION_ERROR", "The request is too large."), nil
	}
	var body HookSaveRequest
	dec := json.NewDecoder(strings.NewReader(req.Body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil || (body.Hook == nil && !body.Reset) {
		return errorResponse(http.StatusBadRequest, "INVALID_BODY", "Invalid request body"), nil
	}
	job, err := h.getShortsJobByID(ctx, claims.UserID, a.jobID)
	if err != nil {
		return errorResponse(http.StatusNotFound, "NOT_FOUND", "Shorts job not found"), nil
	}
	if !clipKnown(job, a.clipID) {
		return errorResponse(http.StatusNotFound, "NOT_FOUND", "Clip not found"), nil
	}
	switch job.Status {
	case "completed", "failed", "uploaded":
	default:
		return errorResponse(http.StatusBadRequest, "INVALID_STATE", "The hook can be changed before rendering starts and after it finishes, not while clips are being made."), nil
	}
	readUpdatedAt := job.UpdatedAt
	now := time.Now().UTC().Format(time.RFC3339)
	var saved *models.ClipHook
	if body.Reset {
		delete(job.ClipHooks, a.clipID)
	} else {
		hk := body.Hook.Normalized()
		hk.UpdatedAt = ""
		if err := hk.Validate(); err != nil {
			return errorResponse(http.StatusBadRequest, "VALIDATION_ERROR", err.Error()), nil
		}
		// The same measuring the renderer does, so a hook that is saved is a hook
		// that is drawn.
		if err := render.CheckHook(hk, job.Caption, job.Effects); err != nil {
			return errorResponse(http.StatusBadRequest, "VALIDATION_ERROR", hookCheckMessage(err)), nil
		}
		hk.UpdatedAt = now
		if job.ClipHooks == nil {
			job.ClipHooks = map[string]models.ClipHook{}
		}
		job.ClipHooks[a.clipID] = hk
		saved = &hk
	}
	job.UpdatedAt = now
	if err := h.putShortsJobIfUnchanged(ctx, *job, readUpdatedAt); err != nil {
		if errors.Is(err, errShortsJobChanged) {
			return shortsJobChanged(), nil
		}
		return errorResponse(http.StatusInternalServerError, "DB_ERROR", "Failed to save the hook."), nil
	}
	if saved == nil {
		return jsonResponse(http.StatusOK, map[string]any{"reset": true, "hook": models.ClipHook{}.Normalized()}), nil
	}
	return jsonResponse(http.StatusOK, map[string]any{"hook": saved}), nil
}

func hookCheckMessage(err error) string {
	switch {
	case errors.Is(err, render.ErrHookTooLong):
		return "This hook is too long to fit on two lines. Shorten it."
	case errors.Is(err, render.ErrHookNoRoom):
		return "There is no room for the hook beside these captions. Make the captions smaller or move them, or put the hook in the other position."
	}
	return "This hook cannot be drawn on the clip."
}

// --- suggestions -----------------------------------------------------------

// HookSuggestRequest is the body of POST .../hook-suggestions.
type HookSuggestRequest struct {
	RequestID string `json:"requestId"`
}

// HookSuggestResponse is the body of a successful POST .../hook-suggestions.
type HookSuggestResponse struct {
	Suggestions []string `json:"suggestions"`
	// Ranker is the ranking model's line, offered beside the suggestions.
	Ranker string `json:"ranker,omitempty"`
	// Dropped is how many lines the checks left out.
	Dropped        int    `json:"dropped"`
	RemainingToday *int64 `json:"remainingToday,omitempty"`
	DailyLimit     int64  `json:"dailyLimit"`
	Replayed       bool   `json:"replayed"`
}

func hookTarget(a clipAction) string { return "HOOKS#" + a.jobID + "#" + a.clipID }

func (h *APIHandler) hookSuggest(ctx context.Context, req events.APIGatewayProxyRequest, a clipAction, claims *models.JWTClaims) (events.APIGatewayProxyResponse, error) {
	if h.HookWriter == nil {
		return errorResponse(http.StatusServiceUnavailable, "UNAVAILABLE", "Hook suggestions are not available right now."), nil
	}
	if len(req.Body) > hookMaxBody {
		return errorResponse(http.StatusBadRequest, "VALIDATION_ERROR", "The request is too large."), nil
	}
	var body HookSuggestRequest
	if err := json.Unmarshal([]byte(req.Body), &body); err != nil {
		return errorResponse(http.StatusBadRequest, "INVALID_BODY", "Invalid request body"), nil
	}
	if !requestIDPattern.MatchString(body.RequestID) {
		return errorResponse(http.StatusBadRequest, "VALIDATION_ERROR", "requestId must be 16 to 64 letters, digits, hyphens or underscores."), nil
	}
	job, err := h.getShortsJobByID(ctx, claims.UserID, a.jobID)
	if err != nil || !clipKnown(job, a.clipID) {
		return errorResponse(http.StatusNotFound, "NOT_FOUND", "Clip not found"), nil
	}
	// The clip's own words only: the transcript inside its window, or the words
	// the person corrected.
	src, resp := h.postCopySource(ctx, claims.UserID, postCopyTarget{kind: videocopy.KindClip, jobID: a.jobID, clipID: a.clipID})
	if resp != nil {
		return *resp, nil
	}
	if err := videocopy.Check(src); err != nil {
		return errorResponse(http.StatusBadRequest, "VALIDATION_ERROR", "There is too little spoken text in this clip to write from."), nil
	}

	now := quotaNow().UTC()
	target := hookTarget(a)
	if h.QuotaTable != "" {
		claimed, err := h.claimIdem(ctx, claims.UserID, body.RequestID, target, now)
		if err != nil {
			log.Printf("hooks_idem_claim_failed user=%s err=%v", claims.UserID, err)
			return errorResponse(http.StatusInternalServerError, "DB_ERROR", "Could not start. Please try again."), nil
		}
		if !claimed {
			return h.hookReplay(ctx, claims.UserID, body.RequestID, target), nil
		}
	}
	used, retryAfter, ok := h.spendQuotaCount(ctx, claims.UserID, quotaCopyHelper)
	if !ok {
		h.releaseIdem(ctx, claims.UserID, body.RequestID)
		return quotaExceeded(quotaCopyHelper, retryAfter), nil
	}

	// A line is offered only if it can be drawn the way the person has the hook
	// set up now (or with the defaults).
	base := models.ClipHook{}.Normalized()
	if saved, ok := job.ClipHooks[a.clipID]; ok {
		base = saved.Normalized()
	}
	fits := func(line string) bool {
		hk := base
		hk.Enabled, hk.Text = true, line
		return render.CheckHook(hk, job.Caption, job.Effects) == nil
	}
	started := time.Now()
	res, genErr := h.HookWriter.SuggestHooks(ctx, src, fits)
	if genErr != nil {
		if errors.Is(genErr, context.Canceled) {
			h.releaseIdem(ctx, claims.UserID, body.RequestID)
		} else {
			h.refundQuota(ctx, claims.UserID, quotaCopyHelper)
			h.releaseIdem(ctx, claims.UserID, body.RequestID)
		}
		log.Printf("hooks_failed user=%s job=%s ms=%d dropped=%d err=%v", claims.UserID, a.jobID, time.Since(started).Milliseconds(), res.RejectedTotal(), genErr)
		return copyErrorResponse(genErr), nil
	}
	ranker := rankerHook(job, a.clipID)
	var lines []string
	for _, l := range res.Hooks {
		if !strings.EqualFold(l, ranker) {
			lines = append(lines, l)
		}
	}
	log.Printf("hooks_ok user=%s job=%s ms=%d hooks=%d dropped=%d model=%s", claims.UserID, a.jobID, time.Since(started).Milliseconds(), len(lines), res.RejectedTotal(), res.Model)
	out := HookSuggestResponse{Suggestions: lines, Ranker: ranker, Dropped: res.RejectedTotal(), RemainingToday: remainingOf(quotaCopyHelper, used), DailyLimit: quotaCopyHelper.limit}
	if out.Suggestions == nil {
		out.Suggestions = []string{}
	}
	if h.QuotaTable != "" {
		stored := out
		stored.RemainingToday = nil
		if b, err := json.Marshal(stored); err == nil {
			h.finishIdem(ctx, claims.UserID, body.RequestID, string(b), now)
		}
	}
	return jsonResponse(http.StatusOK, out), nil
}

// hookReplay answers a try whose id has been seen, without spending a use.
func (h *APIHandler) hookReplay(ctx context.Context, userID, requestID, target string) events.APIGatewayProxyResponse {
	rec, err := h.readIdem(ctx, userID, requestID)
	if err != nil || !rec.found {
		return errorResponse(http.StatusConflict, "IN_PROGRESS", "That request is being handled. Wait a moment, then try again.")
	}
	if rec.target != target {
		return errorResponse(http.StatusConflict, "REQUEST_ID_REUSED", "That request id was used for something else. Start a new request.")
	}
	if rec.status != "done" {
		return errorResponse(http.StatusConflict, "IN_PROGRESS", "Still writing. Wait a moment, then try again with the same request.")
	}
	var out HookSuggestResponse
	if json.Unmarshal([]byte(rec.response), &out) != nil {
		return errorResponse(http.StatusConflict, "IN_PROGRESS", "That request is being handled. Wait a moment, then try again.")
	}
	out.Replayed = true
	out.DailyLimit = quotaCopyHelper.limit
	out.RemainingToday = remainingOf(quotaCopyHelper, h.quotaUsedToday(ctx, userID, quotaCopyHelper))
	return jsonResponse(http.StatusOK, out)
}

// --- rendering one clip again ----------------------------------------------

// ClipRenderRequest is the body of POST .../render.
type ClipRenderRequest struct {
	RequestID string `json:"requestId"`
}

// ClipRenderResponse is the body of an accepted render.
type ClipRenderResponse struct {
	Render         models.ClipRenderView `json:"render"`
	Replayed       bool                  `json:"replayed"`
	RemainingToday *int64                `json:"remainingToday,omitempty"`
	DailyLimit     int64                 `json:"dailyLimit"`
}

// clipRenderWindow is the clip's place in the source, from the clip record or
// its ranked segment. The worker applies a saved trim on top.
func clipRenderWindow(job *models.ShortsJob, clip *models.Clip) (float64, float64, int, bool) {
	start, end, rank := clip.Start, clip.End, clip.Rank
	if end <= start {
		for _, s := range job.Segments {
			if s.ClipID == clip.ClipID {
				start, end, rank = s.Start, s.End, s.Rank
			}
		}
	}
	return start, end, rank, end > start && start >= 0
}

func (h *APIHandler) clipRender(ctx context.Context, req events.APIGatewayProxyRequest, a clipAction, claims *models.JWTClaims) (events.APIGatewayProxyResponse, error) {
	if h.ClipRenderStateMachineARN == "" {
		return errorResponse(http.StatusServiceUnavailable, "UNAVAILABLE", "Rendering one clip again is not available right now."), nil
	}
	if len(req.Body) > hookMaxBody {
		return errorResponse(http.StatusBadRequest, "VALIDATION_ERROR", "The request is too large."), nil
	}
	var body ClipRenderRequest
	if err := json.Unmarshal([]byte(req.Body), &body); err != nil {
		return errorResponse(http.StatusBadRequest, "INVALID_BODY", "Invalid request body"), nil
	}
	if !requestIDPattern.MatchString(body.RequestID) {
		return errorResponse(http.StatusBadRequest, "VALIDATION_ERROR", "requestId must be 16 to 64 letters, digits, hyphens or underscores."), nil
	}

	// Ownership: the job is read from the caller's partition, and the clip must
	// be one of that job's rendered clips. Nothing below is charged until all of
	// this holds.
	job, err := h.getShortsJobByID(ctx, claims.UserID, a.jobID)
	if err != nil {
		return errorResponse(http.StatusNotFound, "NOT_FOUND", "Shorts job not found"), nil
	}
	clip := renderedClip(job, a.clipID)
	if clip == nil {
		return errorResponse(http.StatusNotFound, "NOT_FOUND", "Clip not found"), nil
	}
	if job.Status != "completed" {
		return errorResponse(http.StatusConflict, "NOT_READY", "A clip can be rendered again once the whole video has finished. For a failed video, use Retry."), nil
	}
	start, end, rank, ok := clipRenderWindow(job, clip)
	if !ok || job.SourceKey == "" || job.TranscriptKey == "" {
		return errorResponse(http.StatusConflict, "NOT_RENDERABLE", "This clip was made before it could be rendered again. Make a new video from the file instead."), nil
	}
	if hk, ok := job.ClipHooks[a.clipID]; ok {
		if err := render.CheckHook(hk, job.Caption, job.Effects); err != nil {
			return errorResponse(http.StatusBadRequest, "VALIDATION_ERROR", hookCheckMessage(err)), nil
		}
	}

	now := time.Now().UTC()
	rec, err := cliprender.Get(ctx, h.DB, h.TableName, claims.UserID, a.jobID, a.clipID)
	if err != nil {
		log.Printf("clip_render_read_failed user=%s job=%s err=%v", claims.UserID, a.jobID, err)
		return errorResponse(http.StatusInternalServerError, "DB_ERROR", "Could not start the render. Please try again."), nil
	}
	if rec.HasRequest(body.RequestID) {
		return h.clipRenderReplay(ctx, claims.UserID, rec, now), nil
	}
	if rec.Running(now) {
		return errorResponse(http.StatusConflict, "RENDER_IN_PROGRESS", "This clip is already rendering. Wait for it to finish."), nil
	}
	// The source must still be there, or the render can only fail.
	if meta, err := h.S3.ObjectMeta(ctx, h.Bucket, job.SourceKey); err == nil && !meta.Found {
		return errorResponse(http.StatusConflict, "SOURCE_GONE", "The original file for this video is no longer stored, so its clips cannot be rendered again."), nil
	}

	token, err := cliprender.NewToken()
	if err != nil {
		return errorResponse(http.StatusInternalServerError, "INTERNAL_ERROR", "Could not start the render. Please try again."), nil
	}
	next := cliprender.Record{
		UserID: claims.UserID, JobID: a.jobID, ClipID: a.clipID,
		Token: token, State: models.ClipRenderRendering, RequestedAt: now,
		ExecutionName: cliprender.ExecutionName(token),
	}
	var stale string
	if rec != nil {
		next.ActiveKey, next.Renders = rec.ActiveKey, rec.Renders
		// The file before the one that plays is not needed once a new render
		// starts; it is deleted below, after the render is under way.
		stale = rec.PreviousKey
		next.RequestIDs = append(next.RequestIDs, rec.RequestIDs...)
	}
	next.RequestIDs = append(next.RequestIDs, body.RequestID)
	if n := len(next.RequestIDs); n > cliprender.MaxRequestIDs {
		next.RequestIDs = next.RequestIDs[n-cliprender.MaxRequestIDs:]
	}
	if err := cliprender.Claim(ctx, h.DB, h.TableName, rec, next, now); err != nil {
		if errors.Is(err, cliprender.ErrConflict) {
			// Another request got there first. If it was this one, answer it.
			if again, gerr := cliprender.Get(ctx, h.DB, h.TableName, claims.UserID, a.jobID, a.clipID); gerr == nil && again.HasRequest(body.RequestID) {
				return h.clipRenderReplay(ctx, claims.UserID, again, now), nil
			}
			return errorResponse(http.StatusConflict, "RENDER_IN_PROGRESS", "This clip is already rendering. Wait for it to finish."), nil
		}
		log.Printf("clip_render_claim_failed user=%s job=%s err=%v", claims.UserID, a.jobID, err)
		return errorResponse(http.StatusInternalServerError, "DB_ERROR", "Could not start the render. Please try again."), nil
	}
	undo := func() {
		if err := cliprender.Restore(ctx, h.DB, h.TableName, rec, next); err != nil {
			log.Printf("clip_render_restore_failed user=%s job=%s clip=%s err=%v", claims.UserID, a.jobID, a.clipID, err)
		}
	}

	// One use per accepted request: charged once here, never by the worker or
	// its retries, and given back if the render cannot be started.
	used, retryAfter, ok := h.spendQuotaCount(ctx, claims.UserID, quotaShortsRetry)
	if !ok {
		undo()
		return quotaExceeded(quotaShortsRetry, retryAfter), nil
	}
	input, _ := json.Marshal(map[string]any{
		"jobId": a.jobID, "userId": claims.UserID, "sourceKey": job.SourceKey, "transcriptKey": job.TranscriptKey,
		"clipId": a.clipID, "start": start, "end": end, "rank": rank, "token": token,
	})
	if _, err := h.SFN.StartExecution(ctx, &sfn.StartExecutionInput{
		StateMachineArn: aws.String(h.ClipRenderStateMachineARN),
		Name:            aws.String(next.ExecutionName),
		Input:           aws.String(string(input)),
	}); err != nil && !isExecutionAlreadyExists(err) {
		log.Printf("clip_render_start_failed user=%s job=%s clip=%s err=%v", claims.UserID, a.jobID, a.clipID, err)
		h.refundQuota(ctx, claims.UserID, quotaShortsRetry)
		undo()
		return errorResponse(http.StatusServiceUnavailable, "PIPELINE_ERROR", "Could not start the render. Please try again. This did not use up one of today's tries."), nil
	}
	if cliprender.IsOutputKey(stale, claims.UserID, a.jobID, a.clipID) && stale != next.ActiveKey {
		if err := h.S3.DeleteObjects(ctx, h.Bucket, []string{stale}); err != nil {
			log.Printf("clip_render_cleanup_failed user=%s job=%s err=%v", claims.UserID, a.jobID, err)
		}
	}
	log.Printf("clip_render_started user=%s job=%s clip=%s", claims.UserID, a.jobID, a.clipID)
	return jsonResponse(http.StatusAccepted, ClipRenderResponse{
		Render:         next.View(now),
		RemainingToday: remainingOf(quotaShortsRetry, used),
		DailyLimit:     quotaShortsRetry.limit,
	}), nil
}

// clipRenderReplay answers a request already accepted: the render it started,
// as it stands now. Nothing is charged and nothing new starts.
func (h *APIHandler) clipRenderReplay(ctx context.Context, userID string, rec *cliprender.Record, now time.Time) events.APIGatewayProxyResponse {
	return jsonResponse(http.StatusAccepted, ClipRenderResponse{
		Render:         rec.View(now),
		Replayed:       true,
		RemainingToday: remainingOf(quotaShortsRetry, h.quotaUsedToday(ctx, userID, quotaShortsRetry)),
		DailyLimit:     quotaShortsRetry.limit,
	})
}

// attachClipRenders fills in the state of the job's single-clip renders for the
// page. A read failure is logged and leaves them out: the job still loads.
func (h *APIHandler) attachClipRenders(ctx context.Context, job *models.ShortsJob) {
	recs, err := cliprender.List(ctx, h.DB, h.TableName, job.UserID, job.JobID)
	if err != nil {
		log.Printf("clip_render_list_failed user=%s job=%s err=%v", job.UserID, job.JobID, err)
		return
	}
	if len(recs) == 0 {
		return
	}
	now := time.Now()
	job.ClipRenders = map[string]models.ClipRenderView{}
	for id, r := range recs {
		job.ClipRenders[id] = r.View(now)
	}
}

// activeClipKey is the file a clip plays: the latest single-clip render's, when
// there is one, otherwise the pipeline's.
func (h *APIHandler) activeClipKey(ctx context.Context, userID, jobID string, clip *models.Clip) string {
	rec, err := cliprender.Get(ctx, h.DB, h.TableName, userID, jobID, clip.ClipID)
	if err != nil {
		log.Printf("clip_render_read_failed user=%s job=%s err=%v", userID, jobID, err)
		return clip.S3Key
	}
	if rec != nil && cliprender.IsOutputKey(rec.ActiveKey, userID, jobID, clip.ClipID) {
		return rec.ActiveKey
	}
	return clip.S3Key
}

// removeClipRenders stops running single-clip renders of a job and deletes
// their records. It runs before the job's files are deleted, so a render that is
// stopped cannot point a deleted clip at a new file.
func (h *APIHandler) removeClipRenders(ctx context.Context, userID, jobID string) error {
	running, err := cliprender.DeleteAll(ctx, h.DB, h.TableName, userID, jobID, time.Now())
	if h.ClipRenderStateMachineARN != "" {
		for _, name := range running {
			arn, aerr := stateMachineExecutionARN(h.ClipRenderStateMachineARN, name)
			if aerr != nil {
				continue
			}
			if _, serr := h.SFN.StopExecution(ctx, &sfn.StopExecutionInput{ExecutionArn: aws.String(arn)}); serr != nil {
				log.Printf("clip_render_stop_failed user=%s job=%s err=%v", userID, jobID, serr)
			}
		}
	}
	if err != nil {
		return fmt.Errorf("remove clip renders: %w", err)
	}
	return nil
}
