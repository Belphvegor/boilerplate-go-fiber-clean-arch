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
		Secret:     "test-secret-that-is-long-enough",
		Issuer:     "clean-arch-starter",
		Audience:   "clean-arch-starter-api",
		AccessTTL:  time.Minute,
		RefreshTTL: time.Hour,
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

func TestService_CompleteLogin_RejectsMissingNonceCookie(t *testing.T) {
	// Provider echoes no nonce and the cookie is gone: without the guard both sides
	// are "" and the replay check would pass.
	provider := &fakeProvider{
		token: &ProviderToken{
			Provider:      "https://issuer.example.com",
			Subject:       "provider-subject",
			Email:         "ada@example.com",
			EmailVerified: true,
			Name:          "Ada Lovelace",
		},
	}
	service := NewService(newMemoryAuthRepo(), newMemoryUserRepo(), provider, mustTokenService(t), zerolog.Nop())

	_, err := service.CompleteLogin(context.Background(), CallbackInput{
		Code:          "code",
		State:         "state",
		ExpectedState: "state",
		ExpectedNonce: "",
		Verifier:      "verifier",
	})

	assert.True(t, errors.Is(err, ErrInvalidCallback))
}

func TestService_Refresh_RotatesAndInvalidatesThePreviousToken(t *testing.T) {
	service, first := loginForTest(t)

	second, err := service.Refresh(context.Background(), first.RefreshToken)
	require.NoError(t, err)
	require.NotEmpty(t, second.RefreshToken)
	assert.NotEqual(t, first.RefreshToken, second.RefreshToken, "refresh must rotate the token")
	assert.NotEmpty(t, second.AccessToken)
	assert.Equal(t, first.User.ID, second.User.ID)

	// The rotated-away token is now spent.
	_, err = service.Refresh(context.Background(), first.RefreshToken)
	assert.True(t, errors.Is(err, ErrRefreshReused))
}

func TestService_Refresh_ReplayRevokesTheWholeFamily(t *testing.T) {
	service, first := loginForTest(t)

	second, err := service.Refresh(context.Background(), first.RefreshToken)
	require.NoError(t, err)

	// Replaying the spent token is treated as a leak, not a mistake.
	_, err = service.Refresh(context.Background(), first.RefreshToken)
	require.True(t, errors.Is(err, ErrRefreshReused))

	// So the token the legitimate client is holding dies too.
	_, err = service.Refresh(context.Background(), second.RefreshToken)
	assert.True(t, errors.Is(err, ErrInvalidRefresh),
		"replay must revoke the whole family, including the currently valid token")
}

func TestService_Refresh_RejectsUnknownToken(t *testing.T) {
	service, _ := loginForTest(t)

	_, err := service.Refresh(context.Background(), "not-a-real-token")
	assert.True(t, errors.Is(err, ErrInvalidRefresh))

	_, err = service.Refresh(context.Background(), "")
	assert.True(t, errors.Is(err, ErrInvalidRefresh))
}

func TestService_Logout_RevokesSessionAndReturnsProviderURL(t *testing.T) {
	service, first := loginForTest(t)

	logoutURL, err := service.Logout(context.Background(), first.RefreshToken)
	require.NoError(t, err)
	assert.Contains(t, logoutURL, "id_token_hint=raw-id-token",
		"logout must hint the ID token so the provider ends its session silently")

	_, err = service.Refresh(context.Background(), first.RefreshToken)
	assert.True(t, errors.Is(err, ErrInvalidRefresh), "refresh must fail after logout")
}

func TestService_Refresh_LosesRaceAgainstConcurrentLogout(t *testing.T) {
	// A logout landing between ConsumeSession and the successor insert used to leave
	// the successor unrevoked, handing back a live token for a session the user had
	// just ended. The repo hook below reproduces exactly that interleaving.
	authRepo := newMemoryAuthRepo()
	racing := &revokeOnCreateRepo{memoryAuthRepo: authRepo}
	service, first := loginForTestWith(t, racing)

	racing.revokeDuringNextCreate = true
	_, err := service.Refresh(context.Background(), first.RefreshToken)
	require.True(t, errors.Is(err, ErrInvalidRefresh),
		"a refresh overlapping a logout must not succeed, got %v", err)

	// And no token from that family may survive the race.
	for hash, session := range authRepo.sessions {
		assert.NotNilf(t, session.RevokedAt, "session %s survived the logout race", hash)
	}
}

// revokeOnCreateRepo revokes the family at the instant a successor is inserted,
// simulating a logout that arrives mid-rotation.
type revokeOnCreateRepo struct {
	*memoryAuthRepo
	revokeDuringNextCreate bool
}

func (r *revokeOnCreateRepo) CreateSession(ctx context.Context, session *authentity.Session) error {
	if r.revokeDuringNextCreate {
		r.revokeDuringNextCreate = false
		if err := r.memoryAuthRepo.RevokeSessionFamily(ctx, session.SessionID, time.Now().UTC()); err != nil {
			return err
		}
	}
	return r.memoryAuthRepo.CreateSession(ctx, session)
}

// loginForTest runs a successful login and returns the service plus the first token pair.
func loginForTest(t *testing.T) (*Service, *TokenResponse) {
	t.Helper()
	return loginForTestWith(t, newMemoryAuthRepo())
}

func loginForTestWith(t *testing.T, authRepo authrepo.Repository) (*Service, *TokenResponse) {
	t.Helper()

	provider := &fakeProvider{
		token: &ProviderToken{
			Provider:      "https://issuer.example.com",
			Subject:       "provider-subject",
			Email:         "ada@example.com",
			EmailVerified: true,
			Name:          "Ada Lovelace",
			Nonce:         "nonce",
			IDToken:       "raw-id-token",
		},
	}
	service := NewService(authRepo, newMemoryUserRepo(), provider, mustTokenService(t), zerolog.Nop())

	resp, err := service.CompleteLogin(context.Background(), CallbackInput{
		Code:          "code",
		State:         "state",
		ExpectedState: "state",
		ExpectedNonce: "nonce",
		Verifier:      "verifier",
	})
	require.NoError(t, err)
	require.NotEmpty(t, resp.RefreshToken)

	return service, resp
}

func mustTokenService(t *testing.T) *TokenService {
	t.Helper()
	tokens, err := NewTokenService(config.JWTConfig{
		Secret:     "test-secret-that-is-long-enough",
		Issuer:     "clean-arch-starter",
		Audience:   "clean-arch-starter-api",
		AccessTTL:  time.Minute,
		RefreshTTL: time.Hour,
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

func (p *fakeProvider) EndSessionURL(idToken string) string {
	return "https://issuer.example.com/logout?id_token_hint=" + idToken
}

type memoryAuthRepo struct {
	identities map[string]*authentity.Identity
	sessions   map[string]*authentity.Session
}

func newMemoryAuthRepo() *memoryAuthRepo {
	return &memoryAuthRepo{
		identities: make(map[string]*authentity.Identity),
		sessions:   make(map[string]*authentity.Session),
	}
}

func (r *memoryAuthRepo) FindIdentityByID(ctx context.Context, id uuid.UUID) (*authentity.Identity, error) {
	for _, identity := range r.identities {
		if identity.ID == id {
			return identity, nil
		}
	}
	return nil, authrepo.ErrIdentityNotFound
}

func (r *memoryAuthRepo) CreateSession(ctx context.Context, session *authentity.Session) error {
	copied := *session
	r.sessions[session.TokenHash] = &copied
	return nil
}

func (r *memoryAuthRepo) FindSessionByHash(ctx context.Context, tokenHash string) (*authentity.Session, error) {
	session, ok := r.sessions[tokenHash]
	if !ok {
		return nil, authrepo.ErrSessionNotFound
	}
	copied := *session
	return &copied, nil
}

func (r *memoryAuthRepo) ConsumeSession(ctx context.Context, id uuid.UUID, usedAt time.Time) error {
	for _, session := range r.sessions {
		if session.ID == id && session.UsedAt == nil && session.RevokedAt == nil {
			session.UsedAt = &usedAt
			return nil
		}
	}
	return authrepo.ErrSessionNotFound
}

func (r *memoryAuthRepo) RevokeSessionFamily(ctx context.Context, sessionID uuid.UUID, revokedAt time.Time) error {
	for _, session := range r.sessions {
		if session.SessionID == sessionID && session.RevokedAt == nil {
			session.RevokedAt = &revokedAt
		}
	}
	return nil
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
