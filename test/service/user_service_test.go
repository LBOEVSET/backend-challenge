package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/lboevset/backend-challenge/internal/application"
	"github.com/lboevset/backend-challenge/internal/domain"
	"github.com/lboevset/backend-challenge/pkg/hash"
	repoMock "github.com/lboevset/backend-challenge/test/mock"
)

const testSecret = "test-jwt-secret"

func newService(repo *repoMock.UserRepository) *application.UserService {
	return application.NewUserService(repo, testSecret)
}

// ── Register ──────────────────────────────────────────────────────────────────

func TestRegister_Success(t *testing.T) {
	repo := new(repoMock.UserRepository)
	svc  := newService(repo)

	repo.On("FindByEmail", mock.Anything, "alice@example.com").Return(nil, nil)
	repo.On("Create", mock.Anything, mock.AnythingOfType("*domain.User")).Return(nil)

	user, err := svc.Register(context.Background(), application.RegisterInput{
		Name:     "Alice",
		Email:    "alice@example.com",
		Password: "secret123",
	})

	assert.NoError(t, err)
	assert.Equal(t, "Alice", user.Name)
	assert.Equal(t, "alice@example.com", user.Email)
	assert.NotEmpty(t, user.ID)
	repo.AssertExpectations(t)
}

func TestRegister_DuplicateEmail(t *testing.T) {
	repo := new(repoMock.UserRepository)
	svc  := newService(repo)

	existing := &domain.User{ID: "x", Email: "alice@example.com"}
	repo.On("FindByEmail", mock.Anything, "alice@example.com").Return(existing, nil)

	_, err := svc.Register(context.Background(), application.RegisterInput{
		Name:     "Alice",
		Email:    "alice@example.com",
		Password: "secret123",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")
	repo.AssertExpectations(t)
}

// ── Login ─────────────────────────────────────────────────────────────────────

func TestLogin_Success(t *testing.T) {
	repo := new(repoMock.UserRepository)
	svc  := newService(repo)

	// Hash the password properly so CheckPassword passes.
	hashed, err := hash.Password("secret123")
	if err != nil {
		t.Fatalf("hash.Password: %v", err)
	}

	user := &domain.User{
		ID:        "abc",
		Email:     "bob@example.com",
		Password:  hashed,
		CreatedAt: time.Now(),
	}
	repo.On("FindByEmail", mock.Anything, "bob@example.com").Return(user, nil)

	token, err := svc.Login(context.Background(), application.LoginInput{
		Email:    "bob@example.com",
		Password: "secret123",
	})

	assert.NoError(t, err)
	assert.NotEmpty(t, token)
	repo.AssertExpectations(t)
}

func TestLogin_InvalidPassword(t *testing.T) {
	repo := new(repoMock.UserRepository)
	svc  := newService(repo)

	user := &domain.User{
		ID:       "abc",
		Email:    "bob@example.com",
		Password: "$2a$10$invalid-hash-value",
	}
	repo.On("FindByEmail", mock.Anything, "bob@example.com").Return(user, nil)

	_, err := svc.Login(context.Background(), application.LoginInput{
		Email:    "bob@example.com",
		Password: "wrongpass",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid credentials")
}

func TestLogin_UserNotFound(t *testing.T) {
	repo := new(repoMock.UserRepository)
	svc  := newService(repo)

	repo.On("FindByEmail", mock.Anything, "ghost@example.com").Return(nil, nil)

	_, err := svc.Login(context.Background(), application.LoginInput{
		Email:    "ghost@example.com",
		Password: "any",
	})

	assert.Error(t, err)
}

// ── GetUser ───────────────────────────────────────────────────────────────────

func TestGetUser_Found(t *testing.T) {
	repo := new(repoMock.UserRepository)
	svc  := newService(repo)

	expected := &domain.User{ID: "123", Name: "Carol", Email: "carol@example.com"}
	repo.On("FindByID", mock.Anything, "123").Return(expected, nil)

	user, err := svc.GetUser(context.Background(), "123")
	assert.NoError(t, err)
	assert.Equal(t, "Carol", user.Name)
}

func TestGetUser_NotFound(t *testing.T) {
	repo := new(repoMock.UserRepository)
	svc  := newService(repo)

	repo.On("FindByID", mock.Anything, "999").Return(nil, nil)

	user, err := svc.GetUser(context.Background(), "999")
	assert.NoError(t, err)
	assert.Nil(t, user)
}

// ── DeleteUser ────────────────────────────────────────────────────────────────

func TestDeleteUser_Success(t *testing.T) {
	repo := new(repoMock.UserRepository)
	svc  := newService(repo)

	repo.On("Delete", mock.Anything, "abc").Return(nil)

	err := svc.DeleteUser(context.Background(), "abc")
	assert.NoError(t, err)
}

func TestDeleteUser_NotFound(t *testing.T) {
	repo := new(repoMock.UserRepository)
	svc  := newService(repo)

	repo.On("Delete", mock.Anything, "missing").Return(errors.New("user not found"))

	err := svc.DeleteUser(context.Background(), "missing")
	assert.Error(t, err)
}

// ── CountUsers ────────────────────────────────────────────────────────────────

func TestCountUsers(t *testing.T) {
	repo := new(repoMock.UserRepository)
	svc  := newService(repo)

	repo.On("Count", mock.Anything).Return(int64(42), nil)

	count, err := svc.CountUsers(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, int64(42), count)
}
