// Package shorts defines shared constants and limits for the AI Shorts
// generation pipeline.
package shorts

const (
	// MaxSourceDuration is the longest source the pipeline accepts: 1 hour.
	MaxSourceDuration = 3600.0
	// MaxUploadBytes caps the uploaded source at 2 GB. Amazon Transcribe
	// reads the file straight from S3 and refuses anything larger, so this
	// is the real ceiling until audio is extracted before transcription.
	MaxUploadBytes = 2 * 1024 * 1024 * 1024
	MaxClipCount      = 3
	MinClipDuration   = 30.0
	MaxClipDuration   = 90.0
)
