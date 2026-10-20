package thumbnail

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/rekognition"
	"github.com/aws/aws-sdk-go-v2/service/rekognition/types"
)

type fakeRekognition struct {
	faces     *rekognition.DetectFacesOutput
	labels    *rekognition.DetectLabelsOutput
	facesErr  error
	labelsErr error
	faceIn    *rekognition.DetectFacesInput
}

func (f *fakeRekognition) DetectFaces(_ context.Context, in *rekognition.DetectFacesInput, _ ...func(*rekognition.Options)) (*rekognition.DetectFacesOutput, error) {
	f.faceIn = in
	return f.faces, f.facesErr
}

func (f *fakeRekognition) DetectLabels(_ context.Context, _ *rekognition.DetectLabelsInput, _ ...func(*rekognition.Options)) (*rekognition.DetectLabelsOutput, error) {
	return f.labels, f.labelsErr
}

func fp(v float32) *float32 { return &v }
func sp(v string) *string   { return &v }

func bbox(l, t, w, h float32) *types.BoundingBox {
	return &types.BoundingBox{Left: fp(l), Top: fp(t), Width: fp(w), Height: fp(h)}
}

func TestRekognitionDetectorMapsFacesAndLabels(t *testing.T) {
	c := &fakeRekognition{
		faces: &rekognition.DetectFacesOutput{FaceDetails: []types.FaceDetail{
			{
				BoundingBox: bbox(0.4, 0.2, 0.2, 0.35), Confidence: fp(99),
				EyesOpen: &types.EyeOpen{Value: true},
				Emotions: []types.Emotion{
					{Type: types.EmotionNameCalm, Confidence: fp(10)},
					{Type: types.EmotionNameSurprised, Confidence: fp(80)},
				},
			},
			{BoundingBox: bbox(0.1, 0.1, 0.05, 0.05), Confidence: fp(40)}, // too unsure: dropped
			{Confidence: fp(95)}, // no box: dropped
		}},
		labels: &rekognition.DetectLabelsOutput{Labels: []types.Label{
			{Name: sp("Desk"), Confidence: fp(85)},
			{Name: sp("Laptop"), Confidence: fp(97), Instances: []types.Instance{{BoundingBox: bbox(0.3, 0.4, 0.3, 0.2)}}},
		}},
	}
	det, err := (&RekognitionDetector{Client: c}).Detect(context.Background(), []byte("jpeg"))
	if err != nil {
		t.Fatal(err)
	}
	if len(det.Faces) != 1 {
		t.Fatalf("faces = %d, want 1 (low-confidence and boxless faces are dropped)", len(det.Faces))
	}
	f := det.Faces[0]
	if f.Emotion != "SURPRISED" || f.EmotionConf != 80 || !f.EyesOpen {
		t.Errorf("face = %+v", f)
	}
	if len(c.faceIn.Attributes) == 0 || c.faceIn.Attributes[0] != types.AttributeAll {
		t.Error("emotions need the ALL attribute set")
	}
	if len(det.Labels) != 2 || det.Labels[0].Name != "Laptop" {
		t.Errorf("labels should be sorted by confidence: %+v", det.Labels)
	}
	if len(det.Labels[0].Boxes) != 1 {
		t.Errorf("laptop boxes = %+v", det.Labels[0].Boxes)
	}
}

func TestRekognitionDetectorKeepsFacesWhenLabelsFail(t *testing.T) {
	c := &fakeRekognition{
		faces:     &rekognition.DetectFacesOutput{FaceDetails: []types.FaceDetail{{BoundingBox: bbox(0.4, 0.2, 0.2, 0.3), Confidence: fp(97)}}},
		labelsErr: errors.New("throttled"),
	}
	det, err := (&RekognitionDetector{Client: c}).Detect(context.Background(), nil)
	if err == nil {
		t.Error("the label failure should still be reported")
	}
	if len(det.Faces) != 1 {
		t.Error("faces found before the label call failed must be kept")
	}
}

func TestRekognitionDetectorFaceFailure(t *testing.T) {
	c := &fakeRekognition{facesErr: errors.New("access denied")}
	if _, err := (&RekognitionDetector{Client: c}).Detect(context.Background(), nil); err == nil {
		t.Error("expected an error")
	}
}

func TestLargestFaceAndSubject(t *testing.T) {
	d := Detection{
		Faces: []Face{{Box: Box{Width: 0.1, Height: 0.1}}, {Box: Box{Width: 0.3, Height: 0.4}}},
		Labels: []Label{
			{Name: "Person", Boxes: []Box{{Width: 0.9, Height: 0.9}}}, // generic: ignored as a subject
			{Name: "Guitar", Boxes: []Box{{Width: 0.2, Height: 0.5}, {Width: 0.1, Height: 0.1}}},
		},
	}
	f, ok := d.LargestFace()
	if !ok || f.Box.Width != 0.3 {
		t.Errorf("largest face = %+v", f)
	}
	b, name, ok := d.LargestSubject()
	if !ok || name != "Guitar" || b.Height != 0.5 {
		t.Errorf("largest subject = %+v %q", b, name)
	}
	if _, ok := (Detection{}).LargestFace(); ok {
		t.Error("empty detection has no face")
	}
	if _, _, ok := (Detection{}).LargestSubject(); ok {
		t.Error("empty detection has no subject")
	}
}
