package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestValidate_RequiresOIDCSettingsWhenEnabled(t *testing.T) {
	cfg := validConfig()
	cfg.OIDC.Enabled = true
	cfg.OIDC.IssuerURL = ""

	err := cfg.Validate()

	assert.ErrorContains(t, err, "APP_OIDC_ISSUER_URL")
}

func TestValidate_RejectsWeakProductionJWTSecret(t *testing.T) {
	cfg := validConfig()
	cfg.Environment = "production"
	cfg.JWT.Secret = "change-me"

	err := cfg.Validate()

	assert.ErrorContains(t, err, "APP_JWT_SECRET")
}

func TestValidate_RejectsRefreshTTLShorterThanAccessTTL(t *testing.T) {
	cfg := validConfig()
	cfg.JWT.AccessTTL = time.Hour
	cfg.JWT.RefreshTTL = time.Minute

	err := cfg.Validate()

	assert.ErrorContains(t, err, "APP_JWT_REFRESH_TTL")
}

func TestValidate_RequiresPositiveAuthRateLimit(t *testing.T) {
	cfg := validConfig()
	cfg.Middleware.AuthRateLimit = 0

	err := cfg.Validate()

	assert.ErrorContains(t, err, "APP_AUTH_RATE_LIMIT")
}

func validConfig() *AppConfig {
	return &AppConfig{
		Name:        "clean-arch-starter",
		Environment: "development",
		HTTP: HTTPConfig{
			Port:         "8080",
			ReadTimeout:  time.Second,
			WriteTimeout: time.Second,
		},
		Database: DatabaseConfig{
			Driver: "postgres",
			DSN:    "postgres://app:app@localhost:5432/app?sslmode=disable",
		},
		JWT: JWTConfig{
			Secret:     "test-secret-that-is-long-enough",
			Issuer:     "clean-arch-starter",
			Audience:   "clean-arch-starter-api",
			AccessTTL:  time.Minute,
			RefreshTTL: time.Hour,
		},
		Middleware: MiddlewareConfig{AuthRateLimit: 60},
		OIDC: OIDCConfig{
			Enabled:       false,
			IssuerURL:     "https://issuer.example.com",
			ClientID:      "client",
			ClientSecret:  "secret",
			RedirectURL:   "http://localhost:8080/api/v1/auth/callback",
			Scopes:        []string{"openid", "profile", "email"},
			LoginStateTTL: time.Minute,
		},
	}
}
