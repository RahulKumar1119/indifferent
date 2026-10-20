package thumbnail

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/jpeg"
	"os"
	"path/filepath"
	"sync"
	"time"

	xdraw "golang.org/x/image/draw"
)

// Input is one generation request, with the video already on local disk.
type Input struct {
	VideoPath  string
	WorkDir    string  // scratch space for extracted frames
	Duration   float64 // the job's recorded duration, used if the file reports none
	Transcript string  // plain transcript text; empty when unavailable
	Style      Style   // the user's choice, or "" to let the analysis suggest one
	Format     Format  // the user's choice, or "" to match the video's shape
	Brand      string  // optional channel name drawn as a small tag
}

// Image is one finished thumbnail, ready to upload.
type Image struct {
	Concept       Concept
	Layout        Layout
	JPEG          []byte
	Width, Height int
	FrameTime     float64
	Headline      string
	SecondaryText string
}

// Output is the result of a run.
type Output struct {
	Images         []Image
	Analysis       Analysis
	Funnel         Funnel
	Style          Style
	Format         Format
	Video          VideoInfo
	TranscriptUsed bool
	// DetectionUsed is false when face and object detection was unavailable
	// and frames were chosen from local measurements alone.
	DetectionUsed bool
}

// Service runs the thumbnail pipeline. Detector may be nil, in which case
// frames are chosen from local measurements only.
type Service struct {
	Runner   Runner
	Detector Detector
	Analyzer Analyzer
	// Progress is told which stage is starting. May be nil.
	Progress func(Stage)
	// Logf receives diagnostic lines. May be nil.
	Logf func(format string, args ...any)

	// Tunables, with defaults applied when zero.
	ShortlistSize  int // frames sent to detection (default 10)
	PlanPool       int // best candidates considered for the three concepts (default 6)
	AnalysisFrames int // frames shown to the AI (default 4)
	DedupeDistance int // max differing bits for near-duplicates (default 6)
}

func (s *Service) logf(format string, args ...any) {
	if s.Logf != nil {
		s.Logf(format, args...)
	}
}

func (s *Service) stage(st Stage) {
	if s.Progress != nil {
		s.Progress(st)
	}
}

func orDefault(v, d int) int {
	if v > 0 {
		return v
	}
	return d
}

// Run produces three thumbnails for the video, or a classified error.
func (s *Service) Run(ctx context.Context, in Input) (*Output, error) {
	runner := s.Runner
	if runner == nil {
		runner = ExecRunner{}
	}
	shortN := orDefault(s.ShortlistSize, 10)
	poolN := orDefault(s.PlanPool, 6)
	aiN := orDefault(s.AnalysisFrames, 4)
	dupDist := orDefault(s.DedupeDistance, 6)

	// 1. Probe and extract.
	s.stage(StageExtracting)
	info, err := Probe(ctx, runner, in.VideoPath)
	if err != nil {
		return nil, err
	}
	if !info.HasVideo {
		return nil, Errorf(CodeNoVideoTrack, "This file has no video to make a thumbnail from.", nil)
	}
	duration := info.Duration
	if duration <= 0 {
		duration = in.Duration
	}
	if duration <= 0 {
		return nil, Errorf(CodeSourceUnreadable, "We couldn't work out how long this video is.", nil)
	}
	format := in.Format
	if format == "" {
		format = FormatFor(info.Width, info.Height)
	}
	framesDir := filepath.Join(in.WorkDir, "frames")
	if err := os.MkdirAll(framesDir, 0o755); err != nil {
		return nil, Errorf(CodeTaskFailed, "Thumbnail generation could not start.", err)
	}
	plan := PlanFrames(duration)
	raw, err := Extract(ctx, runner, in.VideoPath, framesDir, plan)
	if err != nil {
		return nil, err
	}
	s.logf("probed %dx%d %.1fs; extracted %d frames (every %.1fs)", info.Width, info.Height, duration, len(raw), plan.Interval)

	// 2. Filter, score, shortlist.
	s.stage(StageSelecting)
	measured := MeasureFrames(raw, nil)
	funnel := Funnel{Extracted: len(raw)}
	usable := Filter(measured)
	funnel.Usable = len(usable)
	if len(usable) == 0 {
		return nil, Errorf(CodeNoSuitableFrames, "Every frame in this video was too dark, blurry or empty to use.", nil)
	}
	ScoreLocal(usable)
	unique := Dedupe(usable, dupDist)
	funnel.Unique = len(unique)
	shortlist := Shortlist(unique, shortN)
	funnel.Candidates = len(shortlist)

	cands, detected := s.detectAll(ctx, shortlist)
	RankCandidates(cands)
	if len(cands) > poolN {
		cands = cands[:poolN]
	}
	s.logf("frames: %d extracted, %d usable, %d unique, %d candidates; detection=%v", funnel.Extracted, funnel.Usable, funnel.Unique, funnel.Candidates, detected)

	// 3. Ask the AI what the video is about.
	s.stage(StageAnalyzing)
	aiFrames, err := analysisFrames(cands, aiN)
	if err != nil {
		return nil, Errorf(CodeTaskFailed, "Thumbnail generation could not prepare frames for analysis.", err)
	}
	analysis, err := s.Analyzer.Analyze(ctx, AnalysisInput{
		Frames:     aiFrames,
		Transcript: in.Transcript,
		Duration:   duration,
		Style:      in.Style,
		Format:     format,
	})
	if err != nil {
		return nil, err
	}

	// 4. Plan three different compositions.
	s.stage(StageConcepts)
	style := ResolveStyle(in.Style, analysis.SuggestedStyle)
	plans, err := BuildPlans(cands, analysis, style, format)
	if err != nil {
		return nil, err
	}

	// 5. Render.
	s.stage(StageRendering)
	out := &Output{
		Analysis:       analysis,
		Funnel:         funnel,
		Style:          style,
		Format:         format,
		Video:          info,
		TranscriptUsed: in.Transcript != "",
		DetectionUsed:  detected,
	}
	cache := map[string]image.Image{}
	for _, p := range plans {
		src, ok := cache[p.Frame.Path]
		if !ok {
			src, err = LoadImage(p.Frame.Path)
			if err != nil {
				return nil, Errorf(CodeRenderFailed, "A thumbnail frame could not be read.", err)
			}
			cache[p.Frame.Path] = src
		}
		r, err := Render(src, p, in.Brand)
		if err != nil {
			return nil, asRenderError(err)
		}
		data, err := r.JPEG()
		if err != nil {
			return nil, Errorf(CodeRenderFailed, "A thumbnail could not be saved.", err)
		}
		w, h := p.Format.Size()
		out.Images = append(out.Images, Image{
			Concept:       p.Concept,
			Layout:        p.Layout,
			JPEG:          data,
			Width:         w,
			Height:        h,
			FrameTime:     p.Frame.Time,
			Headline:      p.Headline,
			SecondaryText: p.Secondary,
		})
		s.logf("rendered %s", p.Describe())
	}
	return out, nil
}

func asRenderError(err error) error {
	if _, ok := err.(*Error); ok {
		return err
	}
	return Errorf(CodeRenderFailed, "A thumbnail could not be drawn.", err)
}

// detectAll runs detection on each shortlisted frame, a few at a time. A
// frame whose detection fails is still scored from its local measurements, so
// a Rekognition outage degrades the choice of frames rather than ending the run.
func (s *Service) detectAll(ctx context.Context, frames []Frame) ([]Candidate, bool) {
	cands := make([]Candidate, len(frames))
	if s.Detector == nil {
		for i, f := range frames {
			cands[i] = ScoreCandidate(f, Detection{}, false)
		}
		return cands, false
	}
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4)
	var mu sync.Mutex
	ok := 0
	for i, f := range frames {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, f Frame) {
			defer wg.Done()
			defer func() { <-sem }()
			data, err := os.ReadFile(f.Path)
			if err != nil {
				cands[i] = ScoreCandidate(f, Detection{}, false)
				return
			}
			callCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
			defer cancel()
			det, err := s.Detector.Detect(callCtx, data)
			// Partial results (faces found, labels failed) are still useful.
			useful := err == nil || len(det.Faces) > 0 || len(det.Labels) > 0
			if err != nil {
				s.logf("detect frame %d failed: %v", f.Index, err)
			}
			cands[i] = ScoreCandidate(f, det, useful)
			if useful {
				mu.Lock()
				ok++
				mu.Unlock()
			}
		}(i, f)
	}
	wg.Wait()
	return cands, ok > 0
}

// maxAIWidth keeps the images sent to the model small: it does not need more
// than about 1000 pixels to read a scene, and smaller images cost less.
const maxAIWidth = 1024

// analysisFrames picks up to n of the best candidates, shows them in time
// order, and shrinks them for the model.
func analysisFrames(cands []Candidate, n int) ([]AnalysisFrame, error) {
	if len(cands) > n {
		cands = cands[:n]
	}
	picked := append([]Candidate(nil), cands...)
	for i := 1; i < len(picked); i++ { // insertion sort by time, n is tiny
		for j := i; j > 0 && picked[j].Time < picked[j-1].Time; j-- {
			picked[j], picked[j-1] = picked[j-1], picked[j]
		}
	}
	out := make([]AnalysisFrame, 0, len(picked))
	for _, c := range picked {
		data, err := shrinkJPEG(c.Path, maxAIWidth)
		if err != nil {
			return nil, err
		}
		out = append(out, AnalysisFrame{Time: c.Time, JPEG: data})
	}
	return out, nil
}

func shrinkJPEG(path string, maxW int) ([]byte, error) {
	img, err := LoadImage(path)
	if err != nil {
		return nil, err
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w > maxW {
		h = h * maxW / w
		w = maxW
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	xdraw.ApproxBiLinear.Scale(dst, dst.Bounds(), img, b, xdraw.Src, nil)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: 82}); err != nil {
		return nil, fmt.Errorf("encode analysis frame: %w", err)
	}
	return buf.Bytes(), nil
}
