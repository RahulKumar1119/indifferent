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
	"sync"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/smithy-go"
	"github.com/rahul/indifferent/backend/internal/models"
	"github.com/rahul/indifferent/backend/internal/storyjob"
	"github.com/rahul/indifferent/backend/internal/storyplan"
)

// Voice preview (#326): hear one scene's narration in the editor, before a video
// is made.
//
//	POST /story-plans/{id}/voice-preview  { revision, scene, narration?, voice? }
//
// The take is made with Polly once and kept, private, in the assets bucket under
// story-preview/<user>/<hash>.mp3, where the hash covers the text, the voice and
// the engine. Asking for the same take again is a plain read: no Polly call and
// no use of the daily allowance. The render looks for the same key
// (storyjob.PreviewKey), so the video says exactly what was previewed.
const (
	// voicePreviewMaxBody bounds the request: one scene's text and a few fields.
	voicePreviewMaxBody = 16 << 10
	// voicePreviewLinkTTL is how long the signed link to a take works.
	voicePreviewLinkTTL = 5 * time.Minute
	// voicePreviewMakeTimeout bounds one Polly call (which retries on its own),
	// inside the API Lambda's 30 seconds.
	voicePreviewMakeTimeout = 20 * time.Second
	// voicePreviewMaxBytes refuses a take that cannot be one scene's narration.
	voicePreviewMaxBytes = 4 << 20
	// voiceLeaseFor is how long one request may hold the right to make a take.
	voiceLeaseFor = 45 * time.Second
)

// How long a request waits for another's take to appear, and how often it
// looks. Variables so a test can shorten them.
var (
	voiceWaitFor   = 12 * time.Second
	voicePollEvery = 300 * time.Millisecond
)

// VoiceSynth makes one take of narration. *narrator.PollyNarrationService
// implements it. Nil turns the preview off (503), so the API can ship first.
type VoiceSynth interface {
	SynthesizeText(ctx context.Context, text string, voiceID string) ([]byte, error)
}

// PreviewFiles is the part of object storage the preview uses. *storage.S3Client
// implements it.
type PreviewFiles interface {
	HeadObject(ctx context.Context, bucket, key string) (bool, error)
	PutObject(ctx context.Context, bucket, key string, data []byte, contentType string) error
	GenerateDownloadURL(ctx context.Context, bucket, key string, expiration time.Duration) (string, error)
}

// VoicePreviewRequest is the body of POST /story-plans/{id}/voice-preview.
type VoicePreviewRequest struct {
	// Revision is the plan revision the page is showing. A preview of a plan
	// that changed elsewhere is refused, so Scene can never mean another scene.
	Revision int `json:"revision"`
	// Scene is the 1-based scene number.
	Scene int `json:"scene"`
	// Narration is the text being edited, not yet saved. Omitted: the scene's
	// saved narration is spoken. It is cleaned as a save would clean it.
	Narration *string `json:"narration,omitempty"`
	// Voice is one of the voices for the plan's language. Omitted: the default.
	Voice string `json:"voice,omitempty"`
}

// VoicePreviewResponse is a take the page can play.
type VoicePreviewResponse struct {
	// URL is a signed link that works for ExpiresInSeconds.
	URL              string `json:"url"`
	ExpiresInSeconds int    `json:"expiresInSeconds"`
	// Cached is true when the take already existed, so it cost nothing.
	Cached bool `json:"cached"`
	// RemainingToday is how many new takes the user can still make today. Only
	// given when a take was just made; DailyLimit is always given.
	RemainingToday *int64 `json:"remainingToday,omitempty"`
	DailyLimit     int64  `json:"dailyLimit"`
}

// voicePreviewPath returns the plan id of /story-plans/{id}/voice-preview.
func voicePreviewPath(path string) string {
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(parts) != 3 || parts[0] != "story-plans" || parts[2] != "voice-preview" || parts[1] == "" || parts[1] == "limits" {
		return ""
	}
	return parts[1]
}

func (h *APIHandler) handleVoicePreview(ctx context.Context, req events.APIGatewayProxyRequest, claims *models.JWTClaims) (events.APIGatewayProxyResponse, error) {
	started := time.Now()
	if h.VoiceSynth == nil || h.PreviewFiles == nil {
		return errorResponse(http.StatusServiceUnavailable, "UNAVAILABLE", "Voice previews are not available yet"), nil
	}
	id := voicePreviewPath(req.Path)
	if !validStoryPlanID(id) {
		return errorResponse(http.StatusNotFound, "NOT_FOUND", "Plan not found"), nil
	}
	id = strings.ToLower(id)
	if len(req.Body) > voicePreviewMaxBody {
		return errorResponse(http.StatusBadRequest, "VALIDATION_ERROR", "The request is too large"), nil
	}
	var body VoicePreviewRequest
	if err := json.Unmarshal([]byte(req.Body), &body); err != nil {
		return errorResponse(http.StatusBadRequest, "INVALID_BODY", "Invalid request body"), nil
	}

	out, err := h.DB.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(h.TableName),
		Key: map[string]dbtypes.AttributeValue{
			"PK": &dbtypes.AttributeValueMemberS{Value: "USER#" + claims.UserID},
			"SK": &dbtypes.AttributeValueMemberS{Value: storyPlanPrefix + id},
		},
	})
	if err != nil {
		log.Printf("story_voice_plan_read_failed user=%s plan=%s err=%v", claims.UserID, id, err)
		return errorResponse(http.StatusInternalServerError, "DB_ERROR", "Could not read the plan"), nil
	}
	if len(out.Item) == 0 {
		return errorResponse(http.StatusNotFound, "NOT_FOUND", "Plan not found"), nil
	}
	rec := itemToStoryPlan(out.Item)
	rec.UserID = claims.UserID
	if rec.Status != storyPlanReady || rec.Plan == nil {
		return errorResponse(http.StatusConflict, "NOT_READY", "This plan has no scenes to hear yet"), nil
	}
	if body.Revision != rec.Revision {
		return errorResponse(http.StatusConflict, "STALE_REVISION", "This plan was changed somewhere else. Reload it and try again."), nil
	}
	if body.Scene < 1 || body.Scene > len(rec.Plan.Scenes) {
		return errorResponse(http.StatusBadRequest, "VALIDATION_ERROR", fmt.Sprintf("This plan has %d scenes.", len(rec.Plan.Scenes))), nil
	}

	text := rec.Plan.Scenes[body.Scene-1].Narration
	if body.Narration != nil {
		cleaned, err := storyplan.CleanNarration(*body.Narration, storyplan.Script(rec.Script), body.Scene)
		if err != nil {
			var ee *storyplan.EditError
			if errors.As(err, &ee) {
				return errorResponse(http.StatusBadRequest, "VALIDATION_ERROR", ee.Reason), nil
			}
			return errorResponse(http.StatusBadRequest, "VALIDATION_ERROR", "That narration cannot be spoken"), nil
		}
		text = cleaned
	}
	if strings.TrimSpace(text) == "" {
		return errorResponse(http.StatusBadRequest, "VALIDATION_ERROR", fmt.Sprintf("Scene %d has no narration to hear.", body.Scene)), nil
	}
	voice, err := storyjob.VoiceFor(rec.Plan.Language, body.Voice)
	if err != nil {
		msg := "That voice cannot read this story."
		var um interface{ UserMessage() string }
		if errors.As(err, &um) {
			msg = um.UserMessage()
		}
		return errorResponse(http.StatusBadRequest, "VALIDATION_ERROR", msg), nil
	}

	key := storyjob.PreviewKey(claims.UserID, text, voice)
	resp := func(cached bool, remaining *int64) (events.APIGatewayProxyResponse, error) {
		url, err := h.PreviewFiles.GenerateDownloadURL(ctx, h.Bucket, key, voicePreviewLinkTTL)
		if err != nil {
			log.Printf("story_voice_link_failed user=%s err=%v", claims.UserID, err)
			return errorResponse(http.StatusInternalServerError, "STORAGE_ERROR", "Could not open the voice. Please try again."), nil
		}
		log.Printf("story_voice_preview user=%s plan=%s scene=%d cached=%t ms=%d", claims.UserID, id, body.Scene, cached, time.Since(started).Milliseconds())
		return jsonResponse(http.StatusOK, VoicePreviewResponse{
			URL: url, ExpiresInSeconds: int(voicePreviewLinkTTL.Seconds()), Cached: cached,
			RemainingToday: remaining, DailyLimit: quotaVoicePreview.limit,
		}), nil
	}
	// A take that exists is only read. Replays have their own, generous cap.
	replay := func() (events.APIGatewayProxyResponse, bool) {
		if retryAfter, ok := h.spendQuota(ctx, claims.UserID, quotaVoicePreviewReplay); !ok {
			return quotaExceeded(quotaVoicePreviewReplay, retryAfter), false
		}
		return events.APIGatewayProxyResponse{}, true
	}
	have, err := h.PreviewFiles.HeadObject(ctx, h.Bucket, key)
	if err != nil {
		log.Printf("story_voice_head_failed user=%s err=%v", claims.UserID, err)
		return errorResponse(http.StatusInternalServerError, "STORAGE_ERROR", "Could not check the voice. Please try again."), nil
	}
	if have {
		if r, ok := replay(); !ok {
			return r, nil
		}
		return resp(true, nil)
	}

	// Making the take. One request at a time per key: first in this process,
	// then across processes with a short lease in DynamoDB. A request that loses
	// waits for the winner's take and plays it, so two clicks, two tabs or two
	// Lambda instances make one Polly call and use one unit of the allowance.
	unlockLocal, err := h.voiceKeys.lock(ctx, key)
	if err != nil {
		return errorResponse(http.StatusServiceUnavailable, "VOICE_BUSY", "The voice is busy. Try again in a moment."), nil
	}
	defer unlockLocal()
	release, outcome := h.acquireVoiceLease(ctx, key)
	defer release()
	if outcome == leaseTimedOut {
		return tooManyRequests("VOICE_BUSY", "The voice is busy. Try again in a moment.", 5), nil
	}
	if outcome == leaseMade || func() bool {
		ok, herr := h.PreviewFiles.HeadObject(ctx, h.Bucket, key)
		return herr == nil && ok
	}() {
		// Someone else made it while this request waited.
		if r, ok := replay(); !ok {
			return r, nil
		}
		return resp(true, nil)
	}

	used, retryAfter, ok := h.spendQuotaCount(ctx, claims.UserID, quotaVoicePreview)
	if !ok {
		return quotaExceeded(quotaVoicePreview, retryAfter), nil
	}
	mctx, cancel := context.WithTimeout(ctx, voicePreviewMakeTimeout)
	defer cancel()
	audio, err := h.VoiceSynth.SynthesizeText(mctx, text, voice)
	if err == nil && (len(audio) == 0 || len(audio) > voicePreviewMaxBytes) {
		err = fmt.Errorf("the take is %d bytes", len(audio))
	}
	if err == nil {
		err = h.PreviewFiles.PutObject(ctx, h.Bucket, key, audio, "audio/mpeg")
		if err != nil {
			h.refundQuota(ctx, claims.UserID, quotaVoicePreview)
			log.Printf("story_voice_store_failed user=%s err=%v", claims.UserID, err)
			return errorResponse(http.StatusInternalServerError, "STORAGE_ERROR", "Could not keep the voice. Please try again."), nil
		}
	}
	if err != nil {
		// Not the user's doing: give the use back.
		h.refundQuota(ctx, claims.UserID, quotaVoicePreview)
		log.Printf("story_voice_make_failed user=%s plan=%s scene=%d err=%v", claims.UserID, id, body.Scene, err)
		return voiceFailure(err), nil
	}
	var remaining *int64
	if used >= 0 {
		r := quotaVoicePreview.limit - used
		if r < 0 {
			r = 0
		}
		remaining = &r
	}
	return resp(false, remaining)
}

// voiceFailure maps a failed Polly call to what the user is told. Polly's own
// retries have already run, so a throttle here means it stayed throttled.
func voiceFailure(err error) events.APIGatewayProxyResponse {
	var api smithy.APIError
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return tooManyRequests("VOICE_BUSY", "The voice took too long. Try again in a moment.", 10)
	case errors.As(err, &api) && strings.Contains(strings.ToLower(api.ErrorCode()), "throttl"):
		return tooManyRequests("VOICE_BUSY", "The voice is busy right now. Try again in a moment.", 10)
	}
	return errorResponse(http.StatusBadGateway, "VOICE_FAILED", "The voice could not be made. Try again in a few minutes.")
}

// keyedLocks serialises work per key inside one process, and gives up when the
// request's context ends.
type keyedLocks struct {
	mu sync.Mutex
	m  map[string]*keyedLock
}

type keyedLock struct {
	sem  chan struct{}
	refs int
}

func (k *keyedLocks) lock(ctx context.Context, key string) (func(), error) {
	k.mu.Lock()
	if k.m == nil {
		k.m = map[string]*keyedLock{}
	}
	l := k.m[key]
	if l == nil {
		l = &keyedLock{sem: make(chan struct{}, 1)}
		k.m[key] = l
	}
	l.refs++
	k.mu.Unlock()
	drop := func() {
		k.mu.Lock()
		l.refs--
		if l.refs == 0 {
			delete(k.m, key)
		}
		k.mu.Unlock()
	}
	select {
	case l.sem <- struct{}{}:
		return func() { <-l.sem; drop() }, nil
	case <-ctx.Done():
		drop()
		return nil, ctx.Err()
	}
}

type leaseOutcome int

const (
	// leaseHeld: this request holds the lease and should make the take.
	leaseHeld leaseOutcome = iota
	// leaseMade: another instance made the take while this one waited.
	leaseMade
	// leaseTimedOut: another instance held the lease and no take appeared in time.
	leaseTimedOut
)

// acquireVoiceLease takes a short lease on a key in DynamoDB, so two Lambda
// instances do not both call Polly for the same take. It fails open: with no
// quota table, or on a database error other than "someone holds it", the caller
// carries on and makes the take (a rare duplicate is harmless, same key and
// same bytes; only the lease is lost). release is always safe to call.
func (h *APIHandler) acquireVoiceLease(ctx context.Context, key string) (release func(), outcome leaseOutcome) {
	noop := func() {}
	if h.QuotaTable == "" {
		return noop, leaseHeld
	}
	lockKey := map[string]dbtypes.AttributeValue{
		"PK": &dbtypes.AttributeValueMemberS{Value: "VOICELOCK#" + key},
		"SK": &dbtypes.AttributeValueMemberS{Value: "LEASE"},
	}
	deadline := time.Now().Add(voiceWaitFor)
	for {
		now := time.Now()
		_, err := h.DB.UpdateItem(ctx, &dynamodb.UpdateItemInput{
			TableName:           aws.String(h.QuotaTable),
			Key:                 lockKey,
			UpdateExpression:    aws.String("SET leaseUntil = :until, expiresAt = :exp"),
			ConditionExpression: aws.String("attribute_not_exists(leaseUntil) OR leaseUntil < :now"),
			ExpressionAttributeValues: map[string]dbtypes.AttributeValue{
				":until": &dbtypes.AttributeValueMemberN{Value: strconv.FormatInt(now.Add(voiceLeaseFor).UnixMilli(), 10)},
				":exp":   &dbtypes.AttributeValueMemberN{Value: strconv.FormatInt(now.Add(time.Hour).Unix(), 10)},
				":now":   &dbtypes.AttributeValueMemberN{Value: strconv.FormatInt(now.UnixMilli(), 10)},
			},
		})
		if err == nil {
			return func() {
				// A fresh context: the request's may be the reason we are leaving.
				dctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()
				if _, derr := h.DB.DeleteItem(dctx, &dynamodb.DeleteItemInput{TableName: aws.String(h.QuotaTable), Key: lockKey}); derr != nil {
					log.Printf("story_voice_lease_release_failed err=%v", derr)
				}
			}, leaseHeld
		}
		var held *dbtypes.ConditionalCheckFailedException
		if !errors.As(err, &held) {
			log.Printf("story_voice_lease_failed err=%v", err)
			return noop, leaseHeld
		}
		// Another instance is making it: wait for the take to appear.
		if ok, herr := h.PreviewFiles.HeadObject(ctx, h.Bucket, key); herr == nil && ok {
			return noop, leaseMade
		}
		if time.Now().After(deadline) || ctx.Err() != nil {
			return noop, leaseTimedOut
		}
		select {
		case <-time.After(voicePollEvery):
		case <-ctx.Done():
			return noop, leaseTimedOut
		}
	}
}
