package usecase

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/gliedabrennung/messenger-core/internal/entity"
	"github.com/gliedabrennung/messenger-core/internal/repository"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// Common errors returned by the auth usecase.
var (
	ErrUserAlreadyExists  = errors.New("user already exists")
	ErrInvalidCredentials = errors.New("invalid credentials")
)

// UserRepository defines the persistence interface for user entities.
type UserRepository interface {
	Create(ctx context.Context, user *entity.User) error
	GetByUsername(ctx context.Context, username string) (*entity.User, error)
}

// AuthUseCase provides the business logic for user authentication and registration.
type AuthUseCase struct {
	repo      UserRepository
	jwtSecret string
	jwtTTL    time.Duration
}

// NewAuthUseCase creates and returns a new instance of AuthUseCase.
func NewAuthUseCase(repo UserRepository, jwtSecret string, jwtTTL time.Duration) *AuthUseCase {
	return &AuthUseCase{
		repo:      repo,
		jwtSecret: jwtSecret,
		jwtTTL:    jwtTTL,
	}
}

// Register creates a new user account with a securely hashed password.
func (a *AuthUseCase) Register(ctx context.Context, username, password string) (*entity.User, error) {
	// Check if the user already exists in the repository
	existing, err := a.repo.GetByUsername(ctx, username)
	if err != nil && !errors.Is(err, repository.ErrUserNotFound) {
		return nil, err
	}
	if existing != nil {
		return nil, ErrUserAlreadyExists
	}

	// Hash the password securely using bcrypt
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user := &entity.User{
		Username: username,
		Password: string(hashedPassword),
	}

	// Persist the new user to the repository
	if err := a.repo.Create(ctx, user); err != nil {
		if errors.Is(err, repository.ErrUserAlreadyExists) {
			return nil, ErrUserAlreadyExists
		}
		return nil, err
	}

	return user, nil
}

// Login authenticates a user and returns their entity along with a signed JWT token.
func (a *AuthUseCase) Login(ctx context.Context, username, password string) (*entity.User, string, error) {
	// Fetch the user from the repository
	user, err := a.repo.GetByUsername(ctx, username)
	if err != nil || user == nil {
		return nil, "", ErrInvalidCredentials
	}

	// Verify the provided password against the stored hash
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return nil, "", ErrInvalidCredentials
	}

	// Generate a JWT token with user claims
	claims := jwt.RegisteredClaims{
		Subject:   strconv.FormatInt(user.ID, 10),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(a.jwtTTL)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	}

	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	token, err := t.SignedString([]byte(a.jwtSecret))
	if err != nil {
		return nil, "", err
	}

	return user, token, nil
}
