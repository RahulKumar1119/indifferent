package auth

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// sessionFamilyStartMs is when the sign-in behind a session began. Sessions
// written before familyStartMs existed fall back to their own createdAt (the
// last rotation), so an old family starts its 30-day clock at its next refresh
// after the deploy. Zero means unknown.
func sessionFamilyStartMs(item map[string]dbtypes.AttributeValue) int64 {
	if n, ok := item["familyStartMs"].(*dbtypes.AttributeValueMemberN); ok {
		if v, err := strconv.ParseInt(n.Value, 10, 64); err == nil && v > 0 {
			return v
		}
	}
	if t, err := time.Parse(time.RFC3339, extractStringAttr(item, "createdAt")); err == nil {
		return t.UnixMilli()
	}
	return 0
}

// sessionsRevokedBefore is the "sign out everywhere" mark on the user's
// profile, in Unix milliseconds: no session whose sign-in began at or before it
// may refresh. Zero when the user never used it, or when no users table is set.
func (s *JWTService) sessionsRevokedBefore(ctx context.Context, userID string) (int64, error) {
	if s.UsersTable == "" {
		return 0, nil
	}
	out, err := s.DB.GetItem(ctx, &dynamodb.GetItemInput{
		TableName:      aws.String(s.UsersTable),
		Key:            userKey(userID),
		ConsistentRead: aws.Bool(true),
	})
	if err != nil {
		return 0, fmt.Errorf("get user: %w", err)
	}
	if out.Item == nil {
		return 0, nil
	}
	if n, ok := out.Item["sessionsRevokedBeforeMs"].(*dbtypes.AttributeValueMemberN); ok {
		if v, err := strconv.ParseInt(n.Value, 10, 64); err == nil {
			return v, nil
		}
	}
	return 0, nil
}

// LogoutAll signs the user out of every device: from now on no refresh token
// from a sign-in made before this moment will work. Access tokens already
// issued keep working until they expire (at most AccessTokenTTL, 15 minutes).
// The sign-in that follows is unaffected, even within the same second, because
// both sides are compared in milliseconds.
func (s *JWTService) LogoutAll(ctx context.Context, userID string) error {
	if s.UsersTable == "" {
		return fmt.Errorf("users table is not configured")
	}
	_, err := s.DB.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:           aws.String(s.UsersTable),
		Key:                 userKey(userID),
		UpdateExpression:    aws.String("SET sessionsRevokedBeforeMs = :now"),
		ConditionExpression: aws.String("attribute_exists(PK)"),
		ExpressionAttributeValues: map[string]dbtypes.AttributeValue{
			":now": &dbtypes.AttributeValueMemberN{Value: strconv.FormatInt(time.Now().UnixMilli(), 10)},
		},
	})
	if err != nil {
		return fmt.Errorf("revoke sessions: %w", err)
	}
	return nil
}
