package message

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gliedabrennung/sedna/internal/common/logger"
	"github.com/gliedabrennung/sedna/internal/entity"
	"github.com/redis/go-redis/v9"
)

const (
	cacheMaxLen = 200
	cacheTTL    = 48 * time.Hour
)

type RedisCache struct {
	client *redis.Client
}

func NewRedisCache(client *redis.Client) *RedisCache {
	return &RedisCache{client: client}
}

func cacheKey(chatID string) string {
	return fmt.Sprintf("chat:%s:cache", chatID)
}

func (r *RedisCache) CacheMessage(ctx context.Context, msg *entity.Message) {
	key := cacheKey(msg.ChatID)
	data, err := json.Marshal(msg)
	if err != nil {
		logger.CtxErrorf(ctx, "redis marshal message failed: %v", err)
		return
	}

	pipe := r.client.Pipeline()
	pipe.LPush(ctx, key, data)
	pipe.LTrim(ctx, key, 0, cacheMaxLen-1)
	pipe.Expire(ctx, key, cacheTTL)

	if _, err := pipe.Exec(ctx); err != nil {
		logger.CtxErrorf(ctx, "redis cache message failed for chat %s: %v", msg.ChatID, err)
	}
}

func (r *RedisCache) GetCachedPage(ctx context.Context, chatID string, limit int) ([]*entity.Message, string, bool) {
	if limit <= 0 {
		return nil, "", false
	}

	cached, err := r.client.LRange(ctx, cacheKey(chatID), 0, int64(limit)).Result()
	if err != nil {
		if err != redis.Nil {
			logger.CtxErrorf(ctx, "redis lrange failed for chat %s: %v", chatID, err)
		}
		return nil, "", false
	}
	if len(cached) <= limit {
		return nil, "", false
	}

	messages := make([]*entity.Message, 0, len(cached))
	for _, data := range cached {
		var msg entity.Message
		if err := json.Unmarshal([]byte(data), &msg); err != nil {
			logger.CtxErrorf(ctx, "redis corrupt cache entry for chat %s: %v", chatID, err)
			return nil, "", false
		}
		messages = append(messages, &msg)
	}

	page := messages[:limit]
	return page, page[limit-1].MessageID, true
}

func (r *RedisCache) WarmUpCache(ctx context.Context, chatID string, messages []*entity.Message) {
	if len(messages) == 0 {
		return
	}
	if len(messages) > cacheMaxLen {
		messages = messages[:cacheMaxLen]
	}

	key := cacheKey(chatID)
	pipe := r.client.Pipeline()
	pipe.Del(ctx, key)

	for i := len(messages) - 1; i >= 0; i-- {
		data, err := json.Marshal(messages[i])
		if err != nil {
			continue
		}
		pipe.LPush(ctx, key, data)
	}

	pipe.Expire(ctx, key, cacheTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		logger.CtxErrorf(ctx, "redis warmup cache failed for chat %s: %v", chatID, err)
	}
}
