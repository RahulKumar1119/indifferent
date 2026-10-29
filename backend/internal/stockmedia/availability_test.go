package stockmedia

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

// toServer returns a transport that sends every connection to srv, whatever
// host the URL names, and trusts its certificate. This lets tests use the
// real provider hostnames, so the production URL policy is what is exercised.
func toServer(srv *httptest.Server) http.RoundTripper {
	return &http.Transport{
		DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, network, srv.Listener.Addr().String())
		},
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
}

func withAssetTransport(rt http.RoundTripper) Option {
	return func(s *Service) { s.assetTransport = rt }
}

// availabilityService uses the production URL policy.
func availabilityService(t *testing.T, srv *httptest.Server) *Service {
	t.Helper()
	s, err := NewService([]Provider{okProvider(ProviderPexels)},
		WithLogger(func(string, ...any) {}), withAssetTransport(toServer(srv)))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func tlsServer(t *testing.T, h http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewTLSServer(h)
	t.Cleanup(srv.Close)
	return srv
}

func TestCheckAvailable_OK(t *testing.T) {
	var methods []string
	srv := tlsServer(t, func(w http.ResponseWriter, r *http.Request) {
		methods = append(methods, r.Method)
		w.Header().Set("Content-Type", "video/mp4")
		w.Header().Set("Content-Length", "5000000")
	})
	if err := availabilityService(t, srv).CheckAvailable(context.Background(), realAsset()); err != nil {
		t.Fatal(err)
	}
	if len(methods) != 1 || methods[0] != http.MethodHead {
		t.Errorf("methods = %v, want a single HEAD (no download)", methods)
	}
}

func TestCheckAvailable_AssetFailingValidationMakesNoRequest(t *testing.T) {
	calls := 0
	srv := tlsServer(t, func(w http.ResponseWriter, r *http.Request) { calls++ })
	a := realAsset()
	a.MediaURL = "https://169.254.169.254/latest/meta-data/"
	err := availabilityService(t, srv).CheckAvailable(context.Background(), a)
	if !errors.Is(err, ErrInvalidAsset) || calls != 0 {
		t.Errorf("err = %v, server calls = %d; want ErrInvalidAsset and no request", err, calls)
	}
}

func TestCheckAvailable_FallsBackToRangedGetWhenHeadRefused(t *testing.T) {
	for _, status := range []int{http.StatusMethodNotAllowed, http.StatusNotImplemented, http.StatusForbidden} {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			var seen []string
			var rangeHeader string
			srv := tlsServer(t, func(w http.ResponseWriter, r *http.Request) {
				seen = append(seen, r.Method)
				if r.Method == http.MethodHead {
					w.WriteHeader(status)
					return
				}
				rangeHeader = r.Header.Get("Range")
				w.Header().Set("Content-Type", "video/mp4")
				w.Header().Set("Content-Range", "bytes 0-0/5000000")
				w.WriteHeader(http.StatusPartialContent)
				w.Write([]byte{0})
			})
			if err := availabilityService(t, srv).CheckAvailable(context.Background(), realAsset()); err != nil {
				t.Fatal(err)
			}
			if strings.Join(seen, ",") != "HEAD,GET" || rangeHeader != "bytes=0-0" {
				t.Errorf("requests = %v range = %q, want HEAD then a one-byte ranged GET", seen, rangeHeader)
			}
		})
	}
}

func TestCheckAvailable_Failures(t *testing.T) {
	tests := []struct {
		name        string
		handler     http.HandlerFunc
		unavailable bool // true when "try again" is reasonable
	}{
		{"gone", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNotFound) }, false},
		{"410", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusGone) }, false},
		{"401", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusUnauthorized) }, false},
		{"server error", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusBadGateway) }, true},
		{"throttled", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusTooManyRequests) }, true},
		{"html instead of video", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
		}, false},
		{"image where a video was promised", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "image/jpeg")
		}, false},
		{"too large", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "video/mp4")
			w.Header().Set("Content-Length", strconv.Itoa(maxVideoBytes+1))
		}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := availabilityService(t, tlsServer(t, tc.handler)).CheckAvailable(context.Background(), realAsset())
			if !errors.Is(err, ErrAssetUnavailable) {
				t.Fatalf("err = %v, want ErrAssetUnavailable", err)
			}
			if errors.Is(err, ErrUnavailable) != tc.unavailable {
				t.Errorf("errors.Is(err, ErrUnavailable) = %v, want %v (%v)", !tc.unavailable, tc.unavailable, err)
			}
			if strings.Contains(err.Error(), "vimeo.com") || strings.Contains(err.Error(), "?s=") {
				t.Errorf("error contains the URL: %v", err)
			}
		})
	}
}

func TestCheckAvailable_OctetStreamIsAccepted(t *testing.T) {
	srv := tlsServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
	})
	if err := availabilityService(t, srv).CheckAvailable(context.Background(), realAsset()); err != nil {
		t.Errorf("CDNs often serve media as octet-stream: %v", err)
	}
}

func TestCheckAvailable_RedirectToAllowedHostIsFollowed(t *testing.T) {
	srv := tlsServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Host == "player.vimeo.com" || r.URL.Path == "/start" {
			http.Redirect(w, r, "https://vod-progressive.akamaized.vimeocdn.com/final.mp4", http.StatusFound)
			return
		}
		w.Header().Set("Content-Type", "video/mp4")
	})
	a := realAsset()
	a.MediaURL = "https://player.vimeo.com/start"
	if err := availabilityService(t, srv).CheckAvailable(context.Background(), a); err != nil {
		t.Errorf("a redirect inside the provider's own domains is normal: %v", err)
	}
}

func TestCheckAvailable_RedirectGuard(t *testing.T) {
	for name, target := range map[string]string{
		"metadata service":  "https://169.254.169.254/latest/meta-data/",
		"other domain":      "https://evil.example.com/x.mp4",
		"plain http":        "http://player.vimeo.com/x.mp4",
		"localhost":         "https://localhost/x.mp4",
		"credentialed host": "https://user:pw@player.vimeo.com/x.mp4",
	} {
		t.Run(name, func(t *testing.T) {
			hits := 0
			srv := tlsServer(t, func(w http.ResponseWriter, r *http.Request) {
				hits++
				http.Redirect(w, r, target, http.StatusFound)
			})
			err := availabilityService(t, srv).CheckAvailable(context.Background(), realAsset())
			if err == nil {
				t.Fatal("a redirect to a disallowed target must fail")
			}
			if errors.Is(err, ErrUnavailable) {
				t.Errorf("a refused redirect is not temporary; callers must not retry it: %v", err)
			}
			if hits != 1 {
				t.Errorf("server saw %d requests, want 1 (the redirect must not be followed)", hits)
			}
			if strings.Contains(err.Error(), "169.254") || strings.Contains(err.Error(), "evil.example") {
				t.Errorf("error repeats the redirect target: %v", err)
			}
		})
	}
}

func TestCheckAvailable_RedirectLoopIsBounded(t *testing.T) {
	hits := 0
	srv := tlsServer(t, func(w http.ResponseWriter, r *http.Request) {
		hits++
		http.Redirect(w, r, "https://player.vimeo.com/loop", http.StatusFound)
	})
	err := availabilityService(t, srv).CheckAvailable(context.Background(), realAsset())
	if err == nil || hits > maxRedirects+1 {
		t.Errorf("err = %v, server hits = %d; want failure after at most %d hops", err, hits, maxRedirects)
	}
}

func TestCheckAvailable_ContextCancelled(t *testing.T) {
	srv := tlsServer(t, func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() })
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	err := availabilityService(t, srv).CheckAvailable(ctx, realAsset())
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v, want context.DeadlineExceeded", err)
	}
}

func TestCheckAvailable_UnreachableHostIsTemporary(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	s := availabilityService(t, srv)
	srv.Close()
	err := s.CheckAvailable(context.Background(), realAsset())
	if !errors.Is(err, ErrAssetUnavailable) || !errors.Is(err, ErrUnavailable) {
		t.Errorf("err = %v, want unavailable and retryable", err)
	}
	if strings.Contains(err.Error(), "127.0.0.1") || strings.Contains(err.Error(), "vimeo.com") {
		t.Errorf("error contains an address: %v", err)
	}
}
