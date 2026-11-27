package cliprender

// These tests run the record's conditional writes against DynamoDB Local, because
// the guarantees (one render per clip, an older render never replacing a newer
// one) live in the condition expressions, which an in-memory fake would only
// imitate. They are skipped unless DYNAMODB_LOCAL_ENDPOINT is set:
//
//	scripts/ops/cliprender_dynamo_check.sh

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/rahul/indifferent/backend/internal/models"
)

func localDB(t *testing.T) (*dynamodb.Client, string) {
	t.Helper()
	endpoint := os.Getenv("DYNAMODB_LOCAL_ENDPOINT")
	if endpoint == "" {
		t.Skip("DYNAMODB_LOCAL_ENDPOINT not set (see scripts/ops/cliprender_dynamo_check.sh)")
	}
	db := dynamodb.New(dynamodb.Options{
		Region:       "ap-south-1",
		BaseEndpoint: aws.String(endpoint),
		Credentials:  credentials.NewStaticCredentialsProvider("local", "local", ""),
	})
	table := fmt.Sprintf("cliprender-%d", time.Now().UnixNano())
	_, err := db.CreateTable(context.Background(), &dynamodb.CreateTableInput{
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
	})
	if err != nil {
		t.Fatalf("create table: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.DeleteTable(context.Background(), &dynamodb.DeleteTableInput{TableName: aws.String(table)})
	})
	return db, table
}

func claimRec(token, requestID string, now time.Time) Record {
	return Record{UserID: "u1", JobID: "j1", ClipID: "clip-1", Token: token, State: models.ClipRenderRendering,
		RequestIDs: []string{requestID}, RequestedAt: now, ExecutionName: ExecutionName(token)}
}

func mustGet(t *testing.T, db DB, table string) *Record {
	t.Helper()
	r, err := Get(context.Background(), db, table, "u1", "j1", "clip-1")
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestLocal_OnlyOneOfManyConcurrentClaimsWins(t *testing.T) {
	db, table := localDB(t)
	ctx := context.Background()
	now := time.Now()
	var wg sync.WaitGroup
	var mu sync.Mutex
	wins, conflicts := 0, 0
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			err := Claim(ctx, db, table, nil, claimRec(fmt.Sprintf("%032x", i+1), fmt.Sprintf("req-%016d", i), now), now)
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				wins++
			case errors.Is(err, ErrConflict):
				conflicts++
			default:
				t.Errorf("claim %d: %v", i, err)
			}
		}(i)
	}
	wg.Wait()
	if wins != 1 || conflicts != 19 {
		t.Fatalf("wins=%d conflicts=%d, want exactly one win", wins, conflicts)
	}
}

func TestLocal_ClaimIsRefusedWhileARenderRuns_AndAllowedOnceStale(t *testing.T) {
	db, table := localDB(t)
	ctx := context.Background()
	now := time.Now()
	tokA, tokB := strings.Repeat("a", 32), strings.Repeat("b", 32)
	if err := Claim(ctx, db, table, nil, claimRec(tokA, "req-aaaaaaaaaaaaaaaa", now), now); err != nil {
		t.Fatal(err)
	}
	cur := mustGet(t, db, table)
	if err := Claim(ctx, db, table, cur, claimRec(tokB, "req-bbbbbbbbbbbbbbbb", now), now); !errors.Is(err, ErrConflict) {
		t.Fatalf("claim over a running render: %v, want ErrConflict", err)
	}
	// The same claim after the render has gone stale takes its place, and the old
	// render can then neither finish nor record an error.
	later := now.Add(StaleAfter + time.Minute)
	if err := Claim(ctx, db, table, cur, claimRec(tokB, "req-bbbbbbbbbbbbbbbb", later), later); err != nil {
		t.Fatalf("claim over a stale render: %v", err)
	}
	if err := Promote(ctx, db, table, "u1", "j1", "clip-1", tokA, "shorts/u1/j1/clips/clip-1.r-"+tokA+".mp4", later); !errors.Is(err, ErrSuperseded) {
		t.Fatalf("stale render promote: %v, want ErrSuperseded", err)
	}
	if err := RecordError(ctx, db, table, "u1", "j1", "clip-1", tokA, "x"); !errors.Is(err, ErrSuperseded) {
		t.Fatalf("stale render error: %v, want ErrSuperseded", err)
	}
	if got := mustGet(t, db, table); got.Token != tokB || got.ActiveKey != "" || got.State != models.ClipRenderRendering {
		t.Fatalf("record after stale takeover: %+v", got)
	}
}

func TestLocal_PromoteKeepsThePreviousFile_AndAnOlderRenderNeverReplacesANewerOne(t *testing.T) {
	db, table := localDB(t)
	ctx := context.Background()
	now := time.Now()
	tok1, tok2, tok3 := strings.Repeat("1", 32), strings.Repeat("2", 32), strings.Repeat("3", 32)
	key := func(tok string) string { return OutputKey("u1", "j1", "clip-1", tok) }

	if err := Claim(ctx, db, table, nil, claimRec(tok1, "req-1111111111111111", now), now); err != nil {
		t.Fatal(err)
	}
	if err := Promote(ctx, db, table, "u1", "j1", "clip-1", tok1, key(tok1), now); err != nil {
		t.Fatal(err)
	}
	r1 := mustGet(t, db, table)
	if r1.State != models.ClipRenderDone || r1.ActiveKey != key(tok1) || r1.PreviousKey != "" || r1.Renders != 1 {
		t.Fatalf("after first render: %+v", r1)
	}
	// Promoting twice is refused: the render is no longer running.
	if err := Promote(ctx, db, table, "u1", "j1", "clip-1", tok1, key(tok1), now); !errors.Is(err, ErrSuperseded) {
		t.Fatalf("second promote: %v", err)
	}

	next := claimRec(tok2, "req-2222222222222222", now)
	next.ActiveKey, next.Renders = r1.ActiveKey, r1.Renders
	if err := Claim(ctx, db, table, r1, next, now); err != nil {
		t.Fatal(err)
	}
	// While render 2 runs, the clip still plays render 1.
	if got := mustGet(t, db, table); got.ActiveKey != key(tok1) {
		t.Fatalf("active key while rendering: %q", got.ActiveKey)
	}
	if err := Promote(ctx, db, table, "u1", "j1", "clip-1", tok2, key(tok2), now); err != nil {
		t.Fatal(err)
	}
	r2 := mustGet(t, db, table)
	if r2.ActiveKey != key(tok2) || r2.PreviousKey != key(tok1) || r2.Renders != 2 {
		t.Fatalf("after second render: %+v", r2)
	}

	// A claim made from a stale read (r1, before render 2 finished) is refused,
	// so it cannot put render 1's file back.
	stale := claimRec(tok3, "req-3333333333333333", now)
	stale.ActiveKey, stale.Renders = r1.ActiveKey, r1.Renders
	if err := Claim(ctx, db, table, r1, stale, now); !errors.Is(err, ErrConflict) {
		t.Fatalf("claim from a stale read: %v, want ErrConflict", err)
	}
	if got := mustGet(t, db, table); got.ActiveKey != key(tok2) {
		t.Fatalf("active key after refused claim: %q", got.ActiveKey)
	}
}

func TestLocal_FailedRenderLeavesTheFileThatPlays(t *testing.T) {
	db, table := localDB(t)
	ctx := context.Background()
	now := time.Now()
	tok1, tok2 := strings.Repeat("c", 32), strings.Repeat("d", 32)
	if err := Claim(ctx, db, table, nil, claimRec(tok1, "req-cccccccccccccccc", now), now); err != nil {
		t.Fatal(err)
	}
	good := OutputKey("u1", "j1", "clip-1", tok1)
	if err := Promote(ctx, db, table, "u1", "j1", "clip-1", tok1, good, now); err != nil {
		t.Fatal(err)
	}
	r1 := mustGet(t, db, table)
	next := claimRec(tok2, "req-dddddddddddddddd", now)
	next.ActiveKey, next.Renders = r1.ActiveKey, r1.Renders
	if err := Claim(ctx, db, table, r1, next, now); err != nil {
		t.Fatal(err)
	}
	if err := RecordError(ctx, db, table, "u1", "j1", "clip-1", tok2, "This clip took too long to render."); err != nil {
		t.Fatal(err)
	}
	// The state machine's MarkFailed state, run exactly as written in the definition.
	runMarkFailed(t, db, table, tok2)
	got := mustGet(t, db, table)
	if got.State != models.ClipRenderFailed || got.Error != "This clip took too long to render." || got.ActiveKey != good || got.Renders != 1 {
		t.Fatalf("after failure: %+v", got)
	}
	// Run again (a second catch) it changes nothing: the render is no longer running.
	if err := markFailed(db, table, tok2); err == nil {
		t.Fatal("MarkFailed on a record already failed should fail its condition")
	}
}

func TestLocal_MarkFailedWithoutAReasonUsesTheGeneralOne_AndSkipsANewerRender(t *testing.T) {
	db, table := localDB(t)
	ctx := context.Background()
	now := time.Now()
	tok1, tok2 := strings.Repeat("e", 32), strings.Repeat("f", 32)
	if err := Claim(ctx, db, table, nil, claimRec(tok1, "req-eeeeeeeeeeeeeeee", now), now); err != nil {
		t.Fatal(err)
	}
	runMarkFailed(t, db, table, tok1)
	if got := mustGet(t, db, table); got.State != models.ClipRenderFailed || !strings.Contains(got.Error, "The clip you had is unchanged") {
		t.Fatalf("after failure without a reason: %+v", got)
	}
	r := mustGet(t, db, table)
	if err := Claim(ctx, db, table, r, claimRec(tok2, "req-ffffffffffffffff", now), now); err != nil {
		t.Fatal(err)
	}
	// The old execution's MarkFailed arriving late does not touch the new render.
	if err := markFailed(db, table, tok1); err == nil {
		t.Fatal("a late MarkFailed for an old token must fail its condition")
	}
	if got := mustGet(t, db, table); got.State != models.ClipRenderRendering || got.Token != tok2 {
		t.Fatalf("new render after a late MarkFailed: %+v", got)
	}
}

func TestLocal_RestorePutsThePreviousRecordBack(t *testing.T) {
	db, table := localDB(t)
	ctx := context.Background()
	now := time.Now()
	tok1, tok2 := strings.Repeat("7", 32), strings.Repeat("8", 32)
	// No record before: a restore removes the claim.
	c := claimRec(tok1, "req-7777777777777777", now)
	if err := Claim(ctx, db, table, nil, c, now); err != nil {
		t.Fatal(err)
	}
	if err := Restore(ctx, db, table, nil, c); err != nil {
		t.Fatal(err)
	}
	if got := mustGet(t, db, table); got != nil {
		t.Fatalf("record after restore to nothing: %+v", got)
	}
	// With a record before: it comes back as it was.
	if err := Claim(ctx, db, table, nil, c, now); err != nil {
		t.Fatal(err)
	}
	if err := Promote(ctx, db, table, "u1", "j1", "clip-1", tok1, OutputKey("u1", "j1", "clip-1", tok1), now); err != nil {
		t.Fatal(err)
	}
	before := mustGet(t, db, table)
	next := claimRec(tok2, "req-8888888888888888", now)
	next.ActiveKey, next.Renders, next.RequestIDs = before.ActiveKey, before.Renders, append(before.RequestIDs, "req-8888888888888888")
	if err := Claim(ctx, db, table, before, next, now); err != nil {
		t.Fatal(err)
	}
	if err := Restore(ctx, db, table, before, next); err != nil {
		t.Fatal(err)
	}
	after := mustGet(t, db, table)
	if after.Token != tok1 || after.State != models.ClipRenderDone || after.ActiveKey != before.ActiveKey || after.HasRequest("req-8888888888888888") {
		t.Fatalf("restored record: %+v", after)
	}
	// A restore for a claim that is no longer current does nothing.
	if err := Restore(ctx, db, table, nil, next); !errors.Is(err, ErrSuperseded) {
		t.Fatalf("restore of a stale claim: %v", err)
	}
}

func TestLocal_DeleteAllReturnsRunningExecutions(t *testing.T) {
	db, table := localDB(t)
	ctx := context.Background()
	now := time.Now()
	run := claimRec(strings.Repeat("9", 32), "req-9999999999999999", now)
	done := claimRec(strings.Repeat("0", 32), "req-0000000000000000", now)
	done.ClipID = "clip-2"
	for _, r := range []Record{run, done} {
		if err := Claim(ctx, db, table, nil, r, now); err != nil {
			t.Fatal(err)
		}
	}
	if err := Promote(ctx, db, table, "u1", "j1", "clip-2", done.Token, "k", now); err != nil {
		t.Fatal(err)
	}
	names, err := DeleteAll(ctx, db, table, "u1", "j1", now)
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 1 || names[0] != run.ExecutionName {
		t.Fatalf("running executions = %v", names)
	}
	left, _ := List(ctx, db, table, "u1", "j1")
	if len(left) != 0 {
		t.Fatalf("records left: %v", left)
	}
}

// markFailed runs the MarkFailed state of clip_render.json against the table.
func markFailed(db *dynamodb.Client, table, token string) error {
	raw, err := os.ReadFile(filepath.Join("..", "pipeline", "clip_render.json"))
	if err != nil {
		return err
	}
	var def struct {
		States map[string]struct {
			Parameters struct {
				UpdateExpression          string
				ConditionExpression       string
				ExpressionAttributeNames  map[string]string
				ExpressionAttributeValues map[string]map[string]string
			}
		}
	}
	if err := json.Unmarshal(raw, &def); err != nil {
		return err
	}
	p := def.States["MarkFailed"].Parameters
	values := map[string]dbtypes.AttributeValue{}
	for k, v := range p.ExpressionAttributeValues {
		if s, ok := v["S"]; ok {
			values[k] = &dbtypes.AttributeValueMemberS{Value: s}
		} else if path, ok := v["S.$"]; ok && path == "$.token" {
			values[k] = &dbtypes.AttributeValueMemberS{Value: token}
		} else {
			return fmt.Errorf("unexpected value %s: %v", k, v)
		}
	}
	_, err = db.UpdateItem(context.Background(), &dynamodb.UpdateItemInput{
		TableName:                 aws.String(table),
		Key:                       Key("u1", "j1", "clip-1"),
		UpdateExpression:          aws.String(p.UpdateExpression),
		ConditionExpression:       aws.String(p.ConditionExpression),
		ExpressionAttributeNames:  p.ExpressionAttributeNames,
		ExpressionAttributeValues: values,
	})
	return err
}

func runMarkFailed(t *testing.T, db *dynamodb.Client, table, token string) {
	t.Helper()
	if err := markFailed(db, table, token); err != nil {
		t.Fatalf("MarkFailed: %v", err)
	}
}
