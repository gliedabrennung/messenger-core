package ws

import "time"

const (
	EventMessage = "message"
	EventAck     = "ack"
	EventError   = "error"
)

const (
	CodeInvalidPayload   = "INVALID_PAYLOAD"
	CodeMessageTooLong   = "MESSAGE_TOO_LONG"
	CodeInvalidRecipient = "INVALID_RECIPIENT"
	CodeUnknownRecipient = "UNKNOWN_RECIPIENT"
	CodeRateLimited      = "RATE_LIMITED"
	CodeMessageNotStored = "MESSAGE_NOT_STORED"
)

type Ack struct {
	Type      string    `json:"type"`
	ClientID  string    `json:"client_id,omitempty"`
	MessageID string    `json:"message_id"`
	To        int64     `json:"to"`
	CreatedAt time.Time `json:"created_at"`
}

func NewAck(clientID, messageID string, to int64, createdAt time.Time) Ack {
	return Ack{
		Type:      EventAck,
		ClientID:  clientID,
		MessageID: messageID,
		To:        to,
		CreatedAt: createdAt,
	}
}

type ErrorEvent struct {
	Type      string `json:"type"`
	Code      string `json:"code"`
	Message   string `json:"message"`
	ClientID  string `json:"client_id,omitempty"`
	MessageID string `json:"message_id,omitempty"`
	To        int64  `json:"to,omitempty"`
}

func NewErrorEvent(code, message, clientID string) ErrorEvent {
	return ErrorEvent{
		Type:     EventError,
		Code:     code,
		Message:  message,
		ClientID: clientID,
	}
}
