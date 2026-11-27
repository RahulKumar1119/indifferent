// Package cliprender keeps the state of single-clip renders (#329): rendering
// one finished clip again with its saved hook, trim and caption corrections.
//
// Each clip has one small record beside its job, PK=USER#<user>,
// SK=CLIPRENDER#<job>#<clip>. It is not part of the job record on purpose: the
// edit routes write the whole job back, and a render claim stored there could be
// lost to one of those writes made in the same second.
//
// The record holds:
//   - the token of the render that may finish, so an older render can never
//     replace a newer one;
//   - the key of the file the clip plays now (ActiveKey). A render writes a new
//     object and only a successful render, still holding the current token,
//     points ActiveKey at it. A failed render leaves the old file playing;
//   - the last few request ids, so a repeated request is answered, not run again.
//
// The clip in the job record (its key, rank, score, hook text, window) is never
// changed here.
package cliprender

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/rahul/indifferent/backend/internal/models"
)

// StaleAfter is how long a render may stay "rendering" before a new request may
// take its place. The state machine gives one render at most three tries of 20
// minutes (two on Spot, one on regular Fargate), so a render older than this has
// died without anything recording it.
const StaleAfter = 75 * time.Minute

// MaxRequestIDs is how many recent request ids a record remembers.
const MaxRequestIDs = 10

// ErrConflict means the record changed since it was read, or another render is
// running.
var ErrConflict = errors.New("the clip render state changed")

// ErrSuperseded means the token is no longer the clip's current render.
var ErrSuperseded = errors.New("a newer render has replaced this one, or the clip is gone")

// DB is the part of the DynamoDB API the records need.
type DB interface {
	GetItem(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error)
	PutItem(ctx context.Context, params *dynamodb.PutItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error)
	UpdateItem(ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error)
	DeleteItem(ctx context.Context, params *dynamodb.DeleteItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error)
	Query(ctx context.Context, params *dynamodb.QueryInput, optFns ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error)
}

// Record is one clip's single-clip render state.
type Record struct {
	UserID, JobID, ClipID string
	// Token names the render that may finish. Empty when none was ever started.
	Token string
	State string
	// RequestIDs are the most recent request ids, newest last.
	RequestIDs  []string
	RequestedAt time.Time
	RenderedAt  time.Time
	Error       string
	// ActiveKey is the file the clip plays. Empty means the original render
	// (the clip's key in the job record).
	ActiveKey string
	// PreviousKey is the file ActiveKey replaced, kept until the next render so
	// a page still playing it is not cut off. It is deleted then.
	PreviousKey   string
	ExecutionName string
	Renders       int
}

// SK is the record's sort key.
func SK(jobID, clipID string) string { return "CLIPRENDER#" + jobID + "#" + clipID }

// Prefix is the sort key prefix of every clip record of a job.
func Prefix(jobID string) string { return "CLIPRENDER#" + jobID + "#" }

// Key is the record's primary key.
func Key(userID, jobID, clipID string) map[string]dbtypes.AttributeValue {
	return map[string]dbtypes.AttributeValue{
		"PK": &dbtypes.AttributeValueMemberS{Value: "USER#" + userID},
		"SK": &dbtypes.AttributeValueMemberS{Value: SK(jobID, clipID)},
	}
}

// OutputKey is where a single-clip render writes its file. It sits under the
// job's own prefix, so deleting the job removes it.
func OutputKey(userID, jobID, clipID, token string) string {
	return fmt.Sprintf("shorts/%s/%s/clips/%s.r-%s.mp4", userID, jobID, clipID, token)
}

// IsOutputKey reports whether key is a single-clip render file of this clip.
func IsOutputKey(key, userID, jobID, clipID string) bool {
	prefix := fmt.Sprintf("shorts/%s/%s/clips/%s.r-", userID, jobID, clipID)
	return strings.HasPrefix(key, prefix) && strings.HasSuffix(key, ".mp4")
}

// NewToken returns a random render token.
func NewToken() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

// ExecutionName is the Step Functions execution name of a render.
func ExecutionName(token string) string { return "clip-" + token }

// ValidToken reports whether s looks like a token from NewToken.
func ValidToken(s string) bool {
	if len(s) != 32 {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil
}

// Running reports whether the record holds a render that may still finish.
func (r *Record) Running(now time.Time) bool {
	return r != nil && r.State == models.ClipRenderRendering && now.Sub(r.RequestedAt) < StaleAfter
}

// HasRequest reports whether id is one of the record's recent request ids.
func (r *Record) HasRequest(id string) bool {
	if r == nil {
		return false
	}
	for _, x := range r.RequestIDs {
		if x == id {
			return true
		}
	}
	return false
}

// View is what the page is shown.
func (r *Record) View(now time.Time) models.ClipRenderView {
	v := models.ClipRenderView{State: r.State, Error: r.Error, Renders: r.Renders}
	if r.State == models.ClipRenderRendering && !r.Running(now) {
		v.State = models.ClipRenderFailed
		v.Error = "This render stopped without finishing. Try again."
	}
	if !r.RequestedAt.IsZero() {
		v.RequestedAt = r.RequestedAt.UTC().Format(time.RFC3339)
	}
	if !r.RenderedAt.IsZero() {
		v.RenderedAt = r.RenderedAt.UTC().Format(time.RFC3339)
	}
	return v
}

// timeLayout has a fixed width, so stored times compare correctly as strings
// in a condition.
const timeLayout = "2006-01-02T15:04:05.000000000Z"

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(timeLayout)
}

func parseTime(s string) time.Time {
	t, _ := time.Parse(timeLayout, s)
	return t
}

// toItem writes the record as a DynamoDB item.
func (r Record) toItem() map[string]dbtypes.AttributeValue {
	item := Key(r.UserID, r.JobID, r.ClipID)
	set := func(k, v string) {
		if v != "" {
			item[k] = &dbtypes.AttributeValueMemberS{Value: v}
		}
	}
	set("type", "CLIPRENDER")
	set("jobId", r.JobID)
	set("clipId", r.ClipID)
	set("token", r.Token)
	set("state", r.State)
	set("requestedAt", formatTime(r.RequestedAt))
	set("renderedAt", formatTime(r.RenderedAt))
	set("error", r.Error)
	set("activeKey", r.ActiveKey)
	set("previousKey", r.PreviousKey)
	set("executionName", r.ExecutionName)
	item["renders"] = &dbtypes.AttributeValueMemberN{Value: strconv.Itoa(r.Renders)}
	ids := make([]dbtypes.AttributeValue, 0, len(r.RequestIDs))
	for _, id := range r.RequestIDs {
		ids = append(ids, &dbtypes.AttributeValueMemberS{Value: id})
	}
	item["requestIds"] = &dbtypes.AttributeValueMemberL{Value: ids}
	return item
}

// FromItem reads a stored record.
func FromItem(item map[string]dbtypes.AttributeValue) Record {
	s := func(k string) string {
		v, _ := item[k].(*dbtypes.AttributeValueMemberS)
		if v == nil {
			return ""
		}
		return v.Value
	}
	r := Record{
		JobID: s("jobId"), ClipID: s("clipId"), Token: s("token"), State: s("state"),
		RequestedAt: parseTime(s("requestedAt")), RenderedAt: parseTime(s("renderedAt")),
		Error: s("error"), ActiveKey: s("activeKey"), PreviousKey: s("previousKey"), ExecutionName: s("executionName"),
	}
	if pk := s("PK"); strings.HasPrefix(pk, "USER#") {
		r.UserID = strings.TrimPrefix(pk, "USER#")
	}
	if v, ok := item["renders"].(*dbtypes.AttributeValueMemberN); ok {
		r.Renders, _ = strconv.Atoi(v.Value)
	}
	if v, ok := item["requestIds"].(*dbtypes.AttributeValueMemberL); ok {
		for _, x := range v.Value {
			if sv, ok := x.(*dbtypes.AttributeValueMemberS); ok {
				r.RequestIDs = append(r.RequestIDs, sv.Value)
			}
		}
	}
	return r
}

// Get reads one clip's record with a consistent read. A clip that was never
// rendered on its own has none: (nil, nil).
func Get(ctx context.Context, db DB, table, userID, jobID, clipID string) (*Record, error) {
	out, err := db.GetItem(ctx, &dynamodb.GetItemInput{
		TableName:      aws.String(table),
		Key:            Key(userID, jobID, clipID),
		ConsistentRead: aws.Bool(true),
	})
	if err != nil {
		return nil, err
	}
	if len(out.Item) == 0 {
		return nil, nil
	}
	r := FromItem(out.Item)
	return &r, nil
}

// List reads every clip record of a job, by clip id.
func List(ctx context.Context, db DB, table, userID, jobID string) (map[string]Record, error) {
	out := map[string]Record{}
	var start map[string]dbtypes.AttributeValue
	for {
		res, err := db.Query(ctx, &dynamodb.QueryInput{
			TableName:              aws.String(table),
			KeyConditionExpression: aws.String("PK = :pk AND begins_with(SK, :sk)"),
			ExpressionAttributeValues: map[string]dbtypes.AttributeValue{
				":pk": &dbtypes.AttributeValueMemberS{Value: "USER#" + userID},
				":sk": &dbtypes.AttributeValueMemberS{Value: Prefix(jobID)},
			},
			ConsistentRead:    aws.Bool(true),
			ExclusiveStartKey: start,
		})
		if err != nil {
			return nil, err
		}
		for _, it := range res.Items {
			r := FromItem(it)
			if r.ClipID != "" {
				out[r.ClipID] = r
			}
		}
		if len(res.LastEvaluatedKey) == 0 {
			return out, nil
		}
		start = res.LastEvaluatedKey
	}
}

// Claim writes next in place of prev, the record this request read (nil when
// there was none). It succeeds only when the stored record is still prev and no
// other render is running, so of two requests racing for one clip exactly one
// starts a render. Anything else is ErrConflict.
func Claim(ctx context.Context, db DB, table string, prev *Record, next Record, now time.Time) error {
	in := &dynamodb.PutItemInput{TableName: aws.String(table), Item: next.toItem()}
	if prev == nil {
		in.ConditionExpression = aws.String("attribute_not_exists(PK)")
	} else {
		// The token and the count of finished renders together say the record is
		// the one read: a render that finished in between changes the count.
		in.ConditionExpression = aws.String("#token = :readToken AND #renders = :readRenders AND (#state <> :rendering OR requestedAt < :stale)")
		in.ExpressionAttributeNames = map[string]string{"#token": "token", "#state": "state", "#renders": "renders"}
		in.ExpressionAttributeValues = map[string]dbtypes.AttributeValue{
			":readToken":   &dbtypes.AttributeValueMemberS{Value: prev.Token},
			":readRenders": &dbtypes.AttributeValueMemberN{Value: strconv.Itoa(prev.Renders)},
			":rendering":   &dbtypes.AttributeValueMemberS{Value: models.ClipRenderRendering},
			":stale":       &dbtypes.AttributeValueMemberS{Value: formatTime(now.Add(-StaleAfter))},
		}
	}
	_, err := db.PutItem(ctx, in)
	var ccf *dbtypes.ConditionalCheckFailedException
	if errors.As(err, &ccf) {
		return ErrConflict
	}
	return err
}

// Restore puts prev back after a claim with token whose render could not be
// started (quota, Step Functions). With prev nil the claimed record is removed.
// It only touches a record still holding token.
func Restore(ctx context.Context, db DB, table string, prev *Record, claimed Record) error {
	cond := aws.String("#token = :token")
	names := map[string]string{"#token": "token"}
	values := map[string]dbtypes.AttributeValue{":token": &dbtypes.AttributeValueMemberS{Value: claimed.Token}}
	var err error
	if prev == nil {
		_, err = db.DeleteItem(ctx, &dynamodb.DeleteItemInput{
			TableName: aws.String(table), Key: Key(claimed.UserID, claimed.JobID, claimed.ClipID),
			ConditionExpression: cond, ExpressionAttributeNames: names, ExpressionAttributeValues: values,
		})
	} else {
		_, err = db.PutItem(ctx, &dynamodb.PutItemInput{
			TableName: aws.String(table), Item: prev.toItem(),
			ConditionExpression: cond, ExpressionAttributeNames: names, ExpressionAttributeValues: values,
		})
	}
	var ccf *dbtypes.ConditionalCheckFailedException
	if errors.As(err, &ccf) {
		return ErrSuperseded
	}
	return err
}

// Promote makes newKey the clip's file, but only while token is still the
// clip's current render and that render is still running. The file it replaces
// becomes PreviousKey (the next claim deletes it). ErrSuperseded means a newer
// render or a delete got there first, and newKey must not be used.
func Promote(ctx context.Context, db DB, table, userID, jobID, clipID, token, newKey string, now time.Time) error {
	update := "SET #state = :done, renderedAt = :now, activeKey = :key ADD renders :one REMOVE #error, lastError"
	values := map[string]dbtypes.AttributeValue{
		":done":      &dbtypes.AttributeValueMemberS{Value: models.ClipRenderDone},
		":rendering": &dbtypes.AttributeValueMemberS{Value: models.ClipRenderRendering},
		":now":       &dbtypes.AttributeValueMemberS{Value: formatTime(now)},
		":key":       &dbtypes.AttributeValueMemberS{Value: newKey},
		":token":     &dbtypes.AttributeValueMemberS{Value: token},
		":one":       &dbtypes.AttributeValueMemberN{Value: "1"},
	}
	// Every expression reads the record as it was, so previousKey takes the
	// activeKey being replaced. A clip whose first render this is has none.
	rec, err := Get(ctx, db, table, userID, jobID, clipID)
	if err != nil {
		return err
	}
	if rec == nil || rec.Token != token {
		return ErrSuperseded
	}
	if rec.ActiveKey != "" {
		update = "SET #state = :done, renderedAt = :now, previousKey = activeKey, activeKey = :key ADD renders :one REMOVE #error, lastError"
	}
	_, err = db.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:                 aws.String(table),
		Key:                       Key(userID, jobID, clipID),
		UpdateExpression:          aws.String(update),
		ConditionExpression:       aws.String("#token = :token AND #state = :rendering"),
		ExpressionAttributeNames:  map[string]string{"#state": "state", "#token": "token", "#error": "error"},
		ExpressionAttributeValues: values,
	})
	var ccf *dbtypes.ConditionalCheckFailedException
	if errors.As(err, &ccf) {
		return ErrSuperseded
	}
	return err
}

// MaxErrorRunes bounds the reason stored for a failed render.
const MaxErrorRunes = 300

// RecordError keeps why the render with token failed, without ending it: the
// state machine may still try again on other capacity. When it gives up it
// marks the record failed and shows this reason (or a general one when there is
// none). Only the current render's record is touched, and the clip's file is
// left as it was.
func RecordError(ctx context.Context, db DB, table, userID, jobID, clipID, token, reason string) error {
	if r := []rune(reason); len(r) > MaxErrorRunes {
		reason = string(r[:MaxErrorRunes])
	}
	_, err := db.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:           aws.String(table),
		Key:                 Key(userID, jobID, clipID),
		UpdateExpression:    aws.String("SET lastError = :reason"),
		ConditionExpression: aws.String("#token = :token AND #state = :rendering"),
		ExpressionAttributeNames: map[string]string{
			"#state": "state", "#token": "token",
		},
		ExpressionAttributeValues: map[string]dbtypes.AttributeValue{
			":rendering": &dbtypes.AttributeValueMemberS{Value: models.ClipRenderRendering},
			":reason":    &dbtypes.AttributeValueMemberS{Value: reason},
			":token":     &dbtypes.AttributeValueMemberS{Value: token},
		},
	})
	var ccf *dbtypes.ConditionalCheckFailedException
	if errors.As(err, &ccf) {
		return ErrSuperseded
	}
	return err
}

// DeleteAll removes every clip record of a job and returns the execution names
// of renders that were still running, so the caller can stop them.
func DeleteAll(ctx context.Context, db DB, table, userID, jobID string, now time.Time) ([]string, error) {
	recs, err := List(ctx, db, table, userID, jobID)
	if err != nil {
		return nil, err
	}
	var running []string
	for _, r := range recs {
		if r.Running(now) && r.ExecutionName != "" {
			running = append(running, r.ExecutionName)
		}
		if _, err := db.DeleteItem(ctx, &dynamodb.DeleteItemInput{TableName: aws.String(table), Key: Key(userID, jobID, r.ClipID)}); err != nil {
			return running, fmt.Errorf("delete clip render record: %w", err)
		}
	}
	return running, nil
}
