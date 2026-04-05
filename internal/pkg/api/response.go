package api

import (
	"context"
	"net/http"

	"github.com/cloudwego/hertz/pkg/app"
)

type Error struct {
	Status    int         `json:"status"`
	Code      string      `json:"code"`
	Message   string      `json:"message"`
	Details   interface{} `json:"details,omitempty"`
	RequestID string      `json:"request_id,omitempty"`
}

func ErrorResponse(c *app.RequestContext, status int, code string, message string, details interface{}) {
	requestID := string(c.Response.Header.Peek("X-Request-Id"))
	resp := Error{
		Status:    status,
		Code:      code,
		Message:   message,
		Details:   details,
		RequestID: requestID,
	}
	c.JSON(status, resp)
}

func CustomErrorHandler() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		c.Next(ctx)
		if len(c.Errors) > 0 {
			err := c.Errors.Last()
			ErrorResponse(c, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", err.Error(), nil)
		}
	}
}
