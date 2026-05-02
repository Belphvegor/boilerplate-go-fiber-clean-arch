package http

import (
	"fmt"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"

	"github.com/casper/go-fiber-clean-arch/internal/bootstrap"
	"github.com/casper/go-fiber-clean-arch/internal/user/infrastructure/persistence"
	"github.com/casper/go-fiber-clean-arch/internal/user/repository"
	"github.com/casper/go-fiber-clean-arch/internal/user/usecase"
	"github.com/casper/go-fiber-clean-arch/pkg/middleware"
)

// Register wires user routes into the application.
func Register(app *fiber.App, container *bootstrap.Container) error {
	repo, err := buildRepository(container)
	if err != nil {
		return err
	}

	svc := usecase.NewService(repo, container.Validator, childLogger(container.Logger))
	handler := NewHandler(svc, container.Validator, childLogger(container.Logger))

	api := app.Group("/api/v1")
	if container.Config.Middleware.JWT {
		handler.Register(api, middleware.JWTProtected(container.Config.JWT))
	} else {
		handler.Register(api)
	}

	return nil
}

func buildRepository(container *bootstrap.Container) (repository.Repository, error) {
	driver := container.Config.Database.Driver
	switch driver {
	case "postgres", "mysql":
		if container.DB.SQL == nil {
			return nil, fmt.Errorf("user: sql connection not available for driver %s", driver)
		}
		return persistence.NewSQLRepository(container.DB.SQL), nil
	case "mongo":
		if container.DB.Mongo == nil {
			return nil, fmt.Errorf("user: mongo connection not available")
		}
		return persistence.NewMongoRepository(container.DB.Mongo), nil
	default:
		return nil, fmt.Errorf("user: unsupported driver %s", driver)
	}
}

func childLogger(base zerolog.Logger) zerolog.Logger {
	return base.With().Str("module", "user").Logger()
}
