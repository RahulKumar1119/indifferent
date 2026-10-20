package thumbnail

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	db "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// DB is the slice of DynamoDB the store uses. Both the API handler's client
// and the worker's satisfy it.
type DB interface {
	GetItem(ctx context.Context, in *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error)
	UpdateItem(ctx context.Context, in *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error)
	DeleteItem(ctx context.Context, in *dynamodb.DeleteItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error)
}

// StaleAfter is how long a queued or running record may go without an update
// before it is treated as abandoned (a crashed task) and may be restarted.
// A normal run takes a minute or two, and the state machine gives up at 15.
const StaleAfter = 20 * time.Minute

var (
	// ErrInFlight means a generation is already queued or running.
	ErrInFlight = errors.New("thumbnail generation already in progress")
	// ErrGone means the record was deleted (usually with its job) while work
	// was running.
	ErrGone = errors.New("thumbnail record no longer exists")
	// ErrNotReady means thumbnails cannot be selected because none are ready.
	ErrNotReady = errors.New("thumbnails are not ready")
)

// Store reads and writes the thumbnail record for a Shorts job. Each job has
// one record, kept apart from the job itself so the pipeline's updates to the
// job and this feature's updates never overwrite each other.
type Store struct {
	DB    DB
	Table string
	Now   func() time.Time
}

func (s *Store) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s *Store) stamp() string { return s.now().UTC().Format(time.RFC3339) }

// Key is the DynamoDB key for a job's thumbnail record.
func Key(userID, jobID string) map[string]db.AttributeValue {
	return map[string]db.AttributeValue{
		"PK": &db.AttributeValueMemberS{Value: "USER#" + userID},
		"SK": &db.AttributeValueMemberS{Value: "THUMBS#" + jobID},
	}
}

func str(v string) db.AttributeValue { return &db.AttributeValueMemberS{Value: v} }

func isConditionFailed(err error) bool {
	var ccf *db.ConditionalCheckFailedException
	return errors.As(err, &ccf)
}

// DynamoDB reserves many ordinary words ("status", "style", "format" and
// more), and rejects an expression that uses one unaliased. Rather than guess
// which words are reserved, every attribute in an expression is written as
// "#name" and names() builds the matching ExpressionAttributeNames. DynamoDB
// also rejects aliases that no expression uses, so names() only lists the ones
// that appear.
var aliasPattern = regexp.MustCompile(`#[A-Za-z][A-Za-z0-9]*`)

func names(exprs ...string) map[string]string {
	out := map[string]string{}
	for _, e := range exprs {
		for _, a := range aliasPattern.FindAllString(e, -1) {
			out[a] = a[1:]
		}
	}
	return out
}

// Get loads a job's record, or nil when none exists yet.
func (s *Store) Get(ctx context.Context, userID, jobID string) (*Set, error) {
	out, err := s.DB.GetItem(ctx, &dynamodb.GetItemInput{TableName: aws.String(s.Table), Key: Key(userID, jobID)})
	if err != nil {
		return nil, fmt.Errorf("get thumbnails: %w", err)
	}
	if out.Item == nil {
		return nil, nil
	}
	set := decodeSet(out.Item)
	set.UserID, set.JobID = userID, jobID
	return set, nil
}

// Queue marks a generation as requested, before the worker has started, so the
// UI can show progress straight away. It fails with ErrInFlight if a fresh
// run is already queued or running. style and format may be empty (automatic).
func (s *Store) Queue(ctx context.Context, userID, jobID string, style Style, format Format) error {
	sets := []string{"#status = :queued", "#updatedAt = :now", "#createdAt = if_not_exists(#createdAt, :now)"}
	removes := []string{"#stage", "#errorCode", "#error"}
	vals := map[string]db.AttributeValue{
		":queued":    str(string(StatusQueued)),
		":now":       str(s.stamp()),
		":completed": str(string(StatusCompleted)),
		":failed":    str(string(StatusFailed)),
		":stale":     str(s.now().Add(-StaleAfter).UTC().Format(time.RFC3339)),
	}
	// The requested style and format are kept on the record while the run is
	// in progress; the worker overwrites them with what it actually used.
	if style != "" {
		sets, vals[":style"] = append(sets, "#reqStyle = :style"), str(string(style))
	} else {
		removes = append(removes, "#reqStyle")
	}
	if format != "" {
		sets, vals[":format"] = append(sets, "#reqFormat = :format"), str(string(format))
	} else {
		removes = append(removes, "#reqFormat")
	}
	update := "SET " + strings.Join(sets, ", ") + " REMOVE " + strings.Join(removes, ", ")
	cond := "attribute_not_exists(PK) OR attribute_not_exists(#status) OR #status IN (:completed, :failed) OR #updatedAt < :stale"
	_, err := s.DB.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:                 aws.String(s.Table),
		Key:                       Key(userID, jobID),
		UpdateExpression:          aws.String(update),
		ConditionExpression:       aws.String(cond),
		ExpressionAttributeNames:  names(update, cond),
		ExpressionAttributeValues: vals,
	})
	if isConditionFailed(err) {
		return ErrInFlight
	}
	if err != nil {
		return fmt.Errorf("queue thumbnails: %w", err)
	}
	return nil
}

// Begin is called by the worker as it starts. It claims the record, bumps the
// generation (which names the output folder) and returns the new generation
// number along with the style and format the user asked for.
func (s *Store) Begin(ctx context.Context, userID, jobID string) (generation int, style Style, format Format, err error) {
	update := "SET #status = :running, #stage = :stage, #updatedAt = :now, #createdAt = if_not_exists(#createdAt, :now) " +
		"REMOVE #errorCode, #error ADD #generation :one"
	cond := "attribute_not_exists(PK) OR attribute_not_exists(#status) OR #status IN (:queued, :completed, :failed) OR #updatedAt < :stale"
	out, err := s.DB.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:                aws.String(s.Table),
		Key:                      Key(userID, jobID),
		UpdateExpression:         aws.String(update),
		ConditionExpression:      aws.String(cond),
		ExpressionAttributeNames: names(update, cond),
		ExpressionAttributeValues: map[string]db.AttributeValue{
			":running":   str(string(StatusRunning)),
			":queued":    str(string(StatusQueued)),
			":completed": str(string(StatusCompleted)),
			":failed":    str(string(StatusFailed)),
			":stage":     str(string(StageExtracting)),
			":now":       str(s.stamp()),
			":stale":     str(s.now().Add(-StaleAfter).UTC().Format(time.RFC3339)),
			":one":       &db.AttributeValueMemberN{Value: "1"},
		},
		ReturnValues: db.ReturnValueAllNew,
	})
	if isConditionFailed(err) {
		return 0, "", "", ErrInFlight
	}
	if err != nil {
		return 0, "", "", fmt.Errorf("begin thumbnails: %w", err)
	}
	set := decodeSet(out.Attributes)
	st, _ := ParseStyle(attrS(out.Attributes, "reqStyle"))
	f, _ := ParseFormat(attrS(out.Attributes, "reqFormat"))
	return set.Generation, st, f, nil
}

// SetStage records which step a running generation is on.
func (s *Store) SetStage(ctx context.Context, userID, jobID string, generation int, stage Stage) error {
	return s.guardedUpdate(ctx, userID, jobID, generation, "SET #stage = :stage, #updatedAt = :now",
		map[string]db.AttributeValue{":stage": str(string(stage))})
}

// Result is what the worker saves on success.
type Result struct {
	Thumbnails     []Asset
	Analysis       Analysis
	Funnel         Funnel
	Style          Style
	Format         Format
	TranscriptUsed bool
	DetectionUsed  bool
}

// Complete saves the finished thumbnails and clears any previous selection.
func (s *Store) Complete(ctx context.Context, userID, jobID string, generation int, r Result) error {
	thumbs, _ := json.Marshal(r.Thumbnails)
	analysis, _ := json.Marshal(r.Analysis)
	funnel, _ := json.Marshal(r.Funnel)
	return s.guardedUpdate(ctx, userID, jobID, generation,
		"SET #status = :completed, #thumbnails = :t, #analysis = :a, #funnel = :f, #style = :style, #format = :format, "+
			"#transcriptUsed = :tu, #detectionUsed = :du, #updatedAt = :now "+
			"REMOVE #stage, #errorCode, #error, #selectedId, #reqStyle, #reqFormat",
		map[string]db.AttributeValue{
			":completed": str(string(StatusCompleted)),
			":t":         str(string(thumbs)),
			":a":         str(string(analysis)),
			":f":         str(string(funnel)),
			":style":     str(string(r.Style)),
			":format":    str(string(r.Format)),
			":tu":        &db.AttributeValueMemberBOOL{Value: r.TranscriptUsed},
			":du":        &db.AttributeValueMemberBOOL{Value: r.DetectionUsed},
		})
}

// Fail records a failure. Earlier thumbnails stay on the record so a failed
// regenerate does not take away what the user already had.
func (s *Store) Fail(ctx context.Context, userID, jobID string, generation int, code, message string) error {
	return s.guardedUpdate(ctx, userID, jobID, generation,
		"SET #status = :failed, #errorCode = :code, #error = :msg, #updatedAt = :now REMOVE #stage",
		map[string]db.AttributeValue{
			":failed": str(string(StatusFailed)),
			":code":   str(code),
			":msg":    str(message),
		})
}

// guardedUpdate applies an update only while the record still exists and still
// belongs to this generation, so a worker whose job was deleted, or that was
// superseded by a newer run, cannot write stale results.
func (s *Store) guardedUpdate(ctx context.Context, userID, jobID string, generation int, expr string, vals map[string]db.AttributeValue) error {
	vals[":now"] = str(s.stamp())
	vals[":gen"] = &db.AttributeValueMemberN{Value: strconv.Itoa(generation)}
	cond := "attribute_exists(PK) AND #generation = :gen"
	in := &dynamodb.UpdateItemInput{
		TableName:                 aws.String(s.Table),
		Key:                       Key(userID, jobID),
		UpdateExpression:          aws.String(expr),
		ConditionExpression:       aws.String(cond),
		ExpressionAttributeNames:  names(expr, cond),
		ExpressionAttributeValues: vals,
	}
	_, err := s.DB.UpdateItem(ctx, in)
	if isConditionFailed(err) {
		return ErrGone
	}
	if err != nil {
		return fmt.Errorf("update thumbnails: %w", err)
	}
	return nil
}

// AbortQueued marks a record that was queued but never started (for example
// because the state machine could not be launched) as failed. It leaves a
// record that a worker has since claimed alone.
func (s *Store) AbortQueued(ctx context.Context, userID, jobID, code, message string) error {
	update := "SET #status = :failed, #errorCode = :code, #error = :msg, #updatedAt = :now REMOVE #stage"
	cond := "attribute_exists(PK) AND #status = :queued"
	_, err := s.DB.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:                 aws.String(s.Table),
		Key:                       Key(userID, jobID),
		UpdateExpression:          aws.String(update),
		ConditionExpression:       aws.String(cond),
		ExpressionAttributeNames:  names(update, cond),
		ExpressionAttributeValues: map[string]db.AttributeValue{":failed": str(string(StatusFailed)), ":queued": str(string(StatusQueued)), ":code": str(code), ":msg": str(message), ":now": str(s.stamp())},
	})
	if isConditionFailed(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("abort queued thumbnails: %w", err)
	}
	return nil
}

// Select marks one thumbnail as the video's chosen thumbnail. It only works on
// a completed set that contains the thumbnail.
func (s *Store) Select(ctx context.Context, userID, jobID, id string) error {
	update := "SET #selectedId = :id, #updatedAt = :now"
	cond := "attribute_exists(PK) AND #status = :completed"
	_, err := s.DB.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:                 aws.String(s.Table),
		Key:                       Key(userID, jobID),
		UpdateExpression:          aws.String(update),
		ConditionExpression:       aws.String(cond),
		ExpressionAttributeNames:  names(update, cond),
		ExpressionAttributeValues: map[string]db.AttributeValue{":id": str(id), ":now": str(s.stamp()), ":completed": str(string(StatusCompleted))},
	})
	if isConditionFailed(err) {
		return ErrNotReady
	}
	if err != nil {
		return fmt.Errorf("select thumbnail: %w", err)
	}
	return nil
}

// Delete removes the record. Deleting one that does not exist is fine.
func (s *Store) Delete(ctx context.Context, userID, jobID string) error {
	_, err := s.DB.DeleteItem(ctx, &dynamodb.DeleteItemInput{TableName: aws.String(s.Table), Key: Key(userID, jobID)})
	if err != nil {
		return fmt.Errorf("delete thumbnails: %w", err)
	}
	return nil
}

func attrS(item map[string]db.AttributeValue, name string) string {
	if v, ok := item[name].(*db.AttributeValueMemberS); ok {
		return v.Value
	}
	return ""
}

func attrBool(item map[string]db.AttributeValue, name string) bool {
	if v, ok := item[name].(*db.AttributeValueMemberBOOL); ok {
		return v.Value
	}
	return false
}

func attrInt(item map[string]db.AttributeValue, name string) int {
	if v, ok := item[name].(*db.AttributeValueMemberN); ok {
		n, _ := strconv.Atoi(v.Value)
		return n
	}
	return 0
}

// decodeSet turns a stored item into a Set. Unreadable JSON columns are left
// empty rather than failing the whole read.
func decodeSet(item map[string]db.AttributeValue) *Set {
	set := &Set{
		Status:         Status(attrS(item, "status")),
		Stage:          Stage(attrS(item, "stage")),
		ErrorCode:      attrS(item, "errorCode"),
		Error:          attrS(item, "error"),
		Style:          Style(attrS(item, "style")),
		Format:         Format(attrS(item, "format")),
		Generation:     attrInt(item, "generation"),
		SelectedID:     attrS(item, "selectedId"),
		TranscriptUsed: attrBool(item, "transcriptUsed"),
		DetectionUsed:  attrBool(item, "detectionUsed"),
		CreatedAt:      attrS(item, "createdAt"),
		UpdatedAt:      attrS(item, "updatedAt"),
	}
	if raw := attrS(item, "thumbnails"); raw != "" {
		_ = json.Unmarshal([]byte(raw), &set.Thumbnails)
	}
	if raw := attrS(item, "analysis"); raw != "" {
		var a Analysis
		if json.Unmarshal([]byte(raw), &a) == nil {
			set.Analysis = &a
		}
	}
	if raw := attrS(item, "funnel"); raw != "" {
		var f Funnel
		if json.Unmarshal([]byte(raw), &f) == nil {
			set.Funnel = &f
		}
	}
	return set
}
