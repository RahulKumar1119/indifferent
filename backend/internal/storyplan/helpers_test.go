package storyplan

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/rahul/indifferent/backend/internal/shorts/rank"
)

// stubModel is a Completer whose answer the test sets.
type stubModel struct {
	mu      sync.Mutex
	answer  string
	err     error
	delay   func(ctx context.Context) error // blocks until done or ctx ends
	prompts []string
	opts    []rank.CompleteOptions
}

func (m *stubModel) Complete(ctx context.Context, prompt string, opts rank.CompleteOptions) (string, error) {
	m.mu.Lock()
	m.prompts = append(m.prompts, prompt)
	m.opts = append(m.opts, opts)
	answer, err, delay := m.answer, m.err, m.delay
	m.mu.Unlock()
	if delay != nil {
		if derr := delay(ctx); derr != nil {
			return "", derr
		}
	}
	return answer, err
}

func (m *stubModel) calls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.prompts)
}

func (m *stubModel) lastPrompt() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.prompts[len(m.prompts)-1]
}

func quietService(m Completer) *Service {
	return NewService(m, "test-model", WithLogger(func(string, ...any) {}))
}

// A story long enough to pass NormalizeStory, in each script.
const (
	englishStory = "A fisherman in Kerala wakes before dawn, rows out into the backwaters, and finds a glowing lantern in his net."
	hindiStory   = "केरल का एक मछुआरा भोर से पहले उठता है, पीछे के पानी में नाव खेता है, और अपने जाल में एक चमकती लालटेन पाता है।"
)

type sceneJSON struct {
	Narration   string `json:"narration"`
	Description string `json:"description"`
	Keywords    any    `json:"keywords"`
	MediaType   string `json:"mediaType"`
	Duration    any    `json:"duration"`
}

type planJSON struct {
	Language string      `json:"language"`
	Hook     string      `json:"hook"`
	Scenes   []sceneJSON `json:"scenes"`
}

// tb is the part of testing.TB the helpers use, so they work with both
// *testing.T and *rapid.T (which does not implement all of testing.TB).
type tb interface {
	Helper()
	Fatalf(format string, args ...any)
}

func mustJSON(t tb, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("%v", err)
	}
	return string(b)
}

// words returns n distinct words, so a narration has a known speaking time.
func words(prefix string, n int) string {
	w := make([]string, n)
	for i := range w {
		w[i] = fmt.Sprintf("%s%c", prefix, 'a'+rune(i%26))
	}
	return strings.Join(w, " ")
}

// englishScene is a valid English scene of about 4.5 seconds.
func englishScene(i int) sceneJSON {
	return sceneJSON{
		Narration:   fmt.Sprintf("Scene %d: %s", i, "the fisherman rows quietly across the dark water at dawn"),
		Description: "A wooden boat on calm backwaters at sunrise",
		Keywords:    []string{"backwaters sunrise", "fishing boat", "Kerala"},
		MediaType:   "video",
		Duration:    5.0,
	}
}

func englishPlan(n int) planJSON {
	p := planJSON{Language: "en", Hook: "Before dawn, a net caught light."}
	for i := 1; i <= n; i++ {
		p.Scenes = append(p.Scenes, englishScene(i))
	}
	return p
}

// hindiScene is a valid Hindi scene: Hindi narration, English everything else.
func hindiScene(i int) sceneJSON {
	return sceneJSON{
		Narration:   fmt.Sprintf("दृश्य %d: मछुआरा भोर के अँधेरे में चुपचाप नाव खेता है", i),
		Description: "A wooden boat on calm backwaters at sunrise",
		Keywords:    []string{"backwaters sunrise", "fishing boat", "Kerala"},
		MediaType:   "video",
		Duration:    5.0,
	}
}

func hindiPlan(n int) planJSON {
	p := planJSON{Language: "hi", Hook: "भोर से पहले, एक जाल में रोशनी आई।"}
	for i := 1; i <= n; i++ {
		p.Scenes = append(p.Scenes, hindiScene(i))
	}
	return p
}

func extractForTest(s string) string { return rank.ExtractJSON(s) }

// parseAndValidate runs the model answer through parsing and validation, as Generate
// does, for tests that exercise only those two steps.
func parseAndValidate(text string, sc Script) (Plan, error) {
	rp, err := parseRawPlan(text, rank.ExtractJSON)
	if err != nil {
		return Plan{}, err
	}
	return validate(rp, sc)
}
