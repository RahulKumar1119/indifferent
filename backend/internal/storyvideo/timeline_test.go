package storyvideo

import (
	"errors"
	"math"
	"strings"
	"testing"
)

func approx(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestBuildTimeline_ScenesFollowEachOtherByTheirAudio(t *testing.T) {
	tl, err := BuildTimeline([]float64{4.5, 4.4, 4.65})
	if err != nil {
		t.Fatal(err)
	}
	want := []Slot{
		{Start: 0, Span: 4.85, Show: 4.85 + TransitionSeconds},
		{Start: 4.85, Span: 4.75, Show: 4.75 + TransitionSeconds},
		{Start: 9.6, Span: 5.0, Show: 5.0 + TransitionSeconds},
	}
	for i, w := range want {
		g := tl.Slots[i]
		if !approx(g.Start, w.Start) || !approx(g.Span, w.Span) || !approx(g.Show, w.Show) {
			t.Errorf("slot %d = %+v, want %+v", i, g, w)
		}
	}
	if !approx(tl.Total, 14.6+TransitionSeconds) {
		t.Errorf("Total = %v, want %v", tl.Total, 14.6+TransitionSeconds)
	}
}

func TestBuildTimeline_EveryPictureOutlivesItsSpanByTheFade(t *testing.T) {
	// The cross-fade into scene i+1 starts at scene i+1's Start, so scene i's
	// picture must still exist for TransitionSeconds after its own span ends.
	tl, _ := BuildTimeline([]float64{3, 3, 3, 3})
	for i, s := range tl.Slots {
		if !approx(s.Show-s.Span, TransitionSeconds) {
			t.Errorf("slot %d shows for %v past its span, want %v", i, s.Show-s.Span, TransitionSeconds)
		}
		if i+1 < len(tl.Slots) && !approx(s.Start+s.Span, tl.Slots[i+1].Start) {
			t.Errorf("slot %d ends at %v but slot %d starts at %v", i, s.Start+s.Span, i+1, tl.Slots[i+1].Start)
		}
	}
	last := tl.Slots[len(tl.Slots)-1]
	if !approx(last.Start+last.Show, tl.Total) {
		t.Errorf("the last picture ends at %v, the video at %v", last.Start+last.Show, tl.Total)
	}
}

func TestBuildTimeline_ATinySceneStillGetsTimeToBeSeen(t *testing.T) {
	tl, err := BuildTimeline([]float64{0.3, 5})
	if err != nil {
		t.Fatal(err)
	}
	if tl.Slots[0].Span != MinSceneSeconds {
		t.Errorf("a 0.3 second scene got %v seconds, want %v", tl.Slots[0].Span, MinSceneSeconds)
	}
}

func TestBuildTimeline_NinetySecondsIsAllowedAndMoreIsNot(t *testing.T) {
	// 9 scenes of exactly 9.6 s audio + pause = 9.95 s each = 89.55 + fade 0.4 = 89.95
	ok := make([]float64, 9)
	for i := range ok {
		ok[i] = 9.6
	}
	if _, err := BuildTimeline(ok); err != nil {
		t.Errorf("89.95 seconds refused: %v", err)
	}
	over := make([]float64, 9)
	for i := range over {
		over[i] = 10.0
	}
	_, err := BuildTimeline(over)
	var tooLong *TooLongError
	if !errors.As(err, &tooLong) {
		t.Fatalf("err = %v, want TooLongError", err)
	}
	// 9 x (10 + 0.35) + 0.4
	if !approx(tooLong.Seconds, 93.55) || tooLong.Max != MaxTotalSeconds {
		t.Errorf("TooLongError = %+v", tooLong)
	}
	msg := tooLong.UserMessage()
	for _, want := range []string{"94 seconds", "90 second limit", "Shorten the narration"} {
		if !strings.Contains(msg, want) {
			t.Errorf("message %q lacks %q", msg, want)
		}
	}
}

func TestBuildTimeline_RefusesWhatCannotBeRendered(t *testing.T) {
	for name, in := range map[string][]float64{
		"no scenes":      nil,
		"empty audio":    {3, 0, 3},
		"negative audio": {3, -1},
		"not a number":   {math.NaN(), 3},
	} {
		if _, err := BuildTimeline(in); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
	_, err := BuildTimeline([]float64{3, 0, 3})
	if err == nil || !strings.Contains(err.Error(), "scene 2") {
		t.Errorf("the error should name the scene: %v", err)
	}
}

func TestBuildTimeline_Hindi_RunsLongerThanThePlanEstimated(t *testing.T) {
	// The plan allowed 90 seconds by estimate; Hindi speech can exceed it, and
	// that must surface as a clear error, never a silently truncated video.
	long := make([]float64, 10)
	for i := range long {
		long[i] = 9.0
	}
	if _, err := BuildTimeline(long); err == nil {
		t.Error("10 scenes of 9 s narration (97.9 s) were accepted")
	}
}
