package api

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	dbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/rahul/indifferent/backend/internal/models"
)

// A project's logo key must only ever be the key the upload route made for it.
// Before this was enforced, POST /projects stored whatever branding.logoKey the
// client sent, and GET /projects/{id}/logo then signed a download link for that
// key, so one account could read any object in the bucket whose key it knew.

const foreignLogoKey = "uploads/victim-user/victim-project/logo.png"

func doAs(t *testing.T, h *APIHandler, user, method, path, body string) events.APIGatewayProxyResponse {
	t.Helper()
	resp, err := h.HandleRequest(context.Background(), events.APIGatewayProxyRequest{
		HTTPMethod: method,
		Path:       path,
		Headers:    map[string]string{"Authorization": "Bearer " + generateTestToken(user)},
		Body:       body,
	})
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	return resp
}

func createProjectAs(t *testing.T, h *APIHandler, user, body string) (id string, resp events.APIGatewayProxyResponse) {
	t.Helper()
	resp = doAs(t, h, user, "POST", "/projects", body)
	if resp.StatusCode != 201 {
		t.Fatalf("create project: status %d (body: %s)", resp.StatusCode, resp.Body)
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal([]byte(resp.Body), &created); err != nil || created.ID == "" {
		t.Fatalf("create project: no id in %s", resp.Body)
	}
	return created.ID, resp
}

// storedBranding returns the branding JSON saved on the project record.
func storedBranding(t *testing.T, db *captureDB, user, projectID string) string {
	t.Helper()
	item, ok := db.items["USER#"+user+"|PROJECT#"+projectID]
	if !ok {
		t.Fatalf("project record not found")
	}
	return strVal(item, "branding")
}

func TestCreateProject_NeverTakesTheLogoKeyFromTheRequest(t *testing.T) {
	db := newCaptureDB()
	h := newUnifiedTestHandler(db)

	id, resp := createProjectAs(t, h, "user1",
		`{"name":"P","template":"classic","voice":"Joanna","branding":{"logoKey":"`+foreignLogoKey+`","channelName":"  @studio  "}}`)

	var created struct {
		Branding *models.Branding `json:"branding"`
	}
	if err := json.Unmarshal([]byte(resp.Body), &created); err != nil {
		t.Fatal(err)
	}
	if created.Branding == nil || created.Branding.LogoKey != "" {
		t.Errorf("the response carries logoKey %+v, want none", created.Branding)
	}
	if created.Branding == nil || created.Branding.ChannelName != "@studio" {
		t.Errorf("the channel name must still be kept (and trimmed), got %+v", created.Branding)
	}
	if saved := storedBranding(t, db, "user1", id); strings.Contains(saved, "victim") || strings.Contains(saved, "logoKey") {
		t.Errorf("the saved record holds the client's key: %s", saved)
	}
	// And the logo route has nothing to sign.
	if got := doAs(t, h, "user1", "GET", "/projects/"+id+"/logo", ""); got.StatusCode != 404 || strings.Contains(got.Body, "victim") {
		t.Errorf("GET /logo = %d %s, want 404 and no link to another user's file", got.StatusCode, got.Body)
	}
}

func TestGetLogo_OnlySignsTheProjectsOwnLogo(t *testing.T) {
	db := newCaptureDB()
	h := newUnifiedTestHandler(db)
	id, _ := createProjectAs(t, h, "user1", `{"name":"P","template":"classic","voice":"Joanna"}`)

	// The legitimate flow still works end to end: the upload route records the key
	// and the logo route signs exactly that key.
	if up := doAs(t, h, "user1", "POST", "/projects/"+id+"/logo", ""); up.StatusCode != 200 {
		t.Fatalf("logo upload: %d %s", up.StatusCode, up.Body)
	}
	own := doAs(t, h, "user1", "GET", "/projects/"+id+"/logo", "")
	if own.StatusCode != 200 || !strings.Contains(own.Body, "uploads/user1/"+id+"/logo.png") {
		t.Fatalf("the project's own logo must be signed, got %d %s", own.StatusCode, own.Body)
	}

	// A record that already holds some other key (written before the create route
	// was fixed, or by any other path) must not be followed.
	for _, bad := range []string{
		foreignLogoKey,
		"uploads/user1/another-project/logo.png", // the user's own, but another project's
		"output/" + id + "/video.mp4",            // a rendered video of this very project
		"uploads/user1/" + id + "/input.txt",     // this project, wrong object
		"uploads/user1/" + id + "/../x/logo.png", // traversal
	} {
		item := db.items["USER#user1|PROJECT#"+id]
		item["branding"] = &dbtypes.AttributeValueMemberS{Value: `{"logoKey":"` + bad + `"}`}
		got := doAs(t, h, "user1", "GET", "/projects/"+id+"/logo", "")
		if got.StatusCode != 404 || strings.Contains(got.Body, "downloads.example") {
			t.Errorf("stored key %q: GET /logo = %d %s, want 404 and no signed link", bad, got.StatusCode, got.Body)
		}
	}
}

func TestProjectLogoKey_IsEmptyUnlessItIsTheProjectsOwnLogo(t *testing.T) {
	cases := []struct {
		name string
		p    models.Project
		want string
	}{
		{"no branding", models.Project{UserID: "u1", ProjectID: "p1"}, ""},
		{"no logo", models.Project{UserID: "u1", ProjectID: "p1", Branding: &models.Branding{ChannelName: "@s"}}, ""},
		{"its own logo", models.Project{UserID: "u1", ProjectID: "p1", Branding: &models.Branding{LogoKey: "uploads/u1/p1/logo.png"}}, "uploads/u1/p1/logo.png"},
		{"another user's logo", models.Project{UserID: "u1", ProjectID: "p1", Branding: &models.Branding{LogoKey: foreignLogoKey}}, ""},
		{"a rendered video", models.Project{UserID: "u1", ProjectID: "p1", Branding: &models.Branding{LogoKey: "output/p1/video.mp4"}}, ""},
		{"a record with no ids", models.Project{Branding: &models.Branding{LogoKey: "uploads//logo.png"}}, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := projectLogoKey(&c.p); got != c.want {
				t.Errorf("projectLogoKey = %q, want %q (this value is what the render pipeline is handed)", got, c.want)
			}
		})
	}
}
