package slidegen

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
	"golang.org/x/image/vector"
)

const (
	slideWidth  = 1920
	slideHeight = 1080

	marginX       = 110
	contentTop    = 140
	contentBottom = 1000
	cardGap       = 26
	cardPadding   = 30
)

// NativeRenderer renders slides using Go's image package — no browser needed.
type NativeRenderer struct {
	regular *opentype.Font
	bold    *opentype.Font
	faces   map[faceKey]font.Face
}

type faceKey struct {
	size int
	bold bool
}

// NewNativeRenderer creates a NativeRenderer instance with properly sized fonts.
func NewNativeRenderer() *NativeRenderer {
	regularFont, err := opentype.Parse(goregular.TTF)
	if err != nil {
		panic(fmt.Sprintf("slidegen: parse regular font: %v", err))
	}
	boldFont, err := opentype.Parse(gobold.TTF)
	if err != nil {
		panic(fmt.Sprintf("slidegen: parse bold font: %v", err))
	}
	return &NativeRenderer{
		regular: regularFont,
		bold:    boldFont,
		faces:   make(map[faceKey]font.Face),
	}
}

// face returns a cached face at size pixels. The slide size is fixed, so a
// handful of sizes covers every slide.
func (r *NativeRenderer) face(size float64, bold bool) font.Face {
	key := faceKey{size: int(math.Round(size)), bold: bold}
	if f, ok := r.faces[key]; ok {
		return f
	}
	src := r.regular
	if bold {
		src = r.bold
	}
	f, err := opentype.NewFace(src, &opentype.FaceOptions{
		Size:    float64(key.size),
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		panic(fmt.Sprintf("slidegen: create font face: %v", err))
	}
	r.faces[key] = f
	return f
}

// RenderTemplate renders a slide as a PNG using Go's native image library.
//
// templateName is "{theme}/{file}", for example "modern/answer-reveal.html".
// The theme part picks the palette and layout (ThemeFor; unknown names render
// as Classic). The file part picks the slide: a name containing "answer" is
// the answer reveal, anything else with question data is the question slide.
func (r *NativeRenderer) RenderTemplate(templateName string, data interface{}) ([]byte, error) {
	img := image.NewRGBA(image.Rect(0, 0, slideWidth, slideHeight))

	themeName, file := splitTemplateName(templateName)
	theme := ThemeFor(themeName)

	switch d := data.(type) {
	case QuestionSlideData:
		if strings.Contains(file, "answer") {
			fillBackground(img, theme.AnswerBG)
			r.drawQuestionAndOptions(img, theme, d, true)
		} else {
			fillBackground(img, theme.QuestionBG)
			r.drawQuestionAndOptions(img, theme, d, false)
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

// drawQuestionAndOptions draws the shared base layout for question and answer
// slides: a centred header, a centred question, then a 2×2 option grid (1×N
// when there is a single option). Designs differ in palette, type scale and
// whether options sit in cards (issue #104).
func (r *NativeRenderer) drawQuestionAndOptions(img *image.RGBA, t Theme, data QuestionSlideData, answer bool) {
	l := t.layout()

	draw.Draw(img, image.Rect(0, 0, slideWidth, 8), &image.Uniform{C: t.Accent}, image.Point{}, draw.Src)

	header := fmt.Sprintf("Question %d of %d", data.QuestionNumber, data.TotalQuestions)
	if answer {
		header += " — Answer"
	}
	headerFace := r.face(l.HeaderSize, true)
	r.drawCenteredText(img, header, slideWidth/2, 82, t.Accent, headerFace)

	cols, rows := 2, (len(data.Options)+1)/2
	if len(data.Options) == 1 {
		cols, rows = 1, 1
	}
	gridWidth := slideWidth - 2*marginX
	cardWidth := (gridWidth - cardGap*(cols-1)) / cols
	minGridHeight := 96
	if rows > 1 {
		minGridHeight = rows*96 + cardGap*(rows-1)
	}

	questionSize := l.QuestionSize
	questionTop := contentTop
	var questionLines []string
	var questionFace font.Face
	for {
		questionFace = r.face(questionSize, true)
		questionLines = r.wrapText(questionFace, data.QuestionText, int(l.QuestionMaxPx))
		lineHeight := int(questionSize * 1.28)
		optionsTop := questionTop + len(questionLines)*lineHeight + 48
		if optionsTop+minGridHeight <= contentBottom || questionSize <= 40 {
			break
		}
		questionSize -= 2
	}

	lineHeight := int(questionSize * 1.28)
	questionHeight := len(questionLines) * lineHeight
	optionsTop := questionTop + questionHeight + 48
	if optionsTop > contentBottom-minGridHeight {
		optionsTop = contentBottom - minGridHeight
	}

	baseline := questionTop + questionFace.Metrics().Ascent.Ceil()
	for _, line := range questionLines {
		r.drawCenteredText(img, line, slideWidth/2, baseline, t.Text, questionFace)
		baseline += lineHeight
	}

	if len(data.Options) == 0 {
		return
	}

	if !l.Cards {
		ruleY := optionsTop - 22
		ruleColor := blend(t.QuestionBG, t.Text, 0.32)
		if answer {
			ruleColor = blend(t.AnswerBG, t.Text, 0.32)
		}
		draw.Draw(img, image.Rect(marginX, ruleY, slideWidth-marginX, ruleY+2), &image.Uniform{C: ruleColor}, image.Point{}, draw.Src)
	}

	available := contentBottom - optionsTop
	cardHeight := (available - cardGap*(rows-1)) / rows
	if cardHeight > 260 {
		cardHeight = 260
	}
	if cardHeight < 88 {
		cardHeight = 88
	}
	gridHeight := rows*cardHeight + cardGap*(rows-1)
	gridTop := optionsTop
	if gridHeight < available {
		gridTop = optionsTop + (available-gridHeight)/2
	}

	optionSize := l.OptionSize
	labelSize := l.LabelSize
	var optionFace, labelFace font.Face
	var lines [][]string
	for {
		optionFace = r.face(optionSize, false)
		labelFace = r.face(labelSize, true)
		lineHeight = int(optionSize * 1.28)
		maxHeight := 0
		lines = make([][]string, len(data.Options))
		for i, opt := range data.Options {
			labelWidth := font.MeasureString(labelFace, opt.Label).Ceil()
			reserve := 0
			if answer && isCorrectIndex(i, data.CorrectIndices) {
				reserve = 58
			}
			textWidth := cardWidth - 2*cardPadding - labelWidth - 18 - reserve
			lines[i] = r.wrapText(optionFace, opt.Text, textWidth)
			if h := len(lines[i]) * lineHeight; h > maxHeight {
				maxHeight = h
			}
		}
		if maxHeight+20 <= cardHeight || optionSize <= 30 {
			break
		}
		optionSize -= 2
		if labelSize > 24 {
			labelSize--
		}
	}

	for i, opt := range data.Options {
		col, row := i%cols, i/cols
		x := marginX + col*(cardWidth+cardGap)
		y := gridTop + row*(cardHeight+cardGap)
		rect := image.Rect(x, y, x+cardWidth, y+cardHeight)

		correct := answer && isCorrectIndex(i, data.CorrectIndices)
		fill := t.cardBG()
		border := t.cardBorder()
		textColor := t.Option
		labelColor := t.Accent
		if answer {
			fill = t.answerCardBG()
			textColor = t.OptionWrong
			labelColor = t.OptionWrong
			if correct {
				fill = t.correctCardBG()
				border = blend(fill, t.Correct, 0.55)
				textColor = t.Correct
				labelColor = t.Correct
			}
		}

		if l.Cards {
			fillRoundedRect(img, rect, l.CardRadius, border)
			inner := rect.Inset(2)
			fillRoundedRect(img, inner, l.CardRadius-2, fill)
		}

		labelWidth := font.MeasureString(labelFace, opt.Label).Ceil()
		textLeft := rect.Min.X + cardPadding + labelWidth + 18
		lineHeight = int(optionSize * 1.28)
		blockHeight := len(lines[i]) * lineHeight
		firstBaseline := rect.Min.Y + (cardHeight-blockHeight)/2 + optionFace.Metrics().Ascent.Ceil()

		r.drawText(img, opt.Label, rect.Min.X+cardPadding, firstBaseline, labelColor, labelFace)
		for j, line := range lines[i] {
			r.drawText(img, line, textLeft, firstBaseline+j*lineHeight, textColor, optionFace)
		}
		if correct {
			drawCheck(img, rect.Max.X-cardPadding-24, rect.Min.Y+cardHeight/2, t.Correct)
		}
	}
}

func (r *NativeRenderer) drawCountdownSlide(img *image.RGBA, t Theme, data CountdownSlideData) {
	numberFace := r.face(280, true)
	labelFace := r.face(40, false)

	number := fmt.Sprintf("%d", data.Seconds)
	numberBaseline := 600
	r.drawCenteredText(img, number, slideWidth/2, numberBaseline, t.Accent, numberFace)
	r.drawCenteredText(img, "seconds remaining", slideWidth/2, numberBaseline+130, t.Subtle, labelFace)
}

func (r *NativeRenderer) drawOutroSlide(img *image.RGBA, t Theme, data OutroSlideData) {
	channelFace := r.face(96, true)
	thanksFace := r.face(56, true)
	subtitleFace := r.face(32, false)

	channelLines := r.wrapText(channelFace, data.ChannelName, 1500)
	lineHeight := 115
	top := 380
	if len(channelLines) > 1 {
		top = 320
	}
	baseline := top + channelFace.Metrics().Ascent.Ceil()
	for _, line := range channelLines {
		r.drawCenteredText(img, line, slideWidth/2, baseline, t.Accent, channelFace)
		baseline += lineHeight
	}

	r.drawCenteredText(img, "Thanks for watching!", slideWidth/2, baseline+70, t.Text, thanksFace)
	r.drawCenteredText(img, "Subscribe & Like for more quiz videos", slideWidth/2, baseline+140, t.Subtle, subtitleFace)
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

func (r *NativeRenderer) drawText(img *image.RGBA, text string, x, y int, col color.Color, face font.Face) {
	if text == "" {
		return
	}
	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(col),
		Face: face,
		Dot:  fixed.Point26_6{X: fixed.I(x), Y: fixed.I(y)},
	}
	d.DrawString(text)
}

func (r *NativeRenderer) drawCenteredText(img *image.RGBA, text string, centerX, y int, col color.Color, face font.Face) {
	width := font.MeasureString(face, text).Ceil()
	r.drawText(img, text, centerX-width/2, y, col, face)
}

// wrapText splits text into lines no wider than maxWidth pixels for the given
// face. It breaks an over-long word by rune rather than letting it run off the
// slide.
func (r *NativeRenderer) wrapText(face font.Face, text string, maxWidth int) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}
	if maxWidth < 1 {
		maxWidth = 1
	}

	var lines []string
	current := ""
	for _, word := range words {
		if font.MeasureString(face, word).Ceil() > maxWidth {
			if current != "" {
				lines = append(lines, current)
				current = ""
			}
			lines = append(lines, splitWord(face, word, maxWidth)...)
			continue
		}
		candidate := word
		if current != "" {
			candidate = current + " " + word
		}
		if font.MeasureString(face, candidate).Ceil() <= maxWidth {
			current = candidate
		} else {
			lines = append(lines, current)
			current = word
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}

func splitWord(face font.Face, word string, maxWidth int) []string {
	var lines []string
	current := ""
	for _, ch := range word {
		candidate := current + string(ch)
		if font.MeasureString(face, candidate).Ceil() <= maxWidth || current == "" {
			current = candidate
			continue
		}
		lines = append(lines, current)
		current = string(ch)
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}

// fillRoundedRect fills a rounded rectangle using an anti-aliased rasterizer.
func fillRoundedRect(img *image.RGBA, rect image.Rectangle, radius int, c color.RGBA) {
	if rect.Dx() <= 0 || rect.Dy() <= 0 {
		return
	}
	if radius <= 0 {
		draw.Draw(img, rect, &image.Uniform{C: c}, image.Point{}, draw.Src)
		return
	}

	width, height := rect.Dx(), rect.Dy()
	corner := float32(math.Min(float64(radius), math.Min(float64(width)/2, float64(height)/2)))
	rast := vector.NewRasterizer(width, height)
	x0, y0 := float32(0), float32(0)
	x1, y1 := float32(width), float32(height)

	rast.MoveTo(x0+corner, y0)
	rast.LineTo(x1-corner, y0)
	rast.QuadTo(x1, y0, x1, y0+corner)
	rast.LineTo(x1, y1-corner)
	rast.QuadTo(x1, y1, x1-corner, y1)
	rast.LineTo(x0+corner, y1)
	rast.QuadTo(x0, y1, x0, y1-corner)
	rast.LineTo(x0, y0+corner)
	rast.QuadTo(x0, y0, x0+corner, y0)
	rast.ClosePath()

	rast.Draw(img, rect, image.NewUniform(c), image.Point{})
}

// drawCheck draws a check mark with round strokes, so the answer reveal does
// not depend on a font glyph being present.
func drawCheck(img *image.RGBA, cx, cy int, c color.RGBA) {
	drawThickLine(img, cx-16, cy, cx-4, cy+12, 5, c)
	drawThickLine(img, cx-4, cy+12, cx+18, cy-15, 5, c)
}

func drawThickLine(img *image.RGBA, x0, y0, x1, y1, radius int, c color.RGBA) {
	steps := int(math.Max(math.Abs(float64(x1-x0)), math.Abs(float64(y1-y0))))
	if steps == 0 {
		drawDisc(img, x0, y0, radius, c)
		return
	}
	for i := 0; i <= steps; i++ {
		t := float64(i) / float64(steps)
		x := int(math.Round(float64(x0) + t*float64(x1-x0)))
		y := int(math.Round(float64(y0) + t*float64(y1-y0)))
		drawDisc(img, x, y, radius, c)
	}
}

func drawDisc(img *image.RGBA, cx, cy, radius int, c color.RGBA) {
	for y := -radius; y <= radius; y++ {
		for x := -radius; x <= radius; x++ {
			if x*x+y*y <= radius*radius {
				img.SetRGBA(cx+x, cy+y, c)
			}
		}
	}
}
