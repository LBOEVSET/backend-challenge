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
	// Role is optional; if empty it defaults to "user".
	// Values: "admin" | "user"
	Role string `json:"role" validate:"omitempty,oneof=admin user"`
}

// Register creates a new user account and returns the created user.
func (s *UserService) Register(ctx context.Context, in RegisterInput) (*domain.User, error) {
	role := in.Role
	if role == "" {
		role = domain.RoleUser
	}
	return s.newUser(ctx, in.Name, in.Email, in.Password, role)
}

// LoginResult is the successful result of Login, carrying the JWT and the user's role.
type LoginResult struct {
	Token string
	Role  string
}

// LoginInput holds the credentials for authentication.
type LoginInput struct {
	Email    string `json:"email"    validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// Login verifies credentials and returns a signed JWT and the user's role on success.
func (s *UserService) Login(ctx context.Context, in LoginInput) (LoginResult, error) {
	user, err := s.repo.FindByEmail(ctx, in.Email)
	if err != nil || user == nil {
		return LoginResult{}, errors.New("invalid credentials")
	}
	if !hash.CheckPassword(user.Password, in.Password) {
		return LoginResult{}, errors.New("invalid credentials")
	}
	token, err := auth.GenerateToken(user.ID, user.Role, s.jwtSecret)
	if err != nil {
		return LoginResult{}, err
	}
	return LoginResult{Token: token, Role: user.Role}, nil
}

// ── CRUD ──────────────────────────────────────────────────────────────────────

// CallerInfo identifies the authenticated user making a mutation request.
type CallerInfo struct {
	ID   string
	Role string
}

// CreateInput is the input for creating a user via the protected API.
type CreateInput struct {
	Name     string `json:"name"     validate:"required,min=2"`
	Email    string `json:"email"    validate:"required,email"`
	Password string `json:"password" validate:"required,min=6"`
	// Role is optional; defaults to "user" if empty.
	Role string `json:"role" validate:"omitempty,oneof=admin user"`
}

// CreateUser creates a user (protected route — caller is already authenticated).
func (s *UserService) CreateUser(ctx context.Context, in CreateInput) (*domain.User, error) {
	role := in.Role
	if role == "" {
		role = domain.RoleUser
	}
	return s.newUser(ctx, in.Name, in.Email, in.Password, role)
}

// newUser is the shared implementation used by both Register and CreateUser.
// It checks for duplicate email, hashes the password, persists, and returns the user.
func (s *UserService) newUser(ctx context.Context, name, email, password, role string) (*domain.User, error) {
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
		Role:      role,
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

// authorizeUserMutation enforces RBAC for UpdateUser and DeleteUser:
//   - anyone: may always mutate their own account
//   - admin:  may mutate any user whose role is NOT admin
//   - user:   may only mutate themselves (covered by the self-edit rule above)
func (s *UserService) authorizeUserMutation(ctx context.Context, caller CallerInfo, targetID string) error {
	// Self-edit is always permitted regardless of role.
	if caller.ID == targetID {
		return nil
	}
	if caller.Role == domain.RoleAdmin {
		target, err := s.repo.FindByID(ctx, targetID)
		if err != nil || target == nil {
			return ErrNotFound
		}
		if target.Role == domain.RoleAdmin {
			return ErrForbidden
		}
		return nil
	}
	// "user" role trying to mutate someone else
	return ErrForbidden
}

// UpdateUser applies a partial update and returns the updated user.
func (s *UserService) UpdateUser(ctx context.Context, caller CallerInfo, id string, in UpdateInput) (*domain.User, error) {
	if err := s.authorizeUserMutation(ctx, caller, id); err != nil {
		return nil, err
	}
	return s.repo.Update(ctx, id, domain.UpdateFields{
		Name:  in.Name,
		Email: in.Email,
	})
}

// DeleteUser removes a user by ID.
func (s *UserService) DeleteUser(ctx context.Context, caller CallerInfo, id string) error {
	if err := s.authorizeUserMutation(ctx, caller, id); err != nil {
		return err
	}
	return s.repo.Delete(ctx, id)
}

// CountUsers returns the total number of users (used by the background goroutine).
func (s *UserService) CountUsers(ctx context.Context) (int64, error) {
	return s.repo.Count(ctx)
}
