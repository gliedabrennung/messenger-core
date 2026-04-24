package messenger

import "encoding/json"

type Hub struct {
	clients    map[int64]*Client
	register   chan *Client
	unregister chan *Client
	direct     chan DirectMessage
}

type DirectMessage struct {
	From    int64  `json:"from"`
	To      int64  `json:"to"`
	Message string `json:"message"`
}

var hub = NewHub()

func StartHub() {
	hub.Run()
}

func NewHub() *Hub {
	return &Hub{
		register:   make(chan *Client),
		unregister: make(chan *Client),
		direct:     make(chan DirectMessage),
		clients:    make(map[int64]*Client),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			if oldClient, ok := h.clients[client.id]; ok {
				close(oldClient.send)
			}
			h.clients[client.id] = client
		case client := <-h.unregister:
			if c, ok := h.clients[client.id]; ok && c == client {
				delete(h.clients, client.id)
				close(client.send)
			}
		case msg := <-h.direct:
			if client, ok := h.clients[msg.To]; ok {
				msgBytes, err := json.Marshal(msg)
				if err != nil {
					continue
				}
				select {
				case client.send <- msgBytes:
				default:
					close(client.send)
					delete(h.clients, client.id)
				}
			}
		}
	}
}
