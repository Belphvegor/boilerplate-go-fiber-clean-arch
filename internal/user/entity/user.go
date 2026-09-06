package entity

import (
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	// ErrInvalidName indicates the provided name fails validation.
	ErrInvalidName = errors.New("user: name must be between 2 and 100 characters")
	// ErrInvalidEmail indicates the email is malformed.
	ErrInvalidEmail = errors.New("user: email must be a valid address")

	emailRegex = regexp.MustCompile(`(?i)^[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}$`)
)

// User represents the core domain entity.
type User struct {
	ID        uuid.UUID
	Name      string
	Email     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// New creates a new user with generated identifiers and timestamps.
func New(name, email string) (*User, error) {
	name = strings.TrimSpace(name)
	email = strings.TrimSpace(strings.ToLower(email))

	if err := validateName(name); err != nil {
		return nil, err
	}
	if err := validateEmail(email); err != nil {
		return nil, err
	}
	now := time.Now().UTC()

	return &User{
		ID:        uuid.New(),
		Name:      name,
		Email:     email,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

func validateName(name string) error {
	if len(name) < 2 || len(name) > 100 {
		return ErrInvalidName
	}
	return nil
}

func validateEmail(email string) error {
	if email == "" || !emailRegex.MatchString(email) {
		return ErrInvalidEmail
	}
	return nil
}
