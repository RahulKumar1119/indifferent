package api

import (
	"context"
	"strings"
	"testing"

	"github.com/aws/aws-lambda-go/events"
)

func call(h *APIHandler, method, path, body string, headers map[string]string) events.APIGatewayProxyResponse {
	resp, _ := h.HandleRequest(context.Background(), events.APIGatewayProxyRequest{HTTPMethod: method, Path: path, Body: body, Headers: headers})
	return resp
}

func setCookies(resp events.APIGatewayProxyResponse) []string {
	return resp.MultiValueHeaders["Set-Cookie"]
}

func TestRefreshCookie_Flags(t *testing.T) {
	c := refreshCookie("tok", 604800)
	for _, want := range []string{"indifferent_rt=tok", "Path=/auth", "Max-Age=604800", "HttpOnly", "Secure", "SameSite=Strict"} {
		if !strings.Contains(c, want) {
			t.Errorf("cookie %q is missing %q", c, want)
		}
	}
	if strings.Contains(c, "Domain=") {
		t.Errorf("the cookie must be host-only, got %q", c)
	}
	gone := refreshCookie("", 0)
	if !strings.Contains(gone, "Max-Age=0") && !strings.Contains(gone, "Max-Age=-1") && !strings.Contains(gone, "expires=") && !strings.Contains(gone, "Expires=") {
		t.Errorf("clearing cookie does not expire it: %q", gone)
	}
}

func TestRefreshCookie_SignInSetsIt(t *testing.T) {
	h := newTestHandler()
	resp := call(h, "POST", "/auth/google/callback", `{"code":"abc"}`, nil)
	if resp.StatusCode != 200 {
		t.Fatalf("status %d %s", resp.StatusCode, resp.Body)
	}
	cs := setCookies(resp)
	if len(cs) != 1 || !strings.Contains(cs[0], "indifferent_rt=mock-refresh-token") || !strings.Contains(cs[0], "HttpOnly") {
		t.Errorf("sign-in must set the cookie, got %v", cs)
	}
	// Still in the body while older web app copies exist.
	if !strings.Contains(resp.Body, "mock-refresh-token") {
		t.Errorf("the body should still carry the refresh token during the move: %s", resp.Body)
	}
}

func TestRefreshCookie_ReadsCookieAndBody(t *testing.T) {
	r := events.APIGatewayProxyRequest{Headers: map[string]string{"cookie": "a=1; indifferent_rt=from-cookie; b=2"}}
	if tok, fromCookie := presentedRefreshToken(r, "from-body"); tok != "from-cookie" || !fromCookie {
		t.Errorf("cookie must win: %q %v", tok, fromCookie)
	}
	r = events.APIGatewayProxyRequest{MultiValueHeaders: map[string][]string{"Cookie": {"indifferent_rt=mv"}}}
	if tok, _ := presentedRefreshToken(r, ""); tok != "mv" {
		t.Errorf("multi-value header not read: %q", tok)
	}
	if tok, fromCookie := presentedRefreshToken(events.APIGatewayProxyRequest{}, " from-body "); tok != "from-body" || fromCookie {
		t.Errorf("body fallback: %q %v", tok, fromCookie)
	}
	if tok, _ := presentedRefreshToken(events.APIGatewayProxyRequest{Headers: map[string]string{"Cookie": "other=1"}}, ""); tok != "" {
		t.Errorf("another cookie must not count: %q", tok)
	}
}

func TestRefreshCookie_RefreshWithNothingIs400(t *testing.T) {
	h := newTestHandler()
	if resp := call(h, "POST", "/auth/refresh", "", nil); resp.StatusCode != 400 {
		t.Errorf("no token and no body: %d", resp.StatusCode)
	}
	if resp := call(h, "POST", "/auth/refresh", "{}", nil); resp.StatusCode != 400 {
		t.Errorf("empty token: %d", resp.StatusCode)
	}
}

func TestRefreshCookie_ForeignOriginIsRefusedForCookieRequests(t *testing.T) {
	h := newTestHandler()
	for _, path := range []string{"/auth/refresh", "/auth/logout"} {
		resp := call(h, "POST", path, "", map[string]string{"Cookie": "indifferent_rt=x", "Origin": "https://evil.example"})
		if resp.StatusCode != 403 || !strings.Contains(resp.Body, "FORBIDDEN_ORIGIN") {
			t.Errorf("%s with a foreign origin: %d %s", path, resp.StatusCode, resp.Body)
		}
	}
	// The same origin is not refused (it fails later on the unknown token, not with 403).
	resp := call(h, "POST", "/auth/refresh", "", map[string]string{"Cookie": "indifferent_rt=x", "Origin": allowedOrigin})
	if resp.StatusCode == 403 {
		t.Errorf("our own origin was refused: %s", resp.Body)
	}
}

func TestRefreshCookie_FailedRefreshClearsTheCookie(t *testing.T) {
	h := newTestHandler()
	resp := call(h, "POST", "/auth/refresh", "", map[string]string{"Cookie": "indifferent_rt=unknown", "Origin": allowedOrigin})
	if resp.StatusCode != 401 {
		t.Fatalf("status %d %s", resp.StatusCode, resp.Body)
	}
	cs := setCookies(resp)
	if len(cs) != 1 || !strings.HasPrefix(cs[0], "indifferent_rt=;") {
		t.Fatalf("expected the cookie to be cleared, got %v", cs)
	}
	if !strings.Contains(cs[0], "Max-Age=0") && !strings.Contains(cs[0], "Max-Age=-1") {
		t.Errorf("the cleared cookie must expire: %q", cs[0])
	}
	// A body token that fails does not touch the cookie: it was never the cookie that failed.
	resp = call(h, "POST", "/auth/refresh", `{"refreshToken":"unknown"}`, nil)
	if resp.StatusCode != 401 || len(setCookies(resp)) != 0 {
		t.Errorf("body-token failure must not set a cookie: %d %v", resp.StatusCode, setCookies(resp))
	}
}

func TestRefreshCookie_LogoutClearsIt(t *testing.T) {
	h := newTestHandler()
	resp := call(h, "POST", "/auth/logout", "", map[string]string{"Cookie": "indifferent_rt=x", "Origin": allowedOrigin})
	if resp.StatusCode != 204 {
		t.Fatalf("status %d %s", resp.StatusCode, resp.Body)
	}
	if cs := setCookies(resp); len(cs) != 1 || !strings.Contains(cs[0], "indifferent_rt=") {
		t.Errorf("logout must clear the cookie: %v", cs)
	}
	// Logging out with nothing to log out is still refused as before.
	if resp := call(h, "POST", "/auth/logout", "", nil); resp.StatusCode != 400 {
		t.Errorf("no token: %d", resp.StatusCode)
	}
}

func TestRefreshCookie_NoOriginHeaderIsNotRefused(t *testing.T) {
	if !originAllowed(events.APIGatewayProxyRequest{}) {
		t.Error("a request without Origin must pass the origin check")
	}
	if originAllowed(events.APIGatewayProxyRequest{Headers: map[string]string{"origin": "https://indifferent.fun.evil.example"}}) {
		t.Error("a look-alike origin must be refused")
	}
}
