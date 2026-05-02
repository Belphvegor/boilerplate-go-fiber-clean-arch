package entity

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrInvalidProvider = errors.New("auth identity: provider is required")
	ErrInvalidSubject  = errors.New("auth identity: subject is required")
	ErrInvalidUserID   = errors.New("auth identity: user id is required")
)

// Identity links an external OIDC subject to a local user.
type Identity struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Provider  string
	Subject   string
	Email     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// NewIdentity creates a stable provider-subject mapping for a user.
func NewIdentity(userID uuid.UUID, provider, subject, email string) (*Identity, error) {
	provider = strings.TrimSpace(provider)
	subject = strings.TrimSpace(subject)
	email = strings.TrimSpace(strings.ToLower(email))

	if userID == uuid.Nil {
		return nil, ErrInvalidUserID
	}
	if provider == "" {
		return nil, ErrInvalidProvider
	}
	if subject == "" {
		return nil, ErrInvalidSubject
	}

	now := time.Now().UTC()
	return &Identity{
		ID:        uuid.New(),
		UserID:    userID,
		Provider:  provider,
		Subject:   subject,
		Email:     email,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}
