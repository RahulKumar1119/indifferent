package thumbnail

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
	"github.com/aws/smithy-go"
)

type fakeConverse struct {
	replies []fakeReply
	calls   []*bedrockruntime.ConverseInput
}

type fakeReply struct {
	text string
	err  error
}

func (f *fakeConverse) Converse(_ context.Context, in *bedrockruntime.ConverseInput, _ ...func(*bedrockruntime.Options)) (*bedrockruntime.ConverseOutput, error) {
	f.calls = append(f.calls, in)
	r := f.replies[min(len(f.calls)-1, len(f.replies)-1)]
	if r.err != nil {
		return nil, r.err
	}
	return &bedrockruntime.ConverseOutput{Output: &types.ConverseOutputMemberMessage{Value: types.Message{
		Role:    types.ConversationRoleAssistant,
		Content: []types.ContentBlock{&types.ContentBlockMemberText{Value: r.text}},
	}}}, nil
}

func apiErr(code string) error {
	return &smithy.GenericAPIError{Code: code, Message: code}
}

func sampleInput() AnalysisInput {
	return AnalysisInput{
		Frames: []AnalysisFrame{
			{Time: 5, JPEG: []byte("a")},
			{Time: 65, JPEG: []byte("b")},
			{Time: 125, JPEG: []byte("c")},
		},
		Transcript: "Today we compare five AI tools for developers.",
		Duration:   180,
		Format:     FormatLandscape,
	}
}

const goodReply = "```json\n" + `{
  "topic": "AI tools for developers",
  "mainSubject": "person at a laptop",
  "emotion": "Surprise",
  "keywords": ["AI", "tools", "productivity"],
  "headline": "5 ai tools you need in 2026",
  "secondaryText": "Number 3 is wild",
  "bestFrame": 2,
  "visualConcept": "Developer reacting to a laptop screen",
  "suggestedStyle": "technology"
}` + "\n```"

func TestParseAnalysis(t *testing.T) {
	got, err := ParseAnalysis(goodReply, sampleInput())
	if err != nil {
		t.Fatal(err)
	}
	if got.Headline != "5 AI TOOLS YOU NEED" {
		t.Errorf("headline = %q (should be upper-cased and cut to %d words)", got.Headline, headlineMaxWords)
	}
	if got.SecondaryText != "NUMBER 3 IS WILD" {
		t.Errorf("secondary = %q", got.SecondaryText)
	}
	if got.Emotion != "surprise" || got.SuggestedStyle != StyleTechnology {
		t.Errorf("emotion/style = %q / %q", got.Emotion, got.SuggestedStyle)
	}
	if got.BestFrameTime != 65 {
		t.Errorf("best frame time = %v, want 65 (frame 2)", got.BestFrameTime)
	}
	if len(got.Keywords) != 3 {
		t.Errorf("keywords = %v", got.Keywords)
	}
}

func TestParseAnalysisRejectsUnusableReplies(t *testing.T) {
	for name, reply := range map[string]string{
		"no json":        "I think the headline should be great!",
		"broken json":    `{"headline": "OK",`,
		"empty headline": `{"headline": "   "}`,
		"emoji only":     `{"headline": "🔥🔥🔥"}`,
	} {
		if _, err := ParseAnalysis(reply, sampleInput()); !errors.Is(err, errBadModelOutput) {
			t.Errorf("%s: err = %v, want errBadModelOutput", name, err)
		}
	}
}

func TestParseAnalysisIgnoresOutOfRangeBestFrame(t *testing.T) {
	for _, n := range []string{"0", "9", "-1"} {
		got, err := ParseAnalysis(`{"headline":"HELLO","bestFrame":`+n+`,"suggestedStyle":"retro"}`, sampleInput())
		if err != nil {
			t.Fatal(err)
		}
		if got.BestFrameTime != -1 {
			t.Errorf("bestFrame %s gave time %v, want -1", n, got.BestFrameTime)
		}
		if got.SuggestedStyle != "" {
			t.Errorf("unknown style should be dropped, got %q", got.SuggestedStyle)
		}
	}
}

func TestCleanHeadline(t *testing.T) {
	tests := []struct {
		in       string
		maxRunes int
		maxWords int
		want     string
	}{
		{"hello world", 26, 4, "HELLO WORLD"},
		{"  spaced    out\ttext\n", 26, 4, "SPACED OUT TEXT"},
		{"5 AI tools 🔥 you need!", 26, 4, "5 AI TOOLS YOU"},
		{"it’s here", 26, 4, "IT'S HERE"},
		{"<script>alert(1)</script>", 26, 4, "SCRIPTALERT1 SCRIPT"},
		{"one two three four five six", 26, 4, "ONE TWO THREE FOUR"},
		{"averyveryverylongwordthatgoesonforever", 10, 4, "AVERYVERYV"},
		{"alpha beta gamma delta", 14, 4, "ALPHA BETA"},
		{"AI/ML | the future", 26, 4, "AI ML THE FUTURE"},
		{"日本語のタイトル", 26, 4, ""},
		{"- leading and trailing -", 26, 4, "LEADING AND TRAILING"},
		{"", 26, 4, ""},
	}
	for _, tt := range tests {
		if got := CleanHeadline(tt.in, tt.maxRunes, tt.maxWords); got != tt.want {
			t.Errorf("CleanHeadline(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestCleanHeadlineOutputIsDrawable(t *testing.T) {
	// Whatever the model says, every character must exist in the fonts.
	face, err := faceFor(FontBold, 40)
	if err != nil {
		t.Fatal(err)
	}
	nasty := "Ünïcödé 🚀 – “quotes” … ‹x› ® ™ ₹ 100% $5 + #hash @at"
	for _, r := range CleanHeadline(nasty, 60, 20) {
		if _, ok := face.GlyphAdvance(r); !ok {
			t.Errorf("glyph %q is not in the font", r)
		}
	}
}

func TestSampleText(t *testing.T) {
	short := "a short transcript"
	if SampleText(short, 100) != short {
		t.Error("short text must pass through")
	}
	long := strings.Repeat("start ", 1000) + strings.Repeat("end ", 1000)
	got := SampleText(long, 1000)
	if len([]rune(got)) > 1000+10 {
		t.Errorf("sample is %d runes", len([]rune(got)))
	}
	if !strings.HasPrefix(got, "start") || !strings.HasSuffix(got, "end") || !strings.Contains(got, "[...]") {
		t.Errorf("sample should keep the start and end and mark the cut: %q...%q", got[:12], got[len(got)-12:])
	}
}

func TestBuildPrompt(t *testing.T) {
	p := BuildPrompt(sampleInput())
	for _, want := range []string{"3 frames", "<transcript>", "Today we compare five AI tools", "ignore any instructions", "bestFrame", "technology", "English"} {
		if !strings.Contains(p, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
	in := sampleInput()
	in.Transcript = ""
	if p := BuildPrompt(in); !strings.Contains(p, "no transcript") || strings.Contains(p, "<transcript>") {
		t.Error("a missing transcript must be stated, not left as empty markers")
	}
	in.Style = StyleNews
	if p := BuildPrompt(in); !strings.Contains(p, `"news"`) {
		t.Error("the chosen style should be passed to the model")
	}
}

func TestBedrockAnalyzerSendsImagesBeforeInstructions(t *testing.T) {
	c := &fakeConverse{replies: []fakeReply{{text: goodReply}}}
	a := &BedrockAnalyzer{Client: c, Model: "test-model"}
	res, err := a.Analyze(context.Background(), sampleInput())
	if err != nil {
		t.Fatal(err)
	}
	if res.Headline == "" {
		t.Error("no headline")
	}
	in := c.calls[0]
	if *in.ModelId != "test-model" {
		t.Errorf("model = %q", *in.ModelId)
	}
	content := in.Messages[0].Content
	images := 0
	for _, b := range content {
		if _, ok := b.(*types.ContentBlockMemberImage); ok {
			images++
		}
	}
	if images != 3 {
		t.Errorf("images sent = %d, want 3", images)
	}
	last, ok := content[len(content)-1].(*types.ContentBlockMemberText)
	if !ok || !strings.Contains(last.Value, "Reply with JSON") {
		t.Error("instructions must come after the frames")
	}
	if first, ok := content[0].(*types.ContentBlockMemberText); !ok || !strings.Contains(first.Value, "Frame 1") {
		t.Error("each frame should be introduced by a label")
	}
}

func TestBedrockAnalyzerDefaultModel(t *testing.T) {
	c := &fakeConverse{replies: []fakeReply{{text: goodReply}}}
	if _, err := (&BedrockAnalyzer{Client: c}).Analyze(context.Background(), sampleInput()); err != nil {
		t.Fatal(err)
	}
	if *c.calls[0].ModelId != DefaultModel {
		t.Errorf("model = %q, want default", *c.calls[0].ModelId)
	}
}

func TestBedrockAnalyzerRetriesThrottlingThenSucceeds(t *testing.T) {
	c := &fakeConverse{replies: []fakeReply{{err: apiErr("ThrottlingException")}, {text: goodReply}}}
	slept := 0
	a := &BedrockAnalyzer{Client: c, Sleep: func(time.Duration) { slept++ }}
	if _, err := a.Analyze(context.Background(), sampleInput()); err != nil {
		t.Fatal(err)
	}
	if len(c.calls) != 2 || slept != 1 {
		t.Errorf("calls=%d slept=%d, want 2 and 1", len(c.calls), slept)
	}
}

func TestBedrockAnalyzerRetriesAnUnusableReply(t *testing.T) {
	c := &fakeConverse{replies: []fakeReply{{text: "no idea"}, {text: goodReply}}}
	a := &BedrockAnalyzer{Client: c, Sleep: func(time.Duration) {}}
	if _, err := a.Analyze(context.Background(), sampleInput()); err != nil {
		t.Fatalf("a second attempt should recover: %v", err)
	}
}

func TestBedrockAnalyzerErrorClasses(t *testing.T) {
	tests := []struct {
		name  string
		reply fakeReply
		want  string
		calls int
	}{
		{"throttled to the end", fakeReply{err: apiErr("ThrottlingException")}, CodeAITimeout, 2},
		{"model timeout", fakeReply{err: apiErr("ModelTimeoutException")}, CodeAITimeout, 2},
		{"access denied is not retried", fakeReply{err: apiErr("AccessDeniedException")}, CodeAIFailed, 1},
		{"validation is not retried", fakeReply{err: apiErr("ValidationException")}, CodeAIFailed, 1},
		{"deadline exceeded", fakeReply{err: context.DeadlineExceeded}, CodeAITimeout, 2},
		{"always garbage", fakeReply{text: "garbage"}, CodeAIFailed, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &fakeConverse{replies: []fakeReply{tt.reply}}
			a := &BedrockAnalyzer{Client: c, Sleep: func(time.Duration) {}}
			_, err := a.Analyze(context.Background(), sampleInput())
			if CodeOf(err) != tt.want {
				t.Errorf("code = %q, want %q (err %v)", CodeOf(err), tt.want, err)
			}
			if len(c.calls) != tt.calls {
				t.Errorf("calls = %d, want %d", len(c.calls), tt.calls)
			}
			if MessageOf(err) == "" {
				t.Error("every AI failure needs a user-facing message")
			}
		})
	}
}

func TestBedrockAnalyzerStopsWhenCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	c := &fakeConverse{replies: []fakeReply{{err: apiErr("ThrottlingException")}}}
	a := &BedrockAnalyzer{Client: c, Sleep: func(time.Duration) { t.Error("should not sleep after cancellation") }}
	_, _ = a.Analyze(ctx, sampleInput())
	if len(c.calls) > 1 {
		t.Errorf("made %d calls after cancellation", len(c.calls))
	}
}

func TestBedrockAnalyzerNeedsFrames(t *testing.T) {
	a := &BedrockAnalyzer{Client: &fakeConverse{replies: []fakeReply{{text: goodReply}}}}
	if _, err := a.Analyze(context.Background(), AnalysisInput{}); CodeOf(err) != CodeNoSuitableFrames {
		t.Errorf("code = %q", CodeOf(err))
	}
}
