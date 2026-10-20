package thumbnail

import (
	"errors"
	"fmt"
	"testing"
)

func TestParseFormat(t *testing.T) {
	for in, want := range map[string]Format{"": "", " 16:9 ": FormatLandscape, "9:16": FormatPortrait} {
		got, err := ParseFormat(in)
		if err != nil || got != want {
			t.Errorf("ParseFormat(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"4:3", "square", "16/9"} {
		if _, err := ParseFormat(bad); err == nil {
			t.Errorf("ParseFormat(%q) should fail", bad)
		}
	}
}

func TestFormatSizes(t *testing.T) {
	if w, h := FormatLandscape.Size(); w != 1280 || h != 720 {
		t.Errorf("landscape size = %dx%d", w, h)
	}
	if w, h := FormatPortrait.Size(); w != 720 || h != 1280 {
		t.Errorf("portrait size = %dx%d", w, h)
	}
	if a := FormatLandscape.Aspect(); a < 1.77 || a > 1.78 {
		t.Errorf("landscape aspect = %v", a)
	}
}

func TestFormatFor(t *testing.T) {
	tests := []struct {
		w, h int
		want Format
	}{
		{1920, 1080, FormatLandscape},
		{1080, 1920, FormatPortrait},
		{1080, 1080, FormatLandscape},
		{720, 1280, FormatPortrait},
	}
	for _, tt := range tests {
		if got := FormatFor(tt.w, tt.h); got != tt.want {
			t.Errorf("FormatFor(%d,%d) = %q, want %q", tt.w, tt.h, got, tt.want)
		}
	}
}

func TestParseStyle(t *testing.T) {
	if got, err := ParseStyle(""); err != nil || got != "" {
		t.Errorf("empty style should be allowed, got %q %v", got, err)
	}
	for _, s := range Styles() {
		got, err := ParseStyle(" " + string(s) + " ")
		if err != nil || got != s {
			t.Errorf("ParseStyle(%q) = %q, %v", s, got, err)
		}
	}
	if got, err := ParseStyle("VIRAL"); err != nil || got != StyleViral {
		t.Errorf("style parsing should ignore case, got %q %v", got, err)
	}
	if _, err := ParseStyle("retro"); err == nil {
		t.Error("unknown style should fail")
	}
}

func TestStyleList(t *testing.T) {
	if len(Styles()) != 7 {
		t.Fatalf("issue lists seven presets, got %d", len(Styles()))
	}
	for _, s := range Styles() {
		if LookFor(s).Style != s {
			t.Errorf("LookFor(%q) did not return that style", s)
		}
	}
	if LookFor("nope").Style != StyleViral {
		t.Error("unknown style should fall back to viral")
	}
}

func TestResolveStyle(t *testing.T) {
	if got := ResolveStyle(StyleNews, StyleMinimal); got != StyleNews {
		t.Errorf("explicit choice must win, got %q", got)
	}
	if got := ResolveStyle("", StyleMinimal); got != StyleMinimal {
		t.Errorf("suggestion should be used, got %q", got)
	}
	if got := ResolveStyle("", ""); got != StyleViral {
		t.Errorf("default should be viral, got %q", got)
	}
}

func TestStatusInFlight(t *testing.T) {
	for s, want := range map[Status]bool{StatusQueued: true, StatusRunning: true, StatusCompleted: false, StatusFailed: false, "": false} {
		if got := s.InFlight(); got != want {
			t.Errorf("%q.InFlight() = %v, want %v", s, got, want)
		}
	}
}

func TestObjectKeys(t *testing.T) {
	got := ObjectKey("u1", "j1", 3, 2)
	if got != "shorts/u1/j1/thumbnails/g3/thumbnail-02.jpg" {
		t.Errorf("key = %q", got)
	}
	// Must live under the job's shorts prefix so deleting the job removes it.
	if p := "shorts/u1/j1/"; got[:len(p)] != p {
		t.Errorf("key %q is outside %q", got, p)
	}
	if GenerationPrefix("u1", "j1", 3) != "shorts/u1/j1/thumbnails/g3/" {
		t.Error("generation prefix wrong")
	}
	if AssetID(1) != "thumb-01" || AssetID(12) != "thumb-12" {
		t.Error("asset IDs wrong")
	}
}

func TestErrorClassification(t *testing.T) {
	err := Errorf(CodeNoVideoTrack, "No video here.", errors.New("boom"))
	wrapped := fmt.Errorf("outer: %w", err)
	if CodeOf(wrapped) != CodeNoVideoTrack {
		t.Errorf("code = %q", CodeOf(wrapped))
	}
	if MessageOf(wrapped) != "No video here." {
		t.Errorf("message = %q", MessageOf(wrapped))
	}
	if CodeOf(errors.New("plain")) != CodeTaskFailed {
		t.Error("unclassified errors should be TASK_FAILED")
	}
	if MessageOf(errors.New("secret internal detail")) == "secret internal detail" {
		t.Error("unclassified error text must not reach users")
	}
}

func TestSetSelection(t *testing.T) {
	s := &Set{Thumbnails: []Asset{{ID: "thumb-01"}, {ID: "thumb-02"}}, SelectedID: "thumb-02"}
	if sel := s.Selected(); sel == nil || sel.ID != "thumb-02" {
		t.Errorf("Selected() = %+v", sel)
	}
	if s.Find("thumb-01") == nil || s.Find("thumb-09") != nil {
		t.Error("Find misbehaves")
	}
	var nilSet *Set
	if nilSet.Selected() != nil || nilSet.Find("x") != nil {
		t.Error("nil set should be safe")
	}
}
