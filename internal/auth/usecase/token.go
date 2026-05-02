package usecase

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/casper/go-fiber-clean-arch/config"
	authentity "github.com/casper/go-fiber-clean-arch/internal/auth/entity"
	userentity "github.com/casper/go-fiber-clean-arch/internal/user/entity"
)

var ErrInvalidTokenConfig = errors.New("auth token: invalid jwt configuration")

// AccessToken contains a signed local API token and metadata.
type AccessToken struct {
	Value     string
	TokenType string
	ExpiresIn int64
}

// Claims are the private claims embedded in local API access tokens.
type Claims struct {
	Email           string `json:"email"`
	Name            string `json:"name"`
	Provider        string `json:"provider"`
	ProviderSubject string `json:"provider_subject"`
	jwt.RegisteredClaims
}

// TokenService signs local API access tokens.
type TokenService struct {
	secret   []byte
	issuer   string
	audience string
	ttl      time.Duration
}

// NewTokenService constructs a TokenService from JWT configuration.
func NewTokenService(cfg config.JWTConfig) (*TokenService, error) {
	if cfg.Secret == "" || cfg.Issuer == "" || cfg.Audience == "" || cfg.AccessTTL <= 0 {
		return nil, ErrInvalidTokenConfig
	}
	return &TokenService{
		secret:   []byte(cfg.Secret),
		issuer:   cfg.Issuer,
		audience: cfg.Audience,
		ttl:      cfg.AccessTTL,
	}, nil
}

// Issue signs a short-lived bearer token for a local user.
func (s *TokenService) Issue(user *userentity.User, identity *authentity.Identity) (*AccessToken, error) {
	now := time.Now().UTC()
	expiresAt := now.Add(s.ttl)

	claims := Claims{
		Email:           user.Email,
		Name:            user.Name,
		Provider:        identity.Provider,
		ProviderSubject: identity.Subject,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   user.ID.String(),
			Issuer:    s.issuer,
			Audience:  jwt.ClaimStrings{s.audience},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(s.secret)
	if err != nil {
		return nil, err
	}

	return &AccessToken{
		Value:     signed,
		TokenType: "Bearer",
		ExpiresIn: int64(s.ttl.Seconds()),
	}, nil
}
