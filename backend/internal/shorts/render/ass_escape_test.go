package render

import (
	"strings"
	"testing"

	"github.com/rahul/indifferent/backend/internal/models"
)

func TestAssEscape_LineBreaksAndControlCharactersBecomeSpaces(t *testing.T) {
	for in, want := range map[string]string{
		"a\nb":         "a b",
		"a\r\nb":       "a  b",
		"a\rb":         "a b",
		"a\tb":         "a b",
		"a\x00b":       "a b",
		"a\x1bb":       "a b",
		"a\u2028b":     "a b",
		"a\u2029b":     "a b",
		"plain":        "plain",
		`back\slash`:   `back\\slash`,
		"{override}":   `\{override\}`,
		"\u0928\u092e": "\u0928\u092e",
	} {
		if got := assEscape(in); got != want {
			t.Errorf("assEscape(%q) = %q, want %q", in, got, want)
		}
	}
}

// The reason it matters: a caption word with a line break must not be able to
// add a line to the subtitle file.
func TestBuildASS_ACaptionWordCannotAddALineToTheFile(t *testing.T) {
	words := []models.Word{
		{Text: "hello", Start: 0, End: 0.5},
		{Text: "x\nDialogue: 0,0:00:00.00,0:00:09.00,Default,,0,0,0,,INJECTED\n[Script Info]", Start: 0.5, End: 1.0},
	}
	out := BuildASS(words, 0, models.CaptionStyle{})
	// The text still appears, inside the one event; what must not happen is a
	// second event or section, which is a line that starts with the keyword.
	starting := func(prefix string) int {
		n := 0
		for _, line := range strings.Split(out, "\n") {
			if strings.HasPrefix(line, prefix) {
				n++
			}
		}
		return n
	}
	if n := starting("Dialogue:"); n != 1 {
		t.Errorf("%d Dialogue lines, want 1 (a caption word added one):\n%s", n, out)
	}
	if n := starting("[Script Info]"); n != 1 {
		t.Errorf("%d [Script Info] headers, want 1:\n%s", n, out)
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "INJECTED") && !strings.HasPrefix(line, "Dialogue:") {
			t.Errorf("injected text starts its own line: %q", line)
		}
	}
}
