package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gliedabrennung/sedna/internal/entity"
	"github.com/gliedabrennung/sedna/internal/testutil"
)

type stubMessageRepo struct {
	chats   []*entity.Chat
	listErr error
	gotUser int64
	history []*entity.Message
}

func (s *stubMessageRepo) NewMessageID() string { return "id" }

func (s *stubMessageRepo) Save(context.Context, *entity.Message) error { return nil }

func (s *stubMessageRepo) GetChatHistory(context.Context, string, int, string) ([]*entity.Message, string, error) {
	return s.history, "", nil
}

func (s *stubMessageRepo) ListChats(_ context.Context, userID int64, _ int) ([]*entity.Chat, error) {
	s.gotUser = userID
	return s.chats, s.listErr
}

func usersWith(t *testing.T, names ...string) *testutil.MockUserRepo {
	t.Helper()
	repo := testutil.NewMockUserRepo()
	for _, n := range names {
		if err := repo.Create(context.Background(), &entity.User{Username: n}); err != nil {
			t.Fatalf("seed %s: %v", n, err)
		}
	}
	return repo
}

func TestChats_AttachesPeerNames(t *testing.T) {
	users := usersWith(t, "alice", "bob")
	alice := users.Users["alice"]
	bob := users.Users["bob"]

	repo := &stubMessageRepo{chats: []*entity.Chat{
		{PeerID: bob.ID, ChatID: entity.MakeChatID(alice.ID, bob.ID), LastMessage: "hi"},
	}}

	uc := NewMessageUseCase(repo)
	uc.SetUsers(users)

	got, err := uc.Chats(context.Background(), alice.ID)
	if err != nil {
		t.Fatalf("chats: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 chat, got %d", len(got))
	}
	if got[0].PeerUsername != "bob" {
		t.Errorf("expected the peer name to be filled in, got %q", got[0].PeerUsername)
	}
	if repo.gotUser != alice.ID {
		t.Errorf("expected the list to be scoped to the caller, got %d", repo.gotUser)
	}
}

func TestChats_DropsPeersThatNoLongerExist(t *testing.T) {
	users := usersWith(t, "alice")
	alice := users.Users["alice"]

	repo := &stubMessageRepo{chats: []*entity.Chat{
		{PeerID: alice.ID + 999, LastMessage: "from a deleted account"},
	}}

	uc := NewMessageUseCase(repo)
	uc.SetUsers(users)

	got, err := uc.Chats(context.Background(), alice.ID)
	if err != nil {
		t.Fatalf("chats: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected chats with unknown peers to be dropped, got %+v", got)
	}
}

func TestChats_EmptyWithoutStorage(t *testing.T) {
	uc := NewMessageUseCase(nil)

	got, err := uc.Chats(context.Background(), 1)
	if err != nil {
		t.Fatalf("chats: %v", err)
	}
	if got == nil || len(got) != 0 {
		t.Errorf("expected an empty list, got %+v", got)
	}
}

func TestChats_PropagatesRepositoryFailure(t *testing.T) {
	repo := &stubMessageRepo{listErr: errors.New("scylla down")}
	uc := NewMessageUseCase(repo)

	if _, err := uc.Chats(context.Background(), 1); err == nil {
		t.Fatal("expected the failure to surface")
	}
}

func TestHistory_RejectsNonPositivePartner(t *testing.T) {
	uc := NewMessageUseCase(&stubMessageRepo{})

	for _, partner := range []int64{0, -1} {
		if _, _, err := uc.History(context.Background(), 1, partner, 50, ""); !errors.Is(err, ErrInvalidPartner) {
			t.Errorf("partner %d: expected ErrInvalidPartner, got %v", partner, err)
		}
	}
}

func TestHistory_NeverReturnsNilSlice(t *testing.T) {
	uc := NewMessageUseCase(&stubMessageRepo{history: nil})

	got, _, err := uc.History(context.Background(), 1, 2, 50, "")
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if got == nil {
		t.Error("expected an empty slice rather than nil, which serialises as null")
	}
}

func TestChats_WithoutUserRepositoryKeepsRawList(t *testing.T) {
	repo := &stubMessageRepo{chats: []*entity.Chat{
		{PeerID: 7, LastActivity: time.Now()},
	}}
	uc := NewMessageUseCase(repo)

	got, err := uc.Chats(context.Background(), 1)
	if err != nil {
		t.Fatalf("chats: %v", err)
	}
	if len(got) != 1 || got[0].PeerID != 7 {
		t.Errorf("unexpected list: %+v", got)
	}
}
