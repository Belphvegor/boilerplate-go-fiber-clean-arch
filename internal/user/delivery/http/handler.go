package http

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/casper/go-fiber-clean-arch/internal/user/entity"
	"github.com/casper/go-fiber-clean-arch/internal/user/usecase"
	"github.com/casper/go-fiber-clean-arch/pkg/response"
)

// Handler exposes HTTP endpoints for the User domain.
type Handler struct {
	service *usecase.Service
}

// NewHandler creates a Handler.
func NewHandler(service *usecase.Service) *Handler {
	return &Handler{service: service}
}

// Register adds user routes to the router.
func (h *Handler) Register(router fiber.Router, protected ...fiber.Handler) {
	getHandlers := append(protected, h.getUser)
	router.Get("/users/:id", getHandlers...)
}

func ownerOnly(ctx *fiber.Ctx) error {
	targetID, err := uuid.Parse(ctx.Params("id"))
	if err != nil {
		return ctx.Next()
	}
	actorID, ok := ctx.Locals("auth.user_id").(string)
	if !ok || actorID != targetID.String() {
		return response.Error(ctx, fiber.StatusForbidden, "forbidden", nil)
	}
	return ctx.Next()
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
