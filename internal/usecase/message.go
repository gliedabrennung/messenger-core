package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/gliedabrennung/sedna/internal/domain"
	"github.com/gliedabrennung/sedna/internal/entity"
)

var ErrInvalidPartner = errors.New("invalid partner id")

const (
	defaultHistoryLimit = 50
	maxHistoryLimit     = 100
)

type MessageUseCase struct {
	repo domain.MessageRepository
}

func NewMessageUseCase(repo domain.MessageRepository) *MessageUseCase {
	return &MessageUseCase{repo: repo}
}

func (u *MessageUseCase) Available() bool {
	return u != nil && u.repo != nil
}

func (u *MessageUseCase) History(
	ctx context.Context, userID, partnerID int64, limit int, cursor string,
) ([]*entity.Message, string, error) {
	if !u.Available() {
		return []*entity.Message{}, "", nil
	}
	if partnerID <= 0 {
		return nil, "", ErrInvalidPartner
	}
	if limit <= 0 || limit > maxHistoryLimit {
		limit = defaultHistoryLimit
	}

	chatID := entity.MakeChatID(userID, partnerID)
	messages, next, err := u.repo.GetChatHistory(ctx, chatID, limit, cursor)
	if err != nil {
		return nil, "", fmt.Errorf("history: %w", err)
	}
	if messages == nil {
		messages = []*entity.Message{}
	}
	return messages, next, nil
}
