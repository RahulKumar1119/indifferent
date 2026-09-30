package rank

import (
	"testing"

	"github.com/rahul/indifferent/backend/internal/shorts"
)

func TestValidateSegments_DropsStartGreaterOrEqualEnd(t *testing.T) {
	raw := []rawSegment{
		{Start: 30, End: 30, Score: 0.9},  // start == end
		{Start: 40, End: 20, Score: 0.8},  // start > end
		{Start: 10, End: 40, Score: 0.7},  // valid (30s)
	}
	got := validateSegments(raw, 600)
	if len(got) != 1 {
		t.Fatalf("expected 1 valid segment, got %d: %+v", len(got), got)
	}
	if got[0].Start != 10 || got[0].End != 40 {
		t.Errorf("unexpected surviving segment: %+v", got[0])
	}
}

func TestValidateSegments_DropsOutOfBounds(t *testing.T) {
	raw := []rawSegment{
		{Start: -5, End: 25, Score: 0.9},  // start < 0
		{Start: 580, End: 620, Score: 0.8}, // end > sourceDuration
		{Start: 100, End: 130, Score: 0.7}, // valid (30s)
	}
	got := validateSegments(raw, 600)
	if len(got) != 1 {
		t.Fatalf("expected 1 valid segment, got %d: %+v", len(got), got)
	}
	if got[0].Start != 100 {
		t.Errorf("unexpected surviving segment: %+v", got[0])
	}
}

func TestValidateSegments_DropsDurationOutsideBounds(t *testing.T) {
	raw := []rawSegment{
		{Start: 0, End: shorts.MinClipDuration - 1, Score: 0.9},  // under 15s
		{Start: 0, End: shorts.MaxClipDuration + 1, Score: 0.8},  // over 60s
		{Start: 0, End: shorts.MinClipDuration, Score: 0.7},      // exactly 15s (valid)
		{Start: 0, End: shorts.MaxClipDuration, Score: 0.6},      // exactly 60s (valid)
	}
	got := validateSegments(raw, 600)
	if len(got) != 2 {
		t.Fatalf("expected 2 valid segments, got %d: %+v", len(got), got)
	}
}

func TestValidateSegments_TruncatesToMaxClipCount(t *testing.T) {
	raw := []rawSegment{
		{Start: 0, End: 20, Score: 0.5},
		{Start: 30, End: 50, Score: 0.9},
		{Start: 60, End: 80, Score: 0.7},
		{Start: 90, End: 110, Score: 0.8},
		{Start: 120, End: 140, Score: 0.6},
	}
	got := validateSegments(raw, 600)
	if len(got) != shorts.MaxClipCount {
		t.Fatalf("expected %d segments, got %d", shorts.MaxClipCount, len(got))
	}
	// Highest scores retained, in descending order, ranked 1..n.
	wantScores := []float64{0.9, 0.8, 0.7}
	for i, s := range got {
		if s.Score != wantScores[i] {
			t.Errorf("segment %d score = %v, want %v", i, s.Score, wantScores[i])
		}
		if s.Rank != i+1 {
			t.Errorf("segment %d rank = %d, want %d", i, s.Rank, i+1)
		}
	}
}

func TestValidateSegments_EmptyInputYieldsEmpty(t *testing.T) {
	if got := validateSegments(nil, 600); len(got) != 0 {
		t.Fatalf("expected empty result for nil input, got %+v", got)
	}
	if got := validateSegments([]rawSegment{}, 600); len(got) != 0 {
		t.Fatalf("expected empty result for empty input, got %+v", got)
	}
}

func TestValidateSegments_AllInvalidYieldsEmpty(t *testing.T) {
	raw := []rawSegment{
		{Start: 40, End: 20, Score: 0.9},  // inverted
		{Start: 0, End: 5, Score: 0.8},    // too short
		{Start: -1, End: 30, Score: 0.7},  // negative start
	}
	got := validateSegments(raw, 600)
	if len(got) != 0 {
		t.Fatalf("expected empty result for all-invalid input, got %+v", got)
	}
	// Caller maps empty output to ErrNoValidSegments.
	if ErrNoValidSegments == nil {
		t.Fatal("ErrNoValidSegments should be defined")
	}
}
