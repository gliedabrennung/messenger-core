package ws

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gliedabrennung/sedna/internal/entity"
)

type stubValidator struct {
	known map[int64]bool
	err   error
	mu    sync.Mutex
	calls int
}

func (s *stubValidator) Exists(_ context.Context, userID int64) (bool, error) {
	s.mu.Lock()
	s.calls++
	s.mu.Unlock()
	if s.err != nil {
		return false, s.err
	}
	return s.known[userID], nil
}

func (s *stubValidator) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

func handleHub(t *testing.T, repo *recordingRepo, v RecipientValidator) (*Hub, *Client) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	var h *Hub
	if repo == nil {
		h = NewHub(nil)
	} else {
		h = NewHub(repo)
	}
	if v != nil {
		h.SetRecipientValidator(v)
	}
	go h.Run(ctx)

	c := testClient(1)
	c.limiter = newTokenBucket(sendRate, sendBurst)
	c.knownRecipients = make(map[int64]struct{})
	registerClient(t, h, c)

	return h, c
}

func nextFrame(t *testing.T, c *Client) map[string]any {
	t.Helper()
	select {
	case raw := <-c.send:
		var frame map[string]any
		if err := json.Unmarshal(raw, &frame); err != nil {
			t.Fatalf("unmarshal frame: %v", err)
		}
		return frame
	case <-time.After(time.Second):
		t.Fatal("expected a frame, got none")
		return nil
	}
}

func send(t *testing.T, c *Client, payload string) bool {
	t.Helper()
	return c.handleMessage(context.Background(), []byte(payload))
}

func TestHandleMessage_AcksSender(t *testing.T) {
	v := &stubValidator{known: map[int64]bool{2: true}}
	_, c := handleHub(t, nil, v)

	if !send(t, c, `{"to":2,"message":"hi","client_id":"c-1"}`) {
		t.Fatal("handleMessage stopped the read loop")
	}

	frame := nextFrame(t, c)
	if frame["type"] != EventAck {
		t.Fatalf("expected an ack, got %v", frame)
	}
	if frame["client_id"] != "c-1" {
		t.Errorf("ack must echo client_id, got %v", frame["client_id"])
	}
	if frame["message_id"] == "" || frame["message_id"] == nil {
		t.Error("ack must carry the stored message id")
	}
}

func TestHandleMessage_RejectsUnknownRecipient(t *testing.T) {
	repo := &recordingRepo{done: make(chan struct{})}
	v := &stubValidator{known: map[int64]bool{2: true}}
	_, c := handleHub(t, repo, v)

	send(t, c, `{"to":999,"message":"hi","client_id":"c-1"}`)

	frame := nextFrame(t, c)
	if frame["type"] != EventError || frame["code"] != CodeUnknownRecipient {
		t.Fatalf("expected %s, got %v", CodeUnknownRecipient, frame)
	}

	repo.mu.Lock()
	defer repo.mu.Unlock()
	if len(repo.saved) != 0 {
		t.Errorf("a message for an unknown recipient must not be stored, got %+v", repo.saved)
	}
}

func TestHandleMessage_RejectsSelfAndNonPositive(t *testing.T) {
	v := &stubValidator{known: map[int64]bool{1: true, 2: true}}
	_, c := handleHub(t, nil, v)

	for _, payload := range []string{
		`{"to":1,"message":"to myself"}`,
		`{"to":0,"message":"zero"}`,
		`{"to":-5,"message":"negative"}`,
	} {
		send(t, c, payload)
		frame := nextFrame(t, c)
		if frame["type"] != EventError || frame["code"] != CodeInvalidRecipient {
			t.Errorf("%s: expected %s, got %v", payload, CodeInvalidRecipient, frame)
		}
	}
}

func TestHandleMessage_CachesRecipientLookups(t *testing.T) {
	v := &stubValidator{known: map[int64]bool{2: true}}
	_, c := handleHub(t, nil, v)

	for range 5 {
		send(t, c, `{"to":2,"message":"hi"}`)
		nextFrame(t, c)
	}

	if got := v.callCount(); got != 1 {
		t.Errorf("expected 1 lookup for 5 messages, got %d", got)
	}
}

func TestHandleMessage_ValidatorErrorFailsOpen(t *testing.T) {
	v := &stubValidator{err: errors.New("db down")}
	_, c := handleHub(t, nil, v)

	send(t, c, `{"to":2,"message":"hi","client_id":"c-1"}`)

	frame := nextFrame(t, c)
	if frame["type"] != EventAck {
		t.Fatalf("expected the message to go through, got %v", frame)
	}
}

func TestHandleMessage_RateLimited(t *testing.T) {
	v := &stubValidator{known: map[int64]bool{2: true}}
	_, c := handleHub(t, nil, v)

	acks, rejects := 0, 0
	for range sendBurst + 10 {
		send(t, c, `{"to":2,"message":"flood"}`)
	}

	for {
		select {
		case raw := <-c.send:
			var frame map[string]any
			json.Unmarshal(raw, &frame)
			switch frame["type"] {
			case EventAck:
				acks++
			case EventError:
				if frame["code"] == CodeRateLimited {
					rejects++
				}
			}
			continue
		case <-time.After(150 * time.Millisecond):
		}
		break
	}

	if acks > sendBurst {
		t.Errorf("accepted %d messages, burst is %d", acks, sendBurst)
	}
	if rejects == 0 {
		t.Error("a flooding client was never told it is rate limited")
	}
}

func TestHandleMessage_RateLimitNoticeIsThrottled(t *testing.T) {
	v := &stubValidator{known: map[int64]bool{2: true}}
	_, c := handleHub(t, nil, v)

	for range sendBurst + 50 {
		send(t, c, `{"to":2,"message":"flood"}`)
	}

	notices := 0
	for {
		select {
		case raw := <-c.send:
			var frame map[string]any
			json.Unmarshal(raw, &frame)
			if frame["code"] == CodeRateLimited {
				notices++
			}
			continue
		case <-time.After(150 * time.Millisecond):
		}
		break
	}

	if notices > 1 {
		t.Errorf("expected at most one notice per interval, got %d", notices)
	}
}

func TestHandleMessage_RejectsBadPayloads(t *testing.T) {
	v := &stubValidator{known: map[int64]bool{2: true}}
	_, c := handleHub(t, nil, v)

	tests := []struct {
		name    string
		payload string
		code    string
	}{
		{name: "not json", payload: `{oops`, code: CodeInvalidPayload},
		{name: "empty text", payload: `{"to":2,"message":"   "}`, code: CodeInvalidPayload},
		{name: "non numeric recipient", payload: `{"to":"abc","message":"hi"}`, code: CodeInvalidPayload},
		{
			name:    "too long",
			payload: `{"to":2,"message":"` + strings.Repeat("a", maxTextLength+1) + `"}`,
			code:    CodeMessageTooLong,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			send(t, c, tt.payload)
			frame := nextFrame(t, c)
			if frame["type"] != EventError || frame["code"] != tt.code {
				t.Errorf("expected %s, got %v", tt.code, frame)
			}
		})
	}
}

func TestHandleMessage_AcceptsMaxLengthMultibyte(t *testing.T) {
	v := &stubValidator{known: map[int64]bool{2: true}}
	_, c := handleHub(t, nil, v)

	payload, err := json.Marshal(map[string]any{
		"to": 2, "message": strings.Repeat("я", maxTextLength),
	})
	if err != nil {
		t.Fatal(err)
	}

	send(t, c, string(payload))
	frame := nextFrame(t, c)
	if frame["type"] != EventAck {
		t.Fatalf("expected the message to be accepted, got %v", frame)
	}
}

type failingRepo struct{ recordingRepo }

func (f *failingRepo) Save(context.Context, *entity.Message) error {
	f.once.Do(func() { close(f.done) })
	return errors.New("scylla down")
}

func TestPersist_FailureNotifiesSender(t *testing.T) {
	repo := &failingRepo{recordingRepo{done: make(chan struct{})}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	h := NewHub(repo)
	go h.Run(ctx)

	c := testClient(1)
	registerClient(t, h, c)

	h.Persist(&entity.Message{
		ChatID: "1:2", MessageID: "m1", FromID: 1, ToID: 2, Content: "doomed",
	}, "c-9")

	frame := nextFrame(t, c)
	if frame["type"] != EventError || frame["code"] != CodeMessageNotStored {
		t.Fatalf("expected %s, got %v", CodeMessageNotStored, frame)
	}
	if frame["client_id"] != "c-9" {
		t.Errorf("failure must name the sender's message, got %v", frame["client_id"])
	}
	if frame["message_id"] != "m1" {
		t.Errorf("failure must name the stored id, got %v", frame["message_id"])
	}
}

func TestHandleMessage_NoValidatorSkipsCheck(t *testing.T) {
	_, c := handleHub(t, nil, nil)

	send(t, c, `{"to":12345,"message":"hi"}`)
	frame := nextFrame(t, c)
	if frame["type"] != EventAck {
		t.Fatalf("expected an ack with no validator configured, got %v", frame)
	}
}

func TestHandleMessage_TruncatesOversizedClientID(t *testing.T) {
	v := &stubValidator{known: map[int64]bool{2: true}}
	_, c := handleHub(t, nil, v)

	long := strings.Repeat("x", maxClientIDLength*3)
	payload, _ := json.Marshal(map[string]any{"to": 2, "message": "hi", "client_id": long})

	send(t, c, string(payload))
	frame := nextFrame(t, c)

	echoed, _ := frame["client_id"].(string)
	if len(echoed) != maxClientIDLength {
		t.Errorf("expected client_id truncated to %d, got %d", maxClientIDLength, len(echoed))
	}
}

func TestHandleMessage_KnownRecipientsCacheIsBounded(t *testing.T) {
	known := make(map[int64]bool, maxKnownRecipients+10)
	for i := int64(2); i < int64(maxKnownRecipients)+12; i++ {
		known[i] = true
	}
	v := &stubValidator{known: known}
	_, c := handleHub(t, nil, v)
	c.limiter = newTokenBucket(1e9, 1e9)

	for i := int64(2); i < int64(maxKnownRecipients)+12; i++ {
		payload, _ := json.Marshal(map[string]any{"to": i, "message": "hi"})
		send(t, c, string(payload))
	}

	if len(c.knownRecipients) > maxKnownRecipients {
		t.Errorf("recipient cache grew to %d, cap is %d", len(c.knownRecipients), maxKnownRecipients)
	}
}
