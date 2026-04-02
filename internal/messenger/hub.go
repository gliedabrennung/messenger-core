package messenger

import (
	"sync"

	"github.com/panjf2000/gnet/v2"
)

type Hub struct {
	// Map username -> connection
	userToConn sync.Map // map[string]gnet.Conn
	// Map connection -> username (to cleanup on disconnect)
	connToUser sync.Map // map[gnet.Conn]string

	broadcast chan []byte
	register   chan registration
	unregister chan gnet.Conn
}

type registration struct {
	conn     gnet.Conn
	username string
}

func NewHub() *Hub {
	return &Hub{
		broadcast:  make(chan []byte, 1024),
		register:   make(chan registration, 256),
		unregister: make(chan gnet.Conn, 256),
	}
}

func (h *Hub) Register(c gnet.Conn, username string) {
	h.register <- registration{c, username}
}

func (h *Hub) Unregister(c gnet.Conn) {
	h.unregister <- c
}

func (h *Hub) Broadcast(msg []byte) {
	h.broadcast <- msg
}

func (h *Hub) SendTo(to string, msg []byte) {
	if conn, ok := h.userToConn.Load(to); ok {
		c := conn.(gnet.Conn)
		c.AsyncWrite(msg, nil)
	}
}

func (h *Hub) GetActiveUsers(query string) []string {
	var users []string
	h.userToConn.Range(func(key, value interface{}) bool {
		username := key.(string)
		// Простой поиск по подстроке
		if query == "" || (len(username) >= len(query) && username[:len(query)] == query) {
			users = append(users, username)
		}
		return true
	})
	return users
}

func (h *Hub) Run() {
	for {
		select {
		case r := <-h.register:
			// Cleanup old registration if any
			if oldUser, ok := h.connToUser.Load(r.conn); ok {
				h.userToConn.Delete(oldUser)
			}
			h.userToConn.Store(r.username, r.conn)
			h.connToUser.Store(r.conn, r.username)
		case c := <-h.unregister:
			if username, ok := h.connToUser.Load(c); ok {
				h.userToConn.Delete(username)
				h.connToUser.Delete(c)
			}
		case msg := <-h.broadcast:
			h.userToConn.Range(func(key, value interface{}) bool {
				c := value.(gnet.Conn)
				c.AsyncWrite(msg, nil)
				return true
			})
		}
	}
}
