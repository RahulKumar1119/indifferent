package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sort"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/rahul/indifferent/backend/internal/models"
)

// The dashboard lists a user's recent work across every feature. These two
// list endpoints exist only for that: story plans and thumbnail sets had no
// list route. Like the story video list, each reads a bounded window of the
// newest records and returns at most recentWorkMax of them, with hasMore
// saying that some were left out.
const (
	recentWorkRead = 100 // records read per list
	recentWorkMax  = 20  // records returned per list
	// planHookMaxRunes caps the hook text used as a plan's title.
	planHookMaxRunes = 200
	thumbnailPrefix  = "THUMBS#"
)

// StoryPlanSummary is one row of GET /story-plans. A plan has no title of its
// own, so the hook stands in for one.
type StoryPlanSummary struct {
	PlanID       string  `json:"planId"`
	Status       string  `json:"status"`
	Hook         string  `json:"hook,omitempty"`
	SceneCount   int     `json:"sceneCount"`
	TotalSeconds float64 `json:"totalSeconds,omitempty"`
	Language     string  `json:"language,omitempty"`
	ErrorCode    string  `json:"errorCode,omitempty"`
	CreatedAt    string  `json:"createdAt"`
	UpdatedAt    string  `json:"updatedAt"`
}

// ThumbnailSetSummary is one row of GET /thumbnails.
type ThumbnailSetSummary struct {
	JobID          string `json:"jobId"`
	Status         string `json:"status"`
	Style          string `json:"style,omitempty"`
	Format         string `json:"format,omitempty"`
	ThumbnailCount int    `json:"thumbnailCount"`
	Selected       bool   `json:"selected"`
	ErrorCode      string `json:"errorCode,omitempty"`
	CreatedAt      string `json:"createdAt"`
	UpdatedAt      string `json:"updatedAt"`
}

// queryNewest reads up to recentWorkRead of the user's records whose sort key
// starts with prefix, newest key first. Attribute names are aliased because
// several (status, plan) are DynamoDB reserved words.
func (h *APIHandler) queryNewest(ctx context.Context, userID, prefix string, attrs []string) (*dynamodb.QueryOutput, error) {
	names := map[string]string{}
	proj := make([]string, 0, len(attrs))
	for i, a := range attrs {
		ph := "#a" + string(rune('a'+i))
		names[ph] = a
		proj = append(proj, ph)
	}
	return h.DB.Query(ctx, &dynamodb.QueryInput{
		TableName:                aws.String(h.TableName),
		KeyConditionExpression:   aws.String("PK = :pk AND begins_with(SK, :sk)"),
		ExpressionAttributeNames: names,
		ExpressionAttributeValues: map[string]dbtypes.AttributeValue{
			":pk": &dbtypes.AttributeValueMemberS{Value: "USER#" + userID},
			":sk": &dbtypes.AttributeValueMemberS{Value: prefix},
		},
		ProjectionExpression: aws.String(strings.Join(proj, ", ")),
		ScanIndexForward:     aws.Bool(false),
		Limit:                aws.Int32(recentWorkRead),
	})
}

// storyPlanListAttrs deliberately leaves out "script", the user's full story
// text, which the list has no use for.
var storyPlanListAttrs = []string{
	"planId", "status", "plan", "errorCode", "createdAt", "updatedAt",
}

// handleListStoryPlans answers GET /story-plans.
func (h *APIHandler) handleListStoryPlans(ctx context.Context, _ events.APIGatewayProxyRequest, claims *models.JWTClaims) (events.APIGatewayProxyResponse, error) {
	out, err := h.queryNewest(ctx, claims.UserID, storyPlanPrefix, storyPlanListAttrs)
	if err != nil {
		log.Printf("story_plan_list_failed user=%s err=%v", claims.UserID, err)
		return errorResponse(http.StatusInternalServerError, "DB_ERROR", "Could not list your story plans. Please try again."), nil
	}
	plans := make([]StoryPlanSummary, 0, recentWorkMax)
	hasMore := out.LastEvaluatedKey != nil
	for _, it := range out.Items {
		rec := itemToStoryPlan(it)
		if rec.PlanID == "" || rec.Status == "" {
			log.Printf("story_plan_list_skip user=%s", claims.UserID)
			continue
		}
		if len(plans) == recentWorkMax {
			hasMore = true
			break
		}
		s := StoryPlanSummary{
			PlanID:    rec.PlanID,
			Status:    rec.Status,
			ErrorCode: rec.ErrorCode,
			CreatedAt: rec.CreatedAt,
			UpdatedAt: rec.UpdatedAt,
		}
		if rec.Plan != nil {
			s.Hook = truncateRunes(rec.Plan.Hook, planHookMaxRunes)
			s.SceneCount = len(rec.Plan.Scenes)
			s.TotalSeconds = rec.Plan.TotalSeconds
			s.Language = rec.Plan.Language
		}
		plans = append(plans, s)
	}
	return jsonResponse(http.StatusOK, map[string]any{"plans": plans, "hasMore": hasMore}), nil
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

var thumbnailListAttrs = []string{
	"SK", "status", "style", "format", "selectedId", "thumbnails", "errorCode", "createdAt", "updatedAt",
}

// handleListThumbnails answers GET /thumbnails. Sets are keyed by job id, which
// is not time ordered, so the window is sorted by update time here.
func (h *APIHandler) handleListThumbnails(ctx context.Context, _ events.APIGatewayProxyRequest, claims *models.JWTClaims) (events.APIGatewayProxyResponse, error) {
	out, err := h.queryNewest(ctx, claims.UserID, thumbnailPrefix, thumbnailListAttrs)
	if err != nil {
		log.Printf("thumbnail_list_failed user=%s err=%v", claims.UserID, err)
		return errorResponse(http.StatusInternalServerError, "DB_ERROR", "Could not list your thumbnails. Please try again."), nil
	}
	sets := make([]ThumbnailSetSummary, 0, len(out.Items))
	for _, it := range out.Items {
		sk := stringAttr(it, "SK")
		jobID := strings.TrimPrefix(sk, thumbnailPrefix)
		status := stringAttr(it, "status")
		if jobID == "" || jobID == sk || status == "" {
			log.Printf("thumbnail_list_skip user=%s", claims.UserID)
			continue
		}
		s := ThumbnailSetSummary{
			JobID:     jobID,
			Status:    status,
			Style:     stringAttr(it, "style"),
			Format:    stringAttr(it, "format"),
			Selected:  stringAttr(it, "selectedId") != "",
			ErrorCode: stringAttr(it, "errorCode"),
			CreatedAt: stringAttr(it, "createdAt"),
			UpdatedAt: stringAttr(it, "updatedAt"),
		}
		if raw := stringAttr(it, "thumbnails"); raw != "" {
			var items []json.RawMessage
			if json.Unmarshal([]byte(raw), &items) == nil {
				s.ThumbnailCount = len(items)
			}
		}
		sets = append(sets, s)
	}
	sort.SliceStable(sets, func(i, j int) bool {
		return latest(sets[i].UpdatedAt, sets[i].CreatedAt) > latest(sets[j].UpdatedAt, sets[j].CreatedAt)
	})
	hasMore := out.LastEvaluatedKey != nil
	if len(sets) > recentWorkMax {
		sets = sets[:recentWorkMax]
		hasMore = true
	}
	return jsonResponse(http.StatusOK, map[string]any{"thumbnails": sets, "hasMore": hasMore}), nil
}

func latest(updated, created string) string {
	if updated != "" {
		return updated
	}
	return created
}
