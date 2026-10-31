package storyvideo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/rahul/indifferent/backend/internal/models"
)

// mark is one line of Amazon Polly word speech marks. Time is milliseconds from
// the start of the audio. Start and End are byte offsets into the text that was
// sent to Polly (not character offsets: a Hindi letter is three bytes).
type mark struct {
	Time  int    `json:"time"`
	Type  string `json:"type"`
	Start int    `json:"start"`
	End   int    `json:"end"`
	Value string `json:"value"`
}

// WordsFromMarks turns Polly's word speech marks into caption words, in seconds
// from the start of the scene's audio. text must be exactly what was sent to
// Polly, and audioSeconds the length of the audio it returned.
//
// Polly reports bare words ("cliff"), without the punctuation around them, so
// each caption word is cut from text instead: from where the word starts to
// where the next one starts, which keeps "cliff." and "mandir," as written.
// A word's end is the start of the next word, so a caption stays on screen
// through a pause inside a sentence; the last word ends with the audio.
//
// Marks that cannot be trusted (bad offsets, times that go backwards) are
// repaired word by word rather than refused: a caption with slightly wrong
// punctuation is better than no video. Marks with no words at all are an
// error, and the caller can fall back to EvenWords.
func WordsFromMarks(text string, marksJSON []byte, audioSeconds float64) ([]models.Word, error) {
	var marks []mark
	for _, line := range bytes.Split(marksJSON, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		var m mark
		if err := json.Unmarshal(line, &m); err != nil {
			return nil, fmt.Errorf("speech marks are not readable: %w", err)
		}
		if m.Type == "word" {
			marks = append(marks, m)
		}
	}
	if len(marks) == 0 {
		return nil, fmt.Errorf("speech marks hold no words")
	}
	if audioSeconds <= 0 || math.IsNaN(audioSeconds) {
		return nil, fmt.Errorf("the audio has no length")
	}

	tokens := make([]string, len(marks))
	for i, m := range marks {
		tokens[i] = tokenFor(text, marks, i, m)
	}
	moveOpeningPunctuation(tokens)

	words := make([]models.Word, 0, len(marks))
	prev := 0.0
	for i, m := range marks {
		if tokens[i] == "" {
			continue
		}
		start := float64(m.Time) / 1000
		if start < prev {
			start = prev // times never go backwards
		}
		if start > audioSeconds {
			start = audioSeconds
		}
		end := audioSeconds
		for j := i + 1; j < len(marks); j++ {
			if tokens[j] != "" {
				end = float64(marks[j].Time) / 1000
				break
			}
		}
		if end > audioSeconds {
			end = audioSeconds
		}
		if end < start {
			end = start
		}
		words = append(words, models.Word{Text: tokens[i], Start: start, End: end})
		prev = start
	}
	if len(words) == 0 {
		return nil, fmt.Errorf("speech marks hold no usable words")
	}
	return words, nil
}

// tokenFor cuts the caption text for mark i out of text: from the word's start
// to the next word's start, trimmed. It falls back to Polly's own value when
// the offsets do not fit the text.
func tokenFor(text string, marks []mark, i int, m mark) string {
	end := len(text)
	if i+1 < len(marks) {
		end = marks[i+1].Start
	}
	if m.Start >= 0 && m.Start < end && end <= len(text) &&
		utf8.RuneStart(text[m.Start]) && (end == len(text) || utf8.RuneStart(text[end])) {
		if tok := strings.TrimSpace(text[m.Start:end]); tok != "" {
			return tok
		}
	}
	return strings.TrimSpace(m.Value)
}

// moveOpeningPunctuation fixes the one thing cutting by "start of next word"
// gets wrong: an opening quote or bracket sits before the next word, so it is
// cut onto the end of the previous one ("said “" then "Hello"). It moves
// opening punctuation to the front of the word it belongs to.
func moveOpeningPunctuation(tokens []string) {
	for i := 0; i+1 < len(tokens); i++ {
		t := tokens[i]
		cut := len(t)
		for cut > 0 {
			r, size := utf8.DecodeLastRuneInString(t[:cut])
			if !(unicode.Is(unicode.Ps, r) || unicode.Is(unicode.Pi, r) || r == '¿' || r == '¡') {
				break
			}
			cut -= size
		}
		if cut < len(t) && cut > 0 { // never empty a word
			tokens[i+1] = t[cut:] + tokens[i+1]
			tokens[i] = strings.TrimSpace(t[:cut]) // the space before the quote stays behind
		}
	}
}

// EvenWords spreads text's words evenly across audioSeconds. It is the
// fallback for audio whose speech marks are missing or unreadable: the video
// still renders and the captions stay roughly in step, but a word may be a
// fraction of a second off. The caller should say so in its log.
func EvenWords(text string, audioSeconds float64) []models.Word {
	fields := strings.Fields(text)
	if len(fields) == 0 || audioSeconds <= 0 || math.IsNaN(audioSeconds) {
		return nil
	}
	// Longer words take longer to say: weight each word by its length.
	total := 0
	for _, f := range fields {
		total += utf8.RuneCountInString(f) + 1
	}
	words := make([]models.Word, len(fields))
	at := 0.0
	for i, f := range fields {
		d := audioSeconds * float64(utf8.RuneCountInString(f)+1) / float64(total)
		words[i] = models.Word{Text: f, Start: at, End: at + d}
		at += d
	}
	words[len(words)-1].End = audioSeconds
	return words
}

// SceneWords is one scene's caption words, in seconds from the start of that
// scene's audio, with the length of the audio they belong to.
type SceneWords struct {
	Words        []models.Word
	AudioSeconds float64
}

// Captioned is every caption word on the video's timeline, plus where each
// scene's words begin so no caption cue runs across a change of scene.
type Captioned struct {
	Words []models.Word
	// BreakBefore[i] is true when word i is the first word of a scene.
	BreakBefore []bool
}

// PlaceWords moves each scene's words to that scene's place on the timeline.
// Words are clamped to the scene's own span, so a late or oversized timing can
// never put a caption over the next scene's picture.
func PlaceWords(scenes []SceneWords, tl Timeline) (Captioned, error) {
	if len(scenes) != len(tl.Slots) {
		return Captioned{}, fmt.Errorf("%d scenes of words for a timeline of %d", len(scenes), len(tl.Slots))
	}
	var out Captioned
	for i, sw := range scenes {
		slot := tl.Slots[i]
		limit := slot.Start + slot.Span
		first := true
		for _, w := range sw.Words {
			start := slot.Start + w.Start
			end := slot.Start + w.End
			if start >= limit {
				continue
			}
			if end > limit {
				end = limit
			}
			if end < start {
				end = start
			}
			out.Words = append(out.Words, models.Word{Text: w.Text, Start: start, End: end})
			out.BreakBefore = append(out.BreakBefore, first)
			first = false
		}
	}
	return out, nil
}
