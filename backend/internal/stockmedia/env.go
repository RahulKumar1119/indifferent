package stockmedia

import (
	"fmt"
	"os"
	"strings"
)

// Environment variables holding the provider API keys. They are set on the
// Lambda from the deployment's secrets, never in code or in the frontend.
const (
	EnvPexelsKey  = "PEXELS_API_KEY"
	EnvPixabayKey = "PIXABAY_API_KEY"
)

// NewServiceFromEnv builds a Service from the environment.
//
// A provider whose key is unset is left out, so the service still works on
// the other one: the feature degrades instead of failing when one key is
// missing or revoked. It returns ErrNotConfigured when neither key is set.
func NewServiceFromEnv(opts ...Option) (*Service, error) {
	return newServiceFromGetenv(os.Getenv, opts...)
}

func newServiceFromGetenv(getenv func(string) string, opts ...Option) (*Service, error) {
	var providers []Provider
	if key := strings.TrimSpace(getenv(EnvPexelsKey)); key != "" {
		p, err := NewPexels(ProviderConfig{APIKey: Secret(key)})
		if err != nil {
			return nil, err
		}
		providers = append(providers, p)
	}
	if key := strings.TrimSpace(getenv(EnvPixabayKey)); key != "" {
		p, err := NewPixabay(ProviderConfig{APIKey: Secret(key)})
		if err != nil {
			return nil, err
		}
		providers = append(providers, p)
	}
	if len(providers) == 0 {
		return nil, fmt.Errorf("%w: set %s and/or %s", ErrNotConfigured, EnvPexelsKey, EnvPixabayKey)
	}
	return NewService(providers, opts...)
}
