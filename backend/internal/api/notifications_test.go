package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

func notifyReq(method, body string) events.APIGatewayProxyRequest {
	return events.APIGatewayProxyRequest{
		HTTPMethod: method,
		Path:       "/settings/notifications",
		Headers:    map[string]string{"Authorization": "Bearer " + generateTestToken("user1")},
		Body:       body,
	}
}

func s(v string) dbtypes.AttributeValue { return &dbtypes.AttributeValueMemberS{Value: v} }
func b(v bool) dbtypes.AttributeValue   { return &dbtypes.AttributeValueMemberBOOL{Value: v} }

// profileStore is a one-item users table that applies the two SET
// expressions the handler issues, so tests can check persistence end to end.
type profileStore struct {
	item    map[string]dbtypes.AttributeValue // nil = no profile
	updates []*dynamodb.UpdateItemInput
}

func (p *profileStore) handler() *APIHandler {
	h := newTestHandler()
	h.DB = &mockDynamoDB{
		getItemFunc: func(_ context.Context, in *dynamodb.GetItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
			return &dynamodb.GetItemOutput{Item: p.item}, nil
		},
		updateItemFunc: func(_ context.Context, in *dynamodb.UpdateItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
			p.updates = append(p.updates, in)
			if p.item == nil {
				return nil, &dbtypes.ConditionalCheckFailedException{}
			}
			for placeholder, name := range in.ExpressionAttributeNames {
				p.item[name] = in.ExpressionAttributeValues[":"+strings.TrimPrefix(placeholder, "#")]
			}
			return &dynamodb.UpdateItemOutput{}, nil
		},
	}
	return h
}

func decodeSettings(t *testing.T, resp events.APIGatewayProxyResponse) NotificationSettings {
	t.Helper()
	var out NotificationSettings
	if err := json.Unmarshal([]byte(resp.Body), &out); err != nil {
		t.Fatalf("invalid JSON %q: %v", resp.Body, err)
	}
	return out
}

func TestNotificationSettingsRequireAuth(t *testing.T) {
	h := newTestHandler()
	for _, method := range []string{"GET", "PUT"} {
		resp, err := h.HandleRequest(context.Background(), events.APIGatewayProxyRequest{HTTPMethod: method, Path: "/settings/notifications", Body: `{"notifyFailed":false}`})
		if err != nil || resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("%s without token: status %d err %v, want 401", method, resp.StatusCode, err)
		}
	}
	resp, _ := h.HandleRequest(context.Background(), notifyReq("POST", `{}`))
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("POST should not be routed, got %d", resp.StatusCode)
	}
}

func TestGetNotificationSettings(t *testing.T) {
	ctx := context.Background()

	t.Run("defaults are on for a Google profile", func(t *testing.T) {
		p := &profileStore{item: map[string]dbtypes.AttributeValue{"email": s("g@example.com")}}
		resp, _ := p.handler().HandleRequest(ctx, notifyReq("GET", ""))
		if resp.StatusCode != 200 {
			t.Fatalf("status %d: %s", resp.StatusCode, resp.Body)
		}
		got := decodeSettings(t, resp)
		want := NotificationSettings{Email: "g@example.com", EmailVerified: true, NotifyCompleted: true, NotifyFailed: true}
		if got != want {
			t.Fatalf("got %+v, want %+v", got, want)
		}
	})

	t.Run("password account is reported as unverified and never leaks the hash", func(t *testing.T) {
		p := &profileStore{item: map[string]dbtypes.AttributeValue{"email": s("p@example.com"), "passwordHash": s("must-never-leak")}}
		resp, _ := p.handler().HandleRequest(ctx, notifyReq("GET", ""))
		got := decodeSettings(t, resp)
		if got.EmailVerified {
			t.Fatal("password accounts must report emailVerified=false")
		}
		if strings.Contains(resp.Body, "must-never-leak") || strings.Contains(strings.ToLower(resp.Body), "passwordhash") {
			t.Fatalf("response leaks the password hash: %s", resp.Body)
		}
	})

	t.Run("stored values are returned", func(t *testing.T) {
		p := &profileStore{item: map[string]dbtypes.AttributeValue{"email": s("g@example.com"), "notifyCompleted": b(false), "notifyFailed": b(true)}}
		resp, _ := p.handler().HandleRequest(ctx, notifyReq("GET", ""))
		got := decodeSettings(t, resp)
		if got.NotifyCompleted || !got.NotifyFailed {
			t.Fatalf("got %+v", got)
		}
	})

	t.Run("unknown user is 404", func(t *testing.T) {
		p := &profileStore{}
		resp, _ := p.handler().HandleRequest(ctx, notifyReq("GET", ""))
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("status %d, want 404", resp.StatusCode)
		}
	})

	t.Run("missing users table is a configuration error", func(t *testing.T) {
		h := (&profileStore{}).handler()
		h.UsersTable = ""
		resp, _ := h.HandleRequest(ctx, notifyReq("GET", ""))
		if resp.StatusCode != http.StatusInternalServerError {
			t.Fatalf("status %d, want 500", resp.StatusCode)
		}
	})
}

func TestUpdateNotificationSettings(t *testing.T) {
	ctx := context.Background()

	t.Run("persists both values and a later read returns them", func(t *testing.T) {
		p := &profileStore{item: map[string]dbtypes.AttributeValue{"email": s("g@example.com")}}
		h := p.handler()

		resp, _ := h.HandleRequest(ctx, notifyReq("PUT", `{"notifyCompleted":false,"notifyFailed":false}`))
		if resp.StatusCode != 200 {
			t.Fatalf("status %d: %s", resp.StatusCode, resp.Body)
		}
		if got := decodeSettings(t, resp); got.NotifyCompleted || got.NotifyFailed {
			t.Fatalf("PUT response %+v", got)
		}

		// "Across sessions": a fresh GET sees the stored values.
		resp, _ = h.HandleRequest(ctx, notifyReq("GET", ""))
		if got := decodeSettings(t, resp); got.NotifyCompleted || got.NotifyFailed {
			t.Fatalf("GET after PUT %+v", got)
		}

		up := p.updates[0]
		if *up.TableName != "users-table" {
			t.Errorf("table %q, want users-table", *up.TableName)
		}
		if pk := up.Key["PK"].(*dbtypes.AttributeValueMemberS).Value; pk != "USER#user1" {
			t.Errorf("PK %q, want USER#user1 (taken from the token)", pk)
		}
		if sk := up.Key["SK"].(*dbtypes.AttributeValueMemberS).Value; sk != "PROFILE" {
			t.Errorf("SK %q, want PROFILE", sk)
		}
		if up.ConditionExpression == nil || *up.ConditionExpression != "attribute_exists(PK)" {
			t.Error("update must not create a profile")
		}
	})

	t.Run("a single field changes only that field", func(t *testing.T) {
		p := &profileStore{item: map[string]dbtypes.AttributeValue{"email": s("g@example.com")}}
		resp, _ := p.handler().HandleRequest(ctx, notifyReq("PUT", `{"notifyFailed":false}`))
		if resp.StatusCode != 200 {
			t.Fatalf("status %d: %s", resp.StatusCode, resp.Body)
		}
		got := decodeSettings(t, resp)
		if !got.NotifyCompleted || got.NotifyFailed {
			t.Fatalf("got %+v, want completed=true failed=false", got)
		}
		if _, touched := p.item["notifyCompleted"]; touched {
			t.Fatal("notifyCompleted must not be written when absent from the body")
		}
		if expr := *p.updates[0].UpdateExpression; expr != "SET #f = :f" {
			t.Fatalf("update expression %q", expr)
		}
	})

	t.Run("both fields build a valid expression", func(t *testing.T) {
		p := &profileStore{item: map[string]dbtypes.AttributeValue{"email": s("g@example.com")}}
		p.handler().HandleRequest(ctx, notifyReq("PUT", `{"notifyCompleted":true,"notifyFailed":false}`))
		if expr := *p.updates[0].UpdateExpression; expr != "SET #c = :c, #f = :f" {
			t.Fatalf("update expression %q", expr)
		}
	})

	t.Run("strict validation rejects bad bodies without writing", func(t *testing.T) {
		bad := map[string]string{
			"empty body":             ``,
			"not json":               `nope`,
			"empty object":           `{}`,
			"array":                  `[true]`,
			"string instead of bool": `{"notifyCompleted":"false"}`,
			"number instead of bool": `{"notifyFailed":0}`,
			"null values":            `{"notifyCompleted":null,"notifyFailed":null}`,
			"unknown field":          `{"notifyCompleted":true,"email":"other@example.com"}`,
			"cannot target a user":   `{"userId":"someone-else","notifyFailed":false}`,
			"weekly summary":         `{"weeklySummary":true}`,
			"trailing second object": `{"notifyCompleted":true}{"notifyFailed":false}`,
		}
		for name, body := range bad {
			t.Run(name, func(t *testing.T) {
				p := &profileStore{item: map[string]dbtypes.AttributeValue{"email": s("g@example.com")}}
				resp, _ := p.handler().HandleRequest(ctx, notifyReq("PUT", body))
				if resp.StatusCode != http.StatusBadRequest {
					t.Fatalf("status %d, want 400 (body %q)", resp.StatusCode, resp.Body)
				}
				if len(p.updates) != 0 {
					t.Fatal("a rejected request must not write")
				}
			})
		}
	})

	t.Run("unknown user is 404", func(t *testing.T) {
		p := &profileStore{}
		resp, _ := p.handler().HandleRequest(ctx, notifyReq("PUT", `{"notifyFailed":false}`))
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("status %d, want 404", resp.StatusCode)
		}
	})

	t.Run("database error is 500", func(t *testing.T) {
		h := newTestHandler()
		h.DB = &mockDynamoDB{updateItemFunc: func(context.Context, *dynamodb.UpdateItemInput, ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
			return nil, context.DeadlineExceeded
		}}
		resp, _ := h.HandleRequest(ctx, notifyReq("PUT", `{"notifyFailed":false}`))
		if resp.StatusCode != http.StatusInternalServerError {
			t.Fatalf("status %d, want 500", resp.StatusCode)
		}
	})
}
