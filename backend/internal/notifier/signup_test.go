package notifier

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/rahul/indifferent/backend/internal/auth"
)

// Issue #297: the owner's sign-up alert.

var at = time.Date(2026, 11, 12, 16, 30, 0, 0, time.UTC)

func TestSignupMessage_SaysWhoHowWhenAndWhichNumber(t *testing.T) {
	m := SignupMessage("admin@indifferent.fun", auth.Signup{Name: "Asha Rao", Email: "asha@gmail.com", Method: "google", At: at, Number: 170})
	if m.To != "admin@indifferent.fun" || m.Subject != "New sign-up #170: Asha Rao" {
		t.Errorf("to %q subject %q", m.To, m.Subject)
	}
	for _, want := range []string{"Name: Asha Rao", "Email: asha@gmail.com", "Signed up with: Google", "When: 12 Nov 2026, 16:30 UTC", "Account number: 170"} {
		if !strings.Contains(m.Text, want) {
			t.Errorf("text missing %q:\n%s", want, m.Text)
		}
	}
	if !strings.Contains(m.HTML, "Asha Rao") || !strings.Contains(m.HTML, "<table>") {
		t.Errorf("html: %s", m.HTML)
	}
	e := SignupMessage("a@b.co", auth.Signup{Name: "P", Email: "p@gmail.com", Method: "email", At: at})
	if e.Subject != "New sign-up: P" || strings.Contains(e.Text, "Account number") || !strings.Contains(e.Text, "Signed up with: Email and password") {
		t.Errorf("without a number: subject %q\n%s", e.Subject, e.Text)
	}
}

// The name and address come from the person signing up. They must not be
// able to add lines to the subject or markup to the body.
func TestSignupMessage_UntrustedNamesCannotInjectHeadersOrMarkup(t *testing.T) {
	m := SignupMessage("admin@indifferent.fun", auth.Signup{
		Name:  "Eve\r\nBcc: victim@example.org <script>alert(1)</script>",
		Email: "eve@gmail.com\nX-Evil: 1",
		At:    at, Number: 3,
	})
	if strings.ContainsAny(m.Subject, "\r\n") {
		t.Errorf("subject has a line break: %q", m.Subject)
	}
	if strings.Contains(m.HTML, "<script>") {
		t.Errorf("markup reached the html body: %s", m.HTML)
	}
	for _, line := range strings.Split(m.Text, "\n") {
		if strings.HasPrefix(line, "Bcc:") || strings.HasPrefix(line, "X-Evil:") {
			t.Errorf("an injected line in the text body: %q", line)
		}
	}
	long := SignupMessage("a@b.co", auth.Signup{Name: strings.Repeat("é", 500), At: at})
	if len([]rune(long.Subject)) > len("New sign-up: ")+maxSignupField+3 {
		t.Errorf("subject not bounded: %d runes", len([]rune(long.Subject)))
	}
	if !strings.Contains(SignupMessage("a@b.co", auth.Signup{Name: "  ", At: at}).Subject, "(no name)") {
		t.Error("an empty name needs a placeholder")
	}
}

type captureMailer struct {
	msgs []Message
	err  error
}

func (c *captureMailer) Send(_ context.Context, m Message) (string, error) {
	c.msgs = append(c.msgs, m)
	return "id-1", c.err
}

func TestSignupMailer_SendsOneMessageToTheOwner(t *testing.T) {
	c := &captureMailer{}
	err := SignupMailer{Mailer: c, To: "admin@indifferent.fun"}.NotifySignup(context.Background(), auth.Signup{Name: "A", Email: "a@gmail.com", At: at})
	if err != nil || len(c.msgs) != 1 || c.msgs[0].To != "admin@indifferent.fun" {
		t.Fatalf("err %v msgs %+v", err, c.msgs)
	}
	c.err = errors.New("ses down")
	if err := (SignupMailer{Mailer: c, To: "x@y.z"}).NotifySignup(context.Background(), auth.Signup{At: at}); err == nil {
		t.Error("a send failure must be returned, so it is logged")
	}
	if err := (SignupMailer{}).NotifySignup(context.Background(), auth.Signup{}); err != nil {
		t.Errorf("an unconfigured mailer sends nothing: %v", err)
	}
}
