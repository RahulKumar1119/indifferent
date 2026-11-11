package shorts

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// Issue #286: a source shorter than one clip can never yield a clip, and the
// clip and source limits stay as they were.
func TestSourceAndClipLimits(t *testing.T) {
	if MinSourceDuration != MinClipDuration {
		t.Errorf("MinSourceDuration = %v, want MinClipDuration (%v)", MinSourceDuration, MinClipDuration)
	}
	if MinClipDuration != 30 || MaxClipDuration != 90 || MaxSourceDuration != 3600 {
		t.Errorf("limits changed: clip %v to %v, source up to %v", MinClipDuration, MaxClipDuration, MaxSourceDuration)
	}
	if !strings.HasPrefix(CodeNoSegments, "VALIDATION") {
		t.Errorf("%s must be a VALIDATION code so it is never retried", CodeNoSegments)
	}
}

// The one-off migration for the jobs that failed before the fix writes the
// same message the pipeline now stores. The two must not drift.
func TestTheRecodeScriptWritesTheSameMessage(t *testing.T) {
	src, err := os.ReadFile("../../../scripts/ops/286_recode_no_segments.py")
	if err != nil {
		t.Fatalf("read the script: %v", err)
	}
	m := regexp.MustCompile(`(?s)NEW_MESSAGE = \(\s*"([^"]*)"\s*"([^"]*)"\s*\)`).FindSubmatch(src)
	if m == nil {
		t.Fatal("NEW_MESSAGE not found in the script; update this test if it moved")
	}
	if got := string(m[1]) + string(m[2]); got != NoSegmentsMessage {
		t.Errorf("script message %q, want %q", got, NoSegmentsMessage)
	}
	if !regexp.MustCompile(`NEW_CODE = "` + CodeNoSegments + `"`).Match(src) {
		t.Errorf("script does not write %s", CodeNoSegments)
	}
}
