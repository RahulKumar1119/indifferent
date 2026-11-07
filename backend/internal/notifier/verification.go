package notifier

import (
	"bytes"
	"context"
	"fmt"
	htmltemplate "html/template"
	texttemplate "text/template"
)

// VerifyMailer sends the "confirm your email address" message through a Mailer.
// It satisfies auth.VerificationMailer without importing that package.
type VerifyMailer struct {
	Mailer Mailer
}

// SendVerification mails link to the address. A Mailer that sent nothing
// returns its error unchanged, so the caller can tell and clean up.
func (m VerifyMailer) SendVerification(ctx context.Context, to, link string) error {
	msg, err := VerificationMessage(to, link)
	if err != nil {
		return err
	}
	_, err = m.Mailer.Send(ctx, msg)
	return err
}

type verificationData struct {
	Heading string
	Intro   string
	Button  string
	URL     string
}

// VerificationMessage builds the confirmation email. The link is the only
// variable part; there is no tracking pixel and no other link.
func VerificationMessage(to, link string) (Message, error) {
	d := verificationData{
		Heading: "Confirm your email address",
		Intro:   "Open the link below to confirm that this address is yours. It works for 24 hours and only once.",
		Button:  "Confirm email address",
		URL:     link,
	}
	var text, html bytes.Buffer
	if err := verificationText.Execute(&text, d); err != nil {
		return Message{}, fmt.Errorf("render verification text: %w", err)
	}
	if err := verificationHTML.Execute(&html, d); err != nil {
		return Message{}, fmt.Errorf("render verification html: %w", err)
	}
	return Message{To: to, Subject: d.Heading, Text: text.String(), HTML: html.String()}, nil
}

var verificationText = texttemplate.Must(texttemplate.New("text").Parse(
	`{{.Heading}}

{{.Intro}}

{{.Button}}:
{{.URL}}

Someone used this address to create an Indifferent account. If that was not you, you can ignore this email.
`))

var verificationHTML = htmltemplate.Must(htmltemplate.New("html").Parse(
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
<p style="margin:0;padding-top:16px;border-top:1px solid #DDD6CA;font-family:Arial,Helvetica,sans-serif;font-size:13px;line-height:1.5;color:#6B6560;">Someone used this address to create an Indifferent account. If that was not you, you can ignore this email.</p>
</div>
</body>
</html>
`))
