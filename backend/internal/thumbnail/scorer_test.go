package thumbnail

import (
	"image"
	"image/color"
	"testing"

	xdraw "golang.org/x/image/draw"
)

func frameFrom(img image.Image, idx int) Frame {
	return Frame{RawFrame: RawFrame{Index: idx, Time: float64(idx) * 5}, Metrics: Measure(img)}
}

func TestMeasureBasics(t *testing.T) {
	black := Measure(solid(320, 180, color.RGBA{A: 255}))
	if black.Brightness > 0.01 || black.DarkRatio < 0.99 {
		t.Errorf("black frame: %+v", black)
	}
	white := Measure(solid(320, 180, color.RGBA{R: 255, G: 255, B: 255, A: 255}))
	if white.Brightness < 0.99 {
		t.Errorf("white frame brightness = %v", white.Brightness)
	}
	sharp := Measure(noiseImage(640, 360, 1))
	soft := Measure(blurred(noiseImage(640, 360, 1)))
	if sharp.Sharpness < 100*soft.Sharpness+1 {
		t.Errorf("sharpness should separate crisp from soft: %v vs %v", sharp.Sharpness, soft.Sharpness)
	}
	if sharp.Edge <= soft.Edge {
		t.Errorf("edge share should be higher for the crisp frame: %v vs %v", sharp.Edge, soft.Edge)
	}
}

func TestMeasureIsIndependentOfResolution(t *testing.T) {
	big := scene(1920, 1080, 4)
	small := image.NewRGBA(image.Rect(0, 0, 640, 360))
	xdraw.ApproxBiLinear.Scale(small, small.Bounds(), big, big.Bounds(), xdraw.Src, nil)
	a, b := Measure(small), Measure(big)
	if d := a.Brightness - b.Brightness; d < -0.03 || d > 0.03 {
		t.Errorf("brightness differs across resolutions: %v vs %v", a.Brightness, b.Brightness)
	}
	if HashDistance(a.Hash, b.Hash) > 8 {
		t.Errorf("same scene hashes %d bits apart across resolutions", HashDistance(a.Hash, b.Hash))
	}
}

func TestFilterRejectsBadFrames(t *testing.T) {
	good := scene(640, 360, 1)
	gray := solid(640, 360, color.RGBA{R: 128, G: 128, B: 128, A: 255})
	tests := []struct {
		name string
		img  image.Image
		want string
	}{
		{"black", solid(640, 360, color.RGBA{A: 255}), RejectBlack},
		{"very dark", darkened(scene(640, 360, 5), 0.2), RejectDark},
		{"all black", solid(640, 360, color.RGBA{R: 14, G: 14, B: 14, A: 255}), RejectBlack},
		{"blown out", solid(640, 360, color.RGBA{R: 252, G: 252, B: 252, A: 255}), RejectBright},
		{"flat", gray, RejectFlat},
		{"blurry", blurred(noiseImage(640, 360, 9)), RejectBlurry},
		{"good", good, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frames := []Frame{frameFrom(tt.img, 0)}
			usable := Filter(frames)
			if frames[0].Reject != tt.want {
				t.Errorf("reject = %q, want %q (metrics %+v)", frames[0].Reject, tt.want, frames[0].Metrics)
			}
			if (tt.want == "") != (len(usable) == 1) {
				t.Errorf("usable = %d for reject %q", len(usable), tt.want)
			}
		})
	}
}

func TestFilterDetectsMotionBlurWithinAScene(t *testing.T) {
	// The same scene three times; the middle frame is much softer, as if the
	// camera had moved.
	crisp := noiseImage(640, 360, 2)
	soft := blurredLightly(crisp)
	frames := []Frame{frameFrom(crisp, 0), frameFrom(soft, 1), frameFrom(crisp, 2)}
	if frames[1].Sharpness*motionBlurRatio >= frames[0].Sharpness {
		t.Fatalf("fixture not soft enough: %v vs %v", frames[1].Sharpness, frames[0].Sharpness)
	}
	if d := HashDistance(frames[0].Hash, frames[1].Hash); d > sameSceneBits {
		t.Fatalf("fixture hashes differ by %d bits, so it is not the same scene", d)
	}
	Filter(frames)
	if frames[1].Reject != RejectBlurry {
		t.Errorf("soft middle frame reject = %q, want blurry", frames[1].Reject)
	}
	if frames[0].Reject != "" || frames[2].Reject != "" {
		t.Errorf("sharp frames wrongly rejected: %q %q", frames[0].Reject, frames[2].Reject)
	}
}

func darkened(src *image.RGBA, k float64) *image.RGBA {
	out := image.NewRGBA(src.Bounds())
	for i := 0; i < len(src.Pix); i += 4 {
		out.Pix[i] = uint8(float64(src.Pix[i]) * k)
		out.Pix[i+1] = uint8(float64(src.Pix[i+1]) * k)
		out.Pix[i+2] = uint8(float64(src.Pix[i+2]) * k)
		out.Pix[i+3] = 255
	}
	return out
}

// blurredLightly softens a scene without wiping out its structure.
func blurredLightly(src *image.RGBA) *image.RGBA {
	b := src.Bounds()
	out := image.NewRGBA(b)
	for y := 1; y < b.Dy()-1; y++ {
		for x := 1; x < b.Dx()-1; x++ {
			var r, g, bl int
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					c := src.RGBAAt(x+dx, y+dy)
					r, g, bl = r+int(c.R), g+int(c.G), bl+int(c.B)
				}
			}
			out.SetRGBA(x, y, color.RGBA{R: uint8(r / 9), G: uint8(g / 9), B: uint8(bl / 9), A: 255})
		}
	}
	return out
}

func TestFilterDoesNotPenaliseCalmScenes(t *testing.T) {
	// A smooth gradient with one shape is soft, but there is no sharper
	// neighbour from the same scene, so it must survive.
	calm := scene(640, 360, 6)
	busy := noiseImage(640, 360, 3)
	frames := []Frame{frameFrom(busy, 0), frameFrom(calm, 1), frameFrom(busy, 2)}
	Filter(frames)
	if frames[1].Reject != "" {
		t.Errorf("calm scene rejected as %q", frames[1].Reject)
	}
}

func TestFilterFlagsTransitionFlash(t *testing.T) {
	steady := func(idx int, brightness float64) Frame {
		return Frame{
			RawFrame: RawFrame{Index: idx},
			Metrics:  Metrics{Brightness: brightness, Contrast: 0.2, Sharpness: 200, Edge: 0.1, Hash: uint64(idx) * 0x0101010101010101},
		}
	}
	// Dark, bright flash, dark again: neighbours alike, middle far from both.
	frames := []Frame{steady(0, 0.35), steady(1, 0.80), steady(2, 0.36)}
	usable := Filter(frames)
	if frames[1].Reject != RejectTransition {
		t.Errorf("flash reject = %q, want transition", frames[1].Reject)
	}
	if len(usable) != 2 {
		t.Errorf("usable = %d, want 2", len(usable))
	}
	// A real change of scene (neighbours differ) is not a flash.
	frames = []Frame{steady(0, 0.30), steady(1, 0.55), steady(2, 0.80)}
	Filter(frames)
	if frames[1].Reject != "" {
		t.Errorf("a gradual change was flagged as %q", frames[1].Reject)
	}
}

func TestDedupeKeepsBestOfNearDuplicates(t *testing.T) {
	a := scene(640, 360, 11)
	// A copy with slightly different brightness is a near duplicate.
	a2 := image.NewRGBA(a.Bounds())
	copy(a2.Pix, a.Pix)
	for i := 0; i < len(a2.Pix); i += 4 {
		a2.Pix[i] = uint8(min(255, int(a2.Pix[i])+6))
	}
	other := scene(640, 360, 12)

	frames := []Frame{frameFrom(a, 0), frameFrom(a2, 1), frameFrom(other, 2)}
	frames[0].Local, frames[1].Local, frames[2].Local = 20, 35, 10
	unique := Dedupe(frames, 6)
	if len(unique) != 2 {
		t.Fatalf("got %d unique frames, want 2", len(unique))
	}
	var kept []int
	for _, f := range unique {
		kept = append(kept, f.Index)
	}
	if kept[0] != 1 || kept[1] != 2 {
		t.Errorf("kept %v, want [1 2] (the higher-scoring duplicate and the different scene, in time order)", kept)
	}
	if frames[0].Reject != RejectDuplicate {
		t.Errorf("dropped frame reject = %q", frames[0].Reject)
	}
}

func TestScoreLocalPrefersSharpWellLitFrames(t *testing.T) {
	crisp := frameFrom(scene(640, 360, 20), 0)
	dim := crisp
	dim.Brightness = 0.14
	soft := crisp
	soft.Sharpness = crisp.Sharpness * 0.05
	frames := []Frame{crisp, dim, soft}
	ScoreLocal(frames)
	if !(frames[0].Local > frames[1].Local && frames[0].Local > frames[2].Local) {
		t.Errorf("scores crisp=%.1f dim=%.1f soft=%.1f", frames[0].Local, frames[1].Local, frames[2].Local)
	}
	for _, f := range frames {
		if f.Local < 0 || f.Local > wSharpness+wLighting+wContrast+wComposition+0.001 {
			t.Errorf("local score %.1f outside 0..50", f.Local)
		}
	}
}

func TestShortlistOrdersByScore(t *testing.T) {
	frames := []Frame{{Local: 5}, {Local: 40}, {Local: 22}, {Local: 31}}
	got := Shortlist(frames, 2)
	if len(got) != 2 || got[0].Local != 40 || got[1].Local != 31 {
		t.Errorf("shortlist = %+v", got)
	}
}

func TestScoreWeightsMatchTheIssue(t *testing.T) {
	// face 25 + subject 15 + sharpness 20 + lighting 10 + contrast 10 +
	// object 10 + composition 10 = 100
	total := wFace + wSubject + wSharpness + wLighting + wContrast + wObject + wComposition
	if total != 100 {
		t.Errorf("weights add up to %v, want 100", total)
	}
}

func TestScoreCandidateRewardsFacesAndPenalisesCutOff(t *testing.T) {
	base := Frame{Metrics: Metrics{Brightness: 0.5}, Local: 30}
	none := ScoreCandidate(base, Detection{}, true)
	withFace := ScoreCandidate(base, Detection{Faces: []Face{{Box: Box{Left: 0.4, Top: 0.2, Width: 0.2, Height: 0.4}}}}, true)
	cut := ScoreCandidate(base, Detection{Faces: []Face{{Box: Box{Left: 0.0, Top: 0.2, Width: 0.2, Height: 0.4}}}}, true)
	tiny := ScoreCandidate(base, Detection{Faces: []Face{{Box: Box{Left: 0.4, Top: 0.2, Width: 0.03, Height: 0.04}}}}, true)

	if withFace.Total <= none.Total+wFace-1 {
		t.Errorf("a clear face should add about %v points: %.1f vs %.1f", wFace, withFace.Total, none.Total)
	}
	if cut.Total >= withFace.Total || !cut.CutOff {
		t.Errorf("a cut-off face must score lower and be flagged: cut=%.1f ok=%.1f flag=%v", cut.Total, withFace.Total, cut.CutOff)
	}
	if tiny.FacePts >= withFace.FacePts {
		t.Errorf("a tiny face should earn fewer points: %v vs %v", tiny.FacePts, withFace.FacePts)
	}
	if withFace.Total > 100 {
		t.Errorf("total %.1f exceeds 100", withFace.Total)
	}
}

func TestScoreCandidateUsesObjectsWhenNoFace(t *testing.T) {
	base := Frame{Local: 30}
	det := Detection{Labels: []Label{
		{Name: "Laptop", Confidence: 98, Boxes: []Box{{Left: 0.2, Top: 0.3, Width: 0.5, Height: 0.5}}},
		{Name: "Desk", Confidence: 91},
		{Name: "Monitor", Confidence: 90},
		{Name: "Person", Confidence: 99}, // generic: should not count
	}}
	c := ScoreCandidate(base, det, true)
	if c.SubjPts <= 0 || c.ObjPts <= 0 {
		t.Errorf("object frame earned no subject/object points: %+v", c)
	}
	if got := det.InterestingLabels(); got != 3 {
		t.Errorf("InterestingLabels = %d, want 3 (Person is generic)", got)
	}
	skipped := ScoreCandidate(base, Detection{}, false)
	if skipped.Total != base.Local {
		t.Errorf("undetected frame should keep its local score, got %v", skipped.Total)
	}
}

func TestRankCandidates(t *testing.T) {
	c := []Candidate{{Total: 10}, {Total: 90}, {Total: 55}}
	RankCandidates(c)
	if c[0].Total != 90 || c[2].Total != 10 {
		t.Errorf("ranked = %+v", c)
	}
}

func TestBoxCutOff(t *testing.T) {
	tests := []struct {
		b    Box
		want bool
	}{
		{Box{Left: 0.3, Top: 0.2, Width: 0.3, Height: 0.4}, false},
		{Box{Left: 0, Top: 0.2, Width: 0.3, Height: 0.4}, true},
		{Box{Left: 0.3, Top: 0.2, Width: 0.71, Height: 0.4}, true},
		{Box{Left: 0.3, Top: 0.7, Width: 0.3, Height: 0.31}, true},
	}
	for i, tt := range tests {
		if tt.b.CutOff() != tt.want {
			t.Errorf("case %d: CutOff = %v, want %v", i, !tt.want, tt.want)
		}
	}
}
