//go:build hookfit

package render

// Writes the hook and caption files that scripts/ops/hook_fit_check.sh renders with the
// production render image (backend/cmd/shortsrender/Dockerfile: alpine, ffmpeg with libass, the
// DejaVu and Noto Devanagari fonts) and measures. It is a generator: the checks are in
// scripts/ops/hook_fit_verify.py, because they need pictures.
//
//	HOOK_FIT_DIR=/tmp/hookfit go test -tags hookfit -run TestWriteHookFitCases ./internal/shorts/render

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rahul/indifferent/backend/internal/models"
)

type hookFitCase struct {
	ID       string `json:"id"`
	Lang     string `json:"lang"`
	Text     string `json:"text"`
	Style    string `json:"style"`
	Position string `json:"position"`
	Captions string `json:"captions"` // a label
	CapsFile string `json:"capsFile,omitempty"`
	CapsStyl string `json:"capsStyle,omitempty"` // force_style for an SRT file
	HookFile string `json:"hookFile"`
	// The filters exactly as the renderer writes them.
	CapsFilter string  `json:"capsFilter,omitempty"`
	HookFilt   string  `json:"hookFilter"`
	Seconds    float64 `json:"seconds"`
	FadeMs     int     `json:"fadeMs"`
	Fade       bool    `json:"fade"` // sample the fades too
	// The layout's estimate: the block (box or outline included) and its widest line.
	BlockY0 float64 `json:"blockY0"`
	BlockY1 float64 `json:"blockY1"`
	EstW    float64 `json:"estW"`
	Lines   int     `json:"lines"`
	SizePx  int     `json:"sizePx"`
	CapsY0  float64 `json:"capsY0"`
	CapsY1  float64 `json:"capsY1"`
}

var hookFitTexts = map[string]string{
	"en80":    hookEnglish80,
	"hi80":    hookHindi80,
	"enShort": hookShort,
	"hiShort": "यह गलती मत करना",
	"hiConj":  "क्षत्रिय ज्ञान श्रद्धा उज्ज्वल राष्ट्रीय स्वतंत्रता का प्रश्न",
}

// captionFitWords is a cue of four long words shown from 0.5 s to 2.4 s, so captions are on screen
// with the hook at 1 s.
func captionFitWords() []models.Word {
	texts := []string{"Absolutely", "everything", "changed", "overnight"}
	var out []models.Word
	for i, w := range texts {
		s := 0.5 + float64(i)*0.475
		out = append(out, models.Word{Text: w, Start: s, End: s + 0.47})
	}
	return out
}

func TestWriteHookFitCases(t *testing.T) {
	dir := os.Getenv("HOOK_FIT_DIR")
	if dir == "" {
		t.Skip("HOOK_FIT_DIR not set")
	}
	capChoices := map[string]HookCaptions{
		"none":            {None: true},
		"bold":            {Style: models.CaptionStyle{}},
		"bold-above":      {Style: models.CaptionStyle{AboveControls: true}},
		"minimal-32":      {Style: models.CaptionStyle{Preset: CaptionMinimal, FontSize: 32}},
		"clean-serif":     {Style: models.CaptionStyle{Preset: CaptionClean, Font: FontSerif}},
		"karaoke":         {Style: models.CaptionStyle{Preset: CaptionKaraoke}},
		"bold-top":        {Style: models.CaptionStyle{Position: "top"}},
		"bold-middle-40":  {Style: models.CaptionStyle{Position: "middle", FontSize: 40}},
		"karaoke-top-24":  {Style: models.CaptionStyle{Preset: CaptionKaraoke, Position: "top", FontSize: 24}},
		"minimal-mid-72":  {Style: models.CaptionStyle{Preset: CaptionMinimal, Position: "middle", FontSize: 72}},
		"animated-mid":    {Animated: &models.AnimatedCaptions{Enabled: true, Font: "sans", FontSize: 24, Color: "#FFFFFF", Highlight: "#FFD60A", Position: "middle", Outline: 6, Background: "dark"}},
		"animated-top-48": {Animated: &models.AnimatedCaptions{Enabled: true, Font: "condensed", FontSize: 48, Color: "#FFFFFF", Highlight: "#FFD60A", Position: "top", Outline: 4, Background: "none"}},
	}
	var cases []hookFitCase
	n := 0
	for capName, caps := range capChoices {
		capsFile, capsStyle, capsFilter := "", "", ""
		if !caps.None {
			words := captionFitWords()
			var doc, ext string
			if caps.Animated != nil {
				doc, ext = BuildAnimatedASS(words, 0, *caps.Animated), ".ass"
			} else {
				doc, ext = Captions(words, 0, caps.Style, nil)
				if ext == ".srt" {
					capsStyle = ResolveForceStyle(caps.Style)
				}
			}
			capsFile = "caps-" + capName + ext
			if err := os.WriteFile(filepath.Join(dir, capsFile), []byte(doc), 0o644); err != nil {
				t.Fatal(err)
			}
			if caps.Animated != nil {
				capsFilter, _ = captionBurn(capsFile, models.CaptionStyle{}, true)
			} else {
				capsFilter = SubtitlesFilter(capsFile, caps.Style)
			}
		}
		cb, _ := captionBand(caps)
		for lang, text := range hookFitTexts {
			for _, style := range hookStyles {
				for _, pos := range hookPositions {
					n++
					h := models.ClipHook{Enabled: true, Text: text, Style: style, Position: pos, Seconds: 2.5}
					p, err := planHook(h, caps)
					if err != nil {
						t.Fatalf("%s %s %s %s: %v", capName, lang, style, pos, err)
					}
					doc, err := BuildHookASS(h, caps, 6)
					if err != nil {
						t.Fatal(err)
					}
					id := fmt.Sprintf("h%03d", n)
					file := id + "-hook.ass"
					if err := os.WriteFile(filepath.Join(dir, file), []byte(doc), 0o644); err != nil {
						t.Fatal(err)
					}
					words := strings.Fields(text)
					lines := []string{strings.Join(words, " ")}
					if p.breakAt > 0 {
						lines = []string{strings.Join(words[:p.breakAt], " "), strings.Join(words[p.breakAt:], " ")}
					}
					measured := int(math.Ceil(float64(p.sizePx) * hookBoldWidth))
					est := 0.0
					for _, l := range lines {
						est = math.Max(est, textWidth(fontDefault, l, measured))
					}
					top := p.top - float64(p.look.pad)
					cases = append(cases, hookFitCase{
						ID: id, Lang: lang, Text: text, Style: style, Position: pos, Captions: capName,
						CapsFile: capsFile, CapsStyl: capsStyle, HookFile: file, CapsFilter: capsFilter, HookFilt: HookFilter(file), Seconds: 2.5, FadeMs: hookFadeMs,
						Fade:    capName == "none" && pos == models.HookTop,
						BlockY0: top, BlockY1: top + hookBlockHeight(p.sizePx, p.lines, p.look),
						EstW: est + 2*float64(p.look.pad) + float64(p.look.shadow), Lines: p.lines, SizePx: p.sizePx,
						CapsY0: cb.y0, CapsY1: cb.y1,
					})
				}
			}
		}
	}
	b, _ := json.MarshalIndent(cases, "", " ")
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
}
