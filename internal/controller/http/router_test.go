package http

import (
	"context"
	"net/http"
	"testing"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/config"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/route"
)

func newTestEngine(t *testing.T) *route.Engine {
	t.Helper()
	opts := config.NewOptions([]config.Option{
		server.WithHandleMethodNotAllowed(true),
	})
	engine := route.NewEngine(opts)

	engine.GET("/", func(ctx context.Context, c *app.RequestContext) {
		c.Status(http.StatusOK)
	})
	engine.GET("/ws", func(ctx context.Context, c *app.RequestContext) {
		c.Status(http.StatusBadRequest)
	})
	engine.NoRoute(func(ctx context.Context, c *app.RequestContext) {
		c.Status(http.StatusNotFound)
	})
	engine.NoMethod(func(ctx context.Context, c *app.RequestContext) {
		c.Status(http.StatusMethodNotAllowed)
	})

	return engine
}

func TestSetupRouter_HomeRoute(t *testing.T) {
	w := ut.PerformRequest(newTestEngine(t), http.MethodGet, "/", nil)
	if got := w.Result().StatusCode(); got != http.StatusOK {
		t.Errorf("GET / : expected 200, got %d", got)
	}
}

func TestSetupRouter_WsRoute_Registered(t *testing.T) {
	w := ut.PerformRequest(newTestEngine(t), http.MethodGet, "/ws", nil)
	if got := w.Result().StatusCode(); got == http.StatusNotFound {
		t.Error("GET /ws : route not registered (got 404)")
	}
}

func TestSetupRouter_NoRoute(t *testing.T) {
	w := ut.PerformRequest(newTestEngine(t), http.MethodGet, "/nonexistent", nil)
	if got := w.Result().StatusCode(); got != http.StatusNotFound {
		t.Errorf("unknown path: expected 404, got %d", got)
	}
}

func TestSetupRouter_NoMethod(t *testing.T) {
	w := ut.PerformRequest(newTestEngine(t), http.MethodPost, "/", nil)
	if got := w.Result().StatusCode(); got != http.StatusMethodNotAllowed {
		t.Errorf("wrong method: expected 405, got %d", got)
	}
}
