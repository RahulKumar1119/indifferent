package storyjob

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// DynamoAPI is the part of the DynamoDB client the store uses.
type DynamoAPI interface {
	GetItem(ctx context.Context, in *dynamodb.GetItemInput, opts ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error)
	UpdateItem(ctx context.Context, in *dynamodb.UpdateItemInput, opts ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error)
}

// DynamoStore is the Store backed by the projects table.
type DynamoStore struct {
	DB    DynamoAPI
	Table string
}

var _ Store = (*DynamoStore)(nil)

// Load reads the job with a consistent read, so a run started right after a
// save sees what was saved.
func (s *DynamoStore) Load(ctx context.Context, userID, jobID string) (*Job, error) {
	out, err := s.DB.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(s.Table), Key: Key(userID, jobID), ConsistentRead: aws.Bool(true),
	})
	if err != nil {
		return nil, err
	}
	if len(out.Item) == 0 {
		return nil, ErrJobGone
	}
	j, err := FromItem(out.Item)
	if err != nil {
		return nil, err
	}
	return &j, nil
}

// Status reads only the status.
func (s *DynamoStore) Status(ctx context.Context, userID, jobID string) (string, error) {
	out, err := s.DB.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(s.Table), Key: Key(userID, jobID), ConsistentRead: aws.Bool(true),
		ProjectionExpression: aws.String("#s"), ExpressionAttributeNames: map[string]string{"#s": "status"},
	})
	if err != nil {
		return "", err
	}
	if len(out.Item) == 0 {
		return "", ErrJobGone
	}
	if v, ok := out.Item["status"].(*types.AttributeValueMemberS); ok {
		return v.Value, nil
	}
	return "", nil
}

// Save writes the job's progress. It refuses to touch a job that is gone or
// already ended, so a slow worker can never undo a cancel or bring a deleted
// job back.
func (s *DynamoStore) Save(ctx context.Context, j *Job) error {
	return s.write(ctx, j, false)
}

// Finish records the end of the job: the output, or the failure.
func (s *DynamoStore) Finish(ctx context.Context, j *Job) error {
	return s.write(ctx, j, true)
}

func (s *DynamoStore) write(ctx context.Context, j *Job, final bool) error {
	scenes, err := json.Marshal(j.Scenes)
	if err != nil {
		return fmt.Errorf("scenes: %w", err)
	}
	set := []string{"#status = :status", "#progress = :progress", "#scenes = :scenes", "#updatedAt = :updatedAt"}
	names := map[string]string{"#status": "status", "#progress": "progress", "#scenes": "scenes", "#updatedAt": "updatedAt"}
	vals := map[string]types.AttributeValue{
		":status": str(j.Status), ":progress": num(strconv.Itoa(j.Progress)), ":scenes": str(string(scenes)), ":updatedAt": str(j.UpdatedAt),
		":completed": str(StatusCompleted), ":failed": str(StatusFailed),
	}
	var remove []string
	field := func(attr, expr, value string) {
		names["#"+attr] = attr
		if value == "" {
			remove = append(remove, "#"+attr)
			return
		}
		set = append(set, fmt.Sprintf("#%s = :%s", attr, expr))
		vals[":"+expr] = str(value)
	}
	field("error", "error", j.Error)
	field("errorCode", "errorCode", j.ErrorCode)
	field("failedStage", "failedStage", j.FailedStage)
	if j.TotalSeconds > 0 {
		names["#totalSeconds"] = "totalSeconds"
		set = append(set, "#totalSeconds = :totalSeconds")
		vals[":totalSeconds"] = num(strconv.FormatFloat(j.TotalSeconds, 'f', -1, 64))
	}
	if final {
		field("outputKey", "outputKey", j.OutputKey)
		field("completedAt", "completedAt", j.CompletedAt)
	}
	expr := "SET " + join(set, ", ")
	if len(remove) > 0 {
		expr += " REMOVE " + join(remove, ", ")
	}

	_, err = s.DB.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(s.Table), Key: Key(j.UserID, j.JobID),
		UpdateExpression: aws.String(expr), ExpressionAttributeNames: names, ExpressionAttributeValues: vals,
		// Alive, and not ended by someone else. A job the worker itself is
		// finishing is still "running" at this point.
		ConditionExpression: aws.String("attribute_exists(PK) AND #status <> :completed AND #status <> :failed"),
	})
	if err == nil {
		return nil
	}
	var cond *types.ConditionalCheckFailedException
	if !errors.As(err, &cond) {
		return err
	}
	// Tell a deleted job from one that ended elsewhere.
	status, serr := s.Status(ctx, j.UserID, j.JobID)
	switch {
	case errors.Is(serr, ErrJobGone):
		return ErrJobGone
	case serr != nil:
		return serr
	case Terminal(status):
		return fmt.Errorf("%w: it is %s", errStoppedElsewhere, status)
	}
	return err
}

func str(v string) types.AttributeValue { return &types.AttributeValueMemberS{Value: v} }
func num(v string) types.AttributeValue { return &types.AttributeValueMemberN{Value: v} }

func join(parts []string, sep string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += sep
		}
		out += p
	}
	return out
}
