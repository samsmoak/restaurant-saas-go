package repository

import (
	"context"
	"strings"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"restaurantsaas/internal/apps/promos/model"
	mongoRepo "restaurantsaas/internal/database/abstractrepository/mongodb"
)

type PromoRepository struct {
	*mongoRepo.MongoRepository[model.Promo]
}

func NewPromoRepository(db *mongo.Database) *PromoRepository {
	return &PromoRepository{
		MongoRepository: mongoRepo.NewMongoRepository[model.Promo](db, "promos"),
	}
}

// FindByCode returns the active promo whose normalised code matches.
// Codes are stored uppercase and looked up case-insensitively.
func (r *PromoRepository) FindByCode(ctx context.Context, code string) (*model.Promo, error) {
	c := strings.ToUpper(strings.TrimSpace(code))
	if c == "" {
		return nil, nil
	}
	return r.FindOne(ctx, bson.D{{Key: "code", Value: c}, {Key: "active", Value: true}})
}

// Upsert inserts the promo when missing, otherwise updates the
// percent_off / amount_off / active flags. Used by the boot-time
// seeder.
func (r *PromoRepository) Upsert(ctx context.Context, p *model.Promo) error {
	p.Code = strings.ToUpper(strings.TrimSpace(p.Code))
	if p.Code == "" {
		return nil
	}
	_, err := r.Collection.UpdateOne(ctx,
		bson.D{{Key: "code", Value: p.Code}},
		bson.D{
			{Key: "$set", Value: bson.D{
				{Key: "percent_off", Value: p.PercentOff},
				{Key: "amount_off_cents", Value: p.AmountOffCents},
				{Key: "min_subtotal_cents", Value: p.MinSubtotalCents},
				{Key: "expires_at", Value: p.ExpiresAt},
				{Key: "active", Value: p.Active},
			}},
			{Key: "$setOnInsert", Value: bson.D{
				{Key: "code", Value: p.Code},
				{Key: "created_at", Value: p.CreatedAt},
			}},
		},
		options.Update().SetUpsert(true),
	)
	return err
}
