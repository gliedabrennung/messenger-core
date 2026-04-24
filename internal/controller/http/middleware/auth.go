package middleware

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/gliedabrennung/messenger-core/internal/pkg/api"
	"github.com/gliedabrennung/messenger-core/internal/pkg/authctx"
	"github.com/golang-jwt/jwt/v5"
)

func JWTAuth(secret string) app.HandlerFunc {
	secretBytes := []byte(secret)
	return func(ctx context.Context, c *app.RequestContext) {
		header := string(c.GetHeader("Authorization"))
		if header == "" {
			header = c.Query("token")
		}
		tokenStr, ok := extractBearerToken(header)
		if !ok {
			api.ErrorResponse(c, http.StatusUnauthorized,
				"UNAUTHORIZED", "missing or malformed authorization header", nil)
			c.Abort()
			return
		}

		claims := &jwt.RegisteredClaims{}
		token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrTokenUnverifiable
			}
			return secretBytes, nil
		})
		if err != nil || !token.Valid {
			api.ErrorResponse(c, http.StatusUnauthorized,
				"UNAUTHORIZED", "invalid or expired token", nil)
			c.Abort()
			return
		}

		userID, err := strconv.ParseInt(claims.Subject, 10, 64)
		if err != nil {
			api.ErrorResponse(c, http.StatusUnauthorized,
				"UNAUTHORIZED", "invalid subject claim", nil)
			c.Abort()
			return
		}

		authctx.SetUserID(c, userID)
		c.Next(ctx)
	}
}

func extractBearerToken(header string) (string, bool) {
	if header == "" {
		return "", false
	}
	if strings.HasPrefix(header, "Bearer ") {
		token := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
		return token, token != ""
	}
	trimmed := strings.TrimSpace(header)
	return trimmed, trimmed != ""
}

