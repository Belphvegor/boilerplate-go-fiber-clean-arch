package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/casper/go-fiber-clean-arch/internal/user/entity"
	"github.com/casper/go-fiber-clean-arch/internal/user/repository"
)

var (
	// ErrNotFound is returned when a user cannot be located.
	ErrNotFound = repository.ErrNotFound
)

// Service orchestrates user domain actions.
type Service struct {
	repo repository.Repository
}

// NewService constructs a Service.
func NewService(repo repository.Repository) *Service {
	return &Service{repo: repo}
}

// Get retrieves a user by identifier.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*entity.User, error) {
	user, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return user, nil
}
