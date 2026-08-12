package http

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/gliedabrennung/sedna/internal/entity"
	"github.com/gliedabrennung/sedna/internal/usecase"
	"github.com/golang-jwt/jwt/v5"
)

const testJWTSecret = "router-test-secret-router-test-secret"

type stubAuthService struct{}

func (stubAuthService) Register(context.Context, string, string) (*entity.User, error) {
	return &entity.User{ID: 1, Username: "stub"}, nil
}

func (stubAuthService) Login(context.Context, string, string) (*entity.User, string, error) {
	return &entity.User{ID: 1, Username: "stub"}, "stub-token", nil
}

func (stubAuthService) Logout(context.Context, string, time.Time) error { return nil }

type stubUserService struct{}

func (stubUserService) SearchUsers(context.Context, string) ([]entity.User, error) {
	return []entity.User{{ID: 1, Username: "stub"}}, nil
}

func (stubUserService) GetUsersByIDs(context.Context, []int64) ([]entity.User, error) {
	return []entity.User{{ID: 1, Username: "stub"}}, nil
}

func routerUnderTest(t *testing.T) *server.Hertz {
	t.Helper()

	h := server.New(server.WithHandleMethodNotAllowed(true))
	SetupRouter(h, Deps{
		Auth:      stubAuthService{},
		Users:     stubUserService{},
		Messages:  usecase.NewMessageUseCase(nil),
		WsHandler: func(_ context.Context, c *app.RequestContext) { c.Status(http.StatusOK) },
		JWTSecret: testJWTSecret,
		Cookie:    CookieConfig{Name: "token", MaxAge: 3600},
	})
	return h
}

func validToken(t *testing.T) string {
	t.Helper()
	claims := jwt.RegisteredClaims{
		Subject:   "1",
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(testJWTSecret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return token
}

func authHeader(t *testing.T) ut.Header {
	return ut.Header{Key: "Authorization", Value: "Bearer " + validToken(t)}
}

func TestSetupRouter_Health(t *testing.T) {
	w := ut.PerformRequest(routerUnderTest(t).Engine, http.MethodGet, "/health", nil)
	if got := w.Result().StatusCode(); got != http.StatusOK {
		t.Errorf("GET /health: expected 200, got %d", got)
	}
}

func TestSetupRouter_ProtectedRoutesRequireAuth(t *testing.T) {
	paths := []string{"/users/search?q=abc", "/users/bulk?ids=1", "/users/me", "/messages?partner_id=2", "/ws"}

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			w := ut.PerformRequest(routerUnderTest(t).Engine, http.MethodGet, path, nil)
			if got := w.Result().StatusCode(); got != http.StatusUnauthorized {
				t.Errorf("expected 401 without a token, got %d", got)
			}
		})
	}
}

func TestSetupRouter_ProtectedRoutesAcceptToken(t *testing.T) {
	paths := []string{"/users/search?q=abc", "/users/bulk?ids=1", "/users/me"}

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			w := ut.PerformRequest(routerUnderTest(t).Engine, http.MethodGet, path, nil, authHeader(t))
			if got := w.Result().StatusCode(); got != http.StatusOK {
				t.Errorf("expected 200 with a valid token, got %d: %s", got, w.Result().Body())
			}
		})
	}
}

func TestSetupRouter_AuthRoutesRegistered(t *testing.T) {
	tests := []struct {
		path   string
		expect int
	}{
		{path: "/auth/register", expect: http.StatusCreated},
		{path: "/auth/login", expect: http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			body := &ut.Body{Body: strings.NewReader(`{"username":"stub","password":"password123"}`), Len: -1}
			w := ut.PerformRequest(routerUnderTest(t).Engine, http.MethodPost, tt.path, body,
				ut.Header{Key: "Content-Type", Value: "application/json"})
			if got := w.Result().StatusCode(); got != tt.expect {
				t.Errorf("expected %d, got %d: %s", tt.expect, got, w.Result().Body())
			}
		})
	}
}

func TestSetupRouter_UnknownAPIPathIsJSON404(t *testing.T) {
	paths := []string{"/auth/nope", "/users/nope", "/messages/nope", "/health/nope"}

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			w := ut.PerformRequest(routerUnderTest(t).Engine, http.MethodGet, path, nil)
			if got := w.Result().StatusCode(); got != http.StatusNotFound {
				t.Fatalf("expected 404, got %d", got)
			}
			if ct := string(w.Result().Header.ContentType()); !strings.Contains(ct, "application/json") {
				t.Errorf("expected a JSON error, got content-type %q", ct)
			}
		})
	}
}

func TestSetupRouter_NoMethod(t *testing.T) {
	w := ut.PerformRequest(routerUnderTest(t).Engine, http.MethodPost, "/health", nil)
	if got := w.Result().StatusCode(); got != consts.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", got)
	}
}
