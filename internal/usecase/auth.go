package usecase

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/gliedabrennung/sedna/internal/apperr"
	"github.com/gliedabrennung/sedna/internal/domain"
	"github.com/gliedabrennung/sedna/internal/entity"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type AuthUseCase struct {
	repo      domain.UserRepository
	jwtSecret string
	jwtTTL    time.Duration
}

var dummyPasswordHash = sync.OnceValue(func() []byte {
	hash, err := bcrypt.GenerateFromPassword([]byte("sedna-timing-equalizer"), bcrypt.DefaultCost)
	if err != nil {
		return []byte("$2a$10$invalidinvalidinvalidinvalidinvalidinvalidinvalidinvalidinv")
	}
	return hash
})

func NewAuthUseCase(repo domain.UserRepository, jwtSecret string, jwtTTL time.Duration) *AuthUseCase {
	return &AuthUseCase{
		repo:      repo,
		jwtSecret: jwtSecret,
		jwtTTL:    jwtTTL,
	}
}

func validUsername(username string) bool {
	runeCount := utf8.RuneCountInString(username)
	if runeCount < 3 || runeCount > 24 {
		return false
	}
	for _, r := range username {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r):
		case r == '_', r == '.', r == '-':
		default:
			return false
		}
	}
	return true
}

func (a *AuthUseCase) Register(ctx context.Context, username, password string) (*entity.User, error) {
	username = strings.TrimSpace(username)
	if !validUsername(username) {
		return nil, apperr.ErrInvalidUsername
	}
	if len(password) < 8 || len(password) > 72 {
		return nil, apperr.ErrInvalidPassword
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("register: hash password: %w", err)
	}

	user := &entity.User{
		Username:     username,
		PasswordHash: string(hashedPassword),
	}

	if err := a.repo.Create(ctx, user); err != nil {
		if errors.Is(err, apperr.ErrUserAlreadyExists) {
			return nil, apperr.ErrUserAlreadyExists
		}
		return nil, fmt.Errorf("register: create user: %w", err)
	}

	retUser := *user
	retUser.PasswordHash = ""
	return &retUser, nil
}

func (a *AuthUseCase) Login(ctx context.Context, username, password string) (*entity.User, string, error) {
	username = strings.TrimSpace(username)
	user, err := a.repo.GetByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, apperr.ErrUserNotFound) {
			_ = bcrypt.CompareHashAndPassword(dummyPasswordHash(), []byte(password))
			return nil, "", apperr.ErrInvalidCredentials
		}
		return nil, "", fmt.Errorf("login: get user: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, "", apperr.ErrInvalidCredentials
	}

	claims := jwt.RegisteredClaims{
		Subject:   strconv.FormatInt(user.ID, 10),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(a.jwtTTL)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	}

	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	token, err := t.SignedString([]byte(a.jwtSecret))
	if err != nil {
		return nil, "", fmt.Errorf("login: sign token: %w", err)
	}

	retUser := *user
	retUser.PasswordHash = ""
	return &retUser, token, nil
}
