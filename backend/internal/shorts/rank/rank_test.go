package rank

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/rahul/indifferent/backend/internal/models"
)

// mockBedrockClient returns a canned response body regardless of input.
type mockBedrockClient struct {
	body []byte
	err  error
	// capture the last request for assertions.
	lastInput *bedrockruntime.InvokeModelInput
}

func (m *mockBedrockClient) InvokeModel(ctx context.Context, in *bedrockruntime.InvokeModelInput, optFns ...func(*bedrockruntime.Options)) (*bedrockruntime.InvokeModelOutput, error) {
	m.lastInput = in
	if m.err != nil {
		return nil, m.err
	}
	return &bedrockruntime.InvokeModelOutput{Body: m.body}, nil
}

// segmentsJSON is the raw model text both adapters must produce.
const segmentsJSON = `{"segments":[{"start":10,"end":40,"score":0.9,"hookText":"a"},{"start":100,"end":130,"score":0.7,"hookText":"b"}]}`

func novaBody(t *testing.T, text string) []byte {
	t.Helper()
	var r novaResponse
	r.Output.Message.Content = []struct {
		Text string `json:"text"`
	}{{Text: text}}
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal nova body: %v", err)
	}
	return b
}

func claudeBody(t *testing.T, text string) []byte {
	t.Helper()
	var r claudeResponse
	r.Content = []struct {
		Text string `json:"text"`
	}{{Text: text}}
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal claude body: %v", err)
	}
	return b
}

func kimiBody(t *testing.T, text string) []byte {
	t.Helper()
	var r kimiResponse
	r.Choices = make([]struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	}, 1)
	r.Choices[0].Message.Content = text
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal kimi body: %v", err)
	}
	return b
}

func sampleTranscript() models.Transcript {
	return models.Transcript{
		Text: "hello world",
		Words: []models.Word{
			{Text: "hello", Start: 12, End: 12.5},
			{Text: "world", Start: 35, End: 35.5},
			{Text: "later", Start: 200, End: 200.5},
		},
	}
}

func TestRank_NovaAndClaudeParseToSameSegments(t *testing.T) {
	nova := NewService(&mockBedrockClient{body: novaBody(t, segmentsJSON)}, ModelNovaPro)
	claude := NewService(&mockBedrockClient{body: claudeBody(t, segmentsJSON)}, ModelClaude)

	tr := sampleTranscript()

	novaSegs, err := nova.Rank(context.Background(), tr, 600)
	if err != nil {
		t.Fatalf("nova rank: %v", err)
	}
	claudeSegs, err := claude.Rank(context.Background(), tr, 600)
	if err != nil {
		t.Fatalf("claude rank: %v", err)
	}

	if len(novaSegs) != 2 || len(claudeSegs) != 2 {
		t.Fatalf("expected 2 segments each, got nova=%d claude=%d", len(novaSegs), len(claudeSegs))
	}
	for i := range novaSegs {
		if novaSegs[i].Start != claudeSegs[i].Start ||
			novaSegs[i].End != claudeSegs[i].End ||
			novaSegs[i].Score != claudeSegs[i].Score ||
			novaSegs[i].Rank != claudeSegs[i].Rank {
			t.Errorf("segment %d differs: nova=%+v claude=%+v", i, novaSegs[i], claudeSegs[i])
		}
	}
	// Ranking: highest score first.
	if novaSegs[0].Score != 0.9 || novaSegs[0].Rank != 1 {
		t.Errorf("expected top segment score 0.9 rank 1, got %+v", novaSegs[0])
	}
}

func TestRank_ModelSelectionHonorsConfig(t *testing.T) {
	// Nova selection produces a Nova request body (has inferenceConfig).
	novaClient := &mockBedrockClient{body: novaBody(t, segmentsJSON)}
	nova := NewService(novaClient, ModelFromString("nova"))
	if _, err := nova.Rank(context.Background(), sampleTranscript(), 600); err != nil {
		t.Fatalf("nova rank: %v", err)
	}
	if *novaClient.lastInput.ModelId != string(ModelNovaPro) {
		t.Errorf("expected model %s, got %s", ModelNovaPro, *novaClient.lastInput.ModelId)
	}
	var novaReq map[string]any
	if err := json.Unmarshal(novaClient.lastInput.Body, &novaReq); err != nil {
		t.Fatalf("unmarshal nova request: %v", err)
	}
	if _, ok := novaReq["inferenceConfig"]; !ok {
		t.Errorf("nova request missing inferenceConfig: %v", novaReq)
	}

	// Claude selection produces an Anthropic request body (has anthropic_version).
	claudeClient := &mockBedrockClient{body: claudeBody(t, segmentsJSON)}
	claude := NewService(claudeClient, ModelFromString("claude"))
	if _, err := claude.Rank(context.Background(), sampleTranscript(), 600); err != nil {
		t.Fatalf("claude rank: %v", err)
	}
	if *claudeClient.lastInput.ModelId != string(ModelClaude) {
		t.Errorf("expected model %s, got %s", ModelClaude, *claudeClient.lastInput.ModelId)
	}
	var claudeReq map[string]any
	if err := json.Unmarshal(claudeClient.lastInput.Body, &claudeReq); err != nil {
		t.Fatalf("unmarshal claude request: %v", err)
	}
	if _, ok := claudeReq["anthropic_version"]; !ok {
		t.Errorf("claude request missing anthropic_version: %v", claudeReq)
	}
}

func TestModelFromString(t *testing.T) {
	cases := map[string]ModelID{
		"claude": ModelClaude,
		"Claude": ModelClaude,
		"CLAUDE": ModelClaude,
		"nova":   ModelNovaPro,
		"kimi":   ModelKimi,
		"Kimi":   ModelKimi,
		"":       ModelKimi, // Kimi is now the default.
		"other":  ModelKimi, // unrecognized falls back to the default.
	}
	for in, want := range cases {
		if got := ModelFromString(in); got != want {
			t.Errorf("ModelFromString(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestRank_KimiParsesSegments(t *testing.T) {
	kimi := NewService(&mockBedrockClient{body: kimiBody(t, segmentsJSON)}, ModelKimi)

	segs, err := kimi.Rank(context.Background(), sampleTranscript(), 600)
	if err != nil {
		t.Fatalf("kimi rank: %v", err)
	}
	if len(segs) != 2 {
		t.Fatalf("expected 2 segments, got %d", len(segs))
	}
	// Ranking: highest score first.
	if segs[0].Score != 0.9 || segs[0].Rank != 1 {
		t.Errorf("expected top segment score 0.9 rank 1, got %+v", segs[0])
	}
	if segs[1].Score != 0.7 || segs[1].Rank != 2 {
		t.Errorf("expected second segment score 0.7 rank 2, got %+v", segs[1])
	}
}

func TestRank_KimiIsDefaultAndProducesOpenAIBody(t *testing.T) {
	// Empty config and "kimi" both select Kimi.
	if got := ModelFromString(""); got != ModelKimi {
		t.Errorf(`ModelFromString("") = %v, want %v`, got, ModelKimi)
	}
	if got := ModelFromString("kimi"); got != ModelKimi {
		t.Errorf(`ModelFromString("kimi") = %v, want %v`, got, ModelKimi)
	}
	// "nova"/"claude" remain selectable.
	if got := ModelFromString("nova"); got != ModelNovaPro {
		t.Errorf(`ModelFromString("nova") = %v, want %v`, got, ModelNovaPro)
	}
	if got := ModelFromString("claude"); got != ModelClaude {
		t.Errorf(`ModelFromString("claude") = %v, want %v`, got, ModelClaude)
	}

	// Kimi selection produces an OpenAI-format request body.
	kimiClient := &mockBedrockClient{body: kimiBody(t, segmentsJSON)}
	kimi := NewService(kimiClient, ModelFromString(""))
	if _, err := kimi.Rank(context.Background(), sampleTranscript(), 600); err != nil {
		t.Fatalf("kimi rank: %v", err)
	}
	if *kimiClient.lastInput.ModelId != string(ModelKimi) {
		t.Errorf("expected model %s, got %s", ModelKimi, *kimiClient.lastInput.ModelId)
	}
	var kimiReq map[string]any
	if err := json.Unmarshal(kimiClient.lastInput.Body, &kimiReq); err != nil {
		t.Fatalf("unmarshal kimi request: %v", err)
	}
	if _, ok := kimiReq["max_tokens"]; !ok {
		t.Errorf("kimi request missing max_tokens: %v", kimiReq)
	}
	if _, ok := kimiReq["messages"]; !ok {
		t.Errorf("kimi request missing messages: %v", kimiReq)
	}
}

func TestRank_MalformedJSONTreatedAsZeroSegments(t *testing.T) {
	cases := []string{
		"this is not json at all",
		`{"segments": [`,        // truncated
		"",                       // empty text
		"here you go: {oops}",   // junk braces
	}
	for _, text := range cases {
		svc := NewService(&mockBedrockClient{body: novaBody(t, text)}, ModelNovaPro)
		_, err := svc.Rank(context.Background(), sampleTranscript(), 600)
		if !errors.Is(err, ErrNoValidSegments) {
			t.Errorf("input %q: expected ErrNoValidSegments, got %v", text, err)
		}
	}
}

func TestRank_ExtractsJSONFromMarkdownFence(t *testing.T) {
	fenced := "```json\n" + segmentsJSON + "\n```"
	svc := NewService(&mockBedrockClient{body: novaBody(t, fenced)}, ModelNovaPro)
	segs, err := svc.Rank(context.Background(), sampleTranscript(), 600)
	if err != nil {
		t.Fatalf("rank with fenced json: %v", err)
	}
	if len(segs) != 2 {
		t.Fatalf("expected 2 segments, got %d", len(segs))
	}
}

func TestRank_CaptionWordsPopulatedFromTranscript(t *testing.T) {
	svc := NewService(&mockBedrockClient{body: novaBody(t, segmentsJSON)}, ModelNovaPro)
	segs, err := svc.Rank(context.Background(), sampleTranscript(), 600)
	if err != nil {
		t.Fatalf("rank: %v", err)
	}
	// Top segment [10,40] should include "hello" (12) and "world" (35), not "later" (200).
	top := segs[0]
	if len(top.CaptionWords) != 2 {
		t.Fatalf("expected 2 caption words in [10,40], got %d: %+v", len(top.CaptionWords), top.CaptionWords)
	}
	if top.CaptionWords[0].Text != "hello" || top.CaptionWords[1].Text != "world" {
		t.Errorf("unexpected caption words: %+v", top.CaptionWords)
	}
	// Second segment [100,130] should include no words.
	if len(segs[1].CaptionWords) != 0 {
		t.Errorf("expected 0 caption words in [100,130], got %+v", segs[1].CaptionWords)
	}
}
