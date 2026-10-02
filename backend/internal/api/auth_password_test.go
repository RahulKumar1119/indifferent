package api

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/rahul/indifferent/backend/internal/auth"
)

// memPasswordDB is a minimal in-memory auth store: PK|SK keyed items with
// conditional-put support for the email reservation.
type memPasswordDB struct {
	items map[string]map[string]dbtypes.AttributeValue
}

func strAttr(item map[string]dbtypes.AttributeValue, key string) string {
	if v, ok := item[key].(*dbtypes.AttributeValueMemberS); ok {
		return v.Value
	}
	return ""
}

func memKey(item map[string]dbtypes.AttributeValue) string {
	return strAttr(item, "PK") + "|" + strAttr(item, "SK")
}

func newPasswordTestHandler() (*APIHandler, *memPasswordDB) {
	db := &memPasswordDB{items: map[string]map[string]dbtypes.AttributeValue{}}
	mock := &mockDynamoDB{
		putItemFunc: func(ctx context.Context, params *dynamodb.PutItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
			if params.ConditionExpression != nil && *params.ConditionExpression == "attribute_not_exists(PK)" {
				if _, exists := db.items[memKey(params.Item)]; exists {
					return nil, &dbtypes.ConditionalCheckFailedException{Message: aws.String("condition failed")}
				}
			}
			db.items[memKey(params.Item)] = params.Item
			return &dynamodb.PutItemOutput{}, nil
		},
		getItemFunc: func(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
			if item, ok := db.items[memKey(params.Key)]; ok {
				return &dynamodb.GetItemOutput{Item: item}, nil
			}
			return &dynamodb.GetItemOutput{}, nil
		},
		updateItemFunc: func(ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
			if item, ok := db.items[memKey(params.Key)]; ok {
				for k, v := range params.ExpressionAttributeValues {
					switch k {
					case ":r":
						if b, ok := v.(*dbtypes.AttributeValueMemberBOOL); ok {
							item["rotated"] = &dbtypes.AttributeValueMemberBOOL{Value: b.Value}
						}
					case ":e":
						if n, ok := v.(*dbtypes.AttributeValueMemberN); ok {
							item["expiresAt"] = &dbtypes.AttributeValueMemberN{Value: n.Value}
						}
					}
				}
			}
			return &dynamodb.UpdateItemOutput{}, nil
		},
		deleteItemFunc: func(ctx context.Context, params *dynamodb.DeleteItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error) {
			delete(db.items, memKey(params.Key))
			return &dynamodb.DeleteItemOutput{}, nil
		},
	}
	jwtService := &auth.JWTService{Secret: testSecret, DB: mock, SessionTable: "sessions-table"}
	h := newTestHandler()
	h.PasswordAuth = auth.NewPasswordAuthService(mock, jwtService, "users-table", "sessions-table")
	return h, db
}

func postJSON(t *testing.T, h *APIHandler, path, body string) events.APIGatewayProxyResponse {
	t.Helper()
	resp, err := h.HandleRequest(context.Background(), events.APIGatewayProxyRequest{
		HTTPMethod: "POST",
		Path:       path,
		Body:       body,
	})
	if err != nil {
		t.Fatalf("%s returned error: %v", path, err)
	}
	return resp
}

func TestPasswordRoutes_SignupLoginRoundTrip(t *testing.T) {
	h, _ := newPasswordTestHandler()

	resp := postJSON(t, h, "/auth/signup", `{"email":"new@example.com","name":"New","password":"s3cret-pass"}`)
	if resp.StatusCode != 201 {
		t.Fatalf("signup status %d, want 201 (body: %s)", resp.StatusCode, resp.Body)
	}
	var tokens struct {
		AccessToken  string `json:"accessToken"`
		RefreshToken string `json:"refreshToken"`
	}
	if err := json.Unmarshal([]byte(resp.Body), &tokens); err != nil || tokens.AccessToken == "" {
		t.Fatalf("signup did not return tokens: %s", resp.Body)
	}

	resp = postJSON(t, h, "/auth/login", `{"email":"NEW@example.com","password":"s3cret-pass"}`)
	if resp.StatusCode != 200 {
		t.Fatalf("login status %d, want 200 (body: %s)", resp.StatusCode, resp.Body)
	}
}

func TestPasswordRoutes_StatusMapping(t *testing.T) {
	h, _ := newPasswordTestHandler()
	postJSON(t, h, "/auth/signup", `{"email":"dup@example.com","name":"D","password":"password-1"}`)

	cases := []struct {
		name string
		path string
		body string
		want int
	}{
		{"duplicate signup is 409", "/auth/signup", `{"email":"dup@example.com","name":"D","password":"password-2"}`, 409},
		{"bad email is 400", "/auth/signup", `{"email":"nope","name":"D","password":"password-2"}`, 400},
		{"short password is 400", "/auth/signup", `{"email":"ok@example.com","name":"D","password":"short"}`, 400},
		{"wrong password is 401", "/auth/login", `{"email":"dup@example.com","password":"nope-nope-no"}`, 401},
		{"unknown user is 401", "/auth/login", `{"email":"ghost@example.com","password":"whatever-12"}`, 401},
	}
	for _, tc := range cases {
		resp := postJSON(t, h, tc.path, tc.body)
		if resp.StatusCode != tc.want {
			t.Errorf("%s: got %d, want %d (body: %s)", tc.name, resp.StatusCode, tc.want, resp.Body)
		}
	}

	// Unknown-user and wrong-password failures share one generic message.
	unknown := postJSON(t, h, "/auth/login", `{"email":"ghost@example.com","password":"whatever-12"}`)
	wrong := postJSON(t, h, "/auth/login", `{"email":"dup@example.com","password":"nope-nope-no"}`)
	if unknown.Body != wrong.Body {
		t.Errorf("login failures must match, got %q vs %q", unknown.Body, wrong.Body)
	}
}
