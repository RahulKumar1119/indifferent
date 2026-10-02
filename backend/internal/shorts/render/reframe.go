package render

import (
	"context"
	"fmt"
	"sort"

	"github.com/aws/aws-sdk-go-v2/service/rekognition"
	"github.com/aws/aws-sdk-go-v2/service/rekognition/types"
)

// FaceBox is a normalized (0-1) bounding box for one detected face.
type FaceBox struct {
	Left, Top, Width, Height float64
}

// CenterX returns the horizontal center of the box in normalized units.
func (b FaceBox) CenterX() float64 { return b.Left + b.Width/2 }

// Area returns the box area in normalized units, used to pick the dominant
// face when several people share a frame.
func (b FaceBox) Area() float64 { return b.Width * b.Height }

// FaceDetector finds faces in a JPEG/PNG image. Implementations must return
// an empty slice (not an error) when no face is visible.
type FaceDetector interface {
	DetectFaces(ctx context.Context, image []byte) ([]FaceBox, error)
}

// minFaceConfidence drops low-confidence detections that usually come from
// background texture, posters, or partial profiles.
const minFaceConfidence = 50.0

// RekognitionAPI is the subset of the AWS Rekognition API used for reframing.
type RekognitionAPI interface {
	DetectFaces(ctx context.Context, in *rekognition.DetectFacesInput, optFns ...func(*rekognition.Options)) (*rekognition.DetectFacesOutput, error)
}

// RekognitionFaceDetector implements FaceDetector with AWS Rekognition
// DetectFaces. Construct it with NewRekognitionFaceDetector.
type RekognitionFaceDetector struct {
	Client RekognitionAPI
}

// NewRekognitionFaceDetector builds a FaceDetector backed by the given
// Rekognition client.
func NewRekognitionFaceDetector(client RekognitionAPI) *RekognitionFaceDetector {
	return &RekognitionFaceDetector{Client: client}
}

// DetectFaces returns the normalized bounding boxes of faces visible in the
// image, filtered by confidence.
func (d *RekognitionFaceDetector) DetectFaces(ctx context.Context, image []byte) ([]FaceBox, error) {
	out, err := d.Client.DetectFaces(ctx, &rekognition.DetectFacesInput{
		Image: &types.Image{Bytes: image},
	})
	if err != nil {
		return nil, fmt.Errorf("rekognition DetectFaces failed: %w", err)
	}
	boxes := make([]FaceBox, 0, len(out.FaceDetails))
	for _, f := range out.FaceDetails {
		if f.Confidence == nil || float64(*f.Confidence) < minFaceConfidence {
			continue
		}
		if f.BoundingBox == nil {
			continue
		}
		bb := f.BoundingBox
		boxes = append(boxes, FaceBox{
			Left:   float64(derefF32(bb.Left)),
			Top:    float64(derefF32(bb.Top)),
			Width:  float64(derefF32(bb.Width)),
			Height: float64(derefF32(bb.Height)),
		})
	}
	return boxes, nil
}

// derefF32 safely dereferences SDK float32 pointers (nil => 0).
func derefF32(v *float32) float32 {
	if v == nil {
		return 0
	}
	return *v
}

// maxSampleFrames caps Rekognition calls per clip: 8 evenly spaced frames is
// enough to track a subject through a <=60s segment at ~$0.01.
const maxSampleFrames = 8

// sampleTimes returns up to maxSampleFrames timestamps evenly spread over
// [start, end] for subject sampling. It always returns at least one time.
func sampleTimes(start, end float64) []float64 {
	if end <= start {
		return []float64{start}
	}
	n := maxSampleFrames
	// Roughly one sample per 5s, but at least 3 and at most maxSampleFrames.
	if span := end - start; span/5 < float64(n) {
		n = int(span / 5)
		if n < 3 {
			n = 3
		}
	}
	times := make([]float64, 0, n)
	for i := 0; i < n; i++ {
		times = append(times, start+(end-start)*float64(i)/float64(n))
	}
	return times
}

// cropWindowWidth returns the width of the 9:16 crop window for a source of
// height h: the widest full-height 9:16 column.
func cropWindowWidth(h int) int {
	return h * 9 / 16
}

// ComputeCropX maps per-frame face detections to a single horizontal crop
// offset (pixels) for the 9:16 window. Each frame contributes its largest
// face; the window centers on the median of those centers so one bad frame
// cannot yank the framing. It returns ok=false when no usable face was seen
// and the caller should fall back to a centered crop.
//
// x is clamped to [0, srcW-windowW] and rounded down to an even value
// (required for yuv420p encoding).
func ComputeCropX(frames [][]FaceBox, srcW, srcH int) (x int, ok bool) {
	windowW := cropWindowWidth(srcH)
	if windowW >= srcW {
		// Source is already vertical (or square-narrow): the window covers
		// the full width, no panning possible.
		return 0, true
	}
	centers := make([]float64, 0, len(frames))
	for _, faces := range frames {
		best := -1
		for i, b := range faces {
			if b.Width <= 0 || b.Height <= 0 {
				continue
			}
			if best == -1 || b.Area() > faces[best].Area() {
				best = i
			}
		}
		if best != -1 {
			centers = append(centers, faces[best].CenterX())
		}
	}
	if len(centers) == 0 {
		return 0, false
	}
	sort.Float64s(centers)
	median := centers[len(centers)/2]

	offset := int(median*float64(srcW)) - windowW/2
	if offset < 0 {
		offset = 0
	}
	if max := srcW - windowW; offset > max {
		offset = max
	}
	return offset &^ 1, true
}
