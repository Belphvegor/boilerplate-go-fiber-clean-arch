package response

import (
	"github.com/gofiber/fiber/v2"
)

// Envelope provides a consistent shape for API responses.
type Envelope map[string]interface{}

// Success writes a standard success envelope to the response.
func Success(ctx *fiber.Ctx, status int, data interface{}) error {
	payload := Envelope{
		"data": data,
	}
	return ctx.Status(status).JSON(payload)
}

// Error writes a standard error envelope with optional detail context.
func Error(ctx *fiber.Ctx, status int, message string, detail interface{}) error {
	payload := Envelope{
		"error": Envelope{
			"message": message,
			"detail":  detail,
		},
	}
	return ctx.Status(status).JSON(payload)
}

// Paginated builds a success envelope for paginated resources.
func Paginated(ctx *fiber.Ctx, status int, data interface{}, meta Envelope) error {
	payload := Envelope{
		"data": data,
		"meta": meta,
	}
	return ctx.Status(status).JSON(payload)
}
