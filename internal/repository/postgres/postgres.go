package postgres

import (
	"context"
	"errors"
	"fmt"

	"strconv"
	"strings"

	"github.com/gliedabrennung/sedna/internal/apperr"
	"github.com/gliedabrennung/sedna/internal/entity"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const uniqueViolationCode = "23505"

type Repository struct {
	db *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{db: pool}
}

func (repo *Repository) Create(ctx context.Context, user *entity.User) error {
	query := `
		INSERT INTO users (username, password)
		VALUES ($1, $2)
		RETURNING id, created_at, updated_at`
	err := repo.db.QueryRow(ctx, query, user.Username, user.PasswordHash).
		Scan(&user.ID, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == uniqueViolationCode {
			return apperr.ErrUserAlreadyExists
		}
		return fmt.Errorf("postgres: create user: %w", err)
	}
	return nil
}

func (repo *Repository) GetByUsername(ctx context.Context, username string) (*entity.User, error) {
	query := `
		SELECT id, username, password, created_at, updated_at
		FROM users
		WHERE lower(username) = lower($1)`
	user := &entity.User{}
	err := repo.db.QueryRow(ctx, query, username).
		Scan(&user.ID, &user.Username, &user.PasswordHash, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperr.ErrUserNotFound
		}
		return nil, fmt.Errorf("postgres: get user by username: %w", err)
	}
	return user, nil
}

func escapeLikePattern(s string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return replacer.Replace(s)
}

func (repo *Repository) Search(ctx context.Context, query string) ([]entity.User, error) {
	var users []entity.User

	pattern := "%" + escapeLikePattern(query) + "%"
	id, err := strconv.ParseInt(query, 10, 64)
	var q string
	var args []any

	if err == nil {
		q = `SELECT id, username, created_at, updated_at FROM users
		     WHERE id = $1 OR username ILIKE $2 LIMIT 20`
		args = []any{id, pattern}
	} else {
		q = `SELECT id, username, created_at, updated_at FROM users
		     WHERE username ILIKE $1 LIMIT 20`
		args = []any{pattern}
	}

	rows, err := repo.db.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres: search users: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var u entity.User
		if err := rows.Scan(&u.ID, &u.Username, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, fmt.Errorf("postgres: scan user: %w", err)
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (repo *Repository) Exists(ctx context.Context, userID int64) (bool, error) {
	var exists bool
	err := repo.db.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM users WHERE id = $1)`, userID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("postgres: user exists: %w", err)
	}
	return exists, nil
}

func (repo *Repository) GetByIDs(ctx context.Context, ids []int64) ([]entity.User, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	q := `SELECT id, username FROM users WHERE id = ANY($1)`
	rows, err := repo.db.Query(ctx, q, ids)
	if err != nil {
		return nil, fmt.Errorf("postgres: get users by ids: %w", err)
	}
	defer rows.Close()

	var users []entity.User
	for rows.Next() {
		var u entity.User
		if err := rows.Scan(&u.ID, &u.Username); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}
