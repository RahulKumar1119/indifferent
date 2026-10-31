package storyjob

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/rahul/indifferent/backend/internal/models"
)

// Key prefixes. A job is stored in the projects table beside the user's other
// records, as the Shorts jobs and story plans are.
const (
	partitionPrefix = "USER#"
	sortPrefix      = "STORYVIDEO#"
)

// Key returns the DynamoDB key of a job.
func Key(userID, jobID string) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		"PK": &types.AttributeValueMemberS{Value: partitionPrefix + userID},
		"SK": &types.AttributeValueMemberS{Value: sortPrefix + jobID},
	}
}

// SortKeyRange returns the sort-key bounds of a user's story videos, for a query.
func SortKeyRange() (lo, hi string) { return sortPrefix, sortPrefix + "\uffff" }

// ToItem converts a job to a DynamoDB item. Scenes and the caption style are
// stored as JSON strings, like the Shorts job's clips and caption.
func ToItem(j Job) (map[string]types.AttributeValue, error) {
	scenes, err := json.Marshal(j.Scenes)
	if err != nil {
		return nil, fmt.Errorf("scenes: %w", err)
	}
	caption, err := json.Marshal(j.Caption)
	if err != nil {
		return nil, fmt.Errorf("caption: %w", err)
	}
	item := Key(j.UserID, j.JobID)
	set := func(name, v string) {
		if v != "" {
			item[name] = &types.AttributeValueMemberS{Value: v}
		}
	}
	item["type"] = &types.AttributeValueMemberS{Value: "storyvideo"}
	item["jobId"] = &types.AttributeValueMemberS{Value: j.JobID}
	item["userId"] = &types.AttributeValueMemberS{Value: j.UserID}
	set("planId", j.PlanID)
	item["planRevision"] = &types.AttributeValueMemberN{Value: strconv.Itoa(j.PlanRevision)}
	item["status"] = &types.AttributeValueMemberS{Value: j.Status}
	item["progress"] = &types.AttributeValueMemberN{Value: strconv.Itoa(j.Progress)}
	set("language", j.Language)
	set("voice", j.Voice)
	item["caption"] = &types.AttributeValueMemberS{Value: string(caption)}
	item["scenes"] = &types.AttributeValueMemberS{Value: string(scenes)}
	if j.TotalSeconds > 0 {
		item["totalSeconds"] = &types.AttributeValueMemberN{Value: strconv.FormatFloat(j.TotalSeconds, 'f', -1, 64)}
	}
	set("outputKey", j.OutputKey)
	set("error", j.Error)
	set("errorCode", j.ErrorCode)
	set("failedStage", j.FailedStage)
	set("createdAt", j.CreatedAt)
	set("updatedAt", j.UpdatedAt)
	set("completedAt", j.CompletedAt)
	return item, nil
}

// FromItem reads a job back. A record that cannot be read is an error and is
// never guessed at: a render built from half a job would be a wrong video.
func FromItem(item map[string]types.AttributeValue) (Job, error) {
	str := func(name string) string {
		if v, ok := item[name].(*types.AttributeValueMemberS); ok {
			return v.Value
		}
		return ""
	}
	num := func(name string) (float64, error) {
		v, ok := item[name].(*types.AttributeValueMemberN)
		if !ok {
			return 0, nil
		}
		return strconv.ParseFloat(v.Value, 64)
	}
	sk := str("SK")
	if !strings.HasPrefix(sk, sortPrefix) {
		return Job{}, fmt.Errorf("not a story video record: %q", sk)
	}
	j := Job{
		UserID: strings.TrimPrefix(str("PK"), partitionPrefix), JobID: strings.TrimPrefix(sk, sortPrefix),
		PlanID: str("planId"), Status: str("status"), Language: str("language"), Voice: str("voice"),
		OutputKey: str("outputKey"), Error: str("error"), ErrorCode: str("errorCode"), FailedStage: str("failedStage"),
		CreatedAt: str("createdAt"), UpdatedAt: str("updatedAt"), CompletedAt: str("completedAt"),
	}
	for name, dst := range map[string]*int{"planRevision": &j.PlanRevision, "progress": &j.Progress} {
		f, err := num(name)
		if err != nil {
			return Job{}, fmt.Errorf("%s: %w", name, err)
		}
		*dst = int(f)
	}
	var err error
	if j.TotalSeconds, err = num("totalSeconds"); err != nil {
		return Job{}, fmt.Errorf("totalSeconds: %w", err)
	}
	if raw := str("scenes"); raw != "" {
		if err := json.Unmarshal([]byte(raw), &j.Scenes); err != nil {
			return Job{}, fmt.Errorf("scenes are unreadable: %w", err)
		}
	}
	if raw := str("caption"); raw != "" {
		var c models.CaptionStyle
		if err := json.Unmarshal([]byte(raw), &c); err != nil {
			return Job{}, fmt.Errorf("caption is unreadable: %w", err)
		}
		j.Caption = c
	}
	return j, nil
}
