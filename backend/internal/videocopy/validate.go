package videocopy

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/rahul/indifferent/backend/internal/storyplan"
)

// Chapter is one entry of the chapter list. Start is the real start of the
// transcript block it names, never a number from the model.
type Chapter struct {
	Start float64 `json:"start"`
	Time  string  `json:"time"`
	Title string  `json:"title"`
}

// Copy is the checked result.
type Copy struct {
	Titles      []string
	Description string
	Chapters    []Chapter
	// DescriptionWithChapters is Description followed by the chapter list, ready
	// to paste. Equal to Description when there are no chapters.
	DescriptionWithChapters string
	// Notes say, in words safe to show, what was left out and why.
	Notes []string
}

type rawCopy struct {
	Titles      []any `json:"titles"`
	Description any   `json:"description"`
	Chapters    []any `json:"chapters"`
}

// minScriptShare is how much of a text's letters must be in the material's
// script for it to count as written in the same language.
const minScriptShare = 0.6

var (
	urlPattern     = regexp.MustCompile(`(?i)\b(?:https?://|www\.)\S+`)
	hashtagPattern = regexp.MustCompile(`(^|\s)#\p{L}[\p{L}\p{N}_]*`)
	blankLines     = regexp.MustCompile(`\n{3,}`)
)

// parseAndCheck turns the model's answer into a Copy, or ErrInvalidAnswer.
func parseAndCheck(answer string, s *source, extract func(string) string) (Copy, error) {
	js := extract(answer)
	if js == "" {
		return Copy{}, ErrInvalidAnswer
	}
	var raw rawCopy
	if err := json.Unmarshal([]byte(js), &raw); err != nil {
		// Models often put a real line break inside the description string,
		// which JSON does not allow. Escape those and read it again.
		if err := json.Unmarshal([]byte(escapeControlInStrings(js)), &raw); err != nil {
			return Copy{}, ErrInvalidAnswer
		}
	}
	var c Copy
	c.Titles = checkTitles(raw.Titles, s)
	if len(c.Titles) < minTitlesToServe {
		return Copy{}, ErrInvalidAnswer
	}
	desc, _ := raw.Description.(string)
	c.Description = checkDescription(desc, s.script)
	if c.Description == "" {
		return Copy{}, ErrInvalidAnswer
	}
	if chaptersAllowed(s) {
		var note string
		c.Chapters, note = checkChapters(raw.Chapters, s)
		if note != "" {
			c.Notes = append(c.Notes, note)
		}
	} else if s.timed {
		c.Notes = append(c.Notes, "This video is too short for chapters.")
	} else {
		c.Notes = append(c.Notes, "There are no timestamps, so there are no chapters.")
	}
	c.DescriptionWithChapters = withChapters(c.Description, c.Chapters)
	return c, nil
}

// escapeControlInStrings escapes line breaks, tabs and other control characters
// that appear inside JSON strings, and leaves everything outside strings alone.
func escapeControlInStrings(js string) string {
	var b strings.Builder
	inStr, esc := false, false
	for _, r := range js {
		switch {
		case esc:
			esc = false
			b.WriteRune(r)
		case inStr && r == '\\':
			esc = true
			b.WriteRune(r)
		case r == '"':
			inStr = !inStr
			b.WriteRune(r)
		case inStr && r < 0x20:
			switch r {
			case '\n':
				b.WriteString(`\n`)
			case '\r':
				b.WriteString(`\r`)
			case '\t':
				b.WriteString(`\t`)
			default:
				fmt.Fprintf(&b, `\u%04x`, r)
			}
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func inScript(text string, sc storyplan.Script) bool {
	if sc == storyplan.ScriptNone || sc == storyplan.ScriptOther {
		return true
	}
	return storyplan.ShareInScript(text, sc) >= minScriptShare
}

func checkTitles(in []any, s *source) []string {
	seen := map[string]bool{}
	for _, p := range s.previous {
		seen[strings.ToLower(p)] = true
	}
	var out []string
	for _, v := range in {
		raw, _ := v.(string)
		t := strings.Trim(storyplan.CleanLine(raw, MaxTitleRunes*2), " \"'\u201c\u201d")
		if t == "" || utf8.RuneCountInString(t) > MaxTitleRunes {
			continue
		}
		t = strings.TrimSpace(hashtagPattern.ReplaceAllString(urlPattern.ReplaceAllString(t, ""), "$1"))
		if t == "" || !inScript(t, s.script) {
			continue
		}
		key := strings.ToLower(t)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, t)
		if len(out) == TitleCount {
			break
		}
	}
	return out
}

// checkDescription cleans a description and keeps its paragraphs. Links and
// hashtags are removed: nothing in the material can vouch for a link.
func checkDescription(raw string, sc storyplan.Script) string {
	if !utf8.ValidString(raw) {
		raw = strings.ToValidUTF8(raw, "")
	}
	raw = strings.ReplaceAll(strings.ReplaceAll(raw, "\r\n", "\n"), "\r", "\n")
	var paras []string
	for _, p := range strings.Split(raw, "\n\n") {
		var lines []string
		for _, l := range strings.Split(p, "\n") {
			l = urlPattern.ReplaceAllString(l, "")
			l = hashtagPattern.ReplaceAllString(l, "$1")
			if l = storyplan.CleanLine(l, bodyBudget); l != "" {
				lines = append(lines, l)
			}
		}
		if len(lines) > 0 {
			paras = append(paras, strings.Join(lines, "\n"))
		}
	}
	text := strings.Join(paras, "\n\n")
	text = blankLines.ReplaceAllString(text, "\n\n")
	if r := []rune(text); len(r) > bodyBudget {
		text = strings.TrimSpace(string(r[:bodyBudget]))
	}
	if strings.IndexFunc(text, unicode.IsLetter) < 0 || !inScript(text, sc) {
		return ""
	}
	return text
}

// checkChapters keeps the chapters that name a real block, in order, at least
// ten seconds apart, and returns them only if the list as a whole is one YouTube
// will show: the first at 0:00 and at least three. Otherwise there are none, and
// the note says why.
func checkChapters(in []any, s *source) ([]Chapter, string) {
	long := s.duration >= 3600
	var out []Chapter
	last := -1
	for _, v := range in {
		m, ok := v.(map[string]any)
		if !ok {
			continue
		}
		idx, ok := blockIndex(m["block"])
		if !ok || idx <= last || idx >= len(s.segments) {
			continue
		}
		title := storyplan.CleanLine(strAny(m["title"]), MaxChapterRunes*2)
		if title == "" || utf8.RuneCountInString(title) > MaxChapterRunes || !inScript(title, s.script) {
			continue
		}
		start := s.segments[idx].Start
		if n := len(out); n > 0 && start-out[n-1].Start < minChapterGap {
			continue
		}
		out = append(out, Chapter{Start: start, Time: FormatTime(start, long), Title: title})
		last = idx
		if len(out) == maxChapters {
			break
		}
	}
	if len(out) < minChapters || out[0].Start != 0 {
		return nil, "Chapters were left out because the suggestions did not fit YouTube's rules (at least three, starting at 0:00). Try again."
	}
	if last := out[len(out)-1].Start; s.duration >= 600 && last < s.duration*0.6 {
		return out, "The chapters only cover the first part of the video. Try again for a list that spreads over all of it."
	}
	return out, ""
}

func strAny(v any) string {
	s, _ := v.(string)
	return s
}

// blockIndex reads a block number the model may write as 3, 3.0 or "3".
func blockIndex(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		if n < 0 {
			return 0, false
		}
		return n, true
	case float64:
		if n < 0 || n != math.Trunc(n) || n > 1e6 {
			return 0, false
		}
		return int(n), true
	case string:
		i, err := strconv.Atoi(strings.TrimSpace(n))
		if err != nil || i < 0 {
			return 0, false
		}
		return i, true
	}
	return 0, false
}

func withChapters(desc string, chapters []Chapter) string {
	if len(chapters) == 0 {
		return desc
	}
	var b strings.Builder
	b.WriteString(desc)
	b.WriteString("\n\n")
	for i, c := range chapters {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(c.Time + " " + c.Title)
	}
	return b.String()
}
