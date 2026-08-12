package testutil

import (
	"context"
	"strings"

	"github.com/gliedabrennung/sedna/internal/apperr"
	"github.com/gliedabrennung/sedna/internal/entity"
)

type MockUserRepo struct {
	Users map[string]*entity.User

	ExistsErr error
}

func NewMockUserRepo() *MockUserRepo {
	return &MockUserRepo{Users: make(map[string]*entity.User)}
}

func (m *MockUserRepo) Create(_ context.Context, user *entity.User) error {
	if _, ok := m.Users[user.Username]; ok {
		return apperr.ErrUserAlreadyExists
	}
	user.ID = int64(len(m.Users) + 1)
	m.Users[user.Username] = user
	return nil
}

func (m *MockUserRepo) GetByUsername(_ context.Context, username string) (*entity.User, error) {
	user, ok := m.Users[username]
	if !ok {
		return nil, apperr.ErrUserNotFound
	}
	return user, nil
}

func (m *MockUserRepo) Search(_ context.Context, query string) ([]entity.User, error) {
	var result []entity.User
	for _, u := range m.Users {
		if strings.Contains(strings.ToLower(u.Username), strings.ToLower(query)) {
			result = append(result, *u)
		}
	}
	return result, nil
}

func (m *MockUserRepo) Exists(_ context.Context, userID int64) (bool, error) {
	if m.ExistsErr != nil {
		return false, m.ExistsErr
	}
	for _, u := range m.Users {
		if u.ID == userID {
			return true, nil
		}
	}
	return false, nil
}

func (m *MockUserRepo) GetByIDs(_ context.Context, ids []int64) ([]entity.User, error) {
	idSet := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		idSet[id] = struct{}{}
	}
	var result []entity.User
	for _, u := range m.Users {
		if _, ok := idSet[u.ID]; ok {
			result = append(result, *u)
		}
	}
	return result, nil
}
