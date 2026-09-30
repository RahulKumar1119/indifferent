package rank

import (
	"testing"

	"github.com/rahul/indifferent/backend/internal/shorts"
	"pgregory.net/rapid"
)

// genRawSegments builds a randomized slice of candidate segments mixing valid
// and invalid values (negative starts, inverted ranges, out-of-range
// durations) so validation is exercised across the full input space.
func genRawSegments(t *rapid.T) []rawSegment {
	n := rapid.IntRange(0, 12).Draw(t, "count")
	segs := make([]rawSegment, 0, n)
	for i := 0; i < n; i++ {
		start := rapid.Float64Range(-50, 700).Draw(t, "start")
		// Span can be negative (start>=end), tiny, in-range, or too large.
		span := rapid.Float64Range(-30, 120).Draw(t, "span")
		segs = append(segs, rawSegment{
			Start:    start,
			End:      start + span,
			Score:    rapid.Float64Range(0, 1).Draw(t, "score"),
			HookText: rapid.String().Draw(t, "hook"),
		})
	}
	return segs
}

// Feature: ai-shorts-generator, Property 1
// Validated segments have start strictly before end.
// Validates: Requirements 3.4
func TestProperty1_StartBeforeEnd(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		raw := genRawSegments(t)
		dur := rapid.Float64Range(0, 700).Draw(t, "sourceDuration")
		for _, s := range validateSegments(raw, dur) {
			if !(s.Start < s.End) {
				t.Fatalf("segment has start >= end: start=%v end=%v", s.Start, s.End)
			}
		}
	})
}

// Feature: ai-shorts-generator, Property 2
// Validated segment duration is within [MinClipDuration, MaxClipDuration].
// Validates: Requirements 3.5
func TestProperty2_DurationBounds(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		raw := genRawSegments(t)
		dur := rapid.Float64Range(0, 700).Draw(t, "sourceDuration")
		for _, s := range validateSegments(raw, dur) {
			d := s.End - s.Start
			if d < shorts.MinClipDuration || d > shorts.MaxClipDuration {
				t.Fatalf("segment duration %v out of bounds [%v, %v]", d, shorts.MinClipDuration, shorts.MaxClipDuration)
			}
		}
	})
}

// Feature: ai-shorts-generator, Property 3
// Validated segment timestamps fall within the source.
// Validates: Requirements 3.6
func TestProperty3_InSourceBounds(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		raw := genRawSegments(t)
		dur := rapid.Float64Range(0, 700).Draw(t, "sourceDuration")
		for _, s := range validateSegments(raw, dur) {
			if s.Start < 0 {
				t.Fatalf("segment start %v < 0", s.Start)
			}
			if s.End > dur {
				t.Fatalf("segment end %v > sourceDuration %v", s.End, dur)
			}
		}
	})
}

// Feature: ai-shorts-generator, Property 4
// Output segment count never exceeds MaxClipCount.
// Validates: Requirements 3.7, 9.2
func TestProperty4_MaxCount(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		raw := genRawSegments(t)
		dur := rapid.Float64Range(0, 700).Draw(t, "sourceDuration")
		if got := len(validateSegments(raw, dur)); got > shorts.MaxClipCount {
			t.Fatalf("returned %d segments, exceeds MaxClipCount %d", got, shorts.MaxClipCount)
		}
	})
}

// Feature: ai-shorts-generator, Property 5
// Validated segments are ordered by descending score with sequential ranks.
// Validates: Requirements 3.2
func TestProperty5_DescendingScoreSequentialRanks(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		raw := genRawSegments(t)
		dur := rapid.Float64Range(0, 700).Draw(t, "sourceDuration")
		result := validateSegments(raw, dur)
		for i := range result {
			if result[i].Rank != i+1 {
				t.Fatalf("segment %d has rank %d, want %d", i, result[i].Rank, i+1)
			}
			if i > 0 && result[i-1].Score < result[i].Score {
				t.Fatalf("scores not non-increasing at %d: %v then %v", i, result[i-1].Score, result[i].Score)
			}
		}
	})
}
