package messenger

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/panjf2000/gnet/v2"
)

func TestWebSocket(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	server := NewWsServer(hub)
	
	// Start server on random port
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()

	go func() {
		gnet.Run(server, "tcp://"+addr)
	}()

	time.Sleep(500 * time.Millisecond)

	// Connect
	conn, _, _, err := ws.Dial(context.Background(), "ws://"+addr+"/ws")
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	msg := "hello gnet"
	err = wsutil.WriteClientText(conn, []byte(msg))
	if err != nil {
		t.Fatalf("Failed to write: %v", err)
	}

	data, err := wsutil.ReadServerText(conn)
	if err != nil {
		t.Fatalf("Failed to read: %v", err)
	}

	if string(data) != msg {
		t.Errorf("Expected %s, got %s", msg, string(data))
	}
}
