package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/casper/go-fiber-clean-arch/internal/user/entity"
)

var (
	// ErrNotFound indicates the user could not be located.
	ErrNotFound = errors.New("user repository: not found")
	// ErrEmailExists indicates the email already exists in the persistence layer.
	ErrEmailExists = errors.New("user repository: email already exists")
)

// Repository defines the persistence behaviour required by the User use case.
type Repository interface {
	Create(ctx context.Context, user *entity.User) error
	Update(ctx context.Context, user *entity.User) error
	FindByID(ctx context.Context, id uuid.UUID) (*entity.User, error)
	FindByEmail(ctx context.Context, email string) (*entity.User, error)
	Delete(ctx context.Context, id uuid.UUID) error
}
