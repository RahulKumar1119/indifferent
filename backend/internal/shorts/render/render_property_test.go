package render

import (
	"context"
	"strings"
	"testing"

	"github.com/rahul/indifferent/backend/internal/models"
	"pgregory.net/rapid"
)

// genSegment produces a valid ranked segment: 0 <= start < end within a source.
func genSegment(t *rapid.T) (start, end float64) {
	start = rapid.Float64Range(0, 500).Draw(t, "start")
	dur := rapid.Float64Range(15, 60).Draw(t, "dur")
	end = start + dur
	return start, end
}

// Feature: ai-shorts-generator, Property 6
// Rendered clips are 9:16: the crop+scale filter produced by
// buildCropCaptionArgs yields 1080x1920 output via a center-crop expression.
// Validates: Requirements 4.2, 4.3
func TestProperty6_RenderedClipsAre9x16(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		start, end := genSegment(t)
		audioOnly := rapid.Bool().Draw(t, "audioOnly")

		r := &Renderer{Bucket: "b", WorkDir: "/tmp"}
		args := r.buildCropCaptionArgs("source.mp4", "captions.srt", "out.mp4", start, end, audioOnly, "", false)
		joined := strings.Join(args, " ")

		// Every clip is normalized to the canonical 9:16 output dimensions.
		if audioOnly {
			// Audio-only path uses a solid 1080x1920 canvas.
			if !strings.Contains(joined, "s=1080x1920") {
				t.Fatalf("audio-only args missing 1080x1920 canvas: %s", joined)
			}
		} else {
			// Video path center-crops then scales to 1080:1920.
			if !strings.Contains(joined, "scale=1080:1920") {
				t.Fatalf("video args missing scale=1080:1920: %s", joined)
			}
			if !strings.Contains(joined, "crop=w=ih*9/16") {
				t.Fatalf("video args missing center-crop expression: %s", joined)
			}
		}
	})
}

// Feature: ai-shorts-generator, Property 9
// Idempotent render produces no duplicate clips: when HeadObject reports the
// clip already exists, Render performs no PutObject and returns the existing
// key.
// Validates: Requirements 5.5
func TestProperty9_IdempotentRender(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		userID := rapid.StringMatching(`[a-z0-9]{1,12}`).Draw(t, "userID")
		jobID := rapid.StringMatching(`[a-z0-9]{1,12}`).Draw(t, "jobID")
		clipID := rapid.StringMatching(`[a-z0-9]{1,12}`).Draw(t, "clipID")

		store := &mockStorage{exists: true}
		r := &Renderer{
			Storage: store,
			Bucket:  "assets",
			// WorkDir is never touched: Render returns before creating it
			// when the clip already exists.
			WorkDir: "/tmp/does-not-matter",
			RunCommand: func(name string, args []string) error {
				t.Fatalf("RunCommand must not be invoked for an existing clip")
				return nil
			},
		}

		in := RenderInput{
			JobID:  jobID,
			UserID: userID,
			Clip:   models.Clip{ClipID: clipID},
		}

		key, err := r.Render(context.Background(), in)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		wantKey := "shorts/" + userID + "/" + jobID + "/clips/" + clipID + ".mp4"
		if key != wantKey {
			t.Fatalf("expected existing key %q, got %q", wantKey, key)
		}
		if store.putCalls != 0 {
			t.Fatalf("expected 0 PutObject calls for an existing clip, got %d", store.putCalls)
		}
		if store.getCalls != 0 {
			t.Fatalf("expected 0 GetObject calls for an existing clip, got %d", store.getCalls)
		}
	})
}
