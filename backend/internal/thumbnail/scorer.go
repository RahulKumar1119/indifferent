package thumbnail

import (
	"image"
	"image/color"
	"math"
	"math/bits"
	"os"
	"sort"

	_ "image/jpeg" // decoders for frames on disk
	_ "image/png"
)

// Why a frame was thrown away. Kept as strings so tests and logs read well.
const (
	RejectBlack      = "black"
	RejectDark       = "dark"
	RejectBright     = "bright"
	RejectFlat       = "flat"
	RejectBlurry     = "blurry"
	RejectTransition = "transition"
	RejectDuplicate  = "duplicate"
)

// workWidth is the width frames are reduced to for measuring. Measuring on a
// fixed small grid makes the numbers independent of the source resolution and
// keeps scoring fast.
const workWidth = 320

// Metrics are cheap, local measurements of one frame.
type Metrics struct {
	Brightness float64 // mean luma, 0 to 1
	Contrast   float64 // standard deviation of luma, 0 to 1
	Sharpness  float64 // variance of the Laplacian on the work grid
	Edge       float64 // share of pixels on a strong edge, 0 to 1
	DarkRatio  float64 // share of near-black pixels, 0 to 1
	Hash       uint64  // 64-bit difference hash for near-duplicate detection
}

// Frame is an extracted frame with its measurements.
type Frame struct {
	RawFrame
	Metrics
	Reject string  // empty when the frame is usable
	Local  float64 // local score, 0 to 50
}

// grayGrid reduces img to a grayscale grid roughly workWidth wide.
func grayGrid(img image.Image) (g []float64, w, h int) {
	b := img.Bounds()
	sw, sh := b.Dx(), b.Dy()
	if sw == 0 || sh == 0 {
		return nil, 0, 0
	}
	w = workWidth
	if sw < w {
		w = sw
	}
	h = sh * w / sw
	if h < 1 {
		h = 1
	}
	g = make([]float64, w*h)
	yc, isYCbCr := img.(*image.YCbCr)
	for y := 0; y < h; y++ {
		y0 := b.Min.Y + y*sh/h
		y1 := b.Min.Y + (y+1)*sh/h
		if y1 <= y0 {
			y1 = y0 + 1
		}
		for x := 0; x < w; x++ {
			x0 := b.Min.X + x*sw/w
			x1 := b.Min.X + (x+1)*sw/w
			if x1 <= x0 {
				x1 = x0 + 1
			}
			// Average a 3x3 sample of the block rather than every pixel.
			var sum float64
			var n int
			for sy := 0; sy < 3; sy++ {
				py := y0 + (y1-y0)*sy/3
				for sx := 0; sx < 3; sx++ {
					px := x0 + (x1-x0)*sx/3
					if isYCbCr {
						sum += float64(yc.Y[yc.YOffset(px, py)])
					} else {
						sum += float64(color.GrayModel.Convert(img.At(px, py)).(color.Gray).Y)
					}
					n++
				}
			}
			g[y*w+x] = sum / float64(n)
		}
	}
	return g, w, h
}

// Measure computes the metrics for one decoded image.
func Measure(img image.Image) Metrics {
	g, w, h := grayGrid(img)
	if w == 0 {
		return Metrics{}
	}
	var sum, sumSq float64
	dark := 0
	for _, v := range g {
		sum += v
		sumSq += v * v
		if v < 16 {
			dark++
		}
	}
	n := float64(len(g))
	mean := sum / n
	variance := sumSq/n - mean*mean
	if variance < 0 {
		variance = 0
	}

	// Laplacian variance (sharpness) and a gradient-based edge share.
	var lapSum, lapSumSq float64
	lapN := 0
	edges := 0
	for y := 1; y < h-1; y++ {
		for x := 1; x < w-1; x++ {
			c := g[y*w+x]
			lap := 4*c - g[y*w+x-1] - g[y*w+x+1] - g[(y-1)*w+x] - g[(y+1)*w+x]
			lapSum += lap
			lapSumSq += lap * lap
			lapN++
			gx := g[y*w+x+1] - g[y*w+x-1]
			gy := g[(y+1)*w+x] - g[(y-1)*w+x]
			if math.Hypot(gx, gy) > 40 {
				edges++
			}
		}
	}
	m := Metrics{
		Brightness: mean / 255,
		Contrast:   math.Sqrt(variance) / 255,
		DarkRatio:  float64(dark) / n,
		Hash:       diffHash(g, w, h),
	}
	if lapN > 0 {
		lm := lapSum / float64(lapN)
		m.Sharpness = lapSumSq/float64(lapN) - lm*lm
		m.Edge = float64(edges) / float64(lapN)
	}
	return m
}

// diffHash builds a 64-bit dHash: shrink to 9x8 and compare neighbours.
func diffHash(g []float64, w, h int) uint64 {
	const hw, hh = 9, 8
	small := make([]float64, hw*hh)
	for y := 0; y < hh; y++ {
		for x := 0; x < hw; x++ {
			x0, x1 := x*w/hw, (x+1)*w/hw
			y0, y1 := y*h/hh, (y+1)*h/hh
			if x1 <= x0 {
				x1 = x0 + 1
			}
			if y1 <= y0 {
				y1 = y0 + 1
			}
			var s float64
			c := 0
			for yy := y0; yy < y1 && yy < h; yy++ {
				for xx := x0; xx < x1 && xx < w; xx++ {
					s += g[yy*w+xx]
					c++
				}
			}
			if c > 0 {
				small[y*hw+x] = s / float64(c)
			}
		}
	}
	var hash uint64
	bit := uint(0)
	for y := 0; y < hh; y++ {
		for x := 0; x < hw-1; x++ {
			if small[y*hw+x] > small[y*hw+x+1] {
				hash |= 1 << bit
			}
			bit++
		}
	}
	return hash
}

// HashDistance is the number of differing bits between two frame hashes.
func HashDistance(a, b uint64) int { return bits.OnesCount64(a ^ b) }

// LoadImage decodes a frame from disk.
func LoadImage(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	return img, err
}

// MeasureFrames decodes and measures each extracted frame. Frames that fail
// to decode are skipped.
func MeasureFrames(raw []RawFrame, load func(string) (image.Image, error)) []Frame {
	if load == nil {
		load = LoadImage
	}
	out := make([]Frame, 0, len(raw))
	for _, r := range raw {
		img, err := load(r.Path)
		if err != nil {
			continue
		}
		out = append(out, Frame{RawFrame: r, Metrics: Measure(img)})
	}
	return out
}

// Thresholds for rejecting a frame outright. They are deliberately gentle:
// the scorer already prefers sharp, well-lit frames, so filtering only has to
// remove frames nobody would put on a thumbnail.
const (
	blackMean       = 0.07
	darkMean        = 0.10
	darkRatioMax    = 0.85
	brightMean      = 0.93
	flatContrast    = 0.035
	absSharpFloor   = 6.0  // below this a frame is blurry whatever the rest look like
	motionBlurRatio = 3.0  // a neighbouring frame of the same scene this much sharper marks motion blur
	sameSceneBits   = 16   // frames this close in hash distance count as the same scene
	transitionJump  = 0.30 // brightness swing that marks a flash or dip frame
	transitionClose = 0.10 // how alike the frames either side of a flash must be
)

// Filter marks unusable frames (black, dark, blown out, flat, blurry, or a
// transition) with a Reject reason and returns the usable ones in order. Blur
// and transitions are judged against neighbouring frames, so it needs the
// whole sequence. Blur is judged within a scene on purpose: a calm shot of a
// person is soft compared with a screen recording but is not blurry.
func Filter(frames []Frame) []Frame {
	if len(frames) == 0 {
		return nil
	}
	for i := range frames {
		f := &frames[i]
		switch {
		case f.Brightness < blackMean && f.Contrast < flatContrast:
			f.Reject = RejectBlack
		case f.Brightness < darkMean || f.DarkRatio > darkRatioMax:
			f.Reject = RejectDark
		case f.Brightness > brightMean:
			f.Reject = RejectBright
		case f.Contrast < flatContrast:
			f.Reject = RejectFlat
		case f.Sharpness < absSharpFloor:
			f.Reject = RejectBlurry
		}
	}

	// Motion blur: much softer than a neighbour showing the same scene.
	for i := range frames {
		if frames[i].Reject != "" {
			continue
		}
		for _, j := range []int{i - 1, i + 1} {
			if j < 0 || j >= len(frames) {
				continue
			}
			n := frames[j]
			if n.Reject == RejectBlack || n.Reject == RejectDark {
				continue
			}
			if HashDistance(frames[i].Hash, n.Hash) <= sameSceneBits && n.Sharpness > motionBlurRatio*frames[i].Sharpness {
				frames[i].Reject = RejectBlurry
				break
			}
		}
	}

	// A transition frame (flash, dip to white, fade) sits between two frames
	// that look alike but differ sharply from it.
	for i := 1; i < len(frames)-1; i++ {
		if frames[i].Reject != "" {
			continue
		}
		prev, cur, next := frames[i-1], frames[i], frames[i+1]
		alike := math.Abs(prev.Brightness-next.Brightness) < transitionClose
		jump := math.Abs(cur.Brightness-prev.Brightness) > transitionJump && math.Abs(cur.Brightness-next.Brightness) > transitionJump
		if alike && jump {
			frames[i].Reject = RejectTransition
		}
	}

	usable := make([]Frame, 0, len(frames))
	for _, f := range frames {
		if f.Reject == "" {
			usable = append(usable, f)
		}
	}
	return usable
}

// Local score weights. Together with the detection weights below they add up
// to the 100 points described in the issue.
const (
	wSharpness    = 20.0
	wLighting     = 10.0
	wContrast     = 10.0
	wComposition  = 10.0 // visual interest: how much of the frame has structure
	wFace         = 25.0
	wSubject      = 15.0
	wObject       = 10.0
	faceFullArea  = 0.015 // a face this big earns the full face points
	faceLargeArea = 0.08  // a face this big earns the full subject points
	subjectLarge  = 0.25  // a non-face subject this big earns the full subject points
	objectsFull   = 4     // distinct specific labels for the full object points
	cutOffFactor  = 0.4   // multiplier when the subject is cut off by the frame edge
)

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// sharpReference returns the sharpness a frame needs to earn full sharpness
// points: the 90th percentile of the video's usable frames, so scoring adapts
// to footage that is soft overall instead of marking everything down.
func sharpReference(frames []Frame) float64 {
	if len(frames) == 0 {
		return 1
	}
	s := make([]float64, 0, len(frames))
	for _, f := range frames {
		s = append(s, f.Sharpness)
	}
	sort.Float64s(s)
	ref := s[int(float64(len(s)-1)*0.9)]
	if ref < absSharpFloor {
		ref = absSharpFloor
	}
	return ref
}

// ScoreLocal fills in the local score (sharpness, lighting, contrast and
// composition, out of 50) for each frame.
func ScoreLocal(frames []Frame) {
	ref := sharpReference(frames)
	for i := range frames {
		f := &frames[i]
		lighting := 1 - clamp01(math.Abs(f.Brightness-0.5)/0.4)
		f.Local = wSharpness*clamp01(f.Sharpness/ref) +
			wLighting*lighting +
			wContrast*clamp01(f.Contrast/0.25) +
			wComposition*clamp01(f.Edge/0.12)
	}
}

// Dedupe drops near-duplicate frames, keeping the better one of each group.
// Frames must already have local scores. Two frames are duplicates when their
// hashes differ by at most maxDistance bits.
func Dedupe(frames []Frame, maxDistance int) []Frame {
	order := make([]int, len(frames))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool { return frames[order[a]].Local > frames[order[b]].Local })

	var kept []int
	for _, idx := range order {
		dup := false
		for _, k := range kept {
			if HashDistance(frames[idx].Hash, frames[k].Hash) <= maxDistance {
				dup = true
				break
			}
		}
		if dup {
			frames[idx].Reject = RejectDuplicate
			continue
		}
		kept = append(kept, idx)
	}
	sort.Ints(kept)
	out := make([]Frame, 0, len(kept))
	for _, k := range kept {
		out = append(out, frames[k])
	}
	return out
}

// Shortlist returns the n best frames by local score, best first.
func Shortlist(frames []Frame, n int) []Frame {
	out := append([]Frame(nil), frames...)
	sort.SliceStable(out, func(a, b int) bool { return out[a].Local > out[b].Local })
	if len(out) > n {
		out = out[:n]
	}
	return out
}

// Candidate is a shortlisted frame together with what the vision service saw
// and its final score.
type Candidate struct {
	Frame
	Detection Detection
	Detected  bool // detection ran for this frame
	Total     float64
	FacePts   float64
	SubjPts   float64
	ObjPts    float64
	CutOff    bool // the main subject touches the frame edge
}

// ScoreCandidate adds the detection points to a frame's local score.
func ScoreCandidate(f Frame, det Detection, detected bool) Candidate {
	c := Candidate{Frame: f, Detection: det, Detected: detected}
	if detected {
		if face, ok := det.LargestFace(); ok {
			a := face.Box.Area()
			c.FacePts = wFace * clamp01(a/faceFullArea)
			c.SubjPts = wSubject * clamp01(a/faceLargeArea)
			if face.Box.CutOff() {
				c.CutOff = true
				c.FacePts *= cutOffFactor
				c.SubjPts *= cutOffFactor
			}
		} else if box, _, ok := det.LargestSubject(); ok {
			c.SubjPts = wSubject * clamp01(box.Area()/subjectLarge)
			if box.CutOff() {
				c.CutOff = true
				c.SubjPts *= cutOffFactor
			}
		}
		c.ObjPts = wObject * clamp01(float64(det.InterestingLabels())/objectsFull)
	}
	c.Total = f.Local + c.FacePts + c.SubjPts + c.ObjPts
	return c
}

// RankCandidates sorts candidates by total score, best first.
func RankCandidates(c []Candidate) {
	sort.SliceStable(c, func(a, b int) bool { return c[a].Total > c[b].Total })
}
