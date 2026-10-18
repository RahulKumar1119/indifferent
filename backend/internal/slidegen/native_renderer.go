package slidegen

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

const (
	slideWidth  = 1920
	slideHeight = 1080
)

// NativeRenderer renders slides using Go's image package — no browser needed.
type NativeRenderer struct {
	regularFace font.Face
	boldFace    font.Face
	largeFace   font.Face
}

// NewNativeRenderer creates a NativeRenderer instance with properly sized fonts.
func NewNativeRenderer() *NativeRenderer {
	r := &NativeRenderer{}

	// Parse regular font
	regularFont, _ := opentype.Parse(goregular.TTF)
	r.regularFace, _ = opentype.NewFace(regularFont, &opentype.FaceOptions{
		Size:    28,
		DPI:     72,
		Hinting: font.HintingFull,
	})

	// Parse bold font
	boldFont, _ := opentype.Parse(gobold.TTF)
	r.boldFace, _ = opentype.NewFace(boldFont, &opentype.FaceOptions{
		Size:    36,
		DPI:     72,
		Hinting: font.HintingFull,
	})

	// Large font for countdown numbers
	r.largeFace, _ = opentype.NewFace(boldFont, &opentype.FaceOptions{
		Size:    200,
		DPI:     72,
		Hinting: font.HintingFull,
	})

	return r
}

// RenderTemplate renders a slide as a PNG using Go's native image library.
//
// templateName is "{theme}/{file}", for example "modern/answer-reveal.html".
// The theme part picks the palette (ThemeFor; unknown names render as
// Classic). The file part picks the slide: a name containing "answer" is the
// answer reveal, anything else with question data is the question slide.
func (r *NativeRenderer) RenderTemplate(templateName string, data interface{}) ([]byte, error) {
	img := image.NewRGBA(image.Rect(0, 0, slideWidth, slideHeight))

	themeName, file := splitTemplateName(templateName)
	theme := ThemeFor(themeName)

	switch d := data.(type) {
	case QuestionSlideData:
		if strings.Contains(file, "answer") {
			fillBackground(img, theme.AnswerBG)
			r.drawAnswerRevealSlide(img, theme, d)
		} else {
			fillBackground(img, theme.QuestionBG)
			r.drawQuestionSlide(img, theme, d)
		}
	case CountdownSlideData:
		fillBackground(img, theme.CountdownBG)
		r.drawCountdownSlide(img, theme, d)
	case OutroSlideData:
		fillBackground(img, theme.OutroBG)
		r.drawOutroSlide(img, theme, d)
	default:
		return nil, fmt.Errorf("unsupported slide data type: %T", data)
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("encode PNG: %w", err)
	}

	return buf.Bytes(), nil
}

// fillBackground paints the whole slide one colour.
func fillBackground(img *image.RGBA, c color.RGBA) {
	draw.Draw(img, img.Bounds(), &image.Uniform{C: c}, image.Point{}, draw.Src)
}

func (r *NativeRenderer) drawQuestionSlide(img *image.RGBA, t Theme, data QuestionSlideData) {
	darkText := t.Text
	accent := t.Accent
	grayText := t.Option

	// Draw accent bar at top
	accentBar := image.Rect(0, 0, slideWidth, 8)
	draw.Draw(img, accentBar, &image.Uniform{C: accent}, image.Point{}, draw.Src)

	// Header - "Question X of Y"
	header := fmt.Sprintf("Question %d of %d", data.QuestionNumber, data.TotalQuestions)
	r.drawTextWithFace(img, header, 100, 80, accent, r.boldFace)

	// Question text (wrapped)
	lines := wrapText(data.QuestionText, 60)
	y := 160
	for _, line := range lines {
		r.drawTextWithFace(img, line, 100, y, darkText, r.boldFace)
		y += 50
	}

	// Options
	y += 40
	for _, opt := range data.Options {
		prefix := fmt.Sprintf("%s)  ", opt.Label)
		// Wrap option text at 75 chars (accounting for the label prefix)
		optLines := wrapText(opt.Text, 75)
		for i, line := range optLines {
			if i == 0 {
				r.drawTextWithFace(img, prefix+line, 140, y, grayText, r.regularFace)
			} else {
				// Indent continuation lines
				r.drawTextWithFace(img, "     "+line, 140, y, grayText, r.regularFace)
			}
			y += 38
		}
		y += 15 // Extra spacing between options
	}
}

func (r *NativeRenderer) drawAnswerRevealSlide(img *image.RGBA, t Theme, data QuestionSlideData) {
	darkText := t.Text
	accent := t.Accent
	green := t.Correct
	grayText := t.OptionWrong

	// Accent bar
	accentBar := image.Rect(0, 0, slideWidth, 8)
	draw.Draw(img, accentBar, &image.Uniform{C: accent}, image.Point{}, draw.Src)

	// Header
	header := fmt.Sprintf("Question %d of %d — Answer", data.QuestionNumber, data.TotalQuestions)
	r.drawTextWithFace(img, header, 100, 80, accent, r.boldFace)

	// Question text
	lines := wrapText(data.QuestionText, 60)
	y := 160
	for _, line := range lines {
		r.drawTextWithFace(img, line, 100, y, darkText, r.boldFace)
		y += 50
	}

	// Options with correct answer(s) highlighted
	y += 40
	for i, opt := range data.Options {
		prefix := fmt.Sprintf("%s)  ", opt.Label)
		optColor := grayText
		suffix := ""
		if isCorrectIndex(i, data.CorrectIndices) {
			optColor = green
			suffix = "  ✓ Correct"
		}
		// Wrap option text
		optLines := wrapText(opt.Text+suffix, 75)
		for j, line := range optLines {
			if j == 0 {
				r.drawTextWithFace(img, prefix+line, 140, y, optColor, r.regularFace)
			} else {
				r.drawTextWithFace(img, "     "+line, 140, y, optColor, r.regularFace)
			}
			y += 38
		}
		y += 15
	}
}

// isCorrectIndex checks whether idx is in the correctIndices slice.
func isCorrectIndex(idx int, correctIndices []int) bool {
	for _, ci := range correctIndices {
		if ci == idx {
			return true
		}
	}
	return false
}

func (r *NativeRenderer) drawCountdownSlide(img *image.RGBA, t Theme, data CountdownSlideData) {
	accent := t.Accent
	grayText := t.Subtle

	// Draw large countdown number centered
	numStr := fmt.Sprintf("%d", data.Seconds)
	r.drawTextWithFace(img, numStr, 880, 600, accent, r.largeFace)

	// Label below
	r.drawTextWithFace(img, "seconds remaining", 800, 720, grayText, r.regularFace)
}

func (r *NativeRenderer) drawOutroSlide(img *image.RGBA, t Theme, data OutroSlideData) {
	accent := t.Accent
	darkText := t.Text
	grayText := t.Subtle

	r.drawTextWithFace(img, data.ChannelName, 700, 420, accent, r.largeFace)
	r.drawTextWithFace(img, "Thanks for watching!", 720, 560, darkText, r.boldFace)
	r.drawTextWithFace(img, "Subscribe & Like for more quiz videos", 640, 640, grayText, r.regularFace)
}

func (r *NativeRenderer) drawTextWithFace(img *image.RGBA, text string, x, y int, col color.Color, face font.Face) {
	point := fixed.Point26_6{X: fixed.I(x), Y: fixed.I(y)}
	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(col),
		Face: face,
		Dot:  point,
	}
	d.DrawString(text)
}

// wrapText splits text into lines of at most maxWidth characters.
func wrapText(text string, maxWidth int) []string {
	words := strings.Fields(text)
	var lines []string
	var current string

	for _, word := range words {
		if len(current)+len(word)+1 > maxWidth {
			if current != "" {
				lines = append(lines, current)
			}
			current = word
		} else {
			if current == "" {
				current = word
			} else {
				current += " " + word
			}
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}
