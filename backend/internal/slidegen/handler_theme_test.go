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
