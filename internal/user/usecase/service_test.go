package usecase_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/casper/go-fiber-clean-arch/internal/user/entity"
	"github.com/casper/go-fiber-clean-arch/internal/user/usecase"
)

type stubRepository struct {
	findByIDFunc func(ctx context.Context, id uuid.UUID) (*entity.User, error)
}

func (s *stubRepository) Create(context.Context, *entity.User) error { return nil }

func (s *stubRepository) FindByID(ctx context.Context, id uuid.UUID) (*entity.User, error) {
	if s.findByIDFunc != nil {
		return s.findByIDFunc(ctx, id)
	}
	return nil, usecase.ErrNotFound
}

func (s *stubRepository) FindByEmail(context.Context, string) (*entity.User, error) {
	return nil, usecase.ErrNotFound
}

func TestService_Get(t *testing.T) {
	id := uuid.New()
	repo := &stubRepository{
		findByIDFunc: func(ctx context.Context, uuid uuid.UUID) (*entity.User, error) {
			return &entity.User{ID: id, Name: "Ada", Email: "ada@example.com"}, nil
		},
	}
	service := usecase.NewService(repo)

	user, err := service.Get(context.Background(), id)

	require.NoError(t, err)
	require.NotNil(t, user)
	assert.Equal(t, id, user.ID)
}
