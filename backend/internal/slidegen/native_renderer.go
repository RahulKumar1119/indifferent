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
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

const (
	slideWidth  = 1920
	slideHeight = 1080
)

// NativeRenderer renders slides using Go's image package — no browser needed.
// This is suitable for Lambda deployment where Playwright/Chromium is not available.
type NativeRenderer struct{}

// NewNativeRenderer creates a NativeRenderer instance.
func NewNativeRenderer() *NativeRenderer {
	return &NativeRenderer{}
}

// RenderTemplate renders a slide as a PNG using Go's native image library.
// The templateName parameter is accepted for interface compatibility but the
// rendering is done programmatically based on the data type.
func (r *NativeRenderer) RenderTemplate(templateName string, data interface{}) ([]byte, error) {
	img := image.NewRGBA(image.Rect(0, 0, slideWidth, slideHeight))

	// Dark blue background
	bgColor := color.RGBA{R: 26, G: 26, B: 46, A: 255}
	draw.Draw(img, img.Bounds(), &image.Uniform{C: bgColor}, image.Point{}, draw.Src)

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
	white := color.RGBA{R: 255, G: 255, B: 255, A: 255}
	accent := color.RGBA{R: 78, G: 204, B: 163, A: 255}

	// Header
	header := fmt.Sprintf("Question %d of %d", data.QuestionNumber, data.TotalQuestions)
	r.drawText(img, header, 80, 100, accent)

	// Question text (wrapped)
	lines := wrapText(data.QuestionText, 80)
	y := 200
	for _, line := range lines {
		r.drawText(img, line, 80, y, white)
		y += 30
	}

	// Options
	y += 40
	for _, opt := range data.Options {
		optText := fmt.Sprintf("%s) %s", opt.Label, opt.Text)
		r.drawText(img, optText, 120, y, white)
		y += 50
	}
}

func (r *NativeRenderer) drawAnswerRevealSlide(img *image.RGBA, data QuestionSlideData) {
	white := color.RGBA{R: 255, G: 255, B: 255, A: 255}
	accent := color.RGBA{R: 78, G: 204, B: 163, A: 255}
	dimmed := color.RGBA{R: 150, G: 150, B: 150, A: 255}

	// Header
	header := fmt.Sprintf("Question %d of %d - Answer", data.QuestionNumber, data.TotalQuestions)
	r.drawText(img, header, 80, 100, accent)

	// Question text
	lines := wrapText(data.QuestionText, 80)
	y := 200
	for _, line := range lines {
		r.drawText(img, line, 80, y, white)
		y += 30
	}

	// Options with correct answer highlighted
	y += 40
	for i, opt := range data.Options {
		optText := fmt.Sprintf("%s) %s", opt.Label, opt.Text)
		optColor := dimmed
		if i == data.CorrectIndex {
			optText = fmt.Sprintf("%s) %s  ✓", opt.Label, opt.Text)
			optColor = accent
		}
		r.drawText(img, optText, 120, y, optColor)
		y += 50
	}
}

func (r *NativeRenderer) drawCountdownSlide(img *image.RGBA, data CountdownSlideData) {
	accent := color.RGBA{R: 78, G: 204, B: 163, A: 255}
	white := color.RGBA{R: 255, G: 255, B: 255, A: 255}

	// Draw countdown number (centered, drawn multiple times for bold effect)
	numStr := fmt.Sprintf("%d", data.Seconds)
	r.drawTextBold(img, numStr, 930, 540, accent)

	// Label below
	r.drawText(img, "seconds remaining", 860, 620, white)
}

func (r *NativeRenderer) drawOutroSlide(img *image.RGBA, data OutroSlideData) {
	white := color.RGBA{R: 255, G: 255, B: 255, A: 255}
	accent := color.RGBA{R: 78, G: 204, B: 163, A: 255}

	r.drawTextBold(img, data.ChannelName, 750, 400, accent)
	r.drawText(img, "Thanks for watching!", 780, 520, white)
	r.drawText(img, "Subscribe & Like", 810, 620, white)
}

func (r *NativeRenderer) drawText(img *image.RGBA, text string, x, y int, col color.Color) {
	face := basicfont.Face7x13
	point := fixed.Point26_6{X: fixed.I(x), Y: fixed.I(y)}
	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(col),
		Face: face,
		Dot:  point,
	}
	d.DrawString(text)
}

func (r *NativeRenderer) drawTextBold(img *image.RGBA, text string, x, y int, col color.Color) {
	face := basicfont.Face7x13
	// Draw multiple times with slight offsets for a bold effect
	for dx := 0; dx < 3; dx++ {
		for dy := 0; dy < 3; dy++ {
			point := fixed.Point26_6{X: fixed.I(x + dx), Y: fixed.I(y + dy)}
			d := &font.Drawer{
				Dst:  img,
				Src:  image.NewUniform(col),
				Face: face,
				Dot:  point,
			}
			d.DrawString(text)
		}
	}
}

// wrapText splits text into lines of at most maxWidth characters, breaking at word boundaries.
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
