// Package parser implements TXT file parsing, validation, and extraction
// for the TXT-to-Video SaaS pipeline.
package parser

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/rahul/indifferent/backend/internal/models"
)

// MaxFileSize is the maximum allowed TXT file size in bytes (5MB).
const MaxFileSize = 5 * 1024 * 1024

// Validation errors.
var (
	ErrFileTooLarge   = errors.New("file size exceeds 5MB limit")
	ErrNoQuestions    = errors.New("no valid questions found in file")
)

// htmlTagPattern matches HTML/XML tags including script, style, and generic tags.
var htmlTagPattern = regexp.MustCompile(`<[^>]*>`)

// scriptPattern matches common script injection patterns.
var scriptPattern = regexp.MustCompile(`(?i)(javascript|vbscript|on\w+)\s*[:=]`)

// ValidateFileSize checks that the content does not exceed the maximum allowed size.
func ValidateFileSize(content []byte) error {
	if len(content) > MaxFileSize {
		return ErrFileTooLarge
	}
	return nil
}

// SanitizeContent strips potentially dangerous content from TXT input.
// It removes HTML/XML tags, script injection patterns, and control characters
// (preserving newlines and tabs which are meaningful for format detection).
func SanitizeContent(content string) string {
	// Remove HTML/XML tags
	result := htmlTagPattern.ReplaceAllString(content, "")

	// Remove script injection patterns (javascript:, vbscript:, onXxx=)
	result = scriptPattern.ReplaceAllString(result, "")

	// Remove control characters except newline (\n), carriage return (\r), and tab (\t)
	var sanitized strings.Builder
	sanitized.Grow(len(result))
	for _, r := range result {
		if r == '\n' || r == '\r' || r == '\t' || r >= 32 {
			sanitized.WriteRune(r)
		}
	}

	return sanitized.String()
}

// ValidateQuestions returns an error if no questions were successfully parsed.
// This is called after extraction to ensure the file contained at least one valid question.
func ValidateQuestions(questions []models.Question, warnings []models.Warning) error {
	if len(questions) == 0 {
		if len(warnings) > 0 {
			return fmt.Errorf("%w: all entries were malformed", ErrNoQuestions)
		}
		return ErrNoQuestions
	}
	return nil
}

// ValidateQuestion checks a single parsed question for completeness.
// It returns a warning reason string if the question is malformed, or empty string if valid.
func ValidateQuestion(q models.Question) string {
	if strings.TrimSpace(q.Text) == "" {
		return "question has no text"
	}
	if len(q.Options) == 0 {
		return "question has no options"
	}
	for i, opt := range q.Options {
		if strings.TrimSpace(opt.Text) == "" {
			return fmt.Sprintf("option %d has no text", i+1)
		}
	}
	// CorrectIndex of -1 means no answer was marked (acceptable)
	// Only reject if it's a positive value that's out of range
	if q.CorrectIndex >= len(q.Options) {
		return "correct answer index out of range"
	}
	return ""
}
