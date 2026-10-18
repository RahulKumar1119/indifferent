// Package notifyprefs holds the email notification preferences stored on a
// user's profile item (users table, PK=USER#<id>, SK=PROFILE) and the rule
// for whether that profile's address may receive notification email.
//
// It is shared by the API (read and update preferences) and the notifier
// Lambda (decide whether to send), so both always agree on the defaults.
package notifyprefs

import (
	dbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// Attribute names on the profile item.
const (
	AttrCompleted = "notifyCompleted"
	AttrFailed    = "notifyFailed"
)

// Prefs are a user's notification choices. Both default to true: these are
// service messages about work the user started, not marketing.
type Prefs struct {
	Completed bool
	Failed    bool
}

// FromItem reads the preferences from a profile item. An absent attribute, or
// one that is not a DynamoDB BOOL, means the default (true).
func FromItem(item map[string]dbtypes.AttributeValue) Prefs {
	return Prefs{
		Completed: boolAttr(item, AttrCompleted),
		Failed:    boolAttr(item, AttrFailed),
	}
}

func boolAttr(item map[string]dbtypes.AttributeValue, key string) bool {
	if v, ok := item[key].(*dbtypes.AttributeValueMemberBOOL); ok {
		return v.Value
	}
	return true
}

// EmailVerified reports whether the profile's address is known to belong to
// the account holder, which is the condition for sending it any email.
//
// Google sign-in profiles are written without a password hash, and Google has
// already verified that address. Password sign-ups always store a password
// hash and the app does not yet confirm ownership of the address they typed,
// so mail to it could reach a stranger and would count against our shared SES
// sender reputation. Those profiles are treated as unverified.
//
// When email verification is added for password accounts, this is the one
// place to change.
func EmailVerified(item map[string]dbtypes.AttributeValue) bool {
	if Email(item) == "" {
		return false
	}
	if v, ok := item["passwordHash"].(*dbtypes.AttributeValueMemberS); ok && v.Value != "" {
		return false
	}
	return true
}

// Email returns the profile's address ("" when absent).
func Email(item map[string]dbtypes.AttributeValue) string {
	if v, ok := item["email"].(*dbtypes.AttributeValueMemberS); ok {
		return v.Value
	}
	return ""
}
