package render

import (
	"strings"
	"testing"

	"github.com/rahul/indifferent/backend/internal/models"
)

func TestCaptionValidate(t *testing.T) {
	if err := (models.CaptionStyle{}).Validate(); err != nil {
		t.Errorf("zero style must validate, got: %v", err)
	}
	valid := models.CaptionStyle{Preset: "karaoke", FontSize: 24, Color: "#FFD60A", Highlight: "#9AA0A6", Position: "middle"}
	if err := valid.Validate(); err != nil {
		t.Errorf("valid style rejected: %v", err)
	}
	for _, bad := range []models.CaptionStyle{
		{Preset: "comic"},
		{FontSize: 200},
		{Color: "red"},
		{Highlight: "#FFF"},
		{Position: "left"},
	} {
		if err := bad.Validate(); err == nil {
			t.Errorf("invalid style accepted: %+v", bad)
		}
	}
}

func TestResolveForceStyle_LegacyExact(t *testing.T) {
	// The zero style must burn byte-identical to the historical filter so
	// existing jobs render pixel-identical output.
	got := ResolveForceStyle(models.CaptionStyle{})
	want := "Alignment=2,FontSize=18,PrimaryColour=&H00FFFFFF,BorderStyle=3,Outline=2"
	if got != want {
		t.Errorf("legacy force_style = %q, want %q", got, want)
	}
}

func TestResolveForceStyle_Presets(t *testing.T) {
	cases := []struct {
		style models.CaptionStyle
		want  []string
	}{
		{models.CaptionStyle{Preset: "minimal"}, []string{"BorderStyle=1", "MarginV=45"}},
		{models.CaptionStyle{Preset: "clean"}, []string{"BorderStyle=1", "MarginV=50"}},
		{models.CaptionStyle{Preset: "bold", Position: "top"}, []string{"Alignment=8"}},
		{models.CaptionStyle{Preset: "bold", Position: "middle", FontSize: 24, Color: "#FF0000"}, []string{"Alignment=5", "FontSize=24", "&H000000FF"}},
	}
	for _, tc := range cases {
		got := ResolveForceStyle(tc.style)
		for _, want := range tc.want {
			if !strings.Contains(got, want) {
				t.Errorf("preset %+v: force_style %q lacks %q", tc.style, got, want)
			}
		}
	}
	if !isKaraokeStyle(models.CaptionStyle{Preset: "karaoke"}) {
		t.Error("karaoke preset must select the ASS path")
	}
	if isKaraokeStyle(models.CaptionStyle{Preset: "bold"}) {
		t.Error("bold preset must not select the ASS path")
	}
}

func TestBuildASS_TimingAndSweep(t *testing.T) {
	words := []models.Word{
		{Text: "alpha", Start: 0.2, End: 0.9},
		{Text: "beta", Start: 2.2, End: 2.9},
	}
	ass := BuildASS(words, 0.0, models.CaptionStyle{Preset: "karaoke"})
	if !strings.Contains(ass, "PlayResX: 1080") || !strings.Contains(ass, "PlayResY: 1920") {
		t.Error("ASS header must declare full-HD play resolution")
	}
	if !strings.Contains(ass, `{\kf70}alpha`) {
		t.Error("expected 70cs sweep tag on first word")
	}
	if !strings.Contains(ass, `{\kf70}beta`) {
		t.Error("expected 70cs sweep tag on second word")
	}
	if !strings.Contains(ass, "0:00:00.20,0:00:00.90") {
		t.Error("expected clip-relative cue timing")
	}
	// Same cue grouping as SRT: 5 rapid words stay in limited cues.
	many := []models.Word{
		{Text: "a", Start: 0.0, End: 0.2}, {Text: "b", Start: 0.2, End: 0.4},
		{Text: "c", Start: 0.4, End: 0.6}, {Text: "d", Start: 0.6, End: 0.8},
		{Text: "e", Start: 0.8, End: 1.0},
	}
	if n := strings.Count(BuildASS(many, 0, models.CaptionStyle{Preset: "karaoke"}), "Dialogue:"); n != 2 {
		t.Errorf("expected 2 cues (4+1 words), got %d", n)
	}
}

func TestBuildASS_EscapesBraces(t *testing.T) {
	words := []models.Word{{Text: "a{b}c", Start: 0.0, End: 0.5}}
	ass := BuildASS(words, 0, models.CaptionStyle{Preset: "karaoke"})
	if !strings.Contains(ass, `a\{b\}c`) {
		t.Error("caption braces must be escaped so libass keeps override blocks intact")
	}
	if strings.Contains(ass, "{b}") {
		t.Error("unescaped braces would open an override block")
	}
}

func TestBuildCaptions_PicksTheFormatTheShortsRendererDoes(t *testing.T) {
	words := []models.Word{{Text: "one", Start: 0, End: 0.4}, {Text: "two", Start: 0.4, End: 0.8}}
	for _, preset := range []string{"", "bold", "minimal", "clean"} {
		doc, ext := BuildCaptions(words, models.CaptionStyle{Preset: preset}, nil)
		if ext != ".srt" || doc != BuildSRT(words, 0) {
			t.Errorf("preset %q: ext %q, want .srt with BuildSRT's output", preset, ext)
		}
	}
	k := models.CaptionStyle{Preset: "karaoke"}
	doc, ext := BuildCaptions(words, k, nil)
	if ext != ".ass" || doc != BuildASS(words, 0, k) {
		t.Errorf("karaoke: ext %q, want .ass with BuildASS's output", ext)
	}
}

func TestSubtitlesFilter_MatchesWhatTheShortsRendererBurns(t *testing.T) {
	if got := SubtitlesFilter("/w/c.ass", models.CaptionStyle{Preset: "karaoke"}); got != "subtitles=/w/c.ass" {
		t.Errorf("karaoke filter = %q", got)
	}
	style := models.CaptionStyle{Preset: "minimal", Position: "top"}
	want := "subtitles=/w/c.srt:force_style='" + ResolveForceStyle(style) + "'"
	if got := SubtitlesFilter("/w/c.srt", style); got != want {
		t.Errorf("filter = %q, want %q", got, want)
	}
	if !IsKaraoke(models.CaptionStyle{Preset: "karaoke"}) || IsKaraoke(models.CaptionStyle{}) {
		t.Error("IsKaraoke is wrong")
	}
}
