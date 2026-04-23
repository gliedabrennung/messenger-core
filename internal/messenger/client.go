package messenger

import (
	"bytes"
	"context"
	"net/http"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/gliedabrennung/messenger-core/internal/pkg/api"
	"github.com/hertz-contrib/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512
)

var (
	newline = []byte{'\n'}
	space   = []byte{' '}
)

var upgrader = websocket.HertzUpgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(ctx *app.RequestContext) bool {
		return true
	},
}

type Client struct {
	conn *websocket.Conn
	send chan []byte
}

func (c *Client) readPump() {
	defer func() {
		hub.unregister <- c
		err := c.conn.Close()
		if err != nil {
			hlog.Errorf(err.Error())
		}
	}()
	c.conn.SetReadLimit(maxMessageSize)
	err := c.conn.SetReadDeadline(time.Now().Add(pongWait))
	if err != nil {
		hlog.Errorf("%v", err)
	}
	c.conn.SetPongHandler(func(string) error {
		err = c.conn.SetReadDeadline(time.Now().Add(pongWait))
		hlog.Errorf(err.Error())
		return nil
	})
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err,
				websocket.CloseGoingAway,
				websocket.CloseAbnormalClosure) {
				hlog.Errorf("websocket read error: %v", err)
			}
			break
		}
		message = bytes.TrimSpace(bytes.Replace(message, newline, space, -1))
		hub.broadcast <- message
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		err := c.conn.Close()
		if err != nil {
			hlog.Error(err.Error())
		}
	}()
	for {
		select {
		case message, ok := <-c.send:
			err := c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err != nil {
				hlog.Error(err.Error())
			}
			if !ok {
				err = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				if err != nil {
					hlog.Error(err.Error())
				}
				return
			}
			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				hlog.Errorf("websocket writer error: %v", err)
				return
			}
			_, err = w.Write(message)
			if err != nil {
				hlog.Errorf("websocket write error: %v", err)
			}
			n := len(c.send)
			for i := 0; i < n; i++ {
				_, err = w.Write(newline)
				if err != nil {
					hlog.Errorf("websocket write error: %v", err)
				}
				_, err = w.Write(<-c.send)
				if err != nil {
					hlog.Errorf("websocket write error: %v", err)
				}
			}
			if err := w.Close(); err != nil {
				hlog.Errorf("websocket close writer error: %v", err)
				return
			}
		case <-ticker.C:
			err := c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err != nil {
				hlog.Error(err.Error())
			}
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				hlog.Errorf("websocket ping error: %v", err)
				return
			}
		}
	}
}

func ServeWs(ctx context.Context, c *app.RequestContext) {
	err := upgrader.Upgrade(c, func(conn *websocket.Conn) {
		client := &Client{conn: conn, send: make(chan []byte, 256)}
		hub.register <- client

		go client.writePump()
		client.readPump()
	})

	if err != nil {
		hlog.CtxErrorf(ctx, "upgrade error: %v", err)
		api.ErrorResponse(c, http.StatusInternalServerError,
			"WEBSOCKET_UPGRADE_FAILED",
			"Could not upgrade to websocket connection",
			err.Error())
		return
	}
}
