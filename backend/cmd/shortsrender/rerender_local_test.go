package main

// renderOne (#329) against DynamoDB Local. Skipped unless DYNAMODB_LOCAL_ENDPOINT
// is set: scripts/ops/cliprender_dynamo_check.sh

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/rahul/indifferent/backend/internal/models"
	"github.com/rahul/indifferent/backend/internal/shorts/cliprender"
	"github.com/rahul/indifferent/backend/internal/shorts/render"
)

type fakeClipRenderer struct {
	got   render.RenderInput
	run   func(ctx context.Context) error
	calls int
}

func (f *fakeClipRenderer) Render(ctx context.Context, in render.RenderInput) (string, error) {
	f.calls++
	f.got = in
	if f.run != nil {
		if err := f.run(ctx); err != nil {
			return "", err
		}
	}
	return in.OutputKey, nil
}

func localTable(t *testing.T) (*dynamodb.Client, string) {
	t.Helper()
	endpoint := os.Getenv("DYNAMODB_LOCAL_ENDPOINT")
	if endpoint == "" {
		t.Skip("DYNAMODB_LOCAL_ENDPOINT not set (see scripts/ops/cliprender_dynamo_check.sh)")
	}
	db := dynamodb.New(dynamodb.Options{Region: "ap-south-1", BaseEndpoint: aws.String(endpoint),
		Credentials: credentials.NewStaticCredentialsProvider("local", "local", "")})
	table := fmt.Sprintf("worker-%d", time.Now().UnixNano())
	if _, err := db.CreateTable(context.Background(), &dynamodb.CreateTableInput{
		TableName: aws.String(table),
		AttributeDefinitions: []dbtypes.AttributeDefinition{
			{AttributeName: aws.String("PK"), AttributeType: dbtypes.ScalarAttributeTypeS},
			{AttributeName: aws.String("SK"), AttributeType: dbtypes.ScalarAttributeTypeS},
		},
		KeySchema: []dbtypes.KeySchemaElement{
			{AttributeName: aws.String("PK"), KeyType: dbtypes.KeyTypeHash},
			{AttributeName: aws.String("SK"), KeyType: dbtypes.KeyTypeRange},
		},
		BillingMode: dbtypes.BillingModePayPerRequest,
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.DeleteTable(context.Background(), &dynamodb.DeleteTableInput{TableName: aws.String(table)})
	})
	old := cancelPollInterval
	cancelPollInterval = 50 * time.Millisecond
	t.Cleanup(func() { cancelPollInterval = old })
	return db, table
}

func claimed(t *testing.T, db *dynamodb.Client, table, token string) {
	t.Helper()
	now := time.Now()
	if err := cliprender.Claim(context.Background(), db, table, nil, cliprender.Record{UserID: "u1", JobID: "j1", ClipID: "clip-1",
		Token: token, State: models.ClipRenderRendering, RequestIDs: []string{"r"}, RequestedAt: now}, now); err != nil {
		t.Fatal(err)
	}
}

func input() render.RenderInput {
	return render.RenderInput{JobID: "j1", UserID: "u1", Clip: models.Clip{ClipID: "clip-1"}}
}

func get(t *testing.T, db *dynamodb.Client, table string) *cliprender.Record {
	r, err := cliprender.Get(context.Background(), db, table, "u1", "j1", "clip-1")
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestLocalWorker_SuccessPointsTheClipAtTheNewFile(t *testing.T) {
	db, table := localTable(t)
	tok := strings.Repeat("a", 32)
	claimed(t, db, table, tok)
	r := &fakeClipRenderer{}
	if err := renderOne(context.Background(), r, db, table, input(), tok); err != nil {
		t.Fatal(err)
	}
	want := cliprender.OutputKey("u1", "j1", "clip-1", tok)
	if r.got.OutputKey != want {
		t.Fatalf("rendered to %q", r.got.OutputKey)
	}
	if rec := get(t, db, table); rec.State != models.ClipRenderDone || rec.ActiveKey != want || rec.Renders != 1 {
		t.Fatalf("record %+v", rec)
	}
}

func TestLocalWorker_FailureKeepsTheReasonAndLeavesTheFile(t *testing.T) {
	db, table := localTable(t)
	tok := strings.Repeat("b", 32)
	claimed(t, db, table, tok)
	r := &fakeClipRenderer{run: func(context.Context) error { return fmt.Errorf("%w: boom", render.ErrRenderTimeout) }}
	err := renderOne(context.Background(), r, db, table, input(), tok)
	if !errors.Is(err, render.ErrRenderTimeout) {
		t.Fatalf("err %v, want the render error so the state machine can try again", err)
	}
	out, _ := db.GetItem(context.Background(), &dynamodb.GetItemInput{TableName: aws.String(table), Key: cliprender.Key("u1", "j1", "clip-1"), ConsistentRead: aws.Bool(true)})
	last, _ := out.Item["lastError"].(*dbtypes.AttributeValueMemberS)
	rec := get(t, db, table)
	if last == nil || last.Value != renderTimeoutMessage || rec.State != models.ClipRenderRendering || rec.ActiveKey != "" {
		t.Fatalf("lastError %v record %+v", last, rec)
	}
	// A second try that succeeds clears the reason.
	if err := renderOne(context.Background(), &fakeClipRenderer{}, db, table, input(), tok); err != nil {
		t.Fatal(err)
	}
	out, _ = db.GetItem(context.Background(), &dynamodb.GetItemInput{TableName: aws.String(table), Key: cliprender.Key("u1", "j1", "clip-1"), ConsistentRead: aws.Bool(true)})
	if _, ok := out.Item["lastError"]; ok {
		t.Fatal("lastError kept after success")
	}
}

func TestLocalWorker_ANewerRequestStopsTheRenderWithoutAnError(t *testing.T) {
	db, table := localTable(t)
	tokOld, tokNew := strings.Repeat("c", 32), strings.Repeat("d", 32)
	claimed(t, db, table, tokOld)
	r := &fakeClipRenderer{run: func(ctx context.Context) error {
		// A newer request takes the clip (the old one went stale).
		cur := get(t, db, table)
		later := time.Now().Add(cliprender.StaleAfter + time.Minute)
		next := cliprender.Record{UserID: "u1", JobID: "j1", ClipID: "clip-1", Token: tokNew, State: models.ClipRenderRendering, RequestedAt: later}
		if err := cliprender.Claim(context.Background(), db, table, cur, next, later); err != nil {
			t.Error(err)
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("%w: stopped", render.ErrRenderCanceled)
		case <-time.After(5 * time.Second):
			return errors.New("the watcher did not stop the render")
		}
	}}
	if err := renderOne(context.Background(), r, db, table, input(), tokOld); err != nil {
		t.Fatalf("a superseded render ends without an error, got %v", err)
	}
	if rec := get(t, db, table); rec.Token != tokNew || rec.State != models.ClipRenderRendering || rec.ActiveKey != "" {
		t.Fatalf("the newer render's record was touched: %+v", rec)
	}
}

func TestLocalWorker_ADeletedJobStopsTheRender(t *testing.T) {
	db, table := localTable(t)
	tok := strings.Repeat("e", 32)
	claimed(t, db, table, tok)
	r := &fakeClipRenderer{run: func(ctx context.Context) error {
		if _, err := cliprender.DeleteAll(context.Background(), db, table, "u1", "j1", time.Now()); err != nil {
			t.Error(err)
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("%w: stopped", render.ErrRenderCanceled)
		case <-time.After(5 * time.Second):
			return errors.New("the watcher did not stop the render")
		}
	}}
	if err := renderOne(context.Background(), r, db, table, input(), tok); err != nil {
		t.Fatalf("got %v", err)
	}
	if get(t, db, table) != nil {
		t.Fatal("record recreated")
	}
}

func TestLocalWorker_SupersededAfterRenderingIsNotPromoted(t *testing.T) {
	db, table := localTable(t)
	tokOld, tokNew := strings.Repeat("f", 32), strings.Repeat("0", 32)
	claimed(t, db, table, tokOld)
	r := &fakeClipRenderer{run: func(context.Context) error {
		cur := get(t, db, table)
		later := time.Now().Add(cliprender.StaleAfter + time.Minute)
		if err := cliprender.Claim(context.Background(), db, table, cur, cliprender.Record{UserID: "u1", JobID: "j1", ClipID: "clip-1", Token: tokNew, State: models.ClipRenderRendering, RequestedAt: later}, later); err != nil {
			t.Error(err)
		}
		return nil // finishes before the watcher looks
	}}
	if err := renderOne(context.Background(), r, db, table, input(), tokOld); err != nil {
		t.Fatal(err)
	}
	if rec := get(t, db, table); rec.ActiveKey != "" || rec.Token != tokNew {
		t.Fatalf("an older render replaced a newer one: %+v", rec)
	}
}

func TestHookFromItem(t *testing.T) {
	item := map[string]dbtypes.AttributeValue{"clipHooks": &dbtypes.AttributeValueMemberS{Value: `{"clip-1":{"enabled":true,"text":" hi ","style":"","position":"","seconds":0},"clip-2":{"enabled":false,"text":"x"},"clip-3":{"enabled":true,"text":"x","style":"neon"}}`}}
	h := hookFromItem(item, "clip-1")
	if h == nil || h.Text != "hi" || h.Style != "box" || h.Position != "top" || h.Seconds != 2.5 {
		t.Fatalf("clip-1 %+v", h)
	}
	if hookFromItem(item, "clip-2") != nil || hookFromItem(item, "clip-3") != nil || hookFromItem(item, "clip-9") != nil {
		t.Fatal("off, invalid or missing hooks must render without one")
	}
	if hookFromItem(map[string]dbtypes.AttributeValue{"clipHooks": &dbtypes.AttributeValueMemberS{Value: "{"}}, "clip-1") != nil {
		t.Fatal("unreadable hooks render without one")
	}
}
