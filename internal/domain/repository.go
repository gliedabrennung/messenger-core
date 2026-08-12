package domain

import (
	"context"

	"github.com/gliedabrennung/sedna/internal/entity"
)

type UserRepository interface {
	Create(ctx context.Context, user *entity.User) error
	GetByUsername(ctx context.Context, username string) (*entity.User, error)
	Search(ctx context.Context, query string) ([]entity.User, error)
	GetByIDs(ctx context.Context, ids []int64) ([]entity.User, error)
	Exists(ctx context.Context, userID int64) (bool, error)
}

type MessageRepository interface {
	NewMessageID() string
	Save(ctx context.Context, msg *entity.Message) error
	GetChatHistory(ctx context.Context, chatID string, limit int, cursor string) ([]*entity.Message, string, error)
}
