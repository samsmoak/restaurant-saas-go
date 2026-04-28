package controller_test

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"restaurantsaas/internal/apps/reviews/controller"
	"restaurantsaas/internal/apps/reviews/model"
	reviewSvc "restaurantsaas/internal/apps/reviews/service"
	"restaurantsaas/internal/middleware"
)

type fakeSvc struct {
	createResult *model.Review
	createErr    error
	listResult   []*model.Review
	listErr      error
	lastCreated  *reviewSvc.CreateRequest
}

func (f *fakeSvc) Create(ctx context.Context, customerID primitive.ObjectID, req *reviewSvc.CreateRequest) (*model.Review, error) {
	f.lastCreated = req
	return f.createResult, f.createErr
}
func (f *fakeSvc) ListForRestaurant(ctx context.Context, restaurantID primitive.ObjectID, limit, offset int64) ([]*model.Review, error) {
	return f.listResult, f.listErr
}

func newMeApp(svc *fakeSvc, uid string) *fiber.App {
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		if uid != "" {
			c.Locals(middleware.LocalUserID, uid)
		}
		return c.Next()
	})
	ctl := controller.New(svc)
	ctl.RegisterMeRoutes(app.Group("/me"))
	return app
}

func newTenantApp(svc *fakeSvc, rid primitive.ObjectID) *fiber.App {
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		if !rid.IsZero() {
			c.Locals(middleware.LocalRestaurantID, rid)
		}
		return c.Next()
	})
	ctl := controller.New(svc)
	ctl.RegisterTenantRoutes(app.Group("/r/:restaurant_id/reviews"))
	return app
}

func TestCreate_HappyPath(t *testing.T) {
	uid := primitive.NewObjectID()
	rev := &model.Review{ID: primitive.NewObjectID(), Rating: 5}
	svc := &fakeSvc{createResult: rev}
	app := newMeApp(svc, uid.Hex())

	body := `{"order_id":"` + primitive.NewObjectID().Hex() + `","rating":5,"comment":"great"}`
	req := httptest.NewRequest("POST", "/me/reviews", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusCreated, resp.StatusCode)
	assert.Equal(t, 5, svc.lastCreated.Rating)
}

func TestCreate_AlreadyReviewed(t *testing.T) {
	uid := primitive.NewObjectID()
	svc := &fakeSvc{createErr: reviewSvc.ErrAlreadyReviewed}
	app := newMeApp(svc, uid.Hex())

	body := `{"order_id":"` + primitive.NewObjectID().Hex() + `","rating":5}`
	req := httptest.NewRequest("POST", "/me/reviews", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusConflict, resp.StatusCode)
}

func TestCreate_NotOwned(t *testing.T) {
	uid := primitive.NewObjectID()
	svc := &fakeSvc{createErr: reviewSvc.ErrOrderNotOwned}
	app := newMeApp(svc, uid.Hex())

	body := `{"order_id":"` + primitive.NewObjectID().Hex() + `","rating":5}`
	req := httptest.NewRequest("POST", "/me/reviews", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req, -1)
	assert.Equal(t, fiber.StatusForbidden, resp.StatusCode)
}

func TestList_HappyPath(t *testing.T) {
	rid := primitive.NewObjectID()
	svc := &fakeSvc{listResult: []*model.Review{{Rating: 5}}}
	app := newTenantApp(svc, rid)

	req := httptest.NewRequest("GET", "/r/"+rid.Hex()+"/reviews/?limit=10", nil)
	resp, err := app.Test(req, -1)
	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func TestList_MissingTenantContext(t *testing.T) {
	svc := &fakeSvc{}
	app := newTenantApp(svc, primitive.NilObjectID)

	req := httptest.NewRequest("GET", "/r/anything/reviews/", nil)
	resp, _ := app.Test(req, -1)
	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}
