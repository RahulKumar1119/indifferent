package videocopy

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/rahul/indifferent/backend/internal/storyplan"
)

// Input limits, in characters.
const (
	MaxTopicRunes      = 300
	MaxTranscriptRunes = 20000
	MaxRefineRunes     = 200
	MaxPreviousTitles  = 10
	// MinSourceRunes rejects a source too thin to write about.
	MinSourceRunes = 15
)

// Word is one transcript word with its time in the video, in seconds.
type Word struct {
	Text  string
	Start float64
	End   float64
}

// Request is what the caller supplies. At least one of Topic, Transcript and
// Words is required.
type Request struct {
	// Topic says what the video is about, in a sentence or two.
	Topic string
	// Transcript is pasted text. Lines that begin with a time ("1:05 text") give
	// it timestamps; plain text has none.
	Transcript string
	// Words is the video's own timed transcript.
	Words []Word
	// Duration is the video's length in seconds when known. It decides whether
	// the video is long enough for chapters.
	Duration float64
	// Refine is a short instruction for adjusting the titles ("shorter").
	Refine string
	// Previous are the titles being adjusted.
	Previous []string
}

// segment is a stretch of the source with a real start time. Start is -1 when
// the source has no times.
type segment struct {
	Start float64
	Text  string
}

// source is a validated request, ready for the prompt.
type source struct {
	topic    string
	segments []segment
	timed    bool
	duration float64
	refine   string
	previous []string
	script   storyplan.Script
	runes    int
}

// Chapters need YouTube's minimum: three of them, the first at 0:00, each at
// least ten seconds long.
const (
	minChapters   = 3
	minChapterGap = 10.0
	maxChapters   = 25
	// minChaptersVideo is the shortest video that can hold three chapters of ten
	// seconds.
	minChaptersVideo = 30.0
)

const (
	blockSeconds = 60.0
	maxBlocks    = 120
	// promptBudget bounds the transcript part of the prompt, in characters.
	promptBudget = 30000
	maxBlockText = 300
)

var timestampLine = regexp.MustCompile(`^[\[(]?(?:(\d{1,2}):)?(\d{1,2}):(\d{2})[\])]?\s*[-–—:.)]?\s*(.*)$`)

func normalize(r Request) (*source, error) {
	topic := storyplan.CleanLine(r.Topic, MaxTopicRunes)
	if utf8.RuneCountInString(strings.TrimSpace(r.Topic)) > MaxTopicRunes*2 {
		return nil, invalidInput("the topic is too long: keep it under %d characters", MaxTopicRunes)
	}
	refine := storyplan.CleanLine(r.Refine, MaxRefineRunes)
	if utf8.RuneCountInString(strings.TrimSpace(r.Refine)) > MaxRefineRunes*2 {
		return nil, invalidInput("the adjustment is too long: keep it under %d characters", MaxRefineRunes)
	}
	if len(r.Previous) > MaxPreviousTitles {
		return nil, invalidInput("too many titles to adjust")
	}
	var previous []string
	for _, p := range r.Previous {
		if t := storyplan.CleanLine(p, MaxTitleRunes); t != "" {
			previous = append(previous, t)
		}
	}
	if utf8.RuneCountInString(r.Transcript) > MaxTranscriptRunes*3 {
		return nil, invalidInput("the transcript is too long: keep it under %d characters", MaxTranscriptRunes)
	}

	s := &source{topic: topic, refine: refine, previous: previous, duration: r.Duration}
	switch {
	case len(r.Words) > 0:
		s.segments, s.timed = blocksFromWords(r.Words, r.Duration)
		if s.timed && s.duration < s.segments[len(s.segments)-1].Start {
			s.duration = s.segments[len(s.segments)-1].Start + 1
		}
	case strings.TrimSpace(r.Transcript) != "":
		text := normalizeNewlines(r.Transcript)
		if utf8.RuneCountInString(text) > MaxTranscriptRunes {
			return nil, invalidInput("the transcript is too long: keep it under %d characters", MaxTranscriptRunes)
		}
		s.segments, s.timed = segmentsFromPasted(text)
		if s.timed && s.duration <= 0 {
			s.duration = s.segments[len(s.segments)-1].Start + blockSeconds
		}
	}

	var all strings.Builder
	all.WriteString(topic)
	for _, g := range s.segments {
		all.WriteString(" ")
		all.WriteString(g.Text)
	}
	s.runes = utf8.RuneCountInString(strings.TrimSpace(all.String()))
	if s.runes < MinSourceRunes {
		return nil, invalidInput("give a topic or a transcript of at least %d characters", MinSourceRunes)
	}
	s.script = storyplan.DominantScript(all.String())
	fitBudget(s.segments)
	return s, nil
}

func normalizeNewlines(s string) string {
	if !utf8.ValidString(s) {
		s = strings.ToValidUTF8(s, "")
	}
	return strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\r", "\n")
}

// blocksFromWords groups timed words into blocks of about a minute. Block 0
// always starts at 0, so a first chapter at 0:00 is always on offer.
func blocksFromWords(words []Word, duration float64) ([]segment, bool) {
	last := 0.0
	for _, w := range words {
		if w.End > last {
			last = w.End
		}
	}
	if duration < last {
		duration = last
	}
	size := blockSeconds
	if n := math.Ceil(duration / maxBlocks); n > size {
		size = n
	}
	var out []segment
	var b strings.Builder
	cur := -1
	flush := func() {
		if cur >= 0 {
			if t := storyplan.CleanLine(b.String(), maxBlockText); t != "" {
				out = append(out, segment{Start: float64(cur) * size, Text: t})
			}
		}
		b.Reset()
	}
	for _, w := range words {
		if w.Start < 0 || math.IsNaN(w.Start) || math.IsInf(w.Start, 0) {
			continue
		}
		idx := int(w.Start / size)
		if idx != cur {
			flush()
			cur = idx
		}
		if b.Len() < maxBlockText*4 {
			b.WriteString(w.Text)
			b.WriteByte(' ')
		}
	}
	flush()
	if len(out) == 0 {
		return nil, false
	}
	// The first block is offered as the video's start.
	out[0].Start = 0
	return out, true
}

// segmentsFromPasted reads "m:ss text" lines. With fewer than three timed lines
// the text is treated as untimed: it still works as material, but offers no chapters.
func segmentsFromPasted(text string) ([]segment, bool) {
	var timedSegs []segment
	var plain []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if m := timestampLine.FindStringSubmatch(line); m != nil {
			h, _ := strconv.Atoi(m[1])
			min, _ := strconv.Atoi(m[2])
			sec, _ := strconv.Atoi(m[3])
			if sec < 60 && (m[1] == "" || min < 60) {
				start := float64(h*3600 + min*60 + sec)
				body := storyplan.CleanLine(m[4], maxBlockText)
				if body != "" {
					if n := len(timedSegs); n > 0 && start <= timedSegs[n-1].Start {
						// Out of order or repeated times cannot be chapters.
						plain = append(plain, body)
						continue
					}
					timedSegs = append(timedSegs, segment{Start: start, Text: body})
					continue
				}
			}
		}
		plain = append(plain, storyplan.CleanLine(line, maxBlockText))
	}
	if len(timedSegs) >= minChapters {
		timedSegs[0].Start = 0
		return timedSegs, true
	}
	all := storyplan.CleanLine(strings.Join(plain, " ")+" "+joinSegs(timedSegs), MaxTranscriptRunes)
	if all == "" {
		return nil, false
	}
	return []segment{{Start: -1, Text: all}}, false
}

func joinSegs(s []segment) string {
	var parts []string
	for _, g := range s {
		parts = append(parts, g.Text)
	}
	return strings.Join(parts, " ")
}

// fitBudget shortens each block so the transcript part of the prompt stays
// within promptBudget characters, however long the video is.
func fitBudget(segs []segment) {
	if len(segs) == 0 {
		return
	}
	per := promptBudget / len(segs)
	if per < 60 {
		per = 60
	}
	if len(segs) == 1 {
		per = MaxTranscriptRunes
	}
	for i := range segs {
		if r := []rune(segs[i].Text); len(r) > per {
			segs[i].Text = strings.TrimSpace(string(r[:per]))
		}
	}
}

// FormatTime writes a chapter time as m:ss, or h:mm:ss when long is true (a
// video of an hour or more), so every time in one list reads the same.
func FormatTime(seconds float64, long bool) string {
	n := int(seconds)
	h, m, s := n/3600, n%3600/60, n%60
	if long {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", n/60, s)
}

// Check validates a request without calling a model. A caller that charges for
// each call uses it first, so a request that cannot succeed costs nothing.
func Check(r Request) error {
	_, err := normalize(r)
	return err
}
