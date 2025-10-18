package http

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/casper/go-fiber-clean-arch/internal/shared/validator"
	"github.com/casper/go-fiber-clean-arch/internal/user/entity"
	"github.com/casper/go-fiber-clean-arch/internal/user/usecase"
	"github.com/casper/go-fiber-clean-arch/pkg/response"
)

// Handler exposes HTTP endpoints for the User domain.
type Handler struct {
	service   *usecase.Service
	validator *validator.Adapter
	logger    zerolog.Logger
}

// NewHandler creates a Handler.
func NewHandler(service *usecase.Service, validator *validator.Adapter, logger zerolog.Logger) *Handler {
	return &Handler{
		service:   service,
		validator: validator,
		logger:    logger,
	}
}

// Register adds user routes to the router.
func (h *Handler) Register(router fiber.Router) {
	router.Post("/users", h.createUser)
	router.Get("/users/:id", h.getUser)
}

func (h *Handler) createUser(ctx *fiber.Ctx) error {
	var req CreateUserRequest
	if err := ctx.BodyParser(&req); err != nil {
		return response.Error(ctx, fiber.StatusBadRequest, "invalid payload", err.Error())
	}

	if err := h.validator.Struct(req); err != nil {
		return response.Error(ctx, fiber.StatusUnprocessableEntity, "validation failed", err)
	}

	user, err := h.service.Register(ctx.Context(), usecase.CreateInput{
		Name:     req.Name,
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		status := fiber.StatusInternalServerError
		var validationErr *validator.Error
		switch {
		case errors.Is(err, usecase.ErrEmailExists):
			status = fiber.StatusConflict
		case errors.As(err, &validationErr):
			status = fiber.StatusUnprocessableEntity
		}
		return response.Error(ctx, status, "could not create user", err.Error())
	}

	return response.Success(ctx, fiber.StatusCreated, toResponse(user))
}

func (h *Handler) getUser(ctx *fiber.Ctx) error {
	idParam := ctx.Params("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		return response.Error(ctx, fiber.StatusBadRequest, "invalid user id", err.Error())
	}

	user, err := h.service.Get(ctx.Context(), id)
	if err != nil {
		status := fiber.StatusInternalServerError
		if errors.Is(err, usecase.ErrNotFound) {
			status = fiber.StatusNotFound
		}
		return response.Error(ctx, status, "could not fetch user", err.Error())
	}

	return response.Success(ctx, fiber.StatusOK, toResponse(user))
}

func toResponse(u *entity.User) *UserResponse {
	return &UserResponse{
		ID:    u.ID.String(),
		Name:  u.Name,
		Email: u.Email,
	}
}
