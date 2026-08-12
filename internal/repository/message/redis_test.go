package message

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/gliedabrennung/sedna/internal/entity"
	"github.com/redis/go-redis/v9"
)

func testRedis(t *testing.T) *redis.Client {
	t.Helper()
	addr := os.Getenv("TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("TEST_REDIS_ADDR not set, skipping redis integration test")
	}

	client := redis.NewClient(&redis.Options{Addr: addr})
	if err := client.Ping(context.Background()).Err(); err != nil {
		t.Fatalf("could not connect to redis: %v", err)
	}
	t.Cleanup(func() { client.Close() })
	return client
}

func seedCache(t *testing.T, cache *RedisCache, chatID string, n int) {
	t.Helper()
	msgs := make([]*entity.Message, 0, n)
	for i := n; i >= 1; i-- {
		msgs = append(msgs, &entity.Message{
			ChatID:    chatID,
			MessageID: fmt.Sprintf("msg-%d", i),
			Content:   fmt.Sprintf("body %d", i),
			CreatedAt: time.Now(),
		})
	}
	cache.WarmUpCache(context.Background(), chatID, msgs)
}

func TestRedisCache_GetCachedPage_ServesFullPage(t *testing.T) {
	client := testRedis(t)
	ctx := context.Background()
	cache := NewRedisCache(client)
	chatID := "test:redis:page"

	client.Del(ctx, cacheKey(chatID))
	seedCache(t, cache, chatID, 5)

	page, next, ok := cache.GetCachedPage(ctx, chatID, 3)
	if !ok {
		t.Fatal("expected cache hit when cache holds more than one page")
	}
	if len(page) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(page))
	}
	if page[0].MessageID != "msg-5" {
		t.Errorf("expected newest message first, got %s", page[0].MessageID)
	}
	if next != "msg-3" {
		t.Errorf("expected cursor msg-3, got %q", next)
	}
}

func TestRedisCache_GetCachedPage_ShortWindowIsMiss(t *testing.T) {
	client := testRedis(t)
	ctx := context.Background()
	cache := NewRedisCache(client)
	chatID := "test:redis:short"

	client.Del(ctx, cacheKey(chatID))
	cache.CacheMessage(ctx, &entity.Message{
		ChatID:    chatID,
		MessageID: "only-one",
		Content:   "latest",
		CreatedAt: time.Now(),
	})

	if _, _, ok := cache.GetCachedPage(ctx, chatID, 50); ok {
		t.Fatal("cache holding fewer messages than the page size must be a miss")
	}
}

func TestRedisCache_GetCachedPage_EmptyIsMiss(t *testing.T) {
	client := testRedis(t)
	ctx := context.Background()
	cache := NewRedisCache(client)
	chatID := "test:redis:empty"

	client.Del(ctx, cacheKey(chatID))

	if _, _, ok := cache.GetCachedPage(ctx, chatID, 10); ok {
		t.Fatal("empty cache must be a miss")
	}
}

func TestRedisCache_WarmUpCache_CapsAtWindow(t *testing.T) {
	client := testRedis(t)
	ctx := context.Background()
	cache := NewRedisCache(client)
	chatID := "test:redis:cap"

	client.Del(ctx, cacheKey(chatID))
	seedCache(t, cache, chatID, cacheMaxLen+50)

	n, err := client.LLen(ctx, cacheKey(chatID)).Result()
	if err != nil {
		t.Fatalf("llen failed: %v", err)
	}
	if n != int64(cacheMaxLen) {
		t.Errorf("expected cache capped at %d, got %d", cacheMaxLen, n)
	}
}

func TestRedisCache_CacheMessage_TrimsToWindow(t *testing.T) {
	client := testRedis(t)
	ctx := context.Background()
	cache := NewRedisCache(client)
	chatID := "test:redis:trim"

	client.Del(ctx, cacheKey(chatID))
	for i := 0; i < cacheMaxLen+10; i++ {
		cache.CacheMessage(ctx, &entity.Message{
			ChatID:    chatID,
			MessageID: fmt.Sprintf("msg-%d", i),
			Content:   "x",
			CreatedAt: time.Now(),
		})
	}

	n, err := client.LLen(ctx, cacheKey(chatID)).Result()
	if err != nil {
		t.Fatalf("llen failed: %v", err)
	}
	if n != int64(cacheMaxLen) {
		t.Errorf("expected %d entries, got %d", cacheMaxLen, n)
	}
}
