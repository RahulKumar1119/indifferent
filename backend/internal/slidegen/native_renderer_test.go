package slidegen

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"

	"github.com/rahul/indifferent/backend/internal/models"
)

func TestNativeRenderer_QuestionSlide(t *testing.T) {
	renderer := NewNativeRenderer()

	data := QuestionSlideData{
		QuestionNumber: 1,
		TotalQuestions: 5,
		QuestionText:   "What is the capital of France?",
		Options: []models.Option{
			{Label: "A", Text: "London"},
			{Label: "B", Text: "Paris"},
			{Label: "C", Text: "Berlin"},
			{Label: "D", Text: "Madrid"},
		},
		CorrectIndex: 1,
	}

	pngBytes, err := renderer.RenderTemplate("classic/question.html", data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify it's a valid PNG
	img, err := png.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		t.Fatalf("output is not a valid PNG: %v", err)
	}

	bounds := img.Bounds()
	if bounds.Dx() != 1920 || bounds.Dy() != 1080 {
		t.Errorf("expected 1920x1080, got %dx%d", bounds.Dx(), bounds.Dy())
	}
}

func TestNativeRenderer_AnswerRevealSlide(t *testing.T) {
	renderer := NewNativeRenderer()

	data := QuestionSlideData{
		QuestionNumber: 2,
		TotalQuestions: 5,
		QuestionText:   "Which planet is closest to the Sun?",
		Options: []models.Option{
			{Label: "A", Text: "Venus"},
			{Label: "B", Text: "Mercury"},
			{Label: "C", Text: "Mars"},
			{Label: "D", Text: "Earth"},
		},
		CorrectIndex:   1,
		CorrectIndices: []int{1},
	}

	pngBytes, err := renderer.RenderTemplate("classic/answer-reveal.html", data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	img, err := png.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		t.Fatalf("output is not a valid PNG: %v", err)
	}

	bounds := img.Bounds()
	if bounds.Dx() != 1920 || bounds.Dy() != 1080 {
		t.Errorf("expected 1920x1080, got %dx%d", bounds.Dx(), bounds.Dy())
	}
}

func TestNativeRenderer_CountdownSlide(t *testing.T) {
	renderer := NewNativeRenderer()

	for sec := 1; sec <= 5; sec++ {
		data := CountdownSlideData{Seconds: sec}
		pngBytes, err := renderer.RenderTemplate("classic/countdown.html", data)
		if err != nil {
			t.Fatalf("unexpected error for seconds=%d: %v", sec, err)
		}

		img, err := png.Decode(bytes.NewReader(pngBytes))
		if err != nil {
			t.Fatalf("output is not a valid PNG for seconds=%d: %v", sec, err)
		}

		bounds := img.Bounds()
		if bounds.Dx() != 1920 || bounds.Dy() != 1080 {
			t.Errorf("seconds=%d: expected 1920x1080, got %dx%d", sec, bounds.Dx(), bounds.Dy())
		}
	}
}

func TestNativeRenderer_OutroSlide(t *testing.T) {
	renderer := NewNativeRenderer()

	data := OutroSlideData{ChannelName: "Quiz Channel"}
	pngBytes, err := renderer.RenderTemplate("classic/outro.html", data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	img, err := png.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		t.Fatalf("output is not a valid PNG: %v", err)
	}

	bounds := img.Bounds()
	if bounds.Dx() != 1920 || bounds.Dy() != 1080 {
		t.Errorf("expected 1920x1080, got %dx%d", bounds.Dx(), bounds.Dy())
	}
}

func TestNativeRenderer_UnsupportedDataType(t *testing.T) {
	renderer := NewNativeRenderer()

	_, err := renderer.RenderTemplate("classic/question.html", "invalid data")
	if err == nil {
		t.Fatal("expected error for unsupported data type, got nil")
	}
}

func TestNativeRenderer_LongQuestionTextWraps(t *testing.T) {
	renderer := NewNativeRenderer()

	data := QuestionSlideData{
		QuestionNumber: 1,
		TotalQuestions: 1,
		QuestionText:   "This is a very long question text that should be wrapped across multiple lines when rendered on the slide because it exceeds the maximum line width defined in the wrapText function which is set to approximately eighty characters per line for readability.",
		Options: []models.Option{
			{Label: "A", Text: "Option A"},
			{Label: "B", Text: "Option B"},
		},
		CorrectIndex: 0,
	}

	pngBytes, err := renderer.RenderTemplate("classic/question.html", data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(pngBytes) == 0 {
		t.Fatal("expected non-empty PNG output")
	}
}

func TestNativeRenderer_AnswerRevealSlide_MultipleCorrect(t *testing.T) {
	renderer := NewNativeRenderer()

	data := QuestionSlideData{
		QuestionNumber: 1,
		TotalQuestions: 3,
		QuestionText:   "Select TWO correct answers.",
		Options: []models.Option{
			{Label: "A", Text: "Wrong"},
			{Label: "B", Text: "Correct one"},
			{Label: "C", Text: "Wrong"},
			{Label: "D", Text: "Correct two"},
		},
		CorrectIndex:   1,
		CorrectIndices: []int{1, 3},
	}

	pngBytes, err := renderer.RenderTemplate("classic/answer-reveal.html", data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	img, err := png.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		t.Fatalf("output is not a valid PNG: %v", err)
	}

	bounds := img.Bounds()
	if bounds.Dx() != 1920 || bounds.Dy() != 1080 {
		t.Errorf("expected 1920x1080, got %dx%d", bounds.Dx(), bounds.Dy())
	}
}

func TestIsCorrectIndex(t *testing.T) {
	tests := []struct {
		idx     int
		indices []int
		want    bool
	}{
		{0, []int{0, 2}, true},
		{1, []int{0, 2}, false},
		{2, []int{0, 2}, true},
		{0, []int{}, false},
		{0, nil, false},
	}

	for _, tt := range tests {
		got := isCorrectIndex(tt.idx, tt.indices)
		if got != tt.want {
			t.Errorf("isCorrectIndex(%d, %v) = %v, want %v", tt.idx, tt.indices, got, tt.want)
		}
	}
}

func maxLengthPreviewData() QuestionSlideData {
	longOption := "A deliberately long answer option that reaches exactly one hundred characters before it wraps"
	if len(longOption) < 100 {
		longOption += strings.Repeat(" x", 100-len(longOption))
	}
	longOption = longOption[:100]
	question := strings.Repeat("Which design satisfies the retention requirement without changing it? ", 8)
	question = question[:500]
	return QuestionSlideData{
		QuestionNumber: 12,
		TotalQuestions: 12,
		QuestionText:   question,
		Options: []models.Option{
			{Label: "A", Text: longOption},
			{Label: "B", Text: "Keep the logs on an encrypted volume and snapshot it nightly for seven years"},
			{Label: "C", Text: "Email the logs to the audit team every morning"},
			{Label: "D", Text: "Store the logs in a database table and restrict write access"},
		},
		CorrectIndex:   0,
		CorrectIndices: []int{0},
	}
}

func TestLongContentStaysOnCanvas(t *testing.T) {
	data := maxLengthPreviewData()
	if len(data.QuestionText) != 500 {
		t.Fatalf("question length = %d, want 500", len(data.QuestionText))
	}
	if len(data.Options[0].Text) != 100 {
		t.Fatalf("option length = %d, want 100", len(data.Options[0].Text))
	}

	for _, name := range ThemeNames() {
		for _, size := range []string{"s", "m", "l"} {
			t.Run(name+"/"+size, func(t *testing.T) {
				data.DesignOptions.TextSize = size
				for _, slide := range []struct {
					file string
					bg   color.RGBA
				}{
					{"question.html", ThemeFor(name).QuestionBG},
					{"answer-reveal.html", ThemeFor(name).AnswerBG},
				} {
					img := renderImage(t, name, slide.file, data)
					if x, y, ok := inkInFrame(img, slide.bg, 12); ok {
						t.Errorf("%s: content reaches the canvas edge at (%d,%d)", slide.file, x, y)
					}
				}
			})
		}
	}
}

// The canvas-edge check cannot see a question that runs into the cards,
// because the cards are painted over it. Assert the planned question block
// ends above the option grid for the maximum accepted question length.
func TestMaxLengthQuestionDoesNotOverlapOptions(t *testing.T) {
	data := maxLengthPreviewData()
	r := NewNativeRenderer()
	for _, name := range ThemeNames() {
		for _, size := range []string{"s", "m", "l"} {
			data.DesignOptions.TextSize = size
			l := ThemeFor(name).layout()
			l.QuestionSize *= textScale(data.DesignOptions)
			plan := r.planQuestion(l, data.QuestionText, 2*96+cardGap)
			blockBottom := contentTop + len(plan.lines)*plan.lineHeight
			if blockBottom > plan.optionsTop {
				t.Errorf("%s/%s: question block ends at %d, options start at %d", name, size, blockBottom, plan.optionsTop)
			}
			if plan.optionsTop+2*96+cardGap > contentBottom {
				t.Errorf("%s/%s: options grid ends at %d, canvas content limit %d", name, size, plan.optionsTop+2*96+cardGap, contentBottom)
			}
		}
	}
}

func inkInFrame(img *image.RGBA, bg color.RGBA, frame int) (int, int, bool) {
	b := img.Bounds()
	for y := frame; y < b.Max.Y; y++ {
		for x := 0; x < b.Max.X; x++ {
			if x >= frame && x < b.Max.X-frame && y < b.Max.Y-frame {
				continue
			}
			if img.RGBAAt(x, y) != bg {
				return x, y, true
			}
		}
	}
	return 0, 0, false
}
