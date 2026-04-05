package api

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/cloudwego/hertz/pkg/app"
)

func TestErrorResponse(t *testing.T) {
	c := app.NewContext(0)
	c.Response.Header.Set("X-Request-Id", "test-req-id")

	ErrorResponse(c, http.StatusBadRequest, "BAD_REQUEST", "invalid input", "detail info")

	if c.Response.StatusCode() != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, c.Response.StatusCode())
	}

	var resp Error
	err := json.Unmarshal(c.Response.Body(), &resp)
	if err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.Status != http.StatusBadRequest {
		t.Errorf("expected status field %d, got %d", http.StatusBadRequest, resp.Status)
	}
	if resp.Code != "BAD_REQUEST" {
		t.Errorf("expected code %s, got %s", "BAD_REQUEST", resp.Code)
	}
	if resp.Message != "invalid input" {
		t.Errorf("expected message %s, got %s", "invalid input", resp.Message)
	}
	if resp.Details != "detail info" {
		t.Errorf("expected details %v, got %v", "detail info", resp.Details)
	}
	if resp.RequestID != "test-req-id" {
		t.Errorf("expected request_id %s, got %s", "test-req-id", resp.RequestID)
	}
}

func TestCustomErrorHandler(t *testing.T) {
	h := CustomErrorHandler()
	c := app.NewContext(0)

	h(context.Background(), c)
	if c.Response.StatusCode() != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, c.Response.StatusCode())
	}

	c.Error(http.ErrAbortHandler)
	h(context.Background(), c)
	if c.Response.StatusCode() != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, c.Response.StatusCode())
	}

	var resp Error
	err := json.Unmarshal(c.Response.Body(), &resp)
	if err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.Code != "INTERNAL_SERVER_ERROR" {
		t.Errorf("expected code %s, got %s", "INTERNAL_SERVER_ERROR", resp.Code)
	}
}
