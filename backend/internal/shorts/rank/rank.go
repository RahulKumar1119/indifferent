package rank

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/rahul/indifferent/backend/internal/models"
	"github.com/rahul/indifferent/backend/internal/shorts"
)

// BedrockClient is the subset of the Bedrock Runtime API used by the ranking
// service. Defining it as an interface allows the service to be tested with a
// mock client (mirroring narrator.PollyClient).
type BedrockClient interface {
	InvokeModel(ctx context.Context, in *bedrockruntime.InvokeModelInput, optFns ...func(*bedrockruntime.Options)) (*bedrockruntime.InvokeModelOutput, error)
}

// ModelID identifies which Bedrock foundation model backs the ranking service.
type ModelID string

const (
	// ModelKimi is the Moonshot Kimi model on Bedrock (OpenAI-compatible format).
	// NOTE: verify this exact model ID in the AWS Bedrock console; it may carry
	// a version suffix.
	ModelKimi ModelID = "moonshotai.kimi-k3"
	// ModelNovaPro is a selectable ranking model.
	ModelNovaPro ModelID = "amazon.nova-pro-v1:0"
	// ModelClaude is selectable via deployment config (Requirement 3.8).
	ModelClaude ModelID = "anthropic.claude-3-5-sonnet-20240620-v1:0"
)

// Service ranks transcript moments using a Bedrock model. The model output is
// never trusted directly; every candidate segment is validated before use.
type Service struct {
	Client BedrockClient
	Model  ModelID
}

// NewService constructs a ranking Service with the given client and model.
func NewService(client BedrockClient, model ModelID) *Service {
	return &Service{Client: client, Model: model}
}

// ModelFromString maps a deployment-config value to a ModelID. Kimi is the
// default, selected when s is "kimi", empty, or unrecognized (case-insensitive).
// "nova" selects Nova Pro and "claude" selects Claude. (Requirement 3.8)
func ModelFromString(s string) ModelID {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "claude":
		return ModelClaude
	case "nova":
		return ModelNovaPro
	case "kimi", "": // default
		return ModelKimi
	default:
		return ModelKimi
	}
}

// responseSegments is the JSON contract the model is asked to return.
type responseSegments struct {
	Segments []struct {
		Start    float64 `json:"start"`
		End      float64 `json:"end"`
		Score    float64 `json:"score"`
		HookText string  `json:"hookText"`
	} `json:"segments"`
}

// Rank prompts the configured model for candidate viral moments, validates the
// response, and returns at most MaxClipCount RankedSegments ordered by
// descending score. CaptionWords are populated by intersecting the transcript's
// word list with each segment's [start, end] window. Malformed or non-JSON
// model output is treated as zero valid segments.
// (Requirements 3.1, 3.2, 3.3, 3.9, 3.10)
func (s *Service) Rank(ctx context.Context, transcript models.Transcript, sourceDuration float64) ([]models.RankedSegment, error) {
	prompt := buildPrompt(transcript, sourceDuration)

	body, err := s.buildRequestBody(prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to build request body: %w", err)
	}

	out, err := s.Client.InvokeModel(ctx, &bedrockruntime.InvokeModelInput{
		ModelId:     aws.String(string(s.Model)),
		Body:        body,
		ContentType: aws.String("application/json"),
		Accept:      aws.String("application/json"),
	})
	if err != nil {
		return nil, fmt.Errorf("bedrock InvokeModel failed: %w", err)
	}

	rawText, err := s.extractText(out.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read model response: %w", err)
	}

	raw := parseSegments(rawText)

	valid := validateSegments(raw, sourceDuration)
	if len(valid) == 0 {
		return nil, ErrNoValidSegments
	}

	for i := range valid {
		valid[i].CaptionWords = wordsInRange(transcript.Words, valid[i].Start, valid[i].End)
	}

	return valid, nil
}

// buildPrompt renders the transcript as timestamped lines and fixes the output
// contract and constraints (per the Bedrock Ranking Prompt Design).
func buildPrompt(transcript models.Transcript, sourceDuration float64) string {
	var lines strings.Builder
	for _, w := range transcript.Words {
		fmt.Fprintf(&lines, "%.2f | %s\n", w.Start, w.Text)
	}

	return fmt.Sprintf(`You are a short-form video editor. From the timestamped transcript below,
identify the %d most engaging, self-contained "viral moment" segments.

Rules:
- Each segment MUST be between %.0f and %.0f seconds long.
- start and end are in seconds, measured from the beginning of the source.
- 0 <= start < end <= %.2f.
- Prefer moments with a strong hook, emotional peak, or a complete thought.
- Return at most %d segments, ordered from most to least engaging.
- Respond with ONLY a JSON object matching the schema. No prose, no markdown.

Schema: {"segments":[{"start":number,"end":number,"score":number,"hookText":string}]}

Transcript (seconds | text):
%s`,
		shorts.MaxClipCount,
		shorts.MinClipDuration,
		shorts.MaxClipDuration,
		sourceDuration,
		shorts.MaxClipCount,
		lines.String(),
	)
}

// novaRequest is the Amazon Nova Pro invocation payload.
type novaRequest struct {
	Messages []novaMessage `json:"messages"`
	InferenceConfig novaInferenceConfig `json:"inferenceConfig"`
}

type novaMessage struct {
	Role    string             `json:"role"`
	Content []novaContentBlock `json:"content"`
}

type novaContentBlock struct {
	Text string `json:"text"`
}

type novaInferenceConfig struct {
	MaxTokens   int     `json:"maxTokens"`
	Temperature float64 `json:"temperature"`
}

// claudeRequest is the Anthropic Messages invocation payload.
type claudeRequest struct {
	AnthropicVersion string          `json:"anthropic_version"`
	MaxTokens        int             `json:"max_tokens"`
	Temperature      float64         `json:"temperature"`
	Messages         []claudeMessage `json:"messages"`
}

type claudeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// kimiRequest is the Moonshot Kimi invocation payload (OpenAI chat format).
type kimiRequest struct {
	Messages    []kimiMessage `json:"messages"`
	MaxTokens   int           `json:"max_tokens"`
	Temperature float64       `json:"temperature"`
}

type kimiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// buildRequestBody builds the model-specific invocation payload.
func (s *Service) buildRequestBody(prompt string) ([]byte, error) {
	switch s.Model {
	case ModelClaude:
		return json.Marshal(claudeRequest{
			AnthropicVersion: "bedrock-2023-05-31",
			MaxTokens:        2000,
			Temperature:      0.2,
			Messages: []claudeMessage{
				{Role: "user", Content: prompt},
			},
		})
	case ModelNovaPro:
		return json.Marshal(novaRequest{
			Messages: []novaMessage{
				{Role: "user", Content: []novaContentBlock{{Text: prompt}}},
			},
			InferenceConfig: novaInferenceConfig{MaxTokens: 2000, Temperature: 0.2},
		})
	default: // Kimi (default)
		return json.Marshal(kimiRequest{
			Messages: []kimiMessage{
				{Role: "user", Content: prompt},
			},
			MaxTokens:   2000,
			Temperature: 0.2,
		})
	}
}

// novaResponse maps the Nova Pro response location output.message.content[0].text.
type novaResponse struct {
	Output struct {
		Message struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"message"`
	} `json:"output"`
}

// claudeResponse maps the Anthropic response location content[0].text.
type claudeResponse struct {
	Content []struct {
		Text string `json:"text"`
	} `json:"content"`
}

// kimiResponse maps the OpenAI-compatible response location
// choices[0].message.content.
type kimiResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// extractText reads the raw model text from the model-specific response body
// and normalizes each adapter to a single string.
func (s *Service) extractText(body []byte) (string, error) {
	switch s.Model {
	case ModelClaude:
		var r claudeResponse
		if err := json.Unmarshal(body, &r); err != nil {
			return "", err
		}
		if len(r.Content) == 0 {
			return "", nil
		}
		return r.Content[0].Text, nil
	case ModelNovaPro:
		var r novaResponse
		if err := json.Unmarshal(body, &r); err != nil {
			return "", err
		}
		if len(r.Output.Message.Content) == 0 {
			return "", nil
		}
		return r.Output.Message.Content[0].Text, nil
	default: // Kimi (default)
		var r kimiResponse
		if err := json.Unmarshal(body, &r); err != nil {
			return "", err
		}
		if len(r.Choices) == 0 {
			return "", nil
		}
		return r.Choices[0].Message.Content, nil
	}
}

// parseSegments extracts and parses the JSON segment list from raw model text.
// It defensively strips markdown fences and surrounding prose, and treats any
// parse failure as zero segments.
func parseSegments(text string) []rawSegment {
	jsonStr := extractJSON(text)
	if jsonStr == "" {
		return nil
	}

	var resp responseSegments
	if err := json.Unmarshal([]byte(jsonStr), &resp); err != nil {
		return nil
	}

	raw := make([]rawSegment, 0, len(resp.Segments))
	for _, seg := range resp.Segments {
		raw = append(raw, rawSegment{
			Start:    seg.Start,
			End:      seg.End,
			Score:    seg.Score,
			HookText: seg.HookText,
		})
	}
	return raw
}

// extractJSON pulls the outermost JSON object out of a model response that may
// be wrapped in markdown code fences or surrounded by prose. It returns the
// empty string when no JSON object is present.
func extractJSON(text string) string {
	s := strings.TrimSpace(text)

	// Strip a leading code fence (```json or ```) and its closing fence.
	if strings.HasPrefix(s, "```") {
		if idx := strings.IndexByte(s, '\n'); idx != -1 {
			s = s[idx+1:]
		}
		if idx := strings.LastIndex(s, "```"); idx != -1 {
			s = s[:idx]
		}
		s = strings.TrimSpace(s)
	}

	// Fall back to the first '{' .. matching last '}' span.
	start := strings.IndexByte(s, '{')
	end := strings.LastIndexByte(s, '}')
	if start == -1 || end == -1 || end < start {
		return ""
	}
	return s[start : end+1]
}

// wordsInRange returns the transcript words whose timing falls within
// [start, end]. A word is included when it overlaps the window.
func wordsInRange(words []models.Word, start, end float64) []models.Word {
	out := make([]models.Word, 0)
	for _, w := range words {
		if w.End >= start && w.Start <= end {
			out = append(out, w)
		}
	}
	return out
}
