package integration_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"

	"github.com/casper/go-fiber-clean-arch/config"
	"github.com/casper/go-fiber-clean-arch/internal/bootstrap"
	userhttp "github.com/casper/go-fiber-clean-arch/internal/user/delivery/http"
	userentity "github.com/casper/go-fiber-clean-arch/internal/user/entity"
	userpersistence "github.com/casper/go-fiber-clean-arch/internal/user/infrastructure/persistence"
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
	require.NotNil(t, container.DB.SQL, "TestUserWorkflow requires a SQL database")

	resp, err := app.Test(httptest.NewRequest(http.MethodPost, "/api/v1/users", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusNotFound, resp.StatusCode)

	user, err := userentity.New("Integration User", "integration-"+time.Now().Format("20060102150405.000000000")+"@example.com")
	require.NoError(t, err)
	require.NoError(t, userpersistence.NewSQLRepository(container.DB.SQL).Create(ctx, user))
	t.Cleanup(func() {
		query := container.DB.SQL.Rebind("DELETE FROM users WHERE id = ?")
		_, _ = container.DB.SQL.ExecContext(context.Background(), query, user.ID)
	})

	reqGet := httptest.NewRequest(http.MethodGet, "/api/v1/users/"+user.ID.String(), nil)

	respGet, err := app.Test(reqGet)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, respGet.StatusCode)

	var getResp struct {
		Data userhttp.UserResponse `json:"data"`
	}
	require.NoError(t, json.NewDecoder(respGet.Body).Decode(&getResp))
	require.Equal(t, user.Email, getResp.Data.Email)
}
