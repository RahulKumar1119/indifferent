package notifier

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	sestypes "github.com/aws/aws-sdk-go-v2/service/sesv2/types"
	"github.com/aws/smithy-go"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

type fakeSES struct {
	got *sesv2.SendEmailInput
	out *sesv2.SendEmailOutput
	err error
}

func (f *fakeSES) SendEmail(_ context.Context, in *sesv2.SendEmailInput, _ ...func(*sesv2.Options)) (*sesv2.SendEmailOutput, error) {
	f.got = in
	return f.out, f.err
}

func TestSESMailerBuildsRequest(t *testing.T) {
	ses := &fakeSES{out: &sesv2.SendEmailOutput{MessageId: aws.String("ses-123")}}
	m := &SESMailer{
		Client:    ses,
		From:      "Indifferent <notifications@indifferent.fun>",
		ReplyTo:   "support@indifferent.fun",
		ConfigSet: "indifferent-notifications",
	}
	id, err := m.Send(context.Background(), Message{To: "person@example.com", Subject: "Hi", Text: "plain", HTML: "<p>html</p>"})
	if err != nil || id != "ses-123" {
		t.Fatalf("id=%q err=%v", id, err)
	}
	in := ses.got
	if aws.ToString(in.FromEmailAddress) != "Indifferent <notifications@indifferent.fun>" {
		t.Errorf("from = %q", aws.ToString(in.FromEmailAddress))
	}
	if len(in.Destination.ToAddresses) != 1 || in.Destination.ToAddresses[0] != "person@example.com" {
		t.Errorf("to = %v", in.Destination.ToAddresses)
	}
	if len(in.Destination.CcAddresses)+len(in.Destination.BccAddresses) != 0 {
		t.Error("no cc/bcc expected")
	}
	if len(in.ReplyToAddresses) != 1 || in.ReplyToAddresses[0] != "support@indifferent.fun" {
		t.Errorf("reply-to = %v", in.ReplyToAddresses)
	}
	if aws.ToString(in.ConfigurationSetName) != "indifferent-notifications" {
		t.Errorf("config set = %q", aws.ToString(in.ConfigurationSetName))
	}
	body := in.Content.Simple.Body
	if aws.ToString(body.Text.Data) != "plain" || aws.ToString(body.Html.Data) != "<p>html</p>" {
		t.Error("text/html parts not set")
	}
	if aws.ToString(in.Content.Simple.Subject.Charset) != "UTF-8" || aws.ToString(body.Html.Charset) != "UTF-8" {
		t.Error("charset must be UTF-8")
	}
}

func TestClassifySESError(t *testing.T) {
	respErr := func(code int) error {
		return &smithyhttp.ResponseError{
			Response: &smithyhttp.Response{Response: &http.Response{StatusCode: code}},
			Err:      errors.New("boom"),
		}
	}
	tests := []struct {
		name       string
		err        error
		wantThrot  bool
		wantReject bool
	}{
		{"TooManyRequests", &sestypes.TooManyRequestsException{Message: aws.String("slow")}, true, false},
		{"LimitExceeded", &sestypes.LimitExceededException{Message: aws.String("limit")}, true, false},
		{"wrapped TooManyRequests", fmt.Errorf("operation error: %w", &sestypes.TooManyRequestsException{}), true, false},
		{"ThrottlingException code", &smithy.GenericAPIError{Code: "ThrottlingException"}, true, false},
		{"InternalFailure code", &smithy.GenericAPIError{Code: "InternalFailure"}, true, false},
		{"HTTP 503", respErr(503), true, false},
		{"HTTP 500", respErr(500), true, false},
		{"MessageRejected", &sestypes.MessageRejected{Message: aws.String("no")}, false, true},
		{"NotFound (identity)", &sestypes.NotFoundException{}, false, true},
		{"BadRequest", &sestypes.BadRequestException{}, false, true},
		{"SendingPaused", &sestypes.SendingPausedException{}, false, true},
		{"MailFromDomainNotVerified", &sestypes.MailFromDomainNotVerifiedException{}, false, true},
		{"AccountSuspended", &sestypes.AccountSuspendedException{}, false, true},
		{"HTTP 400 is not retried", respErr(400), false, false},
		{"plain error (outcome unknown)", errors.New("context deadline exceeded"), false, false},
		{"network timeout (outcome unknown)", &net.OpError{Op: "read", Err: errors.New("i/o timeout")}, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifySESError(tt.err)
			if errors.Is(got, ErrThrottled) != tt.wantThrot {
				t.Errorf("throttled = %v, want %v (err: %v)", errors.Is(got, ErrThrottled), tt.wantThrot, got)
			}
			if errors.Is(got, ErrRejected) != tt.wantReject {
				t.Errorf("rejected = %v, want %v (err: %v)", errors.Is(got, ErrRejected), tt.wantReject, got)
			}
		})
	}
}

func TestSESMailerWrapsErrors(t *testing.T) {
	ses := &fakeSES{err: &sestypes.TooManyRequestsException{}}
	m := &SESMailer{Client: ses, From: "a@b.co"}
	if _, err := m.Send(context.Background(), Message{To: "x@y.co"}); !errors.Is(err, ErrThrottled) {
		t.Fatalf("want ErrThrottled, got %v", err)
	}
}
