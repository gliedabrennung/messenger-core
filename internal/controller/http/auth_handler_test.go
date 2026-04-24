package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/common/config"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/route"
	"github.com/gliedabrennung/messenger-core/internal/entity"
	"github.com/gliedabrennung/messenger-core/internal/repository"
	"github.com/gliedabrennung/messenger-core/internal/usecase"
)

type mockUserRepo struct {
	users map[string]*entity.User
}

func (m *mockUserRepo) Create(ctx context.Context, user *entity.User) error {
	if _, ok := m.users[user.Username]; ok {
		return repository.ErrUserAlreadyExists
	}
	m.users[user.Username] = user
	user.ID = int64(len(m.users))
	return nil
}

func (m *mockUserRepo) GetByUsername(ctx context.Context, username string) (*entity.User, error) {
	user, ok := m.users[username]
	if !ok {
		return nil, repository.ErrUserNotFound
	}
	return user, nil
}

func setupAuthHandler(t *testing.T) (*route.Engine, *AuthHandler) {
	repo := &mockUserRepo{users: make(map[string]*entity.User)}
	au := usecase.NewAuthUseCase(repo, "secret", time.Hour)
	handler := NewAuthHandler(au)

	engine := route.NewEngine(config.NewOptions([]config.Option{}))
	engine.POST("/register", handler.Register)
	engine.POST("/login", handler.Login)

	return engine, handler
}

func TestAuthHandler_Register(t *testing.T) {
	engine, _ := setupAuthHandler(t)

	t.Run("Success", func(t *testing.T) {
		reqBody, _ := json.Marshal(map[string]string{
			"username": "testuser",
			"password": "password123",
		})
		w := ut.PerformRequest(engine, http.MethodPost, "/register", &ut.Body{Body: bytes.NewBuffer(reqBody), Len: len(reqBody)},
			ut.Header{Key: "Content-Type", Value: "application/json"})
		
		if w.Code != http.StatusCreated {
			t.Errorf("expected 201, got %d. Body: %s", w.Code, string(w.Body.Bytes()))
		}
	})

	t.Run("InvalidCredentials", func(t *testing.T) {
		reqBody, _ := json.Marshal(map[string]string{
			"username": "tu",
			"password": "p",
		})
		w := ut.PerformRequest(engine, http.MethodPost, "/register", &ut.Body{Body: bytes.NewBuffer(reqBody), Len: len(reqBody)},
			ut.Header{Key: "Content-Type", Value: "application/json"})
		
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
	})

	t.Run("UserExists", func(t *testing.T) {
		reqBody, _ := json.Marshal(map[string]string{
			"username": "existinguser",
			"password": "password123",
		})
		// Register once
		ut.PerformRequest(engine, http.MethodPost, "/register", &ut.Body{Body: bytes.NewBuffer(reqBody), Len: len(reqBody)},
			ut.Header{Key: "Content-Type", Value: "application/json"})
		// Register again
		w := ut.PerformRequest(engine, http.MethodPost, "/register", &ut.Body{Body: bytes.NewBuffer(reqBody), Len: len(reqBody)},
			ut.Header{Key: "Content-Type", Value: "application/json"})
		
		if w.Code != http.StatusConflict {
			t.Errorf("expected 409, got %d", w.Code)
		}
	})
}

func TestAuthHandler_Login(t *testing.T) {
	engine, _ := setupAuthHandler(t)

	// Pre-register user
	regBody, _ := json.Marshal(map[string]string{
		"username": "loginuser",
		"password": "password123",
	})
	ut.PerformRequest(engine, http.MethodPost, "/register", &ut.Body{Body: bytes.NewBuffer(regBody), Len: len(regBody)},
		ut.Header{Key: "Content-Type", Value: "application/json"})

	t.Run("Success", func(t *testing.T) {
		reqBody, _ := json.Marshal(map[string]string{
			"username": "loginuser",
			"password": "password123",
		})
		w := ut.PerformRequest(engine, http.MethodPost, "/login", &ut.Body{Body: bytes.NewBuffer(reqBody), Len: len(reqBody)},
			ut.Header{Key: "Content-Type", Value: "application/json"})
		
		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d. Body: %s", w.Code, string(w.Body.Bytes()))
		}

		var resp loginResponse
		json.Unmarshal(w.Body.Bytes(), &resp)
		if resp.Token == "" {
			t.Error("expected token in response")
		}
	})

	t.Run("InvalidCredentials", func(t *testing.T) {
		reqBody, _ := json.Marshal(map[string]string{
			"username": "loginuser",
			"password": "wrongpassword",
		})
		w := ut.PerformRequest(engine, http.MethodPost, "/login", &ut.Body{Body: bytes.NewBuffer(reqBody), Len: len(reqBody)},
			ut.Header{Key: "Content-Type", Value: "application/json"})
		
		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", w.Code)
		}
	})
}
