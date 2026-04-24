package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
)

type userResponse struct {
	User struct {
		ID int64 `json:"id"`
	} `json:"user"`
	Token string `json:"token"`
}

func req(path string, payload map[string]interface{}) *userResponse {
	b, _ := json.Marshal(payload)
	resp, err := http.Post("http://localhost:8080"+path, "application/json", bytes.NewReader(b))
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		log.Fatalf("error %s: %s", path, body)
	}
	var out userResponse
	json.Unmarshal(body, &out)
	return &out
}

func main() {
	ts := time.Now().UnixNano()
	u1 := fmt.Sprintf("u1_%d", ts)
	u2 := fmt.Sprintf("u2_%d", ts)
	
	req("/auth/register", map[string]interface{}{"username": u1, "password": "password123"})
	req("/auth/register", map[string]interface{}{"username": u2, "password": "password123"})
	
	r1 := req("/auth/login", map[string]interface{}{"username": u1, "password": "password123"})
	r2 := req("/auth/login", map[string]interface{}{"username": u2, "password": "password123"})

	fmt.Printf("User 1: %d, token: %s\n", r1.User.ID, r1.Token)
	fmt.Printf("User 2: %d, token: %s\n", r2.User.ID, r2.Token)

	u := url.URL{Scheme: "ws", Host: "localhost:8080", Path: "/ws"}
	u.RawQuery = "token=" + r1.Token
	c1, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatal("dial c1:", err)
	}
	defer c1.Close()

	u.RawQuery = "token=" + r2.Token
	c2, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatal("dial c2:", err)
	}
	defer c2.Close()

	time.Sleep(100 * time.Millisecond)

	msg := map[string]interface{}{
		"to":      fmt.Sprintf("%d", r2.User.ID),
		"message": "hello from 1",
	}
	if err := c1.WriteJSON(msg); err != nil {
		log.Fatal("write:", err)
	}

	c2.SetReadDeadline(time.Now().Add(time.Second))
	_, p, err := c2.ReadMessage()
	if err != nil {
		log.Fatal("read c2:", err)
	}
	fmt.Printf("User 2 received: %s\n", p)
}
