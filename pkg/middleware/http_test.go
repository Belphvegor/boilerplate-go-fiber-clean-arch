package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"

	"github.com/casper/go-fiber-clean-arch/config"
	authentity "github.com/casper/go-fiber-clean-arch/internal/auth/entity"
	"github.com/casper/go-fiber-clean-arch/internal/auth/usecase"
	userentity "github.com/casper/go-fiber-clean-arch/internal/user/entity"
)

func TestJWTProtected(t *testing.T) {
	cfg := config.JWTConfig{
		Secret:     "test-secret-that-is-long-enough",
		Issuer:     "clean-arch-starter",
		Audience:   "clean-arch-starter-api",
		AccessTTL:  time.Minute,
		RefreshTTL: time.Hour,
	}
	tokens, err := usecase.NewTokenService(cfg)
	require.NoError(t, err)
	user, err := userentity.New("Ada Lovelace", "ada@example.com")
	require.NoError(t, err)
	identity, err := authentity.NewIdentity(user.ID, "https://issuer.example.com", "provider-subject", user.Email)
	require.NoError(t, err)
	accessToken, err := tokens.Issue(user, identity)
	require.NoError(t, err)

	app := fiber.New()
	app.Get("/protected", JWTProtected(cfg), func(ctx *fiber.Ctx) error {
		require.Equal(t, user.ID.String(), ctx.Locals("auth.user_id"))
		return ctx.SendStatus(fiber.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set(fiber.HeaderAuthorization, "Bearer "+accessToken.Value)

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusNoContent, resp.StatusCode)
}

func TestJWTProtected_RejectsMissingBearer(t *testing.T) {
	app := fiber.New()
	app.Get("/protected", JWTProtected(config.JWTConfig{
		Secret:    "test-secret-that-is-long-enough",
		Issuer:    "clean-arch-starter",
		Audience:  "clean-arch-starter-api",
		AccessTTL: time.Minute,
	}), func(ctx *fiber.Ctx) error {
		return ctx.SendStatus(fiber.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	resp, err := app.Test(req)

	require.NoError(t, err)
	require.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestJWTProtected_RejectsInvalidSubject(t *testing.T) {
	cfg := config.JWTConfig{
		Secret:   "test-secret-that-is-long-enough",
		Issuer:   "clean-arch-starter",
		Audience: "clean-arch-starter-api",
	}

	tests := []struct {
		name       string
		subject    interface{}
		setSubject bool
	}{
		{name: "missing"},
		{name: "wrong type", subject: 42, setSubject: true},
		{name: "not a UUID", subject: "not-a-uuid", setSubject: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims := jwt.MapClaims{
				"iss": cfg.Issuer,
				"aud": cfg.Audience,
				"exp": time.Now().Add(time.Minute).Unix(),
			}
			if tt.setSubject {
				claims["sub"] = tt.subject
			}
			token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(cfg.Secret))
			require.NoError(t, err)

			app := fiber.New()
			app.Get("/protected", JWTProtected(cfg), func(ctx *fiber.Ctx) error {
				return ctx.SendStatus(fiber.StatusNoContent)
			})

			req := httptest.NewRequest(http.MethodGet, "/protected", nil)
			req.Header.Set(fiber.HeaderAuthorization, "Bearer "+token)
			resp, err := app.Test(req)

			require.NoError(t, err)
			require.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
		})
	}
}
