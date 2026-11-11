// Package shorts defines shared constants and limits for the AI Shorts
// generation pipeline.
package shorts

import "fmt"

const (
	// MaxSourceDuration is the longest source the pipeline accepts: 1 hour.
	MaxSourceDuration = 3600.0
	// MaxUploadBytes caps the uploaded source at 2 GB. Amazon Transcribe
	// reads the file straight from S3 and refuses anything larger, so this
	// is the real ceiling until audio is extracted before transcription.
	MaxUploadBytes  = 2 * 1024 * 1024 * 1024
	MaxClipCount    = 3
	MinClipDuration = 30.0
	MaxClipDuration = 90.0
	// MinSourceDuration is the shortest source the pipeline accepts. A clip
	// must be at least MinClipDuration long and lie inside the source, so a
	// shorter source can never yield one (issue #286). Exactly 30s is allowed.
	MinSourceDuration = MinClipDuration
)

// CodeNoSegments is the error code of a job whose ranking found no moment
// that fits the clip length limits. It is a VALIDATION code: the same file
// fails the same way, so it is not retried.
const CodeNoSegments = "VALIDATION_NO_SEGMENTS"

// NoSegmentsMessage is what a person sees for a CodeNoSegments failure.
var NoSegmentsMessage = fmt.Sprintf(
	"We could not find a moment of %.0f to %.0f seconds to cut into a clip. Try a longer file with more speech.",
	MinClipDuration, MaxClipDuration)
