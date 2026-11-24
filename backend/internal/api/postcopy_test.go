package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/rahul/indifferent/backend/internal/auth"
	"github.com/rahul/indifferent/backend/internal/models"
	"github.com/rahul/indifferent/backend/internal/storage"
	"github.com/rahul/indifferent/backend/internal/storyjob"
	"github.com/rahul/indifferent/backend/internal/videocopy"
)

// pcDB is a small in-memory DynamoDB: enough for the copy routes, the quota and
// the idempotency record. It understands only the one conditional write the
// code makes, and fails the test on any other.
type pcDB struct {
	t     *testing.T
	mu    sync.Mutex
	items map[string]map[string]dbtypes.AttributeValue
	// putErr and delErr, when set, fail a write whose sort key starts with it.
	putErr map[string]error
	delErr map[string]error
	gets   int
}

func newPCDB(t *testing.T) *pcDB {
	return &pcDB{t: t, items: map[string]map[string]dbtypes.AttributeValue{}, putErr: map[string]error{}, delErr: map[string]error{}}
}

func pcS(v dbtypes.AttributeValue) string {
	if x, ok := v.(*dbtypes.AttributeValueMemberS); ok {
		return x.Value
	}
	return ""
}

func pcN(v dbtypes.AttributeValue) int64 {
	if x, ok := v.(*dbtypes.AttributeValueMemberN); ok {
		n, _ := strconv.ParseInt(x.Value, 10, 64)
		return n
	}
	return 0
}

func pcKey(table string, k map[string]dbtypes.AttributeValue) string {
	return table + "|" + pcS(k["PK"]) + "|" + pcS(k["SK"])
}

func (d *pcDB) put(table string, item map[string]dbtypes.AttributeValue) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.items[pcKey(table, item)] = item
}

func (d *pcDB) get(table, pk, sk string) map[string]dbtypes.AttributeValue {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.items[table+"|"+pk+"|"+sk]
}

func (d *pcDB) count(table, pkPrefix, skPrefix string) int {
	d.mu.Lock()
	defer d.mu.Unlock()
	n := 0
	for k := range d.items {
		parts := strings.SplitN(k, "|", 3)
		if parts[0] == table && strings.HasPrefix(parts[1], pkPrefix) && strings.HasPrefix(parts[2], skPrefix) {
			n++
		}
	}
	return n
}

func (d *pcDB) GetItem(_ context.Context, in *dynamodb.GetItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.gets++
	return &dynamodb.GetItemOutput{Item: d.items[pcKey(*in.TableName, in.Key)]}, nil
}

func (d *pcDB) PutItem(_ context.Context, in *dynamodb.PutItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for prefix, err := range d.putErr {
		if strings.HasPrefix(pcS(in.Item["SK"]), prefix) {
			return nil, err
		}
	}
	k := pcKey(*in.TableName, in.Item)
	if in.ConditionExpression != nil {
		const want = "attribute_not_exists(PK) OR expiresAt < :now OR (#st = :pending AND startedAt < :stale)"
		if *in.ConditionExpression != want {
			d.t.Fatalf("unexpected condition %q", *in.ConditionExpression)
		}
		if cur, ok := d.items[k]; ok {
			live := pcN(cur["expiresAt"]) >= pcN(in.ExpressionAttributeValues[":now"])
			stale := pcS(cur["status"]) == "pending" && pcN(cur["startedAt"]) < pcN(in.ExpressionAttributeValues[":stale"])
			if live && !stale {
				return nil, &dbtypes.ConditionalCheckFailedException{}
			}
		}
	}
	d.items[k] = in.Item
	return &dynamodb.PutItemOutput{}, nil
}

func (d *pcDB) DeleteItem(_ context.Context, in *dynamodb.DeleteItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for prefix, err := range d.delErr {
		if strings.HasPrefix(pcS(in.Key["SK"]), prefix) {
			return nil, err
		}
	}
	delete(d.items, pcKey(*in.TableName, in.Key))
	return &dynamodb.DeleteItemOutput{}, nil
}

func (d *pcDB) Query(_ context.Context, in *dynamodb.QueryInput, _ ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	pk := pcS(in.ExpressionAttributeValues[":pk"])
	prefix := pcS(in.ExpressionAttributeValues[":sk"])
	out := &dynamodb.QueryOutput{}
	for k, it := range d.items {
		parts := strings.SplitN(k, "|", 3)
		if parts[0] == *in.TableName && parts[1] == pk && prefix != "" && strings.HasPrefix(parts[2], prefix) {
			if in.ProjectionExpression != nil {
				out.Items = append(out.Items, map[string]dbtypes.AttributeValue{"PK": it["PK"], "SK": it["SK"]})
			} else {
				out.Items = append(out.Items, it)
			}
		}
	}
	return out, nil
}

func (d *pcDB) UpdateItem(_ context.Context, in *dynamodb.UpdateItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	k := pcKey(*in.TableName, in.Key)
	cur := d.items[k]
	if cur == nil {
		cur = map[string]dbtypes.AttributeValue{"PK": in.Key["PK"], "SK": in.Key["SK"]}
	}
	switch {
	case in.ExpressionAttributeValues[":one"] != nil:
		cur["uses"] = &dbtypes.AttributeValueMemberN{Value: strconv.FormatInt(pcN(cur["uses"])+1, 10)}
		d.items[k] = cur
		return &dynamodb.UpdateItemOutput{Attributes: map[string]dbtypes.AttributeValue{"uses": cur["uses"]}}, nil
	case in.ExpressionAttributeValues[":minus"] != nil:
		if pcN(cur["uses"]) <= 0 {
			return nil, &dbtypes.ConditionalCheckFailedException{}
		}
		cur["uses"] = &dbtypes.AttributeValueMemberN{Value: strconv.FormatInt(pcN(cur["uses"])-1, 10)}
		d.items[k] = cur
	case in.ExpressionAttributeValues[":done"] != nil:
		cur["status"], cur["response"], cur["expiresAt"] = in.ExpressionAttributeValues[":done"], in.ExpressionAttributeValues[":r"], in.ExpressionAttributeValues[":exp"]
		d.items[k] = cur
	default:
		d.t.Fatalf("unexpected update %q", *in.UpdateExpression)
	}
	return &dynamodb.UpdateItemOutput{}, nil
}

type fakePostWriter struct {
	mu    sync.Mutex
	calls int
	reqs  []videocopy.Request
	kinds []videocopy.Kind
	specs [][]videocopy.PlatformSpec
	res   videocopy.PostResult
	err   error
	gate  chan struct{}
}

func (f *fakePostWriter) GeneratePost(ctx context.Context, r videocopy.Request, kind videocopy.Kind, specs []videocopy.PlatformSpec) (videocopy.PostResult, error) {
	f.mu.Lock()
	f.calls++
	f.reqs = append(f.reqs, r)
	f.kinds = append(f.kinds, kind)
	f.specs = append(f.specs, specs)
	res, err, gate := f.res, f.err, f.gate
	f.mu.Unlock()
	if gate != nil {
		select {
		case <-gate:
		case <-ctx.Done():
			return videocopy.PostResult{}, ctx.Err()
		}
	}
	return res, err
}

func (f *fakePostWriter) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func goodPostResult() videocopy.PostResult {
	tags := []string{"#BrassLamp", "#Restoration", "#Wiring"}
	return videocopy.PostResult{
		Platforms: map[videocopy.Platform]videocopy.PlatformCopy{
			videocopy.YouTube:   {Title: "Fixing a brass lamp", Text: "A brass lamp gets a new cable.", Hashtags: tags},
			videocopy.TikTok:    {Text: "New cable, old lamp.", Hashtags: tags},
			videocopy.Instagram: {Text: "A brass lamp, rewired.\nGrandfather would approve.", Hashtags: tags},
		},
		Notes: []string{"TikTok: 1 sentence left out because it was not in the video."},
		Model: "fake-model",
	}
}

const (
	pcUser    = "114373949285061923844"
	pcOther   = "999999999999999999999"
	pcJob     = "job1"
	pcStoryID = "01939f6c-8a40-7e21-b2c4-aa11bb22cc33"
	pcTable   = "table"
	pcQuota   = "sessions"
)

type pcEnv struct {
	t  *testing.T
	h  *APIHandler
	db *pcDB
	w  *fakePostWriter
	s3 *stubDownloader
}

// newPCEnv seeds one finished Shorts job with two clips, each with words, and one
// finished story video. Both belong to pcUser.
func newPCEnv(t *testing.T) *pcEnv {
	t.Helper()
	fixedQuotaClock(t, time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC))
	db := newPCDB(t)
	w := &fakePostWriter{res: goodPostResult()}
	var words []map[string]any
	say := strings.Fields("I replaced the cracked cable on my grandfather's brass lamp and then I fitted a new cloth wire and a safe socket before the lamp lit the room")
	for i := 0; i < 120; i++ {
		words = append(words, map[string]any{"text": say[i%len(say)], "start": float64(i), "end": float64(i) + 0.8})
	}
	tr, _ := json.Marshal(map[string]any{"text": "x", "words": words})
	s3 := &stubDownloader{
		status:  storage.ObjectStatus{StorageClass: "STANDARD"},
		meta:    storage.ObjectMeta{Found: true},
		objects: map[string][]byte{"shorts/" + pcUser + "/job1/transcript.json": tr},
	}
	h := &APIHandler{
		AuthService:    &mockAuthService{},
		JWTService:     &auth.JWTService{Secret: testSecret, DB: db, SessionTable: pcQuota},
		DB:             db,
		S3:             s3,
		TableName:      pcTable,
		QuotaTable:     pcQuota,
		Bucket:         "bucket",
		PostCopyWriter: w,
	}
	e := &pcEnv{t: t, h: h, db: db, w: w, s3: s3}
	e.seedJob(pcUser, nil)
	e.seedStory(pcUser, storyjob.StatusCompleted)
	return e
}

func (e *pcEnv) seedJob(user string, edits map[string]models.ClipEdit) {
	j := models.ShortsJob{
		UserID: user, JobID: pcJob, Status: "completed", FileType: "mp4", SourceDuration: 120,
		TranscriptKey: "shorts/" + pcUser + "/job1/transcript.json",
		Clips: []models.Clip{
			{ClipID: "clip-1", Rank: 1, Duration: 30, Start: 0, End: 30, HookText: "hook one"},
			{ClipID: "clip-2", Rank: 2, Duration: 30, Start: 60, End: 90, HookText: "hook two"},
		},
		ClipEdits: edits,
	}
	e.db.put(pcTable, shortsJobToItem(j))
}

func (e *pcEnv) seedStory(user, status string) {
	j := storyjob.Job{
		UserID: user, JobID: pcStoryID, PlanID: "01939f6c-8a40-7e21-b2c4-aa11bb22cc44", PlanRevision: 1, Status: status,
		Language: "en", Voice: "Joanna", TotalSeconds: 22,
		Scenes: []storyjob.Scene{
			{Narration: "The fisherman rows quietly across the dark water."},
			{Narration: "He lifts the net and finds it full of silver fish."},
		},
		CreatedAt: "2026-11-20T09:00:00Z", UpdatedAt: "2026-11-20T09:05:00Z",
	}
	item, err := storyjob.ToItem(j)
	if err != nil {
		e.t.Fatal(err)
	}
	e.db.put(pcTable, item)
}

func (e *pcEnv) do(method, path, body, user string) events.APIGatewayProxyResponse {
	headers := map[string]string{}
	if user != "" {
		headers["Authorization"] = "Bearer " + generateTestToken(user)
	}
	resp, _ := e.h.HandleRequest(context.Background(), events.APIGatewayProxyRequest{HTTPMethod: method, Path: path, Body: body, Headers: headers})
	return resp
}

const (
	clipPath  = "/shorts/job1/clips/clip-1/post-copy"
	clip2Path = "/shorts/job1/clips/clip-2/post-copy"
	storyPath = "/story-videos/" + pcStoryID + "/post-copy"
)

func rid(n int) string { return fmt.Sprintf("request-id-%016d", n) }

func (e *pcEnv) gen(path, user string, n int) events.APIGatewayProxyResponse {
	return e.do("POST", path, `{"requestId":"`+rid(n)+`"}`, user)
}

func (e *pcEnv) used() int64 {
	it := e.db.get(pcQuota, "QUOTA#"+pcUser, quotaCopyHelper.name+"#20261120")
	if it == nil {
		return 0
	}
	return pcN(it["uses"])
}

func decodePC(t *testing.T, resp events.APIGatewayProxyResponse) PostCopyResponse {
	t.Helper()
	if resp.StatusCode != 200 {
		t.Fatalf("status %d: %s", resp.StatusCode, resp.Body)
	}
	var out PostCopyResponse
	if err := json.Unmarshal([]byte(resp.Body), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

// --- routing ---------------------------------------------------------------

func TestParsePostCopyPath(t *testing.T) {
	good := map[string]postCopyTarget{
		"/shorts/j1/clips/c1/post-copy": {kind: videocopy.KindClip, jobID: "j1", clipID: "c1"},
		storyPath:                       {kind: videocopy.KindStory, jobID: pcStoryID},
		"/story-videos/" + strings.ToUpper(pcStoryID) + "/post-copy": {kind: videocopy.KindStory, jobID: pcStoryID},
	}
	for path, want := range good {
		if got, ok := parsePostCopyPath(path); !ok || got != want {
			t.Errorf("%s = %+v %v", path, got, ok)
		}
	}
	for _, path := range []string{
		"/shorts/j1/clips/c1/post-copyx", "/shorts//clips/c1/post-copy", "/shorts/j1/clips//post-copy", "/shorts/j1/post-copy",
		"/shorts/j#1/clips/c1/post-copy", "/shorts/j1/clips/c#1/post-copy", "/story-videos/post-copy", "/story-videos/not-a-uuid/post-copy",
		"/story-videos/" + pcStoryID + "/post-copy/x", "/projects/j1/clips/c1/post-copy",
	} {
		if _, ok := parsePostCopyPath(path); ok {
			t.Errorf("%s should not match", path)
		}
	}
	if got := (postCopyTarget{kind: videocopy.KindClip, jobID: "j1", clipID: "c1"}).sk(); got != "POSTCOPY#CLIP#j1#c1" {
		t.Errorf("clip sk %q", got)
	}
	if got := (postCopyTarget{kind: videocopy.KindStory, jobID: "abc"}).sk(); got != "POSTCOPY#STORY#abc" {
		t.Errorf("story sk %q", got)
	}
}

// --- generating ------------------------------------------------------------

func TestPostCopy_GeneratesForAClipFromItsOwnWords(t *testing.T) {
	e := newPCEnv(t)
	out := decodePC(t, e.gen(clipPath, pcUser, 1))
	if len(out.Copy) != 3 || out.Source != "generated" || out.Replayed || out.GeneratedAt == "" {
		t.Errorf("out = %+v", out)
	}
	if out.RemainingToday == nil || *out.RemainingToday != 19 || out.DailyLimit != 20 {
		t.Errorf("remaining %v limit %d", out.RemainingToday, out.DailyLimit)
	}
	if e.used() != 1 {
		t.Errorf("used %d", e.used())
	}
	if len(e.w.reqs) != 1 || e.w.kinds[0] != videocopy.KindClip {
		t.Fatalf("calls %+v", e.w.kinds)
	}
	// Only the words inside the clip's window reach the model.
	words := e.w.reqs[0].Words
	if len(words) == 0 {
		t.Fatal("no words")
	}
	for _, w := range words {
		if w.End < 0 || w.Start > 30 {
			t.Errorf("a word outside the clip: %+v", w)
		}
	}
	if len(e.w.specs[0]) != 3 {
		t.Errorf("platforms asked for: %d", len(e.w.specs[0]))
	}
	// The second clip reads its own window.
	e.gen(clip2Path, pcUser, 2)
	for _, w := range e.w.reqs[1].Words {
		if w.End < 60 || w.Start > 90 {
			t.Errorf("clip 2 got a word outside its window: %+v", w)
		}
	}
}

func TestPostCopy_ClipEditedWordsAreUsedWhenThereAreSome(t *testing.T) {
	e := newPCEnv(t)
	e.seedJob(pcUser, map[string]models.ClipEdit{"clip-1": {Start: 0, End: 20, Words: []models.Word{
		{Text: "corrected", Start: 0, End: 1}, {Text: "words", Start: 1, End: 2}, {Text: "about", Start: 2, End: 3}, {Text: "a", Start: 3, End: 4}, {Text: "lamp", Start: 4, End: 5}, {Text: "repair", Start: 5, End: 6},
	}}})
	decodePC(t, e.gen(clipPath, pcUser, 1))
	got := ""
	for _, w := range e.w.reqs[0].Words {
		got += w.Text + " "
	}
	if got != "corrected words about a lamp repair " {
		t.Errorf("words = %q", got)
	}
	if e.w.reqs[0].Duration != 20 {
		t.Errorf("duration %v", e.w.reqs[0].Duration)
	}
}

func TestPostCopy_StoryUsesTheNarrationAndNeedsAFinishedVideo(t *testing.T) {
	e := newPCEnv(t)
	out := decodePC(t, e.gen(storyPath, pcUser, 1))
	if len(out.Copy) != 3 || e.w.kinds[0] != videocopy.KindStory {
		t.Errorf("%+v %v", out, e.w.kinds)
	}
	want := "The fisherman rows quietly across the dark water. He lifts the net and finds it full of silver fish."
	if e.w.reqs[0].Transcript != want {
		t.Errorf("transcript %q", e.w.reqs[0].Transcript)
	}
	e.seedStory(pcUser, storyjob.StatusRendering)
	resp := e.gen(storyPath, pcUser, 2)
	if resp.StatusCode != 409 || errCode(t, resp) != "NOT_READY" || e.w.callCount() != 1 {
		t.Errorf("a video still being made: %d %s", resp.StatusCode, resp.Body)
	}
}

func TestPostCopy_StoredWithItsClipAndReadBack(t *testing.T) {
	e := newPCEnv(t)
	decodePC(t, e.gen(clipPath, pcUser, 1))
	if e.db.get(pcTable, "USER#"+pcUser, "POSTCOPY#CLIP#job1#clip-1") == nil {
		t.Fatal("no record beside the clip")
	}
	if e.db.get(pcTable, "USER#"+pcUser, "POSTCOPY#CLIP#job1#clip-2") != nil {
		t.Error("another clip got a record")
	}
	var view PostCopyView
	if err := json.Unmarshal([]byte(e.do("GET", clipPath, "", pcUser).Body), &view); err != nil {
		t.Fatal(err)
	}
	if !view.Enabled || view.Saved == nil || view.Saved.Source != "generated" || view.Saved.Copy[videocopy.YouTube].Title != "Fixing a brass lamp" {
		t.Errorf("view %+v", view)
	}
	if len(view.Platforms) != 3 || view.Platforms[0].TextMax != 5000 || view.Platforms[0].TextUnit != videocopy.UnitBytes {
		t.Errorf("platform limits are not served: %+v", view.Platforms)
	}
	if view.RemainingToday == nil || *view.RemainingToday != 19 {
		t.Errorf("remaining %v", view.RemainingToday)
	}
	// Clip 2 has nothing yet.
	var none PostCopyView
	_ = json.Unmarshal([]byte(e.do("GET", clip2Path, "", pcUser).Body), &none)
	if none.Saved != nil {
		t.Error("clip 2 should have nothing saved")
	}
}

// --- quota and retries -----------------------------------------------------

func TestPostCopy_ARetryWithTheSameIdUsesNothing(t *testing.T) {
	e := newPCEnv(t)
	first := decodePC(t, e.gen(clipPath, pcUser, 1))
	second := decodePC(t, e.gen(clipPath, pcUser, 1))
	third := decodePC(t, e.gen(clipPath, pcUser, 1))
	if e.w.callCount() != 1 || e.used() != 1 {
		t.Fatalf("model calls %d, quota used %d: a retry must not spend", e.w.callCount(), e.used())
	}
	if first.Replayed || !second.Replayed || !third.Replayed {
		t.Errorf("replayed flags %v %v %v", first.Replayed, second.Replayed, third.Replayed)
	}
	if second.GeneratedAt != first.GeneratedAt || len(second.Copy) != 3 {
		t.Errorf("a replay must return the same copy")
	}
	if second.RemainingToday == nil || *second.RemainingToday != 19 {
		t.Errorf("remaining %v", second.RemainingToday)
	}
}

func TestPostCopy_ANewIdIsANewUse(t *testing.T) {
	e := newPCEnv(t)
	e.gen(clipPath, pcUser, 1)
	e.gen(clipPath, pcUser, 2)
	if e.w.callCount() != 2 || e.used() != 2 {
		t.Errorf("calls %d used %d", e.w.callCount(), e.used())
	}
}

func TestPostCopy_ADuplicateWhileTheFirstIsBeingWrittenWaits(t *testing.T) {
	e := newPCEnv(t)
	e.w.gate = make(chan struct{})
	var wg sync.WaitGroup
	var first events.APIGatewayProxyResponse
	wg.Add(1)
	go func() { defer wg.Done(); first = e.gen(clipPath, pcUser, 1) }()
	for i := 0; i < 200 && e.w.callCount() == 0; i++ {
		time.Sleep(5 * time.Millisecond)
	}
	if e.w.callCount() != 1 {
		t.Fatal("the first request never reached the model")
	}
	dup := e.gen(clipPath, pcUser, 1)
	if dup.StatusCode != 409 || errCode(t, dup) != "IN_PROGRESS" {
		t.Errorf("duplicate: %d %s", dup.StatusCode, dup.Body)
	}
	close(e.w.gate)
	wg.Wait()
	decodePC(t, first)
	again := decodePC(t, e.gen(clipPath, pcUser, 1))
	if !again.Replayed || e.w.callCount() != 1 || e.used() != 1 {
		t.Errorf("replayed %v calls %d used %d", again.Replayed, e.w.callCount(), e.used())
	}
}

func TestPostCopy_ADeadAttemptDoesNotBlockForever(t *testing.T) {
	e := newPCEnv(t)
	// An attempt that claimed the id and never finished.
	now := quotaNow().UTC()
	claimed, err := e.h.claimIdem(context.Background(), pcUser, rid(1), "POSTCOPY#CLIP#job1#clip-1", now.Add(-5*time.Minute))
	if err != nil || !claimed {
		t.Fatal(err)
	}
	decodePC(t, e.gen(clipPath, pcUser, 1))
	if e.w.callCount() != 1 || e.used() != 1 {
		t.Errorf("calls %d used %d", e.w.callCount(), e.used())
	}
}

func TestPostCopy_ReusingAnIdForAnotherTargetIsRefused(t *testing.T) {
	e := newPCEnv(t)
	e.gen(clipPath, pcUser, 1)
	resp := e.gen(clip2Path, pcUser, 1)
	if resp.StatusCode != 409 || errCode(t, resp) != "REQUEST_ID_REUSED" || e.w.callCount() != 1 || e.used() != 1 {
		t.Errorf("%d %s calls %d used %d", resp.StatusCode, resp.Body, e.w.callCount(), e.used())
	}
}

func TestPostCopy_AFailureCostsNothingAndTheSameIdCanTryAgain(t *testing.T) {
	for name, err := range map[string]error{
		"model failed":   fmt.Errorf("%w: busy", videocopy.ErrModelFailed),
		"model timeout":  fmt.Errorf("%w: %w", videocopy.ErrModelTimeout, videocopy.ErrModelFailed),
		"unusable reply": videocopy.ErrInvalidAnswer,
	} {
		t.Run(name, func(t *testing.T) {
			e := newPCEnv(t)
			e.w.err = err
			resp := e.gen(clipPath, pcUser, 1)
			if resp.StatusCode < 500 || e.used() != 0 {
				t.Fatalf("status %d used %d", resp.StatusCode, e.used())
			}
			if e.db.get(pcTable, "USER#"+pcUser, "POSTCOPY#CLIP#job1#clip-1") != nil {
				t.Error("a failed try left a record")
			}
			if e.db.count(pcQuota, "COPYREQ#", "") != 0 {
				t.Error("a failed try left its id claimed")
			}
			e.w.mu.Lock()
			e.w.err = nil
			e.w.mu.Unlock()
			out := decodePC(t, e.gen(clipPath, pcUser, 1))
			if out.Replayed || e.used() != 1 {
				t.Errorf("replayed %v used %d", out.Replayed, e.used())
			}
		})
	}
}

func TestPostCopy_NotStoringIsNotASuccess(t *testing.T) {
	e := newPCEnv(t)
	e.db.putErr["POSTCOPY#"] = errors.New("dynamo down")
	resp := e.gen(clipPath, pcUser, 1)
	if resp.StatusCode != 500 || e.used() != 0 || e.db.count(pcQuota, "COPYREQ#", "") != 0 {
		t.Errorf("status %d used %d", resp.StatusCode, e.used())
	}
}

func TestPostCopy_QuotaIsSharedAndAnExhaustedOneRefuses(t *testing.T) {
	e := newPCEnv(t)
	for i := 0; i < 20; i++ {
		decodePC(t, e.gen(clipPath, pcUser, i+1))
	}
	resp := e.gen(clip2Path, pcUser, 99)
	if resp.StatusCode != 429 || e.w.callCount() != 20 {
		t.Errorf("status %d calls %d", resp.StatusCode, e.w.callCount())
	}
	// The refused request is not left claimed, and a retry of an old, finished
	// one still replays for free.
	if e.db.get(pcQuota, "COPYREQ#"+pcUser, rid(99)) != nil {
		t.Error("a refused request stayed claimed")
	}
	if r := decodePC(t, e.gen(clipPath, pcUser, 3)); !r.Replayed {
		t.Error("a finished request should still replay when the day is used up")
	}
	// The helper's own use counts against the same allowance.
	e2 := newPCEnv(t)
	for i := 0; i < 19; i++ {
		e2.h.spendQuotaCount(context.Background(), pcUser, quotaCopyHelper)
	}
	decodePC(t, e2.gen(clipPath, pcUser, 1))
	if resp := e2.gen(clip2Path, pcUser, 2); resp.StatusCode != 429 {
		t.Errorf("the quota is not shared with the helper: %d", resp.StatusCode)
	}
}

func TestPostCopy_RequestIdIsRequired(t *testing.T) {
	e := newPCEnv(t)
	for _, body := range []string{`{}`, `{"requestId":""}`, `{"requestId":"short"}`, `{"requestId":"has spaces in it 0123456789"}`, `{"requestId":"` + strings.Repeat("a", 65) + `"}`, `not json`} {
		if resp := e.do("POST", clipPath, body, pcUser); resp.StatusCode != 400 {
			t.Errorf("%q: %d", body, resp.StatusCode)
		}
	}
	if e.w.callCount() != 0 || e.used() != 0 {
		t.Error("a bad request reached the model or spent a use")
	}
}

func TestPostCopy_TooLittleToWriteCostsNothing(t *testing.T) {
	e := newPCEnv(t)
	e.seedJob(pcUser, map[string]models.ClipEdit{"clip-1": {Start: 0, End: 5, Words: []models.Word{{Text: "hi", Start: 0, End: 1}}}})
	resp := e.gen(clipPath, pcUser, 1)
	if resp.StatusCode != 400 || e.w.callCount() != 0 || e.used() != 0 || !strings.Contains(resp.Body, "too little spoken text") {
		t.Errorf("%d %s", resp.StatusCode, resp.Body)
	}
}

// --- ownership and switches ------------------------------------------------

func TestPostCopy_OtherPeopleCannotReachIt(t *testing.T) {
	e := newPCEnv(t)
	decodePC(t, e.gen(clipPath, pcUser, 1))
	decodePC(t, e.gen(storyPath, pcUser, 2))
	for _, path := range []string{clipPath, storyPath} {
		for _, m := range []string{"GET", "POST", "PUT"} {
			body := `{"requestId":"` + rid(50) + `"}`
			if m == "PUT" {
				body = putBody(t, validCopy())
			}
			if resp := e.do(m, path, body, pcOther); resp.StatusCode != 404 {
				t.Errorf("%s %s by another user: %d %s", m, path, resp.StatusCode, resp.Body)
			}
		}
	}
	if resp := e.do("GET", clipPath, "", ""); resp.StatusCode != 401 {
		t.Errorf("no token: %d", resp.StatusCode)
	}
	// A clip that is not in the job is not found.
	if resp := e.gen("/shorts/job1/clips/clip-9/post-copy", pcUser, 51); resp.StatusCode != 404 {
		t.Errorf("unknown clip: %d", resp.StatusCode)
	}
	if e.w.callCount() != 2 {
		t.Errorf("a refused request reached the model: %d", e.w.callCount())
	}
}

func TestPostCopy_OffUntilSwitchedOn(t *testing.T) {
	e := newPCEnv(t)
	e.h.PostCopyWriter = nil
	var view PostCopyView
	resp := e.do("GET", clipPath, "", pcUser)
	if resp.StatusCode != 200 {
		t.Fatalf("%d", resp.StatusCode)
	}
	_ = json.Unmarshal([]byte(resp.Body), &view)
	if view.Enabled || view.Saved != nil {
		t.Errorf("view %+v", view)
	}
	e.db.gets = 0
	for _, m := range []string{"POST", "PUT"} {
		body := `{"requestId":"` + rid(1) + `"}`
		if m == "PUT" {
			body = putBody(t, validCopy())
		}
		if r := e.do(m, clipPath, body, pcUser); r.StatusCode != 503 {
			t.Errorf("%s: %d", m, r.StatusCode)
		}
	}
	if e.used() != 0 {
		t.Error("spent a use while off")
	}
}

// --- saving an edit --------------------------------------------------------

func validCopy() map[videocopy.Platform]videocopy.PlatformCopy {
	return map[videocopy.Platform]videocopy.PlatformCopy{
		videocopy.YouTube:   {Title: "My own title", Text: "My own description.\n\nSecond paragraph with a link https://example.com/a kept whole.", Hashtags: []string{"#BrassLamp", "#दीपक"}},
		videocopy.TikTok:    {Text: "My caption.", Hashtags: []string{}},
		videocopy.Instagram: {Text: "Another caption.", Hashtags: []string{"#Lamp"}},
	}
}

func putBody(t *testing.T, c map[videocopy.Platform]videocopy.PlatformCopy) string {
	t.Helper()
	b, err := json.Marshal(PostCopySaveRequest{Copy: c})
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestPostCopy_SavingAnEditKeepsExactlyWhatWasSent(t *testing.T) {
	e := newPCEnv(t)
	gen := decodePC(t, e.gen(clipPath, pcUser, 1))
	resp := e.do("PUT", clipPath, putBody(t, validCopy()), pcUser)
	if resp.StatusCode != 200 {
		t.Fatalf("%d %s", resp.StatusCode, resp.Body)
	}
	var view PostCopyView
	_ = json.Unmarshal([]byte(e.do("GET", clipPath, "", pcUser).Body), &view)
	if view.Saved == nil || view.Saved.Source != "edited" || view.Saved.GeneratedAt != gen.GeneratedAt {
		t.Fatalf("saved %+v", view.Saved)
	}
	want := validCopy()
	for id, c := range want {
		if got := view.Saved.Copy[id]; got.Title != c.Title || got.Text != c.Text || strings.Join(got.Hashtags, " ") != strings.Join(c.Hashtags, " ") {
			t.Errorf("%s changed in saving: %+v", id, got)
		}
	}
	// Saving does not use the daily allowance.
	if e.used() != 1 {
		t.Errorf("used %d", e.used())
	}
	// A copy can be written by hand for a clip that has none yet.
	if r := e.do("PUT", clip2Path, putBody(t, validCopy()), pcUser); r.StatusCode != 200 {
		t.Errorf("hand-written copy: %d %s", r.StatusCode, r.Body)
	}
}

func TestPostCopy_SavingRefusesWhatIsOverALimitAndChangesNothing(t *testing.T) {
	e := newPCEnv(t)
	decodePC(t, e.gen(clipPath, pcUser, 1))
	before := e.db.get(pcTable, "USER#"+pcUser, "POSTCOPY#CLIP#job1#clip-1")
	over := func(mut func(map[videocopy.Platform]videocopy.PlatformCopy)) string {
		c := validCopy()
		mut(c)
		return putBody(t, c)
	}
	cases := map[string]string{
		"title over 100": over(func(c map[videocopy.Platform]videocopy.PlatformCopy) {
			x := c[videocopy.YouTube]
			x.Title = strings.Repeat("a", 101)
			c[videocopy.YouTube] = x
		}),
		"description bytes": over(func(c map[videocopy.Platform]videocopy.PlatformCopy) {
			x := c[videocopy.YouTube]
			x.Text = strings.Repeat("दीपक ", 900)
			c[videocopy.YouTube] = x
		}),
		"tiktok over 2200": over(func(c map[videocopy.Platform]videocopy.PlatformCopy) {
			x := c[videocopy.TikTok]
			x.Text = strings.Repeat("a", 2201)
			c[videocopy.TikTok] = x
		}),
		"six hashtags": over(func(c map[videocopy.Platform]videocopy.PlatformCopy) {
			x := c[videocopy.Instagram]
			x.Hashtags = []string{"#a1", "#b2", "#c3", "#d4", "#e5", "#f6"}
			c[videocopy.Instagram] = x
		}),
		"bad hashtag": over(func(c map[videocopy.Platform]videocopy.PlatformCopy) {
			x := c[videocopy.Instagram]
			x.Hashtags = []string{"lamp"}
			c[videocopy.Instagram] = x
		}),
		"a platform missing": over(func(c map[videocopy.Platform]videocopy.PlatformCopy) { delete(c, videocopy.TikTok) }),
		"an unknown platform": over(func(c map[videocopy.Platform]videocopy.PlatformCopy) {
			c["facebook"] = videocopy.PlatformCopy{Text: "x"}
		}),
		"no body":   `{}`,
		"not json":  `nope`,
		"too large": `{"copy":{"x":"` + strings.Repeat("a", postCopyMaxBody) + `"}}`,
	}
	for name, body := range cases {
		if r := e.do("PUT", clipPath, body, pcUser); r.StatusCode != 400 {
			t.Errorf("%s: %d %s", name, r.StatusCode, r.Body)
		}
	}
	r := e.do("PUT", clipPath, cases["description bytes"], pcUser)
	if !strings.Contains(r.Body, "YouTube") || !strings.Contains(r.Body, "5,000 bytes") {
		t.Errorf("the message should name the platform and the limit: %s", r.Body)
	}
	after := e.db.get(pcTable, "USER#"+pcUser, "POSTCOPY#CLIP#job1#clip-1")
	if pcS(before["updatedAt"]) != pcS(after["updatedAt"]) || pcS(after["source"]) != "generated" {
		t.Error("a refused save changed the stored copy")
	}
}

// --- deleting --------------------------------------------------------------

func TestPostCopy_DeletedWithTheShortsJob(t *testing.T) {
	e := newPCEnv(t)
	decodePC(t, e.gen(clipPath, pcUser, 1))
	decodePC(t, e.gen(clip2Path, pcUser, 2))
	decodePC(t, e.gen(storyPath, pcUser, 3))
	if err := e.h.removeShortsJob(context.Background(), pcUser, pcJob); err != nil {
		t.Fatal(err)
	}
	if n := e.db.count(pcTable, "USER#"+pcUser, "POSTCOPY#CLIP#job1#"); n != 0 {
		t.Errorf("%d clip copies left behind", n)
	}
	if e.db.get(pcTable, "USER#"+pcUser, "SHORTS#job1") != nil {
		t.Error("the job is still there")
	}
	if e.db.get(pcTable, "USER#"+pcUser, "POSTCOPY#STORY#"+pcStoryID) == nil {
		t.Error("deleting a Shorts job removed a story video's copy")
	}
}

func TestPostCopy_AFailedCopyDeleteKeepsTheJobSoItCanBeRetried(t *testing.T) {
	e := newPCEnv(t)
	decodePC(t, e.gen(clipPath, pcUser, 1))
	e.db.delErr["POSTCOPY#"] = errors.New("dynamo down")
	if err := e.h.removeShortsJob(context.Background(), pcUser, pcJob); err == nil {
		t.Fatal("a failed copy delete must fail the delete")
	}
	if e.db.get(pcTable, "USER#"+pcUser, "SHORTS#job1") == nil {
		t.Error("the job record was removed while its copy remained")
	}
	delete(e.db.delErr, "POSTCOPY#")
	if err := e.h.removeShortsJob(context.Background(), pcUser, pcJob); err != nil {
		t.Fatal(err)
	}
	if e.db.count(pcTable, "USER#"+pcUser, "POSTCOPY#") != 0 {
		t.Error("copy left after the retry")
	}
}

func TestPostCopy_DeletedWithTheStoryVideo(t *testing.T) {
	e := newPCEnv(t)
	decodePC(t, e.gen(storyPath, pcUser, 1))
	decodePC(t, e.gen(clipPath, pcUser, 2))
	if resp := e.do("DELETE", "/story-videos/"+pcStoryID, "", pcUser); resp.StatusCode != 204 {
		t.Fatalf("%d %s", resp.StatusCode, resp.Body)
	}
	if e.db.get(pcTable, "USER#"+pcUser, "POSTCOPY#STORY#"+pcStoryID) != nil {
		t.Error("the story copy was left behind")
	}
	if e.db.get(pcTable, "USER#"+pcUser, "POSTCOPY#CLIP#job1#clip-1") == nil {
		t.Error("deleting a story video removed a clip's copy")
	}
	// A failed copy delete keeps the video so it can be deleted again.
	e.seedStory(pcUser, storyjob.StatusCompleted)
	decodePC(t, e.gen(storyPath, pcUser, 3))
	e.db.delErr["POSTCOPY#"] = errors.New("dynamo down")
	if resp := e.do("DELETE", "/story-videos/"+pcStoryID, "", pcUser); resp.StatusCode != 500 {
		t.Errorf("%d", resp.StatusCode)
	}
	storyItem, _ := storyjob.ToItem(storyjob.Job{UserID: pcUser, JobID: pcStoryID})
	if e.db.get(pcTable, pcS(storyItem["PK"]), pcS(storyItem["SK"])) == nil {
		t.Error("the story record was removed while its copy remained")
	}
}

func TestPostCopy_ListForAJobTellsWhichClipsHaveCopy(t *testing.T) {
	e := newPCEnv(t)
	listPath := "/shorts/job1/post-copy"
	var none PostCopyListView
	if err := json.Unmarshal([]byte(e.do("GET", listPath, "", pcUser).Body), &none); err != nil || !none.Enabled || len(none.Saved) != 0 || len(none.Platforms) != 3 {
		t.Fatalf("nothing saved yet: %+v %v", none, err)
	}
	decodePC(t, e.gen(clip2Path, pcUser, 1))
	e.do("PUT", clip2Path, putBody(t, validCopy()), pcUser)
	decodePC(t, e.gen(storyPath, pcUser, 2))
	var view PostCopyListView
	resp := e.do("GET", listPath, "", pcUser)
	if err := json.Unmarshal([]byte(resp.Body), &view); err != nil {
		t.Fatal(err)
	}
	if len(view.Saved) != 1 || view.Saved["clip-2"].Source != "edited" || view.Saved["clip-2"].Copy[videocopy.TikTok].Text != "My caption." {
		t.Errorf("saved %+v", view.Saved)
	}
	if _, ok := view.Saved["clip-1"]; ok {
		t.Error("clip 1 has nothing saved")
	}
	if view.RemainingToday == nil || *view.RemainingToday != 18 {
		t.Errorf("remaining %v", view.RemainingToday)
	}
	// Another person's job is not found; a job that is not there is not found.
	if r := e.do("GET", listPath, "", pcOther); r.StatusCode != 404 {
		t.Errorf("another user: %d", r.StatusCode)
	}
	if r := e.do("GET", "/shorts/nojob/post-copy", "", pcUser); r.StatusCode != 404 {
		t.Errorf("unknown job: %d", r.StatusCode)
	}
	if r := e.do("POST", listPath, "{}", pcUser); r.StatusCode != 405 {
		t.Errorf("POST: %d", r.StatusCode)
	}
	// Off: it says so and reads nothing.
	e.h.PostCopyWriter = nil
	var off PostCopyListView
	_ = json.Unmarshal([]byte(e.do("GET", listPath, "", pcUser).Body), &off)
	if off.Enabled || len(off.Saved) != 0 {
		t.Errorf("off: %+v", off)
	}
	for _, path := range []string{"/shorts/post-copy", "/shorts//post-copy", "/shorts/j#1/post-copy", "/shorts/j1/post-copy/x"} {
		if isPostCopyListPath(path) {
			t.Errorf("%s should not match", path)
		}
	}
}

// Writing to a clip's copy must not be mistaken for editing the clip.
func TestPostCopy_PutIsNotTheClipEdit(t *testing.T) {
	e := newPCEnv(t)
	if r := e.do("PUT", clipPath, putBody(t, validCopy()), pcUser); r.StatusCode != 200 {
		t.Fatalf("%d %s", r.StatusCode, r.Body)
	}
	job := e.db.get(pcTable, "USER#"+pcUser, "SHORTS#job1")
	if strings.Contains(fmt.Sprint(job["clipEdits"]), "clip-1") {
		t.Error("saving a copy changed the clip's edits")
	}
}
