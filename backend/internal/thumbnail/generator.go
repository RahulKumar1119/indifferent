package thumbnail

import (
	"fmt"
	"image"
	"math"
	"strings"
)

// Layout is how the text and picture are arranged.
type Layout string

const (
	// LayoutSide puts the subject full-bleed on one side and the headline in a
	// column on the other (landscape), or the headline across the top (portrait).
	LayoutSide Layout = "side"
	// LayoutBand lays the headline over a gradient band along the bottom.
	LayoutBand Layout = "band"
	// LayoutCard puts the picture on a card over a blurred, graded backdrop.
	LayoutCard Layout = "card"
)

// Plan says exactly how to draw one thumbnail: which frame, where to crop,
// which layout and treatment. It holds no pixels, so it is cheap to build and
// easy to test.
type Plan struct {
	Concept   Concept
	Layout    Layout
	Frame     Candidate
	Focus     [2]float64 // normalised point in the frame to keep in view
	Anchor    [2]float64 // where that point should land in the crop, 0 to 1
	Zoom      float64    // 1 shows the widest crop; larger zooms in
	Enhance   bool       // push the grade harder (creative concept)
	Headline  string
	Secondary string
	Style     Style
	Format    Format
}

const (
	// faceTargetHeight is how tall a face should be inside the crop.
	faceTargetHeight = 0.46
	maxZoom          = 2.6
	// maxUpscale limits how much a crop may be enlarged before it looks soft.
	maxUpscale = 1.6
	// minFaceArea ignores specks that Rekognition sometimes reports.
	minFaceArea = 0.004
)

// emotionBonus rewards expressive faces: a calm face makes a weak thumbnail.
func emotionBonus(e string) float64 {
	switch strings.ToUpper(e) {
	case "SURPRISED", "FEAR":
		return 8
	case "HAPPY", "ANGRY":
		return 6
	case "DISGUSTED", "SAD", "CONFUSED":
		return 3
	}
	return 0
}

// BuildPlans composes three plans with different compositions. When the
// footage has a usable face the first is a face close-up; when it has none the
// first becomes a second subject shot so the set is still varied. Frames are
// not reused while another candidate remains.
func BuildPlans(cands []Candidate, an Analysis, style Style, format Format) ([]Plan, error) {
	if len(cands) == 0 {
		return nil, Errorf(CodeNoSuitableFrames, "We couldn't find a good frame to build thumbnails from.", nil)
	}
	used := map[int]bool{} // by candidate index
	best := bestFrameBias(cands, an)

	mk := func(concept Concept, layout Layout, idx int) Plan {
		return Plan{
			Concept:   concept,
			Layout:    layout,
			Frame:     cands[idx],
			Zoom:      1,
			Focus:     [2]float64{0.5, 0.5},
			Anchor:    [2]float64{0.5, 0.5},
			Headline:  an.Headline,
			Secondary: an.SecondaryText,
			Style:     style,
			Format:    format,
		}
	}

	// pick returns the best unused candidate by score(c), or the best overall
	// when every candidate is already in use.
	pick := func(score func(Candidate) float64, allow func(Candidate) bool) int {
		bestIdx, bestScore, found := -1, math.Inf(-1), false
		for pass := 0; pass < 2 && !found; pass++ {
			for i, c := range cands {
				if pass == 0 && used[i] {
					continue
				}
				if allow != nil && !allow(c) {
					continue
				}
				if s := score(c) + best[i]; s > bestScore {
					bestIdx, bestScore, found = i, s, true
				}
			}
		}
		return bestIdx
	}

	hasFace := func(c Candidate) bool {
		f, ok := c.Detection.LargestFace()
		return ok && f.Box.Area() >= minFaceArea && !f.Box.CutOff()
	}

	plans := make([]Plan, 0, 3)

	// 1. Face close-up (or a subject shot when nobody is on screen).
	if idx := pick(func(c Candidate) float64 {
		f, _ := c.Detection.LargestFace()
		return c.Total + emotionBonus(f.Emotion) + btoi(f.EyesOpen)*2
	}, hasFace); idx >= 0 && hasFace(cands[idx]) {
		used[idx] = true
		p := mk(ConceptFace, LayoutSide, idx)
		face, _ := cands[idx].Detection.LargestFace()
		framePlanFace(&p, face, cands[idx])
		plans = append(plans, p)
	} else {
		idx := pick(func(c Candidate) float64 { return c.Total }, nil)
		used[idx] = true
		p := mk(ConceptSubject, LayoutSide, idx)
		frameSubject(&p, cands[idx])
		if p.Focus[0] <= 0.5 {
			p.Anchor = [2]float64{0.3, 0.5}
		} else {
			p.Anchor = [2]float64{0.7, 0.5}
		}
		plans = append(plans, p)
	}

	// 2. Subject shot with the headline in a band.
	{
		idx := pick(func(c Candidate) float64 { return c.Total + c.SubjPts + c.ObjPts }, nil)
		used[idx] = true
		p := mk(ConceptSubject, LayoutBand, idx)
		frameSubject(&p, cands[idx])
		// The headline sits in a band along the bottom, so lift the subject
		// into the upper part of the picture instead of letting the text
		// cover it.
		// A vertical crop is already tight, so only landscape needs the zoom.
		if format != FormatPortrait {
			p.Zoom = math.Max(p.Zoom, 1.3)
		}
		p.Anchor = [2]float64{0.5, 0.40}
		plans = append(plans, p)
	}

	// 3. Creative: best remaining frame on an enhanced, blurred backdrop.
	{
		idx := pick(func(c Candidate) float64 { return c.Total + c.Edge*10 }, nil)
		used[idx] = true
		p := mk(ConceptCreative, LayoutCard, idx)
		p.Enhance = true
		frameSubject(&p, cands[idx])
		plans = append(plans, p)
	}
	return plans, nil
}

// bestFrameBias gives a small bonus to the candidate nearest the moment the
// model called the most important, so the AI's read of the video counts for
// something beyond the local score.
func bestFrameBias(cands []Candidate, an Analysis) []float64 {
	bias := make([]float64, len(cands))
	if an.BestFrameTime < 0 || len(cands) == 0 {
		return bias
	}
	nearest, gap := 0, math.Inf(1)
	for i, c := range cands {
		if d := math.Abs(c.Time - an.BestFrameTime); d < gap {
			nearest, gap = i, d
		}
	}
	bias[nearest] = 8
	return bias
}

func btoi(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

// framePlanFace crops tight on the face and parks it on the side opposite the
// headline: a face left of centre goes in the left third and the text takes
// the right, and the other way round.
func framePlanFace(p *Plan, face Face, c Candidate) {
	p.Focus = [2]float64{face.Box.CenterX(), face.Box.CenterY()}
	if p.Format == FormatPortrait {
		p.Anchor = [2]float64{0.5, 0.62} // headline sits above the face
	} else if face.Box.CenterX() <= 0.5 {
		p.Anchor = [2]float64{0.3, 0.5}
	} else {
		p.Anchor = [2]float64{0.7, 0.5}
	}
	p.Zoom = zoomForFace(face.Box)
}

// frameSubject centres on the biggest subject in the frame. A frame with a
// person and nothing else uses the person; one with nothing detected is
// centred.
func frameSubject(p *Plan, c Candidate) {
	p.Zoom = 1
	if box, _, ok := c.Detection.LargestSubject(); ok && box.Area() > 0.02 {
		p.Focus = [2]float64{box.CenterX(), box.CenterY()}
		return
	}
	if f, ok := c.Detection.LargestFace(); ok {
		p.Focus = [2]float64{f.Box.CenterX(), f.Box.CenterY()}
	}
}

// zoomForFace is the zoom that makes the face about faceTargetHeight of the
// crop height, within sane limits.
func zoomForFace(b Box) float64 {
	if b.Height <= 0 {
		return 1
	}
	z := faceTargetHeight / b.Height
	if z < 1 {
		z = 1
	}
	if z > maxZoom {
		z = maxZoom
	}
	return z
}

// CropRect returns the source rectangle to show for a plan. It takes the
// largest window of the target aspect, narrows it by zoom, and places it so
// the focus point lands at the anchor, staying inside the frame. The zoom is
// reduced if it would enlarge the pixels more than maxUpscale for the given
// output height.
func CropRect(srcW, srcH int, aspect, zoom float64, focus, anchor [2]float64, outH int) image.Rectangle {
	if srcW <= 0 || srcH <= 0 || aspect <= 0 {
		return image.Rectangle{}
	}
	baseW, baseH := float64(srcW), float64(srcH)
	if baseW/baseH > aspect {
		baseW = baseH * aspect
	} else {
		baseH = baseW / aspect
	}
	if zoom < 1 {
		zoom = 1
	}
	if outH > 0 {
		if limit := baseH * maxUpscale / float64(outH); zoom > limit && limit >= 1 {
			zoom = limit
		}
	}
	w, h := baseW/zoom, baseH/zoom
	x := focus[0]*float64(srcW) - anchor[0]*w
	y := focus[1]*float64(srcH) - anchor[1]*h
	x = math.Max(0, math.Min(x, float64(srcW)-w))
	y = math.Max(0, math.Min(y, float64(srcH)-h))
	r := image.Rect(int(math.Round(x)), int(math.Round(y)), int(math.Round(x+w)), int(math.Round(y+h)))
	return r.Intersect(image.Rect(0, 0, srcW, srcH))
}

// Describe is a short human summary of a plan for logs.
func (p Plan) Describe() string {
	return fmt.Sprintf("%s/%s frame@%.1fs zoom %.2f", p.Concept, p.Layout, p.Frame.Time, p.Zoom)
}
