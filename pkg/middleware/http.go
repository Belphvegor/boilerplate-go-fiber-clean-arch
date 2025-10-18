package middleware

import (
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	fiberLogger "github.com/gofiber/fiber/v2/middleware/logger"
	fiberRecover "github.com/gofiber/fiber/v2/middleware/recover"
	jwtware "github.com/gofiber/jwt/v2"
	"github.com/rs/zerolog"

	"github.com/casper/go-fiber-clean-arch/config"
)

// Register wires default middleware for the Fiber application.
func Register(app *fiber.App, cfg *config.AppConfig, log zerolog.Logger) {
	app.Use(func(ctx *fiber.Ctx) error {
		ctx.Locals("logger", log)
		return ctx.Next()
	})

	if cfg.Middleware.Recovery {
		app.Use(fiberRecover.New())
	}

	if cfg.Middleware.RequestLogger {
		app.Use(fiberLogger.New(fiberLogger.Config{
			TimeFormat: time.RFC3339,
			Output:     os.Stdout,
			Format:     "[${time}] ${ip} ${method} ${path} -> ${status} (${latency})\n",
		}))
	}

	if cfg.Middleware.CORS {
		app.Use(cors.New())
	}

	if cfg.Middleware.JWT {
		app.Use(jwtware.New(jwtware.Config{
			SigningKey:   []byte(cfg.JWT.Secret),
			ContextKey:   "user",
			ErrorHandler: jwtErrorHandler,
		}))
	}
}

func jwtErrorHandler(ctx *fiber.Ctx, err error) error {
	return ctx.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
		"error": fiber.Map{
			"message": "unauthorized",
			"detail":  err.Error(),
		},
	})
}
