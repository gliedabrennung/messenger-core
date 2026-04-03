package web

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
		api.ErrorResponse(ctx, c, http.StatusNotFound, "NOT_FOUND", "Страница не найдена", nil)
	})
	h.NoMethod(func(ctx context.Context, c *app.RequestContext) {
		api.ErrorResponse(ctx, c, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Метод не поддерживается", nil)
	})

	h.GET("/", ServeHome)
	h.GET("/ws", messenger.ServeWs)
}
