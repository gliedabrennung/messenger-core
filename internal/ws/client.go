package ws

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/gliedabrennung/sedna/internal/common/api"
	"github.com/gliedabrennung/sedna/internal/common/authctx"
	"github.com/gliedabrennung/sedna/internal/common/logger"
	"github.com/gliedabrennung/sedna/internal/entity"
	"github.com/hertz-contrib/websocket"
)

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = (pongWait * 9) / 10

	maxTextLength  = 4000
	maxMessageSize = 4*maxTextLength + 1024

	sendRate  = 10
	sendBurst = 20

	rateNoticeInterval = time.Second

	maxKnownRecipients = 256

	maxClientIDLength = 64

	recipientLookupTimeout = 3 * time.Second
)

type Client struct {
	hub      *Hub
	id       int64
	conn     *websocket.Conn
	send     chan []byte
	done     chan struct{}
	tokenExp time.Time

	limiter        *tokenBucket
	lastRateNotice time.Time

	knownRecipients map[int64]struct{}
}

type incomingMessage struct {
	To       json.Number `json:"to"`
	Message  string      `json:"message"`
	ClientID string      `json:"client_id"`
}

func (c *Client) nextReadDeadline() time.Time {
	deadline := time.Now().Add(pongWait)
	if !c.tokenExp.IsZero() && c.tokenExp.Before(deadline) {
		return c.tokenExp
	}
	return deadline
}

func (c *Client) readPump(ctx context.Context) {
	defer func() {
		c.hub.Unregister(c)
		<-c.done
	}()

	c.conn.SetReadLimit(maxMessageSize)
	if err := c.conn.SetReadDeadline(c.nextReadDeadline()); err != nil {
		logger.Errorf("ws: set read deadline: %v", err)
		return
	}
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(c.nextReadDeadline())
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logger.Errorf("ws: read error: %v", err)
			}
			break
		}

		if !c.handleMessage(ctx, message) {
			return
		}
	}
}

func (c *Client) handleMessage(ctx context.Context, raw []byte) bool {
	var inc incomingMessage
	if err := json.Unmarshal(raw, &inc); err != nil {
		c.reject(CodeInvalidPayload, "malformed message", "", 0)
		return true
	}

	clientID := inc.ClientID
	if len(clientID) > maxClientIDLength {
		clientID = clientID[:maxClientIDLength]
	}

	toID, toErr := inc.To.Int64()

	now := time.Now()
	if !c.limiter.allow(now) {
		if now.Sub(c.lastRateNotice) >= rateNoticeInterval {
			c.lastRateNotice = now
			c.reject(CodeRateLimited, "sending too fast", clientID, toID)
		}
		return true
	}

	if toErr != nil {
		c.reject(CodeInvalidPayload, "invalid recipient id", clientID, 0)
		return true
	}

	inc.Message = strings.TrimSpace(inc.Message)
	if inc.Message == "" {
		c.reject(CodeInvalidPayload, "message is empty", clientID, toID)
		return true
	}
	if utf8.RuneCountInString(inc.Message) > maxTextLength {
		c.reject(CodeMessageTooLong,
			"message exceeds "+strconv.Itoa(maxTextLength)+" characters", clientID, toID)
		return true
	}

	if !c.checkRecipient(ctx, toID, clientID) {
		return true
	}

	msg := DirectMessage{
		Type:      EventMessage,
		MessageID: c.hub.NewMessageID(),
		From:      c.id,
		To:        toID,
		Message:   inc.Message,
		CreatedAt: time.Now().UTC(),
	}

	c.hub.Persist(&entity.Message{
		ChatID:    entity.MakeChatID(c.id, toID),
		MessageID: msg.MessageID,
		FromID:    c.id,
		ToID:      toID,
		Content:   inc.Message,
		CreatedAt: msg.CreatedAt,
	}, clientID)

	c.hub.NotifySender(c.id, NewAck(clientID, msg.MessageID, toID, msg.CreatedAt))

	return c.hub.Send(msg)
}

func (c *Client) checkRecipient(ctx context.Context, toID int64, clientID string) bool {
	if toID <= 0 {
		c.reject(CodeInvalidRecipient, "invalid recipient id", clientID, toID)
		return false
	}
	if toID == c.id {
		c.reject(CodeInvalidRecipient, "cannot send a message to yourself", clientID, toID)
		return false
	}
	if _, ok := c.knownRecipients[toID]; ok {
		return true
	}

	lookupCtx, cancel := context.WithTimeout(ctx, recipientLookupTimeout)
	defer cancel()
	if !c.hub.RecipientExists(lookupCtx, toID) {
		c.reject(CodeUnknownRecipient, "recipient does not exist", clientID, toID)
		return false
	}

	if len(c.knownRecipients) >= maxKnownRecipients {
		clear(c.knownRecipients)
	}
	c.knownRecipients[toID] = struct{}{}
	return true
}

func (c *Client) reject(code, message, clientID string, toID int64) {
	ev := NewErrorEvent(code, message, clientID)
	ev.To = toID
	c.hub.NotifySender(c.id, ev)
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
		close(c.done)
	}()

	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				logger.Errorf("ws: set write deadline: %v", err)
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			if err := c.conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				logger.Errorf("ws: set write deadline (ping): %v", err)
				return
			}
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func NewUpgrader(allowedOrigins []string) websocket.HertzUpgrader {
	allowed := make(map[string]bool, len(allowedOrigins))
	wildcard := false
	for _, o := range allowedOrigins {
		o = strings.TrimSpace(o)
		if o == "" {
			continue
		}
		if o == "*" {
			wildcard = true
			continue
		}
		allowed[normalizeOrigin(o)] = true
	}

	return websocket.HertzUpgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(ctx *app.RequestContext) bool {
			origin := string(ctx.GetHeader("Origin"))

			if origin == "" {
				return true
			}
			if wildcard {
				return true
			}
			if len(allowed) > 0 {
				return allowed[normalizeOrigin(origin)]
			}
			return sameOrigin(origin, string(ctx.Host()))
		},
	}
}

func normalizeOrigin(origin string) string {
	return strings.ToLower(strings.TrimSuffix(strings.TrimSpace(origin), "/"))
}

func sameOrigin(origin, host string) bool {
	if host == "" {
		return false
	}
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return false
	}
	return strings.EqualFold(u.Host, host)
}

func ServeWs(hub *Hub, upgrader websocket.HertzUpgrader) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		userID, ok := authctx.UserID(c)
		if !ok {
			api.ErrorResponse(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing user context", nil)
			return
		}

		tokenExp, _ := authctx.TokenExp(c)

		err := upgrader.Upgrade(c, func(conn *websocket.Conn) {
			client := &Client{
				hub:             hub,
				id:              userID,
				conn:            conn,
				send:            make(chan []byte, 256),
				done:            make(chan struct{}),
				tokenExp:        tokenExp,
				limiter:         newTokenBucket(sendRate, sendBurst),
				knownRecipients: make(map[int64]struct{}),
			}

			hub.Register(client)

			go client.writePump()

			client.readPump(hub.context())
		})

		if err != nil {
			logger.CtxErrorf(ctx, "ws: upgrade error: %v", err)
			api.ErrorResponse(c, http.StatusInternalServerError,
				"WEBSOCKET_UPGRADE_FAILED", "could not upgrade to websocket connection", nil)
		}
	}
}
