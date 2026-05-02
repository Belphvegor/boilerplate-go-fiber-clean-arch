package http

import (
	"errors"
	"time"

	"github.com/gofiber/fiber/v2"
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
	logger       zerolog.Logger
}

// NewHandler creates a Handler.
func NewHandler(service *usecase.Service, cfg *config.AppConfig, logger zerolog.Logger) *Handler {
	return &Handler{
		service:      service,
		stateTTL:     cfg.OIDC.LoginStateTTL,
		secureCookie: cfg.Environment == "production",
		logger:       logger,
	}
}

// Register adds auth routes to the router.
func (h *Handler) Register(router fiber.Router) {
	router.Get("/auth/login", h.login)
	router.Get("/auth/callback", h.callback)
}

func (h *Handler) login(ctx *fiber.Ctx) error {
	session, err := h.service.BeginLogin()
	if err != nil {
		return response.Error(ctx, fiber.StatusInternalServerError, "could not start login", err.Error())
	}

	h.setCookie(ctx, stateCookie, session.State)
	h.setCookie(ctx, nonceCookie, session.Nonce)
	h.setCookie(ctx, verifierCookie, session.Verifier)

	return ctx.Redirect(session.AuthURL, fiber.StatusFound)
}

func (h *Handler) callback(ctx *fiber.Ctx) error {
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

	token, err := h.service.CompleteLogin(ctx.Context(), input)
	if err != nil {
		status := fiber.StatusInternalServerError
		switch {
		case errors.Is(err, usecase.ErrInvalidCallback), errors.Is(err, usecase.ErrInvalidNonce):
			status = fiber.StatusBadRequest
		case errors.Is(err, usecase.ErrEmailRequired), errors.Is(err, usecase.ErrEmailConflict):
			status = fiber.StatusUnauthorized
		}
		return response.Error(ctx, status, "could not complete login", err.Error())
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
}

func (h DisabledHandler) disabled(ctx *fiber.Ctx) error {
	return response.Error(ctx, fiber.StatusServiceUnavailable, "oidc authentication is disabled", nil)
}
