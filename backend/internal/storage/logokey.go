package storage

import "strings"

// ProjectLogoKey is the one key a project's logo is ever stored under. The API
// builds it when it hands out the upload URL, from the signed-in user's id and
// the project's server-made id, and nothing else may put a logo anywhere.
func ProjectLogoKey(userID, projectID string) string {
	return "uploads/" + userID + "/" + projectID + "/logo.png"
}

// IsProjectLogoKey reports whether key is exactly the logo key of this user's
// project. Code that knows both ids (the API, the shorts renderer) uses it
// before it signs, reads or burns in a logo, so a key stored on a project record
// that points at anything else (another user's file, a rendered video) is
// ignored instead of followed.
func IsProjectLogoKey(userID, projectID, key string) bool {
	return userID != "" && projectID != "" && key == ProjectLogoKey(userID, projectID)
}

// LogoKeyBelongsToProject is the weaker check for code that does not receive the
// user id (the quiz renderer gets only the project id): the key must have the
// form uploads/<one segment>/<this project id>/logo.png. Project ids are
// server-made UUIDs, so a key naming a different project can never pass.
func LogoKeyBelongsToProject(projectID, key string) bool {
	if projectID == "" {
		return false
	}
	parts := strings.Split(key, "/")
	if len(parts) != 4 {
		return false
	}
	user := parts[1]
	return parts[0] == "uploads" && user != "" && user != "." && user != ".." &&
		parts[2] == projectID && parts[3] == "logo.png"
}
