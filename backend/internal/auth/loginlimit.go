package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

const (
	// LoginMaxAttempts is how many password logins one email address may try in
	// a LoginWindow. It is deliberately above what a person mistypes, and the
	// count includes the successful attempt.
	LoginMaxAttempts = 10
	// LoginWindow is the fixed window the attempts are counted in.
	LoginWindow = 15 * time.Minute
)

// TooManyAttemptsError is returned by Login when the address has used up its
// attempts for the current window. It is returned before any password check.
type TooManyAttemptsError struct{ RetryAfter time.Duration }

func (e *TooManyAttemptsError) Error() string {
	return fmt.Sprintf("too many login attempts, retry in %s", e.RetryAfter.Round(time.Second))
}

// loginNow is replaced in tests.
var loginNow = time.Now

// loginAttemptKey is the counter item for one address and one window. The
// address is hashed so the table holds no plaintext email in this key, and the
// window number is part of the key so a counter never needs resetting: the
// next window simply uses a new item and TTL removes the old one.
func loginAttemptKey(email string, window int64) map[string]dbtypes.AttributeValue {
	sum := sha256.Sum256([]byte(normalizeEmail(email)))
	return map[string]dbtypes.AttributeValue{
		"PK": &dbtypes.AttributeValueMemberS{Value: "LOGINATTEMPT#" + hex.EncodeToString(sum[:])},
		"SK": &dbtypes.AttributeValueMemberS{Value: "W#" + strconv.FormatInt(window, 10)},
	}
}

// countLoginAttempt records one attempt for the address and reports whether it
// is still within the limit. The increment is a single atomic DynamoDB ADD
// done before the password is checked, so a burst of parallel requests cannot
// all slip under the limit. If the counter cannot be written the attempt is
// allowed (and logged): a DynamoDB fault must not lock everyone out.
func (s *PasswordAuthService) countLoginAttempt(ctx context.Context, email string) (time.Duration, bool) {
	now := loginNow()
	window := now.Unix() / int64(LoginWindow.Seconds())
	windowEnd := time.Unix((window+1)*int64(LoginWindow.Seconds()), 0)

	out, err := s.DB.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:        aws.String(s.SessionTable),
		Key:              loginAttemptKey(email, window),
		UpdateExpression: aws.String("ADD attempts :one SET expiresAt = :exp"),
		ExpressionAttributeValues: map[string]dbtypes.AttributeValue{
			":one": &dbtypes.AttributeValueMemberN{Value: "1"},
			// Keep the item a while after the window so late reads still see it.
			":exp": &dbtypes.AttributeValueMemberN{Value: strconv.FormatInt(windowEnd.Add(LoginWindow).Unix(), 10)},
		},
		ReturnValues: dbtypes.ReturnValueUpdatedNew,
	})
	if err != nil {
		log.Printf("login_attempt_count_failed err=%v", err)
		return 0, true
	}
	n, ok := out.Attributes["attempts"].(*dbtypes.AttributeValueMemberN)
	if !ok {
		return 0, true
	}
	count, err := strconv.Atoi(n.Value)
	if err != nil || count <= LoginMaxAttempts {
		return 0, true
	}
	return windowEnd.Sub(now), false
}

// clearLoginAttempts forgets the current window's attempts after a successful
// login, so a person who typed a few wrong passwords first is not penalised.
func (s *PasswordAuthService) clearLoginAttempts(ctx context.Context, email string) {
	window := loginNow().Unix() / int64(LoginWindow.Seconds())
	if _, err := s.DB.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(s.SessionTable),
		Key:       loginAttemptKey(email, window),
	}); err != nil {
		log.Printf("login_attempt_clear_failed err=%v", err)
	}
}
