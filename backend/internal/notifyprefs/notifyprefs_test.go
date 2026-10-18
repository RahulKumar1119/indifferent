package notifyprefs

import (
	"testing"

	dbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

func s(v string) dbtypes.AttributeValue { return &dbtypes.AttributeValueMemberS{Value: v} }
func b(v bool) dbtypes.AttributeValue   { return &dbtypes.AttributeValueMemberBOOL{Value: v} }

func TestFromItemDefaultsToOn(t *testing.T) {
	got := FromItem(map[string]dbtypes.AttributeValue{"email": s("a@b.co")})
	if !got.Completed || !got.Failed {
		t.Fatalf("absent attributes must default to on, got %+v", got)
	}
}

func TestFromItemReadsStoredValues(t *testing.T) {
	got := FromItem(map[string]dbtypes.AttributeValue{AttrCompleted: b(false), AttrFailed: b(true)})
	if got.Completed || !got.Failed {
		t.Fatalf("got %+v, want completed=false failed=true", got)
	}
}

func TestFromItemIgnoresWrongType(t *testing.T) {
	got := FromItem(map[string]dbtypes.AttributeValue{AttrCompleted: s("false")})
	if !got.Completed {
		t.Fatal("a non-BOOL value must fall back to the default (on)")
	}
}

func TestEmailVerified(t *testing.T) {
	tests := []struct {
		name string
		item map[string]dbtypes.AttributeValue
		want bool
	}{
		{"google profile (no password hash)", map[string]dbtypes.AttributeValue{"email": s("g@x.co")}, true},
		{"password profile", map[string]dbtypes.AttributeValue{"email": s("p@x.co"), "passwordHash": s("$2a$hash")}, false},
		{"empty password hash counts as oauth", map[string]dbtypes.AttributeValue{"email": s("g@x.co"), "passwordHash": s("")}, true},
		{"no email", map[string]dbtypes.AttributeValue{"name": s("x")}, false},
		{"empty profile", map[string]dbtypes.AttributeValue{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := EmailVerified(tt.item); got != tt.want {
				t.Fatalf("EmailVerified = %v, want %v", got, tt.want)
			}
		})
	}
}
