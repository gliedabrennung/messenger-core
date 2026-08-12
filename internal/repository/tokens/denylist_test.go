package tokens

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func testDenylist(t *testing.T) *RedisDenylist {
	t.Helper()
	addr := os.Getenv("TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("TEST_REDIS_ADDR not set, skipping denylist integration test")
	}
	client := redis.NewClient(&redis.Options{Addr: addr})
	if err := client.Ping(context.Background()).Err(); err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { client.Close() })
	return NewRedisDenylist(client)
}

func TestRevokeMakesTokenUnusable(t *testing.T) {
	d := testDenylist(t)
	ctx := context.Background()

	revoked, err := d.IsRevoked(ctx, "fresh-token")
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if revoked {
		t.Fatal("an unrevoked token must not be reported as revoked")
	}

	if err := d.Revoke(ctx, "fresh-token", time.Now().Add(time.Minute)); err != nil {
		t.Fatalf("revoke: %v", err)
	}

	revoked, err = d.IsRevoked(ctx, "fresh-token")
	if err != nil {
		t.Fatalf("check after revoke: %v", err)
	}
	if !revoked {
		t.Error("expected the token to be revoked")
	}
}

func TestRevokeExpiresWithTheToken(t *testing.T) {
	d := testDenylist(t)
	ctx := context.Background()

	if err := d.Revoke(ctx, "short-token", time.Now().Add(time.Second)); err != nil {
		t.Fatalf("revoke: %v", err)
	}

	ttl, err := d.client.TTL(ctx, key("short-token")).Result()
	if err != nil {
		t.Fatalf("ttl: %v", err)
	}
	if ttl <= 0 || ttl > time.Second {
		t.Errorf("expected a ttl of at most 1s, got %v", ttl)
	}
}

func TestRevokeAlreadyExpiredIsNoop(t *testing.T) {
	d := testDenylist(t)
	ctx := context.Background()

	if err := d.Revoke(ctx, "expired-token", time.Now().Add(-time.Hour)); err != nil {
		t.Fatalf("revoke: %v", err)
	}

	revoked, err := d.IsRevoked(ctx, "expired-token")
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if revoked {
		t.Error("an already-expired token needs no denylist entry")
	}
}
