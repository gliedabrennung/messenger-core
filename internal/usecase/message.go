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
	repo  domain.MessageRepository
	users domain.UserRepository
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

const maxChatList = 200

func (u *MessageUseCase) SetUsers(users domain.UserRepository) {
	u.users = users
}

func (u *MessageUseCase) Chats(ctx context.Context, userID int64) ([]*entity.Chat, error) {
	if !u.Available() {
		return []*entity.Chat{}, nil
	}

	chats, err := u.repo.ListChats(ctx, userID, maxChatList)
	if err != nil {
		return nil, fmt.Errorf("chats: %w", err)
	}
	if len(chats) == 0 {
		return []*entity.Chat{}, nil
	}

	if u.users == nil {
		return chats, nil
	}
	return u.hydratePeers(ctx, chats)
}

func (u *MessageUseCase) hydratePeers(ctx context.Context, chats []*entity.Chat) ([]*entity.Chat, error) {
	ids := make([]int64, 0, len(chats))
	for _, c := range chats {
		ids = append(ids, c.PeerID)
	}

	peers, err := u.users.GetByIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("chats: load peers: %w", err)
	}

	names := make(map[int64]string, len(peers))
	for _, p := range peers {
		names[p.ID] = p.Username
	}

	kept := make([]*entity.Chat, 0, len(chats))
	for _, c := range chats {
		name, ok := names[c.PeerID]
		if !ok {
			continue
		}
		c.PeerUsername = name
		kept = append(kept, c)
	}
	return kept, nil
}
