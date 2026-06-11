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

// caller helpers
func userCaller(id string) application.CallerInfo {
	return application.CallerInfo{ID: id, Role: domain.RoleUser}
}
func adminCaller(id string) application.CallerInfo {
	return application.CallerInfo{ID: id, Role: domain.RoleAdmin}
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
	assert.Equal(t, domain.RoleUser, user.Role) // defaults to user
	assert.NotEmpty(t, user.ID)
	repo.AssertExpectations(t)
}

func TestRegister_WithAdminRole(t *testing.T) {
	repo := new(repoMock.UserRepository)
	svc  := newService(repo)

	repo.On("FindByEmail", mock.Anything, "admin@example.com").Return(nil, nil)
	repo.On("Create", mock.Anything, mock.AnythingOfType("*domain.User")).Return(nil)

	user, err := svc.Register(context.Background(), application.RegisterInput{
		Name:     "Admin",
		Email:    "admin@example.com",
		Password: "secret123",
		Role:     "admin",
	})

	assert.NoError(t, err)
	assert.Equal(t, domain.RoleAdmin, user.Role)
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
		Role:      domain.RoleUser,
		CreatedAt: time.Now(),
	}
	repo.On("FindByEmail", mock.Anything, "bob@example.com").Return(user, nil)

	result, err := svc.Login(context.Background(), application.LoginInput{
		Email:    "bob@example.com",
		Password: "secret123",
	})

	assert.NoError(t, err)
	assert.NotEmpty(t, result.Token)
	assert.Equal(t, domain.RoleUser, result.Role)
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

	err := svc.DeleteUser(context.Background(), userCaller("abc"), "abc")
	assert.NoError(t, err)
}

func TestDeleteUser_NotFound(t *testing.T) {
	repo := new(repoMock.UserRepository)
	svc  := newService(repo)

	repo.On("Delete", mock.Anything, "missing").Return(errors.New("user not found"))

	err := svc.DeleteUser(context.Background(), userCaller("missing"), "missing")
	assert.Error(t, err)
}

func TestDeleteUser_UserForbidden(t *testing.T) {
	repo := new(repoMock.UserRepository)
	svc  := newService(repo)

	// User trying to delete someone else → forbidden, no DB call.
	err := svc.DeleteUser(context.Background(), userCaller("u1"), "other")
	assert.ErrorIs(t, err, application.ErrForbidden)
	repo.AssertNotCalled(t, "Delete", mock.Anything, mock.Anything)
}

func TestDeleteUser_AdminCanDeleteUser(t *testing.T) {
	repo := new(repoMock.UserRepository)
	svc  := newService(repo)

	target := &domain.User{ID: "target", Role: domain.RoleUser}
	repo.On("FindByID", mock.Anything, "target").Return(target, nil)
	repo.On("Delete", mock.Anything, "target").Return(nil)

	err := svc.DeleteUser(context.Background(), adminCaller("admin"), "target")
	assert.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestDeleteUser_AdminCanDeleteSelf(t *testing.T) {
	repo := new(repoMock.UserRepository)
	svc  := newService(repo)

	// Admin deleting their own account — self-edit always permitted.
	repo.On("Delete", mock.Anything, "admin").Return(nil)

	err := svc.DeleteUser(context.Background(), adminCaller("admin"), "admin")
	assert.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestDeleteUser_AdminCannotDeleteAdmin(t *testing.T) {
	repo := new(repoMock.UserRepository)
	svc  := newService(repo)

	target := &domain.User{ID: "other-admin", Role: domain.RoleAdmin}
	repo.On("FindByID", mock.Anything, "other-admin").Return(target, nil)

	err := svc.DeleteUser(context.Background(), adminCaller("admin"), "other-admin")
	assert.ErrorIs(t, err, application.ErrForbidden)
	repo.AssertNotCalled(t, "Delete", mock.Anything, mock.Anything)
}

// ── ListUsers ─────────────────────────────────────────────────────────────────

func TestListUsers_Success(t *testing.T) {
	repo := new(repoMock.UserRepository)
	svc  := newService(repo)

	users := []*domain.User{
		{ID: "1", Name: "Alice", Email: "alice@example.com"},
		{ID: "2", Name: "Bob",   Email: "bob@example.com"},
	}
	repo.On("FindAll", mock.Anything, int64(20), int64(0)).Return(users, nil)

	result, err := svc.ListUsers(context.Background(), application.ListUsersInput{})
	assert.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, "Alice", result[0].Name)
	assert.Equal(t, "Bob",   result[1].Name)
	repo.AssertExpectations(t)
}

func TestListUsers_Empty(t *testing.T) {
	repo := new(repoMock.UserRepository)
	svc  := newService(repo)

	repo.On("FindAll", mock.Anything, int64(20), int64(0)).Return([]*domain.User{}, nil)

	result, err := svc.ListUsers(context.Background(), application.ListUsersInput{})
	assert.NoError(t, err)
	assert.Empty(t, result)
}

func TestListUsers_RepoError(t *testing.T) {
	repo := new(repoMock.UserRepository)
	svc  := newService(repo)

	repo.On("FindAll", mock.Anything, int64(20), int64(0)).Return([]*domain.User(nil), errors.New("db error"))

	_, err := svc.ListUsers(context.Background(), application.ListUsersInput{})
	assert.Error(t, err)
}

func TestListUsers_CustomLimit(t *testing.T) {
	repo := new(repoMock.UserRepository)
	svc  := newService(repo)

	repo.On("FindAll", mock.Anything, int64(5), int64(10)).Return([]*domain.User{}, nil)

	_, err := svc.ListUsers(context.Background(), application.ListUsersInput{Limit: 5, Offset: 10})
	assert.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestListUsers_LimitCappedAt100(t *testing.T) {
	repo := new(repoMock.UserRepository)
	svc  := newService(repo)

	// Limit 9999 should be clamped to 20 (default)
	repo.On("FindAll", mock.Anything, int64(20), int64(0)).Return([]*domain.User{}, nil)

	_, err := svc.ListUsers(context.Background(), application.ListUsersInput{Limit: 9999})
	assert.NoError(t, err)
	repo.AssertExpectations(t)
}

// ── UpdateUser ────────────────────────────────────────────────────────────────

func TestUpdateUser_NameOnly(t *testing.T) {
	repo := new(repoMock.UserRepository)
	svc  := newService(repo)

	newName := "Updated Name"
	updated := &domain.User{ID: "1", Name: newName, Email: "alice@example.com"}

	repo.On("Update", mock.Anything, "1", domain.UpdateFields{Name: &newName, Email: nil}).
		Return(updated, nil)

	result, err := svc.UpdateUser(context.Background(), userCaller("1"), "1", application.UpdateInput{Name: &newName})
	assert.NoError(t, err)
	assert.Equal(t, newName, result.Name)
	repo.AssertExpectations(t)
}

func TestUpdateUser_EmailOnly(t *testing.T) {
	repo := new(repoMock.UserRepository)
	svc  := newService(repo)

	newEmail := "new@example.com"
	updated := &domain.User{ID: "1", Name: "Alice", Email: newEmail}

	repo.On("Update", mock.Anything, "1", domain.UpdateFields{Name: nil, Email: &newEmail}).
		Return(updated, nil)

	result, err := svc.UpdateUser(context.Background(), userCaller("1"), "1", application.UpdateInput{Email: &newEmail})
	assert.NoError(t, err)
	assert.Equal(t, newEmail, result.Email)
	repo.AssertExpectations(t)
}

func TestUpdateUser_BothFields(t *testing.T) {
	repo := new(repoMock.UserRepository)
	svc  := newService(repo)

	newName  := "Carol"
	newEmail := "carol@example.com"
	updated  := &domain.User{ID: "2", Name: newName, Email: newEmail}

	repo.On("Update", mock.Anything, "2", domain.UpdateFields{Name: &newName, Email: &newEmail}).
		Return(updated, nil)

	result, err := svc.UpdateUser(context.Background(), userCaller("2"), "2", application.UpdateInput{
		Name: &newName, Email: &newEmail,
	})
	assert.NoError(t, err)
	assert.Equal(t, newName, result.Name)
	assert.Equal(t, newEmail, result.Email)
}

func TestUpdateUser_NotFound(t *testing.T) {
	repo := new(repoMock.UserRepository)
	svc  := newService(repo)

	repo.On("Update", mock.Anything, "999", mock.Anything).
		Return(nil, errors.New("user not found"))

	_, err := svc.UpdateUser(context.Background(), userCaller("999"), "999", application.UpdateInput{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestUpdateUser_UserForbidden(t *testing.T) {
	repo := new(repoMock.UserRepository)
	svc  := newService(repo)

	// User trying to update someone else → forbidden, no DB call.
	_, err := svc.UpdateUser(context.Background(), userCaller("u1"), "other", application.UpdateInput{})
	assert.ErrorIs(t, err, application.ErrForbidden)
	repo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything, mock.Anything)
}

func TestUpdateUser_AdminCanEditUser(t *testing.T) {
	repo := new(repoMock.UserRepository)
	svc  := newService(repo)

	newName := "New Name"
	target  := &domain.User{ID: "target", Role: domain.RoleUser}
	updated := &domain.User{ID: "target", Name: newName, Role: domain.RoleUser}
	repo.On("FindByID", mock.Anything, "target").Return(target, nil)
	repo.On("Update", mock.Anything, "target", mock.Anything).Return(updated, nil)

	result, err := svc.UpdateUser(context.Background(), adminCaller("admin"), "target", application.UpdateInput{Name: &newName})
	assert.NoError(t, err)
	assert.Equal(t, newName, result.Name)
	repo.AssertExpectations(t)
}

func TestUpdateUser_AdminCannotEditAdmin(t *testing.T) {
	repo := new(repoMock.UserRepository)
	svc  := newService(repo)

	target := &domain.User{ID: "other-admin", Role: domain.RoleAdmin}
	repo.On("FindByID", mock.Anything, "other-admin").Return(target, nil)

	_, err := svc.UpdateUser(context.Background(), adminCaller("admin"), "other-admin", application.UpdateInput{})
	assert.ErrorIs(t, err, application.ErrForbidden)
	repo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything, mock.Anything)
}

// ── CreateUser ────────────────────────────────────────────────────────────────

func TestCreateUser_Success(t *testing.T) {
	repo := new(repoMock.UserRepository)
	svc  := newService(repo)

	repo.On("FindByEmail", mock.Anything, "dave@example.com").Return(nil, nil)
	repo.On("Create", mock.Anything, mock.AnythingOfType("*domain.User")).Return(nil)

	user, err := svc.CreateUser(context.Background(), application.CreateInput{
		Name:     "Dave",
		Email:    "dave@example.com",
		Password: "pass1234",
	})

	assert.NoError(t, err)
	assert.Equal(t, "Dave", user.Name)
	assert.Equal(t, "dave@example.com", user.Email)
	assert.Equal(t, domain.RoleUser, user.Role) // CreateUser always assigns user role
	assert.NotEmpty(t, user.ID)
	assert.NotEmpty(t, user.Password) // stored as hash, not plaintext
	assert.NotEqual(t, "pass1234", user.Password)
	repo.AssertExpectations(t)
}

func TestCreateUser_DuplicateEmail(t *testing.T) {
	repo := new(repoMock.UserRepository)
	svc  := newService(repo)

	existing := &domain.User{ID: "x", Email: "dave@example.com"}
	repo.On("FindByEmail", mock.Anything, "dave@example.com").Return(existing, nil)

	_, err := svc.CreateUser(context.Background(), application.CreateInput{
		Name:     "Dave2",
		Email:    "dave@example.com",
		Password: "pass1234",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")
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

func TestCountUsers_RepoError(t *testing.T) {
	repo := new(repoMock.UserRepository)
	svc  := newService(repo)

	repo.On("Count", mock.Anything).Return(int64(0), errors.New("connection lost"))

	_, err := svc.CountUsers(context.Background())
	assert.Error(t, err)
}
