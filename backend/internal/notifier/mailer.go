package notifier

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	sestypes "github.com/aws/aws-sdk-go-v2/service/sesv2/types"
	"github.com/aws/smithy-go"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

// Message is one outgoing email.
type Message struct {
	To      string
	Subject string
	Text    string
	HTML    string
}

// Mailer sends a Message and returns the provider's message id.
//
// Implementations wrap their error with ErrThrottled when nothing was sent
// and a retry is safe, with ErrRejected when nothing was sent and a retry
// will not help, and return any other error unchanged when the outcome of
// the send is unknown.
type Mailer interface {
	Send(ctx context.Context, m Message) (messageID string, err error)
}

var (
	// ErrThrottled marks a send that did not happen and can be retried.
	ErrThrottled = errors.New("send throttled or provider unavailable")
	// ErrRejected marks a send that did not happen and will not succeed on retry.
	ErrRejected = errors.New("send rejected")
)

// SESAPI is the part of the SES v2 client the mailer uses.
type SESAPI interface {
	SendEmail(ctx context.Context, params *sesv2.SendEmailInput, optFns ...func(*sesv2.Options)) (*sesv2.SendEmailOutput, error)
}

// SESMailer sends through Amazon SES v2.
type SESMailer struct {
	Client    SESAPI
	From      string // for example: Indifferent <notifications@indifferent.fun>
	ReplyTo   string
	ConfigSet string
}

// Send implements Mailer.
func (m *SESMailer) Send(ctx context.Context, msg Message) (string, error) {
	in := &sesv2.SendEmailInput{
		FromEmailAddress: aws.String(m.From),
		Destination:      &sestypes.Destination{ToAddresses: []string{msg.To}},
		Content: &sestypes.EmailContent{
			Simple: &sestypes.Message{
				Subject: &sestypes.Content{Data: aws.String(msg.Subject), Charset: aws.String("UTF-8")},
				Body: &sestypes.Body{
					Text: &sestypes.Content{Data: aws.String(msg.Text), Charset: aws.String("UTF-8")},
					Html: &sestypes.Content{Data: aws.String(msg.HTML), Charset: aws.String("UTF-8")},
				},
			},
		},
	}
	if m.ReplyTo != "" {
		in.ReplyToAddresses = []string{m.ReplyTo}
	}
	if m.ConfigSet != "" {
		in.ConfigurationSetName = aws.String(m.ConfigSet)
	}

	out, err := m.Client.SendEmail(ctx, in)
	if err != nil {
		return "", classifySESError(err)
	}
	return aws.ToString(out.MessageId), nil
}

// classifySESError maps an SES v2 error onto the Mailer error contract.
// The SDK has already retried transient failures, so a throttle or 5xx that
// reaches here is handed to the state machine's retry. Errors that are not
// an API response (timeouts, dropped connections) are returned unchanged:
// the request may have been accepted, so a retry could duplicate the email.
func classifySESError(err error) error {
	var tooMany *sestypes.TooManyRequestsException
	var limit *sestypes.LimitExceededException
	if errors.As(err, &tooMany) || errors.As(err, &limit) {
		return fmt.Errorf("%w: %v", ErrThrottled, err)
	}

	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "Throttling", "ThrottlingException", "RequestLimitExceeded",
			"InternalFailure", "InternalServerError", "ServiceUnavailable":
			return fmt.Errorf("%w: %v", ErrThrottled, err)
		}
	}

	var respErr *smithyhttp.ResponseError
	if errors.As(err, &respErr) && respErr.HTTPStatusCode() >= http.StatusInternalServerError {
		return fmt.Errorf("%w: %v", ErrThrottled, err)
	}

	// Definitive client-side refusals: the request reached SES and was rejected.
	var rejected *sestypes.MessageRejected
	var notFound *sestypes.NotFoundException
	var badReq *sestypes.BadRequestException
	var paused *sestypes.SendingPausedException
	var mailFrom *sestypes.MailFromDomainNotVerifiedException
	var suspended *sestypes.AccountSuspendedException
	if errors.As(err, &rejected) || errors.As(err, &notFound) || errors.As(err, &badReq) ||
		errors.As(err, &paused) || errors.As(err, &mailFrom) || errors.As(err, &suspended) {
		return fmt.Errorf("%w: %v", ErrRejected, err)
	}

	return err
}
