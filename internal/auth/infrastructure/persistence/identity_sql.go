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
