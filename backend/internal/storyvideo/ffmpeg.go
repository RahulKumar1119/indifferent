package storyvideo

import (
	"fmt"
	"strings"

	"github.com/rahul/indifferent/backend/internal/models"
	"github.com/rahul/indifferent/backend/internal/shorts/render"
)

// MediaKind is what a scene's picture is.
type MediaKind string

const (
	KindPhoto MediaKind = "photo"
	KindVideo MediaKind = "video"
)

// Framing is how a picture is fitted into the 9:16 frame.
type Framing string

const (
	// FramingCover scales the picture to fill the frame and cuts off the rest.
	FramingCover Framing = "cover"
	// FramingFit shows the whole picture over a blurred copy of itself, so
	// nothing is cut. It is the same look as the AI Shorts blur-fill.
	FramingFit Framing = "fit"
)

// CoverMaxAspect is the widest picture (width / height) that is cropped to
// fill the frame. A 3:4 portrait photo loses a quarter of its width to a 9:16
// crop, which keeps the subject; anything wider would lose too much (a 16:9
// clip would lose two thirds), so it is shown whole over a blurred background.
const CoverMaxAspect = 0.75

// ZoomPercent is how far a still photo zooms in over the time it is shown. A
// still held for ten seconds looks dead; a few percent of slow movement reads
// as a deliberate shot. Clips already move and are not zoomed.
const ZoomPercent = 8

// FrameFor says how a picture of the given size is fitted.
func FrameFor(width, height int) Framing {
	if height > 0 && float64(width)/float64(height) <= CoverMaxAspect {
		return FramingCover
	}
	return FramingFit
}

// Scene is one scene's inputs on disk.
type Scene struct {
	Kind MediaKind
	// MediaPath is the picture or clip.
	MediaPath string
	// Width and Height are the picture's pixel size, for choosing the framing.
	Width, Height int
	// NarrationPath is the scene's narration audio.
	NarrationPath string
}

// Spec is everything the render command needs.
type Spec struct {
	Scenes   []Scene
	Timeline Timeline
	// CaptionsPath is an SRT or ASS file as BuildCaptions makes it, or empty
	// for a video without captions.
	CaptionsPath string
	Caption      models.CaptionStyle
	OutputPath   string
}

// safeFilterPath admits only characters that mean nothing inside an FFmpeg
// filter expression. The worker writes the caption file into its own work
// directory, so a path that needs escaping is a bug, not a case to handle.
func safeFilterPath(p string) error {
	if p == "" {
		return fmt.Errorf("empty path")
	}
	for _, r := range p {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '/', r == '.', r == '_', r == '-':
		default:
			return fmt.Errorf("path %q has a character (%q) that is unsafe inside an FFmpeg filter", p, r)
		}
	}
	return nil
}

// BuildArgs returns the arguments for one FFmpeg run that renders the video:
// 1080x1920 at 30 fps, H.264 and AAC, the AI Shorts output spec.
//
// It is a single filter graph. Each scene's picture is framed to 9:16 (see
// FrameFor), photos get a slow zoom, the scenes are chained with cross-fades,
// the captions are burned in, and the narrations are joined end to end. Each
// scene's picture is made to exist for slot.Show seconds, which is its span
// plus the cross-fade into the next scene, so a fade always has picture to
// fade into.
//
// Clips shorter than their scene loop; longer ones are cut. A clip's own sound
// is never used.
func BuildArgs(spec Spec) ([]string, error) {
	n := len(spec.Scenes)
	if n == 0 {
		return nil, fmt.Errorf("a video needs at least one scene")
	}
	if len(spec.Timeline.Slots) != n {
		return nil, fmt.Errorf("%d scenes for a timeline of %d", n, len(spec.Timeline.Slots))
	}
	if spec.OutputPath == "" {
		return nil, fmt.Errorf("no output path")
	}
	if spec.CaptionsPath != "" {
		if err := safeFilterPath(spec.CaptionsPath); err != nil {
			return nil, fmt.Errorf("captions: %w", err)
		}
	}

	var args, graph []string
	for i, s := range spec.Scenes {
		if s.MediaPath == "" || s.NarrationPath == "" {
			return nil, fmt.Errorf("scene %d is missing its media or narration", i+1)
		}
		if s.Width <= 0 || s.Height <= 0 {
			return nil, fmt.Errorf("scene %d: the picture size is unknown", i+1)
		}
		show := fmt.Sprintf("%.3f", spec.Timeline.Slots[i].Show)
		switch s.Kind {
		case KindPhoto:
			args = append(args, "-loop", "1", "-framerate", fmt.Sprint(FPS), "-t", show, "-i", s.MediaPath)
		case KindVideo:
			args = append(args, "-stream_loop", "-1", "-t", show, "-i", s.MediaPath)
		default:
			return nil, fmt.Errorf("scene %d: unknown media kind %q", i+1, s.Kind)
		}
	}
	for _, s := range spec.Scenes {
		args = append(args, "-i", s.NarrationPath)
	}

	for i, s := range spec.Scenes {
		graph = append(graph, sceneVideoFilter(i, s, spec.Timeline.Slots[i]))
	}

	// Chain the cross-fades. Scene i's fade starts when its narration does, and
	// ends exactly where the picture built so far ends.
	last := "[v0]"
	for i := 1; i < n; i++ {
		out := fmt.Sprintf("[x%d]", i)
		graph = append(graph, fmt.Sprintf("%s[v%d]xfade=transition=fade:duration=%.3f:offset=%.3f%s",
			last, i, TransitionSeconds, spec.Timeline.Slots[i].Start, out))
		last = out
	}
	if spec.CaptionsPath != "" {
		graph = append(graph, fmt.Sprintf("%s%s[vout]", last, render.SubtitlesFilter(spec.CaptionsPath, spec.Caption)))
		last = "[vout]"
	}

	// Narration: each scene's audio is padded with silence to exactly its span
	// and the spans are joined, so the voice of scene i starts at slot.Start.
	var audio []string
	for i := range spec.Scenes {
		graph = append(graph, fmt.Sprintf("[%d:a]aresample=48000,apad=whole_dur=%.3f,atrim=0:%.3f,asetpts=PTS-STARTPTS[a%d]",
			n+i, spec.Timeline.Slots[i].Span, spec.Timeline.Slots[i].Span, i))
		audio = append(audio, fmt.Sprintf("[a%d]", i))
	}
	// The picture is held a fade past the last narration; pad the sound to match.
	graph = append(graph, fmt.Sprintf("%sconcat=n=%d:v=0:a=1,apad=whole_dur=%.3f[aout]", strings.Join(audio, ""), n, spec.Timeline.Total))

	args = append(args,
		"-filter_complex", strings.Join(graph, ";"),
		"-map", last, "-map", "[aout]",
		"-c:v", "libx264", "-preset", "veryfast", "-crf", "23", "-pix_fmt", "yuv420p", "-r", fmt.Sprint(FPS),
		"-c:a", "aac", "-b:a", "128k", "-ar", "48000",
		"-movflags", "+faststart",
		"-t", fmt.Sprintf("%.3f", spec.Timeline.Total),
		"-y", spec.OutputPath,
	)
	return args, nil
}

// frameCover and frameFit are the two ways of getting a picture to exactly
// 1080x1920 (before any zoom).
const (
	frameCover = "scale=1080:1920:force_original_aspect_ratio=increase,crop=1080:1920"
	// frameFit is the Shorts blur-fill: a quarter-size blurred copy as the
	// background (the same look as a full-size blur at a fraction of the cost,
	// measured in issue #137), the whole picture over it.
	frameFit = "split=2[b%[1]d][m%[1]d];[b%[1]d]scale=270:480:force_original_aspect_ratio=increase,crop=270:480,gblur=sigma=10,scale=1080:1920[bg%[1]d];[m%[1]d]scale=1080:1920:force_original_aspect_ratio=decrease[fg%[1]d];[bg%[1]d][fg%[1]d]overlay=(W-w)/2:(H-h)/2"
)

// sceneVideoFilter frames scene i's picture and, for a photo, zooms it. Its
// output label is [v<i>].
func sceneVideoFilter(i int, s Scene, slot Slot) string {
	framed := fmt.Sprintf("[%d:v]", i)
	if FrameFor(s.Width, s.Height) == FramingCover {
		framed += frameCover
	} else {
		framed += fmt.Sprintf(frameFit, i)
	}
	if s.Kind != KindPhoto {
		return fmt.Sprintf("%s,setsar=1,fps=%d,format=yuv420p[v%d]", framed, FPS, i)
	}
	// Zoom a 2x copy and shrink it back. Zooming at the output size moves the
	// picture in whole-pixel steps and the motion visibly shimmers; at twice
	// the size each step is half a pixel. Measured on a 5 second photo, the
	// frame-to-frame change had a coefficient of variation of 0.29 at 1x and
	// 0.13 at 2x, against 0.14 for an 8x reference. It costs about 55% more
	// render time than no zoom (62 s against 40 s for a 38 s video on two
	// cores), which is invisible to the user and cheap on Spot.
	z := float64(ZoomPercent) / 100
	d := slot.Show
	return fmt.Sprintf("%s,scale=2160:3840:flags=bicubic,scale=w='2160*(1+%.2f*t/%.3f)':h='3840*(1+%.2f*t/%.3f)':eval=frame:flags=bilinear,crop=2160:3840,scale=1080:1920:flags=bilinear,setsar=1,fps=%d,format=yuv420p[v%d]",
		framed, z, d, z, d, FPS, i)
}
