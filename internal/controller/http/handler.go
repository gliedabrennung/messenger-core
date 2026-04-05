package http

import (
	"context"
	"net/http"

	"github.com/cloudwego/hertz/pkg/app"
)

func ServeHome(_ context.Context, c *app.RequestContext) {
	c.HTML(http.StatusOK, "static/index.html", nil)
}
