// Package models defines the shared data structures used across
// the TXT-to-Video SaaS pipeline stages and API.
package models

import (
	"fmt"
	"math"
	"regexp"
	"strings"
)

// ParserInput represents the event triggering the parser.
type ParserInput struct {
	ProjectID string `json:"projectId"`
	UserID    string `json:"userId"`
	S3Key     string `json:"s3Key"` // path to uploaded TXT in S3
}

// ParserOutput represents the parser's result.
type ParserOutput struct {
	ProjectID string    `json:"projectId"`
	S3Key     string    `json:"s3Key"` // path to structured JSON in S3
	Questions int       `json:"questions"`
	Format    string    `json:"format"` // "numbered", "bulleted", "tabbed"
	Warnings  []Warning `json:"warnings,omitempty"`
}

// Warning represents a skipped malformed entry.
type Warning struct {
	Line   int    `json:"line"`
	Reason string `json:"reason"`
}

// Question represents a parsed multiple-choice question.
type Question struct {
	Index          int      `json:"index"`
	Text           string   `json:"text"`
	Options        []Option `json:"options"`
	CorrectIndex   int      `json:"correctIndex"`   // Kept for backward compat, first correct answer (-1 if none)
	CorrectIndices []int    `json:"correctIndices"` // All correct answer indices
}

// Option represents an answer choice.
type Option struct {
	Label string `json:"label"` // "A", "B", "C", "D"
	Text  string `json:"text"`
}

// SlideGenInput is the event for slide generation.
type SlideGenInput struct {
	ProjectID     string        `json:"projectId"`
	JSONKey       string        `json:"jsonKey"`  // S3 key for parsed JSON
	Template      string        `json:"template"` // "classic", "modern", etc.
	DesignOptions DesignOptions `json:"designOptions,omitempty"`
}

// SlideGenOutput is the result of slide generation.
type SlideGenOutput struct {
	ProjectID string   `json:"projectId"`
	SlideKeys []string `json:"slideKeys"`        // S3 keys for PNG files
	Failed    []int    `json:"failed,omitempty"` // indices of failed slides
}

// NarratorInput is the event for narration generation.
type NarratorInput struct {
	ProjectID string `json:"projectId"`
	JSONKey   string `json:"jsonKey"`
	Voice     string `json:"voice"` // "Joanna", "Matthew", etc.
}

// NarratorOutput is the result of narration generation.
type NarratorOutput struct {
	ProjectID string   `json:"projectId"`
	AudioKeys []string `json:"audioKeys"` // S3 keys for MP3 files
	Failed    []int    `json:"failed,omitempty"`
}

// RendererInput is the event for video rendering.
type RendererInput struct {
	ProjectID string   `json:"projectId"`
	SlideKeys []string `json:"slideKeys"`
	AudioKeys []string `json:"audioKeys"`
	JSONKey   string   `json:"jsonKey"` // for answer data
	LogoKey   string   `json:"logoKey,omitempty"`
}

// RendererOutput is the result of video rendering.
type RendererOutput struct {
	ProjectID    string `json:"projectId"`
	VideoKey     string `json:"videoKey"`     // S3 key for MP4
	ThumbnailKey string `json:"thumbnailKey"` // S3 key for thumbnail
}

// AuthTokens represents the tokens issued to a user.
type AuthTokens struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	ExpiresIn    int64  `json:"expiresIn"` // seconds
}

// JWTClaims represents the JWT payload.
type JWTClaims struct {
	UserID string `json:"userId"`
	Email  string `json:"email"`
	Exp    int64  `json:"exp"`
	Iat    int64  `json:"iat"`
}

// APIError represents a structured error response from the API.
type APIError struct {
	Code    string `json:"code"`              // Machine-readable error code
	Message string `json:"message"`           // Human-readable description
	Details any    `json:"details,omitempty"` // Additional context
}

// Project represents a video conversion project stored in DynamoDB. A project
// is the unified home for all three tools: quiz video settings live on the
// record itself, shorts runs link via ShortsJobIDs, and watermark preferences
// persist in Watermark so every visit shows the saved configuration.
type Project struct {
	UserID        string             `json:"userId"`
	ProjectID     string             `json:"id"`
	Name          string             `json:"name"`
	Template      string             `json:"template"`
	DesignOptions DesignOptions      `json:"designOptions,omitempty"`
	Voice         string             `json:"voice"`
	Status        string             `json:"status"`
	TxtKey        string             `json:"txtKey,omitempty"`
	JSONKey       string             `json:"jsonKey,omitempty"`
	VideoKey      string             `json:"videoKey,omitempty"`
	ThumbnailKey  string             `json:"thumbnailKey,omitempty"`
	QuestionCount int                `json:"questionCount,omitempty"`
	Warnings      []Warning          `json:"warnings,omitempty"`
	ShortsJobIDs  []string           `json:"shortsJobIds,omitempty"`
	Watermark     *WatermarkSettings `json:"watermark,omitempty"`
	Branding      *Branding          `json:"branding,omitempty"`
	Error         string             `json:"error,omitempty"`
	CreatedAt     string             `json:"createdAt"`
	UpdatedAt     string             `json:"updatedAt"`
	CompletedAt   string             `json:"completedAt,omitempty"`
	// VideoURL/ThumbnailURL are presigned GET URLs resolved on read for
	// completed projects only. Never persisted: projectToItem/itemToProject
	// intentionally ignore them.
	VideoURL     string `json:"videoUrl,omitempty"`
	ThumbnailURL string `json:"thumbnailUrl,omitempty"`
}

// Branding is a project's custom identity burned into its videos: an
// uploaded logo overlay plus a channel handle shown on clips.
type Branding struct {
	LogoKey     string `json:"logoKey,omitempty"`
	ChannelName string `json:"channelName,omitempty"`
}

// WatermarkSettings persists a user's watermark preferences on their project
// (mirrors the controls of the in-browser watermark tool).
type WatermarkSettings struct {
	Text     string  `json:"text"`
	FontSize float64 `json:"fontSize,omitempty"`
	Opacity  float64 `json:"opacity,omitempty"`
	Color    string  `json:"color,omitempty"`
	Rotation float64 `json:"rotation,omitempty"`
	X        float64 `json:"x,omitempty"`
	Y        float64 `json:"y,omitempty"`
}

// ShortsJob is a single AI Shorts processing run, stored in DynamoDB.
// PK = "USER#{UserID}", SK = "SHORTS#{JobID}".
type ShortsJob struct {
	UserID    string `json:"userId"`
	JobID     string `json:"jobId"`
	ProjectID string `json:"projectId,omitempty"`
	Status    string `json:"status"` // uploaded|transcribing|ranking|rendering|completed|failed
	FileType  string `json:"fileType"`
	// Language is the transcription selection: auto|en-US|en-IN|hi-IN.
	// Empty on jobs created before language support (treated as auto).
	Language       string          `json:"language,omitempty"`
	SourceDuration float64         `json:"sourceDuration"`
	SourceKey      string          `json:"sourceKey"`
	TranscriptKey  string          `json:"transcriptKey,omitempty"`
	Segments       []RankedSegment `json:"segments,omitempty"`
	Clips          []Clip          `json:"clips,omitempty"`
	// Progress is 0-100 within the current stage (render tasks refine it via
	// ffmpeg output; status transitions reset it to the stage baseline).
	Progress       int    `json:"progress,omitempty"`
	ProgressDetail string `json:"progressDetail,omitempty"`
	// FailedStage records which stage was running when the job failed
	// (transcribing|ranking|rendering); ErrorCode is a machine-readable
	// failure class (VALIDATION_*|CANCELLED|RANKING_BACKEND|TRANSIENT_*|
	// INFRASTRUCTURE) driving retry guidance. Both feed safe retries.
	FailedStage string `json:"failedStage,omitempty"`
	ErrorCode   string `json:"errorCode,omitempty"`
	// Caption is the reusable caption style chosen for this job's renders
	// (preset + user overrides). Zero value renders the legacy bold look.
	Caption CaptionStyle `json:"caption,omitempty"`
	// Effects holds the phase 1 video effect settings (nil = no effects).
	// Honored by renders and retries, like Caption.
	Effects *VideoEffects `json:"effects,omitempty"`
	// ClipEdits maps clipId to the user's boundary/caption overrides,
	// honored by renders and retries.
	ClipEdits   map[string]ClipEdit `json:"clipEdits,omitempty"`
	Error       string              `json:"error,omitempty"`
	CreatedAt   string              `json:"createdAt"`
	UpdatedAt   string              `json:"updatedAt"`
	CompletedAt string              `json:"completedAt,omitempty"`
}

// RankedSegment is a candidate viral moment with its rank and score.
type RankedSegment struct {
	ClipID       string  `json:"clipId"`       // deterministic ID (clip-<rank>) assigned at validation
	Start        float64 `json:"start"`        // seconds, >= 0
	End          float64 `json:"end"`          // seconds, <= sourceDuration, > Start
	Score        float64 `json:"score"`        // engagement score 0..1
	Rank         int     `json:"rank"`         // 1 = highest score
	HookText     string  `json:"hookText"`     // short label describing the moment
	CaptionWords []Word  `json:"captionWords"` // word-level timings within [Start, End]
}

// Word is a single transcript token with its timing (relative to source start).
type Word struct {
	Text  string  `json:"text"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
}

// Clip is a rendered 9:16 vertical clip.
type Clip struct {
	ClipID   string  `json:"clipId"`
	S3Key    string  `json:"s3Key"`
	Rank     int     `json:"rank"`
	Score    float64 `json:"score"`
	Duration float64 `json:"duration"`
	// HookText carries the model's short label for copy/share captions.
	HookText string `json:"hookText,omitempty"`
	// Start/End locate the clip in the source for future caption rebuilds.
	Start float64 `json:"start,omitempty"`
	End   float64 `json:"end,omitempty"`
}

// CaptionStyle is the stored per-job caption choice: a preset plus user
// overrides. Zero value resolves to the legacy bold look so old jobs render
// exactly as before.
type CaptionStyle struct {
	Preset    string `json:"preset,omitempty"`    // minimal|bold|karaoke|clean
	FontSize  int    `json:"fontSize,omitempty"`  // libass script units, 8-72
	Color     string `json:"color,omitempty"`     // #RRGGBB text color
	Highlight string `json:"highlight,omitempty"` // #RRGGBB karaoke sweep color
	Position  string `json:"position,omitempty"`  // top|middle|bottom
}

var hexColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// Validate rejects unknown presets, out-of-range sizes, malformed colors,
// and unknown positions. Empty fields are allowed (preset defaults apply).
func (s CaptionStyle) Validate() error {
	switch s.Preset {
	case "", "minimal", "bold", "karaoke", "clean":
	default:
		return fmt.Errorf("unknown caption preset %q: must be minimal, bold, karaoke or clean", s.Preset)
	}
	if s.FontSize != 0 && (s.FontSize < 8 || s.FontSize > 72) {
		return fmt.Errorf("font size %d out of range 8-72", s.FontSize)
	}
	for _, c := range []string{s.Color, s.Highlight} {
		if c != "" && !hexColor.MatchString(c) {
			return fmt.Errorf("color %q must be #RRGGBB", c)
		}
	}
	switch s.Position {
	case "", "top", "middle", "bottom":
	default:
		return fmt.Errorf("unknown caption position %q: must be top, middle or bottom", s.Position)
	}
	return nil
}

// ClipEdit is a user's override for one clip: trimmed boundaries and
// corrected caption text. Word timings are preserved as-is (text-only
// edits); words falling outside [Start, End] are filtered at render time.
type ClipEdit struct {
	Start     float64 `json:"start"`
	End       float64 `json:"end"`
	Words     []Word  `json:"words,omitempty"`
	UpdatedAt string  `json:"updatedAt,omitempty"`
}

// Validate checks bounds against the source duration and word sanity.
// An empty Words list means "trim only, keep AI captions".
func (e ClipEdit) Validate(sourceDuration float64) error {
	if math.IsNaN(e.Start) || math.IsInf(e.Start, 0) || math.IsNaN(e.End) || math.IsInf(e.End, 0) {
		return fmt.Errorf("clip boundaries must be finite numbers")
	}
	if e.Start < 0 || e.End <= e.Start {
		return fmt.Errorf("require 0 <= start < end, got start=%.1f end=%.1f", e.Start, e.End)
	}
	if e.End-e.Start < 1.0 {
		return fmt.Errorf("clip must be at least 1 second long")
	}
	if sourceDuration > 0 && e.End > sourceDuration+0.5 {
		return fmt.Errorf("end %.1fs exceeds source duration %.0fs", e.End, sourceDuration)
	}
	for i, w := range e.Words {
		if math.IsNaN(w.Start) || math.IsNaN(w.End) || math.IsInf(w.Start, 0) || math.IsInf(w.End, 0) {
			return fmt.Errorf("word %d has non-finite timing", i+1)
		}
		if w.End <= w.Start {
			return fmt.Errorf("word %d ends before it starts", i+1)
		}
		if strings.TrimSpace(w.Text) == "" {
			return fmt.Errorf("word %d has empty text", i+1)
		}
	}
	return nil
}

type Transcript struct {
	Text  string `json:"text"`
	Words []Word `json:"words"`
}
