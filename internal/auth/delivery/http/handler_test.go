package http

import (
	"context"
	nethttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/casper/go-fiber-clean-arch/config"
	"github.com/casper/go-fiber-clean-arch/internal/auth/usecase"
)

func TestHandler_LoginRedirectsAndSetsTemporaryCookies(t *testing.T) {
	app := fiber.New()
	cfg := &config.AppConfig{
		Environment: "development",
		OIDC: config.OIDCConfig{
			LoginStateTTL: time.Minute,
		},
	}
	service := usecase.NewService(nil, nil, fakeProvider{}, mustTokenService(t), zerolog.Nop())
	NewHandler(service, cfg, zerolog.Nop()).Register(app.Group("/api/v1"))

	req := httptest.NewRequest(nethttp.MethodGet, "/api/v1/auth/login", nil)
	resp, err := app.Test(req)

	require.NoError(t, err)
	require.Equal(t, fiber.StatusFound, resp.StatusCode)
	require.Contains(t, resp.Header.Get("Location"), "https://issuer.example.com/auth?state=")
	requireSetCookie(t, resp, stateCookie)
	requireSetCookie(t, resp, nonceCookie)
	requireSetCookie(t, resp, verifierCookie)
}

func TestDisabledHandler(t *testing.T) {
	app := fiber.New()
	DisabledHandler{}.Register(app.Group("/api/v1"))

	req := httptest.NewRequest(nethttp.MethodGet, "/api/v1/auth/login", nil)
	resp, err := app.Test(req)

	require.NoError(t, err)
	require.Equal(t, fiber.StatusServiceUnavailable, resp.StatusCode)
}

func requireSetCookie(t *testing.T, resp *nethttp.Response, name string) {
	t.Helper()
	for _, value := range resp.Header.Values("Set-Cookie") {
		if strings.HasPrefix(value, name+"=") {
			require.Contains(t, value, "HttpOnly")
			return
		}
	}
	t.Fatalf("cookie %s was not set", name)
}

type fakeProvider struct{}

func (p fakeProvider) AuthCodeURL(state, nonce, verifier string) string {
	return "https://issuer.example.com/auth?state=" + state + "&nonce=" + nonce + "&verifier=" + verifier
}

func (p fakeProvider) Exchange(ctx context.Context, code, verifier string) (*usecase.ProviderToken, error) {
	return nil, nil
}

func mustTokenService(t *testing.T) *usecase.TokenService {
	t.Helper()
	tokens, err := usecase.NewTokenService(config.JWTConfig{
		Secret:    "test-secret-that-is-long-enough",
		Issuer:    "clean-arch-starter",
		Audience:  "clean-arch-starter-api",
		AccessTTL: time.Minute,
	})
	require.NoError(t, err)
	return tokens
}
