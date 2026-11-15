package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/smithy-go"
	"github.com/rahul/indifferent/backend/internal/auth"
	"github.com/rahul/indifferent/backend/internal/narrator"
	"github.com/rahul/indifferent/backend/internal/storyjob"
	"github.com/rahul/indifferent/backend/internal/storyplan"
)

// fakeVoice stands in for Polly.
type fakeVoice struct {
	mu    sync.Mutex
	calls []string // "voice|text"
	err   error
	body  func(text string) []byte
	// gate, when set, holds every call until it is closed.
	gate chan struct{}
	n    atomic.Int32
}

func (f *fakeVoice) SynthesizeText(ctx context.Context, text, voice string) ([]byte, error) {
	f.n.Add(1)
	f.mu.Lock()
	f.calls = append(f.calls, voice+"|"+text)
	err, body, gate := f.err, f.body, f.gate
	f.mu.Unlock()
	if gate != nil {
		select {
		case <-gate:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if err != nil {
		return nil, err
	}
	if body != nil {
		return body(text), nil
	}
	return []byte("MP3:" + text), nil
}

// fakeBucket is the private bucket: objects, and links that carry a signature.
type fakeBucket struct {
	mu      sync.Mutex
	objects map[string][]byte
	types   map[string]string
	headErr error
	putErr  error
	linkErr error
	heads   int
	puts    int
	links   []time.Duration
}

func newFakeBucket() *fakeBucket {
	return &fakeBucket{objects: map[string][]byte{}, types: map[string]string{}}
}
func (b *fakeBucket) HeadObject(_ context.Context, bucket, key string) (bool, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.heads++
	if b.headErr != nil {
		return false, b.headErr
	}
	_, ok := b.objects[bucket+"/"+key]
	return ok, nil
}
func (b *fakeBucket) PutObject(_ context.Context, bucket, key string, data []byte, contentType string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.putErr != nil {
		return b.putErr
	}
	b.puts++
	b.objects[bucket+"/"+key] = data
	b.types[key] = contentType
	return nil
}
func (b *fakeBucket) GenerateDownloadURL(_ context.Context, bucket, key string, ttl time.Duration) (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.linkErr != nil {
		return "", b.linkErr
	}
	b.links = append(b.links, ttl)
	return "https://" + bucket + ".s3.example/" + key + "?X-Amz-Signature=abc&X-Amz-Expires=" + strconv.Itoa(int(ttl.Seconds())), nil
}

// voiceEnv is a story plan environment with the voice preview switched on and a
// DynamoDB that behaves like the real one for counters and leases.
type voiceEnv struct {
	*spEnv
	synth  *fakeVoice
	bucket *fakeBucket
	dbMu   sync.Mutex
	counts map[string]int64
	leases map[string]int64 // lock PK -> leaseUntil in ms
	planID string
	rev    int
}

func newVoiceEnv(t *testing.T) *voiceEnv {
	t.Helper()
	fixedQuotaClock(t, time.Date(2026, 11, 16, 10, 0, 0, 0, time.UTC))
	e := &voiceEnv{spEnv: newSPEnv(t), synth: &fakeVoice{}, bucket: newFakeBucket(), counts: map[string]int64{}, leases: map[string]int64{}}
	e.wire(e.h)
	resp := e.do(t, "POST", "/story-plans", storyBody(spEnglish), "user1")
	if resp.StatusCode != 201 {
		t.Fatalf("plan: %d %s", resp.StatusCode, resp.Body)
	}
	p := decodePlan(t, resp)
	e.planID, e.rev = p.PlanID, p.Revision
	return e
}

// wire gives a handler the preview pieces and the shared counting database.
func (e *voiceEnv) wire(h *APIHandler) {
	h.VoiceSynth = e.synth
	h.PreviewFiles = e.bucket
	h.QuotaTable = "sessions-table"
	db := h.DB.(*mockDynamoDB)
	db.updateItemFunc = func(_ context.Context, in *dynamodb.UpdateItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
		e.dbMu.Lock()
		defer e.dbMu.Unlock()
		pk := in.Key["PK"].(*dbtypes.AttributeValueMemberS).Value
		sk := in.Key["SK"].(*dbtypes.AttributeValueMemberS).Value
		k := pk + "|" + sk
		num := func(name string) int64 {
			n, _ := strconv.ParseInt(in.ExpressionAttributeValues[name].(*dbtypes.AttributeValueMemberN).Value, 10, 64)
			return n
		}
		switch {
		case in.ExpressionAttributeValues[":one"] != nil:
			e.counts[k]++
			return &dynamodb.UpdateItemOutput{Attributes: map[string]dbtypes.AttributeValue{"uses": &dbtypes.AttributeValueMemberN{Value: strconv.FormatInt(e.counts[k], 10)}}}, nil
		case in.ExpressionAttributeValues[":minus"] != nil:
			if e.counts[k] <= 0 {
				return nil, &dbtypes.ConditionalCheckFailedException{}
			}
			e.counts[k]--
			return &dynamodb.UpdateItemOutput{}, nil
		case in.ExpressionAttributeValues[":until"] != nil:
			if until, held := e.leases[pk]; held && until >= num(":now") {
				return nil, &dbtypes.ConditionalCheckFailedException{}
			}
			e.leases[pk] = num(":until")
			return &dynamodb.UpdateItemOutput{}, nil
		}
		return &dynamodb.UpdateItemOutput{}, nil
	}
	db.deleteItemFunc = func(_ context.Context, in *dynamodb.DeleteItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error) {
		e.dbMu.Lock()
		defer e.dbMu.Unlock()
		delete(e.leases, in.Key["PK"].(*dbtypes.AttributeValueMemberS).Value)
		return &dynamodb.DeleteItemOutput{}, nil
	}
}

// used is how many uses of a quota the user has today.
func (e *voiceEnv) used(user string, q quota) int64 {
	e.dbMu.Lock()
	defer e.dbMu.Unlock()
	return e.counts["QUOTA#"+user+"|"+q.name+"#20261116"]
}

type voicePreviewBody struct {
	Revision  int     `json:"revision"`
	Scene     int     `json:"scene"`
	Narration *string `json:"narration,omitempty"`
	Voice     string  `json:"voice,omitempty"`
}

func (e *voiceEnv) preview(t *testing.T, user string, b voicePreviewBody) events.APIGatewayProxyResponse {
	t.Helper()
	raw, _ := json.Marshal(b)
	return e.do(t, "POST", "/story-plans/"+e.planID+"/voice-preview", string(raw), user)
}
func (e *voiceEnv) ok(t *testing.T, user string, b voicePreviewBody) VoicePreviewResponse {
	t.Helper()
	resp := e.preview(t, user, b)
	if resp.StatusCode != 200 {
		t.Fatalf("status %d: %s", resp.StatusCode, resp.Body)
	}
	var out VoicePreviewResponse
	if err := json.Unmarshal([]byte(resp.Body), &out); err != nil {
		t.Fatal(err)
	}
	return out
}
func strPtr(s string) *string { return &s }

func (e *voiceEnv) body(scene int) voicePreviewBody {
	return voicePreviewBody{Revision: e.rev, Scene: scene}
}

func TestVoicePreview_MakesATakeThenReplaysItWithoutCallingPollyOrUsingTheAllowance(t *testing.T) {
	e := newVoiceEnv(t)
	first := e.ok(t, "user1", e.body(1))
	if first.Cached || first.RemainingToday == nil || *first.RemainingToday != 59 || first.DailyLimit != 60 {
		t.Fatalf("first = %+v, want a new take with 59 of 60 left", first)
	}
	if e.synth.n.Load() != 1 || e.used("user1", quotaVoicePreview) != 1 {
		t.Fatalf("polly calls %d, uses %d, want 1 and 1", e.synth.n.Load(), e.used("user1", quotaVoicePreview))
	}
	for i := 0; i < 5; i++ {
		again := e.ok(t, "user1", e.body(1))
		if !again.Cached || again.RemainingToday != nil {
			t.Fatalf("replay %d = %+v, want cached", i, again)
		}
	}
	if e.synth.n.Load() != 1 || e.used("user1", quotaVoicePreview) != 1 {
		t.Errorf("replays used polly %d times and %d uses, want 1 and 1", e.synth.n.Load(), e.used("user1", quotaVoicePreview))
	}
	if e.used("user1", quotaVoicePreviewReplay) != 5 {
		t.Errorf("replays counted %d, want 5 on their own counter", e.used("user1", quotaVoicePreviewReplay))
	}
}

func TestVoicePreview_ScenesWithTheSameTextShareOneTake(t *testing.T) {
	// samplePlan gives both scenes the same narration.
	e := newVoiceEnv(t)
	e.ok(t, "user1", e.body(1))
	second := e.ok(t, "user1", e.body(2))
	if !second.Cached || e.synth.n.Load() != 1 {
		t.Errorf("same text and voice must be one take: cached=%v polly=%d", second.Cached, e.synth.n.Load())
	}
}

func TestVoicePreview_TheTakeIsPrivateAndTheLinkIsShortLived(t *testing.T) {
	e := newVoiceEnv(t)
	out := e.ok(t, "user1", e.body(1))
	if out.ExpiresInSeconds != 300 || len(e.bucket.links) != 1 || e.bucket.links[0] != 5*time.Minute {
		t.Errorf("link ttl %d s / %v, want 300 s", out.ExpiresInSeconds, e.bucket.links)
	}
	if !strings.Contains(out.URL, "X-Amz-Signature") || !strings.Contains(out.URL, "/story-preview/user1/") {
		t.Errorf("url %q is not a signed link into the user's preview folder", out.URL)
	}
	for key, ct := range e.bucket.types {
		if ct != "audio/mpeg" || !strings.HasPrefix(key, storyjob.PreviewPrefix+"user1/") {
			t.Errorf("stored %q as %q", key, ct)
		}
	}
	if strings.Contains(out.URL, "fisherman") {
		t.Error("the link must not carry the text")
	}
}

func TestVoicePreview_SpeaksWhatTheRenderWillLookFor(t *testing.T) {
	e := newVoiceEnv(t)
	text := samplePlan("en").Scenes[0].Narration
	e.ok(t, "user1", e.body(1))
	if _, ok := e.bucket.objects["test-bucket/"+storyjob.PreviewKey("user1", text, "Joanna")]; !ok {
		t.Fatalf("take not stored under storyjob.PreviewKey; have %v", keys(e.bucket.objects))
	}
}

func keys(m map[string][]byte) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestVoicePreview_UnsavedTextIsCleanedAsASaveWouldCleanIt(t *testing.T) {
	e := newVoiceEnv(t)
	e.ok(t, "user1", voicePreviewBody{Revision: e.rev, Scene: 1, Narration: strPtr("  the   <b>lighthouse</b> keeper climbs the stairs  ")})
	cleaned := e.synth.calls[0]
	if strings.Contains(cleaned, "<") || strings.Contains(cleaned, "  ") {
		t.Errorf("spoken text %q still has markup or double spaces", cleaned)
	}
	// The same text typed another way is the same take.
	again := e.ok(t, "user1", voicePreviewBody{Revision: e.rev, Scene: 2, Narration: strPtr("the lighthouse keeper climbs the stairs")})
	if !again.Cached {
		t.Error("text that cleans to the same words must be the same take")
	}
}

func TestVoicePreview_Validation(t *testing.T) {
	e := newVoiceEnv(t)
	long := strings.Repeat("a long long story ", 80)
	cases := []struct {
		name string
		b    voicePreviewBody
		want int
		code string
	}{
		{"scene zero", voicePreviewBody{Revision: e.rev, Scene: 0}, 400, "VALIDATION_ERROR"},
		{"scene past the end", voicePreviewBody{Revision: e.rev, Scene: 3}, 400, "VALIDATION_ERROR"},
		{"stale revision", voicePreviewBody{Revision: e.rev + 1, Scene: 1}, 409, "STALE_REVISION"},
		{"revision missing", voicePreviewBody{Scene: 1}, 409, "STALE_REVISION"},
		{"empty text", voicePreviewBody{Revision: e.rev, Scene: 1, Narration: strPtr("   ")}, 400, "VALIDATION_ERROR"},
		{"markup only", voicePreviewBody{Revision: e.rev, Scene: 1, Narration: strPtr("<b></b>")}, 400, "VALIDATION_ERROR"},
		{"too long", voicePreviewBody{Revision: e.rev, Scene: 1, Narration: strPtr(long)}, 400, "VALIDATION_ERROR"},
		{"wrong script", voicePreviewBody{Revision: e.rev, Scene: 1, Narration: strPtr("मछुआरा चुपचाप अँधेरे पानी में नाव खेता है और गाता है")}, 400, "VALIDATION_ERROR"},
		{"unknown voice", voicePreviewBody{Revision: e.rev, Scene: 1, Voice: "Nobody"}, 400, "VALIDATION_ERROR"},
	}
	for _, tc := range cases {
		resp := e.preview(t, "user1", tc.b)
		if resp.StatusCode != tc.want || decodeErr(t, resp).Code != tc.code {
			t.Errorf("%s: %d %s, want %d %s", tc.name, resp.StatusCode, resp.Body, tc.want, tc.code)
		}
	}
	if e.synth.n.Load() != 0 || e.used("user1", quotaVoicePreview) != 0 {
		t.Errorf("a refused request used polly %d times and %d uses; it must cost nothing", e.synth.n.Load(), e.used("user1", quotaVoicePreview))
	}
	// Bad bodies.
	for name, body := range map[string]string{"not json": "{", "too large": `{"narration":"` + strings.Repeat("x", 17<<10) + `"}`} {
		resp := e.do(t, "POST", "/story-plans/"+e.planID+"/voice-preview", body, "user1")
		if resp.StatusCode != 400 {
			t.Errorf("%s: %d", name, resp.StatusCode)
		}
	}
}

func TestVoicePreview_AVoiceThatCannotReadTheLanguageIsRefusedBeforeAnythingIsSpent(t *testing.T) {
	e := newVoiceEnv(t)
	e.planner.res = storyplan.Result{Plan: samplePlan("hi"), Script: storyplan.ScriptDevanagari, StoryRunes: 100}
	resp := e.do(t, "POST", "/story-plans", storyBody(spHindi), "user1")
	if resp.StatusCode != 201 {
		t.Fatalf("hindi plan: %d %s", resp.StatusCode, resp.Body)
	}
	p := decodePlan(t, resp)
	e.planID, e.rev = p.PlanID, p.Revision
	// An English voice given Devanagari makes Polly answer and say nothing.
	if resp := e.preview(t, "user1", voicePreviewBody{Revision: e.rev, Scene: 1, Voice: "Joanna"}); resp.StatusCode != 400 {
		t.Errorf("Joanna on a Hindi story: %d %s", resp.StatusCode, resp.Body)
	}
	if e.synth.n.Load() != 0 || e.used("user1", quotaVoicePreview) != 0 {
		t.Error("a refused voice must cost nothing")
	}
	e.ok(t, "user1", e.body(1)) // the default voice for Hindi
	if got := e.synth.calls[0]; !strings.HasPrefix(got, "Aditi|") {
		t.Errorf("default voice = %q, want Aditi", got)
	}
}

func TestVoicePreview_AnotherUsersPlanIsNotFoundAndASignedOutCallIsRefused(t *testing.T) {
	e := newVoiceEnv(t)
	if resp := e.preview(t, "user2", e.body(1)); resp.StatusCode != 404 {
		t.Errorf("another user's plan: %d, want 404", resp.StatusCode)
	}
	if resp := e.preview(t, "", e.body(1)); resp.StatusCode != 401 {
		t.Errorf("no token: %d, want 401", resp.StatusCode)
	}
	if resp := e.do(t, "POST", "/story-plans/not-a-uuid/voice-preview", `{}`, "user1"); resp.StatusCode != 404 {
		t.Errorf("bad id: %d, want 404", resp.StatusCode)
	}
	if resp := e.do(t, "GET", "/story-plans/"+e.planID+"/voice-preview", ``, "user1"); resp.StatusCode != 404 {
		t.Errorf("GET: %d, want 404 (only POST)", resp.StatusCode)
	}
	if e.synth.n.Load() != 0 {
		t.Error("polly must not be called for a refused request")
	}
}

func TestVoicePreview_IsASpendingRouteSoUnconfirmedAccountsAreHeldBack(t *testing.T) {
	if !spendingRequest(events.APIGatewayProxyRequest{HTTPMethod: "POST", Path: "/story-plans/abc/voice-preview"}) {
		t.Fatal("the preview costs money and must be gated on a confirmed email")
	}
	e := newVoiceEnv(t)
	e.h.Verifier = &auth.EmailVerifier{DB: e.h.DB.(*mockDynamoDB), UsersTable: "users-table", TokenTable: "sessions-table", BaseURL: "https://indifferent.fun", Mailer: nopMailer{}}
	e.h.UsersTable = "users-table"
	db := e.h.DB.(*mockDynamoDB)
	db.getItemFunc = func(_ context.Context, in *dynamodb.GetItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
		if in.Key["SK"].(*dbtypes.AttributeValueMemberS).Value == "PROFILE" {
			return &dynamodb.GetItemOutput{Item: map[string]dbtypes.AttributeValue{
				"emailVerified": &dbtypes.AttributeValueMemberBOOL{Value: false},
				"email":         &dbtypes.AttributeValueMemberS{Value: "a@example.com"},
			}}, nil
		}
		return &dynamodb.GetItemOutput{}, nil
	}
	resp := e.preview(t, "user1", e.body(1))
	if resp.StatusCode != 403 || decodeErr(t, resp).Code != "EMAIL_NOT_VERIFIED" {
		t.Errorf("unconfirmed account: %d %s", resp.StatusCode, resp.Body)
	}
	if e.synth.n.Load() != 0 {
		t.Error("polly must not be called for an unconfirmed account")
	}
}

func TestVoicePreview_SwitchedOffAnswers503AndTouchesNothing(t *testing.T) {
	e := newVoiceEnv(t)
	e.h.VoiceSynth = nil
	resp := e.preview(t, "user1", e.body(1))
	if resp.StatusCode != 503 || decodeErr(t, resp).Code != "UNAVAILABLE" {
		t.Errorf("%d %s", resp.StatusCode, resp.Body)
	}
	if e.bucket.heads != 0 || e.synth.n.Load() != 0 {
		t.Error("a switched-off preview must not touch storage or polly")
	}
}

func TestVoicePreview_DailyLimitIsEnforcedOnTheServerAtTheBoundary(t *testing.T) {
	withLimit(t, &quotaVoicePreview, 3)
	e := newVoiceEnv(t)
	for i := 1; i <= 3; i++ {
		out := e.ok(t, "user1", voicePreviewBody{Revision: e.rev, Scene: 1, Narration: strPtr(fmt.Sprintf("take number %d of the day", i))})
		if out.RemainingToday == nil || *out.RemainingToday != int64(3-i) {
			t.Fatalf("take %d: remaining %v, want %d", i, out.RemainingToday, 3-i)
		}
	}
	resp := e.preview(t, "user1", voicePreviewBody{Revision: e.rev, Scene: 1, Narration: strPtr("one more than the allowance")})
	if resp.StatusCode != 429 || decodeErr(t, resp).Code != "QUOTA_EXCEEDED" || resp.Headers["Retry-After"] == "" {
		t.Fatalf("the 4th new take: %d %s", resp.StatusCode, resp.Body)
	}
	if e.synth.n.Load() != 3 {
		t.Errorf("polly called %d times, want 3: the refused take must not reach it", e.synth.n.Load())
	}
	// A take already kept still plays after the allowance is used up.
	if out := e.ok(t, "user1", voicePreviewBody{Revision: e.rev, Scene: 1, Narration: strPtr("take number 2 of the day")}); !out.Cached {
		t.Error("a kept take must still play when the allowance is spent")
	}
	// Another user has their own allowance.
	if resp := e.preview(t, "user2", e.body(1)); resp.StatusCode != 404 { // plan belongs to user1
		t.Errorf("another user cannot use this plan: %d", resp.StatusCode)
	}
}

func TestVoicePreview_ReplaysHaveTheirOwnGenerousCap(t *testing.T) {
	withLimit(t, &quotaVoicePreviewReplay, 2)
	e := newVoiceEnv(t)
	e.ok(t, "user1", e.body(1)) // made, not a replay
	e.ok(t, "user1", e.body(1))
	e.ok(t, "user1", e.body(1))
	resp := e.preview(t, "user1", e.body(1))
	if resp.StatusCode != 429 || decodeErr(t, resp).Code != "QUOTA_EXCEEDED" {
		t.Errorf("3rd replay: %d %s", resp.StatusCode, resp.Body)
	}
	if e.used("user1", quotaVoicePreview) != 1 {
		t.Errorf("replays must not touch the make allowance: %d", e.used("user1", quotaVoicePreview))
	}
}

func TestVoicePreview_PollyFailuresGiveTheUseBackAndSayWhatToDo(t *testing.T) {
	throttle := &smithy.GenericAPIError{Code: "ThrottlingException", Message: "slow down"}
	cases := []struct {
		name  string
		err   error
		want  int
		code  string
		retry bool
	}{
		{"throttled", throttle, 429, "VOICE_BUSY", true},
		{"wrapped throttle", fmt.Errorf("polly synthesis failed after 3 attempts: %w", throttle), 429, "VOICE_BUSY", true},
		{"timed out", context.DeadlineExceeded, 429, "VOICE_BUSY", true},
		{"service failure", &smithy.GenericAPIError{Code: "ServiceFailureException"}, 502, "VOICE_FAILED", false},
		{"anything else", errors.New("boom"), 502, "VOICE_FAILED", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newVoiceEnv(t)
			e.synth.err = tc.err
			resp := e.preview(t, "user1", e.body(1))
			if resp.StatusCode != tc.want || decodeErr(t, resp).Code != tc.code {
				t.Fatalf("%d %s, want %d %s", resp.StatusCode, resp.Body, tc.want, tc.code)
			}
			if tc.retry && resp.Headers["Retry-After"] == "" {
				t.Error("a busy answer must say when to try again")
			}
			if strings.Contains(resp.Body, "boom") || strings.Contains(resp.Body, "Throttling") {
				t.Errorf("the message must not leak the cause: %s", resp.Body)
			}
			if e.used("user1", quotaVoicePreview) != 0 {
				t.Errorf("a failure must give the use back, have %d", e.used("user1", quotaVoicePreview))
			}
			if len(e.bucket.objects) != 0 {
				t.Error("nothing may be kept after a failure")
			}
			// Once Polly recovers the same request works and is counted once.
			e.synth.err = nil
			e.ok(t, "user1", e.body(1))
			if e.used("user1", quotaVoicePreview) != 1 {
				t.Errorf("after recovery uses = %d, want 1", e.used("user1", quotaVoicePreview))
			}
		})
	}
}

func TestVoicePreview_AnUnusableTakeIsNotKeptOrCounted(t *testing.T) {
	for name, body := range map[string]func(string) []byte{
		"empty":   func(string) []byte { return nil },
		"too big": func(string) []byte { return make([]byte, voicePreviewMaxBytes+1) },
	} {
		e := newVoiceEnv(t)
		e.synth.body = body
		resp := e.preview(t, "user1", e.body(1))
		if resp.StatusCode != 502 || len(e.bucket.objects) != 0 || e.used("user1", quotaVoicePreview) != 0 {
			t.Errorf("%s: %d, kept %d, uses %d", name, resp.StatusCode, len(e.bucket.objects), e.used("user1", quotaVoicePreview))
		}
	}
}

func TestVoicePreview_StorageFailures(t *testing.T) {
	e := newVoiceEnv(t)
	e.bucket.headErr = errors.New("s3 down")
	if resp := e.preview(t, "user1", e.body(1)); resp.StatusCode != 500 || e.synth.n.Load() != 0 {
		t.Errorf("head failure: %d, polly %d; must stop before the voice", resp.StatusCode, e.synth.n.Load())
	}
	e.bucket.headErr = nil
	e.bucket.putErr = errors.New("s3 full")
	if resp := e.preview(t, "user1", e.body(1)); resp.StatusCode != 500 || decodeErr(t, resp).Code != "STORAGE_ERROR" {
		t.Errorf("put failure: %d %s", resp.StatusCode, resp.Body)
	}
	if e.used("user1", quotaVoicePreview) != 0 {
		t.Errorf("a take that could not be kept gives the use back: %d", e.used("user1", quotaVoicePreview))
	}
	e.bucket.putErr = nil
	e.bucket.linkErr = errors.New("cannot sign")
	if resp := e.preview(t, "user1", e.body(1)); resp.StatusCode != 500 {
		t.Errorf("link failure: %d", resp.StatusCode)
	}
}

func TestVoicePreview_ConcurrentRequestsForOneTakeCallPollyOnceAndSpendOnce(t *testing.T) {
	e := newVoiceEnv(t)
	e.synth.gate = make(chan struct{})
	const n = 12
	var wg sync.WaitGroup
	results := make([]VoicePreviewResponse, n)
	codes := make([]int, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			resp := e.preview(t, "user1", e.body(1))
			codes[i] = resp.StatusCode
			_ = json.Unmarshal([]byte(resp.Body), &results[i])
		}(i)
	}
	// Let every request reach the lock before Polly answers.
	deadline := time.Now().Add(2 * time.Second)
	for e.synth.n.Load() < 1 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	time.Sleep(100 * time.Millisecond)
	close(e.synth.gate)
	wg.Wait()
	made := 0
	for i := 0; i < n; i++ {
		if codes[i] != 200 {
			t.Fatalf("request %d: %d", i, codes[i])
		}
		if !results[i].Cached {
			made++
		}
	}
	if e.synth.n.Load() != 1 || made != 1 {
		t.Errorf("polly calls %d, requests that made the take %d; want 1 and 1", e.synth.n.Load(), made)
	}
	if e.used("user1", quotaVoicePreview) != 1 {
		t.Errorf("uses %d, want 1", e.used("user1", quotaVoicePreview))
	}
	if e.bucket.puts != 1 {
		t.Errorf("stored %d times, want 1", e.bucket.puts)
	}
	if len(e.leases) != 0 {
		t.Errorf("leases left behind: %v", e.leases)
	}
}

func TestVoicePreview_TwoInstancesDoNotBothCallPolly(t *testing.T) {
	e := newVoiceEnv(t)
	e.synth.gate = make(chan struct{})
	// A second Lambda instance: its own handler and locks, the same database and bucket.
	h2 := newShortsTestHandler(t, e.h.DB.(*mockDynamoDB))
	h2.TableName = e.h.TableName
	e.wire(h2)
	old := voicePollEvery
	voicePollEvery = 5 * time.Millisecond
	t.Cleanup(func() { voicePollEvery = old })

	var wg sync.WaitGroup
	codes := make([]int, 2)
	for i, h := range []*APIHandler{e.h, h2} {
		wg.Add(1)
		go func(i int, h *APIHandler) {
			defer wg.Done()
			raw, _ := json.Marshal(e.body(1))
			resp, _ := h.HandleRequest(context.Background(), events.APIGatewayProxyRequest{
				HTTPMethod: "POST", Path: "/story-plans/" + e.planID + "/voice-preview", Body: string(raw),
				Headers: map[string]string{"Authorization": "Bearer " + generateTestToken("user1")},
			})
			codes[i] = resp.StatusCode
		}(i, h)
	}
	deadline := time.Now().Add(2 * time.Second)
	for e.synth.n.Load() < 1 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	time.Sleep(150 * time.Millisecond) // the other instance is now waiting on the lease
	close(e.synth.gate)
	wg.Wait()
	if codes[0] != 200 || codes[1] != 200 {
		t.Fatalf("codes %v", codes)
	}
	if e.synth.n.Load() != 1 || e.used("user1", quotaVoicePreview) != 1 {
		t.Errorf("polly %d, uses %d; want one of each across instances", e.synth.n.Load(), e.used("user1", quotaVoicePreview))
	}
}

func TestVoicePreview_AWaiterGivesUpWhenTheLeaseHolderNeverFinishes(t *testing.T) {
	e := newVoiceEnv(t)
	oldWait, oldPoll := voiceWaitFor, voicePollEvery
	voiceWaitFor, voicePollEvery = 60*time.Millisecond, 5*time.Millisecond
	t.Cleanup(func() { voiceWaitFor, voicePollEvery = oldWait, oldPoll })
	// Another instance holds the lease for this take and never produces it.
	text := samplePlan("en").Scenes[0].Narration
	key := storyjob.PreviewKey("user1", text, "Joanna")
	e.leases["VOICELOCK#"+key] = time.Now().Add(time.Minute).UnixMilli()
	resp := e.preview(t, "user1", e.body(1))
	if resp.StatusCode != 429 || decodeErr(t, resp).Code != "VOICE_BUSY" || resp.Headers["Retry-After"] == "" {
		t.Errorf("%d %s", resp.StatusCode, resp.Body)
	}
	if e.synth.n.Load() != 0 || e.used("user1", quotaVoicePreview) != 0 {
		t.Error("a waiter that gives up must not call polly or spend")
	}
}

func TestVoicePreview_AnExpiredLeaseIsTakenOver(t *testing.T) {
	e := newVoiceEnv(t)
	key := storyjob.PreviewKey("user1", samplePlan("en").Scenes[0].Narration, "Joanna")
	e.leases["VOICELOCK#"+key] = time.Now().Add(-time.Minute).UnixMilli()
	e.ok(t, "user1", e.body(1))
	if e.synth.n.Load() != 1 {
		t.Errorf("polly %d, want 1", e.synth.n.Load())
	}
}

func TestVoicePreview_WorksWithoutAQuotaTable(t *testing.T) {
	e := newVoiceEnv(t)
	e.h.QuotaTable = ""
	if out := e.ok(t, "user1", e.body(1)); out.Cached || out.RemainingToday != nil {
		t.Errorf("%+v: with quotas off a take is made and no count is shown", out)
	}
	if !e.ok(t, "user1", e.body(1)).Cached {
		t.Error("replay must still be cached")
	}
}

func TestKeyedLocks_SerialiseOneKeyAndLetOthersThroughAndGiveUpOnCancel(t *testing.T) {
	var k keyedLocks
	ctx := context.Background()
	unlock, err := k.lock(ctx, "a")
	if err != nil {
		t.Fatal(err)
	}
	other, err := k.lock(ctx, "b")
	if err != nil {
		t.Fatalf("another key must not wait: %v", err)
	}
	other()
	cctx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() { _, err := k.lock(cctx, "a"); done <- err }()
	select {
	case err := <-done:
		t.Fatalf("the second lock on a key must wait, got %v", err)
	case <-time.After(30 * time.Millisecond):
	}
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Errorf("a cancelled waiter must return the context error, got %v", err)
	}
	unlock()
	again, err := k.lock(ctx, "a")
	if err != nil {
		t.Fatal(err)
	}
	again()
	if len(k.m) != 0 {
		t.Errorf("%d locks left in the table", len(k.m))
	}
}

func TestVoicePreview_TheEngineInTheCacheKeyIsTheOneThePollyServiceUses(t *testing.T) {
	for _, v := range narrator.SupportedVoices {
		if got := narrator.EngineFor(v); got != storyjob.PreviewEngine {
			t.Errorf("%s is made with %q but the cache key says %q: previews would not match renders", v, got, storyjob.PreviewEngine)
		}
	}
	if narrator.EngineFor("Nobody") != "" {
		t.Error("an unknown voice has no engine")
	}
}

func TestVoicePreviewPath(t *testing.T) {
	for path, want := range map[string]string{
		"/story-plans/abc/voice-preview":    "abc",
		"/story-plans/abc/voice-preview/x":  "",
		"/story-plans/limits/voice-preview": "",
		"/story-plans/abc":                  "",
		"/story-plans//voice-preview":       "",
		"/other/abc/voice-preview":          "",
	} {
		if got := voicePreviewPath(path); got != want {
			t.Errorf("%s -> %q, want %q", path, got, want)
		}
	}
}

// nopMailer sends nothing: the confirmation email is not what these tests are about.
type nopMailer struct{}

func (nopMailer) SendVerification(context.Context, string, string) error { return nil }

func TestVoicePreview_LimitsSayWhetherThePageMayOfferIt(t *testing.T) {
	e := newVoiceEnv(t)
	read := func() StoryPlanLimits {
		var l StoryPlanLimits
		_ = json.Unmarshal([]byte(e.do(t, "GET", "/story-plans/limits", "", "").Body), &l)
		return l
	}
	if l := read(); !l.VoicePreview || l.VoicePreviewsPerDay != 60 {
		t.Errorf("on: %+v", l)
	}
	e.h.VoiceSynth = nil
	if l := read(); l.VoicePreview {
		t.Errorf("off: %+v, the page must not offer a button that answers 503", l)
	}
}
