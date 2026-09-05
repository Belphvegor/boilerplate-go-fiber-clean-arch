package persistence

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/casper/go-fiber-clean-arch/internal/auth/entity"
	"github.com/casper/go-fiber-clean-arch/internal/auth/repository"
)

// SQLRepository persists auth identities in PostgreSQL/MySQL databases.
type SQLRepository struct {
	db *sqlx.DB
}

// NewSQLRepository constructs a SQLRepository.
func NewSQLRepository(db *sqlx.DB) *SQLRepository {
	return &SQLRepository{db: db}
}

// CreateIdentity inserts a provider-subject mapping.
func (r *SQLRepository) CreateIdentity(ctx context.Context, identity *entity.Identity) error {
	query := `
		INSERT INTO auth_identities (id, user_id, provider, subject, email, created_at, updated_at)
		VALUES (:id, :user_id, :provider, :subject, :email, :created_at, :updated_at)
	`
	params := map[string]interface{}{
		"id":         identity.ID.String(),
		"user_id":    identity.UserID.String(),
		"provider":   identity.Provider,
		"subject":    identity.Subject,
		"email":      identity.Email,
		"created_at": identity.CreatedAt,
		"updated_at": identity.UpdatedAt,
	}

	if _, err := r.db.NamedExecContext(ctx, query, params); err != nil {
		if isDuplicateErr(err) {
			return repository.ErrIdentityExists
		}
		return err
	}
	return nil
}

// FindIdentity retrieves a provider-subject mapping.
func (r *SQLRepository) FindIdentity(ctx context.Context, provider, subject string) (*entity.Identity, error) {
	query := `
		SELECT id, user_id, provider, subject, email, created_at, updated_at
		FROM auth_identities
		WHERE provider = ? AND subject = ?
	`
	query = r.db.Rebind(query)
	var row sqlIdentity
	if err := r.db.GetContext(ctx, &row, query, provider, subject); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repository.ErrIdentityNotFound
		}
		return nil, err
	}
	return row.toEntity()
}

// FindIdentityByID retrieves an identity by its primary key.
func (r *SQLRepository) FindIdentityByID(ctx context.Context, id uuid.UUID) (*entity.Identity, error) {
	query := r.db.Rebind(`
		SELECT id, user_id, provider, subject, email, created_at, updated_at
		FROM auth_identities
		WHERE id = ?
	`)
	var row sqlIdentity
	if err := r.db.GetContext(ctx, &row, query, id.String()); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repository.ErrIdentityNotFound
		}
		return nil, err
	}
	return row.toEntity()
}

// CreateSession inserts a refresh session.
func (r *SQLRepository) CreateSession(ctx context.Context, session *entity.Session) error {
	query := `
		INSERT INTO auth_sessions (id, session_id, user_id, identity_id, token_hash, id_token, expires_at, used_at, revoked_at, created_at)
		VALUES (:id, :session_id, :user_id, :identity_id, :token_hash, :id_token, :expires_at, :used_at, :revoked_at, :created_at)
	`
	params := map[string]interface{}{
		"id":          session.ID.String(),
		"session_id":  session.SessionID.String(),
		"user_id":     session.UserID.String(),
		"identity_id": session.IdentityID.String(),
		"token_hash":  session.TokenHash,
		"id_token":    session.IDToken,
		"expires_at":  session.ExpiresAt,
		"used_at":     session.UsedAt,
		"revoked_at":  session.RevokedAt,
		"created_at":  session.CreatedAt,
	}

	if _, err := r.db.NamedExecContext(ctx, query, params); err != nil {
		return err
	}
	return nil
}

// FindSessionByHash retrieves a refresh session by its token hash.
func (r *SQLRepository) FindSessionByHash(ctx context.Context, tokenHash string) (*entity.Session, error) {
	query := r.db.Rebind(`
		SELECT id, session_id, user_id, identity_id, token_hash, id_token, expires_at, used_at, revoked_at, created_at
		FROM auth_sessions
		WHERE token_hash = ?
	`)
	var row sqlSession
	if err := r.db.GetContext(ctx, &row, query, tokenHash); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repository.ErrSessionNotFound
		}
		return nil, err
	}
	return row.toEntity()
}

// ConsumeSession marks an unused session as used, atomically.
func (r *SQLRepository) ConsumeSession(ctx context.Context, id uuid.UUID, usedAt time.Time) error {
	// Both predicates are concurrency guards: used_at stops two refreshes from
	// consuming one row, revoked_at stops a refresh that overlapped a logout.
	query := r.db.Rebind(`UPDATE auth_sessions SET used_at = ? WHERE id = ? AND used_at IS NULL AND revoked_at IS NULL`)
	result, err := r.db.ExecContext(ctx, query, usedAt, id.String())
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return repository.ErrSessionNotFound
	}
	return nil
}

// RevokeSessionFamily revokes every session sharing a rotation family.
func (r *SQLRepository) RevokeSessionFamily(ctx context.Context, sessionID uuid.UUID, revokedAt time.Time) error {
	query := r.db.Rebind(`UPDATE auth_sessions SET revoked_at = ? WHERE session_id = ? AND revoked_at IS NULL`)
	_, err := r.db.ExecContext(ctx, query, revokedAt, sessionID.String())
	return err
}

// DeleteExpiredSessions removes session families after their fixed deadline.
func (r *SQLRepository) DeleteExpiredSessions(ctx context.Context, before time.Time) error {
	query := r.db.Rebind(`DELETE FROM auth_sessions WHERE expires_at <= ?`)
	_, err := r.db.ExecContext(ctx, query, before)
	return err
}

type sqlSession struct {
	ID         string     `db:"id"`
	SessionID  string     `db:"session_id"`
	UserID     string     `db:"user_id"`
	IdentityID string     `db:"identity_id"`
	TokenHash  string     `db:"token_hash"`
	IDToken    string     `db:"id_token"`
	ExpiresAt  time.Time  `db:"expires_at"`
	UsedAt     *time.Time `db:"used_at"`
	RevokedAt  *time.Time `db:"revoked_at"`
	CreatedAt  time.Time  `db:"created_at"`
}

func (s sqlSession) toEntity() (*entity.Session, error) {
	ids := make([]uuid.UUID, 0, 4)
	for _, raw := range []string{s.ID, s.SessionID, s.UserID, s.IdentityID} {
		parsed, err := uuid.Parse(raw)
		if err != nil {
			return nil, err
		}
		ids = append(ids, parsed)
	}
	return &entity.Session{
		ID:         ids[0],
		SessionID:  ids[1],
		UserID:     ids[2],
		IdentityID: ids[3],
		TokenHash:  s.TokenHash,
		IDToken:    s.IDToken,
		ExpiresAt:  s.ExpiresAt,
		UsedAt:     s.UsedAt,
		RevokedAt:  s.RevokedAt,
		CreatedAt:  s.CreatedAt,
	}, nil
}

type sqlIdentity struct {
	ID        string    `db:"id"`
	UserID    string    `db:"user_id"`
	Provider  string    `db:"provider"`
	Subject   string    `db:"subject"`
	Email     string    `db:"email"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

func (i sqlIdentity) toEntity() (*entity.Identity, error) {
	id, err := uuid.Parse(i.ID)
	if err != nil {
		return nil, err
	}
	userID, err := uuid.Parse(i.UserID)
	if err != nil {
		return nil, err
	}
	return &entity.Identity{
		ID:        id,
		UserID:    userID,
		Provider:  i.Provider,
		Subject:   i.Subject,
		Email:     i.Email,
		CreatedAt: i.CreatedAt,
		UpdatedAt: i.UpdatedAt,
	}, nil
}

func isDuplicateErr(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "duplicate key value") || strings.Contains(msg, "Error 1062")
}
