package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

const (
	// EmailVerificationTTL is how long a confirmation link works.
	EmailVerificationTTL = 24 * time.Hour

	verifyTokenBytes = 32
	verifyKeyPrefix  = "EMAILVERIFY#"
)

// ErrInvalidVerification is returned for a confirmation token that is
// malformed, unknown, already used or expired. The cases are not told apart:
// the caller learns nothing it could use to probe tokens.
var ErrInvalidVerification = errors.New("verification link is invalid or has expired")

// VerificationMailer sends the confirmation email. link is the full URL the
// person opens. The implementation lives in the notifier package (SES).
type VerificationMailer interface {
	SendVerification(ctx context.Context, to, link string) error
}

// EmailVerifier proves that the person who created a password account controls
// the address they typed. It mails a single-use link and, when the link is
// opened, marks the profile emailVerified.
//
// Tokens are 32 random bytes; only their SHA-256 is stored, in the sessions
// table (TokenTable), which has TTL on expiresAt. A token is consumed with one
// conditional delete, so two clicks cannot both succeed.
type EmailVerifier struct {
	DB         DynamoDBClient
	UsersTable string
	TokenTable string
	BaseURL    string // for example https://indifferent.fun
	Mailer     VerificationMailer
}

func verifyTokenKey(token string) map[string]dbtypes.AttributeValue {
	sum := sha256.Sum256([]byte(token))
	return map[string]dbtypes.AttributeValue{
		"PK": &dbtypes.AttributeValueMemberS{Value: verifyKeyPrefix + hex.EncodeToString(sum[:])},
		"SK": &dbtypes.AttributeValueMemberS{Value: "TOKEN"},
	}
}

// validVerifyToken reports whether s has the shape of a token we issue, so
// anything else is refused before it touches the database.
func validVerifyToken(s string) bool {
	if len(s) != verifyTokenBytes*2 {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil
}

// Send creates a token for the account and mails the link to email. If the mail
// cannot be sent the token is removed again, so nothing valid is left behind
// that nobody received.
func (v *EmailVerifier) Send(ctx context.Context, userID, email string) error {
	if v.Mailer == nil {
		return errors.New("verification mail is not configured")
	}
	raw := make([]byte, verifyTokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return fmt.Errorf("generate token: %w", err)
	}
	token := hex.EncodeToString(raw)
	key := verifyTokenKey(token)
	expires := time.Now().Add(EmailVerificationTTL).Unix()

	if _, err := v.DB.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(v.TokenTable),
		Item: map[string]dbtypes.AttributeValue{
			"PK":        key["PK"],
			"SK":        key["SK"],
			"userId":    &dbtypes.AttributeValueMemberS{Value: userID},
			"email":     &dbtypes.AttributeValueMemberS{Value: email},
			"expiresAt": &dbtypes.AttributeValueMemberN{Value: strconv.FormatInt(expires, 10)},
		},
	}); err != nil {
		return fmt.Errorf("store token: %w", err)
	}

	link := strings.TrimRight(v.BaseURL, "/") + "/verify-email?token=" + token
	if err := v.Mailer.SendVerification(ctx, email, link); err != nil {
		_, _ = v.DB.DeleteItem(ctx, &dynamodb.DeleteItemInput{TableName: aws.String(v.TokenTable), Key: key})
		return fmt.Errorf("send verification email: %w", err)
	}
	return nil
}

// Verify consumes the token and marks the account's address verified.
func (v *EmailVerifier) Verify(ctx context.Context, token string) error {
	if !validVerifyToken(token) {
		return ErrInvalidVerification
	}
	// One conditional delete both checks and consumes the token. The expiry is
	// checked here as well as by TTL, because DynamoDB removes expired items
	// some time after they expire.
	out, err := v.DB.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName:           aws.String(v.TokenTable),
		Key:                 verifyTokenKey(token),
		ConditionExpression: aws.String("attribute_exists(PK) AND expiresAt > :now"),
		ExpressionAttributeValues: map[string]dbtypes.AttributeValue{
			":now": &dbtypes.AttributeValueMemberN{Value: strconv.FormatInt(time.Now().Unix(), 10)},
		},
		ReturnValues: dbtypes.ReturnValueAllOld,
	})
	if err != nil {
		var cond *dbtypes.ConditionalCheckFailedException
		if errors.As(err, &cond) {
			return ErrInvalidVerification
		}
		return fmt.Errorf("consume token: %w", err)
	}
	userID := extractStringAttr(out.Attributes, "userId")
	email := extractStringAttr(out.Attributes, "email")
	if userID == "" || email == "" {
		return ErrInvalidVerification
	}

	// The address must still be the one the link was sent to.
	_, err = v.DB.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:           aws.String(v.UsersTable),
		Key:                 userKey(userID),
		UpdateExpression:    aws.String("SET emailVerified = :yes, emailVerifiedAt = :at"),
		ConditionExpression: aws.String("attribute_exists(PK) AND email = :email"),
		ExpressionAttributeValues: map[string]dbtypes.AttributeValue{
			":yes":   &dbtypes.AttributeValueMemberBOOL{Value: true},
			":at":    &dbtypes.AttributeValueMemberS{Value: time.Now().UTC().Format(time.RFC3339)},
			":email": &dbtypes.AttributeValueMemberS{Value: email},
		},
	})
	if err != nil {
		var cond *dbtypes.ConditionalCheckFailedException
		if errors.As(err, &cond) {
			return ErrInvalidVerification
		}
		return fmt.Errorf("mark verified: %w", err)
	}
	return nil
}
