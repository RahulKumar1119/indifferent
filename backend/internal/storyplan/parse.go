package storyplan

import (
	"encoding/json"
	"math"
	"strconv"
	"strings"
)

// rawPlan and rawScene hold the model's answer before any validation. Fields
// that models often get slightly wrong (a keyword list written as one
// comma-separated string, a duration written as "5") are read as any and
// normalised by the helpers below, so one loose field does not discard a scene.
type rawPlan struct {
	Language string            `json:"language"`
	Hook     string            `json:"hook"`
	Scenes   []json.RawMessage `json:"scenes"`
}

type rawScene struct {
	Narration   string `json:"narration"`
	Description string `json:"description"`
	Keywords    any    `json:"keywords"`
	MediaType   string `json:"mediaType"`
	Duration    any    `json:"duration"`
}

// maxRawScenes bounds how many scenes are examined. The model is asked for at
// most MaxScenes; reading a few more lets validation drop invalid ones without
// letting a runaway answer cost unbounded work.
const maxRawScenes = MaxScenes * 3

// parseRawPlan extracts the plan object from the model's text. It tolerates
// markdown fences and prose around the JSON (the shared extractor handles
// both) and returns ErrInvalidPlan when there is no usable object.
func parseRawPlan(text string, extract func(string) string) (rawPlan, error) {
	js := extract(text)
	if js == "" {
		return rawPlan{}, invalidPlan("the answer contains no JSON object")
	}
	var rp rawPlan
	if err := json.Unmarshal([]byte(js), &rp); err != nil {
		return rawPlan{}, invalidPlan("the answer is not a plan in the expected shape")
	}
	if len(rp.Scenes) == 0 {
		return rawPlan{}, invalidPlan("the answer has no scenes")
	}
	if len(rp.Scenes) > maxRawScenes {
		rp.Scenes = rp.Scenes[:maxRawScenes]
	}
	return rp, nil
}

// parseRawScene decodes one scene. A scene that is not an object is reported
// as not ok and skipped by the caller.
func parseRawScene(raw json.RawMessage) (rawScene, bool) {
	var rs rawScene
	if err := json.Unmarshal(raw, &rs); err != nil {
		return rawScene{}, false
	}
	return rs, true
}

// durationValue reads a duration given as a JSON number or a numeric string. It
// returns false for anything else, for NaN or infinity, and for a value that
// is not positive.
func durationValue(v any) (float64, bool) {
	var f float64
	switch t := v.(type) {
	case float64:
		f = t
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(t), "s")), 64)
		if err != nil {
			return 0, false
		}
		f = parsed
	default:
		return 0, false
	}
	if math.IsNaN(f) || math.IsInf(f, 0) || f <= 0 {
		return 0, false
	}
	return f, true
}

// keywordStrings reads a keyword list that is a JSON array of strings or one
// string separated by commas, semicolons or newlines. Non-string entries are
// ignored.
func keywordStrings(v any) []string {
	var out []string
	switch t := v.(type) {
	case []any:
		for _, e := range t {
			if s, ok := e.(string); ok {
				out = append(out, s)
			}
		}
	case string:
		out = strings.FieldsFunc(t, func(r rune) bool { return r == ',' || r == ';' || r == '\n' })
	}
	return out
}
