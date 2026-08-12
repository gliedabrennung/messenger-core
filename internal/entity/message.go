package entity

import (
	"fmt"
	"time"
)

type Message struct {
	ChatID    string    `json:"chat_id"`
	MessageID string    `json:"message_id"`
	FromID    int64     `json:"from_id"`
	ToID      int64     `json:"to_id"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

func MakeChatID(userA, userB int64) string {
	if userA > userB {
		userA, userB = userB, userA
	}
	return fmt.Sprintf("%d:%d", userA, userB)
}

type Chat struct {
	ChatID       string    `json:"chat_id"`
	PeerID       int64     `json:"peer_id"`
	PeerUsername string    `json:"peer_username"`
	LastMessage  string    `json:"last_message"`
	LastFromID   int64     `json:"last_from_id"`
	LastActivity time.Time `json:"last_activity"`
}
