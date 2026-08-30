package http

import (
	"errors"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/rs/zerolog"

	"github.com/casper/go-fiber-clean-arch/config"
	"github.com/casper/go-fiber-clean-arch/internal/auth/usecase"
	"github.com/casper/go-fiber-clean-arch/pkg/response"
)

const (
	stateCookie    = "oidc_state"
	nonceCookie    = "oidc_nonce"
	verifierCookie = "oidc_verifier"
	cookiePath     = "/api/v1/auth"
)

// Handler exposes HTTP endpoints for OAuth2/OIDC login.
type Handler struct {
	service      *usecase.Service
	stateTTL     time.Duration
	secureCookie bool
	rateLimit    int
	logger       zerolog.Logger
}

// NewHandler creates a Handler.
func NewHandler(service *usecase.Service, cfg *config.AppConfig, logger zerolog.Logger) *Handler {
	return &Handler{
		service:      service,
		stateTTL:     cfg.OIDC.LoginStateTTL,
		secureCookie: cfg.Environment == "production",
		rateLimit:    cfg.Middleware.AuthRateLimit,
		logger:       logger,
	}
}

// Register adds auth routes to the router.
func (h *Handler) Register(router fiber.Router) {
	// Per-IP budget shared by all four auth endpoints, to blunt code-guessing and
	// refresh-token brute force without punishing clients behind a shared address.
	throttle := limiter.New(limiter.Config{Max: h.rateLimit, Expiration: time.Minute})
	router.Get("/auth/login", throttle, h.login)
	router.Get("/auth/callback", throttle, h.callback)
	router.Post("/auth/refresh", throttle, h.refresh)
	router.Post("/auth/logout", throttle, h.logout)
}

// refreshRequest carries the refresh token for the refresh and logout endpoints.
type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func (h *Handler) refresh(ctx *fiber.Ctx) error {
	var body refreshRequest
	if err := ctx.BodyParser(&body); err != nil {
		return response.Error(ctx, fiber.StatusBadRequest, "invalid request body", nil)
	}

	token, err := h.service.Refresh(ctx.UserContext(), body.RefreshToken)
	if err != nil {
		return h.refreshError(ctx, err, "refresh failed")
	}

	return response.Success(ctx, fiber.StatusOK, token)
}

func (h *Handler) logout(ctx *fiber.Ctx) error {
	var body refreshRequest
	if err := ctx.BodyParser(&body); err != nil {
		return response.Error(ctx, fiber.StatusBadRequest, "invalid request body", nil)
	}

	logoutURL, err := h.service.Logout(ctx.UserContext(), body.RefreshToken)
	if err != nil {
		return h.refreshError(ctx, err, "logout failed")
	}

	// The client must navigate to logout_url; otherwise the provider's SSO cookie
	// survives and the next login silently re-authenticates the same user.
	return response.Success(ctx, fiber.StatusOK, fiber.Map{"logout_url": logoutURL})
}

func (h *Handler) refreshError(ctx *fiber.Ctx, err error, msg string) error {
	status := fiber.StatusInternalServerError
	switch {
	case errors.Is(err, usecase.ErrInvalidRefresh), errors.Is(err, usecase.ErrRefreshReused):
		// Replay is reported the same as any other invalid token. Telling the caller
		// its token was "already used" would confirm to a thief that they hold a real
		// one; the revocation and the warning log are where that event is handled.
		status = fiber.StatusUnauthorized
	}
	h.logger.Warn().Err(err).Int("status", status).Msg(msg)
	return response.Error(ctx, status, msg, nil)
}

func (h *Handler) login(ctx *fiber.Ctx) error {
	session, err := h.service.BeginLogin()
	if err != nil {
		h.logger.Error().Err(err).Msg("begin oidc login")
		return response.Error(ctx, fiber.StatusInternalServerError, "could not start login", nil)
	}

	h.setCookie(ctx, stateCookie, session.State)
	h.setCookie(ctx, nonceCookie, session.Nonce)
	h.setCookie(ctx, verifierCookie, session.Verifier)

	return ctx.Redirect(session.AuthURL, fiber.StatusFound)
}

func (h *Handler) callback(ctx *fiber.Ctx) error {
	if providerErr := ctx.Query("error"); providerErr != "" {
		h.clearCookie(ctx, stateCookie)
		h.clearCookie(ctx, nonceCookie)
		h.clearCookie(ctx, verifierCookie)
		h.logger.Warn().
			Str("provider_error", providerErr).
			Str("provider_error_description", ctx.Query("error_description")).
			Msg("oidc provider rejected login")
		// Not echoed back: the value is attacker-controllable query input.
		return response.Error(ctx, fiber.StatusUnauthorized, "login was not completed", nil)
	}

	input := usecase.CallbackInput{
		Code:          ctx.Query("code"),
		State:         ctx.Query("state"),
		ExpectedState: ctx.Cookies(stateCookie),
		ExpectedNonce: ctx.Cookies(nonceCookie),
		Verifier:      ctx.Cookies(verifierCookie),
	}
	h.clearCookie(ctx, stateCookie)
	h.clearCookie(ctx, nonceCookie)
	h.clearCookie(ctx, verifierCookie)

	token, err := h.service.CompleteLogin(ctx.UserContext(), input)
	if err != nil {
		status := fiber.StatusInternalServerError
		switch {
		case errors.Is(err, usecase.ErrInvalidCallback), errors.Is(err, usecase.ErrInvalidNonce):
			status = fiber.StatusBadRequest
		case errors.Is(err, usecase.ErrEmailRequired), errors.Is(err, usecase.ErrEmailConflict):
			status = fiber.StatusUnauthorized
		}
		// Detail stays in the log: provider exchange errors quote upstream response bodies.
		h.logger.Warn().Err(err).Int("status", status).Msg("oidc callback failed")
		return response.Error(ctx, status, "could not complete login", nil)
	}

	return response.Success(ctx, fiber.StatusOK, token)
}

func (h *Handler) setCookie(ctx *fiber.Ctx, name, value string) {
	ctx.Cookie(&fiber.Cookie{
		Name:     name,
		Value:    value,
		Path:     cookiePath,
		HTTPOnly: true,
		Secure:   h.secureCookie,
		SameSite: "Lax",
		MaxAge:   int(h.stateTTL.Seconds()),
	})
}

func (h *Handler) clearCookie(ctx *fiber.Ctx, name string) {
	ctx.Cookie(&fiber.Cookie{
		Name:     name,
		Value:    "",
		Path:     cookiePath,
		HTTPOnly: true,
		Secure:   h.secureCookie,
		SameSite: "Lax",
		MaxAge:   -1,
	})
}

// DisabledHandler keeps auth endpoint responses explicit when OIDC is disabled.
type DisabledHandler struct{}

// Register adds disabled auth routes to the router.
func (h DisabledHandler) Register(router fiber.Router) {
	router.Get("/auth/login", h.disabled)
	router.Get("/auth/callback", h.disabled)
	router.Post("/auth/refresh", h.disabled)
	router.Post("/auth/logout", h.disabled)
}

func (h DisabledHandler) disabled(ctx *fiber.Ctx) error {
	return response.Error(ctx, fiber.StatusServiceUnavailable, "oidc authentication is disabled", nil)
}
