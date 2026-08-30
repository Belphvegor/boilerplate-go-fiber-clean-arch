package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/casper/go-fiber-clean-arch/internal/auth/entity"
)

var (
	ErrIdentityNotFound = errors.New("auth repository: identity not found")
	ErrIdentityExists   = errors.New("auth repository: identity already exists")
	ErrSessionNotFound  = errors.New("auth repository: session not found")
)

// Repository defines persistence for external auth identities and refresh sessions.
type Repository interface {
	CreateIdentity(ctx context.Context, identity *entity.Identity) error
	FindIdentity(ctx context.Context, provider, subject string) (*entity.Identity, error)
	FindIdentityByID(ctx context.Context, id uuid.UUID) (*entity.Identity, error)

	CreateSession(ctx context.Context, session *entity.Session) error
	FindSessionByHash(ctx context.Context, tokenHash string) (*entity.Session, error)
	// ConsumeSession marks a session used only if it is still unused, and reports
	// ErrSessionNotFound otherwise. It must be atomic: it is what stops two
	// concurrent refreshes from both minting a token pair off the same row.
	ConsumeSession(ctx context.Context, id uuid.UUID, usedAt time.Time) error
	// RevokeSessionFamily revokes every session sharing sessionID, which ends the
	// login both on logout and on detected token replay.
	RevokeSessionFamily(ctx context.Context, sessionID uuid.UUID, revokedAt time.Time) error
}
