package http

import (
	"net/http"
	"testing"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"
)

func TestSetupRouter(t *testing.T) {
	h := server.Default(server.WithHandleMethodNotAllowed(true))
	h.LoadHTMLFiles("/app/home.html")
	SetupRouter(h)

	w := ut.PerformRequest(h.Engine, "GET", "/", nil)
	if w.Code != http.StatusOK {
		t.Errorf("expected status %d for /, got %d", http.StatusOK, w.Code)
	}

	w = ut.PerformRequest(h.Engine, "GET", "/non-existent", nil)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d for non-existent, got %d", http.StatusNotFound, w.Code)
	}

	w = ut.PerformRequest(h.Engine, "POST", "/", nil)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status %d for POST /, got %d", http.StatusMethodNotAllowed, w.Code)
	}
}
