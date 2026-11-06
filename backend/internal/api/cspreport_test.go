package api

import (
	"bytes"
	"context"
	"log"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-lambda-go/events"
)

func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	return &buf
}

func resetCSPLimiter(t *testing.T, at time.Time) {
	t.Helper()
	cspLimiter.Lock()
	cspLimiter.windowStart = time.Time{}
	cspLimiter.count = 0
	cspLimiter.Unlock()
	old := cspNow
	cspNow = func() time.Time { return at }
	t.Cleanup(func() { cspNow = old })
}

func postCSP(t *testing.T, body string) events.APIGatewayProxyResponse {
	t.Helper()
	resp, err := newTestHandler().HandleRequest(context.Background(), events.APIGatewayProxyRequest{HTTPMethod: "POST", Path: "/csp-report", Body: body})
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestCSPReport_LogsAShortenedViolationWithoutAuth(t *testing.T) {
	resetCSPLimiter(t, time.Now())
	buf := captureLog(t)
	resp := postCSP(t, `{"csp-report":{"document-uri":"https://indifferent.fun/auth/callback?code=SECRET&state=abc","blocked-uri":"https://evil.example/x.js?token=SECRET#frag","effective-directive":"script-src-elem","disposition":"report","source-file":"https://indifferent.fun/main.js","line-number":12}}`)
	if resp.StatusCode != 204 {
		t.Fatalf("status %d, want 204", resp.StatusCode)
	}
	out := buf.String()
	for _, want := range []string{`directive="script-src-elem"`, `blocked="https://evil.example/x.js"`, `page="https://indifferent.fun/auth/callback"`, `line=12`} {
		if !strings.Contains(out, want) {
			t.Errorf("log missing %s:\n%s", want, out)
		}
	}
	if strings.Contains(out, "SECRET") || strings.Contains(out, "frag") {
		t.Errorf("query or fragment leaked into the log:\n%s", out)
	}
}

func TestCSPReport_CannotForgeALogLine(t *testing.T) {
	resetCSPLimiter(t, time.Now())
	buf := captureLog(t)
	postCSP(t, `{"csp-report":{"blocked-uri":"inline\nFAKE csp_violation admin=true","effective-directive":"x\ny"}}`)
	if n := strings.Count(strings.TrimSpace(buf.String()), "\n"); n != 0 {
		t.Errorf("report produced %d extra lines:\n%s", n, buf.String())
	}
}

func TestCSPReport_IgnoresJunkAndOversizeAndStaysQuiet(t *testing.T) {
	resetCSPLimiter(t, time.Now())
	buf := captureLog(t)
	for name, body := range map[string]string{
		"empty":     "",
		"not json":  "<html>",
		"no fields": `{"csp-report":{}}`,
		"wrong":     `{"other":1}`,
		"huge":      `{"csp-report":{"blocked-uri":"` + strings.Repeat("a", cspReportMaxBody) + `","effective-directive":"x"}}`,
	} {
		if resp := postCSP(t, body); resp.StatusCode != 204 {
			t.Errorf("%s: status %d, want 204", name, resp.StatusCode)
		}
	}
	if buf.Len() != 0 {
		t.Errorf("junk was logged:\n%s", buf.String())
	}
}

func TestCSPReport_FieldsAreCapped(t *testing.T) {
	resetCSPLimiter(t, time.Now())
	buf := captureLog(t)
	postCSP(t, `{"csp-report":{"effective-directive":"img-src","blocked-uri":"https://`+strings.Repeat("a", 2000)+`.example/"}}`)
	if buf.Len() > 700 {
		t.Errorf("log line is %d bytes; field caps are not applied", buf.Len())
	}
}

func TestCSPReport_LimitsLinesPerMinute(t *testing.T) {
	start := time.Now()
	resetCSPLimiter(t, start)
	buf := captureLog(t)
	for i := 0; i < cspReportsPerMinute+25; i++ {
		postCSP(t, `{"csp-report":{"effective-directive":"img-src","blocked-uri":"https://a.example/"}}`)
	}
	if n := strings.Count(buf.String(), "csp_violation"); n != cspReportsPerMinute {
		t.Errorf("logged %d lines, want %d", n, cspReportsPerMinute)
	}
	cspNow = func() time.Time { return start.Add(61 * time.Second) }
	postCSP(t, `{"csp-report":{"effective-directive":"img-src","blocked-uri":"https://a.example/"}}`)
	if n := strings.Count(buf.String(), "csp_violation"); n != cspReportsPerMinute+1 {
		t.Errorf("a new minute should log again, have %d lines", n)
	}
}
