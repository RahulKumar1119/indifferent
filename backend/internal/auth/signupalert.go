package auth

import (
	"context"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// Signup is one new account, as the owner's alert describes it (#297).
type Signup struct {
	Name   string
	Email  string
	Method string // "google" or "email"
	At     time.Time
	// Number is the account's place in the running total ("account #170"), or
	// 0 when the counter could not be read.
	Number int64
}

// SignupNotifier tells the site owner about a new account. Implementations
// must be safe to call while a sign-in is waiting: Alerter gives them a
// short deadline and only logs what they return.
type SignupNotifier interface {
	NotifySignup(ctx context.Context, s Signup) error
}

// signupAlertTimeout bounds the counter update and the send together, so a
// slow mail service adds at most this much to a first sign-in.
const signupAlertTimeout = 3 * time.Second

// Counter item in the users table: the number of real accounts ever created.
// It is not a user, and its key starts with neither USER# nor EMAIL#, so code
// that reads users ignores it.
const (
	signupCounterPK   = "COUNTER#accounts"
	signupCounterSK   = "COUNTER"
	signupCounterAttr = "n"
)

// testAccountDomain marks accounts made by tests and checks. They are neither
// counted nor reported.
const testAccountDomain = "@example.com"

// SignupAlerter counts new accounts and tells the owner about each one. The
// zero value and a nil *SignupAlerter do nothing.
type SignupAlerter struct {
	DB         DynamoDBClient
	UsersTable string
	Notifier   SignupNotifier
	// Now replaces time.Now, for tests.
	Now func() time.Time
}

// newAccount is called once for every account that was just created, after
// the account is saved. It never fails the sign-in: every error is logged
// and dropped, and the whole thing ends within signupAlertTimeout.
func (a *SignupAlerter) newAccount(ctx context.Context, name, email, method string) {
	if a == nil || a.Notifier == nil {
		return
	}
	if strings.HasSuffix(strings.ToLower(email), testAccountDomain) {
		return
	}
	// The sign-in's own context may be cancelled the moment it returns; the
	// alert keeps its own short deadline instead.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), signupAlertTimeout)
	defer cancel()
	now := time.Now
	if a.Now != nil {
		now = a.Now
	}
	s := Signup{Name: name, Email: email, Method: method, At: now().UTC(), Number: a.count(ctx)}
	if err := a.Notifier.NotifySignup(ctx, s); err != nil {
		log.Printf("signup_alert_failed method=%s number=%d err=%v", method, s.Number, err)
		return
	}
	log.Printf("signup_alert_sent method=%s number=%d", method, s.Number)
}

// count adds one to the account counter and returns the new value, or 0 when
// the counter cannot be updated (the alert then goes out without a number).
func (a *SignupAlerter) count(ctx context.Context) int64 {
	if a.DB == nil || a.UsersTable == "" {
		return 0
	}
	out, err := a.DB.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(a.UsersTable),
		Key: map[string]dbtypes.AttributeValue{
			"PK": &dbtypes.AttributeValueMemberS{Value: signupCounterPK},
			"SK": &dbtypes.AttributeValueMemberS{Value: signupCounterSK},
		},
		UpdateExpression:          aws.String("ADD #n :one"),
		ExpressionAttributeNames:  map[string]string{"#n": signupCounterAttr},
		ExpressionAttributeValues: map[string]dbtypes.AttributeValue{":one": &dbtypes.AttributeValueMemberN{Value: "1"}},
		ReturnValues:              dbtypes.ReturnValueUpdatedNew,
	})
	if err != nil {
		log.Printf("signup_counter_failed err=%v", err)
		return 0
	}
	v, ok := out.Attributes[signupCounterAttr].(*dbtypes.AttributeValueMemberN)
	if !ok {
		return 0
	}
	n, err := strconv.ParseInt(v.Value, 10, 64)
	if err != nil {
		return 0
	}
	return n
}
