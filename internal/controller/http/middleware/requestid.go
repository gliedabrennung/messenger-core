package middleware

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/gliedabrennung/sedna/internal/common/api"
	"github.com/gliedabrennung/sedna/internal/common/reqid"
)

const requestIDHeader = "X-Request-Id"

func RequestID() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		id := string(c.GetHeader(requestIDHeader))
		if !reqid.Valid(id) {
			id = reqid.New()
		}

		api.SetRequestID(c, id)
		c.Header(requestIDHeader, id)
		c.Next(reqid.WithID(ctx, id))
	}
}
