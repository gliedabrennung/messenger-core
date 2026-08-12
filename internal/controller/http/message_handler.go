package http

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/gliedabrennung/sedna/internal/common/api"
	"github.com/gliedabrennung/sedna/internal/common/authctx"
	"github.com/gliedabrennung/sedna/internal/common/logger"
	"github.com/gliedabrennung/sedna/internal/entity"
	"github.com/gliedabrennung/sedna/internal/usecase"
)

type MessageService interface {
	Available() bool
	History(ctx context.Context, userID, partnerID int64, limit int, cursor string) ([]*entity.Message, string, error)
}

type MessageHandler struct {
	messages MessageService
}

func NewMessageHandler(messages MessageService) *MessageHandler {
	return &MessageHandler{messages: messages}
}

type historyResponse struct {
	Messages   []*entity.Message `json:"messages"`
	NextCursor string            `json:"next_cursor"`
}

func (h *MessageHandler) GetHistory(ctx context.Context, c *app.RequestContext) {
	userID, ok := authctx.UserID(c)
	if !ok {
		api.ErrorResponse(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing user context", nil)
		return
	}

	partnerIDStr := c.Query("partner_id")
	if partnerIDStr == "" {
		api.ErrorResponse(c, http.StatusBadRequest, "INVALID_REQUEST", "partner_id is required", nil)
		return
	}

	partnerID, err := strconv.ParseInt(partnerIDStr, 10, 64)
	if err != nil {
		api.ErrorResponse(c, http.StatusBadRequest, "INVALID_REQUEST", "invalid partner_id", nil)
		return
	}

	limit := 0
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, convErr := strconv.Atoi(limitStr); convErr == nil {
			limit = l
		}
	}

	messages, nextCursor, err := h.messages.History(ctx, userID, partnerID, limit, c.Query("cursor"))
	if err != nil {
		if errors.Is(err, usecase.ErrInvalidPartner) {
			api.ErrorResponse(c, http.StatusBadRequest, "INVALID_REQUEST", "invalid partner_id", nil)
			return
		}
		logger.CtxErrorf(ctx, "failed to get chat history: %v", err)
		api.ErrorResponse(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to retrieve messages", nil)
		return
	}

	c.JSON(http.StatusOK, historyResponse{Messages: messages, NextCursor: nextCursor})
}
