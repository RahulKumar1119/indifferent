package api

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/rahul/indifferent/backend/internal/auth"
	"github.com/rahul/indifferent/backend/internal/models"
	"github.com/rahul/indifferent/backend/internal/notifyprefs"
)

// quotaVerifyEmail caps how many confirmation emails one account can ask for in
// a day. Each one is mail sent from our shared domain, so a script must not be
// able to turn the button into a mail cannon.
var quotaVerifyEmail = quota{name: "verify-email", limit: 5, what: "sending confirmation emails"}

// spendingRequest reports whether the request starts work that costs money or
// render capacity: the same set the daily quotas count. These wait for a
// confirmed email address. Reading, editing, cancelling and deleting never do.
func spendingRequest(req events.APIGatewayProxyRequest) bool {
	if req.HTTPMethod != http.MethodPost {
		return false
	}
	p := req.Path
	switch {
	case p == "/projects", p == "/shorts":
		return true
	case strings.HasPrefix(p, "/story-plans"), strings.HasPrefix(p, "/story-videos"):
		return true
	case strings.HasPrefix(p, "/projects/"):
		return strings.HasSuffix(p, "/start") || strings.HasSuffix(p, "/upload") || strings.HasSuffix(p, "/logo")
	case strings.HasPrefix(p, "/shorts/"):
		return strings.HasSuffix(p, "/start") || strings.HasSuffix(p, "/retry") ||
			strings.Contains(p, "/upload/multipart") || strings.Contains(p, "/thumbnails")
	}
	return false
}

// requireConfirmedEmail refuses a spending request from an account that is
// marked emailVerified=false: a password sign-up whose address nobody has
// confirmed. Accounts without the mark (Google sign-ins, and password accounts
// that existed before confirmation did) are not held back.
//
// With no Verifier (the feature switched off) nothing is held back, including
// accounts that were created while it was on.
//
// It fails open. A missing profile, or a DynamoDB error, lets the request
// through and is logged: this check protects spending, and an outage must not
// also lock every paying user out.
func (h *APIHandler) requireConfirmedEmail(ctx context.Context, req events.APIGatewayProxyRequest, claims *models.JWTClaims) *events.APIGatewayProxyResponse {
	if h.Verifier == nil || h.UsersTable == "" || !spendingRequest(req) {
		return nil
	}
	out, err := h.DB.GetItem(ctx, &dynamodb.GetItemInput{
		TableName:      aws.String(h.UsersTable),
		Key:            map[string]dbtypes.AttributeValue{"PK": &dbtypes.AttributeValueMemberS{Value: "USER#" + claims.UserID}, "SK": &dbtypes.AttributeValueMemberS{Value: "PROFILE"}},
		ConsistentRead: aws.Bool(true),
	})
	if err != nil {
		log.Printf("email_gate_lookup_failed err=%v", err)
		return nil
	}
	if v, ok := out.Item["emailVerified"].(*dbtypes.AttributeValueMemberBOOL); ok && !v.Value {
		resp := errorResponse(http.StatusForbidden, "EMAIL_NOT_VERIFIED",
			"Confirm your email address first. We sent you a link; you can ask for a new one on your account page.")
		return &resp
	}
	return nil
}

// handleVerifyEmail consumes a confirmation token from the emailed link. It
// needs no sign-in: the person may open the link on another device.
func (h *APIHandler) handleVerifyEmail(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	if h.Verifier == nil {
		return errorResponse(http.StatusServiceUnavailable, "NOT_CONFIGURED", "Email confirmation is not available right now"), nil
	}
	var body struct {
		Token string `json:"token"`
	}
	if len(req.Body) > 1024 || json.Unmarshal([]byte(req.Body), &body) != nil {
		return errorResponse(http.StatusBadRequest, "INVALID_BODY", "Invalid request body"), nil
	}
	if err := h.Verifier.Verify(ctx, strings.TrimSpace(body.Token)); err != nil {
		if errors.Is(err, auth.ErrInvalidVerification) {
			return errorResponse(http.StatusBadRequest, "INVALID_TOKEN", "This link has expired or was already used. Sign in and ask for a new one from your account page."), nil
		}
		log.Printf("verify_email_failed err=%v", err)
		return errorResponse(http.StatusInternalServerError, "VERIFY_FAILED", "Could not confirm your email address. Please try again."), nil
	}
	return events.APIGatewayProxyResponse{StatusCode: http.StatusNoContent, Headers: corsHeaders()}, nil
}

// handleResendVerification sends a fresh confirmation link to the signed-in
// account's own address. It is a no-op for an address that is already confirmed.
func (h *APIHandler) handleResendVerification(ctx context.Context, req events.APIGatewayProxyRequest, claims *models.JWTClaims) (events.APIGatewayProxyResponse, error) {
	if h.Verifier == nil || h.UsersTable == "" {
		return errorResponse(http.StatusServiceUnavailable, "NOT_CONFIGURED", "Email confirmation is not available right now"), nil
	}
	out, err := h.DB.GetItem(ctx, &dynamodb.GetItemInput{
		TableName:      aws.String(h.UsersTable),
		Key:            map[string]dbtypes.AttributeValue{"PK": &dbtypes.AttributeValueMemberS{Value: "USER#" + claims.UserID}, "SK": &dbtypes.AttributeValueMemberS{Value: "PROFILE"}},
		ConsistentRead: aws.Bool(true),
	})
	if err != nil {
		return errorResponse(http.StatusInternalServerError, "DB_ERROR", "Could not load your account"), nil
	}
	if out.Item == nil {
		return errorResponse(http.StatusNotFound, "USER_NOT_FOUND", "User profile not found"), nil
	}
	if notifyprefs.EmailVerified(out.Item) {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusNoContent, Headers: corsHeaders()}, nil
	}
	// The address comes from the stored profile, never from the request.
	email := notifyprefs.Email(out.Item)
	if email == "" {
		return errorResponse(http.StatusBadRequest, "NO_EMAIL", "This account has no email address"), nil
	}
	if resp := h.checkQuota(ctx, claims.UserID, quotaVerifyEmail); resp != nil {
		return *resp, nil
	}
	if err := h.Verifier.Send(ctx, claims.UserID, email); err != nil {
		log.Printf("resend_verification_failed user=%s err=%v", claims.UserID, err)
		return errorResponse(http.StatusInternalServerError, "SEND_FAILED", "Could not send the email. Please try again in a few minutes."), nil
	}
	return events.APIGatewayProxyResponse{StatusCode: http.StatusNoContent, Headers: corsHeaders()}, nil
}
