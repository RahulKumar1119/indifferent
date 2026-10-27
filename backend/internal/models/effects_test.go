package models

import (
	"math"
	"strings"
	"testing"
)

// validEffects returns a configuration with every effect enabled and every
// value inside its limits.
func validEffects() *VideoEffects {
	return &VideoEffects{
		Captions: &AnimatedCaptions{
			Enabled: true, Intensity: 50, Font: "sans", FontSize: 18,
			Color: "#FFFFFF", Highlight: "#FFD60A", Position: "bottom",
			Outline: 4, Background: "dark",
		},
		Zoom: &ZoomEffect{
			Enabled: true, Intensity: 40,
			PunchIns: []PunchIn{{At: 2, Duration: 1.5}, {At: 8, Duration: 0.5}},
		},
		Grade: &GradeEffect{Enabled: true, Look: "cinematic", Intensity: 50},
		Freeze: &FreezeEffect{
			Enabled: true, Intensity: 50,
			Freezes:     []FreezeHold{{At: 4, Duration: 0.8}, {At: 12, Duration: 3}},
			ImpactTexts: []ImpactText{{At: 4, Duration: 1, Text: "WAIT, WHAT?"}},
		},
		Fade: &FadeEffect{Enabled: true, InDuration: 0.5, OutDuration: 0.5},
		// Speed is present but off: freeze is on in this fixture, and the two
		// cannot both be enabled. Tests that exercise speed flip freeze off.
		Speed: &SpeedEffect{Enabled: false, Windows: []SpeedWindow{{At: 1, Duration: 2, Rate: 0.5}}},
	}
}

func TestVideoEffectsValidate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(e *VideoEffects)
		wantErr string // substring; empty means valid
	}{
		{"all valid", func(e *VideoEffects) {}, ""},
		{"nil effects", nil, ""},
		{"caption font typo", func(e *VideoEffects) { e.Captions.Font = "Arial; drop" }, "unknown caption font"},
		{"caption size low", func(e *VideoEffects) { e.Captions.FontSize = 7 }, "out of range 8-72"},
		{"caption size high", func(e *VideoEffects) { e.Captions.FontSize = 73 }, "out of range 8-72"},
		{"caption color not hex", func(e *VideoEffects) { e.Captions.Color = "red" }, "must be #RRGGBB"},
		{"caption highlight not hex", func(e *VideoEffects) { e.Captions.Highlight = "#GGGGGG" }, "must be #RRGGBB"},
		{"caption position", func(e *VideoEffects) { e.Captions.Position = "left" }, "unknown caption position"},
		{"caption outline high", func(e *VideoEffects) { e.Captions.Outline = 13 }, "caption outline 13 out of range 0-12"},
		{"caption background", func(e *VideoEffects) { e.Captions.Background = "blur" }, "unknown caption background"},
		{"caption intensity high", func(e *VideoEffects) { e.Captions.Intensity = 101 }, "out of range 0-100"},
		{"caption intensity negative", func(e *VideoEffects) { e.Captions.Intensity = -1 }, "out of range 0-100"},
		{"zoom intensity high", func(e *VideoEffects) { e.Zoom.Intensity = 150 }, "zoom intensity 150 out of range"},
		{"zoom enabled without punch-ins", func(e *VideoEffects) { e.Zoom.PunchIns = nil }, "at least one punch-in"},
		{"zoom disabled without punch-ins", func(e *VideoEffects) { e.Zoom.Enabled = false; e.Zoom.PunchIns = nil }, ""},
		{"too many punch-ins", func(e *VideoEffects) {
			e.Zoom.PunchIns = nil
			for i := 0; i < 11; i++ {
				e.Zoom.PunchIns = append(e.Zoom.PunchIns, PunchIn{At: float64(i * 7), Duration: 0.5})
			}
		}, "too many punch-ins"},
		{"punch-in before clip", func(e *VideoEffects) { e.Zoom.PunchIns[0].At = -1 }, "must be between 0 and 90 seconds"},
		{"punch-in after limit", func(e *VideoEffects) { e.Zoom.PunchIns[0].At = 91 }, "must be between 0 and 90 seconds"},
		{"punch-in NaN", func(e *VideoEffects) { e.Zoom.PunchIns[0].At = math.NaN() }, "must be between 0 and 90 seconds"},
		{"punch-in infinite", func(e *VideoEffects) { e.Zoom.PunchIns[0].At = math.Inf(1) }, "must be between 0 and 90 seconds"},
		{"punch-in too short", func(e *VideoEffects) { e.Zoom.PunchIns[0].Duration = 0.4 }, "duration 0.4 must be between 0.5 and 6 seconds"},
		{"punch-in too long", func(e *VideoEffects) { e.Zoom.PunchIns[0].Duration = 6.5 }, "duration 6.5 must be between 0.5 and 6 seconds"},
		{"punch-ins overlap", func(e *VideoEffects) { e.Zoom.PunchIns = []PunchIn{{At: 2, Duration: 2}, {At: 3, Duration: 1}} }, "punch-ins overlap"},
		{"punch-ins touching is fine", func(e *VideoEffects) { e.Zoom.PunchIns = []PunchIn{{At: 2, Duration: 1}, {At: 3, Duration: 1}} }, ""},
		{"grade look", func(e *VideoEffects) { e.Grade.Look = "sepia" }, "unknown grade look"},
		{"grade intensity", func(e *VideoEffects) { e.Grade.Intensity = 200 }, "grade intensity 200 out of range"},
		{"too many freezes", func(e *VideoEffects) {
			e.Freeze.Freezes = nil
			for i := 0; i < 6; i++ {
				e.Freeze.Freezes = append(e.Freeze.Freezes, FreezeHold{At: float64(i * 10), Duration: 0.2})
			}
		}, "too many freezes"},
		{"freeze too long", func(e *VideoEffects) { e.Freeze.Freezes[0].Duration = 3.5 }, "freeze 1 duration 3.5 must be between 0.2 and 3 seconds"},
		{"freeze too short", func(e *VideoEffects) { e.Freeze.Freezes[0].Duration = 0.1 }, "freeze 1 duration 0.1 must be between 0.2 and 3 seconds"},
		{"freeze total too long", func(e *VideoEffects) {
			e.Freeze.Freezes = []FreezeHold{{At: 1, Duration: 3}, {At: 5, Duration: 3}, {At: 9, Duration: 3}, {At: 13, Duration: 3}}
		}, "add up to 12.0s"},
		{"freezes too close", func(e *VideoEffects) {
			e.Freeze.Freezes = []FreezeHold{{At: 4, Duration: 0.5}, {At: 4.05, Duration: 0.5}}
		}, "less than 0.1s apart"},
		{"freeze time out of range", func(e *VideoEffects) { e.Freeze.Freezes[1].At = 95 }, "freeze 2 time 95 must be between 0 and 90 seconds"},
		{"freeze intensity", func(e *VideoEffects) { e.Freeze.Intensity = -5 }, "freeze intensity -5 out of range"},
		{"impact text empty", func(e *VideoEffects) { e.Freeze.ImpactTexts[0].Text = "   " }, "impact text 1 is empty"},
		{"impact text too long", func(e *VideoEffects) {
			e.Freeze.ImpactTexts[0].Text = strings.Repeat("A", 17)
		}, "impact text 1 is longer than 16 characters"},
		{"impact text sixteen is fine", func(e *VideoEffects) {
			e.Freeze.ImpactTexts[0].Text = strings.Repeat("A", 16)
		}, ""},
		{"impact text newline", func(e *VideoEffects) { e.Freeze.ImpactTexts[0].Text = "LINE\nTWO" }, "must be one line"},
		{"impact text tab", func(e *VideoEffects) { e.Freeze.ImpactTexts[0].Text = "A\tB" }, "must be one line"},
		{"impact text carriage return", func(e *VideoEffects) { e.Freeze.ImpactTexts[0].Text = "A\rB" }, "must be one line"},
		{"impact text invalid UTF-8", func(e *VideoEffects) { e.Freeze.ImpactTexts[0].Text = "A\xffB" }, "not valid UTF-8"},
		{"impact text punctuation is fine", func(e *VideoEffects) {
			e.Freeze.ImpactTexts[0].Text = `'50%: [a];b\c,{}`
		}, ""},
		{"impact duration too short", func(e *VideoEffects) { e.Freeze.ImpactTexts[0].Duration = 0.2 }, "impact text 1 duration 0.2 must be between 0.3 and 4 seconds"},
		{"impact texts overlap", func(e *VideoEffects) {
			e.Freeze.ImpactTexts = []ImpactText{{At: 1, Duration: 2, Text: "ONE"}, {At: 2, Duration: 1, Text: "TWO"}}
		}, "impact texts overlap"},
		{"too many impact texts", func(e *VideoEffects) {
			e.Freeze.ImpactTexts = nil
			for i := 0; i < 6; i++ {
				e.Freeze.ImpactTexts = append(e.Freeze.ImpactTexts, ImpactText{At: float64(i * 5), Duration: 0.5, Text: "X"})
			}
		}, "too many impact texts"},
		{"fade in too long", func(e *VideoEffects) { e.Fade.InDuration = 2.5 }, "fade in duration 2.5 must be between 0 and 2 seconds"},
		{"fade out too long", func(e *VideoEffects) { e.Fade.OutDuration = 3 }, "fade out duration 3 must be between 0 and 2 seconds"},
		{"fade in negative", func(e *VideoEffects) { e.Fade.InDuration = -0.5 }, "fade in duration -0.5 must be between 0 and 2 seconds"},
		{"fade in NaN", func(e *VideoEffects) { e.Fade.InDuration = math.NaN() }, "fade in duration"},
		{"fade out infinite", func(e *VideoEffects) { e.Fade.OutDuration = math.Inf(1) }, "fade out duration"},
		{"fade enabled with both sides zero", func(e *VideoEffects) { e.Fade.InDuration = 0; e.Fade.OutDuration = 0 }, "fade needs a fade-in or fade-out"},
		{"fade disabled with both sides zero is fine", func(e *VideoEffects) { e.Fade.Enabled = false; e.Fade.InDuration = 0; e.Fade.OutDuration = 0 }, ""},
		{"fade in only is fine", func(e *VideoEffects) { e.Fade.OutDuration = 0 }, ""},
		{"fade at max is fine", func(e *VideoEffects) { e.Fade.InDuration = 2; e.Fade.OutDuration = 2 }, ""},
		// Speed. The fixture has freeze on, so speed cases turn freeze off first.
		{"speed valid", func(e *VideoEffects) { e.Freeze.Enabled = false; e.Speed.Enabled = true }, ""},
		{"speed enabled without windows", func(e *VideoEffects) { e.Freeze.Enabled = false; e.Speed.Enabled = true; e.Speed.Windows = nil }, "at least one window"},
		{"speed disabled without windows is fine", func(e *VideoEffects) { e.Speed.Windows = nil }, ""},
		{"too many speed windows", func(e *VideoEffects) {
			e.Freeze.Enabled = false
			e.Speed.Enabled = true
			e.Speed.Windows = nil
			for i := 0; i < 6; i++ {
				e.Speed.Windows = append(e.Speed.Windows, SpeedWindow{At: float64(i * 10), Duration: 1, Rate: 1.5})
			}
		}, "too many speed windows"},
		{"speed rate too low", func(e *VideoEffects) { e.Freeze.Enabled = false; e.Speed.Enabled = true; e.Speed.Windows[0].Rate = 0.1 }, "rate 0.1 must be between 0.25x and 4x"},
		{"speed rate too high", func(e *VideoEffects) { e.Freeze.Enabled = false; e.Speed.Enabled = true; e.Speed.Windows[0].Rate = 5 }, "rate 5 must be between 0.25x and 4x"},
		{"speed rate NaN", func(e *VideoEffects) { e.Freeze.Enabled = false; e.Speed.Enabled = true; e.Speed.Windows[0].Rate = math.NaN() }, "rate NaN must be between"},
		{"speed rate infinite", func(e *VideoEffects) { e.Freeze.Enabled = false; e.Speed.Enabled = true; e.Speed.Windows[0].Rate = math.Inf(1) }, "rate +Inf must be between"},
		{"speed start out of range", func(e *VideoEffects) { e.Freeze.Enabled = false; e.Speed.Enabled = true; e.Speed.Windows[0].At = 95 }, "start 95 must be between 0 and 90 seconds"},
		{"speed duration too short", func(e *VideoEffects) { e.Freeze.Enabled = false; e.Speed.Enabled = true; e.Speed.Windows[0].Duration = 0.1 }, "duration 0.1 must be between 0.3 and 90 seconds"},
		{"speed windows overlap", func(e *VideoEffects) {
			e.Freeze.Enabled = false
			e.Speed.Enabled = true
			e.Speed.Windows = []SpeedWindow{{At: 1, Duration: 3, Rate: 0.5}, {At: 2, Duration: 1, Rate: 2}}
		}, "speed windows overlap"},
		{"speed windows touching is fine", func(e *VideoEffects) {
			e.Freeze.Enabled = false
			e.Speed.Enabled = true
			e.Speed.Windows = []SpeedWindow{{At: 1, Duration: 1, Rate: 0.5}, {At: 2, Duration: 1, Rate: 2}}
		}, ""},
		{"freeze and speed both on is rejected", func(e *VideoEffects) { e.Speed.Enabled = true }, "freeze frame and speed cannot both be on"},
		{"speed on with freeze off is fine", func(e *VideoEffects) { e.Freeze.Enabled = false; e.Speed.Enabled = true }, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var e *VideoEffects
			if tc.mutate != nil {
				e = validEffects()
				tc.mutate(e)
			}
			err := e.Validate()
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("expected valid, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected error containing %q, got %q", tc.wantErr, err.Error())
			}
		})
	}
}

func TestVideoEffectsActive(t *testing.T) {
	var nilEffects *VideoEffects
	if nilEffects.Active() {
		t.Fatal("nil effects must be inactive")
	}
	off := &VideoEffects{Zoom: &ZoomEffect{Enabled: false, Intensity: 40}}
	if off.Active() {
		t.Fatal("a disabled effect must not be active")
	}
	if !validEffects().Active() {
		t.Fatal("enabled effects must be active")
	}
}
