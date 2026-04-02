package models

import "time"

type MessageType string

const (
	TypeAuth    MessageType = "auth"
	TypeChat    MessageType = "chat"
	TypeSearch  MessageType = "search"
	TypeStatus  MessageType = "status"
)

type Message struct {
	Type      MessageType `json:"type"`
	From      string      `json:"from,omitempty"`
	To        string      `json:"to,omitempty"` // Пусто для broadcast
	Content   string      `json:"content"`
	Timestamp time.Time   `json:"timestamp"`
	Users     []string    `json:"users,omitempty"` // Для результатов поиска
}
