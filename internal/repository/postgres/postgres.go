package postgres

import (
	"context"
	"errors"

	"github.com/gliedabrennung/messenger-core/internal/entity"
	"github.com/gliedabrennung/messenger-core/internal/repository"
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
	err := repo.db.QueryRow(ctx, query, user.Username, user.Password).
		Scan(&user.ID, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == uniqueViolationCode {
			return repository.ErrUserAlreadyExists
		}
		return err
	}
	return nil
}

func (repo *Repository) GetByUsername(ctx context.Context, username string) (*entity.User, error) {
	query := `
		SELECT id, username, password, created_at, updated_at
		FROM users
		WHERE username = $1`
	user := &entity.User{}
	err := repo.db.QueryRow(ctx, query, username).
		Scan(&user.ID, &user.Username, &user.Password, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, repository.ErrUserNotFound
		}
		return nil, err
	}
	return user, nil
}
