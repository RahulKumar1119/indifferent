package render

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/rahul/indifferent/backend/internal/models"
)

// outputFPS is the frame rate of every rendered clip. Freeze holds and zoom
// timing are counted in these frames.
const outputFPS = 30

// audioFormat sets every audio piece to one format, so the pieces of a
// concat always match.
const audioFormat = "aformat=sample_fmts=fltp:sample_rates=48000:channel_layouts=stereo"

// Errors a render with effects can return. The caller can tell them apart.
// ffmpeg's own output goes to the task log.
var (
	ErrEffectsInvalid = errors.New("video effect settings are invalid")
	ErrRenderTimeout  = errors.New("render timed out")
	ErrRenderCanceled = errors.New("render canceled")
)

// Encode timeout for renders with effects: effectTimeoutPerSecond for each
// second of output, kept between effectTimeoutMin and effectTimeoutMax.
const (
	effectTimeoutPerSecond = 10 * time.Second
	effectTimeoutMin       = 2 * time.Minute
	effectTimeoutMax       = 15 * time.Minute
)

// impactFontCandidates are DejaVu Sans files on the render image (Alpine
// ttf-dejavu) and on Debian or Ubuntu hosts.
var impactFontCandidates = []string{
	"/usr/share/fonts/ttf-dejavu/DejaVuSans.ttf",
	"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
	"/usr/share/fonts/dejavu/DejaVuSans.ttf",
}

// clipEffects is one clip's resolved effect timeline. Punch-in and freeze
// times are seconds from the clip start on the source timeline. Impact times
// are on the output timeline, which is longer by every freeze before them.
type clipEffects struct {
	outSec   float64                  // output length: the clip plus every held frame
	zoomAmp  float64                  // peak zoom minus 1.0; 0 when zoom is off
	punches  []punchWindow            // sorted by start
	grade    *gradeLook               // nil when grading is off
	holds    []frameHold              // sorted by frame
	texts    []impactCue              // output timeline
	captions *models.AnimatedCaptions // non-nil when animated captions are on
}

// punchWindow is one zoom moment on the source timeline.
type punchWindow struct{ start, dur float64 }

// frameHold holds source frame `frame` on screen for `extra` more frames.
type frameHold struct{ frame, extra int }

// impactCue is one line of impact text on the output timeline. Its text is
// written to a file, and drawtext reads it from there, so the text never
// enters the filtergraph.
type impactCue struct {
	start, end float64
	size       int
	text       string
	file       string
}

// planEffects resolves the saved settings for one clip. It needs e to be
// non-nil and active, and the values to be validated. Events that start at or
// after the clip end are skipped for this clip.
func planEffects(e *models.VideoEffects, clipSec float64) clipEffects {
	p := clipEffects{outSec: clipSec}
	if c := e.Captions; c != nil && c.Enabled {
		caps := *c
		p.captions = &caps
	}
	if z := e.Zoom; z != nil && z.Enabled {
		p.zoomAmp = 0.5 * float64(z.Intensity) / 100
		for _, pi := range z.PunchIns {
			if pi.At >= clipSec {
				continue
			}
			// A punch-in that runs past the clip end is shortened, so its zoom
			// still eases back to 1.0 inside the clip.
			p.punches = append(p.punches, punchWindow{start: pi.At, dur: math.Min(pi.Duration, clipSec-pi.At)})
		}
		sort.Slice(p.punches, func(i, j int) bool { return p.punches[i].start < p.punches[j].start })
	}
	if g := e.Grade; g != nil && g.Enabled {
		look := gradeLookFor(g.Look, float64(g.Intensity)/100)
		p.grade = &look
	}
	if f := e.Freeze; f != nil && f.Enabled {
		p.planFreeze(f, clipSec)
	}
	return p
}

// planFreeze keeps the holds the clip can show and places the impact text on
// the output timeline.
func (p *clipEffects) planFreeze(f *models.FreezeEffect, clipSec float64) {
	// A hold needs at least two more frames after it, so the piece that
	// follows is never empty.
	framesAvail := int(math.Round(clipSec * outputFPS))
	for _, h := range f.Freezes {
		if h.At >= clipSec {
			continue
		}
		frame := int(math.Round(h.At * outputFPS))
		if frame > framesAvail-3 {
			continue
		}
		p.holds = append(p.holds, frameHold{frame: frame, extra: int(math.Round(h.Duration * outputFPS))})
	}
	sort.Slice(p.holds, func(i, j int) bool { return p.holds[i].frame < p.holds[j].frame })
	p.outSec = clipSec + float64(p.heldFrames())/outputFPS

	size := int(math.Round(36 + 0.6*float64(f.Intensity)))
	for _, t := range f.ImpactTexts {
		if t.At >= clipSec {
			continue
		}
		end := math.Min(t.At+t.Duration, clipSec)
		p.texts = append(p.texts, impactCue{
			start: p.mapStart(t.At),
			end:   p.mapEnd(end),
			size:  size,
			text:  t.Text,
		})
	}
}

// heldFrames is the number of frames added by all holds.
func (p clipEffects) heldFrames() int {
	total := 0
	for _, h := range p.holds {
		total += h.extra
	}
	return total
}

// shiftBefore returns the output time added by holds that start before t.
// With inclusive set, a hold that starts exactly at t counts too.
func (p clipEffects) shiftBefore(t float64, inclusive bool) float64 {
	total := 0.0
	for _, h := range p.holds {
		at := float64(h.frame) / outputFPS
		if at < t || (inclusive && at == t) {
			total += float64(h.extra) / outputFPS
		}
	}
	return total
}

// mapStart maps a clip-relative source time to the output timeline. A hold
// that starts at or before t moves t later by the hold's extra time.
func (p clipEffects) mapStart(t float64) float64 {
	return t + p.shiftBefore(t, true)
}

// mapEnd is mapStart for the end of a span. A hold that starts exactly at t
// does not move t.
func (p clipEffects) mapEnd(t float64) float64 {
	return t + p.shiftBefore(t, false)
}

// remapWords moves caption word times to the output timeline. Times stay
// absolute (clipStart plus the clip-relative time), as the caption builders
// expect.
func (p clipEffects) remapWords(words []models.Word, clipStart float64) []models.Word {
	out := make([]models.Word, len(words))
	for i, w := range words {
		out[i] = models.Word{
			Text:  w.Text,
			Start: clipStart + p.mapStart(w.Start-clipStart),
			End:   clipStart + p.mapEnd(w.End-clipStart),
		}
	}
	return out
}

// zoomExpr is the zoompan z expression. Each punch-in adds a half sine pulse,
// so the zoom eases in and out. Outside the punch-ins z is 1.
func (p clipEffects) zoomExpr() string {
	terms := make([]string, 0, len(p.punches))
	for _, w := range p.punches {
		terms = append(terms, fmt.Sprintf("between(time,%.3f,%.3f)*sin(PI*(time-%.3f)/%.3f)",
			w.start, w.start+w.dur, w.start, w.dur))
	}
	return fmt.Sprintf("1+%.4f*(%s)", p.zoomAmp, strings.Join(terms, "+"))
}

// segment is one cut of the clip: a run of source frames, or a hold.
type segment struct {
	hold     bool
	from, to int // frame numbers; to < 0 means the end of the clip
	extra    int // hold only: frames added
}

// segments cuts the clip at each hold. A hold is its own segment, and the
// last segment always runs to the end of the clip.
func (p clipEffects) segments() []segment {
	var segs []segment
	pos := 0
	for _, h := range p.holds {
		if h.frame > pos {
			segs = append(segs, segment{from: pos, to: h.frame})
		}
		segs = append(segs, segment{hold: true, from: h.frame, to: h.frame + 1, extra: h.extra})
		pos = h.frame + 1
	}
	return append(segs, segment{from: pos, to: -1})
}

// gradeLook is one color look with its values already scaled by intensity.
type gradeLook struct {
	contrast, saturation float64
	balance              [9]float64 // rs gs bs rm gm bm rh gh bh
	curves               string     // curves options, "" when the look has none
}

// gradeLookFor returns a look scaled by k (0 to 1). At k 0 every value is the
// identity, so the picture is unchanged.
func gradeLookFor(name string, k float64) gradeLook {
	pt := func(x, y float64) [2]float64 { return [2]float64{x, y} }
	switch name {
	case "warm":
		return gradeLook{
			contrast:   1 + 0.04*k,
			saturation: 1 + 0.10*k,
			balance:    [9]float64{0, 0, 0, 0.06 * k, 0.02 * k, -0.08 * k, 0.08 * k, 0.03 * k, -0.06 * k},
			curves: "r='" + curvePoints(k, pt(0, 0), pt(0.5, 0.53), pt(1, 1)) +
				"':b='" + curvePoints(k, pt(0, 0), pt(0.5, 0.47), pt(1, 1)) + "'",
		}
	case "cool":
		return gradeLook{
			contrast:   1 + 0.04*k,
			saturation: 1 - 0.05*k,
			balance:    [9]float64{-0.03 * k, 0, 0.07 * k, 0, 0, 0.04 * k, -0.05 * k, 0, 0.04 * k},
			curves: "r='" + curvePoints(k, pt(0, 0), pt(0.5, 0.47), pt(1, 1)) +
				"':b='" + curvePoints(k, pt(0, 0), pt(0.5, 0.53), pt(1, 1)) + "'",
		}
	case "high-contrast":
		return gradeLook{
			contrast:   1 + 0.25*k,
			saturation: 1 + 0.12*k,
			curves:     "all='" + curvePoints(k, pt(0, 0), pt(0.25, 0.18), pt(0.75, 0.82), pt(1, 1)) + "'",
		}
	default: // cinematic
		return gradeLook{
			contrast:   1 + 0.10*k,
			saturation: 1 + 0.08*k,
			balance:    [9]float64{-0.06 * k, 0, 0.08 * k, 0, 0, 0, 0.08 * k, 0.02 * k, -0.07 * k},
			curves:     "all='" + curvePoints(k, pt(0, 0), pt(0.25, 0.22), pt(0.75, 0.79), pt(1, 1)) + "'",
		}
	}
}

// curvePoints formats x/y control points, pulling each y toward the identity
// line by k.
func curvePoints(k float64, pts ...[2]float64) string {
	parts := make([]string, len(pts))
	for i, p := range pts {
		parts[i] = fmt.Sprintf("%.3f/%.3f", p[0], p[0]+(p[1]-p[0])*k)
	}
	return strings.Join(parts, " ")
}

// filters returns the grade as a filter chain: eq, curves, colorbalance.
func (g gradeLook) filters() []string {
	out := []string{fmt.Sprintf("eq=contrast=%.3f:saturation=%.3f", g.contrast, g.saturation)}
	if g.curves != "" {
		out = append(out, "curves="+g.curves)
	}
	b := g.balance
	out = append(out, fmt.Sprintf("colorbalance=rs=%.3f:gs=%.3f:bs=%.3f:rm=%.3f:gm=%.3f:bm=%.3f:rh=%.3f:gh=%.3f:bh=%.3f",
		b[0], b[1], b[2], b[3], b[4], b[5], b[6], b[7], b[8]))
	return out
}

// impactFontOption picks the drawtext font. drawtext resolves a family name
// only with fontconfig, which not every ffmpeg build has, so a DejaVu Sans
// file is used when one is installed.
func impactFontOption() string {
	for _, p := range impactFontCandidates {
		if _, err := os.Stat(p); err == nil {
			return "fontfile=" + p
		}
	}
	return "font='DejaVu Sans'"
}

// drawtext returns the drawtext filter for one impact cue. The text is read
// from its file with expansion off, so no user text reaches the graph. The
// text fades in and out over its window.
func (t impactCue) drawtext() (string, error) {
	path, err := filterPath(t.file)
	if err != nil {
		return "", err
	}
	alpha := fmt.Sprintf("if(lt(t,%.3f),(t-%.3f)/0.12,if(gt(t,%.3f),(%.3f-t)/0.12,1))",
		t.start+0.12, t.start, t.end-0.12, t.end)
	return fmt.Sprintf("drawtext=%s:textfile=%s:expansion=none:fontsize=%d:fontcolor=white:borderw=5:bordercolor=black:x=(w-text_w)/2:y=(h-text_h)*0.42:alpha='%s':enable='between(t,%.3f,%.3f)'",
		impactFontOption(), path, t.size, alpha, t.start, t.end), nil
}

// filterPath returns p unchanged when it has only characters that need no
// escaping in a filtergraph. Paths here come from the work directory and the
// clip ID, so anything else is rejected rather than escaped.
func filterPath(p string) (string, error) {
	for _, r := range p {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '/' || r == '.' || r == '_' || r == '-':
		default:
			return "", fmt.Errorf("path %q has characters that cannot go in an ffmpeg filter", p)
		}
	}
	return p, nil
}

// captionBurn returns the subtitles filter for a caption file, or "" when
// there is none. Animated captions carry their own styling in the ASS file.
// Other caption styles follow the existing rules: karaoke is ASS, and the rest
// are SRT with force_style.
func captionBurn(subs string, style models.CaptionStyle, animated bool) (string, error) {
	if subs == "" {
		return "", nil
	}
	path, err := filterPath(subs)
	if err != nil {
		return "", err
	}
	if animated || isKaraokeStyle(style) {
		return "subtitles=" + path, nil
	}
	return fmt.Sprintf("subtitles=%s:force_style='%s'", path, ResolveForceStyle(style)), nil
}

// writeImpactFiles writes each impact line to its own file in dir and points
// the cue at that file.
func writeImpactFiles(dir, clipID string, cues []impactCue) error {
	for i := range cues {
		path := filepath.Join(dir, fmt.Sprintf("%s-impact-%d.txt", clipID, i+1))
		if err := os.WriteFile(path, []byte(cues[i].text), 0o644); err != nil {
			return fmt.Errorf("failed to write impact text: %w", err)
		}
		cues[i].file = path
	}
	return nil
}

// graphInput is what the filter graph needs beyond the clip plan.
type graphInput struct {
	framing string // framing filter chain for the video source
	videoIn string // input reference, "[0:v]"
	audioIn string // input reference, "[0:a]" or "[1:a]"; "" when there is no audio
	burn    string // caption filter from captionBurn, or ""
}

// graph builds the -filter_complex graph for one clip. Stages run in this
// order: framing, zoom, grade, freeze holds, impact text, captions. It returns
// the graph and the audio label to map ("" when audio passes through
// unchanged).
func (p clipEffects) graph(in graphInput) (string, string, error) {
	var chains []string
	n := 0
	label := func(prefix string) string {
		n++
		return fmt.Sprintf("%s%d", prefix, n)
	}

	cur := label("v")
	chains = append(chains, in.videoIn+in.framing+ref(cur))
	if p.zoomAmp > 0 && len(p.punches) > 0 {
		next := label("v")
		chains = append(chains, fmt.Sprintf("%szoompan=z='%s':x='iw/2-(iw/zoom/2)':y='ih/2-(ih/zoom/2)':d=1:s=%dx%d:fps=%d%s",
			ref(cur), p.zoomExpr(), outputWidth, outputHeight, outputFPS, ref(next)))
		cur = next
	}
	if p.grade != nil {
		next := label("v")
		chains = append(chains, ref(cur)+strings.Join(p.grade.filters(), ",")+ref(next))
		cur = next
	}

	audioOut := ""
	if len(p.holds) > 0 {
		var holdChains []string
		holdChains, cur, audioOut = p.holdGraph(ref(cur), in.audioIn, label)
		chains = append(chains, holdChains...)
	}

	if len(p.texts) > 0 {
		next := label("v")
		filters := make([]string, 0, len(p.texts))
		for _, t := range p.texts {
			f, err := t.drawtext()
			if err != nil {
				return "", "", err
			}
			filters = append(filters, f)
		}
		chains = append(chains, ref(cur)+strings.Join(filters, ",")+ref(next))
		cur = next
	}

	if in.burn != "" {
		chains = append(chains, ref(cur)+in.burn+"[vout]")
	} else {
		chains = append(chains, ref(cur)+"null[vout]")
	}
	return strings.Join(chains, ";"), audioOut, nil
}

// holdGraph cuts video and audio at each hold and joins the pieces again. Each
// hold repeats one frame for its extra frames, and its audio is silent for the
// same time, so the two streams stay in step. It returns the chains, the video
// label and the audio label ("" when there is no audio to cut). in and audioIn
// are input references that already carry their brackets.
func (p clipEffects) holdGraph(in, audioIn string, label func(string) string) ([]string, string, string) {
	segs := p.segments()
	var chains []string

	vSplit := make([]string, len(segs))
	for i := range segs {
		vSplit[i] = label("vs")
	}
	chains = append(chains, splitChain(in, vSplit, "split", "null"))

	vPieces := make([]string, len(segs))
	for i, s := range segs {
		vPieces[i] = label("vp")
		src := ref(vSplit[i])
		switch {
		case s.hold:
			chains = append(chains, fmt.Sprintf("%strim=start_frame=%d:end_frame=%d,setpts=PTS-STARTPTS,tpad=stop=%d:stop_mode=clone,setpts=PTS-STARTPTS%s",
				src, s.from, s.to, s.extra, ref(vPieces[i])))
		case s.to < 0:
			chains = append(chains, fmt.Sprintf("%strim=start_frame=%d,setpts=PTS-STARTPTS%s", src, s.from, ref(vPieces[i])))
		default:
			chains = append(chains, fmt.Sprintf("%strim=start_frame=%d:end_frame=%d,setpts=PTS-STARTPTS%s",
				src, s.from, s.to, ref(vPieces[i])))
		}
	}
	vOut := label("vf")
	chains = append(chains, concatChain(vPieces, 1, 0, vOut))

	if audioIn == "" {
		return chains, vOut, ""
	}
	var srcSegs []int
	for i, s := range segs {
		if !s.hold {
			srcSegs = append(srcSegs, i)
		}
	}
	aSplit := make([]string, len(srcSegs))
	for k := range srcSegs {
		aSplit[k] = label("as")
	}
	chains = append(chains, splitChain(audioIn, aSplit, "asplit", "anull"))

	aPieces := make([]string, len(segs))
	k := 0
	for i, s := range segs {
		aPieces[i] = label("ap")
		if s.hold {
			d := float64(s.extra+1) / outputFPS
			chains = append(chains, fmt.Sprintf("anullsrc=r=48000:cl=stereo:d=%.6f,%s%s", d, audioFormat, ref(aPieces[i])))
			continue
		}
		src := ref(aSplit[k])
		k++
		start := float64(s.from) / outputFPS
		if s.to < 0 {
			chains = append(chains, fmt.Sprintf("%satrim=start=%.6f,asetpts=PTS-STARTPTS,%s%s",
				src, start, audioFormat, ref(aPieces[i])))
		} else {
			chains = append(chains, fmt.Sprintf("%satrim=start=%.6f:end=%.6f,asetpts=PTS-STARTPTS,%s%s",
				src, start, float64(s.to)/outputFPS, audioFormat, ref(aPieces[i])))
		}
	}
	aOut := label("af")
	chains = append(chains, concatChain(aPieces, 0, 1, aOut))
	return chains, vOut, aOut
}

// ref writes a filtergraph label reference such as "[v3]".
func ref(name string) string { return "[" + name + "]" }

// splitChain splits in into one output per name. A single output uses the
// identity filter, because split needs at least two outputs.
func splitChain(in string, names []string, filter, identity string) string {
	if len(names) == 1 {
		return in + identity + ref(names[0])
	}
	outs := make([]string, len(names))
	for i, name := range names {
		outs[i] = ref(name)
	}
	return fmt.Sprintf("%s%s=%d%s", in, filter, len(names), strings.Join(outs, ""))
}

// concatChain joins the labelled pieces in order into out.
func concatChain(pieces []string, video, audio int, out string) string {
	refs := make([]string, len(pieces))
	for i, p := range pieces {
		refs[i] = ref(p)
	}
	return fmt.Sprintf("%sconcat=n=%d:v=%d:a=%d%s", strings.Join(refs, ""), len(pieces), video, audio, ref(out))
}

// effectJob is one clip's encode: source, output, window and the resolved
// effects.
type effectJob struct {
	src, out   string
	start, end float64
	audioOnly  bool // no video stream: a black canvas is drawn instead
	hasAudio   bool // video sources only; audio-only sources always have audio
	framing    string
	plan       clipEffects
	burn       string
}

// effectArgs returns the ffmpeg arguments for a clip with effects, as one
// argument vector for os/exec (no shell). Video comes from the source, or
// from a black canvas for audio-only sources. Audio passes through unless a
// hold needs it cut.
func effectArgs(j effectJob) ([]string, error) {
	videoIn, audioIn := "[0:v]", "[0:a]"
	var args []string
	if j.audioOnly {
		audioIn = "[1:a]"
		args = append(args, "-f", "lavfi", "-i", fmt.Sprintf("color=c=black:s=%dx%d:r=%d:d=%s",
			outputWidth, outputHeight, outputFPS, formatSeconds(j.plan.outSec)))
	} else if !j.hasAudio {
		audioIn = ""
	}
	framing := j.framing
	if j.audioOnly {
		framing = "null"
	}
	graph, audioLabel, err := j.plan.graph(graphInput{
		framing: framing,
		videoIn: videoIn,
		audioIn: audioIn,
		burn:    j.burn,
	})
	if err != nil {
		return nil, err
	}
	args = append(args,
		"-ss", formatSeconds(j.start),
		"-to", formatSeconds(j.end),
		"-i", j.src,
		"-filter_complex", graph,
		"-map", "[vout]",
	)
	switch {
	case audioLabel != "":
		args = append(args, "-map", ref(audioLabel))
	case j.audioOnly:
		args = append(args, "-map", "1:a?")
	default:
		args = append(args, "-map", "0:a?")
	}
	args = append(args,
		"-c:v", "libx264",
		"-preset", "veryfast",
		"-pix_fmt", "yuv420p",
		"-r", "30",
		"-c:a", "aac",
		"-b:a", "128k",
		"-ar", "48000",
	)
	if j.audioOnly {
		args = append(args, "-shortest")
	}
	return append(args, "-movflags", "+faststart", "-y", j.out), nil
}

// effectRenderArgs builds the ffmpeg arguments for one clip with effects. The
// audio probe runs only when a hold needs the audio cut.
func (r *Renderer) effectRenderArgs(in RenderInput, plan clipEffects, srcPath, subsPath, outPath string, start, end float64, cropX string, fit bool) ([]string, error) {
	hasAudio := true
	if len(plan.holds) > 0 && !in.AudioOnly {
		probe := r.ProbeHasAudio
		if probe == nil {
			probe = defaultProbeHasAudio
		}
		ok, err := probe(srcPath)
		if err != nil {
			log.Printf("effects: audio probe failed (%v), assuming the source has audio", err)
		} else {
			hasAudio = ok
		}
	}
	burn, err := captionBurn(subsPath, in.Caption, plan.captions != nil)
	if err != nil {
		return nil, err
	}
	return effectArgs(effectJob{
		src:       srcPath,
		out:       outPath,
		start:     start,
		end:       end,
		audioOnly: in.AudioOnly,
		hasAudio:  hasAudio,
		framing:   framingFilter(cropX, fit),
		plan:      plan,
		burn:      burn,
	})
}

// effectTimeout is the encode limit for a render with effects.
func (r *Renderer) effectTimeout(outSec float64) time.Duration {
	if r.EffectTimeout > 0 {
		return r.EffectTimeout
	}
	return effectEncodeTimeout(outSec)
}

// effectEncodeTimeout bounds the ffmpeg run for a render with effects.
func effectEncodeTimeout(outSec float64) time.Duration {
	d := time.Duration(outSec * float64(effectTimeoutPerSecond))
	if d < effectTimeoutMin {
		return effectTimeoutMin
	}
	if d > effectTimeoutMax {
		return effectTimeoutMax
	}
	return d
}

// encodeFailure maps an ffmpeg failure to the render error. A cancelled render
// reports ErrRenderCanceled, and a run that hit its timeout reports
// ErrRenderTimeout. Any other failure keeps the message renders have always
// used.
func encodeFailure(parent, enc context.Context, timeout time.Duration, err error) error {
	switch {
	case parent.Err() != nil:
		return fmt.Errorf("%w: %v", ErrRenderCanceled, err)
	case errors.Is(enc.Err(), context.DeadlineExceeded):
		return fmt.Errorf("%w after %s: %v", ErrRenderTimeout, timeout, err)
	default:
		return fmt.Errorf("ffmpeg render failed: %w", err)
	}
}
