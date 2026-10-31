package storyvideo

import (
	"os"
	"strings"
	"testing"

	"github.com/rahul/indifferent/backend/internal/models"
)

func read(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func texts(ws []models.Word) []string {
	var out []string
	for _, w := range ws {
		out = append(out, w.Text)
	}
	return out
}

// Real recordings from Amazon Polly (see testdata/README.txt).
func TestWordsFromMarks_RealEnglish(t *testing.T) {
	words, err := WordsFromMarks(string(read(t, "en.txt")), read(t, "en.marks.jsonl"), 1.880833)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(texts(words), "|")
	// The full stop that Polly's bare "cliff" mark lacks comes back from the text.
	if got != "The|lighthouse|stood|alone|on|the|cliff." {
		t.Errorf("words = %s", got)
	}
	if !approx(words[0].Start, 0.006) || !approx(words[1].Start, 0.064) {
		t.Errorf("starts = %v, %v; want 0.006, 0.064 from the marks", words[0].Start, words[1].Start)
	}
	if !approx(words[len(words)-1].End, 1.880833) {
		t.Errorf("the last word ends at %v, want the end of the audio", words[len(words)-1].End)
	}
}

func TestWordsFromMarks_RealHindi_KeepsTheDanda(t *testing.T) {
	words, err := WordsFromMarks(string(read(t, "hi.txt")), read(t, "hi.marks.jsonl"), 2.6645)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(texts(words), "|")
	if got != "लाइटहाउस|चट्टान|पर|अकेला|खड़ा|था।" {
		t.Errorf("words = %s", got)
	}
	// Byte offsets, not character offsets: every cut must be valid UTF-8.
	for _, w := range words {
		if strings.ContainsRune(w.Text, '\uFFFD') {
			t.Errorf("cut inside a character: %q", w.Text)
		}
	}
}

func TestWordsFromMarks_TimesAreOrderedAndFillTheAudio(t *testing.T) {
	for _, tc := range []struct {
		txt, marks string
		secs       float64
	}{{"en.txt", "en.marks.jsonl", 1.880833}, {"hi.txt", "hi.marks.jsonl", 2.6645}} {
		words, err := WordsFromMarks(string(read(t, tc.txt)), read(t, tc.marks), tc.secs)
		if err != nil {
			t.Fatal(err)
		}
		for i, w := range words {
			if w.End < w.Start {
				t.Errorf("%s word %d ends before it starts: %+v", tc.txt, i, w)
			}
			if i > 0 && !approx(w.Start, words[i-1].End) {
				t.Errorf("%s word %d starts at %v but the one before ended at %v: a gap or overlap", tc.txt, i, w.Start, words[i-1].End)
			}
		}
	}
}

func TestWordsFromMarks_OpeningQuoteGoesWithItsOwnWord(t *testing.T) {
	text := `He said “Hello there” softly.`
	// Polly's offsets are bytes: “ is 3 bytes.
	marks := strings.Join([]string{
		`{"time":10,"type":"word","start":0,"end":2,"value":"He"}`,
		`{"time":300,"type":"word","start":3,"end":7,"value":"said"}`,
		`{"time":600,"type":"word","start":11,"end":16,"value":"Hello"}`,
		`{"time":900,"type":"word","start":17,"end":22,"value":"there"}`,
		`{"time":1300,"type":"word","start":26,"end":32,"value":"softly"}`,
	}, "\n")
	words, err := WordsFromMarks(text, []byte(marks), 1.8)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(texts(words), "|"); got != "He|said|“Hello|there”|softly." {
		t.Errorf("words = %s", got)
	}
}

func TestWordsFromMarks_RepairsBadMarksInsteadOfRefusing(t *testing.T) {
	text := "one two three"
	t.Run("times that go backwards", func(t *testing.T) {
		marks := `{"time":500,"type":"word","start":0,"end":3,"value":"one"}
{"time":200,"type":"word","start":4,"end":7,"value":"two"}
{"time":900,"type":"word","start":8,"end":13,"value":"three"}`
		words, err := WordsFromMarks(text, []byte(marks), 2)
		if err != nil {
			t.Fatal(err)
		}
		for i, w := range words {
			if w.End < w.Start || (i > 0 && w.Start < words[i-1].Start) {
				t.Errorf("not in order: %+v", words)
			}
		}
	})
	t.Run("offsets outside the text fall back to the word Polly reported", func(t *testing.T) {
		marks := `{"time":0,"type":"word","start":500,"end":503,"value":"one"}
{"time":400,"type":"word","start":600,"end":603,"value":"two"}`
		words, err := WordsFromMarks(text, []byte(marks), 1)
		if err != nil {
			t.Fatal(err)
		}
		if got := strings.Join(texts(words), "|"); got != "one|two" {
			t.Errorf("words = %s", got)
		}
	})
	t.Run("a time past the end of the audio is held at the end", func(t *testing.T) {
		marks := `{"time":0,"type":"word","start":0,"end":3,"value":"one"}
{"time":9000,"type":"word","start":4,"end":7,"value":"two"}`
		words, _ := WordsFromMarks(text, []byte(marks), 1)
		if words[1].Start > 1 || words[1].End > 1 {
			t.Errorf("a word runs past the audio: %+v", words[1])
		}
	})
	t.Run("sentence marks and other types are ignored", func(t *testing.T) {
		marks := `{"time":0,"type":"sentence","start":0,"end":13,"value":"one two three"}
{"time":0,"type":"word","start":0,"end":3,"value":"one"}`
		words, err := WordsFromMarks(text, []byte(marks), 1)
		if err != nil || len(words) != 1 {
			t.Errorf("words = %+v err = %v", words, err)
		}
	})
}

func TestWordsFromMarks_ErrorsWhenThereIsNothingToUse(t *testing.T) {
	for name, tc := range map[string]struct {
		marks string
		secs  float64
	}{
		"empty":         {"", 1},
		"not json":      {"not json at all", 1},
		"no word marks": {`{"time":0,"type":"sentence","start":0,"end":3,"value":"one"}`, 1},
		"no audio":      {`{"time":0,"type":"word","start":0,"end":3,"value":"one"}`, 0},
	} {
		if _, err := WordsFromMarks("one", []byte(tc.marks), tc.secs); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}

func TestEvenWords_SpreadsTextAcrossTheAudio(t *testing.T) {
	words := EvenWords("a longer word and short ones", 6)
	if len(words) != 6 {
		t.Fatalf("got %d words", len(words))
	}
	if words[0].Start != 0 || !approx(words[5].End, 6) {
		t.Errorf("does not span the audio: %+v .. %+v", words[0], words[5])
	}
	for i := 1; i < len(words); i++ {
		if !approx(words[i].Start, words[i-1].End) {
			t.Errorf("gap or overlap at %d", i)
		}
	}
	if words[1].End-words[1].Start <= words[2].End-words[2].Start {
		t.Error("a longer word should be given more time than a short one")
	}
	if EvenWords("", 3) != nil || EvenWords("hello", 0) != nil {
		t.Error("nothing to spread should give nothing")
	}
}

func TestPlaceWords_MovesEachSceneToItsPlaceAndMarksTheBreaks(t *testing.T) {
	tl, _ := BuildTimeline([]float64{2, 3})
	scenes := []SceneWords{
		{Words: []models.Word{{Text: "a", Start: 0, End: 1}, {Text: "b", Start: 1, End: 2}}, AudioSeconds: 2},
		{Words: []models.Word{{Text: "c", Start: 0, End: 1.5}, {Text: "d", Start: 1.5, End: 3}}, AudioSeconds: 3},
	}
	got, err := PlaceWords(scenes, tl)
	if err != nil {
		t.Fatal(err)
	}
	second := tl.Slots[1].Start
	if got.Words[2].Text != "c" || !approx(got.Words[2].Start, second) || !approx(got.Words[3].End, second+3) {
		t.Errorf("scene 2 words are not on scene 2's part of the timeline: %+v", got.Words)
	}
	want := []bool{true, false, true, false}
	for i, w := range want {
		if got.BreakBefore[i] != w {
			t.Errorf("BreakBefore = %v, want %v", got.BreakBefore, want)
			break
		}
	}
}

func TestPlaceWords_NeverLetsACaptionOutliveItsScene(t *testing.T) {
	tl, _ := BuildTimeline([]float64{2, 2})
	scenes := []SceneWords{
		{Words: []models.Word{{Text: "late", Start: 1, End: 99}, {Text: "gone", Start: 500, End: 600}}},
		{Words: []models.Word{{Text: "next", Start: 0, End: 1}}},
	}
	got, _ := PlaceWords(scenes, tl)
	limit := tl.Slots[0].Start + tl.Slots[0].Span
	for _, w := range got.Words {
		if w.Text == "next" {
			continue
		}
		if w.End > limit+1e-9 {
			t.Errorf("%q runs to %v, past scene 1's end at %v", w.Text, w.End, limit)
		}
		if w.Text == "gone" {
			t.Error("a word that starts after the scene is over was kept")
		}
	}
}

func TestPlaceWords_ChecksTheSceneCount(t *testing.T) {
	tl, _ := BuildTimeline([]float64{2, 2})
	if _, err := PlaceWords([]SceneWords{{}}, tl); err == nil {
		t.Error("mismatched counts were accepted")
	}
}
