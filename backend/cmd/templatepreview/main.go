// Command templatepreview renders a sample question slide and answer slide for
// every slide design with the real production renderer (slidegen.NativeRenderer).
//
// The images feed the design picker in the project form and the designs
// section of the site, so what people see is exactly what their video will
// look like. Nothing here is mocked or redrawn.
//
//	go run ./cmd/templatepreview -out /tmp/previews
//
// It writes <design>-question.png and <design>-answer.png at the full
// 1920x1080 slide size. scripts/generate-template-previews.sh turns the
// question slides into the small WebP files the frontend serves.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/rahul/indifferent/backend/internal/models"
	"github.com/rahul/indifferent/backend/internal/slidegen"
)

// sample is the quiz every preview shows. It is deliberately ordinary: a real
// question, four options, one correct answer, so the previews show how a
// theme handles text, not a contrived best case.
func sample() slidegen.QuestionSlideData {
	return slidegen.QuestionSlideData{
		QuestionNumber: 2,
		TotalQuestions: 5,
		QuestionText:   "Which AWS service runs containers without you managing servers?",
		Options: []models.Option{
			{Label: "A", Text: "Amazon EC2 with an auto scaling group"},
			{Label: "B", Text: "AWS Fargate"},
			{Label: "C", Text: "Amazon Lightsail"},
			{Label: "D", Text: "AWS Batch on Spot instances"},
		},
		CorrectIndex:   1,
		CorrectIndices: []int{1},
	}
}

func main() {
	out := flag.String("out", "", "directory to write the PNG files to (required)")
	flag.Parse()
	if *out == "" {
		flag.Usage()
		os.Exit(2)
	}
	if err := os.MkdirAll(*out, 0o755); err != nil {
		log.Fatal(err)
	}

	r := slidegen.NewNativeRenderer()
	for _, name := range slidegen.ThemeNames() {
		for _, slide := range []struct{ kind, file string }{
			{"question", "question.html"},
			{"answer", "answer-reveal.html"},
		} {
			png, err := r.RenderTemplate(name+"/"+slide.file, sample())
			if err != nil {
				log.Fatalf("%s %s: %v", name, slide.kind, err)
			}
			path := filepath.Join(*out, fmt.Sprintf("%s-%s.png", name, slide.kind))
			if err := os.WriteFile(path, png, 0o644); err != nil {
				log.Fatal(err)
			}
			fmt.Println("wrote", path)
		}
	}
}
