package messenger

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"time"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/panjf2000/gnet/v2"
	"github.com/gliedabrennung/messenger-core/pkg/models"
)

type WsServer struct {
	*gnet.BuiltinEventEngine
	hub *Hub
}

func NewWsServer(hub *Hub) *WsServer {
	return &WsServer{
		hub: hub,
	}
}

type connContext struct {
	upgraded bool
	username string
}

func (s *WsServer) OnOpened(c gnet.Conn) (out []byte, action gnet.Action) {
	c.SetContext(&connContext{upgraded: false})
	return
}

func (s *WsServer) OnClosed(c gnet.Conn, err error) (action gnet.Action) {
	s.hub.Unregister(c)
	return
}

type rw struct {
	r io.Reader
	w io.Writer
}

func (r *rw) Read(p []byte) (int, error)  { return r.r.Read(p) }
func (r *rw) Write(p []byte) (int, error) { return r.w.Write(p) }

func (s *WsServer) OnTraffic(c gnet.Conn) (action gnet.Action) {
	ctxRaw := c.Context()
	var ctx *connContext
	if ctxRaw == nil {
		ctx = &connContext{upgraded: false}
		c.SetContext(ctx)
	} else {
		ctx = ctxRaw.(*connContext)
	}

	for {
		data, _ := c.Peek(-1)
		if len(data) == 0 {
			break
		}

		if !ctx.upgraded {
			if bytes.Contains(data, []byte("Upgrade: websocket")) {
				br := bufio.NewReader(bytes.NewReader(data))
				_, err := ws.Upgrade(&rw{r: br, w: c})
				if err != nil {
					log.Printf("Upgrade error: %v", err)
					return gnet.Close
				}
				consumed := len(data) - br.Buffered()
				c.Discard(consumed)
				ctx.upgraded = true
				log.Printf("Connection upgraded to WebSocket")
				continue
			}

			// Обработка статики
			if bytes.HasPrefix(data, []byte("GET /")) {
				content, err := ioutil.ReadFile("static/index.html")
				if err != nil {
					c.AsyncWrite([]byte("HTTP/1.1 404 Not Found\r\n\r\n"), nil)
				} else {
					response := fmt.Sprintf("HTTP/1.1 200 OK\r\nContent-Type: text/html\r\nContent-Length: %d\r\n\r\n%s", len(content), string(content))
					c.AsyncWrite([]byte(response), nil)
				}
				c.Discard(len(data))
				return gnet.None
			}
			break
		}

		br := bufio.NewReader(bytes.NewReader(data))
		header, err := ws.ReadHeader(br)
		if err != nil {
			break
		}

		if int64(len(data)) < int64(len(data)-br.Buffered())+header.Length {
			break
		}

		payload := make([]byte, header.Length)
		_, err = br.Read(payload)
		if err != nil {
			break
		}

		if header.Masked {
			ws.Cipher(payload, header.Mask, 0)
		}

		c.Discard(len(data) - br.Buffered())

		if header.OpCode == ws.OpClose {
			return gnet.Close
		}

		if header.OpCode == ws.OpText || header.OpCode == ws.OpBinary {
			var msg models.Message
			if err := json.Unmarshal(payload, &msg); err != nil {
				log.Printf("Unmarshal error: %v", err)
				continue
			}

			msg.Timestamp = time.Now()

			switch msg.Type {
			case models.TypeAuth:
				ctx.username = msg.Content
				s.hub.Register(c, ctx.username)
				log.Printf("User %s authorized", ctx.username)
				
				resp := models.Message{Type: models.TypeStatus, Content: "authorized"}
				rd, _ := json.Marshal(resp)
				var buf bytes.Buffer
				wsutil.WriteServerMessage(&buf, ws.OpText, rd)
				c.AsyncWrite(buf.Bytes(), nil)

			case models.TypeSearch:
				users := s.hub.GetActiveUsers(msg.Content)
				resp := models.Message{Type: models.TypeSearch, Users: users}
				respData, _ := json.Marshal(resp)
				var buf bytes.Buffer
				wsutil.WriteServerMessage(&buf, ws.OpText, respData)
				c.AsyncWrite(buf.Bytes(), nil)

			case models.TypeChat:
				if ctx.username == "" {
					continue
				}
				msg.From = ctx.username
				respData, _ := json.Marshal(msg)
				var buf bytes.Buffer
				if err := wsutil.WriteServerMessage(&buf, ws.OpText, respData); err == nil {
					if msg.To == "" {
						s.hub.Broadcast(buf.Bytes())
					} else {
						s.hub.SendTo(msg.To, buf.Bytes())
						if msg.To != ctx.username {
							c.AsyncWrite(buf.Bytes(), nil)
						}
					}
				}
			}
		}
	}
	return gnet.None
}
