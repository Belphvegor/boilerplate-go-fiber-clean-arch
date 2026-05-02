package http

import (
	"context"
	"fmt"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"

	"github.com/casper/go-fiber-clean-arch/internal/auth/infrastructure/oidc"
	"github.com/casper/go-fiber-clean-arch/internal/auth/infrastructure/persistence"
	authrepo "github.com/casper/go-fiber-clean-arch/internal/auth/repository"
	"github.com/casper/go-fiber-clean-arch/internal/auth/usecase"
	"github.com/casper/go-fiber-clean-arch/internal/bootstrap"
	userpersistence "github.com/casper/go-fiber-clean-arch/internal/user/infrastructure/persistence"
	userrepo "github.com/casper/go-fiber-clean-arch/internal/user/repository"
)

// Register wires auth routes into the application.
func Register(app *fiber.App, container *bootstrap.Container) error {
	api := app.Group("/api/v1")
	if !container.Config.OIDC.Enabled {
		DisabledHandler{}.Register(api)
		return nil
	}

	authRepo, err := buildAuthRepository(container)
	if err != nil {
		return err
	}
	userRepo, err := buildUserRepository(container)
	if err != nil {
		return err
	}
	provider, err := oidc.NewProvider(context.Background(), container.Config.OIDC)
	if err != nil {
		return err
	}
	tokens, err := usecase.NewTokenService(container.Config.JWT)
	if err != nil {
		return err
	}

	svc := usecase.NewService(authRepo, userRepo, provider, tokens, childLogger(container.Logger))
	NewHandler(svc, container.Config, childLogger(container.Logger)).Register(api)
	return nil
}

func buildAuthRepository(container *bootstrap.Container) (authrepo.Repository, error) {
	driver := container.Config.Database.Driver
	switch driver {
	case "postgres", "mysql":
		if container.DB.SQL == nil {
			return nil, fmt.Errorf("auth: sql connection not available for driver %s", driver)
		}
		return persistence.NewSQLRepository(container.DB.SQL), nil
	case "mongo":
		if container.DB.Mongo == nil {
			return nil, fmt.Errorf("auth: mongo connection not available")
		}
		return persistence.NewMongoRepository(container.DB.Mongo), nil
	default:
		return nil, fmt.Errorf("auth: unsupported driver %s", driver)
	}
}

func buildUserRepository(container *bootstrap.Container) (userrepo.Repository, error) {
	driver := container.Config.Database.Driver
	switch driver {
	case "postgres", "mysql":
		if container.DB.SQL == nil {
			return nil, fmt.Errorf("user: sql connection not available for driver %s", driver)
		}
		return userpersistence.NewSQLRepository(container.DB.SQL), nil
	case "mongo":
		if container.DB.Mongo == nil {
			return nil, fmt.Errorf("user: mongo connection not available")
		}
		return userpersistence.NewMongoRepository(container.DB.Mongo), nil
	default:
		return nil, fmt.Errorf("user: unsupported driver %s", driver)
	}
}

func childLogger(base zerolog.Logger) zerolog.Logger {
	return base.With().Str("module", "auth").Logger()
}
