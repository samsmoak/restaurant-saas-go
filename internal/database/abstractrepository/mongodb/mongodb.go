package mongoRepo

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoRepository[T any] struct {
	Collection *mongo.Collection
}

func NewMongoRepository[T any](db *mongo.Database, collection string) *MongoRepository[T] {
	return &MongoRepository[T]{Collection: db.Collection(collection)}
}

func idFilter(id any) (bson.D, error) {
	switch v := id.(type) {
	case primitive.ObjectID:
		return bson.D{{Key: "_id", Value: v}}, nil
	case string:
		if v == "" {
			return nil, errors.New("empty id")
		}
		if oid, err := primitive.ObjectIDFromHex(v); err == nil {
			return bson.D{{Key: "_id", Value: oid}}, nil
		}
		return bson.D{{Key: "_id", Value: v}}, nil
	default:
		return nil, fmt.Errorf("unsupported id type %T", id)
	}
}

func (r *MongoRepository[T]) GetByID(ctx context.Context, id any) (*T, error) {
	filter, err := idFilter(id)
	if err != nil {
		return nil, fmt.Errorf("MongoRepository.GetByID: %w", err)
	}
	var out T
	err = r.Collection.FindOne(ctx, filter).Decode(&out)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("MongoRepository.GetByID: %w", err)
	}
	return &out, nil
}

func (r *MongoRepository[T]) GetAll(ctx context.Context, opts ...*options.FindOptions) ([]*T, error) {
	return r.FindMany(ctx, bson.D{}, opts...)
}

func (r *MongoRepository[T]) FindOne(ctx context.Context, filter any, opts ...*options.FindOneOptions) (*T, error) {
	var out T
	err := r.Collection.FindOne(ctx, filter, opts...).Decode(&out)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("MongoRepository.FindOne: %w", err)
	}
	return &out, nil
}

func (r *MongoRepository[T]) FindMany(ctx context.Context, filter any, opts ...*options.FindOptions) ([]*T, error) {
	cur, err := r.Collection.Find(ctx, filter, opts...)
	if err != nil {
		return nil, fmt.Errorf("MongoRepository.FindMany: %w", err)
	}
	defer cur.Close(ctx)
	out := make([]*T, 0)
	for cur.Next(ctx) {
		var item T
		if err := cur.Decode(&item); err != nil {
			return nil, fmt.Errorf("MongoRepository.FindMany: %w", err)
		}
		out = append(out, &item)
	}
	if err := cur.Err(); err != nil {
		return nil, fmt.Errorf("MongoRepository.FindMany: %w", err)
	}
	return out, nil
}

func (r *MongoRepository[T]) Create(ctx context.Context, doc *T) (*T, error) {
	res, err := r.Collection.InsertOne(ctx, doc)
	if err != nil {
		return nil, fmt.Errorf("MongoRepository.Create: %w", err)
	}
	setIDField(doc, res.InsertedID)
	return doc, nil
}

func (r *MongoRepository[T]) Update(ctx context.Context, id any, update any) (*T, error) {
	filter, err := idFilter(id)
	if err != nil {
		return nil, fmt.Errorf("MongoRepository.Update: %w", err)
	}
	_, err = r.Collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return nil, fmt.Errorf("MongoRepository.Update: %w", err)
	}
	return r.GetByID(ctx, id)
}

func (r *MongoRepository[T]) FindOneAndUpdate(ctx context.Context, filter any, update any, opts ...*options.FindOneAndUpdateOptions) (*T, error) {
	allOpts := append([]*options.FindOneAndUpdateOptions{
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	}, opts...)
	var out T
	err := r.Collection.FindOneAndUpdate(ctx, filter, update, allOpts...).Decode(&out)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("MongoRepository.FindOneAndUpdate: %w", err)
	}
	return &out, nil
}

func (r *MongoRepository[T]) Delete(ctx context.Context, id any) error {
	filter, err := idFilter(id)
	if err != nil {
		return fmt.Errorf("MongoRepository.Delete: %w", err)
	}
	_, err = r.Collection.DeleteOne(ctx, filter)
	if err != nil {
		return fmt.Errorf("MongoRepository.Delete: %w", err)
	}
	return nil
}

func setIDField(doc any, insertedID any) {
	v := reflect.ValueOf(doc)
	if v.Kind() != reflect.Ptr || v.IsNil() {
		return
	}
	v = v.Elem()
	if v.Kind() != reflect.Struct {
		return
	}
	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		tag := t.Field(i).Tag.Get("bson")
		if tag == "" {
			continue
		}
		if name := primaryBsonName(tag); name == "_id" {
			field := v.Field(i)
			if !field.CanSet() {
				return
			}
			iv := reflect.ValueOf(insertedID)
			if iv.Type().AssignableTo(field.Type()) {
				field.Set(iv)
			}
			return
		}
	}
}

func primaryBsonName(tag string) string {
	for i := 0; i < len(tag); i++ {
		if tag[i] == ',' {
			return tag[:i]
		}
	}
	return tag
}
