// Package shorts defines shared constants and limits for the AI Shorts
// generation pipeline.
package shorts

const (
	MaxSourceDuration = 600.0 // 10 min
	MaxUploadBytes    = 500 * 1024 * 1024
	MaxClipCount      = 3
	MinClipDuration   = 15.0
	MaxClipDuration   = 60.0
)
