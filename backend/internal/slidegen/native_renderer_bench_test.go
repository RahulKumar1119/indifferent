package slidegen

import (
	"fmt"
	"testing"

	"github.com/rahul/indifferent/backend/internal/models"
)

// BenchmarkTwentyQuestionVideo measures the realistic render cost of one
// 20-question quiz: 20 question slides plus 20 answer-reveal slides.
func BenchmarkTwentyQuestionVideo(b *testing.B) {
	r := NewNativeRenderer()
	questions := make([]QuestionSlideData, 20)
	for i := range questions {
		questions[i] = QuestionSlideData{
			QuestionNumber: i + 1,
			TotalQuestions: len(questions),
			QuestionText:   fmt.Sprintf("Question %d: Which option correctly answers this practice test prompt?", i+1),
			Options: []models.Option{
				{Label: "A", Text: "The first plausible answer"},
				{Label: "B", Text: "The second plausible answer"},
				{Label: "C", Text: "The third plausible answer"},
				{Label: "D", Text: "The fourth plausible answer"},
			},
			CorrectIndex: i % 4,
		}
	}

	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		for i := range questions {
			if _, err := r.RenderTemplate("classic-question.html", questions[i]); err != nil {
				b.Fatal(err)
			}
			if _, err := r.RenderTemplate("classic-answer.html", questions[i]); err != nil {
				b.Fatal(err)
			}
		}
	}
	b.ReportMetric(float64(b.Elapsed().Milliseconds())/float64(b.N)/40, "ms/slide")
}
