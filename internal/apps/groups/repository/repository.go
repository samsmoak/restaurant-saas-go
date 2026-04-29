package repository

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"restaurantsaas/internal/apps/groups/model"
	mongoRepo "restaurantsaas/internal/database/abstractrepository/mongodb"
)

type GroupRepository struct {
	groups  *mongoRepo.MongoRepository[model.Group]
	members *mongoRepo.MongoRepository[model.GroupMember]
}

func NewGroupRepository(db *mongo.Database) *GroupRepository {
	return &GroupRepository{
		groups:  mongoRepo.NewMongoRepository[model.Group](db, "groups"),
		members: mongoRepo.NewMongoRepository[model.GroupMember](db, "group_members"),
	}
}

func (r *GroupRepository) GroupsCollection() *mongo.Collection { return r.groups.Collection }

func (r *GroupRepository) Create(ctx context.Context, g *model.Group) error {
	if g.CreatedAt.IsZero() {
		g.CreatedAt = time.Now().UTC()
	}
	_, err := r.groups.Create(ctx, g)
	return err
}

func (r *GroupRepository) FindByShareCode(ctx context.Context, code string) (*model.Group, error) {
	return r.groups.FindOne(ctx, bson.D{{Key: "share_code", Value: code}})
}

func (r *GroupRepository) ShareCodeTaken(ctx context.Context, code string) (bool, error) {
	g, err := r.FindByShareCode(ctx, code)
	if err != nil {
		return false, err
	}
	return g != nil, nil
}

func (r *GroupRepository) SetLockExpiry(ctx context.Context, id primitive.ObjectID, until time.Time) (*model.Group, error) {
	return r.groups.FindOneAndUpdate(ctx,
		bson.D{{Key: "_id", Value: id}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "lock_expires_at", Value: until}}}},
	)
}

func (r *GroupRepository) AddMember(ctx context.Context, m *model.GroupMember) error {
	if m.JoinedAt.IsZero() {
		m.JoinedAt = time.Now().UTC()
	}
	_, err := r.members.Collection.UpdateOne(ctx,
		bson.D{{Key: "group_id", Value: m.GroupID}, {Key: "user_id", Value: m.UserID}},
		bson.D{
			{Key: "$set", Value: bson.D{
				{Key: "name", Value: m.Name},
				{Key: "avatar_url", Value: m.AvatarURL},
			}},
			{Key: "$setOnInsert", Value: bson.D{
				{Key: "group_id", Value: m.GroupID},
				{Key: "user_id", Value: m.UserID},
				{Key: "joined_at", Value: m.JoinedAt},
				{Key: "status", Value: model.MemberStatusEditing},
				{Key: "lines", Value: bson.A{}},
				{Key: "subtotal_cents", Value: int64(0)},
			}},
		},
		options.Update().SetUpsert(true),
	)
	return err
}

func (r *GroupRepository) ListMembers(ctx context.Context, groupID primitive.ObjectID) ([]*model.GroupMember, error) {
	opts := options.Find().SetSort(bson.D{{Key: "joined_at", Value: 1}})
	return r.members.FindMany(ctx, bson.D{{Key: "group_id", Value: groupID}}, opts)
}
