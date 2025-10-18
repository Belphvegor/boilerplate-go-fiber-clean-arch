package validator

import (
	"fmt"

	playground "github.com/go-playground/validator/v10"
)

// Adapter wraps go-playground/validator to expose helper methods.
type Adapter struct {
	validate *playground.Validate
}

// New creates a validator adapter with sensible defaults.
func New() *Adapter {
	v := playground.New()
	return &Adapter{validate: v}
}

// Struct validates a struct and returns a normalized error value.
func (a *Adapter) Struct(s interface{}) error {
	if err := a.validate.Struct(s); err != nil {
		if _, ok := err.(*playground.InvalidValidationError); ok {
			return err
		}
		return normalizeErrors(err.(playground.ValidationErrors))
	}
	return nil
}

// RegisterValidation proxies custom rule registration.
func (a *Adapter) RegisterValidation(tag string, fn playground.Func) error {
	return a.validate.RegisterValidation(tag, fn)
}

func normalizeErrors(errs playground.ValidationErrors) error {
	out := make(map[string]string)
	for _, err := range errs {
		field := err.Field()
		out[field] = fmt.Sprintf("validation failed on '%s' constraint", err.Tag())
	}
	return &Error{Fields: out}
}

// Error represents normalized validation errors.
type Error struct {
	Fields map[string]string
}

func (e *Error) Error() string {
	return "validation failed"
}
