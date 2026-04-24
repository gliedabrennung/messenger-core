package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/gliedabrennung/messenger-core/internal/entity"
	"github.com/gliedabrennung/messenger-core/internal/repository"
)

type mockUserRepo struct {
	users map[string]*entity.User
}

func (m *mockUserRepo) Create(ctx context.Context, user *entity.User) error {
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

func TestAuthUseCase_Register(t *testing.T) {
	repo := &mockUserRepo{users: make(map[string]*entity.User)}
	au := NewAuthUseCase(repo, "secret", time.Hour)

	ctx := context.Background()
	user, err := au.Register(ctx, "testuser", "password")
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if user.Username != "testuser" {
		t.Errorf("expected username testuser, got %s", user.Username)
	}

	_, err = au.Register(ctx, "testuser", "password")
	if err != ErrUserAlreadyExists {
		t.Errorf("expected ErrUserAlreadyExists, got %v", err)
	}
}

func TestAuthUseCase_Login(t *testing.T) {
	repo := &mockUserRepo{users: make(map[string]*entity.User)}
	au := NewAuthUseCase(repo, "secret", time.Hour)

	ctx := context.Background()
	_, _ = au.Register(ctx, "testuser", "password")

	t.Run("Success", func(t *testing.T) {
		user, token, err := au.Login(ctx, "testuser", "password")
		if err != nil {
			t.Fatalf("Login failed: %v", err)
		}
		if user.Username != "testuser" {
			t.Errorf("expected username testuser, got %s", user.Username)
		}
		if token == "" {
			t.Error("expected non-empty token")
		}
	})

	t.Run("InvalidPassword", func(t *testing.T) {
		_, _, err := au.Login(ctx, "testuser", "wrongpassword")
		if err != ErrInvalidCredentials {
			t.Errorf("expected ErrInvalidCredentials, got %v", err)
		}
	})

	t.Run("UserNotFound", func(t *testing.T) {
		_, _, err := au.Login(ctx, "nonexistent", "password")
		if err != ErrInvalidCredentials {
			t.Errorf("expected ErrInvalidCredentials, got %v", err)
		}
	})
}
