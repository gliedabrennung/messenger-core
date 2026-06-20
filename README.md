# Sedna

High-load real-time messenger backend.

## 🛠 Tech Stack
* **Language**: Go 1.26
* **Backend**: Hertz
* **Frontend**: React
* **Databases**: PostgreSQL (users), ScyllaDB (message history)
* **Cache & Pub/Sub**: Redis

## 🚀 Getting Started

Ensure you have **Go 1.26+** and **Docker** installed.

1. Start the infrastructure (DB and Redis):
   docker-compose up -d

2. Run tests:
   go test ./..

## 📡 API Examples

### 1. Authorization (REST)
POST /api/v1/auth/login
{"username": "johndoe", "password": "secretpassword"}
Response: 200 OK (returns a JWT token)

### 2. Retrieve Chat History (REST)
GET /api/v1/messages/history?partner_id=2
Response: 200 OK
{"messages": [{"message_id": "uuid-v4", "content": "Hello!", "from_id": 1, "to_id": 2, "created_at": "2026-05-22T20:00:00Z"}], "next_cursor": ""}

### 3. Send Message (WebSocket)
Connection: ws://localhost:8080/ws
Payload: {"to": 2, "message": "Hello! How are you?"}
