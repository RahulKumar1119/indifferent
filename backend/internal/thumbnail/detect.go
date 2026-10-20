package thumbnail

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/rekognition"
	"github.com/aws/aws-sdk-go-v2/service/rekognition/types"
)

// Box is a normalised (0 to 1) rectangle in an image.
type Box struct {
	Left, Top, Width, Height float64
}

// CenterX and CenterY are the box midpoint.
func (b Box) CenterX() float64 { return b.Left + b.Width/2 }
func (b Box) CenterY() float64 { return b.Top + b.Height/2 }

// Area is the fraction of the frame the box covers.
func (b Box) Area() float64 { return b.Width * b.Height }

// Right and Bottom are the far edges.
func (b Box) Right() float64  { return b.Left + b.Width }
func (b Box) Bottom() float64 { return b.Top + b.Height }

// edgeMargin is how close to the frame edge a box may sit before the subject
// counts as cut off.
const edgeMargin = 0.01

// CutOff reports whether the box touches the frame edge, which usually means
// the subject is partly out of shot.
func (b Box) CutOff() bool {
	return b.Left < edgeMargin || b.Top < edgeMargin || b.Right() > 1-edgeMargin || b.Bottom() > 1-edgeMargin
}

// Face is one detected face.
type Face struct {
	Box         Box
	Confidence  float64
	Emotion     string // CALM, HAPPY, SURPRISED, ...
	EmotionConf float64
	EyesOpen    bool
}

// Label is a detected object or scene with the places it appears.
type Label struct {
	Name       string
	Confidence float64
	Boxes      []Box
}

// Detection is what the vision service saw in one frame.
type Detection struct {
	Faces  []Face
	Labels []Label
}

// LargestFace returns the face covering the most of the frame.
func (d Detection) LargestFace() (Face, bool) {
	best := -1
	for i, f := range d.Faces {
		if best == -1 || f.Box.Area() > d.Faces[best].Box.Area() {
			best = i
		}
	}
	if best == -1 {
		return Face{}, false
	}
	return d.Faces[best], true
}

// LargestSubject returns the box of the biggest non-person object, used when a
// frame has no face to anchor on.
func (d Detection) LargestSubject() (Box, string, bool) {
	var best Box
	var name string
	found := false
	for _, l := range d.Labels {
		if genericLabels[strings.ToLower(l.Name)] {
			continue
		}
		for _, b := range l.Boxes {
			if !found || b.Area() > best.Area() {
				best, name, found = b, l.Name, true
			}
		}
	}
	return best, name, found
}

// InterestingLabels counts distinct specific labels, ignoring generic ones
// like "Person" or "Clothing" that say nothing about what the video is about.
func (d Detection) InterestingLabels() int {
	n := 0
	for _, l := range d.Labels {
		if !genericLabels[strings.ToLower(l.Name)] {
			n++
		}
	}
	return n
}

var genericLabels = map[string]bool{
	"person": true, "human": true, "people": true, "face": true, "head": true,
	"adult": true, "male": true, "female": true, "man": true, "woman": true,
	"boy": true, "girl": true, "child": true, "kid": true, "baby": true,
	"portrait": true, "photography": true, "photo": true, "selfie": true,
	"clothing": true, "apparel": true, "neck": true, "body part": true,
	"hand": true, "finger": true, "smile": true, "happy": true,
	"indoors": true, "outdoors": true, "text": true, "electronics": true,
}

// Detector looks at one image. Implementations should return an empty
// Detection, not an error, when nothing is visible.
type Detector interface {
	Detect(ctx context.Context, jpeg []byte) (Detection, error)
}

// RekognitionAPI is the slice of Rekognition the detector uses.
type RekognitionAPI interface {
	DetectFaces(ctx context.Context, in *rekognition.DetectFacesInput, optFns ...func(*rekognition.Options)) (*rekognition.DetectFacesOutput, error)
	DetectLabels(ctx context.Context, in *rekognition.DetectLabelsInput, optFns ...func(*rekognition.Options)) (*rekognition.DetectLabelsOutput, error)
}

// RekognitionDetector finds faces (with emotion) and objects using AWS
// Rekognition, the same service the Shorts renderer already uses to follow a
// speaker. It avoids shipping OpenCV, which would need cgo and break the
// project's static builds.
type RekognitionDetector struct {
	Client RekognitionAPI
}

// minFaceConfidence drops detections that are usually posters or textures.
const minFaceConfidence = 80.0

// minLabelConfidence keeps only labels the service is sure about.
const minLabelConfidence = 80.0

// Detect runs face and label detection on one JPEG.
func (d *RekognitionDetector) Detect(ctx context.Context, jpeg []byte) (Detection, error) {
	img := &types.Image{Bytes: jpeg}
	var det Detection

	faces, err := d.Client.DetectFaces(ctx, &rekognition.DetectFacesInput{
		Image:      img,
		Attributes: []types.Attribute{types.AttributeAll},
	})
	if err != nil {
		return Detection{}, fmt.Errorf("rekognition DetectFaces: %w", err)
	}
	for _, f := range faces.FaceDetails {
		if f.BoundingBox == nil || f.Confidence == nil || float64(*f.Confidence) < minFaceConfidence {
			continue
		}
		face := Face{
			Box:        boxFrom(f.BoundingBox),
			Confidence: float64(*f.Confidence),
		}
		if f.EyesOpen != nil && f.EyesOpen.Value {
			face.EyesOpen = true
		}
		for _, e := range f.Emotions {
			if e.Confidence != nil && float64(*e.Confidence) > face.EmotionConf {
				face.Emotion, face.EmotionConf = string(e.Type), float64(*e.Confidence)
			}
		}
		det.Faces = append(det.Faces, face)
	}

	labels, err := d.Client.DetectLabels(ctx, &rekognition.DetectLabelsInput{
		Image:         img,
		MaxLabels:     intPtr(15),
		MinConfidence: float32Ptr(minLabelConfidence),
	})
	if err != nil {
		// Faces are the more valuable signal; keep them if labels fail.
		return det, fmt.Errorf("rekognition DetectLabels: %w", err)
	}
	for _, l := range labels.Labels {
		if l.Name == nil || l.Confidence == nil {
			continue
		}
		label := Label{Name: *l.Name, Confidence: float64(*l.Confidence)}
		for _, inst := range l.Instances {
			if inst.BoundingBox != nil {
				label.Boxes = append(label.Boxes, boxFrom(inst.BoundingBox))
			}
		}
		det.Labels = append(det.Labels, label)
	}
	sort.SliceStable(det.Labels, func(i, j int) bool { return det.Labels[i].Confidence > det.Labels[j].Confidence })
	return det, nil
}

func boxFrom(b *types.BoundingBox) Box {
	return Box{Left: f32(b.Left), Top: f32(b.Top), Width: f32(b.Width), Height: f32(b.Height)}
}

func f32(v *float32) float64 {
	if v == nil {
		return 0
	}
	return float64(*v)
}

func intPtr(v int32) *int32         { return &v }
func float32Ptr(v float32) *float32 { return &v }
