package rank

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
)

// Complete lets other pipelines reuse the model adapters. It must send the
// same per-model request shapes Rank does, honour the caller's token budget,
// and return the model's raw text.
func TestComplete_SendsEachModelsShapeWithTheCallersBudget(t *testing.T) {
	for _, tc := range []struct {
		name  string
		model ModelID
		body  func(*testing.T, string) []byte
		check func(*testing.T, []byte)
	}{
		{"kimi", ModelKimi, kimiBody, func(t *testing.T, b []byte) {
			var r kimiRequest
			mustUnmarshal(t, b, &r)
			if r.MaxTokens != 1234 || r.Temperature != 0.7 || len(r.Messages) != 1 || r.Messages[0].Content != "the prompt" {
				t.Errorf("kimi request = %+v", r)
			}
		}},
		{"nova", ModelNovaPro, novaBody, func(t *testing.T, b []byte) {
			var r novaRequest
			mustUnmarshal(t, b, &r)
			if r.InferenceConfig.MaxTokens != 1234 || r.InferenceConfig.Temperature != 0.7 || r.Messages[0].Content[0].Text != "the prompt" {
				t.Errorf("nova request = %+v", r)
			}
		}},
		{"claude", ModelClaude, claudeBody, func(t *testing.T, b []byte) {
			var r claudeRequest
			mustUnmarshal(t, b, &r)
			if r.MaxTokens != 1234 || r.Temperature != 0.7 || r.Messages[0].Content != "the prompt" || r.AnthropicVersion == "" {
				t.Errorf("claude request = %+v", r)
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := &mockBedrockClient{body: tc.body(t, "raw model text")}
			got, err := NewService(client, tc.model).Complete(context.Background(), "the prompt", CompleteOptions{MaxTokens: 1234, Temperature: 0.7})
			if err != nil || got != "raw model text" {
				t.Fatalf("got %q, err %v", got, err)
			}
			if id := *client.lastInput.ModelId; id != string(tc.model) {
				t.Errorf("model id = %q, want %q", id, tc.model)
			}
			tc.check(t, client.lastInput.Body)
		})
	}
}

func TestComplete_ZeroOptionsUseRanksDefaults(t *testing.T) {
	for model, wantTokens := range map[ModelID]int{ModelKimi: 8000, ModelNovaPro: 2000, ModelClaude: 2000} {
		client := &mockBedrockClient{body: kimiBody(t, "x")}
		switch model {
		case ModelNovaPro:
			client.body = novaBody(t, "x")
		case ModelClaude:
			client.body = claudeBody(t, "x")
		}
		if _, err := NewService(client, model).Complete(context.Background(), "p", CompleteOptions{}); err != nil {
			t.Fatal(err)
		}
		var tokens int
		var temp float64
		switch model {
		case ModelKimi:
			var r kimiRequest
			mustUnmarshal(t, client.lastInput.Body, &r)
			tokens, temp = r.MaxTokens, r.Temperature
		case ModelNovaPro:
			var r novaRequest
			mustUnmarshal(t, client.lastInput.Body, &r)
			tokens, temp = r.InferenceConfig.MaxTokens, r.InferenceConfig.Temperature
		default:
			var r claudeRequest
			mustUnmarshal(t, client.lastInput.Body, &r)
			tokens, temp = r.MaxTokens, r.Temperature
		}
		if tokens != wantTokens || temp != 0.2 {
			t.Errorf("%s: tokens=%d temperature=%v, want %d and 0.2", model, tokens, temp, wantTokens)
		}
	}
}

// Rank itself must send exactly what it always has: the refactor behind
// Complete must not change it.
func TestRank_RequestIsUnchangedByTheRefactor(t *testing.T) {
	client := &mockBedrockClient{body: kimiBody(t, segmentsJSON)}
	if _, err := NewService(client, ModelKimi).Rank(context.Background(), sampleTranscript(), 600); err != nil {
		t.Fatal(err)
	}
	var r kimiRequest
	mustUnmarshal(t, client.lastInput.Body, &r)
	if r.MaxTokens != 8000 || r.Temperature != 0.2 {
		t.Errorf("Rank now sends max_tokens=%d temperature=%v, want 8000 and 0.2", r.MaxTokens, r.Temperature)
	}
}

func TestComplete_Errors(t *testing.T) {
	t.Run("invoke failure", func(t *testing.T) {
		boom := errors.New("throttled")
		_, err := NewService(&mockBedrockClient{err: boom}, ModelKimi).Complete(context.Background(), "p", CompleteOptions{})
		if !errors.Is(err, boom) || !strings.Contains(err.Error(), "InvokeModel") {
			t.Errorf("err = %v", err)
		}
	})
	t.Run("undecodable body", func(t *testing.T) {
		_, err := NewService(&mockBedrockClient{body: []byte("not json")}, ModelKimi).Complete(context.Background(), "p", CompleteOptions{})
		if err == nil {
			t.Error("expected an error")
		}
	})
	t.Run("empty content is empty text, not an error", func(t *testing.T) {
		got, err := NewService(&mockBedrockClient{body: []byte(`{"choices":[]}`)}, ModelKimi).Complete(context.Background(), "p", CompleteOptions{})
		if err != nil || got != "" {
			t.Errorf("got %q, err %v", got, err)
		}
	})
	t.Run("the context reaches the client", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		client := &ctxClient{}
		_, _ = NewService(client, ModelKimi).Complete(ctx, "p", CompleteOptions{})
		if !client.sawCancelled {
			t.Error("Complete did not pass its context to InvokeModel")
		}
	})
}

func TestExtractJSON_Exported(t *testing.T) {
	for in, want := range map[string]string{
		"```json\n{\"a\":1}\n```":  `{"a":1}`,
		`prose {"a":{"b":2}} more`: `{"a":{"b":2}}`,
		"no json here":             "",
	} {
		if got := ExtractJSON(in); got != want {
			t.Errorf("ExtractJSON(%q) = %q, want %q", in, got, want)
		}
	}
}

func mustUnmarshal(t *testing.T, b []byte, v any) {
	t.Helper()
	if err := json.Unmarshal(b, v); err != nil {
		t.Fatalf("unmarshal request body: %v\n%s", err, b)
	}
}

// ctxClient records whether the context it received was already cancelled.
type ctxClient struct{ sawCancelled bool }

func (c *ctxClient) InvokeModel(ctx context.Context, in *bedrockruntime.InvokeModelInput, _ ...func(*bedrockruntime.Options)) (*bedrockruntime.InvokeModelOutput, error) {
	c.sawCancelled = ctx.Err() != nil
	return nil, ctx.Err()
}
