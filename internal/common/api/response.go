package api

import (
	"context"
	"net/http"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/gliedabrennung/sedna/internal/common/logger"
)

const requestIDKey = "requestID"

type Error struct {
	Status    int    `json:"status"`
	Code      string `json:"code"`
	Message   string `json:"message"`
	Details   any    `json:"details,omitempty"`
	RequestID string `json:"request_id,omitempty"`
}

func SetRequestID(c *app.RequestContext, id string) {
	c.Set(requestIDKey, id)
}

func RequestID(c *app.RequestContext) string {
	id, _ := c.Value(requestIDKey).(string)
	return id
}

func ErrorResponse(c *app.RequestContext, status int, code string, message string, details any) {
	c.JSON(status, Error{
		Status:    status,
		Code:      code,
		Message:   message,
		Details:   details,
		RequestID: RequestID(c),
	})
}

func CustomErrorHandler() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		c.Next(ctx)
		if len(c.Errors) == 0 {
			return
		}

		err := c.Errors.Last()
		logger.CtxErrorf(ctx, "unhandled error: %v", err)

		if c.Response.StatusCode() != http.StatusOK || len(c.Response.Body()) > 0 {
			return
		}

		ErrorResponse(c, http.StatusInternalServerError,
			"INTERNAL_SERVER_ERROR", "internal server error", nil)
	}
}
