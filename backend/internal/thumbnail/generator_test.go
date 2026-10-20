package thumbnail

import (
	"math"
	"math/rand"
	"testing"
)

func cand(idx int, t, total float64, det Detection) Candidate {
	return Candidate{
		Frame:     Frame{RawFrame: RawFrame{Index: idx, Time: t, Path: "frame.jpg"}, Metrics: Metrics{Edge: 0.1}},
		Detection: det, Detected: true, Total: total,
	}
}

func faceAt(cx, cy, w, h float64, emotion string) Detection {
	return Detection{Faces: []Face{{
		Box:        Box{Left: cx - w/2, Top: cy - h/2, Width: w, Height: h},
		Confidence: 99, Emotion: emotion, EmotionConf: 90, EyesOpen: true,
	}}}
}

func subjectDet() Detection {
	return Detection{Labels: []Label{{Name: "Laptop", Confidence: 95, Boxes: []Box{{Left: 0.3, Top: 0.3, Width: 0.4, Height: 0.4}}}}}
}

var testAnalysis = Analysis{Headline: "5 AI TOOLS", SecondaryText: "YOU NEED", BestFrameTime: -1}

func TestBuildPlansGivesThreeDifferentConcepts(t *testing.T) {
	cands := []Candidate{
		cand(0, 5, 70, faceAt(0.3, 0.4, 0.14, 0.26, "CALM")),
		cand(1, 25, 60, subjectDet()),
		cand(2, 45, 55, faceAt(0.7, 0.4, 0.12, 0.22, "HAPPY")),
		cand(3, 65, 50, Detection{}),
	}
	plans, err := BuildPlans(cands, testAnalysis, StyleViral, FormatLandscape)
	if err != nil {
		t.Fatal(err)
	}
	if len(plans) != 3 {
		t.Fatalf("got %d plans", len(plans))
	}
	wantConcepts := []Concept{ConceptFace, ConceptSubject, ConceptCreative}
	wantLayouts := []Layout{LayoutSide, LayoutBand, LayoutCard}
	frames := map[int]bool{}
	for i, p := range plans {
		if p.Concept != wantConcepts[i] || p.Layout != wantLayouts[i] {
			t.Errorf("plan %d = %s/%s, want %s/%s", i, p.Concept, p.Layout, wantConcepts[i], wantLayouts[i])
		}
		frames[p.Frame.Index] = true
		if p.Headline != "5 AI TOOLS" || p.Secondary != "YOU NEED" {
			t.Errorf("plan %d text = %q / %q", i, p.Headline, p.Secondary)
		}
	}
	if len(frames) != 3 {
		t.Errorf("with four candidates the three plans must use three different frames, got %v", frames)
	}
}

func TestBuildPlansPrefersExpressiveFaces(t *testing.T) {
	cands := []Candidate{
		cand(0, 5, 70, faceAt(0.3, 0.4, 0.14, 0.26, "CALM")),
		cand(1, 25, 66, faceAt(0.3, 0.4, 0.14, 0.26, "SURPRISED")),
		cand(2, 45, 40, Detection{}),
	}
	plans, _ := BuildPlans(cands, testAnalysis, StyleViral, FormatLandscape)
	if plans[0].Frame.Index != 1 {
		t.Errorf("face concept used frame %d; the surprised face (frame 1) should win over a calm one with a slightly higher score", plans[0].Frame.Index)
	}
}

func TestBuildPlansWithoutFacesStillGivesThreeVariants(t *testing.T) {
	cands := []Candidate{cand(0, 5, 70, subjectDet()), cand(1, 25, 60, Detection{}), cand(2, 45, 50, Detection{})}
	plans, err := BuildPlans(cands, testAnalysis, StyleTechnology, FormatLandscape)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range plans {
		if p.Concept == ConceptFace {
			t.Error("there is nobody on screen, so no plan may claim to be a face close-up")
		}
	}
	if plans[0].Concept != ConceptSubject || plans[1].Concept != ConceptSubject || plans[2].Concept != ConceptCreative {
		t.Errorf("concepts = %s %s %s", plans[0].Concept, plans[1].Concept, plans[2].Concept)
	}
	if plans[0].Layout == plans[1].Layout {
		t.Error("the two subject shots need different layouts")
	}
}

func TestBuildPlansIgnoresCutOffFaces(t *testing.T) {
	cut := Detection{Faces: []Face{{Box: Box{Left: 0, Top: 0.2, Width: 0.3, Height: 0.5}, Confidence: 99}}}
	plans, _ := BuildPlans([]Candidate{cand(0, 5, 80, cut), cand(1, 25, 40, Detection{})}, testAnalysis, StyleViral, FormatLandscape)
	if plans[0].Concept == ConceptFace {
		t.Error("a face cut off by the frame edge makes a poor close-up")
	}
}

func TestBuildPlansFromASingleFrameStaysDistinct(t *testing.T) {
	plans, err := BuildPlans([]Candidate{cand(0, 5, 70, faceAt(0.5, 0.4, 0.15, 0.28, "HAPPY"))}, testAnalysis, StyleViral, FormatLandscape)
	if err != nil {
		t.Fatal(err)
	}
	layouts := map[Layout]bool{}
	for _, p := range plans {
		layouts[p.Layout] = true
	}
	if len(layouts) != 3 {
		t.Errorf("one frame must still yield three layouts, got %v", layouts)
	}
}

func TestBuildPlansNeedsAFrame(t *testing.T) {
	if _, err := BuildPlans(nil, testAnalysis, StyleViral, FormatLandscape); CodeOf(err) != CodeNoSuitableFrames {
		t.Errorf("code = %q", CodeOf(err))
	}
}

func TestBuildPlansHonoursTheModelsBestFrame(t *testing.T) {
	cands := []Candidate{
		cand(0, 5, 62, Detection{}),
		cand(1, 25, 60, Detection{}),
		cand(2, 45, 59, Detection{}),
	}
	an := testAnalysis
	an.BestFrameTime = 44
	plans, _ := BuildPlans(cands, an, StyleViral, FormatLandscape)
	if plans[0].Frame.Index != 2 {
		t.Errorf("the model's pick (frame at 45s) should lead, got frame %d", plans[0].Frame.Index)
	}
}

func TestFacePlacement(t *testing.T) {
	left := cand(0, 5, 70, faceAt(0.25, 0.4, 0.14, 0.26, "HAPPY"))
	right := cand(0, 5, 70, faceAt(0.75, 0.4, 0.14, 0.26, "HAPPY"))
	pl, _ := BuildPlans([]Candidate{left}, testAnalysis, StyleViral, FormatLandscape)
	pr, _ := BuildPlans([]Candidate{right}, testAnalysis, StyleViral, FormatLandscape)
	if pl[0].Anchor[0] != 0.3 {
		t.Errorf("a face on the left should stay left of centre, anchor %v", pl[0].Anchor)
	}
	if pr[0].Anchor[0] != 0.7 {
		t.Errorf("a face on the right should stay right of centre, anchor %v", pr[0].Anchor)
	}
	pp, _ := BuildPlans([]Candidate{left}, testAnalysis, StyleViral, FormatPortrait)
	if pp[0].Anchor[1] < 0.55 {
		t.Errorf("in portrait the headline is on top, so the face belongs in the lower half: %v", pp[0].Anchor)
	}
}

func TestZoomForFace(t *testing.T) {
	if z := zoomForFace(Box{Height: 0.9}); z != 1 {
		t.Errorf("a face that already fills the frame needs no zoom, got %v", z)
	}
	if z := zoomForFace(Box{Height: 0.05}); z != maxZoom {
		t.Errorf("a tiny face is capped at %v, got %v", maxZoom, z)
	}
	if z := zoomForFace(Box{Height: 0.23}); math.Abs(z-2) > 0.01 {
		t.Errorf("zoom = %v, want about 2", z)
	}
	if z := zoomForFace(Box{}); z != 1 {
		t.Errorf("degenerate box zoom = %v", z)
	}
}

func TestCropRect(t *testing.T) {
	// Full-size window of the right shape.
	r := CropRect(1920, 1080, 16.0/9, 1, [2]float64{0.5, 0.5}, [2]float64{0.5, 0.5}, 720)
	if r.Dx() != 1920 || r.Dy() != 1080 {
		t.Errorf("full crop = %v", r)
	}
	// Vertical window from a landscape frame.
	r = CropRect(1920, 1080, 9.0/16, 1, [2]float64{0.5, 0.5}, [2]float64{0.5, 0.5}, 1280)
	if r.Dy() != 1080 || math.Abs(float64(r.Dx())-607.5) > 1 {
		t.Errorf("vertical crop = %v", r)
	}
	// Zoomed on the left third: the window must slide as far as the frame allows.
	r = CropRect(1920, 1080, 16.0/9, 2, [2]float64{0.1, 0.5}, [2]float64{0.5, 0.5}, 720)
	if r.Min.X != 0 || r.Dx() != 960 {
		t.Errorf("clamped crop = %v", r)
	}
	// Anchor: with the focus at x=0.5 and anchor x=0.3, the focus sits 30% into the crop.
	r = CropRect(3840, 2160, 16.0/9, 2, [2]float64{0.5, 0.5}, [2]float64{0.3, 0.5}, 720)
	focusPx := 0.5 * 3840
	if got := (focusPx - float64(r.Min.X)) / float64(r.Dx()); math.Abs(got-0.3) > 0.01 {
		t.Errorf("focus lands %.2f into the crop, want 0.30", got)
	}
}

func TestCropRectLimitsUpscaling(t *testing.T) {
	// A 1280x720 source zoomed 3x for a 720p thumbnail would stretch pixels
	// 3x; the zoom is held back to the upscale limit.
	r := CropRect(1280, 720, 16.0/9, 3, [2]float64{0.5, 0.5}, [2]float64{0.5, 0.5}, 720)
	if got := 720.0 / float64(r.Dy()); got > maxUpscale+0.05 {
		t.Errorf("crop upscaled %.2fx, limit %.1fx", got, maxUpscale)
	}
}

func TestCropRectStaysInsideTheFrame(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	for i := 0; i < 2000; i++ {
		w, h := 200+rng.Intn(3000), 200+rng.Intn(3000)
		aspect := []float64{16.0 / 9, 9.0 / 16, 1}[rng.Intn(3)]
		r := CropRect(w, h, aspect, 0.5+rng.Float64()*4, [2]float64{rng.Float64(), rng.Float64()}, [2]float64{rng.Float64(), rng.Float64()}, 720)
		if r.Min.X < 0 || r.Min.Y < 0 || r.Max.X > w || r.Max.Y > h || r.Empty() {
			t.Fatalf("crop %v outside %dx%d (aspect %.2f)", r, w, h, aspect)
		}
		// Integer rounding distorts very small crops; real ones are hundreds of pixels.
		if got := float64(r.Dx()) / float64(r.Dy()); r.Dy() >= 100 && math.Abs(got-aspect) > 0.03 {
			t.Fatalf("crop aspect %.3f, want %.3f", got, aspect)
		}
	}
	if !CropRect(0, 100, 1.7, 1, [2]float64{}, [2]float64{}, 720).Empty() {
		t.Error("an empty source should give an empty crop")
	}
}
