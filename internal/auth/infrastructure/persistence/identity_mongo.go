package persistence

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/casper/go-fiber-clean-arch/internal/auth/entity"
	"github.com/casper/go-fiber-clean-arch/internal/auth/repository"
)

// MongoRepository persists auth identities and refresh sessions in MongoDB.
type MongoRepository struct {
	collection  *mongo.Collection
	sessions    *mongo.Collection
	indexOnce   sync.Once
	indexErr    error
	sessionOnce sync.Once
	sessionErr  error
}

// NewMongoRepository constructs a MongoRepository.
func NewMongoRepository(db *mongo.Database) *MongoRepository {
	return &MongoRepository{
		collection: db.Collection("auth_identities"),
		sessions:   db.Collection("auth_sessions"),
	}
}

// CreateIdentity inserts a provider-subject mapping.
func (r *MongoRepository) CreateIdentity(ctx context.Context, identity *entity.Identity) error {
	if err := r.ensureIndexes(ctx); err != nil {
		return err
	}
	_, err := r.collection.InsertOne(ctx, mongoIdentityFromEntity(identity))
	if isDuplicate(err) {
		return repository.ErrIdentityExists
	}
	return err
}

// FindIdentity retrieves a provider-subject mapping.
func (r *MongoRepository) FindIdentity(ctx context.Context, provider, subject string) (*entity.Identity, error) {
	if err := r.ensureIndexes(ctx); err != nil {
		return nil, err
	}
	filter := bson.M{"provider": provider, "subject": subject}
	var doc mongoIdentity
	if err := r.collection.FindOne(ctx, filter).Decode(&doc); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, repository.ErrIdentityNotFound
		}
		return nil, err
	}
	return doc.toEntity()
}

func (r *MongoRepository) ensureIndexes(ctx context.Context) error {
	r.indexOnce.Do(func() {
		_, r.indexErr = r.collection.Indexes().CreateOne(ctx, mongo.IndexModel{
			Keys: bson.D{
				{Key: "provider", Value: 1},
				{Key: "subject", Value: 1},
			},
			Options: options.Index().SetUnique(true).SetName("ux_auth_identities_provider_subject"),
		})
	})
	return r.indexErr
}

// FindIdentityByID retrieves an identity by its primary key.
func (r *MongoRepository) FindIdentityByID(ctx context.Context, id uuid.UUID) (*entity.Identity, error) {
	var doc mongoIdentity
	if err := r.collection.FindOne(ctx, bson.M{"_id": id.String()}).Decode(&doc); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, repository.ErrIdentityNotFound
		}
		return nil, err
	}
	return doc.toEntity()
}

// CreateSession inserts a refresh session.
func (r *MongoRepository) CreateSession(ctx context.Context, session *entity.Session) error {
	if err := r.ensureSessionIndexes(ctx); err != nil {
		return err
	}
	_, err := r.sessions.InsertOne(ctx, mongoSessionFromEntity(session))
	return err
}

// FindSessionByHash retrieves a refresh session by its token hash.
func (r *MongoRepository) FindSessionByHash(ctx context.Context, tokenHash string) (*entity.Session, error) {
	if err := r.ensureSessionIndexes(ctx); err != nil {
		return nil, err
	}
	var doc mongoSession
	if err := r.sessions.FindOne(ctx, bson.M{"token_hash": tokenHash}).Decode(&doc); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, repository.ErrSessionNotFound
		}
		return nil, err
	}
	return doc.toEntity()
}

// ConsumeSession marks an unused session as used, atomically.
func (r *MongoRepository) ConsumeSession(ctx context.Context, id uuid.UUID, usedAt time.Time) error {
	// used_at/revoked_at nil are the concurrency guards, matching the SQL adapter: a
	// second concurrent refresh, or one overlapping a logout, matches no document.
	result, err := r.sessions.UpdateOne(ctx,
		bson.M{"_id": id.String(), "used_at": nil, "revoked_at": nil},
		bson.M{"$set": bson.M{"used_at": usedAt}},
	)
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return repository.ErrSessionNotFound
	}
	return nil
}

// RevokeSessionFamily revokes every session sharing a rotation family.
func (r *MongoRepository) RevokeSessionFamily(ctx context.Context, sessionID uuid.UUID, revokedAt time.Time) error {
	_, err := r.sessions.UpdateMany(ctx,
		bson.M{"session_id": sessionID.String(), "revoked_at": nil},
		bson.M{"$set": bson.M{"revoked_at": revokedAt}},
	)
	return err
}

// DeleteExpiredSessions removes session families after their fixed deadline.
func (r *MongoRepository) DeleteExpiredSessions(ctx context.Context, before time.Time) error {
	_, err := r.sessions.DeleteMany(ctx, bson.M{"expires_at": bson.M{"$lte": before}})
	return err
}

func (r *MongoRepository) ensureSessionIndexes(ctx context.Context) error {
	r.sessionOnce.Do(func() {
		_, r.sessionErr = r.sessions.Indexes().CreateMany(ctx, []mongo.IndexModel{
			{
				Keys:    bson.D{{Key: "token_hash", Value: 1}},
				Options: options.Index().SetUnique(true).SetName("ux_auth_sessions_token_hash"),
			},
			{
				Keys:    bson.D{{Key: "session_id", Value: 1}},
				Options: options.Index().SetName("ix_auth_sessions_session_id"),
			},
		})
	})
	return r.sessionErr
}

type mongoSession struct {
	ID         string     `bson:"_id"`
	SessionID  string     `bson:"session_id"`
	UserID     string     `bson:"user_id"`
	IdentityID string     `bson:"identity_id"`
	TokenHash  string     `bson:"token_hash"`
	IDToken    string     `bson:"id_token"`
	ExpiresAt  time.Time  `bson:"expires_at"`
	UsedAt     *time.Time `bson:"used_at"`
	RevokedAt  *time.Time `bson:"revoked_at"`
	CreatedAt  time.Time  `bson:"created_at"`
}

func mongoSessionFromEntity(s *entity.Session) mongoSession {
	return mongoSession{
		ID:         s.ID.String(),
		SessionID:  s.SessionID.String(),
		UserID:     s.UserID.String(),
		IdentityID: s.IdentityID.String(),
		TokenHash:  s.TokenHash,
		IDToken:    s.IDToken,
		ExpiresAt:  s.ExpiresAt,
		UsedAt:     s.UsedAt,
		RevokedAt:  s.RevokedAt,
		CreatedAt:  s.CreatedAt,
	}
}

func (s mongoSession) toEntity() (*entity.Session, error) {
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

type mongoIdentity struct {
	ID        string    `bson:"_id"`
	UserID    string    `bson:"user_id"`
	Provider  string    `bson:"provider"`
	Subject   string    `bson:"subject"`
	Email     string    `bson:"email"`
	CreatedAt time.Time `bson:"created_at"`
	UpdatedAt time.Time `bson:"updated_at"`
}

func mongoIdentityFromEntity(i *entity.Identity) mongoIdentity {
	return mongoIdentity{
		ID:        i.ID.String(),
		UserID:    i.UserID.String(),
		Provider:  i.Provider,
		Subject:   i.Subject,
		Email:     i.Email,
		CreatedAt: i.CreatedAt,
		UpdatedAt: i.UpdatedAt,
	}
}

func (i mongoIdentity) toEntity() (*entity.Identity, error) {
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

func isDuplicate(err error) bool {
	if err == nil {
		return false
	}
	var writeErr mongo.WriteException
	if errors.As(err, &writeErr) {
		for _, e := range writeErr.WriteErrors {
			if e.Code == 11000 {
				return true
			}
		}
	}
	var commandErr mongo.CommandError
	if errors.As(err, &commandErr) && commandErr.Code == 11000 {
		return true
	}
	return false
}
