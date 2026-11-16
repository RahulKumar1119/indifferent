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
	"github.com/rahul/indifferent/backend/internal/storage"
	"github.com/rahul/indifferent/backend/internal/videocopy"
)

type fakeCopyWriter struct {
	mu    sync.Mutex
	reqs  []videocopy.Request
	res   videocopy.Result
	err   error
	gate  chan struct{}
	calls int
}

func (f *fakeCopyWriter) Generate(ctx context.Context, r videocopy.Request) (videocopy.Result, error) {
	f.mu.Lock()
	f.calls++
	f.reqs = append(f.reqs, r)
	res, err, gate := f.res, f.err, f.gate
	f.mu.Unlock()
	if gate != nil {
		select {
		case <-gate:
		case <-ctx.Done():
			return videocopy.Result{}, ctx.Err()
		}
	}
	return res, err
}
func (f *fakeCopyWriter) Model() string { return "fake" }

func copyResult() videocopy.Result {
	titles := make([]string, 10)
	for i := range titles {
		titles[i] = fmt.Sprintf("Title idea %d", i+1)
	}
	return videocopy.Result{
		Copy: videocopy.Copy{
			Titles:      titles,
			Description: "A short description.",
			Chapters: []videocopy.Chapter{
				{Start: 0, Time: "0:00", Title: "Start"}, {Start: 60, Time: "1:00", Title: "Middle"}, {Start: 120, Time: "2:00", Title: "End"},
			},
			DescriptionWithChapters: "A short description.\n\n0:00 Start\n1:00 Middle\n2:00 End",
		},
		HasTimestamps: true,
		Model:         "fake",
	}
}

type copyEnv struct {
	*thumbEnv
	w      *fakeCopyWriter
	mu     sync.Mutex
	counts map[string]int64
	s3     *stubDownloader
}

func newCopyEnv(t *testing.T) *copyEnv {
	t.Helper()
	fixedQuotaClock(t, time.Date(2026, 11, 16, 10, 0, 0, 0, time.UTC))
	te := newThumbEnv(t, "completed", "mp4", nil)
	e := &copyEnv{thumbEnv: te, w: &fakeCopyWriter{res: copyResult()}, counts: map[string]int64{}}
	e.s3 = &stubDownloader{
		status:  storage.ObjectStatus{StorageClass: "STANDARD"},
		meta:    storage.ObjectMeta{Found: true},
		objects: map[string][]byte{"shorts/user1/job1/transcript.json": transcriptJSON(5)},
	}
	te.h.S3 = e.s3
	te.h.CopyWriter = e.w
	te.h.QuotaTable = "sessions-table"
	db := te.h.DB.(*mockDynamoDB)
	prevGet := db.getItemFunc
	db.getItemFunc = func(ctx context.Context, in *dynamodb.GetItemInput, o ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
		if pk, _ := in.Key["PK"].(*dbtypes.AttributeValueMemberS); pk != nil && strings.HasPrefix(pk.Value, "QUOTA#") {
			e.mu.Lock()
			defer e.mu.Unlock()
			sk := in.Key["SK"].(*dbtypes.AttributeValueMemberS).Value
			n, ok := e.counts[pk.Value+"|"+sk]
			if !ok {
				return &dynamodb.GetItemOutput{}, nil
			}
			return &dynamodb.GetItemOutput{Item: map[string]dbtypes.AttributeValue{"uses": &dbtypes.AttributeValueMemberN{Value: strconv.FormatInt(n, 10)}}}, nil
		}
		return prevGet(ctx, in, o...)
	}
	db.updateItemFunc = func(_ context.Context, in *dynamodb.UpdateItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
		e.mu.Lock()
		defer e.mu.Unlock()
		k := in.Key["PK"].(*dbtypes.AttributeValueMemberS).Value + "|" + in.Key["SK"].(*dbtypes.AttributeValueMemberS).Value
		switch {
		case in.ExpressionAttributeValues[":one"] != nil:
			e.counts[k]++
			return &dynamodb.UpdateItemOutput{Attributes: map[string]dbtypes.AttributeValue{"uses": &dbtypes.AttributeValueMemberN{Value: strconv.FormatInt(e.counts[k], 10)}}}, nil
		case in.ExpressionAttributeValues[":minus"] != nil:
			if e.counts[k] <= 0 {
				return nil, &dbtypes.ConditionalCheckFailedException{}
			}
			e.counts[k]--
		}
		return &dynamodb.UpdateItemOutput{}, nil
	}
	return e
}

func (e *copyEnv) used(user string) int64 {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.counts["QUOTA#"+user+"|"+quotaCopyHelper.name+"#20261116"]
}

func transcriptJSON(minutes int) []byte {
	var words []map[string]any
	for m := 0; m < minutes; m++ {
		for s := 0; s < 60; s += 10 {
			t := float64(m*60 + s)
			words = append(words, map[string]any{"text": "word", "start": t, "end": t + 1})
		}
	}
	b, _ := json.Marshal(map[string]any{"text": "word word", "words": words})
	return b
}

func (e *copyEnv) post(t *testing.T, user string, body any) events.APIGatewayProxyResponse {
	t.Helper()
	raw, _ := json.Marshal(body)
	return e.do("POST", "/shorts/job1/copy", string(raw), user)
}

func decodeCopy(t *testing.T, resp events.APIGatewayProxyResponse) CopyResponse {
	t.Helper()
	if resp.StatusCode != 200 {
		t.Fatalf("status %d: %s", resp.StatusCode, resp.Body)
	}
	var out CopyResponse
	if err := json.Unmarshal([]byte(resp.Body), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestParseCopyPath(t *testing.T) {
	for path, want := range map[string]copyRoute{
		"/shorts/j1/copy":        {jobID: "j1"},
		"/shorts/j1/copy/status": {jobID: "j1", status: true},
	} {
		if got, ok := parseCopyPath(path); !ok || got != want {
			t.Errorf("%s = %+v %v", path, got, ok)
		}
	}
	for _, path := range []string{"/shorts/copy", "/shorts//copy", "/shorts/j1/copy/other", "/shorts/j1/copy/status/x", "/shorts/j1/copies", "/projects/j1/copy", "/shorts/j1"} {
		if _, ok := parseCopyPath(path); ok {
			t.Errorf("%s should not match", path)
		}
	}
}

func TestCopy_FromATopic(t *testing.T) {
	e := newCopyEnv(t)
	out := decodeCopy(t, e.post(t, "user1", CopyRequest{Topic: "A video about a lighthouse keeper"}))
	if len(out.Titles) != 10 || out.Description == "" || len(out.Chapters) != 3 || !out.HasTimestamps {
		t.Errorf("out = %+v", out)
	}
	if out.RemainingToday == nil || *out.RemainingToday != 19 || out.DailyLimit != 20 {
		t.Errorf("remaining = %v limit %d", out.RemainingToday, out.DailyLimit)
	}
	if len(e.w.reqs) != 1 || e.w.reqs[0].Topic != "A video about a lighthouse keeper" || len(e.w.reqs[0].Words) != 0 {
		t.Errorf("request = %+v", e.w.reqs)
	}
	if e.used("user1") != 1 {
		t.Errorf("used = %d", e.used("user1"))
	}
}

func TestCopy_ChaptersIsAlwaysAList(t *testing.T) {
	e := newCopyEnv(t)
	e.w.res.Chapters = nil
	e.w.res.Notes = nil
	resp := e.post(t, "user1", CopyRequest{Topic: "A video about a lighthouse keeper"})
	if !strings.Contains(resp.Body, `"chapters":[]`) || !strings.Contains(resp.Body, `"notes":[]`) {
		t.Errorf("empty lists must be [] not null: %s", resp.Body)
	}
}

func TestCopy_UsesTheJobsOwnTranscript(t *testing.T) {
	e := newCopyEnv(t)
	decodeCopy(t, e.post(t, "user1", CopyRequest{UseJobTranscript: true, Topic: "about a keeper"}))
	r := e.w.reqs[0]
	if len(r.Words) != 30 || r.Words[6].Start != 60 || r.Duration != 120 {
		t.Errorf("words %d duration %v", len(r.Words), r.Duration)
	}
	if r.Topic != "about a keeper" {
		t.Errorf("topic = %q", r.Topic)
	}
}

func TestCopy_JobTranscriptProblems(t *testing.T) {
	cases := map[string]func(e *copyEnv){
		"no key and nothing at the usual place": func(e *copyEnv) {
			e.h.DB.(*mockDynamoDB).getItemFunc = withJobWithoutTranscript(e)
			e.s3.meta = storage.ObjectMeta{}
		},
		"object missing": func(e *copyEnv) { e.s3.objects = map[string][]byte{} },
		"not json":       func(e *copyEnv) { e.s3.objects["shorts/user1/job1/transcript.json"] = []byte("nope") },
		"no words":       func(e *copyEnv) { e.s3.objects["shorts/user1/job1/transcript.json"] = []byte(`{"text":"","words":[]}`) },
		"too large": func(e *copyEnv) {
			e.s3.objects["shorts/user1/job1/transcript.json"] = make([]byte, copyMaxTranscriptBytes+1)
		},
	}
	for name, setup := range cases {
		e := newCopyEnv(t)
		setup(e)
		resp := e.post(t, "user1", CopyRequest{UseJobTranscript: true})
		if resp.StatusCode < 400 || resp.StatusCode >= 600 {
			t.Errorf("%s: status %d", name, resp.StatusCode)
		}
		if e.w.calls != 0 || e.used("user1") != 0 {
			t.Errorf("%s: the model was called (%d) or a use was spent (%d)", name, e.w.calls, e.used("user1"))
		}
	}
}

// withJobWithoutTranscript is the job lookup for a job that has no transcriptKey.
func withJobWithoutTranscript(e *copyEnv) func(context.Context, *dynamodb.GetItemInput, ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
	return func(_ context.Context, in *dynamodb.GetItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
		pk, _ := in.Key["PK"].(*dbtypes.AttributeValueMemberS)
		sk, _ := in.Key["SK"].(*dbtypes.AttributeValueMemberS)
		if pk != nil && sk != nil && pk.Value == "USER#user1" && sk.Value == "SHORTS#job1" {
			item := jobItemFor("completed", "mp4")
			delete(item, "transcriptKey")
			return &dynamodb.GetItemOutput{Item: item}, nil
		}
		return &dynamodb.GetItemOutput{}, nil
	}
}

func TestCopy_OlderJobsFallBackToTheUsualTranscriptPlace(t *testing.T) {
	e := newCopyEnv(t)
	e.h.DB.(*mockDynamoDB).getItemFunc = withJobWithoutTranscript(e)
	decodeCopy(t, e.post(t, "user1", CopyRequest{UseJobTranscript: true}))
	if len(e.w.reqs[0].Words) == 0 {
		t.Error("the transcript at the conventional key was not used")
	}
}

func TestCopy_ValidationCostsNothing(t *testing.T) {
	e := newCopyEnv(t)
	for name, body := range map[string]any{
		"nothing":      CopyRequest{},
		"thin":         CopyRequest{Topic: "hi"},
		"huge refine":  CopyRequest{Topic: "A video about a keeper", Refine: strings.Repeat("r", 5000)},
		"too many old": CopyRequest{Topic: "A video about a keeper", PreviousTitles: make([]string, 11)},
		"huge text":    CopyRequest{Transcript: strings.Repeat("a b ", 30000)},
	} {
		resp := e.post(t, "user1", body)
		if resp.StatusCode != 400 || decodeErr(t, resp).Code != "VALIDATION_ERROR" || decodeErr(t, resp).Message == "" {
			t.Errorf("%s: %d %s", name, resp.StatusCode, resp.Body)
		}
	}
	if resp := e.do("POST", "/shorts/job1/copy", "{not json", "user1"); resp.StatusCode != 400 {
		t.Errorf("bad json: %d", resp.StatusCode)
	}
	if resp := e.do("POST", "/shorts/job1/copy", strings.Repeat("x", copyMaxBody+1), "user1"); resp.StatusCode != 400 {
		t.Errorf("oversized body: %d", resp.StatusCode)
	}
	if e.w.calls != 0 || e.used("user1") != 0 {
		t.Errorf("model calls %d, uses spent %d", e.w.calls, e.used("user1"))
	}
}

func TestCopy_QuotaBoundary(t *testing.T) {
	e := newCopyEnv(t)
	for i := 1; i <= 20; i++ {
		out := decodeCopy(t, e.post(t, "user1", CopyRequest{Topic: "A video about a lighthouse keeper"}))
		if want := int64(20 - i); out.RemainingToday == nil || *out.RemainingToday != want {
			t.Fatalf("use %d: remaining %v, want %d", i, out.RemainingToday, want)
		}
	}
	resp := e.post(t, "user1", CopyRequest{Topic: "A video about a lighthouse keeper"})
	if resp.StatusCode != 429 || decodeErr(t, resp).Code != "QUOTA_EXCEEDED" || resp.Headers["Retry-After"] == "" {
		t.Errorf("21st use: %d %s", resp.StatusCode, resp.Body)
	}
	if e.w.calls != 20 {
		t.Errorf("the model was called %d times", e.w.calls)
	}
}

func TestCopy_FailuresGiveTheUseBack(t *testing.T) {
	for _, c := range []struct {
		err    error
		status int
		code   string
	}{
		{fmt.Errorf("%w: %w", videocopy.ErrModelTimeout, videocopy.ErrModelFailed), 504, "COPY_TIMEOUT"},
		{fmt.Errorf("%w: busy", videocopy.ErrModelFailed), 503, "COPY_UNAVAILABLE"},
		{videocopy.ErrInvalidAnswer, 502, "COPY_FAILED"},
		{errors.New("something else"), 500, "INTERNAL_ERROR"},
	} {
		e := newCopyEnv(t)
		e.w.err = c.err
		resp := e.post(t, "user1", CopyRequest{Topic: "A video about a lighthouse keeper"})
		if resp.StatusCode != c.status || decodeErr(t, resp).Code != c.code {
			t.Errorf("%v: %d %s", c.err, resp.StatusCode, resp.Body)
		}
		if e.used("user1") != 0 {
			t.Errorf("%v: %d uses left spent", c.err, e.used("user1"))
		}
		if strings.Contains(resp.Body, "request id") {
			t.Errorf("leaks provider detail: %s", resp.Body)
		}
	}
}

func TestCopy_ACancelledRequestKeepsTheUse(t *testing.T) {
	e := newCopyEnv(t)
	e.w.err = context.Canceled
	e.post(t, "user1", CopyRequest{Topic: "A video about a lighthouse keeper"})
	if e.used("user1") != 1 {
		t.Errorf("used = %d", e.used("user1"))
	}
}

func TestCopy_ConcurrentRequestsAreEachCounted(t *testing.T) {
	e := newCopyEnv(t)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if resp := e.post(t, "user1", CopyRequest{Topic: "A video about a lighthouse keeper"}); resp.StatusCode != 200 {
				t.Errorf("status %d", resp.StatusCode)
			}
		}()
	}
	wg.Wait()
	if e.used("user1") != 8 || e.w.calls != 8 {
		t.Errorf("used %d calls %d", e.used("user1"), e.w.calls)
	}
}

func TestCopy_RefinePassesThePreviousTitles(t *testing.T) {
	e := newCopyEnv(t)
	decodeCopy(t, e.post(t, "user1", CopyRequest{Topic: "A video about a lighthouse keeper", Refine: "shorter", PreviousTitles: []string{"One", "Two"}}))
	r := e.w.reqs[0]
	if r.Refine != "shorter" || len(r.Previous) != 2 {
		t.Errorf("request = %+v", r)
	}
	if e.used("user1") != 1 {
		t.Error("a refine is a new call and counts")
	}
}

func TestCopy_SwitchedOff(t *testing.T) {
	e := newCopyEnv(t)
	e.h.CopyWriter = nil
	resp := e.post(t, "user1", CopyRequest{Topic: "A video about a lighthouse keeper"})
	if resp.StatusCode != 503 || decodeErr(t, resp).Code != "UNAVAILABLE" {
		t.Errorf("%d %s", resp.StatusCode, resp.Body)
	}
	if e.used("user1") != 0 {
		t.Error("a switched-off feature must not spend a use")
	}
	st := e.status(t, "user1")
	if st.Enabled || st.TranscriptAvailable || st.RemainingToday != nil {
		t.Errorf("status = %+v", st)
	}
}

func TestCopy_NeedsSignInAndOwnership(t *testing.T) {
	e := newCopyEnv(t)
	if resp := e.do("POST", "/shorts/job1/copy", `{"topic":"A video about a keeper"}`, ""); resp.StatusCode != 401 {
		t.Errorf("no token: %d", resp.StatusCode)
	}
	if resp := e.do("GET", "/shorts/job1/copy/status", "", ""); resp.StatusCode != 401 {
		t.Errorf("status without token: %d", resp.StatusCode)
	}
	// user2 has no such job: someone else's job id is simply not found.
	if resp := e.post(t, "user2", CopyRequest{Topic: "A video about a lighthouse keeper"}); resp.StatusCode != 404 {
		t.Errorf("another user's job: %d %s", resp.StatusCode, resp.Body)
	}
	if resp := e.do("GET", "/shorts/job1/copy/status", "", "user2"); resp.StatusCode != 404 {
		t.Errorf("status of another user's job: %d", resp.StatusCode)
	}
	if e.w.calls != 0 || e.used("user2") != 0 {
		t.Error("nothing should have been spent")
	}
}

func TestCopy_WrongMethods(t *testing.T) {
	e := newCopyEnv(t)
	for _, c := range [][2]string{{"GET", "/shorts/job1/copy"}, {"POST", "/shorts/job1/copy/status"}, {"DELETE", "/shorts/job1/copy"}, {"PUT", "/shorts/job1/copy"}} {
		if resp := e.do(c[0], c[1], "{}", "user1"); resp.StatusCode != 405 {
			t.Errorf("%s %s: %d", c[0], c[1], resp.StatusCode)
		}
	}
}

func TestCopy_RefusedForAnUnconfirmedEmail(t *testing.T) {
	e := newCopyEnv(t)
	e.h.Verifier = &auth.EmailVerifier{DB: e.h.DB.(*mockDynamoDB), UsersTable: "users-table", TokenTable: "sessions-table", BaseURL: "https://indifferent.fun", Mailer: nopMailer{}}
	e.h.UsersTable = "users-table"
	db := e.h.DB.(*mockDynamoDB)
	prev := db.getItemFunc
	db.getItemFunc = func(ctx context.Context, in *dynamodb.GetItemInput, o ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
		if in.Key["SK"].(*dbtypes.AttributeValueMemberS).Value == "PROFILE" {
			return &dynamodb.GetItemOutput{Item: map[string]dbtypes.AttributeValue{"emailVerified": &dbtypes.AttributeValueMemberBOOL{Value: false}}}, nil
		}
		return prev(ctx, in, o...)
	}
	resp := e.post(t, "user1", CopyRequest{Topic: "A video about a lighthouse keeper"})
	if resp.StatusCode != 403 || decodeErr(t, resp).Code != "EMAIL_NOT_VERIFIED" {
		t.Errorf("%d %s", resp.StatusCode, resp.Body)
	}
	if e.w.calls != 0 || e.used("user1") != 0 {
		t.Error("an unconfirmed account must not reach the model or spend a use")
	}
	// Reading the status is not spending.
	if resp := e.do("GET", "/shorts/job1/copy/status", "", "user1"); resp.StatusCode != 200 {
		t.Errorf("status: %d", resp.StatusCode)
	}
}

func (e *copyEnv) status(t *testing.T, user string) CopyStatus {
	t.Helper()
	resp := e.do("GET", "/shorts/job1/copy/status", "", user)
	if resp.StatusCode != 200 {
		t.Fatalf("status %d: %s", resp.StatusCode, resp.Body)
	}
	var st CopyStatus
	if err := json.Unmarshal([]byte(resp.Body), &st); err != nil {
		t.Fatal(err)
	}
	return st
}

func TestCopyStatus(t *testing.T) {
	e := newCopyEnv(t)
	st := e.status(t, "user1")
	if !st.Enabled || !st.TranscriptAvailable || st.DailyLimit != 20 || st.RemainingToday == nil || *st.RemainingToday != 20 {
		t.Errorf("status = %+v", st)
	}
	if st.MaxTopicChars != videocopy.MaxTopicRunes || st.MaxTranscriptChars != videocopy.MaxTranscriptRunes || st.MaxRefineChars != videocopy.MaxRefineRunes {
		t.Errorf("limits = %+v", st)
	}
	for i := 0; i < 3; i++ {
		decodeCopy(t, e.post(t, "user1", CopyRequest{Topic: "A video about a lighthouse keeper"}))
	}
	if st := e.status(t, "user1"); st.RemainingToday == nil || *st.RemainingToday != 17 {
		t.Errorf("after 3 uses: %+v", st)
	}
	// Reading the status never spends a use.
	for i := 0; i < 5; i++ {
		e.status(t, "user1")
	}
	if e.used("user1") != 3 {
		t.Errorf("used = %d", e.used("user1"))
	}
	e.h.DB.(*mockDynamoDB).getItemFunc = withJobWithoutTranscript(e)
	e.s3.meta = storage.ObjectMeta{}
	if st := e.status(t, "user1"); st.TranscriptAvailable {
		t.Error("a job with no transcript must say so")
	}
}

func TestCopy_WorksWithoutAQuotaTable(t *testing.T) {
	e := newCopyEnv(t)
	e.h.QuotaTable = ""
	out := decodeCopy(t, e.post(t, "user1", CopyRequest{Topic: "A video about a lighthouse keeper"}))
	if out.RemainingToday != nil {
		t.Errorf("remaining should be unknown, got %v", *out.RemainingToday)
	}
}
