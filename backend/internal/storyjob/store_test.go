package storyjob

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// fakeDynamo is a DynamoDB table in memory. It does what the real service does
// that this code could get wrong, because the real one cannot be reached from a
// test:
//
//   - an UpdateItem that lists an expression name or value it does not use, or
//     uses one it does not list, fails with a ValidationException;
//   - a raw attribute name that is a reserved word fails the same way;
//   - the condition is evaluated against the item as it was before the update;
//   - an update to a missing item creates it, unless the condition forbids that.
//
// It understands only the expressions this package writes and panics on any
// other, so a change to an expression has to be looked at here.
type fakeDynamo struct {
	mu      sync.Mutex
	items   map[string]map[string]types.AttributeValue
	updates []*dynamodb.UpdateItemInput
	getErr  error
	updErr  error
}

func newDynamo() *fakeDynamo { return &fakeDynamo{items: map[string]map[string]types.AttributeValue{}} }

func (f *fakeDynamo) put(item map[string]types.AttributeValue) {
	f.items[keyOf(item)] = item
}

func keyOf(item map[string]types.AttributeValue) string {
	return item["PK"].(*types.AttributeValueMemberS).Value + "|" + item["SK"].(*types.AttributeValueMemberS).Value
}

func (f *fakeDynamo) GetItem(_ context.Context, in *dynamodb.GetItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.getErr != nil {
		return nil, f.getErr
	}
	if in.ConsistentRead == nil || !*in.ConsistentRead {
		panic("the store must read with ConsistentRead: a run started right after a save has to see it")
	}
	item, ok := f.items[keyOf(in.Key)]
	if !ok {
		return &dynamodb.GetItemOutput{}, nil
	}
	out := map[string]types.AttributeValue{}
	if in.ProjectionExpression != nil {
		for _, ref := range strings.Split(*in.ProjectionExpression, ",") {
			name := in.ExpressionAttributeNames[strings.TrimSpace(ref)]
			if v, ok := item[name]; ok {
				out[name] = v
			}
		}
		return &dynamodb.GetItemOutput{Item: out}, nil
	}
	for k, v := range item {
		out[k] = v
	}
	return &dynamodb.GetItemOutput{Item: out}, nil
}

var (
	nameRef  = regexp.MustCompile(`#[A-Za-z0-9_]+`)
	valueRef = regexp.MustCompile(`:[A-Za-z0-9_]+`)
)

func (f *fakeDynamo) UpdateItem(_ context.Context, in *dynamodb.UpdateItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.updates = append(f.updates, in)
	if f.updErr != nil {
		return nil, f.updErr
	}
	update, cond := *in.UpdateExpression, ""
	if in.ConditionExpression != nil {
		cond = *in.ConditionExpression
	}
	if err := validateRefs(in, update+" "+cond); err != nil {
		return nil, err
	}

	item, exists := f.items[keyOf(in.Key)]
	if cond != "" {
		if cond != "attribute_exists(PK) AND #status <> :completed AND #status <> :failed" {
			panic("fakeDynamo does not understand the condition: " + cond)
		}
		status := ""
		if exists {
			if v, ok := item["status"].(*types.AttributeValueMemberS); ok {
				status = v.Value
			}
		}
		completed := in.ExpressionAttributeValues[":completed"].(*types.AttributeValueMemberS).Value
		failed := in.ExpressionAttributeValues[":failed"].(*types.AttributeValueMemberS).Value
		if !exists || status == completed || status == failed {
			return nil, &types.ConditionalCheckFailedException{}
		}
	}
	if !exists {
		item = map[string]types.AttributeValue{"PK": in.Key["PK"], "SK": in.Key["SK"]}
		f.items[keyOf(in.Key)] = item
	}
	applyUpdate(item, update, in)
	return &dynamodb.UpdateItemOutput{}, nil
}

// reserved is a few of DynamoDB's reserved words that attribute names in a job
// could collide with. Using one raw, outside an #alias, is a ValidationException.
var reserved = map[string]bool{"status": true, "error": true, "scenes": false, "progress": false}

func validateRefs(in *dynamodb.UpdateItemInput, expr string) error {
	used := map[string]bool{}
	for _, n := range nameRef.FindAllString(expr, -1) {
		used[n] = true
		if _, ok := in.ExpressionAttributeNames[n]; !ok {
			return fmt.Errorf("ValidationException: expression attribute name %s is used but not defined", n)
		}
	}
	for n := range in.ExpressionAttributeNames {
		if !used[n] {
			return fmt.Errorf("ValidationException: expression attribute name %s is defined but not used", n)
		}
	}
	usedV := map[string]bool{}
	for _, v := range valueRef.FindAllString(expr, -1) {
		usedV[v] = true
		if _, ok := in.ExpressionAttributeValues[v]; !ok {
			return fmt.Errorf("ValidationException: expression attribute value %s is used but not defined", v)
		}
	}
	for v := range in.ExpressionAttributeValues {
		if !usedV[v] {
			return fmt.Errorf("ValidationException: expression attribute value %s is defined but not used", v)
		}
	}
	stripped := nameRef.ReplaceAllString(valueRef.ReplaceAllString(expr, ""), "")
	for _, word := range regexp.MustCompile(`[A-Za-z_]+`).FindAllString(stripped, -1) {
		if reserved[strings.ToLower(word)] && !isKeyword(word) {
			return fmt.Errorf("ValidationException: attribute name %q is a reserved word and must be aliased", word)
		}
	}
	return nil
}

func isKeyword(w string) bool {
	switch w {
	case "SET", "REMOVE", "AND", "attribute_exists":
		return true
	}
	return false
}

func applyUpdate(item map[string]types.AttributeValue, expr string, in *dynamodb.UpdateItemInput) {
	setPart, removePart := expr, ""
	if i := strings.Index(expr, " REMOVE "); i >= 0 {
		setPart, removePart = expr[:i], expr[i+len(" REMOVE "):]
	}
	if !strings.HasPrefix(setPart, "SET ") {
		panic("fakeDynamo expects SET first: " + expr)
	}
	for _, assign := range strings.Split(strings.TrimPrefix(setPart, "SET "), ", ") {
		kv := strings.SplitN(assign, " = ", 2)
		item[in.ExpressionAttributeNames[kv[0]]] = in.ExpressionAttributeValues[kv[1]]
	}
	if removePart != "" {
		for _, ref := range strings.Split(removePart, ", ") {
			delete(item, in.ExpressionAttributeNames[ref])
		}
	}
}

func seeded(t *testing.T, j Job) (*DynamoStore, *fakeDynamo) {
	t.Helper()
	db := newDynamo()
	item, err := ToItem(j)
	if err != nil {
		t.Fatal(err)
	}
	db.put(item)
	return &DynamoStore{DB: db, Table: "tbl"}, db
}

func TestDynamoStore_LoadReadsTheJobWithAConsistentRead(t *testing.T) {
	st, _ := seeded(t, fullJob())
	j, err := st.Load(context.Background(), "u1", "j1")
	if err != nil {
		t.Fatal(err)
	}
	if j.JobID != "j1" || len(j.Scenes) != 2 || j.Caption.Preset != "karaoke" {
		t.Errorf("job = %+v", j)
	}
}

func TestDynamoStore_ALoadOfAJobThatDoesNotExistIsErrJobGone(t *testing.T) {
	st := &DynamoStore{DB: newDynamo(), Table: "t"}
	if _, err := st.Load(context.Background(), "u", "nope"); !errors.Is(err, ErrJobGone) {
		t.Errorf("err = %v", err)
	}
	if _, err := st.Status(context.Background(), "u", "nope"); !errors.Is(err, ErrJobGone) {
		t.Errorf("Status err = %v", err)
	}
}

func TestDynamoStore_ARecordItCannotReadIsAnErrorNotAGuess(t *testing.T) {
	db := newDynamo()
	item, _ := ToItem(fullJob())
	item["scenes"] = &types.AttributeValueMemberS{Value: "{broken"}
	db.put(item)
	st := &DynamoStore{DB: db, Table: "t"}
	if _, err := st.Load(context.Background(), "u1", "j1"); err == nil || errors.Is(err, ErrJobGone) {
		t.Errorf("err = %v", err)
	}
}

func TestDynamoStore_SaveWritesProgressAndKeepsWhatWasMade(t *testing.T) {
	j := fullJob()
	j.Status, j.Progress, j.Error, j.ErrorCode, j.FailedStage = StatusNarrating, 30, "", "", ""
	j.OutputKey, j.CompletedAt, j.TotalSeconds = "", "", 0
	st, db := seeded(t, j)

	j.Status, j.Progress, j.UpdatedAt = StatusRendering, 55, "2026-10-31T13:00:00Z"
	j.TotalSeconds = 31.45
	j.Scenes[1].Media = &Media{Key: "k2", Kind: "video", Width: 1080, Height: 1920}
	if err := st.Save(context.Background(), &j); err != nil {
		t.Fatal(err)
	}
	got, err := st.Load(context.Background(), "u1", "j1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != StatusRendering || got.Progress != 55 || got.UpdatedAt != "2026-10-31T13:00:00Z" || got.TotalSeconds != 31.45 {
		t.Errorf("job = %+v", got)
	}
	if got.Scenes[1].Media == nil || got.Scenes[1].Media.Key != "k2" {
		t.Errorf("what the stage made was not kept: %+v", got.Scenes[1])
	}
	// A progress write must not carry the output or completion fields.
	for _, u := range db.updates {
		if strings.Contains(*u.UpdateExpression, "outputKey") || strings.Contains(*u.UpdateExpression, "completedAt") {
			t.Errorf("Save wrote an end-of-job field: %s", *u.UpdateExpression)
		}
	}
}

func TestDynamoStore_SaveClearsAnOldFailure(t *testing.T) {
	j := fullJob() // has an Error, ErrorCode and FailedStage
	j.Status = StatusPreparing
	st, db := seeded(t, j)
	j.Error, j.ErrorCode, j.FailedStage = "", "", ""
	if err := st.Save(context.Background(), &j); err != nil {
		t.Fatal(err)
	}
	for _, attr := range []string{"error", "errorCode", "failedStage"} {
		if _, ok := db.items["USER#u1|STORYVIDEO#j1"][attr]; ok {
			t.Errorf("%s is still on the record", attr)
		}
	}
}

func TestDynamoStore_FinishRecordsTheOutputOrTheFailure(t *testing.T) {
	t.Run("completed", func(t *testing.T) {
		j := fullJob()
		j.Status = StatusRendering
		st, _ := seeded(t, j)
		j.Status, j.Progress, j.OutputKey = StatusCompleted, 100, "storyvideo/u1/j1/video.mp4"
		j.Error, j.ErrorCode, j.FailedStage = "", "", ""
		if err := st.Finish(context.Background(), &j); err != nil {
			t.Fatal(err)
		}
		got, _ := st.Load(context.Background(), "u1", "j1")
		if got.Status != StatusCompleted || got.OutputKey != "storyvideo/u1/j1/video.mp4" || got.CompletedAt == "" || got.Error != "" {
			t.Errorf("job = %+v", got)
		}
	})
	t.Run("failed", func(t *testing.T) {
		j := fullJob()
		j.Status, j.Error, j.ErrorCode, j.FailedStage, j.OutputKey, j.CompletedAt = StatusPreparing, "", "", "", "", ""
		st, _ := seeded(t, j)
		j.Status, j.Error, j.ErrorCode, j.FailedStage = StatusFailed, "Scene 2: gone.", CodeMediaUnavailable, StatusPreparing
		if err := st.Finish(context.Background(), &j); err != nil {
			t.Fatal(err)
		}
		got, _ := st.Load(context.Background(), "u1", "j1")
		if got.Status != StatusFailed || got.Error != "Scene 2: gone." || got.ErrorCode != CodeMediaUnavailable || got.OutputKey != "" {
			t.Errorf("job = %+v", got)
		}
	})
}

func TestDynamoStore_ADeletedJobIsNeverBroughtBack(t *testing.T) {
	st := &DynamoStore{DB: newDynamo(), Table: "t"}
	j := fullJob()
	for name, write := range map[string]func(context.Context, *Job) error{"Save": st.Save, "Finish": st.Finish} {
		if err := write(context.Background(), &j); !errors.Is(err, ErrJobGone) {
			t.Errorf("%s: err = %v, want ErrJobGone", name, err)
		}
	}
	if len(st.DB.(*fakeDynamo).items) != 0 {
		t.Error("a write created a record for a deleted job")
	}
}

func TestDynamoStore_AJobThatEndedElsewhereIsNeverOverwritten(t *testing.T) {
	for _, ended := range []string{StatusFailed, StatusCompleted} {
		t.Run(ended, func(t *testing.T) {
			j := fullJob()
			j.Status, j.Error, j.ErrorCode = ended, "Cancelled by the user.", "CANCELLED"
			st, db := seeded(t, j)
			late := fullJob()
			late.Status, late.Progress, late.Error, late.ErrorCode = StatusRendering, 70, "", ""
			err := st.Save(context.Background(), &late)
			if err == nil || errors.Is(err, ErrJobGone) || !errors.Is(err, errStoppedElsewhere) {
				t.Fatalf("err = %v, want errStoppedElsewhere", err)
			}
			rec := db.items["USER#u1|STORYVIDEO#j1"]
			if rec["status"].(*types.AttributeValueMemberS).Value != ended || rec["errorCode"].(*types.AttributeValueMemberS).Value != "CANCELLED" {
				t.Errorf("a late worker overwrote the end state: %v", rec)
			}
			if err := st.Finish(context.Background(), &late); !errors.Is(err, errStoppedElsewhere) {
				t.Errorf("Finish err = %v", err)
			}
		})
	}
}

func TestDynamoStore_OtherErrorsAreReturnedAsTheyAre(t *testing.T) {
	st, db := seeded(t, fullJob())
	db.updErr = errors.New("ProvisionedThroughputExceededException")
	if err := st.Save(context.Background(), &Job{UserID: "u1", JobID: "j1", Status: StatusRendering}); err == nil || errors.Is(err, ErrJobGone) {
		t.Errorf("err = %v", err)
	}
	db.updErr, db.getErr = nil, errors.New("throttled")
	if _, err := st.Load(context.Background(), "u1", "j1"); err == nil || errors.Is(err, ErrJobGone) {
		t.Errorf("Load err = %v", err)
	}
	if _, err := st.Status(context.Background(), "u1", "j1"); err == nil {
		t.Error("Status err = nil")
	}
}

// The double itself: if it did not enforce DynamoDB's rules, the tests above
// would prove nothing.
func TestFakeDynamo_RejectsWhatDynamoDBRejects(t *testing.T) {
	db := newDynamo()
	run := func(update string, names map[string]string, vals map[string]types.AttributeValue) error {
		_, err := db.UpdateItem(context.Background(), &dynamodb.UpdateItemInput{
			Key: Key("u", "j"), UpdateExpression: &update, ExpressionAttributeNames: names, ExpressionAttributeValues: vals,
		})
		return err
	}
	v := &types.AttributeValueMemberS{Value: "x"}
	if err := run("SET #a = :a", map[string]string{"#a": "a", "#unused": "b"}, map[string]types.AttributeValue{":a": v}); err == nil {
		t.Error("an unused name was accepted")
	}
	if err := run("SET #a = :a", map[string]string{"#a": "a"}, map[string]types.AttributeValue{":a": v, ":unused": v}); err == nil {
		t.Error("an unused value was accepted")
	}
	if err := run("SET #a = :a, #b = :b", map[string]string{"#a": "a"}, map[string]types.AttributeValue{":a": v, ":b": v}); err == nil {
		t.Error("an undefined name was accepted")
	}
	if err := run("SET status = :a", nil, map[string]types.AttributeValue{":a": v}); err == nil || !strings.Contains(err.Error(), "reserved") {
		t.Errorf("a raw reserved word was accepted: %v", err)
	}
	if err := run("SET #a = :a", map[string]string{"#a": "a"}, map[string]types.AttributeValue{":a": v}); err != nil {
		t.Errorf("a valid update was refused: %v", err)
	}
}

func TestDynamoStore_EveryWriteItMakesIsOneDynamoDBAccepts(t *testing.T) {
	// Every shape of write: progress, a stage change with an old failure to
	// clear, a failure, and a completion. The double rejects an expression that
	// DynamoDB would, so passing means the expressions are well formed.
	j := fullJob()
	j.Status = StatusPreparing
	st, db := seeded(t, j)
	shapes := []func(*Job){
		func(j *Job) {
			j.Status, j.Progress, j.Error, j.ErrorCode, j.FailedStage = StatusNarrating, 30, "", "", ""
		},
		func(j *Job) { j.Progress, j.TotalSeconds = 33, 0 },
		func(j *Job) {
			j.Status, j.Error, j.ErrorCode, j.FailedStage = StatusRendering, "m", "C", StatusRendering
		},
	}
	for i, shape := range shapes {
		c := fullJob()
		c.Status = StatusPreparing
		shape(&c)
		if err := st.Save(context.Background(), &c); err != nil {
			t.Fatalf("Save shape %d: %v", i, err)
		}
	}
	for name, shape := range map[string]func(*Job){
		"completed": func(j *Job) { j.Status, j.Error, j.ErrorCode, j.FailedStage = StatusCompleted, "", "", "" },
		"failed":    func(j *Job) { j.Status, j.OutputKey, j.CompletedAt = StatusFailed, "", "" },
	} {
		db2 := newDynamo()
		item, _ := ToItem(j)
		db2.put(item)
		st2 := &DynamoStore{DB: db2, Table: "t"}
		c := fullJob()
		shape(&c)
		if err := st2.Finish(context.Background(), &c); err != nil {
			t.Fatalf("Finish %s: %v", name, err)
		}
	}
	var seen []string
	for _, u := range db.updates {
		seen = append(seen, *u.UpdateExpression)
	}
	sort.Strings(seen)
	if len(seen) == 0 {
		t.Fatal("nothing was written")
	}
}
