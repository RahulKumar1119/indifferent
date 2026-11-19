package render

// The inputs and the output summary of the legacy-compatibility check (see legacy_compat_test.go).
// This file uses only functions that existed before caption fitting (#327), so the same file can be
// dropped into the tree before that change to produce the golden file.

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/rand"
	"strings"

	"github.com/rahul/indifferent/backend/internal/models"
)

type legacyInput struct {
	ID       string
	Words    []models.Word
	Style    models.CaptionStyle
	Animated *models.AnimatedCaptions
}

var legacyWordPool = strings.Fields(`the a to of and I you we it is in on at for with my your
quick brown fox jumps over lazy dog today video caption people world story friend happy morning
internationalization extraordinarily counterrevolutionaries uncharacteristically misunderstanding
Hello there MAKE THIS NOW 2026 1234567890 45,000 don't it's well-known
मैंने दादाजी के पुराने पीतल दीपक को कैसे ठीक किया क्षत्रिय ज्ञान श्रद्धा उज्ज्वल राष्ट्रीय
नमस्कार दोस्तों आज मैं आपको दिखाऊँगा सफलता कड़ी मेहनत धैर्य ज़रूरी`)

// legacyInputs is a fixed set of inputs: the same on every run, in every tree.
func legacyInputs() []legacyInput {
	rng := rand.New(rand.NewSource(327))
	presets := []string{"", "minimal", "bold", "karaoke", "clean"}
	sizes := []int{0, 8, 12, 14, 16, 18, 20, 24, 30, 36, 48, 60, 72}
	colors := []string{"", "#FFFFFF", "#FF0000", "#00FF88"}
	positions := []string{"", "top", "middle", "bottom"}
	var out []legacyInput
	for i := 0; i < 600; i++ {
		n := 1 + rng.Intn(12)
		var words []models.Word
		t := rng.Float64()
		for k := 0; k < n; k++ {
			d := 0.15 + rng.Float64()*0.5
			words = append(words, models.Word{Text: legacyWordPool[rng.Intn(len(legacyWordPool))], Start: t, End: t + d})
			t += d + rng.Float64()*0.3
		}
		in := legacyInput{
			ID:    fmt.Sprintf("in%04d", i),
			Words: words,
			Style: models.CaptionStyle{
				Preset: presets[rng.Intn(len(presets))], FontSize: sizes[rng.Intn(len(sizes))],
				Color: colors[rng.Intn(len(colors))], Highlight: colors[rng.Intn(len(colors))], Position: positions[rng.Intn(len(positions))],
			},
		}
		if i%6 == 5 {
			fonts := []string{"sans", "serif", "condensed"}
			in.Animated = &models.AnimatedCaptions{
				Enabled: true, Intensity: rng.Intn(101), Font: fonts[rng.Intn(3)], FontSize: 8 + rng.Intn(65),
				Color: "#FFFFFF", Highlight: "#FFD60A", Position: positions[1+rng.Intn(3)], Outline: rng.Intn(13), Background: []string{"none", "dark", "light"}[rng.Intn(3)],
			}
		}
		out = append(out, in)
	}
	return out
}

// legacyOutput is a hash of everything a caption style produces for the words: the caption file,
// its extension, the force_style and the burn filter.
func legacyOutput(in legacyInput) string {
	var b strings.Builder
	if in.Animated != nil {
		b.WriteString(BuildAnimatedASS(in.Words, 0, *in.Animated))
	} else {
		doc, ext := BuildCaptions(in.Words, in.Style, nil)
		b.WriteString(doc + "\x00" + ext + "\x00" + ResolveForceStyle(in.Style) + "\x00" + SubtitlesFilter("captions"+ext, in.Style))
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}
