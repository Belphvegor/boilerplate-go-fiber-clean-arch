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

// MongoRepository persists auth identities in MongoDB.
type MongoRepository struct {
	collection *mongo.Collection
	indexOnce  sync.Once
	indexErr   error
}

// NewMongoRepository constructs a MongoRepository.
func NewMongoRepository(db *mongo.Database) *MongoRepository {
	return &MongoRepository{
		collection: db.Collection("auth_identities"),
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
