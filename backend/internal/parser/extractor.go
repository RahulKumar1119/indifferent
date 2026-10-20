// Package parser implements TXT file parsing for multiple-choice questions.
package parser

import (
	"regexp"
	"strings"

	"github.com/rahul/indifferent/backend/internal/models"
)

// Regex patterns for question and option detection.
var (
	// Numbered question patterns: "1.", "2.", "Q1:", "Q1.", "Q 1:", "Q 1."
	numberedQuestionRe = regexp.MustCompile(`^(?:Q\s*)?(\d+)[.:]\s*(.+)`)

	// Option patterns for numbered/bulleted formats:
	// "A)", "B)", "a)", "b)", "A.", "B.", "a.", "b.", "A:", "B:"
	optionRe = regexp.MustCompile(`^([A-Fa-f])[.):\s]\s*(.+)`)

	// Option with a leading correct-answer asterisk: "*B. Paris", "*B) Paris".
	// Checked before optionRe because a leading "*" is otherwise read as a
	// bullet prefix and the option is lost.
	markedOptionRe = regexp.MustCompile(`^\*\s*([A-Fa-f])[.):\s]\s*(.+)`)

	// Bulleted option patterns: "•", "-", "*" as prefixes for options.
	// Note: "*" at line start is a bullet unless it's a correct-answer marker mid-option.
	bulletOptionRe = regexp.MustCompile(`^[•\-\*]\s+(.+)`)

	// Correct answer markers
	correctPrefixRe = regexp.MustCompile(`^\*\s*`)
	correctSuffixRe = regexp.MustCompile(`\s*\*$`)
	correctAnnotRe  = regexp.MustCompile(`(?i)\s*\(correct\)\s*$`)

	// Explicit answer line: "Answer: B", "Answer: Jupiter", "Answer: B, D".
	// The payload is free text: the numbered format uses the option letter,
	// the bulleted and tabbed formats use the option text.
	answerLineRe = regexp.MustCompile(`(?i)^answer\s*:\s*(.+)$`)
)

// ExtractQuestions parses the content string according to the given format type
// and returns a slice of Question structs with 0-based indexing.
// Supported formats: "numbered", "bulleted", "tabbed".
func ExtractQuestions(content string, format string) []models.Question {
	switch format {
	case "numbered":
		return extractNumbered(content)
	case "bulleted":
		return extractBulleted(content)
	case "tabbed":
		return extractTabbed(content)
	default:
		return nil
	}
}

// extractNumbered handles the numbered format where questions start with
// patterns like "1.", "2.", "Q1:", "Q1." and options use "A)", "B)", etc.
func extractNumbered(content string) []models.Question {
	lines := splitLines(content)
	var questions []models.Question

	var currentQuestion *questionBuilder

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			// Blank lines can separate questions; finalize if we have pending options
			continue
		}

		// Check if this line starts a new question
		if m := numberedQuestionRe.FindStringSubmatch(trimmed); m != nil {
			// Finalize previous question
			if currentQuestion != nil && currentQuestion.hasOptions() {
				questions = append(questions, currentQuestion.build(len(questions)))
			}
			currentQuestion = &questionBuilder{
				textLines: []string{m[2]},
			}
			continue
		}

		// Option with a leading correct-answer asterisk: "*B. Paris"
		if m := markedOptionRe.FindStringSubmatch(trimmed); m != nil {
			if currentQuestion != nil {
				label := strings.ToUpper(m[1])
				text, _ := checkCorrectMarkers(m[2])
				currentQuestion.addOption(label, text, true)
			}
			continue
		}

		// Check if this line is an option
		if m := optionRe.FindStringSubmatch(trimmed); m != nil {
			if currentQuestion != nil {
				label := strings.ToUpper(m[1])
				text := m[2]
				correct := false

				// Check for correct answer markers
				text, correct = checkCorrectMarkers(text)

				currentQuestion.addOption(label, text, correct)
			}
			continue
		}

		// Explicit answer line: "Answer: B" (numbered) or "Answer: Jupiter"
		// (bulleted and tabbed, where the answer names the option text).
		if m := answerLineRe.FindStringSubmatch(trimmed); m != nil {
			if currentQuestion != nil {
				currentQuestion.addAnswerPayload(m[1])
			}
			continue
		}

		// Otherwise, it's a continuation line for the current question text
		if currentQuestion != nil && !currentQuestion.hasOptions() {
			currentQuestion.textLines = append(currentQuestion.textLines, trimmed)
		}
	}

	// Finalize last question
	if currentQuestion != nil && currentQuestion.hasOptions() {
		questions = append(questions, currentQuestion.build(len(questions)))
	}

	return questions
}

// extractBulleted handles the bulleted format where questions are plain text
// lines followed by options prefixed with "•", "-", or "*".
func extractBulleted(content string) []models.Question {
	lines := splitLines(content)
	var questions []models.Question

	var currentQuestion *questionBuilder
	inOptions := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			// Blank line: if we were collecting options, finalize the question
			if currentQuestion != nil && currentQuestion.hasOptions() {
				questions = append(questions, currentQuestion.build(len(questions)))
				currentQuestion = nil
				inOptions = false
			}
			continue
		}

		// Check if this line is a bulleted option
		if m := bulletOptionRe.FindStringSubmatch(trimmed); m != nil {
			if currentQuestion == nil {
				// Option without a question - skip
				continue
			}
			inOptions = true
			text := m[1]
			correct := false
			text, correct = checkCorrectMarkers(text)

			// Auto-assign label based on option count
			label := optionLabel(currentQuestion.optionCount())
			currentQuestion.addOption(label, text, correct)
			continue
		}

		// Explicit answer line: "Answer: Jupiter" (the bulleted format names
		// the answer by its text). Checked before the bullet and the
		// finalize branch, since an answer line is neither.
		if m := answerLineRe.FindStringSubmatch(trimmed); m != nil {
			if currentQuestion != nil {
				currentQuestion.addAnswerPayload(m[1])
			}
			continue
		}

		// If we're in options mode but hit a non-option, non-blank line,
		// finalize current question and start a new one
		if inOptions && currentQuestion != nil {
			questions = append(questions, currentQuestion.build(len(questions)))
			currentQuestion = &questionBuilder{
				textLines: []string{trimmed},
			}
			inOptions = false
			continue
		}

		// Plain text line: either start a new question or continue the current one
		if currentQuestion == nil {
			currentQuestion = &questionBuilder{
				textLines: []string{trimmed},
			}
		} else if !inOptions {
			// Multi-line question text
			currentQuestion.textLines = append(currentQuestion.textLines, trimmed)
		}
	}

	// Finalize last question
	if currentQuestion != nil && currentQuestion.hasOptions() {
		questions = append(questions, currentQuestion.build(len(questions)))
	}

	return questions
}

// extractTabbed handles the tabbed format where questions are non-indented lines
// and options are tab-indented below them.
func extractTabbed(content string) []models.Question {
	lines := splitLines(content)
	var questions []models.Question

	var currentQuestion *questionBuilder

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		// Explicit answer line: "Answer: Gold" (the tabbed format names the
		// answer by its text). Checked before the indent test so an indented
		// answer line is still read as an answer.
		if m := answerLineRe.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
			if currentQuestion != nil {
				currentQuestion.addAnswerPayload(m[1])
			}
			continue
		}

		// Check if line is tab-indented (option)
		if strings.HasPrefix(line, "\t") || strings.HasPrefix(line, "    ") {
			if currentQuestion == nil {
				continue
			}

			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue
			}

			// Option with a leading correct-answer asterisk: "*B. Paris"
			if m := markedOptionRe.FindStringSubmatch(trimmed); m != nil {
				label := strings.ToUpper(m[1])
				text, _ := checkCorrectMarkers(m[2])
				currentQuestion.addOption(label, text, true)
				continue
			}

			// Try to match labeled option (A), B., etc.)
			if m := optionRe.FindStringSubmatch(trimmed); m != nil {
				label := strings.ToUpper(m[1])
				text := m[2]
				correct := false
				text, correct = checkCorrectMarkers(text)
				currentQuestion.addOption(label, text, correct)
			} else {
				// Plain indented text - treat as option without label
				text := trimmed
				correct := false
				text, correct = checkCorrectMarkers(text)
				label := optionLabel(currentQuestion.optionCount())
				currentQuestion.addOption(label, text, correct)
			}
			continue
		}

		// Non-indented line: either new question or continuation
		trimmed := strings.TrimSpace(line)

		if currentQuestion != nil && !currentQuestion.hasOptions() {
			// Multi-line question text (continuation)
			currentQuestion.textLines = append(currentQuestion.textLines, trimmed)
		} else {
			// Finalize previous question and start new one
			if currentQuestion != nil && currentQuestion.hasOptions() {
				questions = append(questions, currentQuestion.build(len(questions)))
			}
			currentQuestion = &questionBuilder{
				textLines: []string{trimmed},
			}
		}
	}

	// Finalize last question
	if currentQuestion != nil && currentQuestion.hasOptions() {
		questions = append(questions, currentQuestion.build(len(questions)))
	}

	return questions
}

// questionBuilder accumulates data for a single question during parsing.
type questionBuilder struct {
	textLines []string
	options   []optionEntry
	// answerPayloads holds raw "Answer:" payloads. They are resolved against
	// the options when the question is built, because the answer line may
	// appear before the options it refers to.
	answerPayloads []string
}

type optionEntry struct {
	label   string
	text    string
	correct bool
}

func (qb *questionBuilder) addOption(label, text string, correct bool) {
	qb.options = append(qb.options, optionEntry{label: label, text: text, correct: correct})
}

func (qb *questionBuilder) hasOptions() bool {
	return len(qb.options) > 0
}

func (qb *questionBuilder) optionCount() int {
	return len(qb.options)
}

// addAnswerPayload records an explicit answer line payload. It is resolved
// against the options later, in build.
func (qb *questionBuilder) addAnswerPayload(payload string) {
	payload = strings.TrimSpace(payload)
	if payload == "" {
		return
	}
	qb.answerPayloads = append(qb.answerPayloads, payload)
}

// applyAnswerPayload marks the options an answer payload refers to. The
// payload can name an option by its label ("B"), which the numbered format
// uses, or by its text ("Jupiter"), which the bulleted and tabbed formats
// use.
func (qb *questionBuilder) applyAnswerPayload(payload string) {
	payload = strings.TrimSpace(payload)
	if payload == "" {
		return
	}
	// The whole payload may be a single option whose text contains spaces
	// ("Salt and Pepper"), so try it whole before splitting on separators.
	if qb.markCorrectMatch(payload) {
		return
	}
	for _, part := range splitAnswerPayload(payload) {
		qb.markCorrectMatch(part)
	}
}

// markCorrectMatch marks the option whose label or text matches, ignoring
// case. It reports whether an option was matched.
func (qb *questionBuilder) markCorrectMatch(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	// Label match: a single letter A-F.
	if len(s) == 1 {
		upper := strings.ToUpper(s)
		if upper[0] >= 'A' && upper[0] <= 'F' {
			for i := range qb.options {
				if qb.options[i].label == upper {
					qb.options[i].correct = true
					return true
				}
			}
		}
	}
	// Text match: the full text of the option, case-insensitively.
	for i := range qb.options {
		if strings.EqualFold(strings.TrimSpace(qb.options[i].text), s) {
			qb.options[i].correct = true
			return true
		}
	}
	return false
}

// splitAnswerPayload splits a multi-answer payload into its parts:
// "B, D" -> ["B", "D"], "B and D" -> ["B", "D"].
func splitAnswerPayload(payload string) []string {
	replaced := strings.NewReplacer(",", " ", "&", " ", "/", " ").Replace(payload)
	var parts []string
	for _, p := range strings.Fields(replaced) {
		// "and" is a separator word, not an answer.
		if strings.EqualFold(p, "and") {
			continue
		}
		parts = append(parts, p)
	}
	return parts
}

func (qb *questionBuilder) build(index int) models.Question {
	q := models.Question{
		Index:          index,
		Text:           strings.Join(qb.textLines, " "),
		Options:        make([]models.Option, len(qb.options)),
		CorrectIndex:   -1, // default: no correct answer marked
		CorrectIndices: []int{},
	}

	// Explicit "Answer:" lines are resolved here so they work whether they
	// appear before or after the options they refer to.
	for _, payload := range qb.answerPayloads {
		qb.applyAnswerPayload(payload)
	}

	for i, opt := range qb.options {
		q.Options[i] = models.Option{
			Label: opt.label,
			Text:  opt.text,
		}
		if opt.correct {
			q.CorrectIndices = append(q.CorrectIndices, i)
			if q.CorrectIndex == -1 {
				q.CorrectIndex = i // First correct answer for backward compat
			}
		}
	}

	return q
}

// checkCorrectMarkers checks for correct-answer markers in option text
// and returns the cleaned text and whether this option is correct.
func checkCorrectMarkers(text string) (string, bool) {
	correct := false

	// Check "(correct)" annotation
	if correctAnnotRe.MatchString(text) {
		text = correctAnnotRe.ReplaceAllString(text, "")
		correct = true
	}

	// Check "*" prefix
	if correctPrefixRe.MatchString(text) {
		text = correctPrefixRe.ReplaceAllString(text, "")
		correct = true
	}

	// Check "*" suffix
	if correctSuffixRe.MatchString(text) {
		text = correctSuffixRe.ReplaceAllString(text, "")
		correct = true
	}

	return strings.TrimSpace(text), correct
}

// optionLabel returns the label for the nth option (0-indexed): "A", "B", "C", etc.
func optionLabel(n int) string {
	if n < 0 || n > 25 {
		return ""
	}
	return string(rune('A' + n))
}

// splitLines splits content into individual lines, handling different line endings.
func splitLines(content string) []string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")
	return strings.Split(content, "\n")
}
