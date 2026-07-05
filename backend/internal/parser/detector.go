// Package parser provides TXT file parsing functionality for multiple-choice
// question extraction. It supports auto-detection of numbered, bulleted, and
// tabbed question formats.
package parser

import (
	"errors"
	"strings"
	"unicode/utf8"
)

// Format represents the detected question format type.
type Format string

const (
	// FormatNumbered indicates questions use numbered prefixes (e.g., "1.", "Q1").
	FormatNumbered Format = "numbered"
	// FormatBulleted indicates options use bullet prefixes (e.g., "•", "-", "*").
	FormatBulleted Format = "bulleted"
	// FormatTabbed indicates options use tab indentation.
	FormatTabbed Format = "tabbed"
)

// ErrNoFormatDetected is returned when the content does not match any known format.
var ErrNoFormatDetected = errors.New("no detectable question format found")

// DetectFormat analyzes the content of a TXT file and determines the question
// format used. It returns the detected format type or an error if no format
// can be identified.
func DetectFormat(content string) (Format, error) {
	if strings.TrimSpace(content) == "" {
		return "", ErrNoFormatDetected
	}

	lines := strings.Split(content, "\n")

	numberedScore := scoreNumbered(lines)
	bulletedScore := scoreBulleted(lines)
	tabbedScore := scoreTabbed(lines)

	// No format detected at all
	if numberedScore == 0 && bulletedScore == 0 && tabbedScore == 0 {
		return "", ErrNoFormatDetected
	}

	// Return the format with the highest confidence score
	if numberedScore >= bulletedScore && numberedScore >= tabbedScore {
		return FormatNumbered, nil
	}
	if bulletedScore >= tabbedScore {
		return FormatBulleted, nil
	}
	return FormatTabbed, nil
}

// scoreNumbered returns a confidence score for numbered format detection.
// It looks for lines starting with patterns like "1.", "2.", "Q1", "Q2", etc.
func scoreNumbered(lines []string) int {
	score := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if isNumberedLine(trimmed) {
			score++
		}
	}
	return score
}

// isNumberedLine checks if a line starts with a numbered pattern.
// Supported patterns:
//   - "1.", "2.", "10.", etc. (digit followed by dot)
//   - "Q1", "Q2", "Q10", etc. (Q followed by digits)
//   - "1)", "2)", etc. (digit followed by closing paren)
//   - "A)", "B)", "a.", "b.", etc. (letter option prefixes)
func isNumberedLine(line string) bool {
	if len(line) < 2 {
		return false
	}

	// Check for "Q" followed by digits (e.g., "Q1", "Q12")
	if (line[0] == 'Q' || line[0] == 'q') && len(line) > 1 && isDigit(rune(line[1])) {
		return true
	}

	firstRune, size := utf8.DecodeRuneInString(line)

	// Check for digit-based patterns: "1.", "1)", "12."
	if isDigit(firstRune) {
		rest := line[size:]
		// Skip additional digits
		for len(rest) > 0 {
			r, s := utf8.DecodeRuneInString(rest)
			if isDigit(r) {
				rest = rest[s:]
			} else {
				break
			}
		}
		if len(rest) > 0 && (rest[0] == '.' || rest[0] == ')') {
			return true
		}
	}

	// Check for letter-based option patterns: "A)", "B)", "a.", "b.", "A.", "B."
	if isLetter(firstRune) && len(line) > 1 {
		second := line[size]
		if second == ')' || second == '.' {
			// Ensure it's a single letter (not a word start)
			if size == 1 && (len(line) == 2 || line[size+1] == ' ') {
				return true
			}
		}
	}

	return false
}

// scoreBulleted returns a confidence score for bulleted format detection.
// It looks for lines starting with "•", "-", or "*" followed by a space.
func scoreBulleted(lines []string) int {
	score := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if isBulletedLine(trimmed) {
			score++
		}
	}
	return score
}

// isBulletedLine checks if a line starts with a bullet character followed by a space.
func isBulletedLine(line string) bool {
	if len(line) < 2 {
		return false
	}

	firstRune, size := utf8.DecodeRuneInString(line)

	// Check for bullet unicode character "•"
	if firstRune == '•' {
		// Accept with or without trailing space
		if size == len(line) || line[size] == ' ' {
			return true
		}
	}

	// Check for "-" or "*" followed by a space
	if (firstRune == '-' || firstRune == '*') && len(line) > 1 && line[size] == ' ' {
		return true
	}

	return false
}

// scoreTabbed returns a confidence score for tabbed format detection.
// It looks for lines that begin with one or more tab characters.
func scoreTabbed(lines []string) int {
	score := 0
	hasNonTabbed := false
	for _, line := range lines {
		if line == "" {
			continue
		}
		if line[0] == '\t' {
			score++
		} else if strings.TrimSpace(line) != "" {
			hasNonTabbed = true
		}
	}

	// Tabbed format requires both tabbed and non-tabbed lines
	// (questions are non-tabbed, options are tabbed)
	if !hasNonTabbed {
		return 0
	}

	return score
}

// isDigit returns true if the rune is an ASCII digit.
func isDigit(r rune) bool {
	return r >= '0' && r <= '9'
}

// isLetter returns true if the rune is an ASCII letter.
func isLetter(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}
