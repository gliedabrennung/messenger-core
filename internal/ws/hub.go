package ws

import (
	"context"
	"encoding/json"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gliedabrennung/sedna/internal/common/logger"
	"github.com/gliedabrennung/sedna/internal/domain"
	"github.com/gliedabrennung/sedna/internal/entity"
)

const (
	shardCount = 32

	persistWorkers   = 16
	persistQueueSize = 4096
	persistTimeout   = 5 * time.Second

	persistEnqueueWait = 2 * time.Second
)

var localIDSeq uint64

type Delivery struct {
	UserID int64
	Data   []byte
}

type Fanout interface {
	Publish(ctx context.Context, userID int64, data []byte) error
	Subscribe(userID int64) error
	Unsubscribe(userID int64) error
	Delivery() <-chan Delivery
}

type RecipientValidator interface {
	Exists(ctx context.Context, userID int64) (bool, error)
}

type persistJob struct {
	msg      *entity.Message
	clientID string
}

type Hub struct {
	shards    [shardCount]*shard
	done      chan struct{}
	msgRepo   domain.MessageRepository
	fanout    Fanout
	recipient RecipientValidator

	persist   chan persistJob
	stopOnce  sync.Once
	baseCtx   context.Context
	baseReady chan struct{}
}

type shard struct {
	clients    map[int64]map[*Client]struct{}
	register   chan *Client
	unregister chan *Client
	out        chan Delivery
	done       chan struct{}
}

type DirectMessage struct {
	Type      string    `json:"type"`
	MessageID string    `json:"message_id"`
	From      int64     `json:"from"`
	To        int64     `json:"to"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
}

func NewHub(msgRepo domain.MessageRepository) *Hub {
	return NewHubWithFanout(msgRepo, nil)
}

func NewHubWithFanout(msgRepo domain.MessageRepository, f Fanout) *Hub {
	h := &Hub{
		done:      make(chan struct{}),
		msgRepo:   msgRepo,
		fanout:    f,
		persist:   make(chan persistJob, persistQueueSize),
		baseCtx:   context.Background(),
		baseReady: make(chan struct{}),
	}
	for i := range h.shards {
		h.shards[i] = &shard{
			clients:    make(map[int64]map[*Client]struct{}),
			register:   make(chan *Client),
			unregister: make(chan *Client),
			out:        make(chan Delivery, 256),
			done:       h.done,
		}
	}
	return h
}

func (h *Hub) getShard(userID int64) *shard {
	idx := userID % shardCount
	if idx < 0 {
		idx = -idx
	}
	return h.shards[idx]
}

func (h *Hub) Register(c *Client) {
	s := h.getShard(c.id)
	select {
	case s.register <- c:
		if h.fanout != nil {
			if err := h.fanout.Subscribe(c.id); err != nil {
				logger.Errorf("hub: fanout subscribe user %d: %v", c.id, err)
			}
		}
	case <-h.done:
	}
}

func (h *Hub) Unregister(c *Client) {
	s := h.getShard(c.id)
	select {
	case s.unregister <- c:
		if h.fanout != nil {
			if err := h.fanout.Unsubscribe(c.id); err != nil {
				logger.Errorf("hub: fanout unsubscribe user %d: %v", c.id, err)
			}
		}
	case <-h.done:
	}
}

func (h *Hub) Send(msg DirectMessage) bool {
	data, err := json.Marshal(msg)
	if err != nil {
		logger.Errorf("hub: marshal direct message: %v", err)
		return true
	}

	ok := h.route(msg.To, data)
	if msg.From != msg.To {
		h.route(msg.From, data)
	}
	return ok
}

func (h *Hub) route(userID int64, data []byte) bool {
	if h.fanout != nil {
		ctx, cancel := context.WithTimeout(h.context(), writeWait)
		err := h.fanout.Publish(ctx, userID, data)
		cancel()
		if err == nil {
			return true
		}

		logger.Errorf("hub: fanout publish to user %d: %v", userID, err)
	}

	return h.deliverLocal(Delivery{UserID: userID, Data: data})
}

func (h *Hub) deliverLocal(d Delivery) bool {
	s := h.getShard(d.UserID)
	select {
	case s.out <- d:
		return true
	case <-h.done:
		return false
	}
}

func (h *Hub) SetRecipientValidator(v RecipientValidator) {
	h.recipient = v
}

func (h *Hub) RecipientExists(ctx context.Context, userID int64) bool {
	if h.recipient == nil {
		return true
	}
	ok, err := h.recipient.Exists(ctx, userID)
	if err != nil {
		logger.Errorf("hub: recipient lookup for user %d: %v", userID, err)
		return true
	}
	return ok
}

func (h *Hub) Persist(msg *entity.Message, clientID string) {
	if h.msgRepo == nil {
		return
	}
	job := persistJob{msg: msg, clientID: clientID}

	select {
	case h.persist <- job:
		return
	default:
	}

	timer := time.NewTimer(persistEnqueueWait)
	defer timer.Stop()
	select {
	case h.persist <- job:
	case <-h.done:
	case <-timer.C:
		logger.Errorf("hub: persist queue full, dropped message %s for chat %s",
			msg.MessageID, msg.ChatID)
		h.NotifySender(msg.FromID, storeFailure(msg, clientID))
	}
}

func storeFailure(msg *entity.Message, clientID string) ErrorEvent {
	ev := NewErrorEvent(CodeMessageNotStored,
		"message was delivered but could not be stored", clientID)
	ev.MessageID = msg.MessageID
	ev.To = msg.ToID
	return ev
}

func (h *Hub) NotifySender(userID int64, event any) {
	data, err := json.Marshal(event)
	if err != nil {
		logger.Errorf("hub: marshal event for user %d: %v", userID, err)
		return
	}
	h.deliverLocal(Delivery{UserID: userID, Data: data})
}

func (h *Hub) Done() <-chan struct{} {
	return h.done
}

func (h *Hub) Stop() {
	h.stopOnce.Do(func() { close(h.done) })
}

func (h *Hub) MsgRepo() domain.MessageRepository {
	return h.msgRepo
}

func (h *Hub) NewMessageID() string {
	if h.msgRepo != nil {
		return h.msgRepo.NewMessageID()
	}

	return "local-" + strconv.FormatUint(atomic.AddUint64(&localIDSeq, 1), 36) +
		"-" + strconv.FormatInt(time.Now().UnixNano(), 36)
}

func (h *Hub) context() context.Context {
	select {
	case <-h.baseReady:
		return h.baseCtx
	default:
		return context.Background()
	}
}

func (h *Hub) Run(ctx context.Context) {
	h.baseCtx = ctx
	close(h.baseReady)

	go func() {
		<-ctx.Done()
		h.Stop()
	}()

	var wg sync.WaitGroup
	for _, s := range h.shards {
		wg.Add(1)
		go func(s *shard) {
			defer wg.Done()
			s.run()
		}(s)
	}

	for i := 0; i < persistWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			h.runPersist(ctx)
		}()
	}

	if h.fanout != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			h.runFanout(ctx)
		}()
	}

	wg.Wait()
	logger.Info("hub: shutdown complete")
}

func (h *Hub) runPersist(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-h.done:
			return
		case job := <-h.persist:
			saveCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), persistTimeout)
			err := h.msgRepo.Save(saveCtx, job.msg)
			cancel()
			if err != nil {
				logger.Errorf("hub: save message %s for chat %s: %v",
					job.msg.MessageID, job.msg.ChatID, err)
				h.NotifySender(job.msg.FromID, storeFailure(job.msg, job.clientID))
			}
		}
	}
}

func (h *Hub) runFanout(ctx context.Context) {
	in := h.fanout.Delivery()
	for {
		select {
		case <-ctx.Done():
			return
		case <-h.done:
			return
		case d, ok := <-in:
			if !ok {
				return
			}
			h.deliverLocal(d)
		}
	}
}

func (s *shard) run() {
	for {
		select {
		case <-s.done:
			s.shutdown()
			return
		case client := <-s.register:
			conns := s.clients[client.id]
			if conns == nil {
				conns = make(map[*Client]struct{})
				s.clients[client.id] = conns
			}
			conns[client] = struct{}{}
		case client := <-s.unregister:
			s.drop(client)
		case d := <-s.out:
			for client := range s.clients[d.UserID] {
				select {
				case client.send <- d.Data:
				default:

					s.drop(client)
				}
			}
		}
	}
}

func (s *shard) drop(client *Client) {
	conns, ok := s.clients[client.id]
	if !ok {
		return
	}
	if _, ok := conns[client]; !ok {
		return
	}
	delete(conns, client)
	if len(conns) == 0 {
		delete(s.clients, client.id)
	}
	close(client.send)
}

func (s *shard) shutdown() {
	for id, conns := range s.clients {
		for client := range conns {
			close(client.send)
		}
		delete(s.clients, id)
	}
}
