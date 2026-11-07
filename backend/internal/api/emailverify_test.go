package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/rahul/indifferent/backend/internal/auth"
	"github.com/rahul/indifferent/backend/internal/models"
)

type recordingVerifyMailer struct {
	to    []string
	links []string
	err   error
}

func (r *recordingVerifyMailer) SendVerification(ctx context.Context, to, link string) error {
	if r.err != nil {
		return r.err
	}
	r.to = append(r.to, to)
	r.links = append(r.links, link)
	return nil
}

// verifyEnv is a handler wired for email confirmation over a small in-memory
// store that understands the few DynamoDB operations confirmation uses.
type verifyEnv struct {
	h      *APIHandler
	db     *mockDynamoDB
	items  map[string]map[string]dbtypes.AttributeValue
	counts map[string]int64
	mail   *recordingVerifyMailer
	getErr error
}

func kvKey(table *string, key map[string]dbtypes.AttributeValue) string {
	return aws.ToString(table) + "|" + key["PK"].(*dbtypes.AttributeValueMemberS).Value + "|" + key["SK"].(*dbtypes.AttributeValueMemberS).Value
}

func newVerifyEnv(t *testing.T) *verifyEnv {
	t.Helper()
	e := &verifyEnv{items: map[string]map[string]dbtypes.AttributeValue{}, counts: map[string]int64{}, mail: &recordingVerifyMailer{}}
	e.db = &mockDynamoDB{
		getItemFunc: func(ctx context.Context, p *dynamodb.GetItemInput, o ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
			if e.getErr != nil && aws.ToString(p.TableName) == "users-table" {
				return nil, e.getErr
			}
			return &dynamodb.GetItemOutput{Item: e.items[kvKey(p.TableName, p.Key)]}, nil
		},
		putItemFunc: func(ctx context.Context, p *dynamodb.PutItemInput, o ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
			e.items[kvKey(p.TableName, map[string]dbtypes.AttributeValue{"PK": p.Item["PK"], "SK": p.Item["SK"]})] = p.Item
			return &dynamodb.PutItemOutput{}, nil
		},
		deleteItemFunc: func(ctx context.Context, p *dynamodb.DeleteItemInput, o ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error) {
			k := kvKey(p.TableName, p.Key)
			old := e.items[k]
			if p.ConditionExpression != nil {
				exp, _ := old["expiresAt"].(*dbtypes.AttributeValueMemberN)
				now, _ := strconv.ParseInt(p.ExpressionAttributeValues[":now"].(*dbtypes.AttributeValueMemberN).Value, 10, 64)
				if old == nil || exp == nil {
					return nil, &dbtypes.ConditionalCheckFailedException{Message: aws.String("condition failed")}
				}
				if at, _ := strconv.ParseInt(exp.Value, 10, 64); at <= now {
					return nil, &dbtypes.ConditionalCheckFailedException{Message: aws.String("condition failed")}
				}
			}
			delete(e.items, k)
			return &dynamodb.DeleteItemOutput{Attributes: old}, nil
		},
		updateItemFunc: func(ctx context.Context, p *dynamodb.UpdateItemInput, o ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
			if _, ok := p.ExpressionAttributeValues[":one"]; ok { // quota counter
				k := kvKey(p.TableName, p.Key)
				e.counts[k]++
				return &dynamodb.UpdateItemOutput{Attributes: map[string]dbtypes.AttributeValue{"uses": &dbtypes.AttributeValueMemberN{Value: strconv.FormatInt(e.counts[k], 10)}}}, nil
			}
			if p.UpdateExpression != nil && strings.Contains(*p.UpdateExpression, "emailVerified") {
				item := e.items[kvKey(p.TableName, p.Key)]
				want := p.ExpressionAttributeValues[":email"].(*dbtypes.AttributeValueMemberS).Value
				if item == nil || item["email"].(*dbtypes.AttributeValueMemberS).Value != want {
					return nil, &dbtypes.ConditionalCheckFailedException{Message: aws.String("condition failed")}
				}
				item["emailVerified"] = p.ExpressionAttributeValues[":yes"]
			}
			return &dynamodb.UpdateItemOutput{}, nil
		},
	}
	e.h = newShortsTestHandler(t, e.db)
	e.h.UsersTable = "users-table"
	e.h.QuotaTable = "sessions-table"
	e.h.Verifier = &auth.EmailVerifier{DB: e.db, UsersTable: "users-table", TokenTable: "sessions-table", BaseURL: "https://indifferent.fun", Mailer: e.mail}
	e.h.JWTService.UsersTable = "users-table"
	e.h.PasswordAuth = auth.NewPasswordAuthService(e.db, e.h.JWTService, "users-table", "sessions-table")
	e.h.PasswordAuth.Verifier = e.h.Verifier
	return e
}

// profile stores a user profile. confirmed: nil leaves the mark off.
func (e *verifyEnv) profile(userID, email string, confirmed *bool) {
	item := map[string]dbtypes.AttributeValue{
		"PK":    &dbtypes.AttributeValueMemberS{Value: "USER#" + userID},
		"SK":    &dbtypes.AttributeValueMemberS{Value: "PROFILE"},
		"email": &dbtypes.AttributeValueMemberS{Value: email},
		"name":  &dbtypes.AttributeValueMemberS{Value: "N"},
	}
	if confirmed != nil {
		item["emailVerified"] = &dbtypes.AttributeValueMemberBOOL{Value: *confirmed}
	}
	e.items["users-table|USER#"+userID+"|PROFILE"] = item
}

func (e *verifyEnv) call(method, path, token, body string) events.APIGatewayProxyResponse {
	headers := map[string]string{}
	if token != "" {
		headers["Authorization"] = "Bearer " + token
	}
	resp, _ := e.h.HandleRequest(context.Background(), events.APIGatewayProxyRequest{HTTPMethod: method, Path: path, Headers: headers, Body: body})
	return resp
}

func ptr(b bool) *bool { return &b }

func codeOf(t *testing.T, body string) string {
	t.Helper()
	var e models.APIError
	_ = json.Unmarshal([]byte(body), &e)
	return e.Code
}

func TestSpendingRequest(t *testing.T) {
	for _, tc := range []struct {
		method, path string
		want         bool
	}{
		{"POST", "/projects", true},
		{"POST", "/shorts", true},
		{"POST", "/projects/p1/start", true},
		{"POST", "/projects/p1/upload", true},
		{"POST", "/projects/p1/logo", true},
		{"POST", "/shorts/j1/start", true},
		{"POST", "/shorts/j1/retry", true},
		{"POST", "/shorts/j1/thumbnails", true},
		{"POST", "/shorts/j1/upload/multipart/initiate", true},
		{"POST", "/story-plans", true},
		{"POST", "/story-videos", true},
		{"GET", "/projects", false},
		{"GET", "/shorts", false},
		{"GET", "/projects/p1/logo", false},
		{"GET", "/story-plans/limits", false},
		{"DELETE", "/projects/p1", false},
		{"DELETE", "/shorts/j1", false},
		{"PUT", "/shorts/j1/style", false},
		{"PUT", "/projects/p1", false},
		{"POST", "/shorts/j1/cancel", false},
		{"POST", "/auth/logout-all", false},
		{"POST", "/auth/resend-verification", false},
		{"POST", "/projects/p1/slide-preview", false},
	} {
		got := spendingRequest(events.APIGatewayProxyRequest{HTTPMethod: tc.method, Path: tc.path})
		if got != tc.want {
			t.Errorf("%s %s: spending = %v, want %v", tc.method, tc.path, got, tc.want)
		}
	}
}

func TestEmailGate_HoldsSpendingRequestsOfAnUnconfirmedAccount(t *testing.T) {
	e := newVerifyEnv(t)
	e.profile("u1", "u1@example.com", ptr(false))
	tok := generateTestToken("u1")
	for _, tc := range [][2]string{
		{"POST", "/projects"}, {"POST", "/shorts"}, {"POST", "/projects/p1/start"}, {"POST", "/projects/p1/upload"},
		{"POST", "/projects/p1/logo"}, {"POST", "/shorts/j1/start"}, {"POST", "/shorts/j1/retry"},
		{"POST", "/shorts/j1/thumbnails"}, {"POST", "/shorts/j1/upload/multipart/initiate"},
		{"POST", "/story-plans"}, {"POST", "/story-videos"},
	} {
		resp := e.call(tc[0], tc[1], tok, `{"fileType":"mp4","duration":60}`)
		if resp.StatusCode != 403 || codeOf(t, resp.Body) != "EMAIL_NOT_VERIFIED" {
			t.Errorf("%s %s: %d %s, want 403 EMAIL_NOT_VERIFIED", tc[0], tc[1], resp.StatusCode, resp.Body)
		}
	}
	if !strings.Contains(e.call("POST", "/shorts", tok, "{}").Body, "account page") {
		t.Error("the message does not say where to ask for a new link")
	}
}

func TestEmailGate_LeavesEverythingElseAlone(t *testing.T) {
	e := newVerifyEnv(t)
	e.profile("u1", "u1@example.com", ptr(false))
	tok := generateTestToken("u1")
	for _, tc := range [][2]string{
		{"GET", "/projects"}, {"GET", "/shorts"}, {"GET", "/auth/me"}, {"DELETE", "/projects/p1"},
		{"DELETE", "/shorts/j1"}, {"PUT", "/shorts/j1/style"}, {"POST", "/shorts/j1/cancel"}, {"POST", "/auth/logout-all"},
	} {
		if resp := e.call(tc[0], tc[1], tok, "{}"); resp.StatusCode == 403 {
			t.Errorf("%s %s was held back: %s", tc[0], tc[1], resp.Body)
		}
	}
}

func TestEmailGate_LetsThroughEveryoneWhoIsNotMarkedUnconfirmed(t *testing.T) {
	body := `{"fileType":"mp4","duration":60}`
	t.Run("confirmed", func(t *testing.T) {
		e := newVerifyEnv(t)
		e.profile("u1", "u1@example.com", ptr(true))
		if resp := e.call("POST", "/shorts", generateTestToken("u1"), body); resp.StatusCode != 201 {
			t.Errorf("%d %s", resp.StatusCode, resp.Body)
		}
	})
	t.Run("account from before confirmation existed", func(t *testing.T) {
		e := newVerifyEnv(t)
		e.profile("u1", "u1@example.com", nil)
		if resp := e.call("POST", "/shorts", generateTestToken("u1"), body); resp.StatusCode != 201 {
			t.Errorf("%d %s", resp.StatusCode, resp.Body)
		}
	})
	t.Run("feature switched off", func(t *testing.T) {
		e := newVerifyEnv(t)
		e.profile("u1", "u1@example.com", ptr(false))
		e.h.Verifier = nil
		if resp := e.call("POST", "/shorts", generateTestToken("u1"), body); resp.StatusCode != 201 {
			t.Errorf("an unconfirmed account must not be held when the feature is off: %d %s", resp.StatusCode, resp.Body)
		}
	})
	t.Run("no profile", func(t *testing.T) {
		e := newVerifyEnv(t)
		if resp := e.call("POST", "/shorts", generateTestToken("ghost"), body); resp.StatusCode != 201 {
			t.Errorf("%d %s", resp.StatusCode, resp.Body)
		}
	})
	t.Run("lookup fails", func(t *testing.T) {
		e := newVerifyEnv(t)
		e.profile("u1", "u1@example.com", ptr(false))
		e.getErr = errors.New("dynamodb is down")
		if resp := e.call("POST", "/shorts", generateTestToken("u1"), body); resp.StatusCode != 201 {
			t.Errorf("a DynamoDB fault must not lock users out: %d %s", resp.StatusCode, resp.Body)
		}
	})
}

// The whole path: sign up, be held, open the link, be let through.
func TestEmailConfirmation_EndToEnd(t *testing.T) {
	e := newVerifyEnv(t)
	signup := e.call("POST", "/auth/signup", "", `{"email":"Flow@Example.com","name":"F","password":"correct-horse"}`)
	if signup.StatusCode != 201 {
		t.Fatalf("signup: %d %s", signup.StatusCode, signup.Body)
	}
	var tokens struct {
		AccessToken string `json:"accessToken"`
	}
	_ = json.Unmarshal([]byte(signup.Body), &tokens)
	if len(e.mail.links) != 1 || e.mail.to[0] != "flow@example.com" {
		t.Fatalf("mail sent: %v", e.mail.to)
	}

	body := `{"fileType":"mp4","duration":60}`
	if resp := e.call("POST", "/shorts", tokens.AccessToken, body); resp.StatusCode != 403 {
		t.Fatalf("before confirming: %d, want 403", resp.StatusCode)
	}
	me := e.call("GET", "/auth/me", tokens.AccessToken, "")
	if !strings.Contains(me.Body, `"emailVerified":false`) {
		t.Errorf("/auth/me = %s, want emailVerified false", me.Body)
	}

	u, _ := url.Parse(e.mail.links[0])
	token := u.Query().Get("token")
	if resp := e.call("POST", "/auth/verify-email", "", `{"token":"`+token+`"}`); resp.StatusCode != 204 {
		t.Fatalf("verify: %d %s", resp.StatusCode, resp.Body)
	}
	if resp := e.call("POST", "/shorts", tokens.AccessToken, body); resp.StatusCode != 201 {
		t.Errorf("after confirming: %d %s, want 201", resp.StatusCode, resp.Body)
	}
	me = e.call("GET", "/auth/me", tokens.AccessToken, "")
	if !strings.Contains(me.Body, `"emailVerified":true`) {
		t.Errorf("/auth/me = %s, want emailVerified true", me.Body)
	}

	// The same link a second time.
	resp := e.call("POST", "/auth/verify-email", "", `{"token":"`+token+`"}`)
	if resp.StatusCode != 400 || codeOf(t, resp.Body) != "INVALID_TOKEN" {
		t.Errorf("reused link: %d %s, want 400 INVALID_TOKEN", resp.StatusCode, resp.Body)
	}
}

func TestVerifyEmailEndpoint_RefusesBadInput(t *testing.T) {
	e := newVerifyEnv(t)
	for name, body := range map[string]string{
		"empty body":    "",
		"not json":      "<token>",
		"no token":      `{}`,
		"short token":   `{"token":"abc"}`,
		"unknown token": `{"token":"` + strings.Repeat("ab", 32) + `"}`,
		"huge body":     `{"token":"` + strings.Repeat("a", 2000) + `"}`,
	} {
		resp := e.call("POST", "/auth/verify-email", "", body)
		if resp.StatusCode != 400 {
			t.Errorf("%s: %d %s, want 400", name, resp.StatusCode, resp.Body)
		}
	}
	e.h.Verifier = nil
	if resp := e.call("POST", "/auth/verify-email", "", `{"token":"`+strings.Repeat("ab", 32)+`"}`); resp.StatusCode != 503 {
		t.Errorf("feature off: %d, want 503", resp.StatusCode)
	}
}

func TestResendVerification(t *testing.T) {
	e := newVerifyEnv(t)
	e.profile("u1", "real@example.com", ptr(false))
	tok := generateTestToken("u1")

	if resp := e.call("POST", "/auth/resend-verification", "", ""); resp.StatusCode != 401 {
		t.Fatalf("without sign-in: %d, want 401", resp.StatusCode)
	}
	// The address is the stored one, whatever the request claims.
	if resp := e.call("POST", "/auth/resend-verification", tok, `{"email":"attacker@example.com"}`); resp.StatusCode != 204 {
		t.Fatalf("resend: %d %s", resp.StatusCode, resp.Body)
	}
	if len(e.mail.to) != 1 || e.mail.to[0] != "real@example.com" {
		t.Fatalf("mail went to %v, want the stored address only", e.mail.to)
	}

	// A link from a resend works.
	u, _ := url.Parse(e.mail.links[0])
	if resp := e.call("POST", "/auth/verify-email", "", `{"token":"`+u.Query().Get("token")+`"}`); resp.StatusCode != 204 {
		t.Fatalf("verify: %d %s", resp.StatusCode, resp.Body)
	}
	// Already confirmed: nothing more is sent.
	if resp := e.call("POST", "/auth/resend-verification", tok, ""); resp.StatusCode != 204 || len(e.mail.to) != 1 {
		t.Errorf("confirmed account: %d, %d mails; want 204 and no new mail", resp.StatusCode, len(e.mail.to))
	}
}

func TestResendVerification_IsLimitedPerDay(t *testing.T) {
	e := newVerifyEnv(t)
	e.profile("u1", "real@example.com", ptr(false))
	tok := generateTestToken("u1")
	fixedQuotaClock(t, time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC))
	for i := 1; i <= int(quotaVerifyEmail.limit); i++ {
		if resp := e.call("POST", "/auth/resend-verification", tok, ""); resp.StatusCode != 204 {
			t.Fatalf("send %d: %d %s", i, resp.StatusCode, resp.Body)
		}
	}
	resp := e.call("POST", "/auth/resend-verification", tok, "")
	if resp.StatusCode != 429 || codeOf(t, resp.Body) != "QUOTA_EXCEEDED" {
		t.Fatalf("over the limit: %d %s, want 429 QUOTA_EXCEEDED", resp.StatusCode, resp.Body)
	}
	if got := len(e.mail.to); got != int(quotaVerifyEmail.limit) {
		t.Errorf("%d mails sent, want %d", got, quotaVerifyEmail.limit)
	}
}

func TestResendVerification_ReportsFailureAndSwitchedOff(t *testing.T) {
	e := newVerifyEnv(t)
	e.profile("u1", "real@example.com", ptr(false))
	tok := generateTestToken("u1")
	e.mail.err = errors.New("ses down")
	if resp := e.call("POST", "/auth/resend-verification", tok, ""); resp.StatusCode != 500 {
		t.Errorf("mail failure: %d, want 500", resp.StatusCode)
	}
	for k := range e.items {
		if strings.Contains(k, "EMAILVERIFY#") {
			t.Error("a token for a mail that was not sent was left valid")
		}
	}
	e.h.Verifier = nil
	if resp := e.call("POST", "/auth/resend-verification", tok, ""); resp.StatusCode != 503 {
		t.Errorf("feature off: %d, want 503", resp.StatusCode)
	}
}

func TestAuthMe_ReportsEmailVerified(t *testing.T) {
	e := newVerifyEnv(t)
	for _, tc := range []struct {
		name      string
		confirmed *bool
		hash      bool
		want      string
	}{
		{"unconfirmed password sign-up", ptr(false), true, `"emailVerified":false`},
		{"confirmed password sign-up", ptr(true), true, `"emailVerified":true`},
		{"google account", nil, false, `"emailVerified":true`},
		{"old password account", nil, true, `"emailVerified":false`},
	} {
		e.profile("u-"+tc.name, "x@example.com", tc.confirmed)
		if tc.hash {
			e.items["users-table|USER#u-"+tc.name+"|PROFILE"]["passwordHash"] = &dbtypes.AttributeValueMemberS{Value: "$2a$x"}
		}
		resp := e.call("GET", "/auth/me", generateTestToken("u-"+tc.name), "")
		if !strings.Contains(resp.Body, tc.want) {
			t.Errorf("%s: %s, want %s", tc.name, resp.Body, tc.want)
		}
	}
}
