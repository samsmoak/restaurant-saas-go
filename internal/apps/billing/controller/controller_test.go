package controller_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"restaurantsaas/internal/apps/billing/controller"
	"restaurantsaas/internal/apps/billing/model"
	billingSvc "restaurantsaas/internal/apps/billing/service"
	"restaurantsaas/internal/middleware"
)

type fakeSvc struct {
	usage *billingSvc.UsageView
	tier  *billingSvc.TierView
}

func (f *fakeSvc) GetStatus(ctx context.Context, restaurantID primitive.ObjectID) (*model.Billing, error) {
	return nil, nil
}
func (f *fakeSvc) CreateSetupCheckout(ctx context.Context, restaurantID primitive.ObjectID, returnURL string) (string, error) {
	return "", nil
}
func (f *fakeSvc) CreateSubscriptionCheckout(ctx context.Context, restaurantID primitive.ObjectID, returnURL string) (string, error) {
	return "", nil
}
func (f *fakeSvc) CreatePortal(ctx context.Context, restaurantID primitive.ObjectID, returnURL string) (string, error) {
	return "", nil
}
func (f *fakeSvc) HandleWebhook(ctx context.Context, rawBody []byte, signature string) error {
	return nil
}
func (f *fakeSvc) RecordOrder(ctx context.Context, restaurantID primitive.ObjectID, fee float64) error {
	return nil
}
func (f *fakeSvc) GetUsage(ctx context.Context, restaurantID primitive.ObjectID) (*billingSvc.UsageView, error) {
	return f.usage, nil
}
func (f *fakeSvc) GetTier(ctx context.Context, restaurantID primitive.ObjectID) (*billingSvc.TierView, error) {
	return f.tier, nil
}

func newApp(svc *fakeSvc, rid primitive.ObjectID) *fiber.App {
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		if !rid.IsZero() {
			c.Locals(middleware.LocalRestaurantID, rid)
		}
		return c.Next()
	})
	ctl := controller.New(svc)
	ctl.RegisterRoutes(app.Group("/admin/billing"))
	return app
}

func TestGetUsage_HappyPath(t *testing.T) {
	rid := primitive.NewObjectID()
	now := time.Now().UTC()
	svc := &fakeSvc{usage: &billingSvc.UsageView{
		PeriodStart:      now,
		PeriodEnd:        now.AddDate(0, 1, 0),
		OrderCount:       100,
		PerOrderFeeTotal: 99.0,
		Tier:             1,
		BasePrice:        49,
		ProjectedTotal:   148.0,
		Currency:         "usd",
	}}
	app := newApp(svc, rid)

	req := httptest.NewRequest("GET", "/admin/billing/usage", nil)
	resp, err := app.Test(req, -1)
	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var out map[string]interface{}
	_ = json.Unmarshal(body, &out)
	assert.EqualValues(t, 100, out["order_count"])
	assert.EqualValues(t, 1, out["tier"])
	assert.EqualValues(t, 49, out["base_price"])
}

func TestGetTier_HappyPath(t *testing.T) {
	rid := primitive.NewObjectID()
	next := 251
	svc := &fakeSvc{tier: &billingSvc.TierView{
		CurrentTier:    1,
		BasePrice:      49,
		IncludesOrders: 250,
		NextTierAt:     &next,
		OrderCount:     10,
	}}
	app := newApp(svc, rid)

	req := httptest.NewRequest("GET", "/admin/billing/tier", nil)
	resp, err := app.Test(req, -1)
	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var out map[string]interface{}
	_ = json.Unmarshal(body, &out)
	assert.EqualValues(t, 1, out["current_tier"])
	assert.EqualValues(t, 251, out["next_tier_at"])
}

func TestGetUsage_MissingTenant(t *testing.T) {
	svc := &fakeSvc{}
	app := newApp(svc, primitive.NilObjectID)

	req := httptest.NewRequest("GET", "/admin/billing/usage", nil)
	resp, _ := app.Test(req, -1)
	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}
