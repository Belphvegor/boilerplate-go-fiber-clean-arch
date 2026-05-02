package middleware

import (
	"errors"
	"os"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	fiberLogger "github.com/gofiber/fiber/v2/middleware/logger"
	fiberRecover "github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/golang-jwt/jwt/v5"
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
}

// JWTProtected validates local API bearer tokens for protected routes.
func JWTProtected(cfg config.JWTConfig) fiber.Handler {
	return func(ctx *fiber.Ctx) error {
		tokenString, err := bearerToken(ctx.Get(fiber.HeaderAuthorization))
		if err != nil {
			return jwtErrorHandler(ctx, err)
		}

		claims := jwt.MapClaims{}
		token, err := jwt.ParseWithClaims(
			tokenString,
			claims,
			func(token *jwt.Token) (interface{}, error) {
				return []byte(cfg.Secret), nil
			},
			jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
			jwt.WithIssuer(cfg.Issuer),
			jwt.WithAudience(cfg.Audience),
			jwt.WithExpirationRequired(),
		)
		if err != nil || token == nil || !token.Valid {
			if err == nil {
				err = errors.New("invalid token")
			}
			return jwtErrorHandler(ctx, err)
		}

		ctx.Locals("auth.claims", claims)
		ctx.Locals("auth.user_id", claims["sub"])
		return ctx.Next()
	}
}

func bearerToken(header string) (string, error) {
	if header == "" {
		return "", errors.New("missing authorization header")
	}
	parts := strings.Fields(header)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return "", errors.New("invalid authorization header")
	}
	return parts[1], nil
}

func jwtErrorHandler(ctx *fiber.Ctx, err error) error {
	return ctx.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
		"error": fiber.Map{
			"message": "unauthorized",
			"detail":  err.Error(),
		},
	})
}
