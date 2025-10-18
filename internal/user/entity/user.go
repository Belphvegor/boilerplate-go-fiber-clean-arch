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
	ID           uuid.UUID
	Name         string
	Email        string
	PasswordHash string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// New creates a new user with generated identifiers and timestamps.
func New(name, email, passwordHash string) (*User, error) {
	name = strings.TrimSpace(name)
	email = strings.TrimSpace(strings.ToLower(email))

	if err := validateName(name); err != nil {
		return nil, err
	}
	if err := validateEmail(email); err != nil {
		return nil, err
	}
	if passwordHash == "" {
		return nil, errors.New("user: password hash is required")
	}

	now := time.Now().UTC()

	return &User{
		ID:           uuid.New(),
		Name:         name,
		Email:        email,
		PasswordHash: passwordHash,
		CreatedAt:    now,
		UpdatedAt:    now,
	}, nil
}

// UpdateName updates the user's display name.
func (u *User) UpdateName(name string) error {
	name = strings.TrimSpace(name)
	if err := validateName(name); err != nil {
		return err
	}
	u.Name = name
	u.touch()
	return nil
}

// UpdateEmail updates the user's email address.
func (u *User) UpdateEmail(email string) error {
	email = strings.TrimSpace(strings.ToLower(email))
	if err := validateEmail(email); err != nil {
		return err
	}
	u.Email = email
	u.touch()
	return nil
}

func (u *User) touch() {
	u.UpdatedAt = time.Now().UTC()
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
