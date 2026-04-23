package database

import (
	"context"
	"fmt"
	"log"
	"os"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func Connect(ctx context.Context) (*mongo.Client, *mongo.Database, error) {
	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		return nil, nil, fmt.Errorf("MONGO_URI is required")
	}
	name := os.Getenv("MONGO_DB_NAME")
	if name == "" {
		return nil, nil, fmt.Errorf("MONGO_DB_NAME is required")
	}
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, nil, fmt.Errorf("database.Connect: %w", err)
	}
	if err := client.Ping(ctx, nil); err != nil {
		return nil, nil, fmt.Errorf("database.Connect ping: %w", err)
	}
	return client, client.Database(name), nil
}

func EnsureIndexes(ctx context.Context, db *mongo.Database) error {
	specs := map[string][]mongo.IndexModel{
		"users": {
			{Keys: bson.D{{Key: "email", Value: 1}}, Options: options.Index().SetUnique(true)},
			{
				Keys: bson.D{{Key: "google_sub", Value: 1}},
				Options: options.Index().
					SetUnique(true).
					SetPartialFilterExpression(bson.D{{Key: "google_sub", Value: bson.D{{Key: "$exists", Value: true}}}}),
			},
		},
		"customer_profiles": {
			{Keys: bson.D{{Key: "user_id", Value: 1}}, Options: options.Index().SetUnique(true)},
		},
		"restaurants": {
			{Keys: bson.D{{Key: "slug", Value: 1}}, Options: options.Index().SetUnique(true)},
			{Keys: bson.D{{Key: "owner_id", Value: 1}}},
		},
		"admin_users": {
			{
				Keys:    bson.D{{Key: "user_id", Value: 1}, {Key: "restaurant_id", Value: 1}},
				Options: options.Index().SetUnique(true),
			},
			{Keys: bson.D{{Key: "restaurant_id", Value: 1}}},
		},
		"admin_invites": {
			{Keys: bson.D{{Key: "code", Value: 1}}, Options: options.Index().SetUnique(true)},
			{Keys: bson.D{{Key: "restaurant_id", Value: 1}}},
		},
		"categories": {
			{Keys: bson.D{{Key: "restaurant_id", Value: 1}, {Key: "display_order", Value: 1}}},
		},
		"menu_items": {
			{Keys: bson.D{{Key: "restaurant_id", Value: 1}, {Key: "category_id", Value: 1}}},
			{Keys: bson.D{{Key: "restaurant_id", Value: 1}, {Key: "display_order", Value: 1}}},
		},
		"orders": {
			{Keys: bson.D{{Key: "order_number", Value: 1}}, Options: options.Index().SetUnique(true)},
			{Keys: bson.D{{Key: "restaurant_id", Value: 1}, {Key: "created_at", Value: -1}}},
			{Keys: bson.D{{Key: "restaurant_id", Value: 1}, {Key: "status", Value: 1}, {Key: "created_at", Value: -1}}},
			{Keys: bson.D{{Key: "customer_id", Value: 1}, {Key: "created_at", Value: -1}}},
			{Keys: bson.D{{Key: "payment_intent_id", Value: 1}}},
		},
	}
	for coll, models := range specs {
		if _, err := db.Collection(coll).Indexes().CreateMany(ctx, models); err != nil {
			log.Printf("database.EnsureIndexes: %s: %v", coll, err)
		}
	}
	return nil
}
