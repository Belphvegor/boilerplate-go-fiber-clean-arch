package usecase

import (
	"context"
	"errors"
	"strings"
	"time"

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
	ErrInvalidRefresh    = errors.New("auth: refresh token is invalid or expired")
	ErrRefreshReused     = errors.New("auth: refresh token was already used; session revoked")
)

// ProviderToken is the verified OIDC identity returned by the provider adapter.
type ProviderToken struct {
	Provider      string
	Subject       string
	Email         string
	EmailVerified bool
	Name          string
	Nonce         string
	// IDToken is the raw compact JWT, kept so logout can pass it to the provider as
	// id_token_hint. Keycloak only logs out silently when that hint is present.
	IDToken string
}

// OIDCProvider abstracts the production OIDC client and test fakes.
type OIDCProvider interface {
	AuthCodeURL(state, nonce, verifier string) string
	Exchange(ctx context.Context, code, verifier string) (*ProviderToken, error)
	// EndSessionURL builds the provider's RP-initiated logout URL. Returns "" when
	// the provider published no end_session_endpoint.
	EndSessionURL(idToken string) string
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

// TokenResponse is returned after a successful login callback or refresh.
type TokenResponse struct {
	AccessToken      string       `json:"access_token"`
	TokenType        string       `json:"token_type"`
	ExpiresIn        int64        `json:"expires_in"`
	RefreshToken     string       `json:"refresh_token"`
	RefreshExpiresIn int64        `json:"refresh_expires_in"`
	User             UserResponse `json:"user"`
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
	// Every cookie must be present: a missing nonce cookie would otherwise make the
	// replay check below compare "" against "" and pass.
	if input.Code == "" || input.State == "" || input.ExpectedState == "" || input.ExpectedNonce == "" || input.Verifier == "" {
		return nil, ErrInvalidCallback
	}
	if !secureEqual(input.State, input.ExpectedState) {
		return nil, ErrInvalidCallback
	}

	providerToken, err := s.provider.Exchange(ctx, input.Code, input.Verifier)
	if err != nil {
		return nil, err
	}
	if providerToken == nil {
		return nil, ErrProvisioningState
	}
	if providerToken.Nonce == "" || !secureEqual(providerToken.Nonce, input.ExpectedNonce) {
		return nil, ErrInvalidNonce
	}

	user, identity, err := s.resolveUser(ctx, providerToken)
	if err != nil {
		return nil, err
	}

	session, refreshToken, err := s.startSession(ctx, user, identity, providerToken.IDToken)
	if err != nil {
		return nil, err
	}

	return s.issueTokenPair(user, identity, session, refreshToken)
}

// Refresh exchanges a refresh token for a new token pair, rotating the refresh token.
//
// Rotation plus replay detection follows RFC 9700: each refresh token is single use,
// and presenting one that was already exchanged is treated as proof the token leaked,
// so the whole rotation family is revoked rather than just the replayed token.
func (s *Service) Refresh(ctx context.Context, rawToken string) (*TokenResponse, error) {
	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" {
		return nil, ErrInvalidRefresh
	}

	session, err := s.authRepo.FindSessionByHash(ctx, hashToken(rawToken))
	if err != nil {
		if errors.Is(err, authrepo.ErrSessionNotFound) {
			return nil, ErrInvalidRefresh
		}
		return nil, err
	}

	now := time.Now().UTC()
	if session.UsedAt != nil {
		if revokeErr := s.authRepo.RevokeSessionFamily(ctx, session.SessionID, now); revokeErr != nil {
			s.logger.Error().Err(revokeErr).
				Str("session_id", session.SessionID.String()).
				Msg("could not revoke replayed refresh token family")
		}
		s.logger.Warn().
			Str("user_id", session.UserID.String()).
			Str("session_id", session.SessionID.String()).
			Msg("refresh token replay detected; session revoked")
		return nil, ErrRefreshReused
	}
	// Expiry only: ConsumeSession's revoked_at predicate re-checks revocation atomically,
	// and UsedAt was handled by the replay branch above.
	if !now.Before(session.ExpiresAt) {
		return nil, ErrInvalidRefresh
	}

	// Atomic compare-and-swap: two concurrent refreshes race here and exactly one wins.
	if err := s.authRepo.ConsumeSession(ctx, session.ID, now); err != nil {
		if errors.Is(err, authrepo.ErrSessionNotFound) {
			return nil, ErrInvalidRefresh
		}
		return nil, err
	}

	identity, err := s.authRepo.FindIdentityByID(ctx, session.IdentityID)
	if err != nil {
		return nil, err
	}
	user, err := s.userRepo.FindByID(ctx, session.UserID)
	if err != nil {
		return nil, err
	}

	rotated, refreshToken, err := s.rotateSession(ctx, session)
	if err != nil {
		// The presented token is already spent, so leaving the family alive would hand
		// the client an ambiguous state: its only token is dead, a retry looks like
		// replay, and nothing says the session ended. Revoking makes it definite.
		if revokeErr := s.authRepo.RevokeSessionFamily(ctx, session.SessionID, now); revokeErr != nil {
			s.logger.Error().Err(revokeErr).
				Str("session_id", session.SessionID.String()).
				Msg("could not revoke session after failed rotation")
		}
		if errors.Is(err, authentity.ErrInvalidSessionTTL) {
			return nil, ErrInvalidRefresh
		}
		return nil, err
	}

	// A logout that landed between ConsumeSession and the insert above would have
	// revoked every row that existed at the time, missing the successor. Re-reading the
	// predecessor closes that window: if it is revoked now, the family ended mid-refresh
	// and the successor must not survive.
	//
	// ponytail: compensating re-check rather than a transaction, because the Mongo
	// adapter would need a replica set for one. Move both writes into a transaction if
	// this ever needs to be airtight under sustained concurrency.
	if predecessor, err := s.authRepo.FindSessionByHash(ctx, session.TokenHash); err == nil && predecessor.RevokedAt != nil {
		if revokeErr := s.authRepo.RevokeSessionFamily(ctx, session.SessionID, now); revokeErr != nil {
			s.logger.Error().Err(revokeErr).
				Str("session_id", session.SessionID.String()).
				Msg("could not revoke session rotated during logout")
		}
		return nil, ErrInvalidRefresh
	}

	return s.issueTokenPair(user, identity, rotated, refreshToken)
}

// Logout revokes the caller's session family and returns the provider's RP-initiated
// logout URL, or "" when the provider published no end_session_endpoint.
//
// The local session is already revoked when this returns; sending the browser to that
// URL is what also drops the provider's SSO cookie, without which a later /auth/login
// silently signs the same user straight back in.
func (s *Service) Logout(ctx context.Context, rawToken string) (string, error) {
	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" {
		return "", ErrInvalidRefresh
	}

	session, err := s.authRepo.FindSessionByHash(ctx, hashToken(rawToken))
	if err != nil {
		if errors.Is(err, authrepo.ErrSessionNotFound) {
			return "", ErrInvalidRefresh
		}
		return "", err
	}

	// Revoke even an already-used or already-revoked row: holding any token from the
	// family is enough to ask for the whole login to end, and that keeps logout
	// idempotent for a client retrying after a network failure.
	if err := s.authRepo.RevokeSessionFamily(ctx, session.SessionID, time.Now().UTC()); err != nil {
		return "", err
	}

	s.logger.Info().
		Str("user_id", session.UserID.String()).
		Str("session_id", session.SessionID.String()).
		Msg("session revoked by logout")

	return s.provider.EndSessionURL(session.IDToken), nil
}

func (s *Service) startSession(ctx context.Context, user *userentity.User, identity *authentity.Identity, idToken string) (*authentity.Session, string, error) {
	raw, err := randomURLToken()
	if err != nil {
		return nil, "", err
	}
	session, err := authentity.NewSession(user.ID, identity.ID, hashToken(raw), idToken, s.tokens.RefreshTTL())
	if err != nil {
		return nil, "", err
	}
	if err := s.authRepo.CreateSession(ctx, session); err != nil {
		return nil, "", err
	}
	return session, raw, nil
}

func (s *Service) rotateSession(ctx context.Context, previous *authentity.Session) (*authentity.Session, string, error) {
	raw, err := randomURLToken()
	if err != nil {
		return nil, "", err
	}
	// ponytail: fixed session deadline, see Session.Rotate. Carry a separate absolute
	// deadline if you ever want sliding expiry with a hard cap.
	next, err := previous.Rotate(hashToken(raw))
	if err != nil {
		return nil, "", err
	}
	if err := s.authRepo.CreateSession(ctx, next); err != nil {
		return nil, "", err
	}
	return next, raw, nil
}

func (s *Service) issueTokenPair(user *userentity.User, identity *authentity.Identity, session *authentity.Session, refreshToken string) (*TokenResponse, error) {
	accessToken, err := s.tokens.Issue(user, identity)
	if err != nil {
		return nil, err
	}

	return &TokenResponse{
		AccessToken:      accessToken.Value,
		TokenType:        accessToken.TokenType,
		ExpiresIn:        accessToken.ExpiresIn,
		RefreshToken:     refreshToken,
		RefreshExpiresIn: int64(time.Until(session.ExpiresAt).Seconds()),
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

	// ponytail: links to an existing local user purely on verified email. Safe with one
	// trusted issuer; add a linking-confirmation step before adding a second IdP or a
	// password login, otherwise a pre-registered email row becomes a takeover vector.
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
