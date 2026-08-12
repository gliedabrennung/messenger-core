package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/config"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/route"
)

func newEngine() *route.Engine {
	return route.NewEngine(config.NewOptions([]config.Option{}))
}

func decode(t *testing.T, body []byte) Error {
	t.Helper()
	var resp Error
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	return resp
}

func TestErrorResponse(t *testing.T) {
	engine := newEngine()
	engine.GET("/error", func(ctx context.Context, c *app.RequestContext) {
		SetRequestID(c, "assigned-id")
		ErrorResponse(c, http.StatusBadRequest, "TEST_CODE", "test message", "test details")
	})

	w := ut.PerformRequest(engine, http.MethodGet, "/error", nil)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}

	resp := decode(t, w.Body.Bytes())
	if resp.Code != "TEST_CODE" {
		t.Errorf("expected TEST_CODE, got %s", resp.Code)
	}
	if resp.Message != "test message" {
		t.Errorf("expected test message, got %s", resp.Message)
	}
	if resp.Details != "test details" {
		t.Errorf("expected test details, got %v", resp.Details)
	}
	if resp.RequestID != "assigned-id" {
		t.Errorf("expected assigned-id, got %s", resp.RequestID)
	}
}

func TestErrorResponse_IgnoresRawHeader(t *testing.T) {
	engine := newEngine()
	engine.GET("/error", func(ctx context.Context, c *app.RequestContext) {
		ErrorResponse(c, http.StatusBadRequest, "TEST", "msg", nil)
	})

	w := ut.PerformRequest(engine, http.MethodGet, "/error", nil,
		ut.Header{Key: "X-Request-Id", Value: "spoofed-by-client"})

	if got := decode(t, w.Body.Bytes()).RequestID; got != "" {
		t.Errorf("expected the unvalidated header to be ignored, got %q", got)
	}
}

func TestErrorResponse_NoRequestID(t *testing.T) {
	engine := newEngine()
	engine.GET("/error", func(ctx context.Context, c *app.RequestContext) {
		ErrorResponse(c, http.StatusBadRequest, "TEST", "msg", nil)
	})

	w := ut.PerformRequest(engine, http.MethodGet, "/error", nil)

	if got := decode(t, w.Body.Bytes()).RequestID; got != "" {
		t.Errorf("expected empty request_id, got %s", got)
	}
}

func TestCustomErrorHandler_WritesWhenHandlerDidNot(t *testing.T) {
	engine := newEngine()
	engine.Use(CustomErrorHandler())
	engine.GET("/boom", func(ctx context.Context, c *app.RequestContext) {
		c.Error(errors.New("test error"))
	})

	w := ut.PerformRequest(engine, http.MethodGet, "/boom", nil)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
	if got := decode(t, w.Body.Bytes()).Code; got != "INTERNAL_SERVER_ERROR" {
		t.Errorf("expected INTERNAL_SERVER_ERROR, got %s", got)
	}
}

func TestCustomErrorHandler_DoesNotLeakErrorText(t *testing.T) {
	engine := newEngine()
	engine.Use(CustomErrorHandler())
	engine.GET("/boom", func(ctx context.Context, c *app.RequestContext) {
		c.Error(errors.New("connection to 10.0.0.5:5432 refused"))
	})

	w := ut.PerformRequest(engine, http.MethodGet, "/boom", nil)

	if body := w.Body.String(); strings.Contains(body, "10.0.0.5") {
		t.Errorf("internal error text leaked to the client: %s", body)
	}
}

func TestCustomErrorHandler_KeepsExistingResponse(t *testing.T) {
	engine := newEngine()
	engine.Use(CustomErrorHandler())
	engine.GET("/partial", func(ctx context.Context, c *app.RequestContext) {
		c.JSON(http.StatusOK, map[string]string{"status": "done"})
		c.Error(errors.New("logged but already answered"))
	})

	w := ut.PerformRequest(engine, http.MethodGet, "/partial", nil)

	if w.Code != http.StatusOK {
		t.Errorf("expected the handler's 200 to survive, got %d", w.Code)
	}
	if body := w.Body.String(); !strings.Contains(body, `"status":"done"`) {
		t.Errorf("expected the handler's body to survive, got %s", body)
	}
}

func TestCustomErrorHandler_KeepsExistingErrorStatus(t *testing.T) {
	engine := newEngine()
	engine.Use(CustomErrorHandler())
	engine.GET("/notfound", func(ctx context.Context, c *app.RequestContext) {
		ErrorResponse(c, http.StatusNotFound, "NOT_FOUND", "nope", nil)
		c.Error(errors.New("also logged"))
	})

	w := ut.PerformRequest(engine, http.MethodGet, "/notfound", nil)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 to survive, got %d", w.Code)
	}
}
