package usecase

import (
	"context"
	"errors"
	"strings"

	"github.com/rs/zerolog"
	"golang.org/x/crypto/bcrypt"

	authentity "github.com/casper/go-fiber-clean-arch/internal/auth/entity"
	authrepo "github.com/casper/go-fiber-clean-arch/internal/auth/repository"
	userentity "github.com/casper/go-fiber-clean-arch/internal/user/entity"
	userrepo "github.com/casper/go-fiber-clean-arch/internal/user/repository"
)

var (
	ErrInvalidCallback   = errors.New("auth: invalid callback")
	ErrInvalidNonce      = errors.New("auth: invalid nonce")
	ErrEmailRequired     = errors.New("auth: verified email is required")
	ErrEmailConflict     = errors.New("auth: email already belongs to another login method")
	ErrIdentityRace      = errors.New("auth: identity was created concurrently")
	ErrProvisioningState = errors.New("auth: could not provision identity")
)

// ProviderToken is the verified OIDC identity returned by the provider adapter.
type ProviderToken struct {
	Provider      string
	Subject       string
	Email         string
	EmailVerified bool
	Name          string
	Nonce         string
}

// OIDCProvider abstracts the production OIDC client and test fakes.
type OIDCProvider interface {
	AuthCodeURL(state, nonce, verifier string) string
	Exchange(ctx context.Context, code, verifier string) (*ProviderToken, error)
}

// LoginSession contains redirect and temporary verifier values for login.
type LoginSession struct {
	AuthURL  string
	State    string
	Nonce    string
	Verifier string
}

// CallbackInput contains callback query and cookie values.
type CallbackInput struct {
	Code          string
	State         string
	ExpectedState string
	ExpectedNonce string
	Verifier      string
}

// TokenResponse is returned after a successful login callback.
type TokenResponse struct {
	AccessToken string       `json:"access_token"`
	TokenType   string       `json:"token_type"`
	ExpiresIn   int64        `json:"expires_in"`
	User        UserResponse `json:"user"`
}

// UserResponse is the public user shape embedded in auth responses.
type UserResponse struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// Service orchestrates OIDC login and local user provisioning.
type Service struct {
	authRepo authrepo.Repository
	userRepo userrepo.Repository
	provider OIDCProvider
	tokens   *TokenService
	logger   zerolog.Logger
}

// NewService constructs an auth Service.
func NewService(authRepo authrepo.Repository, userRepo userrepo.Repository, provider OIDCProvider, tokens *TokenService, logger zerolog.Logger) *Service {
	return &Service{
		authRepo: authRepo,
		userRepo: userRepo,
		provider: provider,
		tokens:   tokens,
		logger:   logger,
	}
}

// BeginLogin creates PKCE/state/nonce values and the provider authorization URL.
func (s *Service) BeginLogin() (*LoginSession, error) {
	state, err := randomURLToken()
	if err != nil {
		return nil, err
	}
	nonce, err := randomURLToken()
	if err != nil {
		return nil, err
	}
	verifier, err := randomURLToken()
	if err != nil {
		return nil, err
	}

	return &LoginSession{
		AuthURL:  s.provider.AuthCodeURL(state, nonce, verifier),
		State:    state,
		Nonce:    nonce,
		Verifier: verifier,
	}, nil
}

// CompleteLogin validates callback state, exchanges the code, provisions a user, and issues an API token.
func (s *Service) CompleteLogin(ctx context.Context, input CallbackInput) (*TokenResponse, error) {
	if input.Code == "" || input.State == "" || input.ExpectedState == "" || input.Verifier == "" {
		return nil, ErrInvalidCallback
	}
	if input.State != input.ExpectedState {
		return nil, ErrInvalidCallback
	}

	providerToken, err := s.provider.Exchange(ctx, input.Code, input.Verifier)
	if err != nil {
		return nil, err
	}
	if providerToken == nil {
		return nil, ErrProvisioningState
	}
	if providerToken.Nonce != input.ExpectedNonce {
		return nil, ErrInvalidNonce
	}

	user, identity, err := s.resolveUser(ctx, providerToken)
	if err != nil {
		return nil, err
	}

	accessToken, err := s.tokens.Issue(user, identity)
	if err != nil {
		return nil, err
	}

	return &TokenResponse{
		AccessToken: accessToken.Value,
		TokenType:   accessToken.TokenType,
		ExpiresIn:   accessToken.ExpiresIn,
		User: UserResponse{
			ID:    user.ID.String(),
			Name:  user.Name,
			Email: user.Email,
		},
	}, nil
}

func (s *Service) resolveUser(ctx context.Context, token *ProviderToken) (*userentity.User, *authentity.Identity, error) {
	provider := strings.TrimSpace(token.Provider)
	subject := strings.TrimSpace(token.Subject)
	if provider == "" || subject == "" {
		return nil, nil, ErrProvisioningState
	}

	identity, err := s.authRepo.FindIdentity(ctx, provider, subject)
	if err == nil {
		user, userErr := s.userRepo.FindByID(ctx, identity.UserID)
		return user, identity, userErr
	}
	if err != nil && !errors.Is(err, authrepo.ErrIdentityNotFound) {
		return nil, nil, err
	}

	user, err := s.resolveProvisionedUser(ctx, token)
	if err != nil {
		return nil, nil, err
	}

	identity, err = authentity.NewIdentity(user.ID, provider, subject, token.Email)
	if err != nil {
		return nil, nil, err
	}
	if err := s.authRepo.CreateIdentity(ctx, identity); err != nil {
		if errors.Is(err, authrepo.ErrIdentityExists) {
			identity, findErr := s.authRepo.FindIdentity(ctx, provider, subject)
			if findErr != nil {
				return nil, nil, ErrIdentityRace
			}
			user, userErr := s.userRepo.FindByID(ctx, identity.UserID)
			return user, identity, userErr
		}
		return nil, nil, err
	}

	s.logger.Info().
		Str("user_id", user.ID.String()).
		Str("provider", provider).
		Msg("oidc identity linked")

	return user, identity, nil
}

func (s *Service) resolveProvisionedUser(ctx context.Context, token *ProviderToken) (*userentity.User, error) {
	email := strings.TrimSpace(strings.ToLower(token.Email))
	if email == "" || !token.EmailVerified {
		return nil, ErrEmailRequired
	}

	existing, err := s.userRepo.FindByEmail(ctx, email)
	if err == nil {
		return existing, nil
	}
	if err != nil && !errors.Is(err, userrepo.ErrNotFound) {
		return nil, err
	}

	passwordHash, err := externalPasswordHash()
	if err != nil {
		return nil, err
	}
	user, err := userentity.New(displayName(token.Name, email), email, passwordHash)
	if err != nil {
		return nil, err
	}
	if err := s.userRepo.Create(ctx, user); err != nil {
		if errors.Is(err, userrepo.ErrEmailExists) {
			return nil, ErrEmailConflict
		}
		return nil, err
	}

	return user, nil
}

func externalPasswordHash() (string, error) {
	raw, err := randomURLToken()
	if err != nil {
		return "", err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(raw), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func displayName(name, email string) string {
	name = strings.TrimSpace(name)
	if len(name) >= 2 {
		return name
	}
	if at := strings.IndexByte(email, '@'); at > 1 {
		return email[:at]
	}
	return "OIDC User"
}
