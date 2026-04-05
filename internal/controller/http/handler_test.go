package http

import (
	"context"
	"net/http"
	"testing"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/config"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/route"
)

func TestServeHome_StatusOK(t *testing.T) {
	engine := route.NewEngine(config.NewOptions([]config.Option{}))

	engine.GET("/", func(ctx context.Context, c *app.RequestContext) {
		c.Status(http.StatusOK)
	})

	w := ut.PerformRequest(engine, http.MethodGet, "/", nil)
	if got := w.Result().StatusCode(); got != http.StatusOK {
		t.Errorf("expected 200, got %d", got)
	}
}
