// Package projectfail turns the error stored on a failed quiz project into a
// short code the app can explain to the person.
//
// The stored error is the raw Step Functions error (a JSON string that names
// internal stages), so it never goes to the browser or into an email. Only the
// code does, and only for the failures where the person can fix something.
package projectfail

import "strings"

// Code names a failure the person can act on. The empty Code means "any other
// failure": the app then says something generic.
type Code string

const (
	// NoQuestionFormat: the file has no numbered, bulleted or tab-indented
	// multiple-choice questions at all (parser.ErrNoFormatDetected).
	NoQuestionFormat Code = "NO_QUESTION_FORMAT"
	// NoValidQuestions: the format was recognised but no question was complete
	// enough to use (parser.ErrNoQuestions).
	NoValidQuestions Code = "NO_VALID_QUESTIONS"
)

// Classify returns the code for a stored project error, or "" when the error
// is not one the person can fix by choosing a different file.
func Classify(stored string) Code {
	s := strings.ToLower(stored)
	switch {
	case strings.Contains(s, "no detectable question format"):
		return NoQuestionFormat
	case strings.Contains(s, "no valid questions found"):
		return NoValidQuestions
	}
	return ""
}

// FileProblem reports whether the failure is a problem with the uploaded file
// (so choosing another file is the fix).
func (c Code) FileProblem() bool { return c == NoQuestionFormat || c == NoValidQuestions }

// Explanation is one plain sentence about what was wrong with the file, for
// the email. The app screen has its own, longer wording.
func (c Code) Explanation() string {
	switch c {
	case NoQuestionFormat:
		return "We could not find any multiple-choice questions in your file."
	case NoValidQuestions:
		return "We found text in your file, but no question was complete enough to use."
	}
	return ""
}
