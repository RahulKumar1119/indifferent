// Package config provides application configuration loading
// from environment variables and AWS-specific settings.
package config

import (
	"os"
	"strings"
)

// S3Accelerate reports whether the presigned part URLs of a multipart upload
// point at the S3 Transfer Acceleration endpoint. It is read from the
// S3_ACCELERATE environment variable and is off unless that is set to a value
// Enabled accepts, so it doubles as the kill switch.
func S3Accelerate() bool {
	return Enabled(os.Getenv("S3_ACCELERATE"))
}

// Enabled reports whether an on/off setting is on. Only "true" (any case) and
// "1" turn it on; an empty value or anything else leaves it off.
func Enabled(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "1":
		return true
	}
	return false
}
