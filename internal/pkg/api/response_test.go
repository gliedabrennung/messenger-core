package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/config"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/route"
)

func TestErrorResponse(t *testing.T) {
	engine := route.NewEngine(config.NewOptions([]config.Option{}))
	engine.GET("/error", func(ctx context.Context, c *app.RequestContext) {
		c.Response.Header.Set("X-Request-Id", "test-id")
		ErrorResponse(c, http.StatusBadRequest, "TEST_CODE", "test message", "test details")
	})

	w := ut.PerformRequest(engine, http.MethodGet, "/error", nil)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}

	var resp Error
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Code != "TEST_CODE" {
		t.Errorf("expected TEST_CODE, got %s", resp.Code)
	}
	if resp.Message != "test message" {
		t.Errorf("expected test message, got %s", resp.Message)
	}
	if resp.Details != "test details" {
		t.Errorf("expected test details, got %v", resp.Details)
	}
	if resp.RequestID != "test-id" {
		t.Errorf("expected test-id, got %s", resp.RequestID)
	}
}

func TestCustomErrorHandler(t *testing.T) {
	engine := route.NewEngine(config.NewOptions([]config.Option{}))
	engine.Use(CustomErrorHandler())
	engine.GET("/panic", func(ctx context.Context, c *app.RequestContext) {
		c.Error(errors.New("test error"))
	})

	w := ut.PerformRequest(engine, http.MethodGet, "/panic", nil)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}
