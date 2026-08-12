package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gliedabrennung/sedna/internal/common/logger"

	"github.com/gorilla/websocket"
)

type userResponse struct {
	User struct {
		ID int64 `json:"id"`
	} `json:"user"`
	Token string `json:"token"`
}

func req(baseURL, path string, payload map[string]any) *userResponse {
	b, _ := json.Marshal(payload)
	resp, err := http.Post(baseURL+path, "application/json", bytes.NewReader(b))
	if err != nil {
		logger.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		logger.Fatalf("error %s: %s", path, body)
	}
	var out userResponse
	if err := json.Unmarshal(body, &out); err != nil {
		logger.Fatalf("unmarshal %s: %v", path, err)
	}
	return &out
}

func main() {
	baseURL := "http://localhost:8080"
	if v := os.Getenv("BASE_URL"); v != "" {
		baseURL = strings.TrimRight(v, "/")
	}

	ts := time.Now().UnixNano()
	u1 := fmt.Sprintf("u1_%d", ts)
	u2 := fmt.Sprintf("u2_%d", ts)

	req(baseURL, "/auth/register", map[string]any{"username": u1, "password": "password123"})
	req(baseURL, "/auth/register", map[string]any{"username": u2, "password": "password123"})

	r1 := req(baseURL, "/auth/login", map[string]any{"username": u1, "password": "password123"})
	r2 := req(baseURL, "/auth/login", map[string]any{"username": u2, "password": "password123"})

	fmt.Printf("User 1: %d, token: %s\n", r1.User.ID, r1.Token)
	fmt.Printf("User 2: %d, token: %s\n", r2.User.ID, r2.Token)

	wsURL := url.URL{Scheme: "ws", Host: "localhost:8080", Path: "/ws"}
	wsURL.RawQuery = "token=" + r1.Token
	c1, _, err := websocket.DefaultDialer.Dial(wsURL.String(), nil)
	if err != nil {
		logger.Fatal("dial c1:", err)
	}
	defer c1.Close()

	wsURL.RawQuery = "token=" + r2.Token
	c2, _, err := websocket.DefaultDialer.Dial(wsURL.String(), nil)
	if err != nil {
		logger.Fatal("dial c2:", err)
	}
	defer c2.Close()

	time.Sleep(100 * time.Millisecond)

	msg := map[string]any{
		"to":        fmt.Sprintf("%d", r2.User.ID),
		"message":   "hello from 1",
		"client_id": "smoke-1",
	}
	if err := c1.WriteJSON(msg); err != nil {
		logger.Fatal("write:", err)
	}

	if err := c1.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		logger.Fatal("set read deadline:", err)
	}
	ack := readFrame(c1, "ack")
	fmt.Printf("User 1 ack: message_id=%s client_id=%s\n", ack.MessageID, ack.ClientID)

	if err := c2.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		logger.Fatal("set read deadline:", err)
	}
	got := readFrame(c2, "message")
	fmt.Printf("User 2 received: %s (message_id=%s)\n", got.Message, got.MessageID)

	if got.MessageID != ack.MessageID {
		logger.Fatalf("delivered id %s does not match acked id %s", got.MessageID, ack.MessageID)
	}
}

type frame struct {
	Type      string `json:"type"`
	Code      string `json:"code"`
	ClientID  string `json:"client_id"`
	MessageID string `json:"message_id"`
	From      int64  `json:"from"`
	To        int64  `json:"to"`
	Message   string `json:"message"`
}

func readFrame(c *websocket.Conn, want string) frame {
	for {
		_, payload, err := c.ReadMessage()
		if err != nil {
			logger.Fatalf("read %s frame: %v", want, err)
		}
		var f frame
		if err := json.Unmarshal(payload, &f); err != nil {
			logger.Fatalf("unmarshal frame: %v", err)
		}
		if f.Type == "error" {
			logger.Fatalf("server rejected the message: %s", payload)
		}
		if f.Type == want {
			return f
		}
	}
}
