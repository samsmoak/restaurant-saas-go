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

	// Drop legacy `slug` unique index if it still exists from a prior deploy.
	// We no longer use slug — the restaurant's ObjectID is the tenant key.
	if err := dropLegacyIndex(ctx, db, "restaurants", "slug_1"); err != nil {
		log.Printf("database.EnsureIndexes: drop restaurants.slug_1: %v", err)
	}
	return nil
}

// dropLegacyIndex removes an index by name if it exists; ignores "not found"
// errors so this is safe to call on fresh clusters.
func dropLegacyIndex(ctx context.Context, db *mongo.Database, collection, indexName string) error {
	_, err := db.Collection(collection).Indexes().DropOne(ctx, indexName)
	if err == nil {
		log.Printf("database: dropped legacy index %s.%s", collection, indexName)
		return nil
	}
	// IndexNotFound is error code 27; also tolerate NamespaceNotFound (26).
	if mongo.IsDuplicateKeyError(err) {
		return nil
	}
	// Swallow if the error message indicates the index doesn't exist.
	msg := err.Error()
	if contains(msg, "index not found") || contains(msg, "IndexNotFound") ||
		contains(msg, "ns does not exist") || contains(msg, "NamespaceNotFound") {
		return nil
	}
	return err
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
