package usecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/casper/go-fiber-clean-arch/internal/shared/validator"
	"github.com/casper/go-fiber-clean-arch/internal/user/entity"
	"github.com/casper/go-fiber-clean-arch/internal/user/usecase"
)

type stubRepository struct {
	findByEmailFunc func(ctx context.Context, email string) (*entity.User, error)
	createFunc      func(ctx context.Context, user *entity.User) error
	findByIDFunc    func(ctx context.Context, id uuid.UUID) (*entity.User, error)
}

func (s *stubRepository) Create(ctx context.Context, user *entity.User) error {
	if s.createFunc != nil {
		return s.createFunc(ctx, user)
	}
	return nil
}

func (s *stubRepository) Update(ctx context.Context, user *entity.User) error { return nil }

func (s *stubRepository) FindByID(ctx context.Context, id uuid.UUID) (*entity.User, error) {
	if s.findByIDFunc != nil {
		return s.findByIDFunc(ctx, id)
	}
	return nil, errors.New("not implemented")
}

func (s *stubRepository) FindByEmail(ctx context.Context, email string) (*entity.User, error) {
	if s.findByEmailFunc != nil {
		return s.findByEmailFunc(ctx, email)
	}
	return nil, errors.New("not implemented")
}

func (s *stubRepository) Delete(ctx context.Context, id uuid.UUID) error { return nil }

func TestService_Register_Success(t *testing.T) {
	repo := &stubRepository{
		findByEmailFunc: func(ctx context.Context, email string) (*entity.User, error) {
			return nil, usecase.ErrNotFound
		},
		createFunc: func(ctx context.Context, user *entity.User) error {
			assert.NotEqual(t, "", user.PasswordHash)
			return nil
		},
	}

	service := usecase.NewService(repo, validator.New(), zerolog.Nop())

	user, err := service.Register(context.Background(), usecase.CreateInput{
		Name:     "Ada Lovelace",
		Email:    "ada@example.com",
		Password: "Str0ngPass!",
	})

	require.NoError(t, err)
	require.NotNil(t, user)
	assert.Equal(t, "Ada Lovelace", user.Name)
	assert.Equal(t, "ada@example.com", user.Email)
	assert.WithinDuration(t, time.Now(), user.CreatedAt, time.Second)
	assert.NotEmpty(t, user.ID)
}

func TestService_Register_DuplicateEmail(t *testing.T) {
	repo := &stubRepository{
		findByEmailFunc: func(ctx context.Context, email string) (*entity.User, error) {
			return &entity.User{}, nil
		},
	}

	service := usecase.NewService(repo, validator.New(), zerolog.Nop())

	_, err := service.Register(context.Background(), usecase.CreateInput{
		Name:     "Ada Lovelace",
		Email:    "ada@example.com",
		Password: "Str0ngPass!",
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, usecase.ErrEmailExists))
}

func TestService_Register_InvalidInput(t *testing.T) {
	repo := &stubRepository{}
	service := usecase.NewService(repo, validator.New(), zerolog.Nop())

	_, err := service.Register(context.Background(), usecase.CreateInput{
		Name:     "A",
		Email:    "invalid",
		Password: "short",
	})

	require.Error(t, err)
}

func TestService_Get(t *testing.T) {
	id := uuid.New()
	repo := &stubRepository{
		findByIDFunc: func(ctx context.Context, uuid uuid.UUID) (*entity.User, error) {
			return &entity.User{ID: id, Name: "Ada", Email: "ada@example.com"}, nil
		},
	}
	service := usecase.NewService(repo, validator.New(), zerolog.Nop())

	user, err := service.Get(context.Background(), id)

	require.NoError(t, err)
	require.NotNil(t, user)
	assert.Equal(t, id, user.ID)
}
