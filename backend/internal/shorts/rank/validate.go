// Package rank contains the Bedrock-backed ranking service and the pure
// validation logic that turns untrusted model output into safe, ordered
// RankedSegments for the AI Shorts pipeline.
package rank

import (
	"errors"
	"sort"

	"github.com/rahul/indifferent/backend/internal/models"
	"github.com/rahul/indifferent/backend/internal/shorts"
)

// ErrNoValidSegments is returned by callers when validation leaves zero
// segments (the model identified no usable moment). The orchestrator maps
// this to a failed job (Requirement 3.10).
var ErrNoValidSegments = errors.New("no engaging segments identified")

// rawSegment is a single unvalidated candidate moment as proposed by the
// ranking model. The service never trusts these values directly.
type rawSegment struct {
	Start    float64
	End      float64
	Score    float64
	HookText string
}

// validateSegments enforces every segment constraint, drops invalid
// candidates, orders the survivors by descending score, assigns sequential
// ranks, and truncates to MaxClipCount.
//
// Dropped when: start >= end; duration outside [MinClipDuration,
// MaxClipDuration]; start < 0; or end > sourceDuration. CaptionWords are left
// empty (populated later by the ranking service).
// (Requirements 3.2, 3.4, 3.5, 3.6, 3.7, 3.9)
func validateSegments(raw []rawSegment, sourceDuration float64) []models.RankedSegment {
	valid := make([]models.RankedSegment, 0, len(raw))
	for _, r := range raw {
		if r.Start >= r.End {
			continue
		}
		duration := r.End - r.Start
		if duration < shorts.MinClipDuration || duration > shorts.MaxClipDuration {
			continue
		}
		if r.Start < 0 || r.End > sourceDuration {
			continue
		}
		valid = append(valid, models.RankedSegment{
			Start:    r.Start,
			End:      r.End,
			Score:    r.Score,
			HookText: r.HookText,
		})
	}

	// Sort by descending score. Use a stable sort so equal scores keep their
	// relative input order deterministically.
	sort.SliceStable(valid, func(i, j int) bool {
		return valid[i].Score > valid[j].Score
	})

	if len(valid) > shorts.MaxClipCount {
		valid = valid[:shorts.MaxClipCount]
	}

	for i := range valid {
		valid[i].Rank = i + 1
	}

	return valid
}
