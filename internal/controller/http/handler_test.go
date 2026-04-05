package http

import (
	"net/http"
	"testing"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"
)

func TestServeHome(t *testing.T) {
	h := server.Default()
	h.LoadHTMLFiles("/app/home.html")
	h.GET("/", ServeHome)

	w := ut.PerformRequest(h.Engine, "GET", "/", nil)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}
