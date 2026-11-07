package notifier

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type recordingMailer struct {
	sent []Message
	err  error
}

func (r *recordingMailer) Send(ctx context.Context, m Message) (string, error) {
	if r.err != nil {
		return "", r.err
	}
	r.sent = append(r.sent, m)
	return "id-1", nil
}

func TestVerificationMessage_HasTheLinkAndNothingElseToClick(t *testing.T) {
	link := "https://indifferent.fun/verify-email?token=" + strings.Repeat("ab", 32)
	msg, err := VerificationMessage("person@example.com", link)
	if err != nil {
		t.Fatal(err)
	}
	if msg.To != "person@example.com" || msg.Subject != "Confirm your email address" {
		t.Errorf("to/subject = %q / %q", msg.To, msg.Subject)
	}
	for name, body := range map[string]string{"text": msg.Text, "html": msg.HTML} {
		if !strings.Contains(body, link) {
			t.Errorf("%s body does not contain the link", name)
		}
	}
	if n := strings.Count(msg.HTML, "href="); n != 2 {
		t.Errorf("html has %d links, want the button and the fallback copy of the same link", n)
	}
	if strings.Contains(msg.HTML, "<img") {
		t.Error("the email must not carry an image or tracking pixel")
	}
	for _, banned := range []string{"\u2014", "!"} {
		if strings.Contains(msg.Text, banned) {
			t.Errorf("text contains %q", banned)
		}
	}
}

func TestVerificationMessage_EscapesWhatIsInsertedIntoHTML(t *testing.T) {
	msg, err := VerificationMessage("a@example.com", `https://indifferent.fun/x?a="><script>alert(1)</script>`)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(msg.HTML, "<script>") {
		t.Errorf("link was not escaped in the html body:\n%s", msg.HTML)
	}
}

func TestVerifyMailer_SendsOnceAndReturnsTheMailerError(t *testing.T) {
	rec := &recordingMailer{}
	if err := (VerifyMailer{Mailer: rec}).SendVerification(context.Background(), "p@example.com", "https://indifferent.fun/verify-email?token=t"); err != nil {
		t.Fatal(err)
	}
	if len(rec.sent) != 1 || rec.sent[0].To != "p@example.com" {
		t.Fatalf("sent = %+v", rec.sent)
	}

	boom := errors.New("ses down")
	err := (VerifyMailer{Mailer: &recordingMailer{err: boom}}).SendVerification(context.Background(), "p@example.com", "https://indifferent.fun/x")
	if !errors.Is(err, boom) {
		t.Errorf("err = %v, want the mailer's error", err)
	}
}
