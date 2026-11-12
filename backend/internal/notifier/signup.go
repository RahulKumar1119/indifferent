package notifier

import (
	"context"
	"fmt"
	"html"
	"strings"
	"unicode"

	"github.com/rahul/indifferent/backend/internal/auth"
)

// SignupMailer sends the site owner one email per new account (#297). It
// implements auth.SignupNotifier.
type SignupMailer struct {
	Mailer Mailer
	To     string // the owner's address, for example admin@indifferent.fun
}

// NotifySignup implements auth.SignupNotifier.
func (m SignupMailer) NotifySignup(ctx context.Context, s auth.Signup) error {
	if m.Mailer == nil || m.To == "" {
		return nil
	}
	_, err := m.Mailer.Send(ctx, SignupMessage(m.To, s))
	return err
}

// maxSignupField bounds a name or address in the alert. Both come from the
// person signing up (or their Google profile), so they are not trusted.
const maxSignupField = 200

// cleanField makes untrusted text safe for a subject line or a plain-text
// body: control characters (newlines included) become spaces, and the result
// is trimmed and bounded.
func cleanField(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, s)
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > maxSignupField {
		s = string(r[:maxSignupField]) + "..."
	}
	return s
}

// SignupMessage builds the owner's alert. It carries the name, the address,
// how the person signed up, when, and the running total; nothing else about
// the account.
func SignupMessage(to string, s auth.Signup) Message {
	name := cleanField(s.Name)
	if name == "" {
		name = "(no name)"
	}
	email := cleanField(s.Email)
	method := "Google"
	if s.Method == "email" {
		method = "Email and password"
	}
	when := s.At.UTC().Format("2 Jan 2006, 15:04 UTC")

	subject := "New sign-up: " + name
	if s.Number > 0 {
		subject = fmt.Sprintf("New sign-up #%d: %s", s.Number, name)
	}

	rows := [][2]string{{"Name", name}, {"Email", email}, {"Signed up with", method}, {"When", when}}
	if s.Number > 0 {
		rows = append(rows, [2]string{"Account number", fmt.Sprint(s.Number)})
	}

	var text, table strings.Builder
	text.WriteString("Someone just created an Indifferent account.\n\n")
	for _, r := range rows {
		fmt.Fprintf(&text, "%s: %s\n", r[0], r[1])
		fmt.Fprintf(&table, `<tr><th align="left" style="padding:4px 12px 4px 0">%s</th><td style="padding:4px 0">%s</td></tr>`,
			html.EscapeString(r[0]), html.EscapeString(r[1]))
	}
	text.WriteString("\nYou get this because admin alerts are on for indifferent.fun (ADMIN_ALERT_EMAIL).\n")
	body := `<!doctype html><html><body style="font-family:Arial,sans-serif;color:#1a1a1a">` +
		`<p>Someone just created an Indifferent account.</p><table>` + table.String() + `</table>` +
		`<p style="color:#555;font-size:13px">You get this because admin alerts are on for indifferent.fun (ADMIN_ALERT_EMAIL).</p>` +
		`</body></html>`
	return Message{To: to, Subject: subject, Text: text.String(), HTML: body}
}
