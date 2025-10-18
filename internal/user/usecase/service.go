package usecase

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"golang.org/x/crypto/bcrypt"

	"github.com/casper/go-fiber-clean-arch/internal/shared/validator"
	"github.com/casper/go-fiber-clean-arch/internal/user/entity"
	"github.com/casper/go-fiber-clean-arch/internal/user/repository"
)

var (
	// ErrEmailExists is returned when attempting to register with a duplicate email.
	ErrEmailExists = repository.ErrEmailExists
	// ErrNotFound is returned when a user cannot be located.
	ErrNotFound = repository.ErrNotFound
)

// Service orchestrates user domain actions.
type Service struct {
	repo      repository.Repository
	validator *validator.Adapter
	logger    zerolog.Logger
}

// NewService constructs a Service.
func NewService(repo repository.Repository, validator *validator.Adapter, logger zerolog.Logger) *Service {
	return &Service{
		repo:      repo,
		validator: validator,
		logger:    logger,
	}
}

// CreateInput contains data required to register a user.
type CreateInput struct {
	Name     string `validate:"required,min=2,max=100"`
	Email    string `validate:"required,email"`
	Password string `validate:"required,min=8"`
}

// Register creates a new user ensuring email uniqueness.
func (s *Service) Register(ctx context.Context, input CreateInput) (*entity.User, error) {
	if err := s.validator.Struct(input); err != nil {
		return nil, err
	}

	existing, err := s.repo.FindByEmail(ctx, input.Email)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return nil, err
	}
	if existing != nil {
		return nil, repository.ErrEmailExists
	}

	hash, err := hashPassword(input.Password)
	if err != nil {
		return nil, err
	}

	user, err := entity.New(input.Name, input.Email, hash)
	if err != nil {
		return nil, err
	}

	if err := s.repo.Create(ctx, user); err != nil {
		return nil, err
	}

	s.logger.Info().
		Str("user_id", user.ID.String()).
		Str("email", user.Email).
		Msg("user registered")

	return user, nil
}

// Get retrieves a user by identifier.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*entity.User, error) {
	user, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return user, nil
}

func hashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}
