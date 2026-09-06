package persistence

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	// "go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	// "go.mongodb.org/mongo-driver/mongo/options"

	"github.com/casper/go-fiber-clean-arch/internal/user/entity"
	"github.com/casper/go-fiber-clean-arch/internal/user/repository"
)

// MongoRepository persists users in MongoDB.
type MongoRepository struct {
	collection *mongo.Collection
}

// NewMongoRepository constructs a MongoRepository.
func NewMongoRepository(db *mongo.Database) *MongoRepository {
	return &MongoRepository{
		collection: db.Collection("users"),
	}
}

// Create inserts a user document.
func (r *MongoRepository) Create(ctx context.Context, user *entity.User) error {
	doc := mongoUserFromEntity(user)
	_, err := r.collection.InsertOne(ctx, doc)
	if isDuplicate(err) {
		return repository.ErrEmailExists
	}
	return err
}

// FindByID retrieves a user by identifier.
func (r *MongoRepository) FindByID(ctx context.Context, id uuid.UUID) (*entity.User, error) {
	filter := bson.M{"_id": id.String()}
	var doc mongoUser
	if err := r.collection.FindOne(ctx, filter).Decode(&doc); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return doc.toEntity(), nil
}

// FindByEmail retrieves a user by email address.
func (r *MongoRepository) FindByEmail(ctx context.Context, email string) (*entity.User, error) {
	filter := bson.M{"email": email}
	var doc mongoUser
	if err := r.collection.FindOne(ctx, filter).Decode(&doc); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return doc.toEntity(), nil
}

type mongoUser struct {
	ID        string    `bson:"_id"`
	Name      string    `bson:"name"`
	Email     string    `bson:"email"`
	CreatedAt time.Time `bson:"created_at"`
	UpdatedAt time.Time `bson:"updated_at"`
}

func mongoUserFromEntity(u *entity.User) mongoUser {
	return mongoUser{
		ID:        u.ID.String(),
		Name:      u.Name,
		Email:     u.Email,
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
	}
}

func (u mongoUser) toEntity() *entity.User {
	id, _ := uuid.Parse(u.ID)
	return &entity.User{
		ID:        id,
		Name:      u.Name,
		Email:     u.Email,
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
	}
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
