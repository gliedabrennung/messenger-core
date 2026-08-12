package middleware

import (
	"context"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/config"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/route"
	"github.com/gliedabrennung/sedna/internal/common/authctx"
	"github.com/golang-jwt/jwt/v5"
)

func TestJWTAuth(t *testing.T) {
	secret := "testsecret"
	mw := JWTAuth(JWTConfig{Secret: secret})

	engine := route.NewEngine(config.NewOptions([]config.Option{}))
	engine.GET("/protected", mw, func(ctx context.Context, c *app.RequestContext) {
		userID, ok := authctx.UserID(c)
		if !ok {
			c.Status(http.StatusInternalServerError)
			return
		}
		c.JSON(http.StatusOK, map[string]int64{"user_id": userID})
	})

	t.Run("ValidToken", func(t *testing.T) {
		userID := int64(123)
		claims := jwt.RegisteredClaims{
			Subject:   strconv.FormatInt(userID, 10),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenStr, _ := token.SignedString([]byte(secret))

		w := ut.PerformRequest(engine, http.MethodGet, "/protected", nil,
			ut.Header{Key: "Authorization", Value: "Bearer " + tokenStr})

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
	})

	t.Run("ValidTokenInQuery", func(t *testing.T) {
		userID := int64(123)
		claims := jwt.RegisteredClaims{
			Subject:   strconv.FormatInt(userID, 10),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenStr, _ := token.SignedString([]byte(secret))

		w := ut.PerformRequest(engine, http.MethodGet, "/protected?token="+tokenStr, nil)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
	})

	t.Run("MissingToken", func(t *testing.T) {
		w := ut.PerformRequest(engine, http.MethodGet, "/protected", nil)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", w.Code)
		}
	})

	t.Run("InvalidToken", func(t *testing.T) {
		w := ut.PerformRequest(engine, http.MethodGet, "/protected", nil,
			ut.Header{Key: "Authorization", Value: "Bearer invalidtoken"})
		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", w.Code)
		}
	})

	t.Run("ExpiredToken", func(t *testing.T) {
		claims := jwt.RegisteredClaims{
			Subject:   "123",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
		}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenStr, _ := token.SignedString([]byte(secret))

		w := ut.PerformRequest(engine, http.MethodGet, "/protected", nil,
			ut.Header{Key: "Authorization", Value: "Bearer " + tokenStr})
		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", w.Code)
		}
	})

	t.Run("NoBearerPrefix", func(t *testing.T) {
		claims := jwt.RegisteredClaims{
			Subject:   "123",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenStr, _ := token.SignedString([]byte(secret))

		w := ut.PerformRequest(engine, http.MethodGet, "/protected", nil,
			ut.Header{Key: "Authorization", Value: tokenStr})
		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 for missing Bearer prefix, got %d", w.Code)
		}
	})
}

func TestExtractBearerToken(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   string
		ok     bool
	}{
		{"Valid", "Bearer abc123", "abc123", true},
		{"Empty", "", "", false},
		{"NoBearerPrefix", "abc123", "", false},
		{"BearerOnly", "Bearer ", "", false},
		{"BearerWithSpaces", "Bearer  abc123 ", "abc123", true},
		{"LowercaseBearer", "bearer abc123", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := extractBearerToken(tt.header)
			if ok != tt.ok {
				t.Errorf("ok = %v, want %v", ok, tt.ok)
			}
			if got != tt.want {
				t.Errorf("token = %q, want %q", got, tt.want)
			}
		})
	}
}
