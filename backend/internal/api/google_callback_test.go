package api

import (
	"context"
	"strings"
	"testing"

	"github.com/aws/aws-lambda-go/events"

	"github.com/rahul/indifferent/backend/internal/models"
)

// The callback passes the browser's PKCE verifier on to Google (SEC-07). It is
// optional, so that a page loaded before the verifier existed can still finish a
// sign-in, but when it is there it must be well formed.

func callbackWith(t *testing.T, body string) (resp events.APIGatewayProxyResponse, gotCode, gotVerifier string, called bool) {
	t.Helper()
	h := newTestHandler()
	h.AuthService = &mockAuthService{authenticateFunc: func(_ context.Context, code, verifier string) (*models.AuthTokens, error) {
		called, gotCode, gotVerifier = true, code, verifier
		return &models.AuthTokens{AccessToken: "a", RefreshToken: "r", ExpiresIn: 900}, nil
	}}
	resp, err := h.HandleRequest(context.Background(), events.APIGatewayProxyRequest{
		HTTPMethod: "POST", Path: "/auth/google/callback", Body: body,
	})
	if err != nil {
		t.Fatal(err)
	}
	return resp, gotCode, gotVerifier, called
}

func TestGoogleCallback_ForwardsTheCodeVerifier(t *testing.T) {
	verifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	resp, code, got, called := callbackWith(t, `{"code":"abc","codeVerifier":"`+verifier+`"}`)
	if resp.StatusCode != 200 || !called {
		t.Fatalf("status %d called=%v (body: %s)", resp.StatusCode, called, resp.Body)
	}
	if code != "abc" || got != verifier {
		t.Errorf("AuthService got code %q verifier %q, want abc and the verifier", code, got)
	}
}

func TestGoogleCallback_StillAcceptsACodeWithNoVerifier(t *testing.T) {
	resp, _, got, called := callbackWith(t, `{"code":"abc"}`)
	if resp.StatusCode != 200 || !called || got != "" {
		t.Errorf("status %d called=%v verifier=%q; a page loaded before PKCE must still sign in", resp.StatusCode, called, got)
	}
}

func TestGoogleCallback_RefusesAMalformedVerifierBeforeCallingGoogle(t *testing.T) {
	for name, v := range map[string]string{
		"too short":       "short",
		"too long":        strings.Repeat("a", 129),
		"has a space":     strings.Repeat("a", 20) + " " + strings.Repeat("a", 30),
		"has a plus sign": strings.Repeat("a", 20) + "+" + strings.Repeat("a", 30),
	} {
		t.Run(name, func(t *testing.T) {
			resp, _, _, called := callbackWith(t, `{"code":"abc","codeVerifier":"`+v+`"}`)
			if resp.StatusCode != 400 {
				t.Errorf("status %d, want 400 (body: %s)", resp.StatusCode, resp.Body)
			}
			if called {
				t.Error("Google was asked to redeem the code although the verifier was malformed")
			}
		})
	}
}
