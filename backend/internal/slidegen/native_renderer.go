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
func (r *NativeRenderer) RenderTemplate(templateName string, data interface{}) ([]byte, error) {
	img := image.NewRGBA(image.Rect(0, 0, slideWidth, slideHeight))

	// White background
	white := color.RGBA{R: 255, G: 255, B: 255, A: 255}
	draw.Draw(img, img.Bounds(), &image.Uniform{C: white}, image.Point{}, draw.Src)

	switch d := data.(type) {
	case QuestionSlideData:
		if strings.Contains(templateName, "answer") {
			r.drawAnswerRevealSlide(img, d)
		} else {
			r.drawQuestionSlide(img, d)
		}
	case CountdownSlideData:
		r.drawCountdownSlide(img, d)
	case OutroSlideData:
		r.drawOutroSlide(img, d)
	default:
		return nil, fmt.Errorf("unsupported slide data type: %T", data)
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("encode PNG: %w", err)
	}

	return buf.Bytes(), nil
}

func (r *NativeRenderer) drawQuestionSlide(img *image.RGBA, data QuestionSlideData) {
	darkText := color.RGBA{R: 30, G: 30, B: 30, A: 255}
	accent := color.RGBA{R: 124, G: 58, B: 237, A: 255}  // Purple
	grayText := color.RGBA{R: 80, G: 80, B: 80, A: 255}

	// Draw purple accent bar at top
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

func (r *NativeRenderer) drawAnswerRevealSlide(img *image.RGBA, data QuestionSlideData) {
	darkText := color.RGBA{R: 30, G: 30, B: 30, A: 255}
	accent := color.RGBA{R: 124, G: 58, B: 237, A: 255}
	green := color.RGBA{R: 22, G: 163, B: 74, A: 255}      // Green for correct
	grayText := color.RGBA{R: 140, G: 140, B: 140, A: 255}

	// Purple accent bar
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

func (r *NativeRenderer) drawCountdownSlide(img *image.RGBA, data CountdownSlideData) {
	accent := color.RGBA{R: 124, G: 58, B: 237, A: 255}
	grayText := color.RGBA{R: 100, G: 100, B: 100, A: 255}

	// Light gray background for countdown
	lightBg := color.RGBA{R: 245, G: 245, B: 250, A: 255}
	draw.Draw(img, img.Bounds(), &image.Uniform{C: lightBg}, image.Point{}, draw.Src)

	// Draw large countdown number centered
	numStr := fmt.Sprintf("%d", data.Seconds)
	r.drawTextWithFace(img, numStr, 880, 600, accent, r.largeFace)

	// Label below
	r.drawTextWithFace(img, "seconds remaining", 800, 720, grayText, r.regularFace)
}

func (r *NativeRenderer) drawOutroSlide(img *image.RGBA, data OutroSlideData) {
	accent := color.RGBA{R: 124, G: 58, B: 237, A: 255}
	darkText := color.RGBA{R: 30, G: 30, B: 30, A: 255}
	grayText := color.RGBA{R: 100, G: 100, B: 100, A: 255}

	// Light purple background
	lightBg := color.RGBA{R: 250, G: 245, B: 255, A: 255}
	draw.Draw(img, img.Bounds(), &image.Uniform{C: lightBg}, image.Point{}, draw.Src)

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
