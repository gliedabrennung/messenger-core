package middleware

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/gliedabrennung/sedna/internal/common/api"
	"github.com/gliedabrennung/sedna/internal/common/authctx"
	"github.com/gliedabrennung/sedna/internal/common/logger"
	"github.com/golang-jwt/jwt/v5"
)

type RevocationChecker interface {
	IsRevoked(ctx context.Context, tokenID string) (bool, error)
}

type JWTConfig struct {
	Secret     string
	CookieName string

	Revoked RevocationChecker
}

func JWTAuth(cfg JWTConfig) app.HandlerFunc {
	secretBytes := []byte(cfg.Secret)
	cookieName := cfg.CookieName
	if cookieName == "" {
		cookieName = "token"
	}

	return func(ctx context.Context, c *app.RequestContext) {
		var tokenStr string

		if t, ok := extractBearerToken(string(c.GetHeader("Authorization"))); ok {
			tokenStr = t
		} else if q := strings.TrimSpace(c.Query("token")); q != "" {
			tokenStr = q
		} else if cookie := string(c.Cookie(cookieName)); cookie != "" {
			tokenStr = cookie
		}

		if tokenStr == "" {
			api.ErrorResponse(c, http.StatusUnauthorized,
				"UNAUTHORIZED", "missing or malformed authorization header", nil)
			c.Abort()
			return
		}

		claims := &jwt.RegisteredClaims{}
		token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
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

		if cfg.Revoked != nil && claims.ID != "" {
			revoked, err := cfg.Revoked.IsRevoked(ctx, claims.ID)
			if err != nil {
				logger.CtxErrorf(ctx, "revocation lookup for token %s: %v", claims.ID, err)
			} else if revoked {
				api.ErrorResponse(c, http.StatusUnauthorized,
					"UNAUTHORIZED", "session has been ended", nil)
				c.Abort()
				return
			}
		}

		authctx.SetUserID(c, userID)
		authctx.SetTokenID(c, claims.ID)
		if claims.ExpiresAt != nil {
			authctx.SetTokenExp(c, claims.ExpiresAt.Time)
		}
		c.Next(ctx)
	}
}

func extractBearerToken(header string) (string, bool) {
	if !strings.HasPrefix(header, "Bearer ") {
		return "", false
	}
	token := strings.TrimSpace(header[7:])
	return token, token != ""
}
