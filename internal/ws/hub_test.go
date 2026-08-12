package ws

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func testHub(t *testing.T) (*Hub, context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	h := NewHub(nil)
	go h.Run(ctx)
	return h, cancel
}

func testClient(id int64) *Client {
	return &Client{id: id, send: make(chan []byte, 256), done: make(chan struct{})}
}

func registerClient(t *testing.T, h *Hub, c *Client) {
	t.Helper()
	c.hub = h
	h.Register(c)
}

func TestHub_Run_DirectMessage(t *testing.T) {
	h, cancel := testHub(t)
	defer cancel()

	c1 := testClient(1)
	c2 := testClient(2)
	registerClient(t, h, c1)
	registerClient(t, h, c2)

	msg := DirectMessage{From: 1, To: 2, Message: "hello"}
	if !h.Send(msg) {
		t.Fatal("Send returned false")
	}

	select {
	case gotBytes := <-c2.send:
		var gotMsg DirectMessage
		if err := json.Unmarshal(gotBytes, &gotMsg); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if gotMsg.Message != "hello" || gotMsg.From != 1 {
			t.Errorf("got %+v, want %+v", gotMsg, msg)
		}
	case <-time.After(time.Second):
		t.Fatal("message not delivered to client")
	}

	drain(c1)

	h.Unregister(c1)
	h.Unregister(c2)

	select {
	case _, ok := <-c1.send:
		if ok {
			t.Error("expected closed channel c1")
		}
	case <-time.After(time.Second):
		t.Fatal("channel c1 not closed")
	}
}

func drain(c *Client) {
	for {
		select {
		case <-c.send:
		default:
			return
		}
	}
}

func TestHub_DirectMessage_NonExistentClient(t *testing.T) {
	h, cancel := testHub(t)
	defer cancel()

	msg := DirectMessage{From: 1, To: 999, Message: "hello"}
	if !h.Send(msg) {
		t.Error("Send should return true for non-existent client")
	}
}

func TestHub_Unregister_NotFound(t *testing.T) {
	h, cancel := testHub(t)
	defer cancel()

	c := testClient(1)
	c.hub = h
	h.Unregister(c)
}

func TestHub_Shutdown_ViaContext(t *testing.T) {
	h, cancel := testHub(t)

	c := testClient(1)
	registerClient(t, h, c)

	cancel()

	select {
	case _, ok := <-c.send:
		if ok {
			t.Error("expected client channel to be closed on shutdown")
		}
	case <-time.After(time.Second):
		t.Fatal("client channel not closed after shutdown")
	}
}

func TestHub_Shutdown_ViaStop(t *testing.T) {
	h, cancel := testHub(t)
	defer cancel()

	c := testClient(1)
	registerClient(t, h, c)

	h.Stop()

	select {
	case _, ok := <-c.send:
		if ok {
			t.Error("expected client channel to be closed on stop")
		}
	case <-time.After(time.Second):
		t.Fatal("client channel not closed after stop")
	}
}

func TestHub_Sharding(t *testing.T) {
	h, cancel := testHub(t)
	defer cancel()

	clients := make([]*Client, 100)
	for i := range clients {
		clients[i] = testClient(int64(i + 1))
		registerClient(t, h, clients[i])
	}

	msg := DirectMessage{From: 1, To: 50, Message: "cross-shard"}
	if !h.Send(msg) {
		t.Fatal("Send returned false")
	}

	select {
	case gotBytes := <-clients[49].send:
		var gotMsg DirectMessage
		if err := json.Unmarshal(gotBytes, &gotMsg); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if gotMsg.Message != "cross-shard" {
			t.Errorf("got message %q, want %q", gotMsg.Message, "cross-shard")
		}
	case <-time.After(time.Second):
		t.Fatal("cross-shard message not delivered")
	}
}
