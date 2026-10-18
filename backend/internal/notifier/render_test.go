package notifier

import (
	"strings"
	"testing"
)

func TestRenderEscapesProjectNameInHTML(t *testing.T) {
	c, err := Render(KindQuiz, OutcomeCompleted, `<script>alert(1)</script> & "quotes"`, "p1", testBase)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(c.HTML, "<script>") {
		t.Fatalf("unescaped <script> in HTML:\n%s", c.HTML)
	}
	if !strings.Contains(c.HTML, "&lt;script&gt;") {
		t.Fatalf("expected escaped script tag in HTML:\n%s", c.HTML)
	}
}

func TestRenderSubjectNeverContainsLineBreaksOrControlChars(t *testing.T) {
	names := []string{
		"Quiz\r\nBcc: attacker@example.com",
		"Quiz\nSubject: injected",
		"Quiz\rX-Header: 1",
		"Quiz\u2028line sep\u2029para sep",
		"Quiz\u0085next line",
		"tab\there\x00nul\x1bescape",
		"evil\u202eexe.txt\u202c", // bidirectional override
		"zero\u200bwidth\u200djoiner",
	}
	for _, name := range names {
		c, err := Render(KindQuiz, OutcomeCompleted, name, "p1", testBase)
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range c.Subject {
			if r < 0x20 || r == 0x7f || r == 0x85 || r == 0x2028 || r == 0x2029 || (r >= 0x200b && r <= 0x200f) || (r >= 0x202a && r <= 0x202e) {
				t.Fatalf("subject for %q contains forbidden rune %U: %q", name, r, c.Subject)
			}
		}
		if strings.ContainsAny(c.Subject, "\r\n") {
			t.Fatalf("subject has CR/LF: %q", c.Subject)
		}
	}
}

func TestRenderCRLFNameStaysOnOneSubjectLine(t *testing.T) {
	c, err := Render(KindQuiz, OutcomeCompleted, "Quiz\r\nBcc: attacker@example.com", "p1", testBase)
	if err != nil {
		t.Fatal(err)
	}
	if c.Subject != `Your video "Quiz Bcc: attacker@example.com" is ready` {
		t.Fatalf("subject = %q", c.Subject)
	}
}

func TestSanitizeNameTruncates(t *testing.T) {
	long := strings.Repeat("é", 200)
	got := SanitizeName(long)
	if n := len([]rune(got)); n != maxNameRunes {
		t.Fatalf("rune length %d, want %d", n, maxNameRunes)
	}
	if !strings.HasSuffix(got, "…") {
		t.Fatalf("truncated name should end with an ellipsis: %q", got)
	}
	if SanitizeName("  a   b \t c ") != "a b c" {
		t.Fatal("whitespace not collapsed")
	}
}

func TestRenderEveryKindAndOutcome(t *testing.T) {
	tests := []struct {
		kind, outcome, name string
		subject             string
		link                string
	}{
		{KindQuiz, OutcomeCompleted, "Week 3", `Your video "Week 3" is ready`, testBase + "/projects/p1/preview"},
		{KindQuiz, OutcomeCompleted, "", "Your video is ready", testBase + "/projects/p1/preview"},
		{KindQuiz, OutcomeFailed, "Week 3", `Your video "Week 3" could not be finished`, testBase + "/projects/p1/progress"},
		{KindQuiz, OutcomeFailed, "", "Your video could not be finished", testBase + "/projects/p1/progress"},
		{KindShorts, OutcomeCompleted, "", "Your Shorts clips are ready", testBase + "/shorts/p1/clips"},
		{KindShorts, OutcomeFailed, "", "Your Shorts job could not be finished", testBase + "/shorts/p1/progress"},
	}
	for _, tt := range tests {
		t.Run(tt.kind+"/"+tt.outcome+"/"+tt.name, func(t *testing.T) {
			c, err := Render(tt.kind, tt.outcome, tt.name, "p1", testBase+"/")
			if err != nil {
				t.Fatal(err)
			}
			if c.Subject != tt.subject {
				t.Errorf("subject = %q, want %q", c.Subject, tt.subject)
			}
			for label, body := range map[string]string{"text": c.Text, "html": c.HTML} {
				if !strings.Contains(body, tt.link) {
					t.Errorf("%s missing link %s:\n%s", label, tt.link, body)
				}
				if !strings.Contains(body, testBase+"/profile/settings") {
					t.Errorf("%s missing settings link", label)
				}
				if strings.Contains(body, "—") || strings.Contains(body, "–") {
					t.Errorf("%s contains a dash punctuation character", label)
				}
				if strings.Contains(body, "%!") || strings.Contains(body, "%s") {
					t.Errorf("%s has an unformatted placeholder:\n%s", label, body)
				}
			}
			if strings.Contains(c.HTML, "<img") {
				t.Error("emails must not load remote images")
			}
			if strings.Contains(c.Subject+c.Text+c.HTML, "X-Amz") || strings.Contains(c.Subject+c.Text+c.HTML, "Signature=") {
				t.Error("presigned URLs must never appear in email")
			}
		})
	}
}

func TestRenderEscapesIDInLink(t *testing.T) {
	c, err := Render(KindQuiz, OutcomeCompleted, "x", "a/b?c=d", testBase)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(c.Text, "/projects/a%2Fb%3Fc=d/preview") {
		t.Fatalf("id not path-escaped:\n%s", c.Text)
	}
}

func TestRenderRejectsUnknownCombination(t *testing.T) {
	if _, err := Render("other", OutcomeCompleted, "", "p1", testBase); err == nil {
		t.Fatal("want error for unknown kind")
	}
}
