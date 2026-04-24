package messenger

import (
	"encoding/json"
	"testing"
	"time"
)

func testClient(id int64) *Client {
	return &Client{id: id, send: make(chan []byte, 256), done: make(chan struct{})}
}

func TestHub_Run_DirectMessage(t *testing.T) {
	h := NewHub()
	go h.Run()

	c1 := testClient(1)
	c2 := testClient(2)

	h.register <- c1
	h.register <- c2
	time.Sleep(50 * time.Millisecond)

	msg := DirectMessage{From: 1, To: 2, Message: "hello"}
	h.direct <- msg

	select {
	case gotBytes := <-c2.send:
		var gotMsg DirectMessage
		json.Unmarshal(gotBytes, &gotMsg)
		if gotMsg.Message != "hello" || gotMsg.From != 1 {
			t.Errorf("got %+v, want %+v", gotMsg, msg)
		}
	case <-time.After(time.Second):
		t.Fatal("message not delivered to client")
	}

	// Unregister
	h.unregister <- c1
	h.unregister <- c2
	time.Sleep(50 * time.Millisecond)

	select {
	case _, ok := <-c1.send:
		if ok {
			t.Error("expected closed channel c1")
		}
	case <-time.After(time.Second):
		t.Fatal("channel c1 not closed")
	}
}

func TestHub_Register_ReplacesOldConnection(t *testing.T) {
	h := NewHub()
	go h.Run()

	c1_old := testClient(1)
	h.register <- c1_old
	time.Sleep(50 * time.Millisecond)

	c1_new := testClient(1)
	h.register <- c1_new
	time.Sleep(50 * time.Millisecond)

	select {
	case _, ok := <-c1_old.send:
		if ok {
			t.Error("expected old connection channel to be closed")
		}
	case <-time.After(time.Second):
		t.Fatal("old connection channel not closed")
	}
}

func TestHub_DirectMessage_NonExistentClient(t *testing.T) {
	h := NewHub()
	go h.Run()

	msg := DirectMessage{From: 1, To: 999, Message: "hello"}
	h.direct <- msg
	// Should not panic or block
	time.Sleep(50 * time.Millisecond)
}

func TestHub_Unregister_NotFound(t *testing.T) {
	h := NewHub()
	go h.Run()

	c := testClient(1)
	h.unregister <- c
	// Should not panic or block
	time.Sleep(50 * time.Millisecond)
}
