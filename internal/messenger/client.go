package messenger

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/gliedabrennung/messenger-core/internal/pkg/api"
	"github.com/gliedabrennung/messenger-core/internal/pkg/authctx"
	"github.com/hertz-contrib/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 1024
)

var upgrader = websocket.HertzUpgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(ctx *app.RequestContext) bool {
		return true
	},
}

type Client struct {
	id   int64
	conn *websocket.Conn
	send chan []byte
	done chan struct{}
}

type incomingMessage struct {
	To      any    `json:"to"`
	Message string `json:"message"`
}

func (c *Client) readPump() {
	defer func() {
		hub.unregister <- c
		<-c.done
	}()
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				hlog.Errorf("websocket read error: %v", err)
			}
			break
		}

		var inc incomingMessage
		if err := json.NewDecoder(bytes.NewReader(message)).Decode(&inc); err != nil {
			hlog.Errorf("invalid json format: %v", err)
			continue
		}

		var toID int64
		switch v := inc.To.(type) {
		case float64:
			toID = int64(v)
		case string:
			parsed, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				hlog.Errorf("invalid 'to' ID format: %v", err)
				continue
			}
			toID = parsed
		default:
			hlog.Errorf("invalid 'to' ID type")
			continue
		}

		hub.direct <- DirectMessage{
			From:    c.id,
			To:      toID,
			Message: inc.Message,
		}
	}
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
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func ServeWs(ctx context.Context, c *app.RequestContext) {
	userID, ok := authctx.UserID(c)
	if !ok {
		api.ErrorResponse(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing user context", nil)
		return
	}

	err := upgrader.Upgrade(c, func(conn *websocket.Conn) {
		client := &Client{
			id:   userID,
			conn: conn,
			send: make(chan []byte, 256),
			done: make(chan struct{}),
		}
		hub.register <- client

		go client.writePump()
		client.readPump()
	})

	if err != nil {
		hlog.CtxErrorf(ctx, "upgrade error: %v", err)
		api.ErrorResponse(c, http.StatusInternalServerError, "WEBSOCKET_UPGRADE_FAILED", "Could not upgrade to websocket connection", err.Error())
		return
	}
}
