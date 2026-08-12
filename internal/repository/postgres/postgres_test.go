package postgres

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/gliedabrennung/sedna/internal/apperr"
	"github.com/gliedabrennung/sedna/internal/entity"
	"github.com/gliedabrennung/sedna/migrations"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set, skipping postgres integration test")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping: %v", err)
	}
	if err := Migrate(ctx, pool, migrations.SQL); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func uniqueName(t *testing.T, prefix string) string {
	t.Helper()
	return fmt.Sprintf("%s%d", prefix, time.Now().UnixNano()%1_000_000_000)
}

func createUser(t *testing.T, repo *Repository, username string) *entity.User {
	t.Helper()
	user := &entity.User{Username: username, PasswordHash: "hash"}
	if err := repo.Create(context.Background(), user); err != nil {
		t.Fatalf("create %s: %v", username, err)
	}
	return user
}

func TestMigrate_IsIdempotent(t *testing.T) {
	pool := testPool(t)

	if err := Migrate(context.Background(), pool, migrations.SQL); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
}

func TestCreateAndGetByUsername(t *testing.T) {
	repo := NewPostgresRepository(testPool(t))
	name := uniqueName(t, "user")

	created := createUser(t, repo, name)
	if created.ID == 0 {
		t.Fatal("expected an assigned id")
	}
	if created.CreatedAt.IsZero() {
		t.Error("expected created_at to be populated")
	}

	got, err := repo.GetByUsername(context.Background(), name)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ID != created.ID || got.PasswordHash != "hash" {
		t.Errorf("unexpected user: %+v", got)
	}
}

func TestGetByUsername_IsCaseInsensitive(t *testing.T) {
	repo := NewPostgresRepository(testPool(t))
	name := uniqueName(t, "MixedCase")
	created := createUser(t, repo, name)

	for _, variant := range []string{name, lower(name), upper(name)} {
		got, err := repo.GetByUsername(context.Background(), variant)
		if err != nil {
			t.Fatalf("lookup %q: %v", variant, err)
		}
		if got.ID != created.ID {
			t.Errorf("lookup %q returned a different user", variant)
		}
	}
}

func TestCreate_RejectsCaseInsensitiveDuplicate(t *testing.T) {
	repo := NewPostgresRepository(testPool(t))
	name := uniqueName(t, "Dup")
	createUser(t, repo, name)

	err := repo.Create(context.Background(),
		&entity.User{Username: lower(name), PasswordHash: "hash"})
	if !errors.Is(err, apperr.ErrUserAlreadyExists) {
		t.Fatalf("expected ErrUserAlreadyExists, got %v", err)
	}
}

func TestGetByUsername_NotFound(t *testing.T) {
	repo := NewPostgresRepository(testPool(t))
	_, err := repo.GetByUsername(context.Background(), uniqueName(t, "missing"))
	if !errors.Is(err, apperr.ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}
}

func TestSearch_FindsByUsernameAndID(t *testing.T) {
	repo := NewPostgresRepository(testPool(t))
	name := uniqueName(t, "searchable")
	created := createUser(t, repo, name)

	byName, err := repo.Search(context.Background(), name)
	if err != nil {
		t.Fatalf("search by name: %v", err)
	}
	if len(byName) != 1 || byName[0].ID != created.ID {
		t.Errorf("expected to find %s, got %+v", name, byName)
	}

	byID, err := repo.Search(context.Background(), fmt.Sprint(created.ID))
	if err != nil {
		t.Fatalf("search by id: %v", err)
	}
	if len(byID) == 0 {
		t.Error("expected to find the user by id")
	}
}

func TestSearch_EscapesLikeWildcards(t *testing.T) {
	repo := NewPostgresRepository(testPool(t))
	createUser(t, repo, uniqueName(t, "wildcardvictim"))

	for _, pattern := range []string{"%", "_", "%%%", "___", "\\"} {
		got, err := repo.Search(context.Background(), pattern)
		if err != nil {
			t.Fatalf("search %q: %v", pattern, err)
		}
		if len(got) != 0 {
			t.Errorf("search %q matched %d users; wildcards must be literal", pattern, len(got))
		}
	}
}

func TestSearch_NeverReturnsPasswordHash(t *testing.T) {
	repo := NewPostgresRepository(testPool(t))
	name := uniqueName(t, "secretive")
	createUser(t, repo, name)

	got, err := repo.Search(context.Background(), name)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	for _, u := range got {
		if u.PasswordHash != "" {
			t.Error("search leaked a password hash")
		}
	}
}

func TestExists(t *testing.T) {
	repo := NewPostgresRepository(testPool(t))
	created := createUser(t, repo, uniqueName(t, "present"))

	ok, err := repo.Exists(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("exists: %v", err)
	}
	if !ok {
		t.Error("expected the user to exist")
	}

	ok, err = repo.Exists(context.Background(), -1)
	if err != nil {
		t.Fatalf("exists on a missing id: %v", err)
	}
	if ok {
		t.Error("expected a missing id to report false")
	}
}

func TestGetByIDs(t *testing.T) {
	repo := NewPostgresRepository(testPool(t))
	a := createUser(t, repo, uniqueName(t, "bulka"))
	b := createUser(t, repo, uniqueName(t, "bulkb"))

	got, err := repo.GetByIDs(context.Background(), []int64{a.ID, b.ID, -1})
	if err != nil {
		t.Fatalf("get by ids: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 users, got %d", len(got))
	}

	for _, u := range got {
		if u.CreatedAt.IsZero() || u.UpdatedAt.IsZero() {
			t.Errorf("user %d came back without timestamps", u.ID)
		}
	}
}

func TestGetByIDs_Empty(t *testing.T) {
	repo := NewPostgresRepository(testPool(t))
	got, err := repo.GetByIDs(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected no users, got %d", len(got))
	}
}

func lower(s string) string {
	out := []rune(s)
	for i, r := range out {
		if r >= 'A' && r <= 'Z' {
			out[i] = r + 32
		}
	}
	return string(out)
}

func upper(s string) string {
	out := []rune(s)
	for i, r := range out {
		if r >= 'a' && r <= 'z' {
			out[i] = r - 32
		}
	}
	return string(out)
}
