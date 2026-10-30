package api

import (
	"context"
	"crypto/rand"
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
	"github.com/google/uuid"
	"github.com/rahul/indifferent/backend/internal/models"
	"github.com/rahul/indifferent/backend/internal/stockmedia"
	"github.com/rahul/indifferent/backend/internal/storyplan"
)

// Story plan endpoints (issue #191, phase 2 of #162):
//
//	GET  /story-plans/limits   what the planner accepts (no sign-in needed)
//	POST /story-plans          plan a video from a story
//	GET  /story-plans/{id}     read a stored plan back
//
// The plan is generated synchronously. Measured against real Bedrock with the
// default model (Kimi K3): 34 calls across English, Hindi and Hinglish stories,
// including stories at the 5,000 character limit, had a median of about 5 s and
// a slowest of 14.4 s (Hindi is the slower language, since Devanagari costs more
// tokens). A 25 s deadline leaves a wide margin under the 29 s API Gateway
// limit. If a slower model is ever configured, this is the place that has to
// become asynchronous.
//
// Storage: one item per attempt in the projects table, PK USER#{userId} and
// SK STORYPLAN#{planId}. The plan ID is a UUIDv7, whose text form sorts by
// creation time, so "the plans of the last hour" is one range query on the sort
// key and "newest first" needs no index. The story text is never stored.

const (
	storyPlanPrefix = "STORYPLAN#"

	// storyPlanDefaultPerHour is how many plans one user may create per hour,
	// counting failed attempts, which cost model tokens too.
	storyPlanDefaultPerHour = 20
	storyPlanWindow         = time.Hour

	// storyPlanMaxBodyBytes bounds the request body before it is parsed. A
	// story of MaxStoryRunes characters in Devanagari is 15 KB in UTF-8; JSON
	// escaping can double that. 64 KB is generous and still tiny.
	storyPlanMaxBodyBytes = 64 << 10
)

// Plan record statuses.
const (
	storyPlanPending = "pending"
	storyPlanReady   = "ready"
	storyPlanFailed  = "failed"
)

// StoryPlanner turns a story into a plan. *storyplan.Service implements it.
type StoryPlanner interface {
	Generate(ctx context.Context, story string) (storyplan.Result, error)
	Model() string
}

// CreateStoryPlanRequest is the body of POST /story-plans.
type CreateStoryPlanRequest struct {
	Story string `json:"story"`
}

// StoryPlanResponse is a stored plan as the API returns it. Plan is present
// only when Status is "ready"; ErrorCode only when it is "failed".
//
// Revision counts the saves of the plan, starting at 1. A PUT must send the
// revision it read, so two open tabs cannot silently overwrite each other.
type StoryPlanResponse struct {
	PlanID    string          `json:"planId"`
	Status    string          `json:"status"`
	Model     string          `json:"model,omitempty"`
	Revision  int             `json:"revision"`
	Plan      *storyplan.Plan `json:"plan,omitempty"`
	ErrorCode string          `json:"errorCode,omitempty"`
	CreatedAt string          `json:"createdAt"`
	UpdatedAt string          `json:"updatedAt"`
	// Media holds, for each scene that has a pick, the asset as it is now
	// (fresh preview URLs) or why it cannot be shown. Index i is scene i; a
	// scene with no pick has a nil entry. Omitted when the plan has no picks or
	// stock search is not configured.
	Media []*StockAssetItem `json:"media,omitempty"`
	// MediaAttributions are the credits to show for the assets in Media.
	MediaAttributions []StockAttribution `json:"mediaAttributions,omitempty"`
}

// UpdateStoryPlanRequest is the body of PUT /story-plans/:id. Scenes[i] edits
// scene i, and there must be exactly one entry per scene.
type UpdateStoryPlanRequest struct {
	// Revision is the revision the edit was made against.
	Revision int                   `json:"revision"`
	Scenes   []storyplan.SceneEdit `json:"scenes"`
}

// StoryPlanLimits tells the page what the planner accepts, so the form never
// drifts from the backend (the shorts limits endpoint does the same).
type StoryPlanLimits struct {
	Enabled         bool    `json:"enabled"`
	MinStoryChars   int     `json:"minStoryChars"`
	MaxStoryChars   int     `json:"maxStoryChars"`
	MinScenes       int     `json:"minScenes"`
	MaxScenes       int     `json:"maxScenes"`
	MaxTotalSeconds float64 `json:"maxTotalSeconds"`
	PerHour         int     `json:"perHour"`
}

// storyPlanRecord is the stored form of one attempt.
type storyPlanRecord struct {
	UserID     string
	PlanID     string
	Status     string
	Model      string
	StoryRunes int
	Script     string
	ErrorCode  string
	LatencyMs  int64
	Revision   int
	Plan       *storyplan.Plan
	CreatedAt  string
	UpdatedAt  string
}

// storyPlanMaxPerHour bounds the configured allowance. The allowance check reads
// up to this many keys, so a mistaken huge value cannot turn it into a scan.
const storyPlanMaxPerHour = 1000

func (h *APIHandler) storyPlansPerHour() int {
	switch {
	case h.StoryPlansPerHour <= 0:
		return storyPlanDefaultPerHour
	case h.StoryPlansPerHour > storyPlanMaxPerHour:
		return storyPlanMaxPerHour
	}
	return h.StoryPlansPerHour
}

func (h *APIHandler) storyPlanNow() time.Time {
	if h.StoryPlanNow != nil {
		return h.StoryPlanNow()
	}
	return time.Now()
}

// isStoryPlanPath reports whether path is /story-plans or under it.
func isStoryPlanPath(path string) bool {
	return path == "/story-plans" || strings.HasPrefix(path, "/story-plans/")
}

// storyPlanIDFromPath returns the plan ID of /story-plans/{id}, or "" for any
// other shape (including deeper paths and the reserved word "limits").
func storyPlanIDFromPath(path string) string {
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(parts) != 2 || parts[0] != "story-plans" || parts[1] == "" || parts[1] == "limits" {
		return ""
	}
	return parts[1]
}

// validStoryPlanID reports whether id is a UUID in its canonical 36 character
// form, in either case. Checking it before the database read keeps arbitrary
// text out of key lookups; uuid.Parse alone would also accept braces, a urn
// prefix and the form without dashes.
func validStoryPlanID(id string) bool {
	if len(id) != 36 {
		return false
	}
	u, err := uuid.Parse(id)
	return err == nil && u.String() == strings.ToLower(id)
}

// handleStoryPlans routes the story-plan endpoints other than the public
// limits one.
func (h *APIHandler) handleStoryPlans(ctx context.Context, req events.APIGatewayProxyRequest, claims *models.JWTClaims) (events.APIGatewayProxyResponse, error) {
	switch {
	case req.HTTPMethod == "POST" && req.Path == "/story-plans":
		return h.handleCreateStoryPlan(ctx, req, claims)
	case req.HTTPMethod == "GET" && storyPlanIDFromPath(req.Path) != "":
		return h.handleGetStoryPlan(ctx, req, claims)
	case req.HTTPMethod == "PUT" && storyPlanIDFromPath(req.Path) != "":
		return h.handleUpdateStoryPlan(ctx, req, claims)
	}
	return errorResponse(http.StatusNotFound, "NOT_FOUND", "Route not found"), nil
}

// handleGetStoryPlanLimits answers GET /story-plans/limits.
func (h *APIHandler) handleGetStoryPlanLimits() events.APIGatewayProxyResponse {
	return jsonResponse(http.StatusOK, StoryPlanLimits{
		Enabled:         h.StoryPlanner != nil,
		MinStoryChars:   storyplan.MinStoryRunes,
		MaxStoryChars:   storyplan.MaxStoryRunes,
		MinScenes:       storyplan.MinScenes,
		MaxScenes:       storyplan.MaxScenes,
		MaxTotalSeconds: storyplan.MaxTotalSeconds,
		PerHour:         h.storyPlansPerHour(),
	})
}

// handleCreateStoryPlan validates the story, checks the user's hourly allowance,
// records the attempt, asks the planner, and stores and returns the result.
//
// The order matters. Validation comes first so a bad request neither spends
// allowance nor reaches the model. The allowance check and the "pending" record
// come before the model call, so an attempt that crashes or runs concurrently
// still counts, and the check fails closed: if the database cannot be read, no
// model call is made.
func (h *APIHandler) handleCreateStoryPlan(ctx context.Context, req events.APIGatewayProxyRequest, claims *models.JWTClaims) (events.APIGatewayProxyResponse, error) {
	if h.StoryPlanner == nil {
		return errorResponse(http.StatusServiceUnavailable, "UNAVAILABLE", "Story planning is not available yet"), nil
	}
	if len(req.Body) > storyPlanMaxBodyBytes {
		return errorResponse(http.StatusBadRequest, "VALIDATION_ERROR", "The request is too large"), nil
	}
	var body CreateStoryPlanRequest
	if err := json.Unmarshal([]byte(req.Body), &body); err != nil {
		return errorResponse(http.StatusBadRequest, "INVALID_BODY", "Invalid request body"), nil
	}
	story, err := storyplan.NormalizeStory(body.Story)
	if err != nil {
		return errorResponse(http.StatusBadRequest, "VALIDATION_ERROR", storyplan.StoryReason(err)), nil
	}

	now := h.storyPlanNow().UTC()
	limited, retryAfter, err := h.storyPlanLimited(ctx, claims.UserID, now)
	if err != nil {
		log.Printf("story_plan_limit_check_failed user=%s err=%v", claims.UserID, err)
		return errorResponse(http.StatusInternalServerError, "DB_ERROR", "Could not create the plan. Please try again."), nil
	}
	if limited {
		resp := errorResponse(http.StatusTooManyRequests, "RATE_LIMITED",
			fmt.Sprintf("You can create %d plans an hour. Try again later.", h.storyPlansPerHour()))
		resp.Headers["Retry-After"] = strconv.Itoa(maxInt(1, int(retryAfter.Seconds())))
		return resp, nil
	}

	planID, err := newStoryPlanID(now)
	if err != nil {
		return errorResponse(http.StatusInternalServerError, "INTERNAL_ERROR", "Could not create the plan"), nil
	}
	stamp := now.Format(time.RFC3339)
	rec := storyPlanRecord{
		UserID:     claims.UserID,
		PlanID:     planID,
		Status:     storyPlanPending,
		Model:      h.StoryPlanner.Model(),
		StoryRunes: len([]rune(story)),
		CreatedAt:  stamp,
		UpdatedAt:  stamp,
	}
	if err := h.putStoryPlan(ctx, rec); err != nil {
		log.Printf("story_plan_record_failed user=%s plan=%s err=%v", claims.UserID, rec.PlanID, err)
		return errorResponse(http.StatusInternalServerError, "DB_ERROR", "Could not create the plan. Please try again."), nil
	}

	res, genErr := h.StoryPlanner.Generate(ctx, story)

	rec.UpdatedAt = h.storyPlanNow().UTC().Format(time.RFC3339)
	rec.Script = string(res.Script)
	rec.LatencyMs = res.Latency.Milliseconds()
	if genErr != nil {
		rec.Status = storyPlanFailed
		rec.ErrorCode = storyPlanErrorCode(genErr)
		// Record the outcome with a context that outlives a cancelled request, so
		// the attempt is not left looking like it is still running.
		saveCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		err := h.putStoryPlan(saveCtx, rec)
		cancel()
		if err != nil {
			log.Printf("story_plan_record_failed user=%s plan=%s err=%v", claims.UserID, rec.PlanID, err)
		}
		return storyPlanErrorResponse(genErr), nil
	}

	rec.Status = storyPlanReady
	rec.Revision = 1
	rec.Plan = &res.Plan
	if err := h.putStoryPlan(ctx, rec); err != nil {
		log.Printf("story_plan_save_failed user=%s plan=%s err=%v", claims.UserID, rec.PlanID, err)
		return errorResponse(http.StatusInternalServerError, "DB_ERROR", "The plan was made but could not be saved. Please try again."), nil
	}
	return jsonResponse(http.StatusCreated, rec.response()), nil
}

// storyPlanErrorCode is the stable code stored with a failed attempt.
func storyPlanErrorCode(err error) string {
	switch {
	case errors.Is(err, storyplan.ErrModelTimeout):
		return "timeout"
	case errors.Is(err, storyplan.ErrModelFailed):
		return "model_failed"
	case errors.Is(err, storyplan.ErrInvalidPlan):
		return "invalid_plan"
	case errors.Is(err, context.Canceled):
		return "canceled"
	}
	return "error"
}

// storyPlanErrorResponse maps a planner failure to a response. The messages
// are fixed text: nothing from the model or the story is echoed back.
func storyPlanErrorResponse(err error) events.APIGatewayProxyResponse {
	switch {
	case errors.Is(err, storyplan.ErrInvalidStory):
		return errorResponse(http.StatusBadRequest, "VALIDATION_ERROR", storyplan.StoryReason(err))
	case errors.Is(err, storyplan.ErrModelTimeout):
		return errorResponse(http.StatusGatewayTimeout, "PLAN_TIMEOUT", "The planner took too long. Please try again.")
	case errors.Is(err, storyplan.ErrModelFailed):
		return errorResponse(http.StatusServiceUnavailable, "PLAN_UNAVAILABLE", "The planner is busy right now. Please try again in a minute.")
	case errors.Is(err, storyplan.ErrInvalidPlan):
		return errorResponse(http.StatusBadGateway, "PLAN_FAILED", "The planner did not produce a usable plan. Please try again.")
	}
	return errorResponse(http.StatusInternalServerError, "INTERNAL_ERROR", "Could not create the plan. Please try again.")
}

// handleGetStoryPlan reads a stored plan. The read is keyed by the caller's own
// partition, so another user's plan is simply not found.
func (h *APIHandler) handleGetStoryPlan(ctx context.Context, req events.APIGatewayProxyRequest, claims *models.JWTClaims) (events.APIGatewayProxyResponse, error) {
	id := storyPlanIDFromPath(req.Path)
	if !validStoryPlanID(id) {
		return errorResponse(http.StatusNotFound, "NOT_FOUND", "Plan not found"), nil
	}
	id = strings.ToLower(id) // plan IDs are stored in lower case
	out, err := h.DB.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(h.TableName),
		Key: map[string]dbtypes.AttributeValue{
			"PK": &dbtypes.AttributeValueMemberS{Value: "USER#" + claims.UserID},
			"SK": &dbtypes.AttributeValueMemberS{Value: storyPlanPrefix + id},
		},
	})
	if err != nil {
		log.Printf("story_plan_read_failed user=%s plan=%s err=%v", claims.UserID, id, err)
		return errorResponse(http.StatusInternalServerError, "DB_ERROR", "Could not read the plan"), nil
	}
	if len(out.Item) == 0 {
		return errorResponse(http.StatusNotFound, "NOT_FOUND", "Plan not found"), nil
	}
	rec := itemToStoryPlan(out.Item)
	rec.UserID = claims.UserID
	resp := rec.response()
	h.attachMedia(ctx, &resp)
	return jsonResponse(http.StatusOK, resp), nil
}

// attachMedia fills resp.Media with the current details of each chosen asset.
// It never fails the request: an asset that cannot be looked up is reported as
// such in its entry, so a plan always opens. A plan with no picks costs no
// lookups.
func (h *APIHandler) attachMedia(ctx context.Context, resp *StoryPlanResponse) {
	if h.StockMedia == nil || resp.Plan == nil {
		return
	}
	var refs []stockmedia.AssetRef
	var at []int // scene index of each ref
	for i, sc := range resp.Plan.Scenes {
		if sc.Media == nil {
			continue
		}
		ref := stockmedia.AssetRef{Provider: stockmedia.ProviderName(sc.Media.Provider), Type: stockmedia.MediaType(sc.Media.Type), ID: sc.Media.ID}
		if !ref.Valid() {
			continue
		}
		refs = append(refs, ref)
		at = append(at, i)
	}
	if len(refs) == 0 {
		return
	}
	items, atts := h.resolveAssets(ctx, refs)
	resp.Media = make([]*StockAssetItem, len(resp.Plan.Scenes))
	for k, item := range items {
		item := item
		resp.Media[at[k]] = &item
	}
	resp.MediaAttributions = atts
}

// handleUpdateStoryPlan saves a user's edits: narration and the stock asset
// chosen for each scene. The story plan stays the user's own: edits are held to
// the same rules as the model's output (storyplan.ApplyEdits).
//
// The write is conditional on the revision the edit was made against, so a
// stale edit gets 409 and the page can reload instead of overwriting newer work.
// A newly chosen asset is looked up first, so only references that resolve are
// stored; a reference that was already in the plan is kept without a lookup, so
// editing narration never depends on a stock provider being reachable.
func (h *APIHandler) handleUpdateStoryPlan(ctx context.Context, req events.APIGatewayProxyRequest, claims *models.JWTClaims) (events.APIGatewayProxyResponse, error) {
	id := storyPlanIDFromPath(req.Path)
	if !validStoryPlanID(id) {
		return errorResponse(http.StatusNotFound, "NOT_FOUND", "Plan not found"), nil
	}
	id = strings.ToLower(id)
	if len(req.Body) > storyPlanMaxBodyBytes {
		return errorResponse(http.StatusBadRequest, "VALIDATION_ERROR", "The request is too large"), nil
	}
	var body UpdateStoryPlanRequest
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
		log.Printf("story_plan_read_failed user=%s plan=%s err=%v", claims.UserID, id, err)
		return errorResponse(http.StatusInternalServerError, "DB_ERROR", "Could not read the plan"), nil
	}
	if len(out.Item) == 0 {
		return errorResponse(http.StatusNotFound, "NOT_FOUND", "Plan not found"), nil
	}
	rec := itemToStoryPlan(out.Item)
	rec.UserID = claims.UserID
	if rec.Status != storyPlanReady || rec.Plan == nil {
		return errorResponse(http.StatusConflict, "NOT_READY", "This plan has nothing to edit"), nil
	}
	if body.Revision != rec.Revision {
		return errorResponse(http.StatusConflict, "STALE_REVISION", "This plan was changed somewhere else. Reload it and try again."), nil
	}

	script := storyplan.Script(rec.Script)
	edited, err := storyplan.ApplyEdits(*rec.Plan, body.Scenes, script)
	if err != nil {
		var ee *storyplan.EditError
		if errors.As(err, &ee) {
			return errorResponse(http.StatusBadRequest, "VALIDATION_ERROR", ee.Reason), nil
		}
		return errorResponse(http.StatusBadRequest, "VALIDATION_ERROR", "Those changes cannot be applied"), nil
	}

	if resp := h.verifyNewPicks(ctx, *rec.Plan, edited); resp != nil {
		return *resp, nil
	}

	prev := rec.Revision
	rec.Plan = &edited
	rec.Revision = prev + 1
	rec.UpdatedAt = h.storyPlanNow().UTC().Format(time.RFC3339)
	item, err := storyPlanToItem(rec)
	if err != nil {
		return errorResponse(http.StatusInternalServerError, "INTERNAL_ERROR", "Could not save the plan"), nil
	}
	put := &dynamodb.PutItemInput{
		TableName:                aws.String(h.TableName),
		Item:                     item,
		ExpressionAttributeNames: map[string]string{"#rev": "revision"},
	}
	if prev == 0 {
		// A plan saved before revisions existed has no revision attribute, and
		// "revision = 0" is false for a missing attribute.
		put.ConditionExpression = aws.String("attribute_not_exists(#rev)")
	} else {
		put.ConditionExpression = aws.String("#rev = :prev")
		put.ExpressionAttributeValues = map[string]dbtypes.AttributeValue{
			":prev": &dbtypes.AttributeValueMemberN{Value: strconv.Itoa(prev)},
		}
	}
	_, err = h.DB.PutItem(ctx, put)
	if err != nil {
		var cf *dbtypes.ConditionalCheckFailedException
		if errors.As(err, &cf) {
			return errorResponse(http.StatusConflict, "STALE_REVISION", "This plan was changed somewhere else. Reload it and try again."), nil
		}
		log.Printf("story_plan_update_failed user=%s plan=%s err=%v", claims.UserID, id, err)
		return errorResponse(http.StatusInternalServerError, "DB_ERROR", "Could not save the plan. Please try again."), nil
	}
	resp := rec.response()
	h.attachMedia(ctx, &resp)
	return jsonResponse(http.StatusOK, resp), nil
}

// verifyNewPicks checks every media reference that is new in edited (absent
// from before, or different) against the provider, and returns an error
// response if one cannot be used. References unchanged from before are not
// looked up. It returns nil when all is well.
func (h *APIHandler) verifyNewPicks(ctx context.Context, before, edited storyplan.Plan) *events.APIGatewayProxyResponse {
	var refs []stockmedia.AssetRef
	var scene []int
	for i, sc := range edited.Scenes {
		if sc.Media == nil {
			continue
		}
		if old := before.Scenes[i].Media; old != nil && *old == *sc.Media {
			continue
		}
		refs = append(refs, stockmedia.AssetRef{Provider: stockmedia.ProviderName(sc.Media.Provider), Type: stockmedia.MediaType(sc.Media.Type), ID: sc.Media.ID})
		scene = append(scene, i+1)
	}
	if len(refs) == 0 {
		return nil
	}
	if h.StockMedia == nil {
		r := errorResponse(http.StatusServiceUnavailable, "UNAVAILABLE", "Stock media is not available yet, so a new picture cannot be chosen")
		return &r
	}
	for k, res := range h.StockMedia.GetMany(ctx, refs) {
		switch {
		case res.Asset != nil:
		case errors.Is(res.Err, stockmedia.ErrNotFound):
			r := errorResponse(http.StatusBadRequest, "VALIDATION_ERROR", fmt.Sprintf("Scene %d: that picture is no longer available. Choose another.", scene[k]))
			return &r
		default:
			r := errorResponse(http.StatusServiceUnavailable, "PLAN_UNAVAILABLE", "The photo library could not confirm that choice. Please try again.")
			return &r
		}
	}
	return nil
}

func (r storyPlanRecord) response() StoryPlanResponse {
	return StoryPlanResponse{
		PlanID:    r.PlanID,
		Status:    r.Status,
		Model:     r.Model,
		Revision:  r.Revision,
		Plan:      r.Plan,
		ErrorCode: r.ErrorCode,
		CreatedAt: r.CreatedAt,
		UpdatedAt: r.UpdatedAt,
	}
}

// storyPlanLimited reports whether the user already made the hourly allowance
// of plans, and if so how long until the oldest of them leaves the window.
//
// It reads at most `limit` keys of the user's last-hour range, oldest first.
// The range comes from the time ordering of the plan IDs, so no scan or index
// is involved. Reaching `limit` items means the allowance is used.
func (h *APIHandler) storyPlanLimited(ctx context.Context, userID string, now time.Time) (bool, time.Duration, error) {
	limit := h.storyPlansPerHour()
	// Bounded here as well as in storyPlansPerHour, next to the conversion, so
	// the int32 below is visibly safe.
	if limit < 1 {
		limit = 1
	} else if limit > storyPlanMaxPerHour {
		limit = storyPlanMaxPerHour
	}
	out, err := h.DB.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(h.TableName),
		KeyConditionExpression: aws.String("PK = :pk AND SK BETWEEN :lo AND :hi"),
		ExpressionAttributeValues: map[string]dbtypes.AttributeValue{
			":pk": &dbtypes.AttributeValueMemberS{Value: "USER#" + userID},
			":lo": &dbtypes.AttributeValueMemberS{Value: storyPlanPrefix + uuidV7Floor(now.Add(-storyPlanWindow))},
			// '~' sorts after every character a UUID can contain.
			":hi": &dbtypes.AttributeValueMemberS{Value: storyPlanPrefix + "~"},
		},
		ProjectionExpression: aws.String("SK, createdAt"),
		ScanIndexForward:     aws.Bool(true),
		Limit:                aws.Int32(int32(limit)),
	})
	if err != nil {
		return false, 0, err
	}
	if len(out.Items) < limit {
		return false, 0, nil
	}
	retry := time.Minute
	if v, ok := out.Items[0]["createdAt"].(*dbtypes.AttributeValueMemberS); ok {
		if t, err := time.Parse(time.RFC3339, v.Value); err == nil {
			if wait := t.Add(storyPlanWindow).Sub(now); wait > 0 {
				retry = wait
			}
		}
	}
	return true, retry, nil
}

// newStoryPlanID returns a UUIDv7 for time now: the 48 bit millisecond timestamp
// first, so IDs sort by creation time as text, then 74 random bits. It is built
// here, not with uuid.NewV7, so that the time comes from the handler's clock
// (which tests control) and the ID and the allowance window cannot disagree.
func newStoryPlanID(now time.Time) (string, error) {
	var u uuid.UUID
	if _, err := rand.Read(u[:]); err != nil {
		return "", err
	}
	ms := now.UnixMilli()
	if ms < 0 {
		ms = 0
	}
	t := uint64(ms) & (1<<48 - 1)
	u[0], u[1], u[2], u[3], u[4], u[5] = byte(t>>40), byte(t>>32), byte(t>>24), byte(t>>16), byte(t>>8), byte(t)
	u[6] = u[6]&0x0f | 0x70 // version 7
	u[8] = u[8]&0x3f | 0x80 // RFC 4122 variant
	return u.String(), nil
}

// uuidV7Floor returns the leading 13 characters of the UUIDv7 text form for
// time t: the 48 bit millisecond timestamp as "xxxxxxxx-xxxx". Every UUIDv7
// created at or after t sorts at or after this prefix, and every one created
// before it sorts before.
func uuidV7Floor(t time.Time) string {
	ms := t.UnixMilli()
	if ms < 0 {
		ms = 0
	}
	u := uint64(ms) & (1<<48 - 1)
	return fmt.Sprintf("%08x-%04x", u>>16, u&0xffff)
}

func (h *APIHandler) putStoryPlan(ctx context.Context, rec storyPlanRecord) error {
	item, err := storyPlanToItem(rec)
	if err != nil {
		return err
	}
	_, err = h.DB.PutItem(ctx, &dynamodb.PutItemInput{TableName: aws.String(h.TableName), Item: item})
	return err
}

// storyPlanToItem converts a record to a DynamoDB item. The plan is stored as a
// JSON string, as the shorts job stores its clips. The story text has no field
// here on purpose.
func storyPlanToItem(r storyPlanRecord) (map[string]dbtypes.AttributeValue, error) {
	item := map[string]dbtypes.AttributeValue{
		"PK":         &dbtypes.AttributeValueMemberS{Value: "USER#" + r.UserID},
		"SK":         &dbtypes.AttributeValueMemberS{Value: storyPlanPrefix + r.PlanID},
		"planId":     &dbtypes.AttributeValueMemberS{Value: r.PlanID},
		"status":     &dbtypes.AttributeValueMemberS{Value: r.Status},
		"model":      &dbtypes.AttributeValueMemberS{Value: r.Model},
		"storyRunes": &dbtypes.AttributeValueMemberN{Value: strconv.Itoa(r.StoryRunes)},
		"createdAt":  &dbtypes.AttributeValueMemberS{Value: r.CreatedAt},
		"updatedAt":  &dbtypes.AttributeValueMemberS{Value: r.UpdatedAt},
	}
	if r.Script != "" {
		item["script"] = &dbtypes.AttributeValueMemberS{Value: r.Script}
	}
	if r.ErrorCode != "" {
		item["errorCode"] = &dbtypes.AttributeValueMemberS{Value: r.ErrorCode}
	}
	if r.LatencyMs > 0 {
		item["latencyMs"] = &dbtypes.AttributeValueMemberN{Value: strconv.FormatInt(r.LatencyMs, 10)}
	}
	if r.Revision > 0 {
		item["revision"] = &dbtypes.AttributeValueMemberN{Value: strconv.Itoa(r.Revision)}
	}
	if r.Plan != nil {
		b, err := json.Marshal(r.Plan)
		if err != nil {
			return nil, fmt.Errorf("encoding plan: %w", err)
		}
		item["plan"] = &dbtypes.AttributeValueMemberS{Value: string(b)}
	}
	return item, nil
}

// itemToStoryPlan converts a DynamoDB item to a record. A plan that does not
// decode is left out, so the caller sees the record without it and not an error.
func itemToStoryPlan(item map[string]dbtypes.AttributeValue) storyPlanRecord {
	str := func(k string) string {
		if v, ok := item[k].(*dbtypes.AttributeValueMemberS); ok {
			return v.Value
		}
		return ""
	}
	// Counts are parsed straight to the type they are kept in: converting a
	// 64-bit result down to int is what CodeQL flags (see PR #197).
	count := func(k string) int {
		if v, ok := item[k].(*dbtypes.AttributeValueMemberN); ok {
			if n, err := strconv.Atoi(v.Value); err == nil && n >= 0 {
				return n
			}
		}
		return 0
	}
	millis := func(k string) int64 {
		if v, ok := item[k].(*dbtypes.AttributeValueMemberN); ok {
			if n, err := strconv.ParseInt(v.Value, 10, 64); err == nil && n >= 0 {
				return n
			}
		}
		return 0
	}
	rec := storyPlanRecord{
		PlanID:     str("planId"),
		Status:     str("status"),
		Model:      str("model"),
		StoryRunes: count("storyRunes"),
		Revision:   count("revision"),
		Script:     str("script"),
		ErrorCode:  str("errorCode"),
		LatencyMs:  millis("latencyMs"),
		CreatedAt:  str("createdAt"),
		UpdatedAt:  str("updatedAt"),
	}
	if raw := str("plan"); raw != "" {
		var p storyplan.Plan
		if err := json.Unmarshal([]byte(raw), &p); err == nil {
			rec.Plan = &p
		}
	}
	return rec
}
