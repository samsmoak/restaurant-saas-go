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
	// The controller surfaces service errors as 400 (the service returns an
	// error only for bad input — graceful degradation otherwise).
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

// ── Chat ─────────────────────────────────────────────────────────────────

func TestChat_FallbackReplyIs200(t *testing.T) {
	// Service contract: when LLM is unavailable, return 200 with a fallback
	// reply (not 5xx). The controller forwards verbatim.
	svc := &fakeSvc{chatResp: &aiSvc.ChatResponse{
		Reply: "AI is unavailable right now — please try again later.",
	}}
	app := newApp(svc)

	body := `{"messages":[{"role":"user","content":"hello"}]}`
	req := httptest.NewRequest("POST", "/ai/chat", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	out, _ := io.ReadAll(resp.Body)
	var got map[string]any
	_ = json.Unmarshal(out, &got)
	assert.Contains(t, got["reply"], "unavailable")
}

func TestChat_HappyPath(t *testing.T) {
	svc := &fakeSvc{chatResp: &aiSvc.ChatResponse{Reply: "Try the green curry."}}
	app := newApp(svc)

	body := `{"messages":[{"role":"user","content":"recommend thai"}]}`
	req := httptest.NewRequest("POST", "/ai/chat", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	assert.Equal(t, "user", svc.lastChat.Messages[0].Role)
}

func TestChat_BadShapeIs400(t *testing.T) {
	// An empty messages array is the only error the chat service surfaces;
	// fakeSvc forwards it as a 400 via the controller's BadRequest path.
	svc := &fakeSvc{chatErr: errors.New("at least one message is required")}
	app := newApp(svc)

	req := httptest.NewRequest("POST", "/ai/chat", strings.NewReader(`{"messages":[]}`))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req, -1)
	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestChat_InvalidJSONIs400(t *testing.T) {
	app := newApp(&fakeSvc{})
	req := httptest.NewRequest("POST", "/ai/chat", strings.NewReader(`{not json`))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req, -1)
	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}
