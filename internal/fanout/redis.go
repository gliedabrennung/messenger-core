package fanout

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/gliedabrennung/sedna/internal/common/logger"
	"github.com/gliedabrennung/sedna/internal/ws"
	"github.com/redis/go-redis/v9"
)

const (
	channelPrefix = "ws:u:"

	deliveryBuffer = 1024
)

type Redis struct {
	client *redis.Client
	pubsub *redis.PubSub
	out    chan ws.Delivery

	baseCtx context.Context

	mu   sync.Mutex
	refs map[int64]int

	closeOnce sync.Once
	closed    chan struct{}
	wg        sync.WaitGroup
}

func NewRedis(ctx context.Context, client *redis.Client) *Redis {
	r := &Redis{
		client:  client,
		pubsub:  client.Subscribe(ctx),
		out:     make(chan ws.Delivery, deliveryBuffer),
		baseCtx: ctx,
		refs:    make(map[int64]int),
		closed:  make(chan struct{}),
	}

	r.wg.Add(1)
	go r.run(ctx)
	return r
}

func channelFor(userID int64) string {
	return channelPrefix + strconv.FormatInt(userID, 10)
}

func userFromChannel(channel string) (int64, bool) {
	raw, ok := strings.CutPrefix(channel, channelPrefix)
	if !ok {
		return 0, false
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, false
	}
	return id, true
}

func (r *Redis) Publish(ctx context.Context, userID int64, data []byte) error {
	if err := r.client.Publish(ctx, channelFor(userID), data).Err(); err != nil {
		return fmt.Errorf("fanout: publish to user %d: %w", userID, err)
	}
	return nil
}

func (r *Redis) Subscribe(userID int64) error {
	r.mu.Lock()
	r.refs[userID]++
	first := r.refs[userID] == 1
	r.mu.Unlock()

	if !first {
		return nil
	}

	if err := r.pubsub.Subscribe(r.baseCtx, channelFor(userID)); err != nil {
		r.mu.Lock()
		r.refs[userID]--
		if r.refs[userID] <= 0 {
			delete(r.refs, userID)
		}
		r.mu.Unlock()
		return fmt.Errorf("fanout: subscribe user %d: %w", userID, err)
	}
	return nil
}

func (r *Redis) Unsubscribe(userID int64) error {
	r.mu.Lock()
	n, ok := r.refs[userID]
	if !ok {
		r.mu.Unlock()
		return nil
	}
	n--
	if n > 0 {
		r.refs[userID] = n
		r.mu.Unlock()
		return nil
	}
	delete(r.refs, userID)
	r.mu.Unlock()

	if err := r.pubsub.Unsubscribe(r.baseCtx, channelFor(userID)); err != nil {
		return fmt.Errorf("fanout: unsubscribe user %d: %w", userID, err)
	}
	return nil
}

func (r *Redis) Delivery() <-chan ws.Delivery {
	return r.out
}

func (r *Redis) Close() error {
	var err error
	r.closeOnce.Do(func() {
		close(r.closed)
		err = r.pubsub.Close()
		r.wg.Wait()
	})
	return err
}

func (r *Redis) run(ctx context.Context) {
	defer r.wg.Done()
	defer close(r.out)

	in := r.pubsub.Channel(redis.WithChannelSize(deliveryBuffer))

	for {
		select {
		case <-ctx.Done():
			return
		case <-r.closed:
			return
		case msg, ok := <-in:
			if !ok {
				return
			}
			userID, ok := userFromChannel(msg.Channel)
			if !ok {
				logger.Errorf("fanout: unexpected channel %q", msg.Channel)
				continue
			}
			select {
			case r.out <- ws.Delivery{UserID: userID, Data: []byte(msg.Payload)}:
			case <-ctx.Done():
				return
			case <-r.closed:
				return
			}
		}
	}
}
