package oidc

import (
	"context"
	"fmt"

	oidcclient "github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"github.com/casper/go-fiber-clean-arch/config"
	"github.com/casper/go-fiber-clean-arch/internal/auth/usecase"
)

// Provider implements the production OIDC authorization-code client.
type Provider struct {
	issuer       string
	oauth2Config oauth2.Config
	verifier     *oidcclient.IDTokenVerifier
}

// NewProvider discovers OIDC metadata and builds an OAuth2 client.
func NewProvider(ctx context.Context, cfg config.OIDCConfig) (*Provider, error) {
	provider, err := oidcclient.NewProvider(ctx, cfg.IssuerURL)
	if err != nil {
		return nil, fmt.Errorf("discover oidc provider: %w", err)
	}

	scopes := normalizeScopes(cfg.Scopes)
	oauthCfg := oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  cfg.RedirectURL,
		Endpoint:     provider.Endpoint(),
		Scopes:       scopes,
	}

	return &Provider{
		issuer:       cfg.IssuerURL,
		oauth2Config: oauthCfg,
		verifier:     provider.Verifier(&oidcclient.Config{ClientID: cfg.ClientID}),
	}, nil
}

// AuthCodeURL returns the provider redirect URL using state, nonce, and PKCE S256.
func (p *Provider) AuthCodeURL(state, nonce, verifier string) string {
	return p.oauth2Config.AuthCodeURL(
		state,
		oauth2.SetAuthURLParam("nonce", nonce),
		oauth2.S256ChallengeOption(verifier),
	)
}

// Exchange exchanges the code and verifies the ID token.
func (p *Provider) Exchange(ctx context.Context, code, verifier string) (*usecase.ProviderToken, error) {
	token, err := p.oauth2Config.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		return nil, fmt.Errorf("exchange oidc code: %w", err)
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		return nil, fmt.Errorf("exchange oidc code: id_token missing")
	}

	idToken, err := p.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("verify id token: %w", err)
	}

	var claims struct {
		Subject           string `json:"sub"`
		Email             string `json:"email"`
		EmailVerified     bool   `json:"email_verified"`
		Name              string `json:"name"`
		PreferredUsername string `json:"preferred_username"`
		Nonce             string `json:"nonce"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("decode id token claims: %w", err)
	}

	name := claims.Name
	if name == "" {
		name = claims.PreferredUsername
	}

	return &usecase.ProviderToken{
		Provider:      p.issuer,
		Subject:       claims.Subject,
		Email:         claims.Email,
		EmailVerified: claims.EmailVerified,
		Name:          name,
		Nonce:         claims.Nonce,
	}, nil
}

func normalizeScopes(scopes []string) []string {
	seen := map[string]struct{}{"openid": {}}
	normalized := []string{"openid"}
	for _, scope := range scopes {
		if scope == "" {
			continue
		}
		if _, ok := seen[scope]; ok {
			continue
		}
		seen[scope] = struct{}{}
		normalized = append(normalized, scope)
	}
	return normalized
}
