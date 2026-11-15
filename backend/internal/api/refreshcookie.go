package api

import (
	"net/http"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/rahul/indifferent/backend/internal/auth"
	"github.com/rahul/indifferent/backend/internal/models"
)

// The refresh token travels in an HttpOnly cookie (SEC-04 follow-up), so a script
// on the page (an XSS, a bad third-party script) cannot read it and keep a
// 30-day session. The cookie is host-only on the API's own domain, sent only to
// /auth, and SameSite=Strict: api.indifferent.fun and indifferent.fun are the same
// site, so the web app's own calls carry it and a request started from another
// site does not.
//
// During the move the refresh token is also still returned in the response body
// and still accepted in the request body, so a web app that has not been updated
// keeps working. Both are removed once every older browser copy has expired.
const (
	refreshCookieName = "indifferent_rt"
	refreshCookiePath = "/auth"
	// allowedOrigin is the only web origin that may use the cookie (see corsHeaders).
	allowedOrigin = "https://indifferent.fun"
)

func refreshCookie(value string, maxAgeSeconds int) string {
	c := &http.Cookie{
		Name:     refreshCookieName,
		Value:    value,
		Path:     refreshCookiePath,
		MaxAge:   maxAgeSeconds,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	}
	if maxAgeSeconds <= 0 {
		c.MaxAge = -1 // tells the browser to delete it now
	}
	return c.String()
}

// withSetCookie adds a Set-Cookie header. API Gateway needs it in
// MultiValueHeaders; the single-value map cannot carry more than one.
func withSetCookie(resp events.APIGatewayProxyResponse, cookie string) events.APIGatewayProxyResponse {
	if resp.MultiValueHeaders == nil {
		resp.MultiValueHeaders = map[string][]string{}
	}
	resp.MultiValueHeaders["Set-Cookie"] = append(resp.MultiValueHeaders["Set-Cookie"], cookie)
	return resp
}

// tokensResponse is the response for a sign-in or refresh: the tokens as JSON
// (as before) and the refresh token in the cookie.
func tokensResponse(status int, tokens *models.AuthTokens) events.APIGatewayProxyResponse {
	resp := jsonResponse(status, tokens)
	return withSetCookie(resp, refreshCookie(tokens.RefreshToken, int(auth.RefreshTokenTTL.Seconds())))
}

// clearRefreshCookie expires the cookie in the browser.
func clearRefreshCookie(resp events.APIGatewayProxyResponse) events.APIGatewayProxyResponse {
	return withSetCookie(resp, refreshCookie("", 0))
}

// headerValues finds a request header whatever its case, and joins repeats.
func headerValues(req events.APIGatewayProxyRequest, name string) string {
	var parts []string
	for k, v := range req.MultiValueHeaders {
		if strings.EqualFold(k, name) {
			parts = append(parts, v...)
		}
	}
	if len(parts) == 0 {
		for k, v := range req.Headers {
			if strings.EqualFold(k, name) {
				parts = append(parts, v)
			}
		}
	}
	return strings.Join(parts, "; ")
}

// refreshTokenFromCookie returns the refresh token the browser sent in its cookie.
func refreshTokenFromCookie(req events.APIGatewayProxyRequest) string {
	raw := headerValues(req, "Cookie")
	if raw == "" {
		return ""
	}
	r := http.Request{Header: http.Header{"Cookie": []string{raw}}}
	c, err := r.Cookie(refreshCookieName)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(c.Value)
}

// presentedRefreshToken picks the refresh token for a request: the cookie when
// there is one, otherwise the one in the body (older web app copies). fromCookie
// tells the caller which, because the cookie is sent by the browser without the
// page's say-so and so needs the origin check below.
func presentedRefreshToken(req events.APIGatewayProxyRequest, bodyToken string) (token string, fromCookie bool) {
	if t := refreshTokenFromCookie(req); t != "" {
		return t, true
	}
	return strings.TrimSpace(bodyToken), false
}

// originAllowed is the second guard for a request that relies on the cookie, on
// top of SameSite=Strict: when the browser names an Origin it must be ours.
// Requests with no Origin (not from a browser page) are not refused here; they
// cannot carry the browser's cookie from another site anyway.
func originAllowed(req events.APIGatewayProxyRequest) bool {
	origin := strings.TrimSpace(headerValues(req, "Origin"))
	return origin == "" || origin == allowedOrigin
}
