package storyplan

import (
	"fmt"
	"math"
	"strings"
	"unicode"
	"unicode/utf8"
)

// minScriptShare is the fraction of a text's letters that must be in the
// story's script for it to count as written in the story's language. Half
// leaves room for English product names and proper nouns inside Hindi text,
// and still catches a narration that came back in the wrong language.
const minScriptShare = 0.5

// devanagariLanguages are the language codes that write in Devanagari.
var devanagariLanguages = map[string]bool{
	"hi": true, "mr": true, "ne": true, "sa": true, "mai": true, "bho": true, "kok": true, "doi": true,
}

// validate turns the model's raw answer into a Plan that satisfies every
// invariant in the package documentation, or returns ErrInvalidPlan.
//
// Repairs are made only where they cannot change the meaning: a duration that
// does not fit its narration is replaced by the estimate, a missing hook is
// taken from the first scene, a media type defaults. A scene that cannot be
// fixed (no narration, narration in the wrong language, no usable English
// keyword) is dropped and recorded in Warnings. The plan fails when too few
// scenes survive or the total length is out of bounds. story is the script of
// the user's story.
func validate(rp rawPlan, story Script) (Plan, error) {
	var warnings []string
	warn := func(format string, args ...any) { warnings = append(warnings, fmt.Sprintf(format, args...)) }

	scenes := make([]Scene, 0, len(rp.Scenes))
	for i, raw := range rp.Scenes {
		n := i + 1
		rs, ok := parseRawScene(raw)
		if !ok {
			warn("scene %d dropped: not a scene object", n)
			continue
		}
		sc, reason := cleanScene(rs, story, warn, n)
		if reason != "" {
			warn("scene %d dropped: %s", n, reason)
			continue
		}
		scenes = append(scenes, sc)
	}

	if len(scenes) > MaxScenes {
		warn("kept the first %d of %d scenes", MaxScenes, len(scenes))
		scenes = scenes[:MaxScenes]
	}

	// Over the total length: drop from the end. The opening and the middle of
	// a story carry more than its tail, and the plan can be regenerated.
	total := sumDurations(scenes)
	for total > MaxTotalSeconds && len(scenes) > 0 {
		total -= scenes[len(scenes)-1].Duration
		scenes = scenes[:len(scenes)-1]
		warn("a scene was cut from the end to keep the video within %.0f seconds", MaxTotalSeconds)
	}
	total = sumDurations(scenes)

	switch {
	case len(scenes) < MinScenes:
		return Plan{}, invalidPlan("only %d usable scene(s) remained, at least %d are needed", len(scenes), MinScenes)
	case total < MinTotalSeconds:
		return Plan{}, invalidPlan("the plan is %.1f seconds long, under the %.0f second minimum", total, MinTotalSeconds)
	}

	hook := cleanHook(rp.Hook, story)
	if hook == "" {
		hook = firstSentence(scenes[0].Narration, MaxHookWords)
		warn("the hook was missing or not in the story's language, so it was taken from the first scene")
	}

	lang, langWarn := cleanLanguage(rp.Language, story)
	if langWarn != "" {
		warn("%s", langWarn)
	}

	return Plan{
		Language:     lang,
		Hook:         hook,
		Scenes:       scenes,
		TotalSeconds: round1(total),
		Warnings:     warnings,
	}, nil
}

// cleanScene validates and repairs one scene. A non-empty reason means the
// scene cannot be used.
func cleanScene(rs rawScene, story Script, warn func(string, ...any), n int) (Scene, string) {
	narration := cleanText(rs.Narration, MaxNarrationRunes)
	if narration == "" {
		return Scene{}, "no narration"
	}
	if story != ScriptNone && story != ScriptOther {
		if ShareInScript(narration, story) < minScriptShare {
			return Scene{}, "narration is not in the story's language"
		}
	}

	keywords := cleanKeywords(keywordStrings(rs.Keywords))
	if len(keywords) == 0 {
		return Scene{}, "no usable English search keyword"
	}

	description := cleanText(rs.Description, MaxDescriptionRunes)
	if description == "" || HasIndianScript(description) {
		// The description is English by contract. One written in Hindi, or
		// left empty, is replaced by the keywords, which are known good.
		description = strings.Join(keywords, ", ")
		warn("scene %d: the description was missing or not English, so the keywords were used", n)
	}

	speech := estimateSpeech(narration)
	if speech > MaxSceneSeconds*1.2 {
		return Scene{}, "narration is too long to say in one scene"
	}
	duration, changed := sceneDuration(rs.Duration, speech)
	if changed {
		warn("scene %d: the duration was corrected to fit the narration", n)
	}

	return Scene{
		Narration:   narration,
		Description: description,
		Keywords:    keywords,
		MediaType:   cleanMedia(rs.MediaType),
		Duration:    duration,
	}, ""
}

// estimateSpeech is the time to say text at WordsPerSecond. Text with very few
// space-separated words (a script written without spaces) is estimated from its
// length instead.
func estimateSpeech(text string) float64 {
	words := wordCount(text)
	if words < 3 {
		runes := utf8.RuneCountInString(text)
		return float64(runes) / (WordsPerSecond * 5)
	}
	return float64(words) / WordsPerSecond
}

// sceneDuration chooses a scene's duration. The model's own number is kept
// when it is close to the narration's estimated length; otherwise the estimate
// is used. The result is always within [MinSceneSeconds, MaxSceneSeconds].
// changed reports that the model's number was replaced.
func sceneDuration(model any, speech float64) (float64, bool) {
	est := clamp(speech+scenePad, MinSceneSeconds, MaxSceneSeconds)
	d, ok := durationValue(model)
	if !ok {
		return round1(est), true
	}
	if math.Abs(d-est) > durationTolerance*est {
		return round1(est), true
	}
	return round1(clamp(d, MinSceneSeconds, MaxSceneSeconds)), false
}

func clamp(v, lo, hi float64) float64 { return math.Min(math.Max(v, lo), hi) }

func round1(v float64) float64 { return math.Round(v*10) / 10 }

func sumDurations(scenes []Scene) float64 {
	var t float64
	for _, s := range scenes {
		t += s.Duration
	}
	return t
}

// cleanKeywords keeps only English search terms: no Indian-script letter (the
// providers return nothing for them), at least one Latin letter, no markup.
// Duplicates are removed ignoring case, and at most MaxKeywords are kept in the
// order given.
func cleanKeywords(in []string) []string {
	out := make([]string, 0, MaxKeywords)
	seen := make(map[string]bool, MaxKeywords)
	for _, k := range in {
		k = cleanText(k, MaxKeywordRunes*3)
		k = strings.TrimLeft(k, "#-*•\"'`“‘ ")
		k = strings.TrimRight(k, "\"'`”’,.;: ")
		if HasIndianScript(k) {
			continue
		}
		if !hasLatinLetter(k) {
			continue
		}
		k = truncateAtWord(k, MaxKeywordRunes)
		key := strings.ToLower(k)
		if k == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, k)
		if len(out) == MaxKeywords {
			break
		}
	}
	return out
}

func hasLatinLetter(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) && unicode.Is(unicode.Latin, r) {
			return true
		}
	}
	return false
}

// truncateAtWord cuts s to at most max characters, at a word boundary when one
// exists in the second half of the allowance.
func truncateAtWord(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	runes := []rune(s)[:max]
	cut := string(runes)
	if i := strings.LastIndex(cut, " "); i > max/2 {
		cut = cut[:i]
	}
	return strings.TrimSpace(cut)
}

// cleanMedia maps the model's media type to photo or video. Anything it does
// not recognise gets the default, which is cosmetic: the phase that fetches
// media can fall back from one type to the other.
func cleanMedia(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "photo", "image", "picture", "still":
		return MediaPhoto
	case "video", "clip", "footage", "motion":
		return MediaVideo
	}
	return defaultMedia
}

// cleanHook returns the hook if it is usable, or "" if it should be replaced:
// empty, or not in the story's script. A hook over MaxHookWords is cut to its
// first sentence.
func cleanHook(h string, story Script) string {
	h = cleanText(h, MaxHookRunes)
	if h == "" {
		return ""
	}
	if story != ScriptNone && story != ScriptOther && ShareInScript(h, story) < minScriptShare {
		return ""
	}
	if wordCount(h) > MaxHookWords {
		h = firstSentence(h, MaxHookWords)
	}
	return h
}

// cleanLanguage returns a language code for the plan. The model's own code is
// kept when it is well-formed and agrees with the story's script. A Devanagari
// story is never labelled with a language that does not write in Devanagari,
// because a later phase picks the narration voice from this code.
func cleanLanguage(code string, story Script) (string, string) {
	code = strings.ToLower(strings.TrimSpace(code))
	if i := strings.IndexAny(code, "-_"); i > 0 {
		code = code[:i] // "hi-IN" -> "hi"
	}
	wellFormed := len(code) >= 2 && len(code) <= MaxLanguageRunes
	for _, r := range code {
		if r < 'a' || r > 'z' {
			wellFormed = false
		}
	}
	fallback := languageForScript(story)
	switch {
	case !wellFormed:
		if fallback == "" {
			return "", ""
		}
		return fallback, ""
	case story == ScriptDevanagari && !devanagariLanguages[code]:
		return "hi", fmt.Sprintf("the language was set to hi because the story is in Devanagari script (the model said %s)", code)
	}
	return code, ""
}
