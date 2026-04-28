package controller_test

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"restaurantsaas/internal/apps/favorites/controller"
	favoriteSvc "restaurantsaas/internal/apps/favorites/service"
	"restaurantsaas/internal/middleware"
	restaurantModel "restaurantsaas/internal/apps/restaurant/model"
)

type fakeSvc struct {
	addErr     error
	removeErr  error
	listResult []*restaurantModel.PublicView
	listErr    error

	lastAddCustomer    primitive.ObjectID
	lastAddRestaurant  primitive.ObjectID
	lastRemoveCustomer primitive.ObjectID
}

func (f *fakeSvc) Add(ctx context.Context, customerID, restaurantID primitive.ObjectID) error {
	f.lastAddCustomer = customerID
	f.lastAddRestaurant = restaurantID
	return f.addErr
}
func (f *fakeSvc) Remove(ctx context.Context, customerID, restaurantID primitive.ObjectID) error {
	f.lastRemoveCustomer = customerID
	return f.removeErr
}
func (f *fakeSvc) List(ctx context.Context, customerID primitive.ObjectID) ([]*restaurantModel.PublicView, error) {
	return f.listResult, f.listErr
}

func newApp(svc *fakeSvc, signedInUserID string) *fiber.App {
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		if signedInUserID != "" {
			c.Locals(middleware.LocalUserID, signedInUserID)
		}
		return c.Next()
	})
	ctl := controller.New(svc)
	ctl.RegisterMeRoutes(app.Group("/me"))
	return app
}

func TestAdd_HappyPath(t *testing.T) {
	uid := primitive.NewObjectID()
	rid := primitive.NewObjectID()
	svc := &fakeSvc{}
	app := newApp(svc, uid.Hex())

	req := httptest.NewRequest("POST", "/me/favorites", strings.NewReader(`{"restaurant_id":"`+rid.Hex()+`"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	assert.Equal(t, uid, svc.lastAddCustomer)
	assert.Equal(t, rid, svc.lastAddRestaurant)
}

func TestAdd_RestaurantNotFound(t *testing.T) {
	uid := primitive.NewObjectID()
	rid := primitive.NewObjectID()
	svc := &fakeSvc{addErr: favoriteSvc.ErrRestaurantNotFound}
	app := newApp(svc, uid.Hex())

	req := httptest.NewRequest("POST", "/me/favorites", strings.NewReader(`{"restaurant_id":"`+rid.Hex()+`"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)
}

func TestAdd_Unauthenticated(t *testing.T) {
	svc := &fakeSvc{}
	app := newApp(svc, "")
	req := httptest.NewRequest("POST", "/me/favorites", strings.NewReader(`{"restaurant_id":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req, -1)
	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestRemove_HappyPath(t *testing.T) {
	uid := primitive.NewObjectID()
	rid := primitive.NewObjectID()
	svc := &fakeSvc{}
	app := newApp(svc, uid.Hex())

	req := httptest.NewRequest("DELETE", "/me/favorites/"+rid.Hex(), nil)
	resp, err := app.Test(req, -1)
	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusNoContent, resp.StatusCode)
}

func TestList_HappyPath(t *testing.T) {
	uid := primitive.NewObjectID()
	svc := &fakeSvc{listResult: []*restaurantModel.PublicView{{Name: "A"}}}
	app := newApp(svc, uid.Hex())

	req := httptest.NewRequest("GET", "/me/favorites", nil)
	resp, err := app.Test(req, -1)
	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func TestList_ServiceError(t *testing.T) {
	uid := primitive.NewObjectID()
	svc := &fakeSvc{listErr: errors.New("db down")}
	app := newApp(svc, uid.Hex())

	req := httptest.NewRequest("GET", "/me/favorites", nil)
	resp, err := app.Test(req, -1)
	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}
