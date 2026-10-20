package thumbnail

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"math"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	xdraw "golang.org/x/image/draw"
)

// noiseImage is a maximally sharp frame.
func noiseImage(w, h int, seed int64) *image.RGBA {
	r := rand.New(rand.NewSource(seed))
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < len(img.Pix); i += 4 {
		v := uint8(r.Intn(256))
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = v, v, v, 255
	}
	return img
}

// blurred shrinks and re-enlarges an image, destroying its detail.
func blurred(src *image.RGBA) *image.RGBA {
	b := src.Bounds()
	small := image.NewRGBA(image.Rect(0, 0, 16, 9))
	xdraw.ApproxBiLinear.Scale(small, small.Bounds(), src, b, xdraw.Src, nil)
	out := image.NewRGBA(b)
	xdraw.CatmullRom.Scale(out, b, small, small.Bounds(), xdraw.Src, nil)
	return out
}

func solid(w, h int, c color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = c.R, c.G, c.B, 255
	}
	return img
}

// scene draws a recognisable synthetic frame: a gradient sky, a horizon and a
// few bold shapes whose positions depend on seed, so different seeds give
// clearly different pictures.
func scene(w, h int, seed int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	r := rand.New(rand.NewSource(int64(seed)))
	base := color.RGBA{R: uint8(40 + r.Intn(160)), G: uint8(40 + r.Intn(160)), B: uint8(40 + r.Intn(160)), A: 255}
	for y := 0; y < h; y++ {
		t := float64(y) / float64(h)
		for x := 0; x < w; x++ {
			i := img.PixOffset(x, y)
			img.Pix[i] = uint8(float64(base.R) * (0.6 + 0.4*t))
			img.Pix[i+1] = uint8(float64(base.G) * (0.6 + 0.4*(1-t)))
			img.Pix[i+2] = uint8(float64(base.B) * (0.8 + 0.2*t))
			img.Pix[i+3] = 255
		}
	}
	for n := 0; n < 6; n++ {
		cx, cy := r.Intn(w), r.Intn(h)
		rad := w/14 + r.Intn(w/10)
		c := color.RGBA{R: uint8(r.Intn(256)), G: uint8(r.Intn(256)), B: uint8(r.Intn(256)), A: 255}
		for y := max(0, cy-rad); y < min(h, cy+rad); y++ {
			for x := max(0, cx-rad); x < min(w, cx+rad); x++ {
				if (x-cx)*(x-cx)+(y-cy)*(y-cy) <= rad*rad {
					img.SetRGBA(x, y, c)
				}
			}
		}
	}
	return img
}

func writeJPEG(t *testing.T, path string, img image.Image) {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

func meanAbsDiff(a, b *image.RGBA) float64 {
	if a.Bounds() != b.Bounds() {
		return math.Inf(1)
	}
	var sum float64
	for i := range a.Pix {
		d := float64(a.Pix[i]) - float64(b.Pix[i])
		sum += math.Abs(d)
	}
	return sum / float64(len(a.Pix))
}

func needTool(t *testing.T, names ...string) {
	t.Helper()
	for _, n := range names {
		if _, err := exec.LookPath(n); err != nil {
			t.Skipf("%s not on PATH", n)
		}
	}
}

// fakeRunner records calls and returns canned output.
type fakeRunner struct {
	mu    sync.Mutex
	calls [][]string
	run   func(name string, args []string) ([]byte, error)
}

func (f *fakeRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	f.mu.Lock()
	f.calls = append(f.calls, append([]string{name}, args...))
	f.mu.Unlock()
	return f.run(name, args)
}

// fakeDetector returns canned detections keyed by a substring of the image
// bytes' hash is too fragile, so it keys on call order via a function.
type fakeDetector struct {
	fn func(jpeg []byte) (Detection, error)
}

func (f fakeDetector) Detect(_ context.Context, jpeg []byte) (Detection, error) {
	return f.fn(jpeg)
}

type fakeAnalyzer struct {
	got AnalysisInput
	out Analysis
	err error
}

func (f *fakeAnalyzer) Analyze(_ context.Context, in AnalysisInput) (Analysis, error) {
	f.got = in
	return f.out, f.err
}

func hasSub(s, sub string) bool { return strings.Contains(s, sub) }

func tmpFile(t *testing.T, dir, name string) string { return filepath.Join(dir, name) }
