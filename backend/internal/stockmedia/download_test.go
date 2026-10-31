package stockmedia

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// body is a stand-in file whose bytes can be recognised.
func body(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(i % 251)
	}
	return b
}

func serveFile(data []byte, contentType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", contentType)
		if r.Method == http.MethodHead {
			w.Header().Set("Content-Length", "5000")
			return
		}
		w.Write(data)
	}
}

func TestDownload_WritesTheWholeFile(t *testing.T) {
	data := body(4096)
	srv := tlsServer(t, serveFile(data, "video/mp4"))
	var buf bytes.Buffer
	n, err := availabilityService(t, srv).Download(context.Background(), realAsset(), &buf)
	if err != nil {
		t.Fatal(err)
	}
	if n != int64(len(data)) || !bytes.Equal(buf.Bytes(), data) {
		t.Errorf("wrote %d bytes, content equal: %v; want %d bytes identical", n, bytes.Equal(buf.Bytes(), data), len(data))
	}
}

func TestDownload_SendsNoCredentials(t *testing.T) {
	var got http.Header
	srv := tlsServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			got = r.Header.Clone()
		}
		serveFile(body(100), "video/mp4")(w, r)
	})
	if _, err := availabilityService(t, srv).Download(context.Background(), realAsset(), &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	for _, h := range []string{"Authorization", "Cookie", "X-Api-Key", "Key"} {
		if v := got.Get(h); v != "" {
			t.Errorf("sent %s: %q", h, v)
		}
	}
	if !strings.HasPrefix(got.Get("User-Agent"), "indifferent-stockmedia") {
		t.Errorf("User-Agent = %q", got.Get("User-Agent"))
	}
}

func TestDownload_AnAssetThatFailsValidationMakesNoRequestAtAll(t *testing.T) {
	var calls int32
	srv := tlsServer(t, func(w http.ResponseWriter, r *http.Request) { atomic.AddInt32(&calls, 1) })
	for name, mutate := range map[string]func(*Asset){
		"internal address": func(a *Asset) { a.MediaURL = "https://169.254.169.254/latest/meta-data/" },
		"other host":       func(a *Asset) { a.MediaURL = "https://evil.example/x.mp4" },
		"plain http":       func(a *Asset) { a.MediaURL = "http://player.vimeo.com/x.mp4" },
		"too small":        func(a *Asset) { a.Width, a.Height = 320, 240 },
	} {
		a := realAsset()
		mutate(&a)
		_, err := availabilityService(t, srv).Download(context.Background(), a, &bytes.Buffer{})
		if !errors.Is(err, ErrInvalidAsset) {
			t.Errorf("%s: err = %v, want ErrInvalidAsset", name, err)
		}
	}
	if calls != 0 {
		t.Errorf("%d requests reached the server", calls)
	}
}

func TestDownload_ARedirectToAnInternalAddressIsRefused(t *testing.T) {
	var internalHits int32
	srv := tlsServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.Header().Set("Content-Type", "video/mp4")
			return
		}
		if r.Host == "169.254.169.254" {
			atomic.AddInt32(&internalHits, 1)
			w.Write([]byte("secret"))
			return
		}
		http.Redirect(w, r, "https://169.254.169.254/latest/meta-data/", http.StatusFound)
	})
	var buf bytes.Buffer
	_, err := availabilityService(t, srv).Download(context.Background(), realAsset(), &buf)
	if !errors.Is(err, ErrAssetUnavailable) {
		t.Fatalf("err = %v, want ErrAssetUnavailable", err)
	}
	if internalHits != 0 || buf.Len() != 0 {
		t.Errorf("followed the redirect: %d hits, %d bytes written", internalHits, buf.Len())
	}
	if strings.Contains(err.Error(), "169.254") {
		t.Errorf("the error repeats the redirect target: %v", err)
	}
}

func TestDownload_ARedirectBetweenAllowedHostsIsFollowed(t *testing.T) {
	data := body(2048)
	srv := tlsServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.Header().Set("Content-Type", "video/mp4")
			return
		}
		if r.URL.Path == "/start" || strings.Contains(r.URL.Path, "342571552") {
			http.Redirect(w, r, "https://player.vimeo.com/cdn/file.mp4", http.StatusFound)
			return
		}
		w.Header().Set("Content-Type", "video/mp4")
		w.Write(data)
	})
	var buf bytes.Buffer
	if _, err := availabilityService(t, srv).Download(context.Background(), realAsset(), &buf); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(buf.Bytes(), data) {
		t.Errorf("did not get the file after the redirect: %d bytes", buf.Len())
	}
}

func TestDownload_AServerThatLiesAboutItsSizeCannotFillTheDisk(t *testing.T) {
	// The headers say 5000 bytes. The body is far over the photo limit.
	huge := maxPhotoBytes + 4096
	srv := tlsServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		if r.Method == http.MethodHead {
			w.Header().Set("Content-Length", "5000")
			return
		}
		chunk := make([]byte, 1<<20)
		for sent := 0; sent < huge; sent += len(chunk) {
			if _, err := w.Write(chunk); err != nil {
				return
			}
		}
	})
	a := realAsset()
	a.Type, a.Format, a.Duration = MediaPhoto, "image/jpeg", 0
	a.MediaURL = "https://images.pexels.com/photos/1/big.jpeg"
	var buf bytes.Buffer
	n, err := availabilityService(t, srv).Download(context.Background(), a, &buf)
	if !errors.Is(err, ErrAssetUnavailable) || errors.Is(err, ErrUnavailable) {
		t.Errorf("err = %v, want ErrAssetUnavailable and not a retryable error", err)
	}
	if n > maxPhotoBytes+1 || int64(buf.Len()) > int64(maxPhotoBytes)+1 {
		t.Errorf("wrote %d bytes, more than the %d byte limit", n, maxPhotoBytes)
	}
}

func TestDownload_FailureCases(t *testing.T) {
	cases := []struct {
		name  string
		serve http.HandlerFunc
		retry bool // a retry could help
	}{
		{"the file is gone", func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodHead {
				w.Header().Set("Content-Type", "video/mp4")
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}, false},
		{"the CDN is down", func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodHead {
				w.Header().Set("Content-Type", "video/mp4")
				return
			}
			w.WriteHeader(http.StatusServiceUnavailable)
		}, true},
		{"rate limited", func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodHead {
				w.Header().Set("Content-Type", "video/mp4")
				return
			}
			w.WriteHeader(http.StatusTooManyRequests)
		}, true},
		{"an empty file", serveFile(nil, "video/mp4"), false},
		{"a web page instead of a video", serveFile([]byte("<html>"), "text/html"), false},
		{"the connection drops part way", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "video/mp4")
			if r.Method == http.MethodHead {
				return
			}
			w.Header().Set("Content-Length", "100000")
			w.Write(body(1000))
			panic(http.ErrAbortHandler)
		}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv := tlsServer(t, c.serve)
			_, err := availabilityService(t, srv).Download(context.Background(), realAsset(), &bytes.Buffer{})
			if !errors.Is(err, ErrAssetUnavailable) {
				t.Fatalf("err = %v, want ErrAssetUnavailable", err)
			}
			if got := errors.Is(err, ErrUnavailable); got != c.retry {
				t.Errorf("retryable = %v, want %v (err: %v)", got, c.retry, err)
			}
			if strings.Contains(err.Error(), "vimeo.com/external") || strings.Contains(err.Error(), "s=abc") {
				t.Errorf("the error contains the URL: %v", err)
			}
		})
	}
}

func TestDownload_StopsWhenTheCallerGivesUp(t *testing.T) {
	release := make(chan struct{})
	srv := tlsServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "video/mp4")
		if r.Method == http.MethodHead {
			return
		}
		w.(http.Flusher).Flush()
		select {
		case <-release:
		case <-r.Context().Done():
		}
	})
	defer close(release)
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := availabilityService(t, srv).Download(ctx, realAsset(), &bytes.Buffer{})
	if err == nil {
		t.Fatal("a stalled download returned no error")
	}
	if time.Since(start) > 5*time.Second {
		t.Errorf("took %v to give up", time.Since(start))
	}
}

func TestDownload_AFailedWriteIsNotBlamedOnTheProvider(t *testing.T) {
	srv := tlsServer(t, serveFile(body(4096), "video/mp4"))
	_, err := availabilityService(t, srv).Download(context.Background(), realAsset(), failingWriter{})
	if err == nil {
		t.Fatal("no error when the disk is full")
	}
	// Nothing is wrong with the asset, and trying another would not help.
	for _, notThis := range []error{ErrAssetUnavailable, ErrUnavailable, ErrInvalidAsset} {
		if errors.Is(err, notThis) {
			t.Errorf("a full disk was reported as %v: %v", notThis, err)
		}
	}
	if !strings.Contains(err.Error(), "no space left") {
		t.Errorf("the real cause is lost: %v", err)
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("no space left on device") }
