package http

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/config"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/route"
	"github.com/gliedabrennung/sedna/internal/common/authctx"
	"github.com/gliedabrennung/sedna/internal/entity"
)

type mockMessageRepo struct {
	getChatHistory func(ctx context.Context, chatID string, limit int, cursor string) ([]*entity.Message, string, error)
	save           func(ctx context.Context, msg *entity.Message) error
	newMessageID   func() string
}

func (m *mockMessageRepo) NewMessageID() string {
	if m.newMessageID != nil {
		return m.newMessageID()
	}
	return "mock-id"
}

func (m *mockMessageRepo) Save(ctx context.Context, msg *entity.Message) error {
	if m.save != nil {
		return m.save(ctx, msg)
	}
	return nil
}

func (m *mockMessageRepo) GetChatHistory(ctx context.Context, chatID string, limit int, cursor string) ([]*entity.Message, string, error) {
	if m.getChatHistory != nil {
		return m.getChatHistory(ctx, chatID, limit, cursor)
	}
	return nil, "", nil
}

func TestMessageHandler_GetHistory(t *testing.T) {
	mockRepo := &mockMessageRepo{
		getChatHistory: func(ctx context.Context, chatID string, limit int, cursor string) ([]*entity.Message, string, error) {
			return []*entity.Message{{MessageID: "1", Content: "hello"}}, "next", nil
		},
	}
	h := NewMessageHandler(mockRepo)
	engine := route.NewEngine(config.NewOptions([]config.Option{}))
	engine.Use(func(c context.Context, ctx *app.RequestContext) {
		if string(ctx.Request.URI().Path()) == "/history_auth" {
			authctx.SetUserID(ctx, 1)
		}
		ctx.Next(c)
	})
	engine.GET("/history", h.GetHistory)
	engine.GET("/history_auth", h.GetHistory)

	t.Run("Success", func(t *testing.T) {
		w := ut.PerformRequest(engine, http.MethodGet, "/history_auth?partner_id=2", nil)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
		var resp historyResponse
		json.Unmarshal(w.Body.Bytes(), &resp)
		if len(resp.Messages) != 1 || resp.Messages[0].Content != "hello" || resp.NextCursor != "next" {
			t.Errorf("unexpected response: %v", string(w.Body.Bytes()))
		}
	})

	t.Run("Unauthorized", func(t *testing.T) {
		w := ut.PerformRequest(engine, http.MethodGet, "/history?partner_id=2", nil)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", w.Code)
		}
	})

	t.Run("NoPartnerID", func(t *testing.T) {
		w := ut.PerformRequest(engine, http.MethodGet, "/history_auth", nil)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
	})

	t.Run("InvalidPartnerID", func(t *testing.T) {
		w := ut.PerformRequest(engine, http.MethodGet, "/history_auth?partner_id=invalid", nil)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
	})
}
