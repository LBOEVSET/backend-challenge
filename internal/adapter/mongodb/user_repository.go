// Package mongodb provides the MongoDB adapter for port.UserRepository.
package mongodb

import (
	"context"
	"errors"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/lboevset/backend-challenge/internal/domain"
)

const collectionName = "users"

// userDocument is the MongoDB BSON representation of a User.
type userDocument struct {
	ID        string    `bson:"_id"`
	Name      string    `bson:"name"`
	Email     string    `bson:"email"`
	Role      string    `bson:"role"`
	Password  string    `bson:"password"`
	CreatedAt time.Time `bson:"created_at"`
}

func toDocument(u *domain.User) *userDocument {
	return &userDocument{
		ID:        u.ID,
		Name:      u.Name,
		Email:     u.Email,
		Role:      u.Role,
		Password:  u.Password,
		CreatedAt: u.CreatedAt,
	}
}

func toDomain(d *userDocument) *domain.User {
	return &domain.User{
		ID:        d.ID,
		Name:      d.Name,
		Email:     d.Email,
		Role:      d.Role,
		Password:  d.Password,
		CreatedAt: d.CreatedAt,
	}
}

// UserRepository is the MongoDB implementation of port.UserRepository.
type UserRepository struct {
	col *mongo.Collection
}

// NewUserRepository constructs a UserRepository, ensures a unique index on email,
// and migrates any existing users without a role to "admin".
func NewUserRepository(db *mongo.Database) (*UserRepository, error) {
	col := db.Collection(collectionName)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Unique index on email
	idx := mongo.IndexModel{
		Keys:    bson.D{primitive.E{Key: "email", Value: 1}},
		Options: options.Index().SetUnique(true),
	}
	if _, err := col.Indexes().CreateOne(ctx, idx); err != nil {
		return nil, err
	}

	// One-time migration: existing users without a role are treated as admins.
	filter := bson.M{"$or": bson.A{
		bson.M{"role": bson.M{"$exists": false}},
		bson.M{"role": ""},
	}}
	res, err := col.UpdateMany(ctx, filter, bson.M{"$set": bson.M{"role": domain.RoleAdmin}})
	if err != nil {
		log.Printf("[migration] failed to backfill admin role: %v", err)
	} else if res.ModifiedCount > 0 {
		log.Printf("[migration] assigned role=admin to %d existing users", res.ModifiedCount)
	}

	return &UserRepository{col: col}, nil
}

func (r *UserRepository) Create(ctx context.Context, user *domain.User) error {
	_, err := r.col.InsertOne(ctx, toDocument(user))
	return err
}

func (r *UserRepository) FindByID(ctx context.Context, id string) (*domain.User, error) {
	var doc userDocument
	err := r.col.FindOne(ctx, bson.M{"_id": id}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return toDomain(&doc), nil
}

func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	var doc userDocument
	err := r.col.FindOne(ctx, bson.M{"email": email}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return toDomain(&doc), nil
}

func (r *UserRepository) FindAll(ctx context.Context, limit, offset int64) ([]*domain.User, error) {
	opts := options.Find()
	if limit > 0 {
		opts.SetLimit(limit)
	}
	if offset > 0 {
		opts.SetSkip(offset)
	}
	cursor, err := r.col.Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var docs []userDocument
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, err
	}

	users := make([]*domain.User, len(docs))
	for i, d := range docs {
		users[i] = toDomain(&d)
	}
	return users, nil
}

func (r *UserRepository) Update(ctx context.Context, id string, fields domain.UpdateFields) (*domain.User, error) {
	set := bson.M{}
	if fields.Name != nil {
		set["name"] = *fields.Name
	}
	if fields.Email != nil {
		set["email"] = *fields.Email
	}
	if len(set) == 0 {
		return r.FindByID(ctx, id)
	}

	after := options.After
	opts := options.FindOneAndUpdate().SetReturnDocument(after)
	var doc userDocument
	err := r.col.FindOneAndUpdate(ctx, bson.M{"_id": id}, bson.M{"$set": set}, opts).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, errors.New("user not found")
	}
	if err != nil {
		return nil, err
	}
	return toDomain(&doc), nil
}

func (r *UserRepository) Delete(ctx context.Context, id string) error {
	res, err := r.col.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return errors.New("user not found")
	}
	return nil
}

func (r *UserRepository) Count(ctx context.Context) (int64, error) {
	return r.col.CountDocuments(ctx, bson.M{})
}
