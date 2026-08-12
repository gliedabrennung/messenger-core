package fanout

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/gliedabrennung/sedna/internal/ws"
	"github.com/redis/go-redis/v9"
)

func testClient(t *testing.T) *redis.Client {
	t.Helper()
	addr := os.Getenv("TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("TEST_REDIS_ADDR not set, skipping fanout integration test")
	}
	client := redis.NewClient(&redis.Options{Addr: addr})
	if err := client.Ping(context.Background()).Err(); err != nil {
		t.Fatalf("could not connect to redis: %v", err)
	}
	t.Cleanup(func() { client.Close() })
	return client
}

func waitForDelivery(t *testing.T, ch <-chan ws.Delivery) (ws.Delivery, bool) {
	t.Helper()
	select {
	case d := <-ch:
		return d, true
	case <-time.After(2 * time.Second):
		return ws.Delivery{}, false
	}
}

func TestRedis_CrossInstanceDelivery(t *testing.T) {
	client := testClient(t)
	ctx := t.Context()

	receiver := NewRedis(ctx, client)
	defer receiver.Close()
	sender := NewRedis(ctx, client)
	defer sender.Close()

	const userID = int64(4242)
	if err := receiver.Subscribe(userID); err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	if err := sender.Publish(ctx, userID, []byte(`{"message":"cross node"}`)); err != nil {
		t.Fatalf("publish: %v", err)
	}

	got, ok := waitForDelivery(t, receiver.Delivery())
	if !ok {
		t.Fatal("message never crossed instances")
	}
	if got.UserID != userID {
		t.Errorf("expected user %d, got %d", userID, got.UserID)
	}
	if string(got.Data) != `{"message":"cross node"}` {
		t.Errorf("payload mismatch: %s", got.Data)
	}
}

func TestRedis_IgnoresOtherUsers(t *testing.T) {
	client := testClient(t)
	ctx := t.Context()

	node := NewRedis(ctx, client)
	defer node.Close()

	if err := node.Subscribe(1); err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	time.Sleep(100 * time.Millisecond)

	if err := node.Publish(ctx, 2, []byte(`{"message":"not yours"}`)); err != nil {
		t.Fatalf("publish: %v", err)
	}

	select {
	case d := <-node.Delivery():
		t.Fatalf("received traffic for an unsubscribed user: %+v", d)
	case <-time.After(300 * time.Millisecond):
	}
}

func TestRedis_UnsubscribeStopsDelivery(t *testing.T) {
	client := testClient(t)
	ctx := t.Context()

	node := NewRedis(ctx, client)
	defer node.Close()

	const userID = int64(4343)
	if err := node.Subscribe(userID); err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	time.Sleep(100 * time.Millisecond)
	if err := node.Unsubscribe(userID); err != nil {
		t.Fatalf("unsubscribe: %v", err)
	}
	time.Sleep(100 * time.Millisecond)

	if err := node.Publish(ctx, userID, []byte(`{"message":"gone"}`)); err != nil {
		t.Fatalf("publish: %v", err)
	}

	select {
	case d := <-node.Delivery():
		t.Fatalf("still receiving after unsubscribe: %+v", d)
	case <-time.After(300 * time.Millisecond):
	}
}

func TestRedis_SubscriptionIsRefCounted(t *testing.T) {
	client := testClient(t)
	ctx := t.Context()

	node := NewRedis(ctx, client)
	defer node.Close()

	const userID = int64(4444)
	if err := node.Subscribe(userID); err != nil {
		t.Fatalf("first subscribe: %v", err)
	}
	if err := node.Subscribe(userID); err != nil {
		t.Fatalf("second subscribe: %v", err)
	}

	if err := node.Unsubscribe(userID); err != nil {
		t.Fatalf("unsubscribe: %v", err)
	}
	time.Sleep(100 * time.Millisecond)

	if err := node.Publish(ctx, userID, []byte(`{"message":"still subscribed"}`)); err != nil {
		t.Fatalf("publish: %v", err)
	}

	if _, ok := waitForDelivery(t, node.Delivery()); !ok {
		t.Fatal("dropped the subscription while a connection was still open")
	}

	if err := node.Unsubscribe(userID); err != nil {
		t.Fatalf("final unsubscribe: %v", err)
	}
	time.Sleep(100 * time.Millisecond)

	if err := node.Publish(ctx, userID, []byte(`{"message":"gone"}`)); err != nil {
		t.Fatalf("publish: %v", err)
	}
	select {
	case d := <-node.Delivery():
		t.Fatalf("still receiving after the last connection closed: %+v", d)
	case <-time.After(300 * time.Millisecond):
	}
}

func TestRedis_UnsubscribeUnknownUserIsNoop(t *testing.T) {
	client := testClient(t)
	node := NewRedis(t.Context(), client)
	defer node.Close()

	if err := node.Unsubscribe(9999); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRedis_CloseIsIdempotent(t *testing.T) {
	client := testClient(t)
	node := NewRedis(t.Context(), client)

	if err := node.Close(); err != nil {
		t.Fatalf("first close: %v", err)
	}
	if err := node.Close(); err != nil {
		t.Errorf("second close: %v", err)
	}
}

func TestUserFromChannel(t *testing.T) {
	tests := []struct {
		channel string
		want    int64
		ok      bool
	}{
		{channel: "ws:u:42", want: 42, ok: true},
		{channel: "ws:u:-1", want: -1, ok: true},
		{channel: "ws:u:abc", ok: false},
		{channel: "ws:u:", ok: false},
		{channel: "other:42", ok: false},
	}

	for _, tt := range tests {
		got, ok := userFromChannel(tt.channel)
		if ok != tt.ok || (ok && got != tt.want) {
			t.Errorf("userFromChannel(%q) = (%d, %v), want (%d, %v)",
				tt.channel, got, ok, tt.want, tt.ok)
		}
	}
}
