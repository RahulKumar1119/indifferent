//go:build captionfit

package render

// Writes the caption files that scripts/ops/caption_fit_check.sh renders with the real ffmpeg and
// fonts in alpine:3.21 (the packages the render images install) and measures. It is a generator,
// not a test of its own: the checks are in scripts/ops/caption_fit_verify.py, because they need
// pictures.
//
//	CAPTION_FIT_DIR=/tmp/fit go test -tags captionfit -run TestWriteCaptionFitCases ./internal/shorts/render

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rahul/indifferent/backend/internal/models"
)

type fitCase struct {
	ID        string    `json:"id"`
	Kind      string    `json:"kind"` // "layout", "sweep" or "equivalence"
	Preset    string    `json:"preset"`
	Font      string    `json:"font"`
	Units     int       `json:"units"`
	Text      string    `json:"text"`
	File      string    `json:"file"`
	Style     string    `json:"style,omitempty"` // force_style, for an SRT file
	Fits      bool      `json:"fits"`
	Shrunk    bool      `json:"shrunk"`
	Lines     int       `json:"lines"`
	EstPx     float64   `json:"estPx"` // widest line, as estimated
	OutlinePx int       `json:"outlinePx"`
	Other     string    `json:"other,omitempty"` // for equivalence: the ASS twin of the SRT file
	Times     []float64 `json:"times,omitempty"`
	At        float64   `json:"at"`       // when to take the frame: just before the cue ends
	SinglePx  float64   `json:"singlePx"` // the whole cue on one line at the size asked for
}

var fitTexts = map[string]string{
	"en-sentence": "The quick brown fox jumps over the lazy dog today",
	"en-long":     "internationalization counterrevolutionaries extraordinarily",
	"en-caps":     "MAKE THE VIDEO NOW WITH THIS",
	"en-digits":   "In 2026 we made 1234567890 videos for 45 000 people",
	"hi-sentence": "मैंने दादाजी के पुराने पीतल के दीपक को कैसे ठीक किया",
	"hi-conjunct": "क्षत्रिय ज्ञान श्रद्धा उज्ज्वल राष्ट्रीय स्वतंत्रता",
	"hinglish":    "Rohan ne apni pehli salary se maa ke liye silai machine kharidi",
	"mixed":       "आज hum बात करेंगे about दीपक की सफाई",
}

func writeCase(t *testing.T, dir string, c fitCase, doc string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, c.File), []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestWriteCaptionFitCases(t *testing.T) {
	dir := os.Getenv("CAPTION_FIT_DIR")
	if dir == "" {
		t.Skip("set CAPTION_FIT_DIR")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	var cases []fitCase
	n := 0
	// One cue of four words, shown from 0 to 2.4 seconds, so a frame at 2.3 s shows all of it.
	words := func(text string) []models.Word {
		fs := strings.Fields(text)
		if len(fs) > maxWordsPerCue {
			fs = fs[:maxWordsPerCue]
		}
		out := make([]models.Word, len(fs))
		for i, w := range fs {
			out[i] = models.Word{Text: w, Start: float64(i) * 0.6, End: float64(i)*0.6 + 0.55}
		}
		return out
	}
	for _, preset := range []string{"minimal", "bold", "karaoke", "clean"} {
		for _, font := range []string{"", "sans", "serif", "condensed"} {
			for _, units := range []int{8, 18, 30, 48, 72} {
				for name, text := range fitTexts {
					w := words(text)
					st := models.CaptionStyle{Preset: preset, FontSize: units, Font: font}
					doc, ext := Captions(w, 0, st, nil)
					texts := make([]string, len(w))
					for i := range w {
						texts[i] = w[i].Text
					}
					r := resolvedCaption(st)
					ps, ok := presetStyles[r.Preset]
					outline := 3
					if ok {
						outline = presetOutlinePx(ps)
					}
					sizePx := int(float64(r.FontSize)*assScale + 0.5)
					lay := layoutCue(texts, measureFont(font), sizePx, outline)
					widest := 0.0
					lines := [][]string{texts}
					if lay.breakAt > 0 {
						lines = [][]string{texts[:lay.breakAt], texts[lay.breakAt:]}
					}
					for _, ln := range lines {
						if x := textWidth(measureFont(font), strings.Join(ln, " "), lay.sizePx); x > widest {
							widest = x
						}
					}
					single := textWidth(measureFont(font), strings.Join(texts, " "), sizePx)
					n++
					c := fitCase{
						At: w[len(w)-1].End - 0.05, SinglePx: single,
						ID: fmt.Sprintf("c%04d", n), Kind: "layout", Preset: preset, Font: font, Units: units, Text: name,
						File: fmt.Sprintf("c%04d%s", n, ext), Fits: lay.fits, Shrunk: lay.shrunk, Lines: len(lines), EstPx: widest, OutlinePx: outline,
					}
					if ext == ".srt" {
						c.Style = ResolveForceStyle(st)
						// libass wraps SRT text itself, so the lines are its own business: the estimate is
						// of the whole cue on one line, split in two only where it would not fit.
					}
					cases = append(cases, c)
					writeCase(t, dir, c, doc)
				}
			}
		}
	}
	// The sweep must cross a line break in step with the words.
	for _, units := range []int{30, 40} {
		w := words(fitTexts["en-sentence"])
		st := models.CaptionStyle{Preset: "karaoke", FontSize: units}
		doc, ext := Captions(w, 0, st, nil)
		n++
		c := fitCase{ID: fmt.Sprintf("c%04d", n), Kind: "sweep", Preset: "karaoke", Units: units, Text: "en-sentence", File: fmt.Sprintf("c%04d%s", n, ext),
			Times: []float64{0.1, 0.5, 1.0, 1.5, 2.0, 2.3}, At: 2.3}
		cases = append(cases, c)
		writeCase(t, dir, c, doc)
	}
	// A cue that fits must look the same as SRT or as the ASS fallback: the fallback is only used
	// when a different cue in the same file needs it.
	for _, preset := range []string{"minimal", "bold", "clean"} {
		for _, font := range []string{"", "serif"} {
			for _, above := range []bool{false, true} {
				w := words("Hello there friend")
				st := models.CaptionStyle{Preset: preset, Font: font, AboveControls: above}
				n++
				c := fitCase{ID: fmt.Sprintf("c%04d", n), Kind: "equivalence", Preset: preset, Font: font, Units: 18, Text: fmt.Sprintf("above=%v", above),
					File: fmt.Sprintf("c%04d.srt", n), Style: ResolveForceStyle(st), Other: fmt.Sprintf("c%04d-twin.ass", n)}
				cases = append(cases, c)
				writeCase(t, dir, c, BuildSRT(w, 0))
				if err := os.WriteFile(filepath.Join(dir, c.Other), []byte(buildStyledASS(w, 0, st, nil, false)), 0o644); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	b, _ := json.MarshalIndent(cases, "", " ")
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("wrote %d cases to %s", len(cases), dir)
}
