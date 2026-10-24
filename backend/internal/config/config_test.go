package config

import (
	"os"
	"testing"
)

func TestEnabled(t *testing.T) {
	for value, want := range map[string]bool{
		"true": true, "TRUE": true, "True": true, " true ": true, "1": true,
		"": false, "false": false, "0": false, "yes": false, "on": false, "2": false, "truee": false,
	} {
		if got := Enabled(value); got != want {
			t.Errorf("Enabled(%q) = %v, want %v", value, got, want)
		}
	}
}

func TestS3Accelerate(t *testing.T) {
	t.Setenv("S3_ACCELERATE", "") // registers the restore of the original value
	os.Unsetenv("S3_ACCELERATE")
	if S3Accelerate() {
		t.Error("S3_ACCELERATE unset must leave acceleration off")
	}
	t.Setenv("S3_ACCELERATE", "true")
	if !S3Accelerate() {
		t.Error("S3_ACCELERATE=true must turn acceleration on")
	}
	t.Setenv("S3_ACCELERATE", "false")
	if S3Accelerate() {
		t.Error("S3_ACCELERATE=false must turn acceleration off")
	}
}
