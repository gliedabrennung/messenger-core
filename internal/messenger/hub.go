package messenger

import (
	"sync"
)

type Hub struct {
	clients     map[*Client]struct{}
	clientsLock sync.RWMutex

	broadcast chan []byte

	register chan *Client

	unregister chan *Client
}

var hub = NewHub()

func StartHub() {
	hub.Run()
}

func NewHub() *Hub {
	return &Hub{
		broadcast:  make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]struct{}),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.Register(client)
		case client := <-h.unregister:
			h.Unregister(client)
		case message := <-h.broadcast:
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
		}
	}
}

func (h *Hub) Register(client *Client) {
	h.AddClient(client)
}

func (h *Hub) AddClient(client *Client) {
	h.clientsLock.Lock()
	defer h.clientsLock.Unlock()
	h.clients[client] = struct{}{}
}

func (h *Hub) Unregister(client *Client) {
	h.DelClient(client)
}

func (h *Hub) DelClient(client *Client) {
	h.clientsLock.Lock()
	defer h.clientsLock.Unlock()
	if _, ok := h.clients[client]; ok {
		delete(h.clients, client)
		close(client.send)
	}
}
