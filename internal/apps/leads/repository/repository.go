package repository

import (
	"go.mongodb.org/mongo-driver/mongo"

	"restaurantsaas/internal/apps/leads/model"
	mongoRepo "restaurantsaas/internal/database/abstractrepository/mongodb"
)

type LeadsRepository struct {
	*mongoRepo.MongoRepository[model.Lead]
}

func NewLeadsRepository(db *mongo.Database) *LeadsRepository {
	return &LeadsRepository{
		MongoRepository: mongoRepo.NewMongoRepository[model.Lead](db, "leads"),
	}
}
