package api

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-lambda-go/events"
)

// A browser that blocks something under the site's Content-Security-Policy
// (or would block it, in report-only mode) posts a small JSON report here, so
// the policy can be checked against real traffic before it is enforced.
//
// The endpoint is public by necessity: browsers send reports without
// credentials. So it stores nothing, trusts nothing and is bounded: the body is
// capped, only a few fields are read, every logged value is shortened and
// printed with %q (a report cannot forge a log line), and each Lambda instance
// logs at most cspReportsPerMinute lines a minute.
const (
	cspReportMaxBody    = 8 << 10
	cspReportsPerMinute = 30
	cspFieldMax         = 160
)

var cspLimiter = struct {
	sync.Mutex
	windowStart time.Time
	count       int
}{}

// cspNow is replaced in tests.
var cspNow = time.Now

func cspAllow() bool {
	now := cspNow()
	cspLimiter.Lock()
	defer cspLimiter.Unlock()
	if now.Sub(cspLimiter.windowStart) >= time.Minute {
		cspLimiter.windowStart = now
		cspLimiter.count = 0
	}
	cspLimiter.count++
	return cspLimiter.count <= cspReportsPerMinute
}

// cspShort keeps an origin and a path and drops the query and fragment, which
// can carry tokens (a Google sign-in redirect has a one-time code in it).
func cspShort(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" {
		// "inline", "eval", "data" and the like are not URLs.
		return cspCut(raw)
	}
	if u.Host == "" {
		return cspCut(u.Scheme)
	}
	return cspCut(u.Scheme + "://" + u.Host + u.Path)
}

func cspCut(s string) string {
	if len(s) > cspFieldMax {
		return s[:cspFieldMax]
	}
	return s
}

// handleCSPReport accepts the legacy report-uri format
// ({"csp-report": {...}}). It always answers 204: the browser does not read
// the reply, and an error would only invite retries.
func (h *APIHandler) handleCSPReport(req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	done := events.APIGatewayProxyResponse{StatusCode: http.StatusNoContent, Headers: corsHeaders()}
	if len(req.Body) == 0 || len(req.Body) > cspReportMaxBody {
		return done, nil
	}
	var in struct {
		Report struct {
			Document  string `json:"document-uri"`
			Blocked   string `json:"blocked-uri"`
			Effective string `json:"effective-directive"`
			Violated  string `json:"violated-directive"`
			Source    string `json:"source-file"`
			Line      int    `json:"line-number"`
			Disp      string `json:"disposition"`
		} `json:"csp-report"`
	}
	if err := json.Unmarshal([]byte(req.Body), &in); err != nil {
		return done, nil
	}
	r := in.Report
	directive := r.Effective
	if f := strings.Fields(r.Violated); directive == "" && len(f) > 0 {
		directive = f[0]
	}
	if directive == "" && r.Blocked == "" {
		return done, nil
	}
	if !cspAllow() {
		return done, nil
	}
	log.Printf("csp_violation directive=%q blocked=%q page=%q source=%q line=%d disposition=%q",
		cspCut(directive), cspShort(r.Blocked), cspShort(r.Document), cspShort(r.Source), r.Line, cspCut(r.Disp))
	return done, nil
}
