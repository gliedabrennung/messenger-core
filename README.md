# Sedna

High-load real-time messenger backend.

## 🛠 Tech Stack

* **Language**: Go 1.26
* **Backend**: Hertz
* **Frontend**: React
* **Databases**: PostgreSQL (users), ScyllaDB (message history)
* **Cache & Pub/Sub**: Redis

## 🚀 Getting Started

Ensure you have **Go 1.26+**, **Node 22+** and **Docker** installed.

1. Create a `.env` from the template and fill it in:
   ```sh
   cp .env.example .env
   ```
   `JWT_SECRET` must be at least 32 characters — the server refuses to start
   with anything shorter.

2. Start the infrastructure (Postgres, Scylla, Redis):
   ```sh
   docker compose up -d db redis scylla
   ```

3. Run the server. It applies pending SQL migrations at startup:
   ```sh
   go run ./cmd/server
   ```

4. Run the tests:
   ```sh
   go test ./...
   ```
   Repository tests are skipped unless the backing services are reachable:
   ```sh
   TEST_REDIS_ADDR=localhost:6379 TEST_SCYLLA_HOSTS=localhost go test ./...
   ```

Or run everything, frontend included, in containers:
```sh
docker compose up --build
```

## ⚙️ Configuration

All settings come from the environment (or a `.env` file). See `.env.example`
for the full list; the ones worth knowing about:

| Variable | Default | Notes |
|---|---|---|
| `DSN` | — | Postgres connection string. Required. |
| `JWT_SECRET` | — | HS256 key, minimum 32 characters. Required. |
| `ALLOWED_ORIGINS` | empty | Websocket origin allow-list. Empty means same-origin only; `*` disables the check. |
| `TRUSTED_PROXIES` | empty | CIDRs allowed to set `X-Forwarded-For`. Empty means the header is ignored, so per-IP rate limits cannot be forged. |
| `COOKIE_SECURE` | `false` | Must be `true` when served over HTTPS. |
| `STORAGE_REQUIRED` | `true` | Refuse to start when Scylla or Redis is unreachable, instead of accepting messages that would be lost. |
| `RUN_MIGRATIONS` | `true` | Apply pending SQL migrations at startup. |

## 📡 API

Authentication is a JWT, sent either as `Authorization: Bearer <token>` or in
the `token` cookie set by `/auth/login`.

### Register

```
POST /auth/register
{"username": "johndoe", "password": "secretpassword"}

201 Created
{"user": {"id": 100000000001, "username": "johndoe", ...}}
```

Usernames are 3–24 letters, digits, `_`, `.` or `-`, and are unique
case-insensitively.

### Login

```
POST /auth/login
{"username": "johndoe", "password": "secretpassword"}

200 OK
{"token": "<jwt>", "user": {...}}
```

### Chat history

```
GET /messages?partner_id=2&limit=50&cursor=<next_cursor>

200 OK
{
  "messages": [
    {"message_id": "uuid-v1", "content": "Hello!", "from_id": 1, "to_id": 2,
     "created_at": "2026-05-22T20:00:00Z"}
  ],
  "next_cursor": ""
}
```

Newest first. An empty `next_cursor` means there is nothing older.

### Conversations

```
GET /chats

200 OK
{
  "chats": [
    {"chat_id": "1:2", "peer_id": 2, "peer_username": "johndoe",
     "last_message": "Hello!", "last_from_id": 1,
     "last_activity": "2026-05-22T20:00:00Z"}
  ]
}
```

Most recently active first. The list is maintained server-side, so it follows
the user to a new device.

### Other endpoints

| Route | Purpose |
|---|---|
| `POST /auth/logout` | Clears the auth cookie. |
| `GET /users/search?q=` | Search by username or id. Minimum 3 characters. |
| `GET /users/bulk?ids=1,2,3` | Look up users by id (max 100). |
| `GET /users/me` | The authenticated user. |
| `GET /health` | Liveness probe. |

## 🔌 Websocket protocol

```
ws://localhost:8080/ws
```

The connection is authenticated by the same cookie or token as the REST API.

**Client → server**

```json
{"to": 2, "message": "Hello! How are you?", "client_id": "c-42"}
```

`client_id` is the sender's own reference for the message. It is optional, but
without it the sender cannot match the acknowledgement to the message it sent.

**Server → client** — every frame carries a `type`:

```json
{"type": "message", "message_id": "uuid-v1", "from": 1, "to": 2,
 "message": "Hello!", "created_at": "2026-05-22T20:00:00Z"}

{"type": "ack", "client_id": "c-42", "message_id": "uuid-v1", "to": 2,
 "created_at": "2026-05-22T20:00:00Z"}

{"type": "error", "code": "RATE_LIMITED", "message": "sending too fast",
 "client_id": "c-42", "to": 2}
```

`message_id` is assigned when the message is accepted, so the id in the ack is
the id the message will have in history. Error codes: `INVALID_PAYLOAD`,
`MESSAGE_TOO_LONG`, `INVALID_RECIPIENT`, `UNKNOWN_RECIPIENT`, `RATE_LIMITED`,
`MESSAGE_NOT_STORED`.

Limits: 4000 characters per message, 10 messages per second per connection
(burst 20).

## 🏗 Architecture notes

* **Delivery** goes through Redis pub/sub, one channel per user
  (`internal/fanout`). A node subscribes only for the users it holds
  connections for, so several instances can serve one conversation.
* **Persistence** happens off the connection's read loop in a worker pool. The
  message id is a time UUID stamped at ingest, so it can be broadcast before the
  write completes and insert order does not affect history order.
* **History caching** keeps a trailing window per chat in Redis. The cache is
  only trusted when it holds more than one page, because a shorter window cannot
  prove that nothing older exists.
* **Conversations** are written to `user_chats` on both sides of every message,
  using the message time as the Cassandra write timestamp so an out-of-order
  write cannot overwrite a newer one. The list is read as one partition and
  ordered in the application, which avoids the tombstone churn of keeping
  activity time in the clustering key.
* **Schema**: Postgres migrations are embedded in the binary and applied at
  startup under an advisory lock, so several instances can boot at once; the
  Scylla keyspace is created by the server, with
  `migrations/scylla/direct_messages.cql` as the reference copy.
