package persistence

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/casper/go-fiber-clean-arch/internal/user/entity"
	"github.com/casper/go-fiber-clean-arch/internal/user/repository"
)

// SQLRepository persists users in PostgreSQL/MySQL databases.
type SQLRepository struct {
	db *sqlx.DB
}

// NewSQLRepository constructs a SQLRepository.
func NewSQLRepository(db *sqlx.DB) *SQLRepository {
	return &SQLRepository{db: db}
}

// Create inserts a user record.
func (r *SQLRepository) Create(ctx context.Context, user *entity.User) error {
	query := `
		INSERT INTO users (id, name, email, password_hash, created_at, updated_at)
		VALUES (:id, :name, :email, :password_hash, :created_at, :updated_at)
	`
	params := map[string]interface{}{
		"id":            user.ID,
		"name":          user.Name,
		"email":         user.Email,
		"password_hash": user.PasswordHash,
		"created_at":    user.CreatedAt,
		"updated_at":    user.UpdatedAt,
	}

	if _, err := r.db.NamedExecContext(ctx, query, params); err != nil {
		if isDuplicateErr(err) {
			return repository.ErrEmailExists
		}
		return err
	}
	return nil
}

// Update persists user updates.
func (r *SQLRepository) Update(ctx context.Context, user *entity.User) error {
	query := `
		UPDATE users
		SET name = :name, email = :email, password_hash = :password_hash, updated_at = :updated_at
		WHERE id = :id
	`
	params := map[string]interface{}{
		"id":            user.ID,
		"name":          user.Name,
		"email":         user.Email,
		"password_hash": user.PasswordHash,
		"updated_at":    user.UpdatedAt,
	}

	result, err := r.db.NamedExecContext(ctx, query, params)
	if err != nil {
		if isDuplicateErr(err) {
			return repository.ErrEmailExists
		}
		return err
	}
	affected, err := result.RowsAffected()
	if err == nil && affected == 0 {
		return repository.ErrNotFound
	}
	return err
}

// FindByID retrieves a user by identifier.
func (r *SQLRepository) FindByID(ctx context.Context, id uuid.UUID) (*entity.User, error) {
	query := `
		SELECT id, name, email, password_hash, created_at, updated_at
		FROM users
		WHERE id = ?
	`
	query = r.db.Rebind(query)
	var row sqlUser
	if err := r.db.GetContext(ctx, &row, query, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return row.toEntity(), nil
}

// FindByEmail retrieves a user by email address.
func (r *SQLRepository) FindByEmail(ctx context.Context, email string) (*entity.User, error) {
	query := `
		SELECT id, name, email, password_hash, created_at, updated_at
		FROM users
		WHERE email = ?
	`
	query = r.db.Rebind(query)
	var row sqlUser
	if err := r.db.GetContext(ctx, &row, query, strings.ToLower(email)); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return row.toEntity(), nil
}

// Delete removes a user by identifier.
func (r *SQLRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM users WHERE id = ?`
	query = r.db.Rebind(query)
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err == nil && affected == 0 {
		return repository.ErrNotFound
	}
	return err
}

type sqlUser struct {
	ID           uuid.UUID `db:"id"`
	Name         string    `db:"name"`
	Email        string    `db:"email"`
	PasswordHash string    `db:"password_hash"`
	CreatedAt    time.Time `db:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"`
}

func (u sqlUser) toEntity() *entity.User {
	return &entity.User{
		ID:           u.ID,
		Name:         u.Name,
		Email:        u.Email,
		PasswordHash: u.PasswordHash,
		CreatedAt:    u.CreatedAt,
		UpdatedAt:    u.UpdatedAt,
	}
}

func isDuplicateErr(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "duplicate key value") || strings.Contains(msg, "Error 1062")
}
