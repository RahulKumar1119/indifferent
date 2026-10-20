package thumbnail

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	db "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type fakeDB struct {
	item    map[string]db.AttributeValue
	updates []*dynamodb.UpdateItemInput
	deletes []*dynamodb.DeleteItemInput
	updErr  error
	updOut  *dynamodb.UpdateItemOutput
}

func (f *fakeDB) GetItem(_ context.Context, _ *dynamodb.GetItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
	return &dynamodb.GetItemOutput{Item: f.item}, nil
}

func (f *fakeDB) UpdateItem(_ context.Context, in *dynamodb.UpdateItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
	f.updates = append(f.updates, in)
	if f.updErr != nil {
		return nil, f.updErr
	}
	if f.updOut != nil {
		return f.updOut, nil
	}
	return &dynamodb.UpdateItemOutput{}, nil
}

func (f *fakeDB) DeleteItem(_ context.Context, in *dynamodb.DeleteItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error) {
	f.deletes = append(f.deletes, in)
	return &dynamodb.DeleteItemOutput{}, nil
}

var fixedNow = time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)

func newStore(f *fakeDB) *Store {
	return &Store{DB: f, Table: "table", Now: func() time.Time { return fixedNow }}
}

func condFailed() error { return &db.ConditionalCheckFailedException{} }

func TestKeyIsPerUserAndJob(t *testing.T) {
	k := Key("u1", "j1")
	if k["PK"].(*db.AttributeValueMemberS).Value != "USER#u1" || k["SK"].(*db.AttributeValueMemberS).Value != "THUMBS#j1" {
		t.Errorf("key = %v", k)
	}
	// It must not collide with the job record, which lists as SHORTS#.
	if strings.HasPrefix(k["SK"].(*db.AttributeValueMemberS).Value, "SHORTS#") {
		t.Error("thumbnail records must not appear in the Shorts job list")
	}
}

func TestQueueRefusesWhileAFreshRunIsInFlight(t *testing.T) {
	f := &fakeDB{updErr: condFailed()}
	err := newStore(f).Queue(context.Background(), "u1", "j1", StyleNews, FormatPortrait)
	if !errors.Is(err, ErrInFlight) {
		t.Fatalf("err = %v, want ErrInFlight", err)
	}
	in := f.updates[0]
	cond := *in.ConditionExpression
	for _, want := range []string{"attribute_not_exists(PK)", ":completed", ":failed", "updatedAt < :stale"} {
		if !strings.Contains(cond, want) {
			t.Errorf("condition %q missing %q", cond, want)
		}
	}
	// A run is stale after StaleAfter.
	stale := in.ExpressionAttributeValues[":stale"].(*db.AttributeValueMemberS).Value
	if want := fixedNow.Add(-StaleAfter).Format(time.RFC3339); stale != want {
		t.Errorf("stale cutoff = %q, want %q", stale, want)
	}
}

func TestQueueStoresRequestedOptions(t *testing.T) {
	f := &fakeDB{}
	if err := newStore(f).Queue(context.Background(), "u1", "j1", StyleNews, FormatPortrait); err != nil {
		t.Fatal(err)
	}
	in := f.updates[0]
	expr := *in.UpdateExpression
	if !strings.Contains(expr, "reqStyle = :style") || !strings.Contains(expr, "reqFormat = :format") {
		t.Errorf("expression %q should save the request", expr)
	}
	if in.ExpressionAttributeValues[":queued"].(*db.AttributeValueMemberS).Value != "queued" {
		t.Error("status should become queued")
	}

	f = &fakeDB{}
	_ = newStore(f).Queue(context.Background(), "u1", "j1", "", "")
	expr = *f.updates[0].UpdateExpression
	if !strings.Contains(expr, "REMOVE") || !strings.Contains(expr, "reqStyle") || strings.Contains(expr, "reqStyle = ") {
		t.Errorf("automatic choices should clear any earlier request: %q", expr)
	}
	// Keeps the previous thumbnails: nothing in the expression removes them.
	if strings.Contains(expr, "thumbnails") {
		t.Error("queueing must not discard the previous thumbnails")
	}
}

func TestBeginReturnsGenerationAndRequest(t *testing.T) {
	f := &fakeDB{updOut: &dynamodb.UpdateItemOutput{Attributes: map[string]db.AttributeValue{
		"generation": &db.AttributeValueMemberN{Value: "4"},
		"reqStyle":   str("cinematic"),
		"reqFormat":  str("9:16"),
	}}}
	gen, style, format, err := newStore(f).Begin(context.Background(), "u1", "j1")
	if err != nil {
		t.Fatal(err)
	}
	if gen != 4 || style != StyleCinematic || format != FormatPortrait {
		t.Errorf("gen=%d style=%q format=%q", gen, style, format)
	}
	if !strings.Contains(*f.updates[0].UpdateExpression, "ADD #generation") {
		t.Error("the generation must be incremented atomically")
	}
	if !strings.Contains(*f.updates[0].ConditionExpression, ":queued") {
		t.Error("a queued record must be claimable")
	}
}

func TestBeginRefusesAFreshRunningRecord(t *testing.T) {
	f := &fakeDB{updErr: condFailed()}
	if _, _, _, err := newStore(f).Begin(context.Background(), "u1", "j1"); !errors.Is(err, ErrInFlight) {
		t.Errorf("err = %v, want ErrInFlight", err)
	}
}

func TestGuardedWritesRequireTheSameGeneration(t *testing.T) {
	ctx := context.Background()
	f := &fakeDB{}
	s := newStore(f)
	if err := s.SetStage(ctx, "u1", "j1", 3, StageRendering); err != nil {
		t.Fatal(err)
	}
	if err := s.Fail(ctx, "u1", "j1", 3, CodeAIFailed, "The AI could not analyze this video."); err != nil {
		t.Fatal(err)
	}
	if err := s.Complete(ctx, "u1", "j1", 3, Result{Style: StyleViral, Format: FormatLandscape}); err != nil {
		t.Fatal(err)
	}
	for i, in := range f.updates {
		if !strings.Contains(*in.ConditionExpression, "attribute_exists(PK)") || !strings.Contains(*in.ConditionExpression, "generation = :gen") {
			t.Errorf("update %d is not guarded: %q", i, *in.ConditionExpression)
		}
		if in.ExpressionAttributeValues[":gen"].(*db.AttributeValueMemberN).Value != "3" {
			t.Errorf("update %d uses the wrong generation", i)
		}
	}
	// A job deleted mid-run, or a newer run, shows up as ErrGone.
	f.updErr = condFailed()
	if err := s.Complete(ctx, "u1", "j1", 3, Result{}); !errors.Is(err, ErrGone) {
		t.Errorf("err = %v, want ErrGone", err)
	}
}

func TestCompleteClearsSelectionAndKeepsNothingStale(t *testing.T) {
	f := &fakeDB{}
	_ = newStore(f).Complete(context.Background(), "u1", "j1", 2, Result{
		Thumbnails: []Asset{{ID: "thumb-01"}},
		Style:      StyleViral, Format: FormatLandscape,
	})
	expr := *f.updates[0].UpdateExpression
	for _, want := range []string{"REMOVE", "selectedId", "errorCode", "stage", "reqStyle"} {
		if !strings.Contains(expr, want) {
			t.Errorf("complete expression missing %q: %s", want, expr)
		}
	}
}

func TestFailKeepsEarlierThumbnails(t *testing.T) {
	f := &fakeDB{}
	_ = newStore(f).Fail(context.Background(), "u1", "j1", 2, CodeNoSuitableFrames, "msg")
	if strings.Contains(*f.updates[0].UpdateExpression, "thumbnails") {
		t.Error("a failed regenerate must not remove the thumbnails the user already has")
	}
}

func TestSelectNeedsACompletedSet(t *testing.T) {
	f := &fakeDB{}
	s := newStore(f)
	if err := s.Select(context.Background(), "u1", "j1", "thumb-02"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(*f.updates[0].ConditionExpression, "#status = :completed") {
		t.Errorf("condition = %q", *f.updates[0].ConditionExpression)
	}
	f.updErr = condFailed()
	if err := s.Select(context.Background(), "u1", "j1", "thumb-02"); !errors.Is(err, ErrNotReady) {
		t.Errorf("err = %v, want ErrNotReady", err)
	}
}

func TestDeleteIsScopedToTheJobRecord(t *testing.T) {
	f := &fakeDB{}
	if err := newStore(f).Delete(context.Background(), "u1", "j1"); err != nil {
		t.Fatal(err)
	}
	if got := f.deletes[0].Key["SK"].(*db.AttributeValueMemberS).Value; got != "THUMBS#j1" {
		t.Errorf("deleted %q", got)
	}
}

func TestGetDecodesARecordAndToleratesNone(t *testing.T) {
	f := &fakeDB{}
	set, err := newStore(f).Get(context.Background(), "u1", "j1")
	if err != nil || set != nil {
		t.Fatalf("missing record: %v %v", set, err)
	}
	f.item = map[string]db.AttributeValue{
		"status":         str("completed"),
		"style":          str("viral"),
		"format":         str("16:9"),
		"generation":     &db.AttributeValueMemberN{Value: "2"},
		"selectedId":     str("thumb-02"),
		"transcriptUsed": &db.AttributeValueMemberBOOL{Value: true},
		"thumbnails":     str(`[{"id":"thumb-01","concept":"face","key":"k1","headline":"HI"},{"id":"thumb-02","concept":"subject","key":"k2","headline":"HI"}]`),
		"analysis":       str(`{"topic":"t","headline":"HI","bestFrameTime":12}`),
		"funnel":         str(`{"extracted":40,"usable":30,"unique":20,"candidates":8}`),
	}
	set, err = newStore(f).Get(context.Background(), "u1", "j1")
	if err != nil {
		t.Fatal(err)
	}
	if set.Status != StatusCompleted || set.Generation != 2 || len(set.Thumbnails) != 2 || !set.TranscriptUsed {
		t.Errorf("set = %+v", set)
	}
	if sel := set.Selected(); sel == nil || sel.Key != "k2" {
		t.Errorf("selected = %+v", sel)
	}
	if set.Funnel == nil || set.Funnel.Candidates != 8 || set.Analysis == nil || set.Analysis.BestFrameTime != 12 {
		t.Errorf("analysis/funnel = %+v %+v", set.Analysis, set.Funnel)
	}
	if set.JobID != "j1" || set.UserID != "u1" {
		t.Errorf("ids = %q %q", set.UserID, set.JobID)
	}

	// Damaged JSON columns must not break the read.
	f.item["thumbnails"] = str("{not json")
	f.item["analysis"] = str("oops")
	set, err = newStore(f).Get(context.Background(), "u1", "j1")
	if err != nil || set == nil || len(set.Thumbnails) != 0 || set.Analysis != nil {
		t.Errorf("damaged record: %+v %v", set, err)
	}
}
