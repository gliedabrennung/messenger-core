package http

import (
	"context"

	"net/http"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/gliedabrennung/messenger-core/internal/messenger"
	"github.com/gliedabrennung/messenger-core/internal/pkg/api"
)

func SetupRouter(h *server.Hertz) {
	h.Use(api.CustomErrorHandler())
	h.NoRoute(func(ctx context.Context, c *app.RequestContext) {
		api.ErrorResponse(c, http.StatusNotFound,
			"NOT_FOUND",
			"Page not found",
			nil)
	})
	h.NoMethod(func(ctx context.Context, c *app.RequestContext) {
		api.ErrorResponse(c, http.StatusMethodNotAllowed,
			"METHOD_NOT_ALLOWED",
			"Method not allowed",
			nil)
	})

	h.GET("/", ServeHome)
	h.GET("/ws", messenger.ServeWs)
}
