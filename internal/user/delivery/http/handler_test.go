package http

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestHandlerDoesNotRegisterPasswordSignup(t *testing.T) {
	app := fiber.New()
	NewHandler(nil).Register(app.Group("/api/v1"))

	resp, err := app.Test(httptest.NewRequest(http.MethodPost, "/api/v1/users", nil))

	require.NoError(t, err)
	require.Equal(t, fiber.StatusNotFound, resp.StatusCode)
}

func TestOwnerOnly(t *testing.T) {
	ownerID := uuid.New()
	tests := []struct {
		name       string
		actor      interface{}
		target     string
		wantStatus int
		wantNext   bool
	}{
		{name: "owner", actor: ownerID.String(), target: strings.ToUpper(ownerID.String()), wantStatus: fiber.StatusNoContent, wantNext: true},
		{name: "different user", actor: ownerID.String(), target: uuid.NewString(), wantStatus: fiber.StatusForbidden},
		{name: "missing identity", target: ownerID.String(), wantStatus: fiber.StatusForbidden},
		{name: "invalid target", actor: ownerID.String(), target: "not-a-uuid", wantStatus: fiber.StatusBadRequest, wantNext: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			app := fiber.New()
			app.Get("/users/:id", func(ctx *fiber.Ctx) error {
				if tt.actor != nil {
					ctx.Locals("auth.user_id", tt.actor)
				}
				return ctx.Next()
			}, ownerOnly, func(ctx *fiber.Ctx) error {
				called = true
				if _, err := uuid.Parse(ctx.Params("id")); err != nil {
					return ctx.SendStatus(fiber.StatusBadRequest)
				}
				return ctx.SendStatus(fiber.StatusNoContent)
			})

			resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/users/"+tt.target, nil))

			require.NoError(t, err)
			require.Equal(t, tt.wantStatus, resp.StatusCode)
			require.Equal(t, tt.wantNext, called)
			if tt.wantStatus == fiber.StatusForbidden {
				body, err := io.ReadAll(resp.Body)
				require.NoError(t, err)
				require.JSONEq(t, `{"error":{"message":"forbidden","detail":null}}`, string(body))
			}
		})
	}
}
