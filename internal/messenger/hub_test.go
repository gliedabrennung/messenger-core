package messenger

import (
	"testing"
	"time"
)

func TestNewHub(t *testing.T) {
	h := NewHub()
	if h == nil {
		t.Fatal("expected hub not to be nil")
	}
	if h.broadcast == nil {
		t.Error("expected broadcast channel not to be nil")
	}
	if h.register == nil {
		t.Error("expected register channel not to be nil")
	}
	if h.unregister == nil {
		t.Error("expected unregister channel not to be nil")
	}
	if h.clients == nil {
		t.Error("expected clients map not to be nil")
	}
}

func TestHubRegisterUnregister(t *testing.T) {
	h := NewHub()
	client := &Client{send: make(chan []byte, 1)}

	h.Register(client)
	h.clientsLock.RLock()
	_, ok := h.clients[client]
	h.clientsLock.RUnlock()
	if !ok {
		t.Fatal("expected client to be registered")
	}

	h.Unregister(client)
	h.clientsLock.RLock()
	_, ok = h.clients[client]
	h.clientsLock.RUnlock()
	if ok {
		t.Fatal("expected client to be unregistered")
	}

	select {
	case _, ok := <-client.send:
		if ok {
			t.Error("expected send channel to be closed")
		}
	default:
		t.Fatal("expected receive from closed channel")
	}
}

func TestHubRun(t *testing.T) {
	h := NewHub()
	go h.Run()

	client := &Client{send: make(chan []byte, 1)}
	h.register <- client

	time.Sleep(10 * time.Millisecond)
	h.clientsLock.RLock()
	_, ok := h.clients[client]
	h.clientsLock.RUnlock()
	if !ok {
		t.Fatal("expected client to be registered")
	}

	msg := []byte("hello")
	h.broadcast <- msg

	select {
	case received := <-client.send:
		if string(received) != string(msg) {
			t.Errorf("expected %s, got %s", msg, received)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for message")
	}

	h.unregister <- client
	time.Sleep(10 * time.Millisecond)
	h.clientsLock.RLock()
	_, ok = h.clients[client]
	h.clientsLock.RUnlock()
	if ok {
		t.Fatal("expected client to be unregistered")
	}
}
