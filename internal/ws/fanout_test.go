package ws

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/gliedabrennung/sedna/internal/entity"
)

type fakeFanout struct {
	mu          sync.Mutex
	published   []Delivery
	subscribed  []int64
	unsubbed    []int64
	publishErr  error
	deliveries  chan Delivery
	echoToLocal bool
}

func newFakeFanout() *fakeFanout {
	return &fakeFanout{deliveries: make(chan Delivery, 16)}
}

func (f *fakeFanout) Publish(_ context.Context, userID int64, data []byte) error {
	f.mu.Lock()
	if f.publishErr != nil {
		err := f.publishErr
		f.mu.Unlock()
		return err
	}
	f.published = append(f.published, Delivery{UserID: userID, Data: data})
	echo := f.echoToLocal
	f.mu.Unlock()

	if echo {
		f.deliveries <- Delivery{UserID: userID, Data: data}
	}
	return nil
}

func (f *fakeFanout) Subscribe(userID int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.subscribed = append(f.subscribed, userID)
	return nil
}

func (f *fakeFanout) Unsubscribe(userID int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.unsubbed = append(f.unsubbed, userID)
	return nil
}

func (f *fakeFanout) Delivery() <-chan Delivery { return f.deliveries }

func (f *fakeFanout) snapshot() ([]Delivery, []int64, []int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]Delivery(nil), f.published...),
		append([]int64(nil), f.subscribed...),
		append([]int64(nil), f.unsubbed...)
}

func fanoutHub(t *testing.T, f Fanout) (*Hub, context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	h := NewHubWithFanout(nil, f)
	go h.Run(ctx)
	return h, cancel
}

func TestHub_Send_GoesThroughFanout(t *testing.T) {
	f := newFakeFanout()
	h, cancel := fanoutHub(t, f)
	defer cancel()

	if !h.Send(DirectMessage{MessageID: "m1", From: 1, To: 2, Message: "hi"}) {
		t.Fatal("Send returned false")
	}

	published, _, _ := f.snapshot()
	if len(published) != 1 {
		t.Fatalf("expected 1 published message, got %d", len(published))
	}
	if published[0].UserID != 2 {
		t.Errorf("expected publish addressed to user 2, got %d", published[0].UserID)
	}

	var got DirectMessage
	if err := json.Unmarshal(published[0].Data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Message != "hi" || got.MessageID != "m1" || got.From != 1 {
		t.Errorf("payload mismatch: %+v", got)
	}
}

func TestHub_FanoutDelivery_ReachesLocalClient(t *testing.T) {
	f := newFakeFanout()
	h, cancel := fanoutHub(t, f)
	defer cancel()

	c := testClient(2)
	registerClient(t, h, c)

	f.deliveries <- Delivery{UserID: 2, Data: []byte(`{"message":"from another node"}`)}

	select {
	case got := <-c.send:
		var msg DirectMessage
		if err := json.Unmarshal(got, &msg); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if msg.Message != "from another node" {
			t.Errorf("got %q", msg.Message)
		}
	case <-time.After(time.Second):
		t.Fatal("fanout delivery never reached the local client")
	}
}

func TestHub_RegisterUnregister_TracksFanoutSubscription(t *testing.T) {
	f := newFakeFanout()
	h, cancel := fanoutHub(t, f)
	defer cancel()

	c := testClient(7)
	registerClient(t, h, c)
	h.Unregister(c)

	_, subscribed, unsubbed := f.snapshot()
	if len(subscribed) != 1 || subscribed[0] != 7 {
		t.Errorf("expected a subscription for user 7, got %v", subscribed)
	}
	if len(unsubbed) != 1 || unsubbed[0] != 7 {
		t.Errorf("expected an unsubscription for user 7, got %v", unsubbed)
	}
}

func TestHub_Send_FallsBackToLocalWhenFanoutFails(t *testing.T) {
	f := newFakeFanout()
	f.publishErr = errors.New("redis down")
	h, cancel := fanoutHub(t, f)
	defer cancel()

	c := testClient(2)
	registerClient(t, h, c)

	if !h.Send(DirectMessage{From: 1, To: 2, Message: "still here"}) {
		t.Fatal("Send returned false")
	}

	select {
	case got := <-c.send:
		var msg DirectMessage
		if err := json.Unmarshal(got, &msg); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if msg.Message != "still here" {
			t.Errorf("got %q", msg.Message)
		}
	case <-time.After(time.Second):
		t.Fatal("no local fallback delivery")
	}
}

func TestHub_Send_NoDoubleDelivery(t *testing.T) {
	f := newFakeFanout()
	f.echoToLocal = true
	h, cancel := fanoutHub(t, f)
	defer cancel()

	c := testClient(2)
	registerClient(t, h, c)

	if !h.Send(DirectMessage{From: 1, To: 2, Message: "once"}) {
		t.Fatal("Send returned false")
	}

	select {
	case <-c.send:
	case <-time.After(time.Second):
		t.Fatal("message never delivered")
	}

	select {
	case extra := <-c.send:
		t.Fatalf("message delivered twice: %s", extra)
	case <-time.After(200 * time.Millisecond):
	}
}

type recordingRepo struct {
	mu    sync.Mutex
	saved []*entity.Message
	done  chan struct{}
	once  sync.Once
}

func (r *recordingRepo) NewMessageID() string { return "repo-id" }

func (r *recordingRepo) Save(_ context.Context, msg *entity.Message) error {
	r.mu.Lock()
	r.saved = append(r.saved, msg)
	r.mu.Unlock()
	r.once.Do(func() { close(r.done) })
	return nil
}

func (r *recordingRepo) GetChatHistory(context.Context, string, int, string) ([]*entity.Message, string, error) {
	return nil, "", nil
}

func TestHub_Persist_WritesAsynchronously(t *testing.T) {
	repo := &recordingRepo{done: make(chan struct{})}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	h := NewHub(repo)
	go h.Run(ctx)

	h.Persist(&entity.Message{ChatID: "1:2", MessageID: "m1", Content: "async"}, "c1")

	select {
	case <-repo.done:
	case <-time.After(2 * time.Second):
		t.Fatal("message was never persisted")
	}

	repo.mu.Lock()
	defer repo.mu.Unlock()
	if len(repo.saved) != 1 || repo.saved[0].MessageID != "m1" {
		t.Errorf("unexpected saved messages: %+v", repo.saved)
	}
}

func TestHub_Persist_NoRepoIsNoop(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	h := NewHub(nil)
	go h.Run(ctx)

	done := make(chan struct{})
	go func() {
		h.Persist(&entity.Message{ChatID: "1:2", MessageID: "m1"}, "c1")
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Persist blocked with no repository configured")
	}
}

func TestHub_NewMessageID_UsesRepo(t *testing.T) {
	h := NewHub(&recordingRepo{done: make(chan struct{})})
	if got := h.NewMessageID(); got != "repo-id" {
		t.Errorf("expected the repository to mint the id, got %q", got)
	}
}

func TestHub_NewMessageID_FallbackIsUnique(t *testing.T) {
	h := NewHub(nil)
	seen := make(map[string]bool, 100)
	for range 100 {
		id := h.NewMessageID()
		if seen[id] {
			t.Fatalf("duplicate fallback id %q", id)
		}
		seen[id] = true
	}
}

func TestHub_Stop_IsIdempotent(t *testing.T) {
	h := NewHub(nil)
	h.Stop()
	h.Stop()
}
