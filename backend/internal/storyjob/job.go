// Package storyjob is the unit of work behind a Story2Shorts video: the job
// record, the steps that prepare its inputs (copy the chosen media into our
// storage, narrate each scene), and the errors those steps report.
//
// Rendering is in package storyvideo. This package sits between the two: it
// takes a confirmed plan and produces everything the renderer needs.
//
// Every step is safe to run again. A render worker on Fargate Spot can be
// interrupted, and Step Functions then starts it again; a step that finds its
// output already in storage does not repeat the download or the Polly call.
package storyjob

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/rahul/indifferent/backend/internal/models"
	"github.com/rahul/indifferent/backend/internal/narrator"
	"github.com/rahul/indifferent/backend/internal/storyplan"
)

// Job statuses, in the order a healthy job moves through them.
const (
	StatusQueued    = "queued"
	StatusPreparing = "preparing" // copying the chosen media
	StatusNarrating = "narrating"
	StatusRendering = "rendering"
	StatusCompleted = "completed"
	StatusFailed    = "failed"
)

// StageProgress is the progress, out of 100, at which each status begins. A
// status moves it forward and never back: the render stage fills 40 to 99 as
// FFmpeg reports.
var StageProgress = map[string]int{
	StatusQueued:    0,
	StatusPreparing: 5,
	StatusNarrating: 25,
	StatusRendering: 40,
	StatusCompleted: 100,
}

// Terminal reports whether a status is final.
func Terminal(status string) bool {
	return status == StatusCompleted || status == StatusFailed
}

// Job is one story video, from the moment the user confirms a plan to the
// finished MP4.
type Job struct {
	UserID string `json:"userId"`
	JobID  string `json:"jobId"`
	// PlanID and PlanRevision name the plan this job was made from. The plan
	// itself is copied into Scenes, so editing it afterwards changes nothing here.
	PlanID       string `json:"planId"`
	PlanRevision int    `json:"planRevision"`
	Status       string `json:"status"`
	// Progress is 0 to 100.
	Progress int `json:"progress"`
	// Language is the plan's ISO 639-1 code; Voice is the Polly voice.
	Language string              `json:"language"`
	Voice    string              `json:"voice"`
	Caption  models.CaptionStyle `json:"caption,omitempty"`
	Scenes   []Scene             `json:"scenes"`
	// TotalSeconds is the video's length, known once every scene is narrated.
	TotalSeconds float64 `json:"totalSeconds,omitempty"`
	// OutputKey is the finished MP4 in the assets bucket.
	OutputKey string `json:"outputKey,omitempty"`
	// Error says in words fit for the user why the job failed. ErrorCode is a
	// stable code for the UI and for retry advice; FailedStage is the status
	// the job was in.
	Error       string `json:"error,omitempty"`
	ErrorCode   string `json:"errorCode,omitempty"`
	FailedStage string `json:"failedStage,omitempty"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
	CompletedAt string `json:"completedAt,omitempty"`
}

// Scene is one scene of the job.
type Scene struct {
	// Narration is the text that is spoken, copied from the plan.
	Narration string `json:"narration"`
	// Pick is the stock asset the user chose. It is a reference only; the file
	// is in Media once copied.
	Pick storyplan.MediaRef `json:"pick"`
	// Media is the copy of the asset in our storage, set when it is made.
	Media *Media `json:"media,omitempty"`
	// Voice is the narration audio, set when it is made.
	Voice *Voice `json:"voice,omitempty"`
}

// Media is a stock asset copied into our storage.
type Media struct {
	Key    string `json:"key"`
	Kind   string `json:"kind"` // "photo" or "video"
	Format string `json:"format"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	// Credit and PageURL are the provider's attribution for the asset.
	Credit  string `json:"credit,omitempty"`
	PageURL string `json:"pageUrl,omitempty"`
}

// Voice is a scene's narration in our storage.
type Voice struct {
	AudioKey string `json:"audioKey"`
	// MarksKey is Polly's word timings. It is empty when they could not be
	// had, and captions then fall back to an even spread across the audio.
	MarksKey string `json:"marksKey,omitempty"`
	// Seconds is the audio's length, measured from the file.
	Seconds float64 `json:"seconds"`
}

// Storage layout. Working files expire: the stock files are copied for the
// render and are not kept (see stockmedia/TERMS.md), and the narration can be
// made again. Only the finished video is kept.
const (
	// WorkPrefix is the prefix the bucket's lifecycle rule expires after 7 days.
	WorkPrefix = "storyvideo-work/"
	// OutputPrefix holds finished videos.
	OutputPrefix = "storyvideo/"
)

// WorkKey is the key of a working file of a scene. n is the scene's number,
// from 1; name is e.g. "media.mp4" or "voice.mp3".
func WorkKey(userID, jobID string, n int, name string) string {
	return fmt.Sprintf("%s%s/%s/scene-%02d-%s", WorkPrefix, userID, jobID, n, name)
}

// OutputKey is the key of the finished video.
func OutputKey(userID, jobID string) string {
	return fmt.Sprintf("%s%s/%s/video.mp4", OutputPrefix, userID, jobID)
}

// Narration languages. Polly's standard engine has voices for English and
// Hindi among the languages the planner can write in; a story in any other
// language is refused when the job is made rather than narrated badly.
var voicesByLanguage = map[string][]string{
	"en": {"Joanna", "Matthew", "Amy", "Brian", "Aditi"},
	"hi": {"Aditi"},
}

// DefaultVoice is the voice used when the user does not choose one.
func DefaultVoice(language string) string {
	if v := voicesByLanguage[language]; len(v) > 0 {
		return v[0]
	}
	return ""
}

// UnsupportedLanguageError means the story is in a language that cannot be narrated.
type UnsupportedLanguageError struct{ Language string }

func (e *UnsupportedLanguageError) Error() string {
	return fmt.Sprintf("narration is not available in language %q", e.Language)
}

// UserMessage is the explanation shown to the user.
func (e *UnsupportedLanguageError) UserMessage() string {
	return "Narration is available in English and Hindi. This story is in another language."
}

// VoiceFor picks the voice for a story in a language. requested may be empty,
// meaning the default. A voice that cannot read the language (an English voice
// given Devanagari) is refused: Polly would answer without error and the
// narration would be wrong.
func VoiceFor(language, requested string) (string, error) {
	allowed, ok := voicesByLanguage[language]
	if !ok {
		return "", &UnsupportedLanguageError{Language: language}
	}
	if requested == "" {
		return allowed[0], nil
	}
	for _, v := range allowed {
		if v == requested && narrator.IsValidVoice(v) {
			return v, nil
		}
	}
	return "", fmt.Errorf("the voice %q cannot narrate a story in %q: choose %s", requested, language, strings.Join(allowed, ", "))
}

// Voices lists the voices a story in a language can use, default first.
func Voices(language string) []string {
	return append([]string(nil), voicesByLanguage[language]...)
}

// MissingPicturesError means some scenes of the plan have no picture chosen.
type MissingPicturesError struct{ Scenes []int }

func (e *MissingPicturesError) Error() string {
	return fmt.Sprintf("scenes without a picture: %v", e.Scenes)
}

// UserMessage names the scenes.
func (e *MissingPicturesError) UserMessage() string {
	nums := make([]string, len(e.Scenes))
	for i, n := range e.Scenes {
		nums[i] = fmt.Sprint(n)
	}
	if len(nums) == 1 {
		return fmt.Sprintf("Scene %s has no picture. Choose one for it and try again.", nums[0])
	}
	return fmt.Sprintf("Scenes %s have no picture. Choose one for each and try again.", strings.Join(nums, ", "))
}

// NewFromPlan makes a queued job from a plan the user has confirmed. It
// refuses a plan that cannot be rendered: no scenes, a scene without a
// picture, a picture reference that is not well formed, or a story in a
// language that cannot be narrated. These are all things the user can fix, so
// they are found now, before anything is copied or synthesised.
//
// The plan's text and picks are copied, so the job does not change if the plan
// is edited afterwards.
func NewFromPlan(userID, jobID, planID string, revision int, plan storyplan.Plan, voice string, caption models.CaptionStyle, now time.Time) (Job, error) {
	if len(plan.Scenes) == 0 {
		return Job{}, fmt.Errorf("the plan has no scenes")
	}
	v, err := VoiceFor(plan.Language, voice)
	if err != nil {
		return Job{}, err
	}
	if err := caption.Validate(); err != nil {
		return Job{}, err
	}
	var missing []int
	scenes := make([]Scene, len(plan.Scenes))
	for i, s := range plan.Scenes {
		if s.Media == nil {
			missing = append(missing, i+1)
			continue
		}
		if !s.Media.Valid() {
			return Job{}, fmt.Errorf("scene %d: the chosen picture is not valid", i+1)
		}
		scenes[i] = Scene{Narration: s.Narration, Pick: *s.Media}
	}
	if len(missing) > 0 {
		sort.Ints(missing)
		return Job{}, &MissingPicturesError{Scenes: missing}
	}
	ts := now.UTC().Format(time.RFC3339)
	return Job{
		UserID: userID, JobID: jobID, PlanID: planID, PlanRevision: revision,
		Status: StatusQueued, Progress: StageProgress[StatusQueued],
		Language: plan.Language, Voice: v, Caption: caption, Scenes: scenes,
		CreatedAt: ts, UpdatedAt: ts,
	}, nil
}
