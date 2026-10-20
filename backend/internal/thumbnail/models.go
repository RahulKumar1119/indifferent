// Package thumbnail turns an uploaded video into three YouTube-style
// thumbnail suggestions (issue #77).
//
// The pipeline is: probe the video, extract candidate frames with FFmpeg,
// filter and score them locally, ask Rekognition about faces and objects on
// the best few, ask a Bedrock vision model for the topic and a headline from
// those frames plus the transcript, plan three different compositions, and
// render them with the application's own text renderer so spelling and
// branding are always exact.
//
// Everything that talks to AWS or runs a process sits behind a small
// interface, so the selection, planning and rendering logic is covered by
// ordinary unit tests.
package thumbnail

import (
	"errors"
	"fmt"
	"strings"
)

// Format is a thumbnail aspect ratio.
type Format string

const (
	// FormatLandscape is the standard YouTube video thumbnail, 1280x720.
	FormatLandscape Format = "16:9"
	// FormatPortrait is the YouTube Shorts / vertical cover, 720x1280.
	FormatPortrait Format = "9:16"
)

// Size returns the output size in pixels.
func (f Format) Size() (w, h int) {
	if f == FormatPortrait {
		return 720, 1280
	}
	return 1280, 720
}

// Aspect is width divided by height.
func (f Format) Aspect() float64 {
	w, h := f.Size()
	return float64(w) / float64(h)
}

// ParseFormat validates a format string. The empty string is allowed and
// means "choose from the video's shape".
func ParseFormat(s string) (Format, error) {
	switch Format(strings.TrimSpace(s)) {
	case "":
		return "", nil
	case FormatLandscape:
		return FormatLandscape, nil
	case FormatPortrait:
		return FormatPortrait, nil
	}
	return "", fmt.Errorf("unknown format %q: must be 16:9 or 9:16", s)
}

// FormatFor picks the format that suits a video's displayed size: vertical
// footage gets a vertical thumbnail, everything else gets 16:9.
func FormatFor(width, height int) Format {
	if height > width {
		return FormatPortrait
	}
	return FormatLandscape
}

// Style is a look preset for the rendered thumbnails.
type Style string

const (
	StyleViral       Style = "viral"
	StyleTechnology  Style = "technology"
	StyleEducational Style = "educational"
	StyleNews        Style = "news"
	StylePodcast     Style = "podcast"
	StyleCinematic   Style = "cinematic"
	StyleMinimal     Style = "minimal"
)

// Styles lists every preset in display order.
func Styles() []Style {
	return []Style{StyleViral, StyleTechnology, StyleEducational, StyleNews, StylePodcast, StyleCinematic, StyleMinimal}
}

// ParseStyle validates a style string. The empty string is allowed and means
// "let the analysis suggest one".
func ParseStyle(s string) (Style, error) {
	st := Style(strings.ToLower(strings.TrimSpace(s)))
	if st == "" {
		return "", nil
	}
	for _, known := range Styles() {
		if st == known {
			return st, nil
		}
	}
	return "", fmt.Errorf("unknown style %q", s)
}

// Concept is the composition idea behind one thumbnail.
type Concept string

const (
	ConceptFace     Concept = "face"     // a person with a strong expression, large headline
	ConceptSubject  Concept = "subject"  // the main object or scene, headline in a band
	ConceptCreative Concept = "creative" // best frame over an enhanced backdrop with effects
)

// Status is where a generation run stands.
type Status string

const (
	StatusQueued    Status = "queued"
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
)

// InFlight reports whether a run is still working.
func (s Status) InFlight() bool { return s == StatusQueued || s == StatusRunning }

// Stage is the step a running generation is on. The stored value drives the
// progress list in the UI.
type Stage string

const (
	StageExtracting Stage = "extracting" // probing the video and pulling frames
	StageSelecting  Stage = "selecting"  // filtering, scoring, detecting faces
	StageAnalyzing  Stage = "analyzing"  // transcript and frames to the AI
	StageConcepts   Stage = "concepts"   // planning three compositions
	StageRendering  Stage = "rendering"  // drawing text and saving images
)

// Machine-readable failure classes. The API returns them and the UI maps
// each to guidance and a retry option.
const (
	CodeNoVideoTrack     = "NO_VIDEO_TRACK"
	CodeSourceUnreadable = "SOURCE_UNREADABLE"
	CodeNoSuitableFrames = "NO_SUITABLE_FRAMES"
	CodeAITimeout        = "AI_TIMEOUT"
	CodeAIFailed         = "AI_FAILED"
	CodeRenderFailed     = "RENDER_FAILED"
	CodeTaskFailed       = "TASK_FAILED"
	CodePipelineError    = "PIPELINE_ERROR"
)

// Error carries a failure class alongside the underlying cause.
type Error struct {
	Code    string
	Message string
	Err     error
}

func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *Error) Unwrap() error { return e.Err }

// Errorf builds an Error. message is the user-facing sentence.
func Errorf(code, message string, cause error) *Error {
	return &Error{Code: code, Message: message, Err: cause}
}

// CodeOf extracts the failure class from err, or CodeTaskFailed for errors
// that were never classified.
func CodeOf(err error) string {
	var te *Error
	if errors.As(err, &te) {
		return te.Code
	}
	return CodeTaskFailed
}

// MessageOf returns a sentence that is safe to show to a user.
func MessageOf(err error) string {
	var te *Error
	if errors.As(err, &te) && te.Message != "" {
		return te.Message
	}
	return "Thumbnail generation failed. Please try again."
}

// Asset is one finished thumbnail.
type Asset struct {
	ID            string  `json:"id"` // thumb-01, thumb-02, thumb-03
	Concept       Concept `json:"concept"`
	Style         Style   `json:"style"`
	Format        Format  `json:"format"`
	Key           string  `json:"key"` // S3 key
	Headline      string  `json:"headline"`
	SecondaryText string  `json:"secondaryText,omitempty"`
	FrameTime     float64 `json:"frameTime"` // seconds into the source video
	Width         int     `json:"width"`
	Height        int     `json:"height"`
}

// Funnel records how many frames survived each filtering step, for example
// 48 extracted, 36 usable, 20 unique, 8 candidates.
type Funnel struct {
	Extracted  int `json:"extracted"`
	Usable     int `json:"usable"`
	Unique     int `json:"unique"`
	Candidates int `json:"candidates"`
}

// Analysis is what the AI understood about the video.
type Analysis struct {
	Topic          string   `json:"topic"`
	MainSubject    string   `json:"mainSubject"`
	Emotion        string   `json:"emotion"`
	Headline       string   `json:"headline"`
	SecondaryText  string   `json:"secondaryText,omitempty"`
	Keywords       []string `json:"keywords,omitempty"`
	SuggestedStyle Style    `json:"suggestedStyle,omitempty"`
	VisualConcept  string   `json:"visualConcept,omitempty"`
	// BestFrameTime is the moment (seconds) the model judged most important,
	// or -1 when it did not pick one.
	BestFrameTime float64 `json:"bestFrameTime"`
}

// Set is the stored record of a job's thumbnails: the latest run's state plus
// its results. DynamoDB: PK = "USER#{userId}", SK = "THUMBS#{jobId}".
type Set struct {
	JobID          string
	UserID         string
	Status         Status
	Stage          Stage
	ErrorCode      string
	Error          string
	Style          Style
	Format         Format
	Generation     int
	Thumbnails     []Asset
	SelectedID     string
	Analysis       *Analysis
	Funnel         *Funnel
	TranscriptUsed bool
	DetectionUsed  bool
	CreatedAt      string
	UpdatedAt      string
}

// Selected returns the chosen thumbnail, if any.
func (s *Set) Selected() *Asset {
	if s == nil || s.SelectedID == "" {
		return nil
	}
	for i := range s.Thumbnails {
		if s.Thumbnails[i].ID == s.SelectedID {
			return &s.Thumbnails[i]
		}
	}
	return nil
}

// Find returns the thumbnail with the given ID.
func (s *Set) Find(id string) *Asset {
	if s == nil {
		return nil
	}
	for i := range s.Thumbnails {
		if s.Thumbnails[i].ID == id {
			return &s.Thumbnails[i]
		}
	}
	return nil
}

// ObjectKey is where a generated thumbnail lives in the assets bucket. It sits
// under the job's existing shorts/ prefix so deleting the job removes it, and
// each generation gets its own folder so a regenerate never serves a cached
// older image from the same URL.
func ObjectKey(userID, jobID string, generation, n int) string {
	return fmt.Sprintf("shorts/%s/%s/thumbnails/g%d/thumbnail-%02d.jpg", userID, jobID, generation, n)
}

// GenerationPrefix is the folder holding one generation's images.
func GenerationPrefix(userID, jobID string, generation int) string {
	return fmt.Sprintf("shorts/%s/%s/thumbnails/g%d/", userID, jobID, generation)
}

// AssetID is the ID for the nth (1-based) thumbnail of a run.
func AssetID(n int) string { return fmt.Sprintf("thumb-%02d", n) }
