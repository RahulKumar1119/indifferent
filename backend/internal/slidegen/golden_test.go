package slidegen

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/rahul/indifferent/backend/internal/models"
)

// The Classic template must render exactly as it did before themes existed
// (issue #102: "existing projects using classic render exactly as before").
// testdata/golden/classic-*.png were produced by the renderer as it was before
// any theme code was added. Pixels are compared, not PNG bytes, so a change in
// Go's PNG encoder cannot fail the test. A deliberate change to Classic's look
// must replace these files in the same commit.

func goldenQuestion() QuestionSlideData {
	return QuestionSlideData{
		QuestionNumber: 3,
		TotalQuestions: 12,
		QuestionText:   "A company stores audit logs that must never be modified or deleted for seven years, and auditors need to read them quickly when asked. Which design satisfies both requirements?",
		Options: []models.Option{
			{Label: "A", Text: "Write the logs to a bucket with object lock in compliance mode and a seven year retention period"},
			{Label: "B", Text: "Keep the logs on an encrypted volume and snapshot it nightly"},
			{Label: "C", Text: "Email the logs to the audit team"},
			{Label: "D", Text: "Store the logs in a database table and restrict write access to administrators"},
		},
		CorrectIndex:   0,
		CorrectIndices: []int{0, 3},
	}
}

func goldenSlides() map[string]struct {
	template string
	data     interface{}
} {
	return map[string]struct {
		template string
		data     interface{}
	}{
		"question":  {"question.html", goldenQuestion()},
		"answer":    {"answer-reveal.html", goldenQuestion()},
		"countdown": {"countdown.html", CountdownSlideData{Seconds: 3}},
		"outro":     {"outro.html", OutroSlideData{ChannelName: "Quiz Channel"}},
	}
}

func decodePNG(t *testing.T, data []byte) image.Image {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("not a valid PNG: %v", err)
	}
	return img
}

// samePixels reports the first differing pixel, if any.
func samePixels(a, b image.Image) (bool, string) {
	if a.Bounds() != b.Bounds() {
		return false, fmt.Sprintf("bounds differ: %v vs %v", a.Bounds(), b.Bounds())
	}
	for y := a.Bounds().Min.Y; y < a.Bounds().Max.Y; y++ {
		for x := a.Bounds().Min.X; x < a.Bounds().Max.X; x++ {
			ar, ag, ab, aa := a.At(x, y).RGBA()
			br, bg, bb, ba := b.At(x, y).RGBA()
			if ar != br || ag != bg || ab != bb || aa != ba {
				return false, fmt.Sprintf("first difference at (%d,%d): %v vs %v", x, y, color.RGBAModel.Convert(a.At(x, y)), color.RGBAModel.Convert(b.At(x, y)))
			}
		}
	}
	return true, ""
}

func TestClassicRendersExactlyAsBeforeThemes(t *testing.T) {
	r := NewNativeRenderer()
	for name, slide := range goldenSlides() {
		t.Run(name, func(t *testing.T) {
			golden, err := os.ReadFile(filepath.Join("testdata", "golden", "classic-"+name+".png"))
			if err != nil {
				t.Fatalf("golden image missing: %v", err)
			}
			got, err := r.RenderTemplate("classic/"+slide.template, slide.data)
			if err != nil {
				t.Fatal(err)
			}
			if ok, why := samePixels(decodePNG(t, golden), decodePNG(t, got)); !ok {
				t.Fatalf("Classic %s slide no longer matches the pre-themes golden image: %s", name, why)
			}
		})
	}
}
