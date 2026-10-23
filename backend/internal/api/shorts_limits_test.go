package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/rahul/indifferent/backend/internal/shorts"
)

// The limits endpoint is the single source of truth the client reads, so it
// must always agree with the constants the pipeline enforces.
func TestGetShortsLimits_MatchesEnforcedConstants(t *testing.T) {
	h := &APIHandler{}
	resp, err := h.handleGetShortsLimits(t.Context(), events.APIGatewayProxyRequest{})
	if err != nil {
		t.Fatalf("handleGetShortsLimits: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var limits ShortsLimits
	if err := json.Unmarshal([]byte(resp.Body), &limits); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	if limits.MaxDurationSeconds != shorts.MaxSourceDuration {
		t.Errorf("MaxDurationSeconds = %v, want %v", limits.MaxDurationSeconds, shorts.MaxSourceDuration)
	}
	if limits.MaxDurationMinutes != 60 {
		t.Errorf("MaxDurationMinutes = %d, want 60", limits.MaxDurationMinutes)
	}
	if limits.MaxUploadBytes != shorts.MaxUploadBytes {
		t.Errorf("MaxUploadBytes = %d, want %d", limits.MaxUploadBytes, shorts.MaxUploadBytes)
	}
	if limits.MaxUploadMB != 2048 {
		t.Errorf("MaxUploadMB = %d, want 2048", limits.MaxUploadMB)
	}
	if limits.MaxClipCount != shorts.MaxClipCount {
		t.Errorf("MaxClipCount = %d, want %d", limits.MaxClipCount, shorts.MaxClipCount)
	}
	if len(limits.AcceptedFileTypes) != len(validShortsFileTypes) {
		t.Errorf("AcceptedFileTypes = %v, want %d entries", limits.AcceptedFileTypes, len(validShortsFileTypes))
	}
	// Sorted, so the client sees a stable order.
	for i := 1; i < len(limits.AcceptedFileTypes); i++ {
		if limits.AcceptedFileTypes[i-1] > limits.AcceptedFileTypes[i] {
			t.Errorf("AcceptedFileTypes not sorted: %v", limits.AcceptedFileTypes)
		}
	}
	if len(limits.Languages) != len(shorts.SupportedLanguages) {
		t.Errorf("Languages = %v, want %d entries", limits.Languages, len(shorts.SupportedLanguages))
	}
	seenHindi := false
	for _, lang := range limits.Languages {
		if lang.Code == shorts.LanguageHindi {
			seenHindi = true
		}
	}
	if !seenHindi {
		t.Errorf("Languages = %v, want hi-IN exposed", limits.Languages)
	}
}

// The route must be reachable and must not be shadowed by the generic
// "/shorts/<id>" case.
func TestGetShortsLimits_RouteResolves(t *testing.T) {
	h := newShortsTestHandler(t, &mockDynamoDB{})
	resp, err := h.HandleRequest(t.Context(), events.APIGatewayProxyRequest{
		HTTPMethod: "GET",
		Path:       "/shorts/limits",
	})
	if err != nil {
		t.Fatalf("HandleRequest: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d (%s), want 200", resp.StatusCode, resp.Body)
	}
	var limits ShortsLimits
	if err := json.Unmarshal([]byte(resp.Body), &limits); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if limits.MaxDurationSeconds != shorts.MaxSourceDuration {
		t.Errorf("MaxDurationSeconds = %v, want %v", limits.MaxDurationSeconds, shorts.MaxSourceDuration)
	}
}
