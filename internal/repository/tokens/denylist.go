package tokens

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const keyPrefix = "token:revoked:"

type RedisDenylist struct {
	client *redis.Client
}

func NewRedisDenylist(client *redis.Client) *RedisDenylist {
	return &RedisDenylist{client: client}
}

func key(tokenID string) string {
	return keyPrefix + tokenID
}

func (d *RedisDenylist) Revoke(ctx context.Context, tokenID string, expiresAt time.Time) error {
	ttl := time.Until(expiresAt)
	if ttl <= 0 {
		return nil
	}
	if err := d.client.Set(ctx, key(tokenID), "1", ttl).Err(); err != nil {
		return fmt.Errorf("tokens: revoke: %w", err)
	}
	return nil
}

func (d *RedisDenylist) IsRevoked(ctx context.Context, tokenID string) (bool, error) {
	n, err := d.client.Exists(ctx, key(tokenID)).Result()
	if err != nil {
		return false, fmt.Errorf("tokens: check revoked: %w", err)
	}
	return n > 0, nil
}
