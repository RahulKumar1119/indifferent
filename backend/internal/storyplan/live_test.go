//go:build live

package storyplan

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/rahul/indifferent/backend/internal/shorts/rank"
)

// Live checks against real Bedrock. They are excluded from normal builds and
// from CI (they need AWS credentials and spend money):
//
//	PLAN_LIVE_RUNS=3 go test -tags live -run Live -v -timeout 30m ./internal/storyplan/
//
// They answer two questions the unit tests cannot: how long does planning take
// (which decides whether the API can answer synchronously inside the gateway's
// 29 second limit), and does the model really answer a Hindi story in Hindi
// with English keywords.

type liveStory struct {
	label string
	text  string
}

var liveStories = []liveStory{
	{"en-short", "A fisherman in Kerala wakes before dawn, rows out into the backwaters, and finds that the net he cast last night has caught a tiny glowing lantern instead of fish."},
	{"en-medium", `Meera had run the tea stall outside the Pune railway station for twenty years. Every morning at five she lit the stove, and every morning the same stray dog sat by her feet.
One monsoon night the station flooded. Meera stayed to serve tea to stranded travellers, free of charge, until the water went down. The next morning a stranger left an envelope with enough money to rebuild the stall with a proper roof.
She never learned his name. She pinned the empty envelope above the stove, and the dog still sits by her feet.`},
	{"en-long", strings.Repeat(`The old lighthouse at the edge of the town had been dark for forty years, and nobody remembered why. The children said a ghost lived in it. The fishermen said the lamp had simply worn out. Anya, who had just moved from the city, decided to climb it one windy afternoon. At the top she found a logbook, and in the logbook a name she knew: her grandfather's. He had been the last keeper. She spent the evening reading his notes about storms, ships and a promise he made to keep the light burning for a boat that never returned. `, 4)},
	{"hi-short", "केरल का एक मछुआरा भोर से पहले उठता है, पीछे के पानी में नाव खेता है, और देखता है कि कल रात डाला उसका जाल मछलियों की जगह एक छोटी चमकती लालटेन पकड़ लाया है।"},
	{"hi-medium", `मीरा बीस साल से पुणे रेलवे स्टेशन के बाहर चाय की दुकान चला रही थी। हर सुबह पाँच बजे वह चूल्हा जलाती, और हर सुबह वही आवारा कुत्ता उसके पैरों के पास बैठा मिलता।
एक बरसात की रात स्टेशन में पानी भर गया। मीरा फँसे हुए यात्रियों को मुफ़्त चाय पिलाती रही, जब तक पानी उतर नहीं गया। अगली सुबह एक अजनबी एक लिफ़ाफ़ा छोड़ गया, जिसमें दुकान पर पक्की छत बनवाने भर के पैसे थे।
वह उसका नाम कभी नहीं जान पाई। उसने खाली लिफ़ाफ़ा चूल्हे के ऊपर टाँग दिया, और कुत्ता आज भी उसके पैरों के पास बैठता है।`},
	{"hi-long", strings.Repeat(`कस्बे के किनारे का पुराना प्रकाश-स्तंभ चालीस साल से बंद था, और किसी को याद नहीं था क्यों। बच्चे कहते थे कि उसमें भूत रहता है। मछुआरे कहते थे कि दीया बस घिस गया है। अन्या, जो अभी-अभी शहर से आई थी, एक हवादार दोपहर उस पर चढ़ने का फ़ैसला करती है। ऊपर उसे एक लॉगबुक मिली, और उसमें एक नाम जिसे वह जानती थी: उसके दादाजी का। वे आख़िरी रखवाले थे। उसने शाम उनके तूफ़ानों, जहाज़ों और उस वादे के बारे में लिखे नोट पढ़ते हुए बिताई जो उन्होंने एक कभी न लौटी नाव के लिए बत्ती जलाए रखने का किया था। `, 4)},
	{"hinglish", "Rohan ne apni pehli salary se maa ke liye ek chhoti si silai machine kharidi. Maa ne kaha ki ab woh pure mohalle ke kapde silengi, aur ek saal baad unki dukaan ka naam board par chamak raha tha."},
}

func liveRuns(t *testing.T) int {
	if v := os.Getenv("PLAN_LIVE_RUNS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			t.Fatalf("PLAN_LIVE_RUNS=%q", v)
		}
		return n
	}
	return 1
}

func liveClient(t *testing.T) *bedrockruntime.Client {
	t.Helper()
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		t.Skipf("no AWS configuration: %v", err)
	}
	if cfg.Region == "" {
		t.Skip("no AWS region configured")
	}
	return bedrockruntime.NewFromConfig(cfg)
}

type sample struct {
	model, story, outcome string
	latency               time.Duration
	scenes                int
	total                 float64
	warnings              int
}

func percentile(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	i := int(float64(len(sorted)-1) * p)
	return sorted[i]
}

// TestLive_LatencyAndQuality measures planning latency and validity for each
// model across English and Hindi stories. It prints one plan per model and
// language so the Hindi narration and English keywords can be read by eye.
func TestLive_LatencyAndQuality(t *testing.T) {
	client := liveClient(t)
	runs := liveRuns(t)
	models := []string{"kimi", "nova"}
	if v := os.Getenv("PLAN_LIVE_MODELS"); v != "" {
		models = strings.Split(v, ",")
	}

	var all []sample
	printed := map[string]bool{}
	for _, name := range models {
		model := rank.ModelFromString(name)
		// A long timeout: the point is to see how long it really takes. The
		// production timeout would hide the slow calls as failures.
		svc := NewService(rank.NewService(client, model), string(model), WithTimeout(120*time.Second), WithLogger(func(string, ...any) {}))
		for r := 0; r < runs; r++ {
			for _, st := range liveStories {
				res, err := svc.Generate(context.Background(), st.text)
				s := sample{model: name, story: st.label, latency: res.Latency}
				switch {
				case err == nil:
					s.outcome, s.scenes, s.total, s.warnings = "ok", len(res.Plan.Scenes), res.Plan.TotalSeconds, len(res.Plan.Warnings)
				case errors.Is(err, ErrInvalidPlan):
					s.outcome = "invalid_plan: " + err.Error()
				case errors.Is(err, ErrModelTimeout):
					s.outcome = "timeout"
				default:
					s.outcome = "error: " + err.Error()
				}
				all = append(all, s)
				t.Logf("%-5s %-9s run%d  %6.1fs  %-8s scenes=%d total=%.0fs warnings=%d", name, st.label, r+1, s.latency.Seconds(), firstWord(s.outcome), s.scenes, s.total, s.warnings)

				key := name + "/" + st.label
				if err == nil && !printed[key] && (st.label == "hi-medium" || st.label == "en-medium" || st.label == "hinglish") {
					printed[key] = true
					t.Logf("---- %s plan (language=%s, total=%.1fs)\n%s", key, res.Plan.Language, res.Plan.TotalSeconds, describePlan(res.Plan))
				}
			}
		}
	}

	t.Log("\n==== SUMMARY ====")
	for _, name := range models {
		var lat []time.Duration
		ok, bad := 0, 0
		for _, s := range all {
			if s.model != name {
				continue
			}
			if s.outcome == "ok" {
				ok++
				lat = append(lat, s.latency)
			} else {
				bad++
			}
		}
		sort.Slice(lat, func(i, j int) bool { return lat[i] < lat[j] })
		t.Logf("%-5s ok=%d failed=%d  median=%.1fs p90=%.1fs max=%.1fs  (over 25s: %d of %d)",
			name, ok, bad, percentile(lat, .5).Seconds(), percentile(lat, .9).Seconds(), percentile(lat, 1).Seconds(), countOver(lat, 25*time.Second), len(lat))
		for _, lang := range []string{"en", "hi", "hinglish"} {
			var l []time.Duration
			for _, s := range all {
				if s.model == name && s.outcome == "ok" && strings.HasPrefix(s.story, lang) {
					l = append(l, s.latency)
				}
			}
			sort.Slice(l, func(i, j int) bool { return l[i] < l[j] })
			if len(l) > 0 {
				t.Logf("        %-8s n=%d median=%.1fs p90=%.1fs max=%.1fs", lang, len(l), percentile(l, .5).Seconds(), percentile(l, .9).Seconds(), percentile(l, 1).Seconds())
			}
		}
	}
}

func countOver(l []time.Duration, d time.Duration) int {
	n := 0
	for _, x := range l {
		if x > d {
			n++
		}
	}
	return n
}

func firstWord(s string) string {
	if i := strings.IndexAny(s, ": "); i > 0 {
		return s[:i]
	}
	return s
}

func describePlan(p Plan) string {
	var b strings.Builder
	fmt.Fprintf(&b, "hook: %s\n", p.Hook)
	for i, s := range p.Scenes {
		fmt.Fprintf(&b, "  %d. [%s %.1fs] %s\n     show: %s\n     keywords: %v\n", i+1, s.MediaType, s.Duration, s.Narration, s.Description, s.Keywords)
	}
	if len(p.Warnings) > 0 {
		fmt.Fprintf(&b, "  warnings: %v\n", p.Warnings)
	}
	return b.String()
}

// TestLive_MaxLengthStories times the worst case the API accepts: a story at
// MaxStoryRunes. It is the case that decides whether the synchronous timeout
// has headroom. Devanagari costs several times more tokens per word than
// English, so Hindi is the slower of the two.
func TestLive_MaxLengthStories(t *testing.T) {
	client := liveClient(t)
	name := os.Getenv("PLAN_LIVE_MODEL")
	if name == "" {
		name = "kimi"
	}
	model := rank.ModelFromString(name)
	svc := NewService(rank.NewService(client, model), string(model), WithTimeout(120*time.Second), WithLogger(func(string, ...any) {}))

	// fill repeats base, then trims to a whole word at the limit, so the story is
	// as long as the API allows whatever the length of base.
	fill := func(base string) string {
		var b strings.Builder
		for len([]rune(b.String())) < MaxStoryRunes+200 {
			b.WriteString(strings.TrimSpace(base))
			b.WriteString(" ")
		}
		runes := []rune(b.String())[:MaxStoryRunes]
		s := string(runes)
		if i := strings.LastIndex(s, " "); i > MaxStoryRunes-200 {
			s = s[:i]
		}
		return strings.TrimSpace(s)
	}
	cases := map[string]string{
		"en-max": fill(liveStories[2].text),
		"hi-max": fill(liveStories[5].text),
	}
	runs := liveRuns(t)
	for label, story := range cases {
		if n := len([]rune(story)); n > MaxStoryRunes || n < MaxStoryRunes-400 {
			t.Fatalf("%s story has %d characters, want close to %d", label, n, MaxStoryRunes)
		}
		for r := 0; r < runs; r++ {
			res, err := svc.Generate(context.Background(), story)
			if err != nil {
				t.Errorf("%s %s run%d: %v", name, label, r+1, err)
				continue
			}
			t.Logf("%s %-6s run%d  %5.1fs  scenes=%d total=%.0fs warnings=%d", name, label, r+1, res.Latency.Seconds(), len(res.Plan.Scenes), res.Plan.TotalSeconds, len(res.Plan.Warnings))
		}
	}
}

// TestLive_HindiStoryKeepsHindiNarrationAndEnglishKeywords is the acceptance
// check for the language rules, against the real model: Hindi narration in
// Devanagari, English keywords with no Indian-script letter, English
// description.
func TestLive_HindiStoryKeepsHindiNarrationAndEnglishKeywords(t *testing.T) {
	client := liveClient(t)
	name := os.Getenv("PLAN_LIVE_MODEL")
	if name == "" {
		name = "kimi"
	}
	model := rank.ModelFromString(name)
	svc := NewService(rank.NewService(client, model), string(model), WithTimeout(120*time.Second), WithLogger(func(string, ...any) {}))

	res, err := svc.Generate(context.Background(), liveStories[4].text) // hi-medium
	if err != nil {
		t.Fatal(err)
	}
	p := res.Plan
	if p.Language != "hi" {
		t.Errorf("language = %q, want hi", p.Language)
	}
	if ShareInScript(p.Hook, ScriptDevanagari) < 0.5 {
		t.Errorf("hook is not Hindi: %q", p.Hook)
	}
	for i, s := range p.Scenes {
		if ShareInScript(s.Narration, ScriptDevanagari) < 0.5 {
			t.Errorf("scene %d narration is not Hindi: %q", i+1, s.Narration)
		}
		if HasIndianScript(strings.Join(s.Keywords, " ")) {
			t.Errorf("scene %d has a non-English keyword: %v", i+1, s.Keywords)
		}
		if HasIndianScript(s.Description) {
			t.Errorf("scene %d description is not English: %q", i+1, s.Description)
		}
	}
	t.Logf("model=%s latency=%.1fs\n%s", name, res.Latency.Seconds(), describePlan(p))
}
