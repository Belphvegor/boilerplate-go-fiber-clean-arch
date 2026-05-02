package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/casper/go-fiber-clean-arch/config"
	authentity "github.com/casper/go-fiber-clean-arch/internal/auth/entity"
	authrepo "github.com/casper/go-fiber-clean-arch/internal/auth/repository"
	userentity "github.com/casper/go-fiber-clean-arch/internal/user/entity"
	userrepo "github.com/casper/go-fiber-clean-arch/internal/user/repository"
)

func TestService_CompleteLogin_AutoProvisionAndIssueToken(t *testing.T) {
	authRepo := newMemoryAuthRepo()
	userRepo := newMemoryUserRepo()
	provider := &fakeProvider{
		token: &ProviderToken{
			Provider:      "https://issuer.example.com",
			Subject:       "provider-subject",
			Email:         "ada@example.com",
			EmailVerified: true,
			Name:          "Ada Lovelace",
			Nonce:         "nonce",
		},
	}
	tokens, err := NewTokenService(config.JWTConfig{
		Secret:    "test-secret-that-is-long-enough",
		Issuer:    "clean-arch-starter",
		Audience:  "clean-arch-starter-api",
		AccessTTL: time.Minute,
	})
	require.NoError(t, err)

	service := NewService(authRepo, userRepo, provider, tokens, zerolog.Nop())
	resp, err := service.CompleteLogin(context.Background(), CallbackInput{
		Code:          "code",
		State:         "state",
		ExpectedState: "state",
		ExpectedNonce: "nonce",
		Verifier:      "verifier",
	})

	require.NoError(t, err)
	require.NotEmpty(t, resp.AccessToken)
	assert.Equal(t, "Bearer", resp.TokenType)
	assert.Equal(t, int64(60), resp.ExpiresIn)
	assert.Equal(t, "Ada Lovelace", resp.User.Name)
	assert.Equal(t, "ada@example.com", resp.User.Email)

	identity, err := authRepo.FindIdentity(context.Background(), "https://issuer.example.com", "provider-subject")
	require.NoError(t, err)
	assert.Equal(t, resp.User.ID, identity.UserID.String())
}

func TestService_CompleteLogin_RejectsInvalidState(t *testing.T) {
	service := NewService(newMemoryAuthRepo(), newMemoryUserRepo(), &fakeProvider{}, mustTokenService(t), zerolog.Nop())

	_, err := service.CompleteLogin(context.Background(), CallbackInput{
		Code:          "code",
		State:         "actual",
		ExpectedState: "expected",
		ExpectedNonce: "nonce",
		Verifier:      "verifier",
	})

	assert.True(t, errors.Is(err, ErrInvalidCallback))
}

func TestService_CompleteLogin_RejectsUnverifiedEmail(t *testing.T) {
	provider := &fakeProvider{
		token: &ProviderToken{
			Provider:      "https://issuer.example.com",
			Subject:       "provider-subject",
			Email:         "ada@example.com",
			EmailVerified: false,
			Name:          "Ada Lovelace",
			Nonce:         "nonce",
		},
	}
	service := NewService(newMemoryAuthRepo(), newMemoryUserRepo(), provider, mustTokenService(t), zerolog.Nop())

	_, err := service.CompleteLogin(context.Background(), CallbackInput{
		Code:          "code",
		State:         "state",
		ExpectedState: "state",
		ExpectedNonce: "nonce",
		Verifier:      "verifier",
	})

	assert.True(t, errors.Is(err, ErrEmailRequired))
}

func mustTokenService(t *testing.T) *TokenService {
	t.Helper()
	tokens, err := NewTokenService(config.JWTConfig{
		Secret:    "test-secret-that-is-long-enough",
		Issuer:    "clean-arch-starter",
		Audience:  "clean-arch-starter-api",
		AccessTTL: time.Minute,
	})
	require.NoError(t, err)
	return tokens
}

type fakeProvider struct {
	token *ProviderToken
}

func (p *fakeProvider) AuthCodeURL(state, nonce, verifier string) string {
	return "https://issuer.example.com/auth?state=" + state
}

func (p *fakeProvider) Exchange(ctx context.Context, code, verifier string) (*ProviderToken, error) {
	return p.token, nil
}

type memoryAuthRepo struct {
	identities map[string]*authentity.Identity
}

func newMemoryAuthRepo() *memoryAuthRepo {
	return &memoryAuthRepo{identities: make(map[string]*authentity.Identity)}
}

func (r *memoryAuthRepo) CreateIdentity(ctx context.Context, identity *authentity.Identity) error {
	key := identity.Provider + "\x00" + identity.Subject
	if _, exists := r.identities[key]; exists {
		return authrepo.ErrIdentityExists
	}
	r.identities[key] = identity
	return nil
}

func (r *memoryAuthRepo) FindIdentity(ctx context.Context, provider, subject string) (*authentity.Identity, error) {
	identity, ok := r.identities[provider+"\x00"+subject]
	if !ok {
		return nil, authrepo.ErrIdentityNotFound
	}
	return identity, nil
}

type memoryUserRepo struct {
	byID    map[uuid.UUID]*userentity.User
	byEmail map[string]*userentity.User
}

func newMemoryUserRepo() *memoryUserRepo {
	return &memoryUserRepo{
		byID:    make(map[uuid.UUID]*userentity.User),
		byEmail: make(map[string]*userentity.User),
	}
}

func (r *memoryUserRepo) Create(ctx context.Context, user *userentity.User) error {
	if _, exists := r.byEmail[user.Email]; exists {
		return userrepo.ErrEmailExists
	}
	r.byID[user.ID] = user
	r.byEmail[user.Email] = user
	return nil
}

func (r *memoryUserRepo) Update(ctx context.Context, user *userentity.User) error {
	r.byID[user.ID] = user
	r.byEmail[user.Email] = user
	return nil
}

func (r *memoryUserRepo) FindByID(ctx context.Context, id uuid.UUID) (*userentity.User, error) {
	user, ok := r.byID[id]
	if !ok {
		return nil, userrepo.ErrNotFound
	}
	return user, nil
}

func (r *memoryUserRepo) FindByEmail(ctx context.Context, email string) (*userentity.User, error) {
	user, ok := r.byEmail[email]
	if !ok {
		return nil, userrepo.ErrNotFound
	}
	return user, nil
}

func (r *memoryUserRepo) Delete(ctx context.Context, id uuid.UUID) error {
	user, ok := r.byID[id]
	if !ok {
		return userrepo.ErrNotFound
	}
	delete(r.byID, id)
	delete(r.byEmail, user.Email)
	return nil
}
