// Package application contains the use-case layer.
// It orchestrates domain entities and calls ports — no HTTP or database details here.
package application

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/lboevset/backend-challenge/internal/domain"
	"github.com/lboevset/backend-challenge/internal/port"
	"github.com/lboevset/backend-challenge/pkg/auth"
	"github.com/lboevset/backend-challenge/pkg/hash"
)

// UserService implements all user-related use cases.
type UserService struct {
	repo      port.UserRepository
	jwtSecret string
}

// NewUserService constructs a UserService with the given repository and JWT secret.
func NewUserService(repo port.UserRepository, jwtSecret string) *UserService {
	return &UserService{repo: repo, jwtSecret: jwtSecret}
}

// ── Auth ──────────────────────────────────────────────────────────────────────

// RegisterInput holds the data required to register a new user.
type RegisterInput struct {
	Name     string `json:"name"     validate:"required,min=2"`
	Email    string `json:"email"    validate:"required,email"`
	Password string `json:"password" validate:"required,min=6"`
}

// Register creates a new user account and returns the created user.
func (s *UserService) Register(ctx context.Context, in RegisterInput) (*domain.User, error) {
	return s.newUser(ctx, in.Name, in.Email, in.Password)
}

// LoginInput holds the credentials for authentication.
type LoginInput struct {
	Email    string `json:"email"    validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// Login verifies credentials and returns a signed JWT on success.
func (s *UserService) Login(ctx context.Context, in LoginInput) (string, error) {
	user, err := s.repo.FindByEmail(ctx, in.Email)
	if err != nil || user == nil {
		return "", errors.New("invalid credentials")
	}
	if !hash.CheckPassword(user.Password, in.Password) {
		return "", errors.New("invalid credentials")
	}
	return auth.GenerateToken(user.ID, s.jwtSecret)
}

// ── CRUD ──────────────────────────────────────────────────────────────────────

// CreateInput is the input for creating a user via the protected API.
type CreateInput struct {
	Name     string `json:"name"     validate:"required,min=2"`
	Email    string `json:"email"    validate:"required,email"`
	Password string `json:"password" validate:"required,min=6"`
}

// CreateUser creates a user (protected route — caller is already authenticated).
func (s *UserService) CreateUser(ctx context.Context, in CreateInput) (*domain.User, error) {
	return s.newUser(ctx, in.Name, in.Email, in.Password)
}

// newUser is the shared implementation used by both Register and CreateUser.
// It checks for duplicate email, hashes the password, persists, and returns the user.
func (s *UserService) newUser(ctx context.Context, name, email, password string) (*domain.User, error) {
	existing, _ := s.repo.FindByEmail(ctx, email)
	if existing != nil {
		return nil, errors.New("email already registered")
	}
	hashed, err := hash.Password(password)
	if err != nil {
		return nil, err
	}
	user := &domain.User{
		ID:        uuid.NewString(),
		Name:      name,
		Email:     email,
		Password:  hashed,
		CreatedAt: time.Now().UTC(),
	}
	if err := s.repo.Create(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

// GetUser fetches a single user by ID.
func (s *UserService) GetUser(ctx context.Context, id string) (*domain.User, error) {
	return s.repo.FindByID(ctx, id)
}

// ListUsersInput holds pagination parameters for listing users.
type ListUsersInput struct {
	Limit  int64
	Offset int64
}

// ListUsers returns a page of users. Limit defaults to 20; max is 100.
func (s *UserService) ListUsers(ctx context.Context, in ListUsersInput) ([]*domain.User, error) {
	if in.Limit <= 0 || in.Limit > 100 {
		in.Limit = 20
	}
	if in.Offset < 0 {
		in.Offset = 0
	}
	return s.repo.FindAll(ctx, in.Limit, in.Offset)
}

// UpdateInput holds the optional fields that can be changed.
type UpdateInput struct {
	Name  *string `json:"name"  validate:"omitempty,min=2"`
	Email *string `json:"email" validate:"omitempty,email"`
}

// UpdateUser applies a partial update and returns the updated user.
func (s *UserService) UpdateUser(ctx context.Context, id string, in UpdateInput) (*domain.User, error) {
	return s.repo.Update(ctx, id, domain.UpdateFields{
		Name:  in.Name,
		Email: in.Email,
	})
}

// DeleteUser removes a user by ID.
func (s *UserService) DeleteUser(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

// CountUsers returns the total number of users (used by the background goroutine).
func (s *UserService) CountUsers(ctx context.Context) (int64, error) {
	return s.repo.Count(ctx)
}
