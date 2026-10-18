package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/rahul/indifferent/backend/internal/models"
	"github.com/rahul/indifferent/backend/internal/notifyprefs"
)

// NotificationSettings is the response of GET and PUT /settings/notifications.
// EmailVerified tells the client whether notification email can reach this
// account at all (see notifyprefs.EmailVerified); preferences can still be
// stored while it is false.
type NotificationSettings struct {
	Email           string `json:"email"`
	EmailVerified   bool   `json:"emailVerified"`
	NotifyCompleted bool   `json:"notifyCompleted"`
	NotifyFailed    bool   `json:"notifyFailed"`
}

func notificationSettingsFromItem(item map[string]dbtypes.AttributeValue) NotificationSettings {
	prefs := notifyprefs.FromItem(item)
	return NotificationSettings{
		Email:           notifyprefs.Email(item),
		EmailVerified:   notifyprefs.EmailVerified(item),
		NotifyCompleted: prefs.Completed,
		NotifyFailed:    prefs.Failed,
	}
}

// loadProfile reads the caller's profile item from the users table.
func (h *APIHandler) loadProfile(ctx context.Context, userID string) (map[string]dbtypes.AttributeValue, error) {
	result, err := h.DB.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(h.UsersTable),
		Key: map[string]dbtypes.AttributeValue{
			"PK": &dbtypes.AttributeValueMemberS{Value: "USER#" + userID},
			"SK": &dbtypes.AttributeValueMemberS{Value: "PROFILE"},
		},
		ConsistentRead: aws.Bool(true),
	})
	if err != nil {
		return nil, err
	}
	return result.Item, nil
}

// handleGetNotificationSettings returns the caller's email notification
// preferences and the address they apply to.
func (h *APIHandler) handleGetNotificationSettings(ctx context.Context, req events.APIGatewayProxyRequest, claims *models.JWTClaims) (events.APIGatewayProxyResponse, error) {
	if h.UsersTable == "" {
		return errorResponse(http.StatusInternalServerError, "NOT_CONFIGURED", "User profiles are not enabled"), nil
	}
	item, err := h.loadProfile(ctx, claims.UserID)
	if err != nil {
		return errorResponse(http.StatusInternalServerError, "DB_ERROR", "Failed to load notification settings"), nil
	}
	if item == nil {
		return errorResponse(http.StatusNotFound, "USER_NOT_FOUND", "User profile not found"), nil
	}
	return jsonResponse(http.StatusOK, notificationSettingsFromItem(item)), nil
}

// handleUpdateNotificationSettings changes one or both preferences. The body
// must be a JSON object holding only notifyCompleted and/or notifyFailed, each
// a boolean. Anything else is rejected rather than ignored.
func (h *APIHandler) handleUpdateNotificationSettings(ctx context.Context, req events.APIGatewayProxyRequest, claims *models.JWTClaims) (events.APIGatewayProxyResponse, error) {
	if h.UsersTable == "" {
		return errorResponse(http.StatusInternalServerError, "NOT_CONFIGURED", "User profiles are not enabled"), nil
	}

	var body struct {
		NotifyCompleted *bool `json:"notifyCompleted"`
		NotifyFailed    *bool `json:"notifyFailed"`
	}
	dec := json.NewDecoder(bytes.NewReader([]byte(req.Body)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil || dec.More() {
		return errorResponse(http.StatusBadRequest, "INVALID_BODY", "Body must be a JSON object with boolean notifyCompleted and/or notifyFailed"), nil
	}
	if body.NotifyCompleted == nil && body.NotifyFailed == nil {
		return errorResponse(http.StatusBadRequest, "INVALID_INPUT", "Provide notifyCompleted, notifyFailed or both"), nil
	}

	expr := "SET "
	names := map[string]string{}
	values := map[string]dbtypes.AttributeValue{}
	add := func(name, placeholder string, v *bool) {
		if v == nil {
			return
		}
		if len(names) > 0 {
			expr += ", "
		}
		names["#"+placeholder] = name
		values[":"+placeholder] = &dbtypes.AttributeValueMemberBOOL{Value: *v}
		expr += "#" + placeholder + " = :" + placeholder
	}
	add(notifyprefs.AttrCompleted, "c", body.NotifyCompleted)
	add(notifyprefs.AttrFailed, "f", body.NotifyFailed)

	_, err := h.DB.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(h.UsersTable),
		Key: map[string]dbtypes.AttributeValue{
			"PK": &dbtypes.AttributeValueMemberS{Value: "USER#" + claims.UserID},
			"SK": &dbtypes.AttributeValueMemberS{Value: "PROFILE"},
		},
		UpdateExpression:          aws.String(expr),
		ExpressionAttributeNames:  names,
		ExpressionAttributeValues: values,
		// Never create a profile just to hold preferences.
		ConditionExpression: aws.String("attribute_exists(PK)"),
	})
	if err != nil {
		var cond *dbtypes.ConditionalCheckFailedException
		if errors.As(err, &cond) {
			return errorResponse(http.StatusNotFound, "USER_NOT_FOUND", "User profile not found"), nil
		}
		return errorResponse(http.StatusInternalServerError, "DB_ERROR", "Failed to save notification settings"), nil
	}

	item, err := h.loadProfile(ctx, claims.UserID)
	if err != nil || item == nil {
		return errorResponse(http.StatusInternalServerError, "DB_ERROR", "Saved, but failed to read the settings back"), nil
	}
	return jsonResponse(http.StatusOK, notificationSettingsFromItem(item)), nil
}
