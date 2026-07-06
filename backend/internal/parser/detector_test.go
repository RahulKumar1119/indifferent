package parser

import (
	"testing"
)

func TestDetectFormat_Numbered(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{
			name: "digit dot format",
			content: `1. What is the capital of France?
A) Paris
B) London
C) Berlin
D) Madrid

2. What is 2+2?
A) 3
B) 4
C) 5
D) 6`,
		},
		{
			name: "Q prefix format",
			content: `Q1 What is the capital of France?
A) Paris
B) London
C) Berlin
D) Madrid

Q2 What is 2+2?
A) 3
B) 4
C) 5
D) 6`,
		},
		{
			name: "digit paren format",
			content: `1) What is the capital of France?
a. Paris
b. London
c. Berlin
d. Madrid

2) What is 2+2?
a. 3
b. 4
c. 5
d. 6`,
		},
		{
			name: "mixed numbered with letter options",
			content: `1. What color is the sky?
A. Blue
B. Red
C. Green

2. What color is grass?
A. Blue
B. Red
C. Green`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			format, err := DetectFormat(tt.content)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if format != FormatNumbered {
				t.Errorf("expected %q, got %q", FormatNumbered, format)
			}
		})
	}
}

func TestDetectFormat_Bulleted(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{
			name: "dash bullets",
			content: `What is the capital of France?
- Paris
- London
- Berlin
- Madrid

What is 2+2?
- 3
- 4
- 5
- 6`,
		},
		{
			name: "asterisk bullets",
			content: `What is the capital of France?
* Paris
* London
* Berlin
* Madrid

What is 2+2?
* 3
* 4
* 5
* 6`,
		},
		{
			name: "unicode bullet character",
			content: `What is the capital of France?
• Paris
• London
• Berlin
• Madrid

What is 2+2?
• 3
• 4
• 5
• 6`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			format, err := DetectFormat(tt.content)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if format != FormatBulleted {
				t.Errorf("expected %q, got %q", FormatBulleted, format)
			}
		})
	}
}

func TestDetectFormat_Tabbed(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{
			name: "tab indented options",
			content: `What is the capital of France?
	Paris
	London
	Berlin
	Madrid

What is 2+2?
	3
	4
	5
	6`,
		},
		{
			name: "multiple questions with tabs",
			content: `What color is the sky?
	Blue
	Red
	Green

What color is grass?
	Blue
	Green
	Yellow`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			format, err := DetectFormat(tt.content)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if format != FormatTabbed {
				t.Errorf("expected %q, got %q", FormatTabbed, format)
			}
		})
	}
}

func TestDetectFormat_NoFormat(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{
			name:    "empty string",
			content: "",
		},
		{
			name:    "whitespace only",
			content: "   \n\n   \t\n  ",
		},
		{
			name:    "plain text without structure",
			content: "This is just some random text without any question structure.",
		},
		{
			name: "paragraph text",
			content: `Lorem ipsum dolor sit amet, consectetur adipiscing elit.
Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua.
Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris.`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DetectFormat(tt.content)
			if err != ErrNoFormatDetected {
				t.Errorf("expected ErrNoFormatDetected, got %v", err)
			}
		})
	}
}

func TestDetectFormat_FormatConstants(t *testing.T) {
	if FormatNumbered != "numbered" {
		t.Errorf("FormatNumbered = %q, want %q", FormatNumbered, "numbered")
	}
	if FormatBulleted != "bulleted" {
		t.Errorf("FormatBulleted = %q, want %q", FormatBulleted, "bulleted")
	}
	if FormatTabbed != "tabbed" {
		t.Errorf("FormatTabbed = %q, want %q", FormatTabbed, "tabbed")
	}
}

func TestIsNumberedLine(t *testing.T) {
	tests := []struct {
		line     string
		expected bool
	}{
		{"1. Question", true},
		{"2. Another", true},
		{"10. Multi digit", true},
		{"Q1 Question", true},
		{"Q12 Question", true},
		{"q1 lowercase", true},
		{"1) Paren format", true},
		{"A) Option A", true},
		{"B) Option B", true},
		{"a. Option a", true},
		{"b. Option b", true},
		{"Hello world", false},
		{"", false},
		{"x", false},
		{"Question text here", false},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			result := isNumberedLine(tt.line)
			if result != tt.expected {
				t.Errorf("isNumberedLine(%q) = %v, want %v", tt.line, result, tt.expected)
			}
		})
	}
}

func TestIsBulletedLine(t *testing.T) {
	tests := []struct {
		line     string
		expected bool
	}{
		{"- Option", true},
		{"* Option", true},
		{"• Option", true},
		{"-No space", false},
		{"*No space", false},
		{"Normal text", false},
		{"", false},
		{"-", false},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			result := isBulletedLine(tt.line)
			if result != tt.expected {
				t.Errorf("isBulletedLine(%q) = %v, want %v", tt.line, result, tt.expected)
			}
		})
	}
}
