package api

import (
	"sort"
	"testing"

	"github.com/rahul/indifferent/backend/internal/slidegen"
)

// A template name the API accepts but the renderer has no theme for would
// silently render as Classic. A theme the renderer has but the API rejects
// could never be chosen. Keep the two lists identical.
func TestValidTemplatesMatchRendererThemes(t *testing.T) {
	var accepted []string
	for name := range validTemplates {
		accepted = append(accepted, name)
	}
	sort.Strings(accepted)

	themed := slidegen.ThemeNames()

	if len(accepted) != len(themed) {
		t.Fatalf("API accepts %v, renderer themes %v", accepted, themed)
	}
	for i := range accepted {
		if accepted[i] != themed[i] {
			t.Fatalf("API accepts %v, renderer themes %v", accepted, themed)
		}
	}
}
