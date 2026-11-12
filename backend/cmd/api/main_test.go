package main

import (
	"testing"

	"github.com/rahul/indifferent/backend/internal/shorts/rank"
)

// Issue #293: the planner defaults to Qwen3 with DeepSeek as the fallback, and
// a typo must not quietly select Kimi.
func TestPlanningModel(t *testing.T) {
	for _, c := range []struct {
		in   string
		def  rank.ModelID
		want rank.ModelID
	}{
		{"", rank.ModelQwen3, rank.ModelQwen3},
		{"  ", rank.ModelDeepSeek, rank.ModelDeepSeek},
		{"qwen", rank.ModelDeepSeek, rank.ModelQwen3},
		{"DeepSeek", rank.ModelQwen3, rank.ModelDeepSeek},
		{"kimi", rank.ModelQwen3, rank.ModelKimi},
		{"nova", rank.ModelQwen3, rank.ModelNovaPro},
		{"claude", rank.ModelQwen3, rank.ModelClaude},
		{"qwenn", rank.ModelQwen3, rank.ModelQwen3},
		{"minimax", rank.ModelDeepSeek, rank.ModelDeepSeek},
	} {
		if got := planningModel(c.in, c.def); got != c.want {
			t.Errorf("planningModel(%q, %s) = %s, want %s", c.in, c.def, got, c.want)
		}
	}
}
