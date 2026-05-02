package main

import (
	"context"
	// "errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	// "github.com/valyala/fasthttp"

	"github.com/casper/go-fiber-clean-arch/config"
	authhttp "github.com/casper/go-fiber-clean-arch/internal/auth/delivery/http"
	"github.com/casper/go-fiber-clean-arch/internal/bootstrap"
	userhttp "github.com/casper/go-fiber-clean-arch/internal/user/delivery/http"
	"github.com/casper/go-fiber-clean-arch/pkg/middleware"
	"github.com/casper/go-fiber-clean-arch/pkg/response"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load configuration: %v", err)
	}

	container, err := bootstrap.Build(ctx, cfg)
	if err != nil {
		log.Fatalf("bootstrap container: %v", err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := container.Shutdown(shutdownCtx); err != nil {
			container.Logger.Error().Err(err).Msg("shutdown container")
		}
	}()

	app := fiber.New(fiber.Config{
		AppName:      cfg.Name,
		ErrorHandler: errorHandler(container),
	})

	middleware.Register(app, cfg, container.Logger)

	if err := container.RegisterRoutes(app, authhttp.Register, userhttp.Register); err != nil {
		container.Logger.Fatal().Err(err).Msg("register routes")
	}

	// Route registration deferred to modules; placeholder health endpoint
	app.Get("/healthz", func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"status": "ok",
		})
	})

	if !cfg.Middleware.JWT {
		container.Logger.Warn().Msg("JWT route protection disabled; protected endpoints are public")
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := app.ShutdownWithContext(shutdownCtx); err != nil {
			container.Logger.Error().Err(err).Msg("shutdown server")
		}
	}()

	addr := fmt.Sprintf(":%s", cfg.HTTP.Port)
	container.Logger.Info().Str("address", addr).Msg("starting http server")
	if err := app.Listen(addr); err != nil {
		container.Logger.Fatal().Err(err).Msg("http server crashed")
	}
}

func errorHandler(container *bootstrap.Container) fiber.ErrorHandler {
	return func(ctx *fiber.Ctx, err error) error {
		var (
			status = fiber.StatusInternalServerError
			msg    = "internal server error"
		)

		if fe, ok := err.(*fiber.Error); ok {
			status = fe.Code
			msg = fe.Message
		}

		container.Logger.Error().
			Err(err).
			Str("path", ctx.Path()).
			Str("method", ctx.Method()).
			Msg("request error")

		return response.Error(ctx, status, msg, nil)
	}
}
