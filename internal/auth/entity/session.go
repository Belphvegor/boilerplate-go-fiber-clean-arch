package entity

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var ErrInvalidSessionTTL = errors.New("auth session: ttl must be positive")

// Session is one issued refresh token.
//
// Rotation creates a new row that keeps the same SessionID, so a family of rows
// tracks one login. Superseded rows are kept, not deleted: presenting a row that
// already has UsedAt set is how token replay is detected.
type Session struct {
	ID         uuid.UUID
	SessionID  uuid.UUID
	UserID     uuid.UUID
	IdentityID uuid.UUID
	TokenHash  string
	IDToken    string
	ExpiresAt  time.Time
	UsedAt     *time.Time
	RevokedAt  *time.Time
	CreatedAt  time.Time
}

// NewSession starts a new rotation family for a fresh login.
func NewSession(userID, identityID uuid.UUID, tokenHash, idToken string, ttl time.Duration) (*Session, error) {
	return newSession(uuid.New(), userID, identityID, tokenHash, idToken, ttl)
}

// Rotate issues the successor to this session, keeping it in the same family and
// carrying the ID token forward so logout can still hint it to the provider.
//
// The successor inherits the original deadline rather than extending it, so a session
// cannot be kept alive forever by refreshing. A refresh arriving in the last instants
// of a session leaves a non-positive remainder and is rejected as expired.
func (s *Session) Rotate(tokenHash string) (*Session, error) {
	return newSession(s.SessionID, s.UserID, s.IdentityID, tokenHash, s.IDToken, time.Until(s.ExpiresAt))
}

func newSession(sessionID, userID, identityID uuid.UUID, tokenHash, idToken string, ttl time.Duration) (*Session, error) {
	if userID == uuid.Nil || identityID == uuid.Nil {
		return nil, ErrInvalidUserID
	}
	if ttl <= 0 {
		return nil, ErrInvalidSessionTTL
	}

	now := time.Now().UTC()
	return &Session{
		ID:         uuid.New(),
		SessionID:  sessionID,
		UserID:     userID,
		IdentityID: identityID,
		TokenHash:  tokenHash,
		IDToken:    idToken,
		ExpiresAt:  now.Add(ttl),
		CreatedAt:  now,
	}, nil
}
