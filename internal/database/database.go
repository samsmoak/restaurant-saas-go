package database

import (
	"context"
	"fmt"
	"log"
	"os"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
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
	// Collapse any legacy duplicate admin_users rows before the unique index on
	// (user_id, restaurant_id) is built — if dupes exist when CreateMany runs,
	// Mongo refuses to build the index and the error is only logged below,
	// leaving the constraint permanently absent in prod.
	dedupeAdminUsers(ctx, db)

	// Drop admin_users rows whose restaurant no longer exists. Restaurants
	// today are deleted directly in the DB with no cascade, so their
	// membership rows otherwise leak into /api/auth/memberships as orphans.
	pruneOrphanAdminUsers(ctx, db)

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

// dedupeAdminUsers collapses rows that share the same (user_id, restaurant_id)
// pair. For each duplicate group it keeps the row with the highest role
// precedence (owner > admin > staff) and deletes the rest. Errors are logged
// and swallowed — dedupe is best-effort so a failure here does not block boot.
func dedupeAdminUsers(ctx context.Context, db *mongo.Database) {
	coll := db.Collection("admin_users")
	pipeline := mongo.Pipeline{
		bson.D{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: bson.D{
				{Key: "user_id", Value: "$user_id"},
				{Key: "restaurant_id", Value: "$restaurant_id"},
			}},
			{Key: "rows", Value: bson.D{{Key: "$push", Value: bson.D{
				{Key: "id", Value: "$_id"},
				{Key: "role", Value: "$role"},
			}}}},
			{Key: "count", Value: bson.D{{Key: "$sum", Value: 1}}},
		}}},
		bson.D{{Key: "$match", Value: bson.D{{Key: "count", Value: bson.D{{Key: "$gt", Value: 1}}}}}},
	}
	cur, err := coll.Aggregate(ctx, pipeline)
	if err != nil {
		log.Printf("database.EnsureIndexes: admin_users dedupe aggregate: %v", err)
		return
	}
	defer cur.Close(ctx)

	var removed int64
	for cur.Next(ctx) {
		var group struct {
			Rows []struct {
				ID   primitive.ObjectID `bson:"id"`
				Role string             `bson:"role"`
			} `bson:"rows"`
		}
		if err := cur.Decode(&group); err != nil {
			log.Printf("database.EnsureIndexes: admin_users dedupe decode: %v", err)
			continue
		}
		if len(group.Rows) < 2 {
			continue
		}
		keep := 0
		for i := 1; i < len(group.Rows); i++ {
			if adminRolePrec(group.Rows[i].Role) > adminRolePrec(group.Rows[keep].Role) {
				keep = i
			}
		}
		toDelete := make([]primitive.ObjectID, 0, len(group.Rows)-1)
		for i, r := range group.Rows {
			if i == keep {
				continue
			}
			toDelete = append(toDelete, r.ID)
		}
		res, err := coll.DeleteMany(ctx, bson.D{{Key: "_id", Value: bson.D{{Key: "$in", Value: toDelete}}}})
		if err != nil {
			log.Printf("database.EnsureIndexes: admin_users dedupe delete: %v", err)
			continue
		}
		removed += res.DeletedCount
	}
	if removed > 0 {
		log.Printf("database.EnsureIndexes: removed %d duplicate admin_users rows", removed)
	}
}

// pruneOrphanAdminUsers deletes admin_users rows whose restaurant_id no longer
// exists in the restaurants collection. Best-effort; errors are logged.
func pruneOrphanAdminUsers(ctx context.Context, db *mongo.Database) {
	adminColl := db.Collection("admin_users")
	restColl := db.Collection("restaurants")

	rawIDs, err := adminColl.Distinct(ctx, "restaurant_id", bson.D{})
	if err != nil {
		log.Printf("database.EnsureIndexes: admin_users orphan scan: %v", err)
		return
	}
	if len(rawIDs) == 0 {
		return
	}
	refIDs := make([]primitive.ObjectID, 0, len(rawIDs))
	for _, v := range rawIDs {
		if oid, ok := v.(primitive.ObjectID); ok {
			refIDs = append(refIDs, oid)
		}
	}
	if len(refIDs) == 0 {
		return
	}

	cur, err := restColl.Find(ctx,
		bson.D{{Key: "_id", Value: bson.D{{Key: "$in", Value: refIDs}}}},
		options.Find().SetProjection(bson.D{{Key: "_id", Value: 1}}),
	)
	if err != nil {
		log.Printf("database.EnsureIndexes: restaurants existence scan: %v", err)
		return
	}
	defer cur.Close(ctx)

	existing := make(map[primitive.ObjectID]struct{}, len(refIDs))
	for cur.Next(ctx) {
		var doc struct {
			ID primitive.ObjectID `bson:"_id"`
		}
		if err := cur.Decode(&doc); err != nil {
			log.Printf("database.EnsureIndexes: restaurants scan decode: %v", err)
			continue
		}
		existing[doc.ID] = struct{}{}
	}

	orphans := make([]primitive.ObjectID, 0)
	for _, id := range refIDs {
		if _, ok := existing[id]; !ok {
			orphans = append(orphans, id)
		}
	}
	if len(orphans) == 0 {
		return
	}

	res, err := adminColl.DeleteMany(ctx,
		bson.D{{Key: "restaurant_id", Value: bson.D{{Key: "$in", Value: orphans}}}},
	)
	if err != nil {
		log.Printf("database.EnsureIndexes: admin_users orphan delete: %v", err)
		return
	}
	if res.DeletedCount > 0 {
		log.Printf("database.EnsureIndexes: removed %d orphan admin_users rows (referenced %d missing restaurants)", res.DeletedCount, len(orphans))
	}
}

func adminRolePrec(role string) int {
	switch role {
	case "owner":
		return 3
	case "admin":
		return 2
	default:
		return 1
	}
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
