package notifier

import (
	"bytes"
	"fmt"
	htmltemplate "html/template"
	"net/url"
	"strings"
	texttemplate "text/template"
	"unicode"
)

// Job kinds and outcomes accepted by the notifier.
const (
	KindQuiz   = "quiz"
	KindShorts = "shorts"

	OutcomeCompleted = "completed"
	OutcomeFailed    = "failed"
)

// maxNameRunes bounds how much of a user-chosen project name reaches an email.
const maxNameRunes = 80

// Content is a rendered email.
type Content struct {
	Subject string
	Text    string
	HTML    string
}

// copyFor holds the wording and link for one (kind, outcome) pair. The failed
// variants are deliberately generic: raw pipeline errors never reach email.
type copyFor struct {
	subject string // %s is replaced by the quoted name when there is one
	heading string
	intro   string
	button  string
	path    string // %s is the escaped id
}

func lookupCopy(kind, outcome string, hasName bool) (copyFor, bool) {
	switch {
	case kind == KindQuiz && outcome == OutcomeCompleted:
		c := copyFor{
			subject: "Your video is ready",
			heading: "Your video is ready",
			intro:   "Your video has finished processing. You can watch and download it now.",
			button:  "Watch your video",
			path:    "/projects/%s/preview",
		}
		if hasName {
			c.subject = "Your video %s is ready"
			c.intro = "%s has finished processing. You can watch and download it now."
		}
		return c, true
	case kind == KindQuiz && outcome == OutcomeFailed:
		c := copyFor{
			subject: "Your video could not be finished",
			heading: "Your video did not finish",
			intro:   "Something went wrong while making your video, so it was not completed. Open the project to try again.",
			button:  "Open the project",
			path:    "/projects/%s",
		}
		if hasName {
			c.subject = "Your video %s could not be finished"
			c.intro = "Something went wrong while making %s, so it was not completed. Open the project to try again."
		}
		return c, true
	case kind == KindShorts && outcome == OutcomeCompleted:
		return copyFor{
			subject: "Your Shorts clips are ready",
			heading: "Your clips are ready",
			intro:   "Your clips have finished processing. You can preview and download them now.",
			button:  "View your clips",
			path:    "/shorts/%s/clips",
		}, true
	case kind == KindShorts && outcome == OutcomeFailed:
		return copyFor{
			subject: "Your Shorts job could not be finished",
			heading: "Your Shorts job did not finish",
			intro:   "Something went wrong while processing your video, so the job was not completed. Open it to see what happened and to try again if that is possible.",
			button:  "Open the job",
			path:    "/shorts/%s/progress",
		}, true
	}
	return copyFor{}, false
}

// Render builds the email for one job. name may be empty (Shorts jobs have no
// title). baseURL is the site origin such as https://indifferent.fun.
func Render(kind, outcome, name, id, baseURL string) (Content, error) {
	cleaned := SanitizeName(name)
	c, ok := lookupCopy(kind, outcome, cleaned != "")
	if !ok {
		return Content{}, fmt.Errorf("no email for kind %q outcome %q", kind, outcome)
	}

	quoted := ""
	if cleaned != "" {
		quoted = `"` + cleaned + `"`
	}
	subject := c.subject
	intro := c.intro
	if cleaned != "" && strings.Contains(subject, "%s") {
		subject = fmt.Sprintf(subject, quoted)
	}
	if cleaned != "" && strings.Contains(intro, "%s") {
		intro = fmt.Sprintf(intro, quoted)
	}

	base := strings.TrimRight(baseURL, "/")
	link := base + fmt.Sprintf(c.path, url.PathEscape(id))
	settings := base + "/profile/settings"

	data := renderData{
		Heading:     c.heading,
		Intro:       intro,
		Button:      c.button,
		URL:         link,
		SettingsURL: settings,
	}

	var text bytes.Buffer
	if err := textBody.Execute(&text, data); err != nil {
		return Content{}, fmt.Errorf("render text: %w", err)
	}
	var html bytes.Buffer
	if err := htmlBody.Execute(&html, data); err != nil {
		return Content{}, fmt.Errorf("render html: %w", err)
	}

	return Content{Subject: subject, Text: text.String(), HTML: html.String()}, nil
}

// SanitizeName makes a user-chosen name safe to place in an email subject and
// body: control characters (including CR and LF), invisible formatting and
// bidirectional override characters are dropped, whitespace is collapsed, and
// the result is cut to maxNameRunes.
func SanitizeName(name string) string {
	var b strings.Builder
	for _, r := range name {
		switch {
		case unicode.IsSpace(r):
			b.WriteRune(' ')
		case unicode.IsControl(r), unicode.Is(unicode.Cf, r), r == '\u2028', r == '\u2029':
			// dropped
		default:
			b.WriteRune(r)
		}
	}
	out := strings.Join(strings.Fields(b.String()), " ")
	runes := []rune(out)
	if len(runes) > maxNameRunes {
		out = strings.TrimSpace(string(runes[:maxNameRunes-1])) + "…"
	}
	return out
}

type renderData struct {
	Heading     string
	Intro       string
	Button      string
	URL         string
	SettingsURL string
}

var textBody = texttemplate.Must(texttemplate.New("text").Parse(
	`{{.Heading}}

{{.Intro}}

{{.Button}}:
{{.URL}}

You get this email because email notifications are on for your account.
You can change that in Settings:
{{.SettingsURL}}
`))

var htmlBody = htmltemplate.Must(htmltemplate.New("html").Parse(
	`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Heading}}</title>
</head>
<body style="margin:0;padding:0;background:#FAF7F2;">
<div style="max-width:600px;margin:0 auto;padding:32px 24px;color:#1A1714;font-family:Georgia,'Times New Roman',serif;">
<p style="margin:0 0 24px;font-family:Arial,Helvetica,sans-serif;font-size:14px;letter-spacing:0.04em;color:#6B6560;">Indifferent</p>
<h1 style="margin:0 0 16px;font-size:28px;line-height:1.25;font-weight:normal;color:#1A1714;">{{.Heading}}</h1>
<p style="margin:0 0 28px;font-family:Arial,Helvetica,sans-serif;font-size:16px;line-height:1.5;color:#1A1714;">{{.Intro}}</p>
<p style="margin:0 0 28px;"><a href="{{.URL}}" style="display:inline-block;padding:14px 24px;background:#1A1714;color:#FAF7F2;text-decoration:none;border-radius:999px;font-family:Arial,Helvetica,sans-serif;font-size:16px;font-weight:bold;">{{.Button}}</a></p>
<p style="margin:0 0 32px;font-family:Arial,Helvetica,sans-serif;font-size:14px;line-height:1.5;color:#6B6560;">If the button does not work, copy this link into your browser:<br><a href="{{.URL}}" style="color:#1A1714;word-break:break-all;">{{.URL}}</a></p>
<p style="margin:0;padding-top:16px;border-top:1px solid #DDD6CA;font-family:Arial,Helvetica,sans-serif;font-size:13px;line-height:1.5;color:#6B6560;">You get this email because email notifications are on for your account. <a href="{{.SettingsURL}}" style="color:#1A1714;">Change this in Settings</a>.</p>
</div>
</body>
</html>
`))
