package bootstrap

import (
	"context"
	"fmt"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"

	"github.com/casper/go-fiber-clean-arch/config"
	"github.com/casper/go-fiber-clean-arch/internal/shared/validator"
	"github.com/casper/go-fiber-clean-arch/pkg/database"
	"github.com/casper/go-fiber-clean-arch/pkg/logger"
)

// Container aggregates shared dependencies for application modules.
type Container struct {
	Config    *config.AppConfig
	Logger    zerolog.Logger
	DB        *database.Connection
	Validator *validator.Adapter
}

// RouteRegistrar registers HTTP routes using shared dependencies.
type RouteRegistrar func(*fiber.App, *Container) error

// Build constructs the dependency container and bootstraps shared services.
func Build(ctx context.Context, cfg *config.AppConfig) (*Container, error) {
	log := logger.New(logger.Config{
		ServiceName: cfg.Name,
		Level:       cfg.Logging.Level,
		Pretty:      cfg.Logging.Pretty,
	})

	dbFactory := database.NewFactory(cfg)
	conn, err := dbFactory.Open(ctx)
	if err != nil {
		return nil, fmt.Errorf("bootstrap database: %w", err)
	}

	validate := validator.New()

	return &Container{
		Config:    cfg,
		Logger:    log,
		DB:        conn,
		Validator: validate,
	}, nil
}

// RegisterRoutes wires module-specific routes into the provided Fiber app.
func (c *Container) RegisterRoutes(app *fiber.App, registrars ...RouteRegistrar) error {
	for _, register := range registrars {
		if err := register(app, c); err != nil {
			return err
		}
	}
	return nil
}

// Shutdown releases resources gracefully.
func (c *Container) Shutdown(ctx context.Context) error {
	if c.DB != nil {
		if err := c.DB.Close(ctx); err != nil {
			return err
		}
	}
	return nil
}
