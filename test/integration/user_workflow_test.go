package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/casper/go-fiber-clean-arch/config"
	"github.com/casper/go-fiber-clean-arch/internal/bootstrap"
	userhttp "github.com/casper/go-fiber-clean-arch/internal/user/delivery/http"
	"github.com/casper/go-fiber-clean-arch/pkg/middleware"
)

func TestUserWorkflow(t *testing.T) {
	if os.Getenv("INTEGRATION") != "true" {
		t.Skip("integration test disabled; set INTEGRATION=true to enable")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cfg, err := config.Load()
	require.NoError(t, err)
	cfg.Middleware.JWT = false

	container, err := bootstrap.Build(ctx, cfg)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = container.Shutdown(context.Background())
	})

	app := fiber.New()
	middleware.Register(app, cfg, container.Logger)
	require.NoError(t, container.RegisterRoutes(app, userhttp.Register))

	payload := map[string]string{
		"name":     "Integration User",
		"email":    "integration@example.com",
		"password": "StrongPass123",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusCreated, resp.StatusCode)

	var createResp struct {
		Data userhttp.UserResponse `json:"data"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&createResp))
	require.NotEmpty(t, createResp.Data.ID)

	userID := createResp.Data.ID

	reqGet := httptest.NewRequest(http.MethodGet, "/api/v1/users/"+userID, nil)

	respGet, err := app.Test(reqGet)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, respGet.StatusCode)

	var getResp struct {
		Data userhttp.UserResponse `json:"data"`
	}
	require.NoError(t, json.NewDecoder(respGet.Body).Decode(&getResp))
	require.Equal(t, createResp.Data.Email, getResp.Data.Email)

	if container.DB.SQL != nil {
		rebind := container.DB.SQL.Rebind
		query := rebind("DELETE FROM users WHERE id = ?")
		_, _ = container.DB.SQL.ExecContext(context.Background(), query, uuid.MustParse(userID))
	}
}
