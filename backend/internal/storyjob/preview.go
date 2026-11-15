package storyjob

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// PreviewEngine is the Polly engine the story voices use. The voice preview and
// the render must agree on it, because it is part of the key of a cached take.
// A test in package api pins it to narrator.EngineFor for every voice.
const PreviewEngine = "standard"

// previewVersion is raised when anything that changes the sound of a take is
// changed (speaking rate, a different audio format), so an older take is never
// served as the new one.
const previewVersion = "v1"

// PreviewPrefix is where cached voice takes live. It is not under
// storyvideo-work/ (7 day expiry of job files) and not under storyvideo/ (the
// finished videos), so each prefix keeps its own lifecycle rule.
const PreviewPrefix = "story-preview/"

// PreviewKey is where the voice take for this text and voice is kept. The user
// is part of the key so one account's takes are never found through another's
// and removing a user's data finds them under one prefix. The key holds no text:
// it is a hash of the version, engine, voice and the exact text.
func PreviewKey(userID, text, voice string) string {
	sum := sha256.Sum256([]byte(previewVersion + "\x00" + PreviewEngine + "\x00" + voice + "\x00" + text))
	return fmt.Sprintf("%s%s/%s.mp3", PreviewPrefix, userID, hex.EncodeToString(sum[:]))
}
