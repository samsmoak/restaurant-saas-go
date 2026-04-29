package controller_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"restaurantsaas/internal/apps/ai/controller"
	"restaurantsaas/internal/apps/ai/parser"
	aiSvc "restaurantsaas/internal/apps/ai/service"
)

type fakeSvc struct {
	searchResp *aiSvc.SearchResponse
	searchErr  error
	chatResp   *aiSvc.ChatResponse
	chatErr    error

	lastSearch *aiSvc.SearchRequest
	lastChat   *aiSvc.ChatRequest
}

func (f *fakeSvc) Search(ctx context.Context, req *aiSvc.SearchRequest) (*aiSvc.SearchResponse, error) {
	f.lastSearch = req
	return f.searchResp, f.searchErr
}
func (f *fakeSvc) Chat(ctx context.Context, req *aiSvc.ChatRequest) (*aiSvc.ChatResponse, error) {
	f.lastChat = req
	return f.chatResp, f.chatErr
}

// New SSE-era methods on AIService — stubbed so the test fake
// satisfies the interface.  The controller_test only exercises the
// legacy POST /search path.
func (f *fakeSvc) ListDishes(ctx context.Context, query string, lat, lng *float64, taste *aiSvc.TasteFingerprint, filter aiSvc.DishFilter, limit int) []*aiSvc.Dish {
	return nil
}
func (f *fakeSvc) GetDishByID(ctx context.Context, id primitive.ObjectID) (*aiSvc.Dish, error) {
	return nil, nil
}
func (f *fakeSvc) Recommend(ctx context.Context, taste *aiSvc.TasteFingerprint, lat, lng *float64) []*aiSvc.Dish {
	return nil
}
func (f *fakeSvc) HydrateDishes(ctx context.Context, ids []primitive.ObjectID) []any { return nil }
func (f *fakeSvc) StreamChat(ctx context.Context, userID primitive.ObjectID, req *aiSvc.StreamChatRequest, emit func(any)) {
	emit(map[string]any{"type": "answer", "text": "ok", "sources": []any{}, "dishes": []any{}})
}

func newApp(svc *fakeSvc) *fiber.App {
	app := fiber.New()
	ctl := controller.New(svc)
	app.Post("/ai/search", ctl.Search)
	app.Post("/ai/chat", ctl.Chat)
	return app
}

// ── Search ────────────────────────────────────────────────────────────────

func TestSearch_HappyPath(t *testing.T) {
	svc := &fakeSvc{searchResp: &aiSvc.SearchResponse{
		Intent: parser.Intent{Cuisine: "thai", OriginalQuery: "thai food"},
	}}
	app := newApp(svc)

	body := `{"query":"thai food","lat":40.7,"lng":-74.0}`
	req := httptest.NewRequest("POST", "/ai/search", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	out, _ := io.ReadAll(resp.Body)
	var got map[string]any
	_ = json.Unmarshal(out, &got)
	intent, _ := got["intent"].(map[string]any)
	assert.Equal(t, "thai", intent["cuisine"])
	assert.Equal(t, "thai food", svc.lastSearch.Query)
}

func TestSearch_EmptyQueryServiceErrorIs400(t *testing.T) {
	svc := &fakeSvc{searchErr: errors.New("query is required")}
	app := newApp(svc)

	req := httptest.NewRequest("POST", "/ai/search", strings.NewReader(`{"query":""}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestSearch_InvalidJSONIs400(t *testing.T) {
	app := newApp(&fakeSvc{})
	req := httptest.NewRequest("POST", "/ai/search", strings.NewReader(`{not json`))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req, -1)
	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// ── Chat (SSE) ────────────────────────────────────────────────────────────

func TestChat_SSEContentType(t *testing.T) {
	app := newApp(&fakeSvc{})
	body := `{"message":"hello"}`
	req := httptest.NewRequest("POST", "/ai/chat", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))
}

func TestChat_InvalidJSONIs400(t *testing.T) {
	app := newApp(&fakeSvc{})
	req := httptest.NewRequest("POST", "/ai/chat", strings.NewReader(`{not json`))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req, -1)
	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}
