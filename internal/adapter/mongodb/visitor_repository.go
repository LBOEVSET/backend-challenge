package mongodb

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/lboevset/backend-challenge/internal/domain"
)

const visitorCollection = "visitorRecord"

type VisitorRepository struct {
	col *mongo.Collection
}

// NewVisitorRepository creates the collection and ensures an index on (ip, user_agent)
// so upsert lookups are O(log n) instead of a full scan.
func NewVisitorRepository(db *mongo.Database) (*VisitorRepository, error) {
	col := db.Collection(visitorCollection)

	_, err := col.Indexes().CreateOne(context.Background(), mongo.IndexModel{
		Keys:    bson.D{{Key: "ip", Value: 1}, {Key: "user_agent", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	if err != nil {
		return nil, err
	}

	return &VisitorRepository{col: col}, nil
}

// Upsert inserts a new visitor document or, if the same IP+UA already exists,
// increments visit_count and refreshes last_visit. Never overwrites first_visit.
func (r *VisitorRepository) Upsert(ctx context.Context, v *domain.VisitorRecord) error {
	filter := bson.M{
		"ip":         v.IP,
		"user_agent": v.UserAgent,
	}
	update := bson.M{
		"$set": bson.M{
			"ip":         v.IP,
			"user_agent": v.UserAgent,
			"device":     v.Device,
			"os":         v.OS,
			"browser":    v.Browser,
			"last_visit": v.LastVisit,
		},
		"$inc": bson.M{
			"visit_count": 1,
		},
		"$setOnInsert": bson.M{
			"first_visit": v.FirstVisit, // only written on the very first insert
		},
	}

	_, err := r.col.UpdateOne(ctx, filter, update, options.Update().SetUpsert(true))
	return err
}
