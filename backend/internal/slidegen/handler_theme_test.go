package slidegen

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/rahul/indifferent/backend/internal/models"
)

// The template chosen on the project must reach the renderer, otherwise every
// project would render as Classic no matter what the user picked.
func TestHandleRequest_PassesTemplateToRenderer(t *testing.T) {
	questions := []models.Question{
		{Index: 0, Text: "Q1?", Options: []models.Option{{Label: "A", Text: "A1"}}, CorrectIndex: 0},
	}
	jsonData, _ := json.Marshal(questions)

	for _, name := range ThemeNames() {
		t.Run(name, func(t *testing.T) {
			store := newMockStorage()
			store.objects["parsed/p/questions.json"] = jsonData

			var got []string
			renderer := &mockRenderer{
				renderFunc: func(templateName string, _ interface{}) ([]byte, error) {
					got = append(got, templateName)
					return []byte("png"), nil
				},
			}

			h := NewHandler(renderer, store, "bucket")
			_, err := h.HandleRequest(context.Background(), models.SlideGenInput{
				ProjectID: "p",
				JSONKey:   "parsed/p/questions.json",
				Template:  name,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			want := []string{name + "/question.html", name + "/answer-reveal.html"}
			if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
				t.Errorf("renderer called with %v, want %v", got, want)
			}
		})
	}
}

func TestHandleRequest_PassesDesignOptionsToRenderer(t *testing.T) {
	questions := []models.Question{
		{Index: 0, Text: "Q1?", Options: []models.Option{{Label: "A", Text: "A1"}}, CorrectIndex: 0},
	}
	jsonData, _ := json.Marshal(questions)
	store := newMockStorage()
	store.objects["parsed/p/questions.json"] = jsonData

	show := false
	want := models.DesignOptions{
		Accent:             "#92400E",
		TextSize:           "l",
		ShowQuestionNumber: &show,
		AnswerLabels:       "numbers",
	}
	var got []models.DesignOptions
	renderer := &mockRenderer{
		renderFunc: func(_ string, data interface{}) ([]byte, error) {
			got = append(got, data.(QuestionSlideData).DesignOptions)
			return []byte("png"), nil
		},
	}

	h := NewHandler(renderer, store, "bucket")
	if _, err := h.HandleRequest(context.Background(), models.SlideGenInput{
		ProjectID:     "p",
		JSONKey:       "parsed/p/questions.json",
		Template:      "classic",
		DesignOptions: want,
	}); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("rendered %d slides, want 2", len(got))
	}
	for _, options := range got {
		if options.Accent != want.Accent || options.TextSize != want.TextSize || options.AnswerLabels != want.AnswerLabels {
			t.Errorf("renderer options = %+v, want %+v", options, want)
		}
		if options.ShowQuestionNumber == nil || *options.ShowQuestionNumber {
			t.Errorf("showQuestionNumber = %v, want false", options.ShowQuestionNumber)
		}
	}
}
