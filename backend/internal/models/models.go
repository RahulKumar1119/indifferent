// Package models defines the shared data structures used across
// the TXT-to-Video SaaS pipeline stages and API.
package models

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
	ProjectID string `json:"projectId"`
	JSONKey   string `json:"jsonKey"`   // S3 key for parsed JSON
	Template  string `json:"template"` // "classic", "modern", etc.
}

// SlideGenOutput is the result of slide generation.
type SlideGenOutput struct {
	ProjectID string   `json:"projectId"`
	SlideKeys []string `json:"slideKeys"` // S3 keys for PNG files
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
	Error         string             `json:"error,omitempty"`
	CreatedAt     string             `json:"createdAt"`
	UpdatedAt     string             `json:"updatedAt"`
	CompletedAt   string             `json:"completedAt,omitempty"`
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
	UserID         string          `json:"userId"`
	JobID          string          `json:"jobId"`
	ProjectID      string          `json:"projectId,omitempty"`
	Status         string          `json:"status"` // uploaded|transcribing|ranking|rendering|completed|failed
	FileType       string          `json:"fileType"`
	SourceDuration float64         `json:"sourceDuration"`
	SourceKey      string          `json:"sourceKey"`
	TranscriptKey  string          `json:"transcriptKey,omitempty"`
	Segments       []RankedSegment `json:"segments,omitempty"`
	Clips          []Clip          `json:"clips,omitempty"`
	Error          string          `json:"error,omitempty"`
	CreatedAt      string          `json:"createdAt"`
	UpdatedAt      string          `json:"updatedAt"`
	CompletedAt    string          `json:"completedAt,omitempty"`
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
}

// Transcript is the normalized word-level transcription used by ranking/rendering.
type Transcript struct {
	Text  string `json:"text"`
	Words []Word `json:"words"`
}
