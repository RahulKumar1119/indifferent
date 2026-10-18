package render

import (
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/rahul/indifferent/backend/internal/models"
)

// legacyFitFillFilter is the blur-fill filter before issue #137: it blurred a
// full 1080x1920 copy of the frame (sigma 40). Kept here only so the current
// filter can be checked against it.
const legacyFitFillFilter = "split=2[blur][main];[blur]scale=1080:1920:force_original_aspect_ratio=increase,crop=1080:1920,gblur=sigma=40[bg];[main]scale=1080:1920:force_original_aspect_ratio=decrease[fg];[bg][fg]overlay=(W-w)/2:(H-h)/2,setsar=1,fps=30"

// The background must be blurred on a small copy and scaled up afterwards.
// Blurring the full-size copy is what made the filter slow (issue #137).
func TestFitFillFilter_BlursASmallCopyThenScalesUp(t *testing.T) {
	bg := regexp.MustCompile(`\[blur\](.*?)\[bg\]`).FindStringSubmatch(fitFillFilter)
	if bg == nil {
		t.Fatalf("no [blur]...[bg] chain in %s", fitFillFilter)
	}
	chain := bg[1]

	down := strings.Index(chain, "scale=270:480")
	blur := strings.Index(chain, "gblur=sigma=10")
	up := strings.LastIndex(chain, "scale=1080:1920")
	if down < 0 || blur < 0 || up < 0 {
		t.Fatalf("background chain must scale down to 270x480, blur, then scale up to 1080x1920: %s", chain)
	}
	if !(down < blur && blur < up) {
		t.Errorf("order must be downscale, blur, upscale; got positions %d, %d, %d in %s", down, blur, up, chain)
	}
	// Sigma 10 at a quarter of the size is the same radius as sigma 40 at full size.
	if strings.Contains(fitFillFilter, "sigma=40") {
		t.Error("the full-size sigma=40 blur must not come back")
	}
	// The foreground and compositing are unchanged.
	for _, want := range []string{
		"[main]scale=1080:1920:force_original_aspect_ratio=decrease[fg]",
		"[bg][fg]overlay=(W-w)/2:(H-h)/2,setsar=1,fps=30",
	} {
		if !strings.Contains(fitFillFilter, want) {
			t.Errorf("filter lost %q", want)
		}
	}
}

// A faceless clip goes through the blur-fill path end to end with real ffmpeg
// and must still come out as a 1080x1920, 30 fps, yuv420p H.264 clip.
func TestRenderOutput_BlurFillKeepsThePortraitPreset(t *testing.T) {
	requireFFmpeg(t)
	src := makeSource(t, 640, 360, 4, true)
	tr := makeTranscript(t, []models.Word{word("hello", 0.2, 0.7), word("world", 1.2, 1.7)})
	det := &stubDetector{boxes: [][]FaceBox{{}}} // no faces -> blur fill
	out := renderFixture(t, src, tr, 0, 3, false, det)

	info := probe(t, out)
	assertPortraitPreset(t, info, 3)
	if !info.hasAudio {
		t.Error("blur-fill clip lost its audio")
	}
}

// The cheaper filter must look like the old one. Render the same clip both
// ways and compare with ffmpeg's SSIM (1.0 = identical). On real footage the
// two measured 0.997 to 0.999; the floor here leaves room for the small
// synthetic fixture while still failing on a visibly different background.
func TestFitFillFilter_MatchesLegacyLook(t *testing.T) {
	requireFFmpeg(t)
	src := makeSource(t, 1280, 720, 3, false)
	dir := t.TempDir()
	r := &Renderer{}
	build := func(out, filter string) []string {
		args := r.buildCropCaptionArgs(src, "", out, 0, 2, false, "", true, models.CaptionStyle{})
		for i := range args {
			if i > 0 && args[i-1] == "-vf" {
				args[i] = strings.Replace(args[i], fitFillFilter, filter, 1)
			}
		}
		return args
	}
	oldOut, newOut := filepath.Join(dir, "old.mp4"), filepath.Join(dir, "new.mp4")
	runCmd(t, "ffmpeg", build(oldOut, legacyFitFillFilter)...)
	runCmd(t, "ffmpeg", build(newOut, fitFillFilter)...)

	out, err := exec.Command("ffmpeg", "-hide_banner", "-i", newOut, "-i", oldOut, "-lavfi", "ssim", "-f", "null", "-").CombinedOutput()
	if err != nil {
		t.Fatalf("ssim failed: %v\n%s", err, out)
	}
	m := regexp.MustCompile(`All:([0-9.]+)`).FindStringSubmatch(string(out))
	if m == nil {
		t.Fatalf("no SSIM result in:\n%s", out)
	}
	ssim, _ := strconv.ParseFloat(m[1], 64)
	if ssim < 0.98 {
		t.Errorf("SSIM(new, legacy) = %.4f, want >= 0.98: the blurred background no longer matches the old look", ssim)
	}
	t.Logf("SSIM(new, legacy) = %.4f", ssim)
}
