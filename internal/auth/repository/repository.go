package repository

import (
	"context"
	"errors"

	"github.com/casper/go-fiber-clean-arch/internal/auth/entity"
)

var (
	ErrIdentityNotFound = errors.New("auth repository: identity not found")
	ErrIdentityExists   = errors.New("auth repository: identity already exists")
)

// Repository defines persistence for external auth identities.
type Repository interface {
	CreateIdentity(ctx context.Context, identity *entity.Identity) error
	FindIdentity(ctx context.Context, provider, subject string) (*entity.Identity, error)
}
