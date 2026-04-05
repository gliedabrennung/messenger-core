package messenger

import (
	"sync"
	"testing"
	"time"
)

func testClient() *Client {
	return &Client{send: make(chan []byte, 256)}
}

func runHub(h *Hub) func() {
	stop := make(chan struct{})
	go func() {
		for {
			select {
			case client := <-h.register:
				h.Register(client)
			case client := <-h.unregister:
				h.Unregister(client)
			case message := <-h.broadcast:
				h.clientsLock.RLock()
				clients := make([]*Client, 0, len(h.clients))
				for c := range h.clients {
					clients = append(clients, c)
				}
				h.clientsLock.RUnlock()
				for _, c := range clients {
					select {
					case c.send <- message:
					default:
						close(c.send)
						h.clientsLock.Lock()
						delete(h.clients, c)
						h.clientsLock.Unlock()
					}
				}
			case <-stop:
				return
			}
		}
	}()
	return func() { close(stop) }
}

func clientCount(h *Hub) int {
	h.clientsLock.RLock()
	defer h.clientsLock.RUnlock()
	return len(h.clients)
}

func TestNewHub_InitialState(t *testing.T) {
	h := NewHub()

	if h.clients == nil {
		t.Error("clients map is nil")
	}
	if h.broadcast == nil {
		t.Error("broadcast channel is nil")
	}
	if h.register == nil {
		t.Error("register channel is nil")
	}
	if h.unregister == nil {
		t.Error("unregister channel is nil")
	}
	if clientCount(h) != 0 {
		t.Errorf("expected 0 clients, got %d", clientCount(h))
	}
}

func TestAddClient_AddsToMap(t *testing.T) {
	h := NewHub()
	c := testClient()

	h.AddClient(c)

	if clientCount(h) != 1 {
		t.Errorf("expected 1 client, got %d", clientCount(h))
	}
}

func TestAddClient_Idempotent(t *testing.T) {
	h := NewHub()
	c := testClient()

	h.AddClient(c)
	h.AddClient(c)

	if clientCount(h) != 1 {
		t.Errorf("map must deduplicate: expected 1, got %d", clientCount(h))
	}
}

func TestDelClient_RemovesFromMap(t *testing.T) {
	h := NewHub()
	c := testClient()

	h.AddClient(c)
	h.DelClient(c)

	if clientCount(h) != 0 {
		t.Errorf("expected 0 clients after delete, got %d", clientCount(h))
	}
}

func TestDelClient_ClosesSendChannel(t *testing.T) {
	h := NewHub()
	c := testClient()

	h.AddClient(c)
	h.DelClient(c)

	select {
	case _, ok := <-c.send:
		if ok {
			t.Error("expected closed channel, got value")
		}
	default:
		t.Error("channel is not closed")
	}
}

func TestDelClient_NotRegistered_NoPanic(t *testing.T) {
	h := NewHub()
	c := testClient()

	h.DelClient(c)
}

func TestDelClient_DoubleDelete_NoPanic(t *testing.T) {
	h := NewHub()
	c := testClient()

	h.AddClient(c)
	h.DelClient(c)

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("double DelClient panicked: %v", r)
		}
	}()
	h.DelClient(c)
}

func TestRun_RegisterAddsClient(t *testing.T) {
	h := NewHub()
	stop := runHub(h)
	defer stop()

	c := testClient()
	h.register <- c

	time.Sleep(50 * time.Millisecond)

	if clientCount(h) != 1 {
		t.Errorf("expected 1 client after register, got %d", clientCount(h))
	}
}

func TestRun_UnregisterRemovesClient(t *testing.T) {
	h := NewHub()
	stop := runHub(h)
	defer stop()

	c := testClient()
	h.register <- c
	time.Sleep(50 * time.Millisecond)

	h.unregister <- c
	time.Sleep(50 * time.Millisecond)

	if clientCount(h) != 0 {
		t.Errorf("expected 0 clients after unregister, got %d", clientCount(h))
	}
}

func TestRun_BroadcastDelivered(t *testing.T) {
	h := NewHub()
	stop := runHub(h)
	defer stop()

	c := testClient()
	h.register <- c
	time.Sleep(50 * time.Millisecond)

	msg := []byte("hello")
	h.broadcast <- msg

	select {
	case got := <-c.send:
		if string(got) != string(msg) {
			t.Errorf("got %q, want %q", got, msg)
		}
	case <-time.After(time.Second):
		t.Fatal("message not delivered to client")
	}
}

func TestRun_BroadcastToMultipleClients(t *testing.T) {
	h := NewHub()
	stop := runHub(h)
	defer stop()

	const n = 5
	clients := make([]*Client, n)
	for i := range clients {
		clients[i] = testClient()
		h.register <- clients[i]
	}
	time.Sleep(50 * time.Millisecond)

	msg := []byte("multicast")
	h.broadcast <- msg

	for i, c := range clients {
		select {
		case got := <-c.send:
			if string(got) != string(msg) {
				t.Errorf("client %d: got %q, want %q", i, got, msg)
			}
		case <-time.After(time.Second):
			t.Fatalf("client %d did not receive message", i)
		}
	}
}

func TestRun_BroadcastDropsSlowClient(t *testing.T) {
	h := NewHub()
	stop := runHub(h)
	defer stop()
	slow := &Client{send: make(chan []byte)}
	h.register <- slow
	time.Sleep(50 * time.Millisecond)

	h.broadcast <- []byte("drop me")
	time.Sleep(50 * time.Millisecond)

	if clientCount(h) != 0 {
		t.Errorf("slow client should have been dropped, got %d clients", clientCount(h))
	}

	select {
	case _, ok := <-slow.send:
		if ok {
			t.Error("slow client channel should be closed")
		}
	default:
		t.Error("slow client channel is not closed")
	}
}

func TestAddDelClient_ConcurrentNoPanic(t *testing.T) {
	h := NewHub()
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c := testClient()
			h.AddClient(c)
			h.DelClient(c)
		}()
	}

	wg.Wait()
}
