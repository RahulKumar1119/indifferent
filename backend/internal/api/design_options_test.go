package api

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/service/sfn"
	"github.com/rahul/indifferent/backend/internal/models"
)

func TestValidateCreateProjectDesignOptions(t *testing.T) {
	tests := []struct {
		name    string
		opts    models.DesignOptions
		wantErr bool
	}{
		{"defaults", models.DesignOptions{}, false},
		{"second accent", models.DesignOptions{Accent: "#92400E"}, false},
		{"large numbers", models.DesignOptions{TextSize: "l", AnswerLabels: "numbers"}, false},
		{"accent from another design", models.DesignOptions{Accent: "#FF3DCB"}, true},
		{"bad size", models.DesignOptions{TextSize: "xl"}, true},
		{"bad labels", models.DesignOptions{AnswerLabels: "roman"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := CreateProjectRequest{Name: "Test", Template: "classic", Voice: "Joanna", DesignOptions: tt.opts}
			if err := validateCreateProject(&req); (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestProjectDesignOptionsRoundTrip(t *testing.T) {
	show := false
	want := models.DesignOptions{
		Accent:             "#92400E",
		TextSize:           "l",
		ShowQuestionNumber: &show,
		AnswerLabels:       "numbers",
	}
	p := previewProject()
	p.DesignOptions = want
	got := itemToProject(projectToItem(p))
	if got.DesignOptions.Accent != want.Accent ||
		got.DesignOptions.TextSize != want.TextSize ||
		got.DesignOptions.AnswerLabels != want.AnswerLabels ||
		got.DesignOptions.ShowQuestionNumber == nil ||
		*got.DesignOptions.ShowQuestionNumber != false {
		t.Errorf("round trip = %+v, want %+v", got.DesignOptions, want)
	}
}

func TestStartPipelinePassesDesignOptions(t *testing.T) {
	project := previewProject()
	project.Status = "created"
	h := previewTestHandler(t, project, &stubDownloader{}, &fakePreviewRenderer{})

	var started string
	h.SFN = &mockSFN{startExecutionFunc: func(_ context.Context, in *sfn.StartExecutionInput, _ ...func(*sfn.Options)) (*sfn.StartExecutionOutput, error) {
		started = *in.Input
		return &sfn.StartExecutionOutput{}, nil
	}}

	req := events.APIGatewayProxyRequest{
		HTTPMethod: "POST",
		Path:       "/projects/p1/start",
		Headers:    map[string]string{"Authorization": "Bearer " + generateTestToken("user1")},
	}
	resp, err := h.HandleRequest(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, resp.Body)
	}
	var input map[string]json.RawMessage
	if err := json.Unmarshal([]byte(started), &input); err != nil {
		t.Fatal(err)
	}
	var opts models.DesignOptions
	if err := json.Unmarshal(input["designOptions"], &opts); err != nil {
		t.Fatalf("designOptions missing from execution input: %v (%s)", err, started)
	}
	if opts.Accent != "#7C3AED" || opts.TextSize != "m" || opts.AnswerLabels != "letters" {
		t.Errorf("designOptions = %+v", opts)
	}
}
