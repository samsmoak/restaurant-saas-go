package controller_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"

	"restaurantsaas/internal/apps/discovery/controller"
	"restaurantsaas/internal/apps/discovery/model"
	restaurantModel "restaurantsaas/internal/apps/restaurant/model"
)

type fakeSvc struct {
	listResults    []*model.RestaurantResult
	listTotal      int64
	searchResults  []*model.RestaurantResult
	searchTotal    int64
	suggestions    []string
	listErr        error
	searchErr      error
	suggestErr     error
	lastListParams model.ListParams
}

func (f *fakeSvc) List(ctx context.Context, p model.ListParams) ([]*model.RestaurantResult, int64, error) {
	f.lastListParams = p
	return f.listResults, f.listTotal, f.listErr
}
func (f *fakeSvc) Search(ctx context.Context, p model.ListParams) ([]*model.RestaurantResult, int64, error) {
	f.lastListParams = p
	return f.searchResults, f.searchTotal, f.searchErr
}
func (f *fakeSvc) Suggest(ctx context.Context, prefix string) ([]string, error) {
	return f.suggestions, f.suggestErr
}

func newApp(svc *fakeSvc) *fiber.App {
	app := fiber.New()
	ctl := controller.New(svc)
	ctl.RegisterRoutes(app.Group("/restaurants"))
	return app
}

func TestList_HappyPath(t *testing.T) {
	svc := &fakeSvc{
		listResults: []*model.RestaurantResult{
			{PublicView: &restaurantModel.PublicView{Name: "Pasta Place"}},
		},
		listTotal: 1,
	}
	app := newApp(svc)

	req := httptest.NewRequest("GET", "/restaurants?lat=40.7&lng=-74.0&limit=10", nil)
	resp, err := app.Test(req, -1)
	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var out map[string]interface{}
	_ = json.Unmarshal(body, &out)
	assert.EqualValues(t, 1, out["total"])
	assert.NotNil(t, svc.lastListParams.Lat)
	assert.NotNil(t, svc.lastListParams.Lng)
}

func TestList_ServiceError(t *testing.T) {
	svc := &fakeSvc{listErr: errors.New("db down")}
	app := newApp(svc)

	req := httptest.NewRequest("GET", "/restaurants", nil)
	resp, err := app.Test(req, -1)
	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

func TestSearch_HappyPath(t *testing.T) {
	svc := &fakeSvc{
		searchResults: []*model.RestaurantResult{
			{PublicView: &restaurantModel.PublicView{Name: "Sushi Bar"}},
		},
		searchTotal: 1,
	}
	app := newApp(svc)

	req := httptest.NewRequest("GET", "/restaurants/search?q=sushi", nil)
	resp, err := app.Test(req, -1)
	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	assert.Equal(t, "sushi", svc.lastListParams.Q)
}

func TestSuggest_HappyPath(t *testing.T) {
	svc := &fakeSvc{suggestions: []string{"sushi", "thai"}}
	app := newApp(svc)

	req := httptest.NewRequest("GET", "/restaurants/search/suggest?q=su", nil)
	resp, err := app.Test(req, -1)
	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var out map[string]interface{}
	_ = json.Unmarshal(body, &out)
	suggestions, ok := out["suggestions"].([]interface{})
	assert.True(t, ok)
	assert.Len(t, suggestions, 2)
}
