package message

import (
	"context"

	"github.com/gliedabrennung/sedna/internal/entity"
	"github.com/gocql/gocql"
	"github.com/redis/go-redis/v9"
)

type Repository struct {
	scylla *ScyllaStorage
	redis  *RedisCache
}

func NewRepository(scyllaSession *gocql.Session, rdb *redis.Client, keyspace string) *Repository {
	repo := &Repository{scylla: NewScyllaStorage(scyllaSession, keyspace)}
	if rdb != nil {
		repo.redis = NewRedisCache(rdb)
	}
	return repo
}

func (r *Repository) NewMessageID() string {
	return gocql.TimeUUID().String()
}

func (r *Repository) Save(ctx context.Context, msg *entity.Message) error {
	if err := r.scylla.Save(ctx, msg); err != nil {
		return err
	}
	if r.redis != nil {
		r.redis.CacheMessage(ctx, msg)
	}
	return nil
}

func (r *Repository) GetChatHistory(ctx context.Context, chatID string, limit int, cursor string) ([]*entity.Message, string, error) {
	if cursor != "" {
		return r.scylla.GetHistory(ctx, chatID, limit, cursor)
	}

	if r.redis != nil {
		if page, next, ok := r.redis.GetCachedPage(ctx, chatID, limit); ok {
			return page, next, nil
		}
	}

	fetch := limit
	if fetch < cacheMaxLen {
		fetch = cacheMaxLen
	}

	window, _, err := r.scylla.GetHistory(ctx, chatID, fetch, "")
	if err != nil {
		return nil, "", err
	}

	if r.redis != nil && len(window) > 0 {
		r.redis.WarmUpCache(ctx, chatID, window)
	}

	if len(window) > limit {
		page := window[:limit]
		return page, page[limit-1].MessageID, nil
	}
	return window, "", nil
}
