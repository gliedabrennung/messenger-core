package message

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/gliedabrennung/sedna/internal/entity"
	"github.com/gocql/gocql"
	"github.com/redis/go-redis/v9"
)

func TestRepository_Integration(t *testing.T) {
	redisAddr := os.Getenv("TEST_REDIS_ADDR")
	scyllaHosts := os.Getenv("TEST_SCYLLA_HOSTS")
	if redisAddr == "" || scyllaHosts == "" {
		t.Skip("TEST_REDIS_ADDR or TEST_SCYLLA_HOSTS not set, skipping repository integration test")
	}

	ctx := context.Background()
	if err := InitSchema(ctx, []string{scyllaHosts}, "ws"); err != nil {
		t.Fatalf("could not initialize schema: %v", err)
	}

	redisClient := redis.NewClient(&redis.Options{Addr: redisAddr})
	defer redisClient.Close()

	cluster := gocql.NewCluster(scyllaHosts)
	cluster.Keyspace = "ws"
	scyllaSession, err := cluster.CreateSession()
	if err != nil {
		t.Fatalf("could not connect to scylla: %v", err)
	}
	defer scyllaSession.Close()

	repo := NewRepository(scyllaSession, redisClient, "ws")
	chatID := fmt.Sprintf("test:repo:%d", time.Now().UnixNano())

	redisClient.Del(ctx, "chat:"+chatID+":cache")

	msg := &entity.Message{
		ChatID:  chatID,
		FromID:  1,
		ToID:    2,
		Content: "full integration message",
	}

	err = repo.Save(ctx, msg)
	if err != nil {
		t.Fatalf("save failed: %v", err)
	}

	msgs, _, err := repo.GetChatHistory(ctx, chatID, 1, "")
	if err != nil {
		t.Fatalf("get history failed: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}

	redisClient.Del(ctx, "chat:"+chatID+":cache")

	msgs2, _, err := repo.GetChatHistory(ctx, chatID, 1, "")
	if err != nil {
		t.Fatalf("get history with fallback failed: %v", err)
	}
	if len(msgs2) != 1 {
		t.Fatalf("expected 1 message from fallback, got %d", len(msgs2))
	}

	time.Sleep(50 * time.Millisecond)

	cached, err := redisClient.LRange(ctx, "chat:"+chatID+":cache", 0, -1).Result()
	if err != nil {
		t.Fatalf("redis lrange failed: %v", err)
	}
	if len(cached) != 1 {
		t.Fatalf("cache was not warmed up, expected 1, got %d", len(cached))
	}
}
