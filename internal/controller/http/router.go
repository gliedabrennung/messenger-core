package http

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/gliedabrennung/sedna/internal/common/api"
	"github.com/gliedabrennung/sedna/internal/controller/http/middleware"
)

type Deps struct {
	Auth      AuthService
	Users     UserService
	Messages  MessageService
	WsHandler app.HandlerFunc
	JWTSecret string
	Cookie    CookieConfig
	Revoked   middleware.RevocationChecker
}

const distDir = "./frontend/dist"

func SetupRouter(h *server.Hertz, deps Deps) {
	h.Use(middleware.RequestID(), api.CustomErrorHandler())

	authHandler := NewAuthHandler(deps.Auth, deps.Cookie)
	userHandler := NewUserHandler(deps.Users)
	authLimiter := middleware.NewRateLimiter(5, 10)
	authMiddleware := middleware.JWTAuth(middleware.JWTConfig{
		Secret:     deps.JWTSecret,
		CookieName: deps.Cookie.Name,
		Revoked:    deps.Revoked,
	})

	h.GET("/health", func(_ context.Context, c *app.RequestContext) {
		c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	auth := h.Group("/auth")
	auth.Use(authLimiter.Handler())
	auth.POST("/register", authHandler.Register)
	auth.POST("/login", authHandler.Login)

	auth.POST("/logout", authMiddleware, authHandler.Logout)

	searchLimiter := middleware.NewRateLimiter(2, 10)

	users := h.Group("/users", authMiddleware)
	users.GET("/search", searchLimiter.Handler(), userHandler.Search)
	users.GET("/bulk", userHandler.GetBulk)
	users.GET("/me", userHandler.Me)

	msgHandler := NewMessageHandler(deps.Messages)
	h.GET("/messages", authMiddleware, msgHandler.GetHistory)

	h.GET("/ws", authMiddleware, deps.WsHandler)

	h.GET("/assets/*filepath", func(ctx context.Context, c *app.RequestContext) {
		c.File(filepath.Join(distDir, string(c.Request.Path())))
	})
	h.GET("/favicon.svg", func(ctx context.Context, c *app.RequestContext) {
		c.File(filepath.Join(distDir, "favicon.svg"))
	})

	h.NoRoute(serveSPA)
	h.NoMethod(func(ctx context.Context, c *app.RequestContext) {
		api.ErrorResponse(c, http.StatusMethodNotAllowed,
			"METHOD_NOT_ALLOWED",
			"Method not allowed",
			nil)
	})
}

func serveSPA(_ context.Context, c *app.RequestContext) {
	path := string(c.Request.Path())

	if strings.HasPrefix(path, "/auth") ||
		strings.HasPrefix(path, "/users") ||
		strings.HasPrefix(path, "/messages") ||
		strings.HasPrefix(path, "/ws") ||
		strings.HasPrefix(path, "/health") {
		api.ErrorResponse(c, http.StatusNotFound,
			"NOT_FOUND",
			"Page not found",
			nil)
		return
	}

	tryPath := filepath.Join(distDir, path)
	if info, err := os.Stat(tryPath); err == nil && !info.IsDir() {
		c.File(tryPath)
		return
	}

	c.File(filepath.Join(distDir, "index.html"))
}
