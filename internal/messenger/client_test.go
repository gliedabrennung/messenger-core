package messenger

import (
	"context"
	"net/http"
	"testing"

	"github.com/cloudwego/hertz/pkg/app"
)

func TestServeWs_Fail(t *testing.T) {
	c := app.NewContext(0)
	c.Request.Header.Set("Connection", "keep-alive")

	ServeWs(context.Background(), c)

	if c.Response.StatusCode() != http.StatusInternalServerError {
		t.Errorf("expected status %d for failed upgrade, got %d", http.StatusInternalServerError, c.Response.StatusCode())
	}
}
