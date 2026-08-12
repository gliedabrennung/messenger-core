package http

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/gliedabrennung/sedna/internal/common/api"
	"github.com/gliedabrennung/sedna/internal/common/authctx"
	"github.com/gliedabrennung/sedna/internal/common/logger"
	"github.com/gliedabrennung/sedna/internal/entity"
)

type UserService interface {
	SearchUsers(ctx context.Context, query string) ([]entity.User, error)
	GetUsersByIDs(ctx context.Context, ids []int64) ([]entity.User, error)
}

type UserHandler struct {
	users UserService
}

func NewUserHandler(users UserService) *UserHandler {
	return &UserHandler{users: users}
}

const (
	minSearchLength = 3
	maxSearchLength = 24

	maxBulkIDs = 100
)

func (h *UserHandler) Search(ctx context.Context, c *app.RequestContext) {
	q := strings.TrimSpace(c.Query("q"))
	if utf8.RuneCountInString(q) < minSearchLength {
		api.ErrorResponse(c, http.StatusBadRequest, "INVALID_REQUEST",
			"query parameter 'q' must be at least 3 characters", nil)
		return
	}
	if utf8.RuneCountInString(q) > maxSearchLength {
		api.ErrorResponse(c, http.StatusBadRequest, "INVALID_REQUEST",
			"query parameter 'q' is too long", nil)
		return
	}

	users, err := h.users.SearchUsers(ctx, q)
	if err != nil {
		logger.CtxErrorf(ctx, "search failed: %v", err)
		api.ErrorResponse(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to search users", nil)
		return
	}
	if users == nil {
		users = []entity.User{}
	}

	c.JSON(http.StatusOK, users)
}

func (h *UserHandler) GetBulk(ctx context.Context, c *app.RequestContext) {
	idsStr := c.Query("ids")
	if idsStr == "" {
		c.JSON(http.StatusOK, []entity.User{})
		return
	}

	parts := strings.Split(idsStr, ",")
	if len(parts) > maxBulkIDs {
		api.ErrorResponse(c, http.StatusBadRequest, "INVALID_REQUEST",
			"too many ids requested", nil)
		return
	}

	ids := make([]int64, 0, len(parts))
	for _, p := range parts {
		id, err := strconv.ParseInt(strings.TrimSpace(p), 10, 64)
		if err != nil {
			api.ErrorResponse(c, http.StatusBadRequest, "INVALID_REQUEST",
				"ids must be a comma separated list of integers", nil)
			return
		}
		ids = append(ids, id)
	}

	users, err := h.users.GetUsersByIDs(ctx, ids)
	if err != nil {
		logger.CtxErrorf(ctx, "get bulk failed: %v", err)
		api.ErrorResponse(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get users", nil)
		return
	}
	if users == nil {
		users = []entity.User{}
	}

	c.JSON(http.StatusOK, users)
}

func (h *UserHandler) Me(ctx context.Context, c *app.RequestContext) {
	userID, ok := authctx.UserID(c)
	if !ok {
		api.ErrorResponse(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing user context", nil)
		return
	}

	users, err := h.users.GetUsersByIDs(ctx, []int64{userID})
	if err != nil || len(users) == 0 {
		api.ErrorResponse(c, http.StatusUnauthorized, "UNAUTHORIZED", "user not found", nil)
		return
	}

	c.JSON(http.StatusOK, users[0])
}
