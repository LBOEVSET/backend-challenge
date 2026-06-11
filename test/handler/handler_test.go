// Package handler_test exercises the HTTP adapter layer end-to-end using
// httptest so that handler.go, middleware.go, and router.go are all covered.
package handler_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	httpAdapter "github.com/lboevset/backend-challenge/internal/adapter/http"
	"github.com/lboevset/backend-challenge/internal/application"
	"github.com/lboevset/backend-challenge/internal/domain"
	"github.com/lboevset/backend-challenge/pkg/auth"
	"github.com/lboevset/backend-challenge/pkg/hash"
	mockRepo "github.com/lboevset/backend-challenge/test/mock"
)

const testSecret = "test-jwt-secret"

func init() {
	gin.SetMode(gin.TestMode)
}

// newRouter creates a fresh Gin router backed by mock repos for each test.
// The visitor repository mock is wired in but calls are not asserted —
// visitor tracking is tested separately in the middleware test.
func newRouter() (*gin.Engine, *mockRepo.UserRepository) {
	repo := new(mockRepo.UserRepository)
	visitorRepo := new(mockRepo.VisitorRepository)
	visitorRepo.On("Upsert", mock.Anything, mock.Anything).Return(nil)
	svc := application.NewUserService(repo, testSecret)
	router := httpAdapter.NewRouter(svc, testSecret, visitorRepo, "http://localhost:5173")
	return router, repo
}

// authHeader generates a valid "Bearer <token>" header for a regular user.
func authHeader(userID string) string {
	token, _ := auth.GenerateToken(userID, domain.RoleUser, testSecret)
	return "Bearer " + token
}

// authHeaderAdmin generates a valid "Bearer <token>" header for an admin user.
func authHeaderAdmin(userID string) string {
	token, _ := auth.GenerateToken(userID, domain.RoleAdmin, testSecret)
	return "Bearer " + token
}

// jsonBody encodes v as JSON into a buffer for use as a request body.
func jsonBody(v any) *bytes.Buffer {
	b, _ := json.Marshal(v)
	return bytes.NewBuffer(b)
}

// ── Health ────────────────────────────────────────────────────────────────────

func TestHealth(t *testing.T) {
	router, _ := newRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/health", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

// ── Register ──────────────────────────────────────────────────────────────────

func TestRegister_Success(t *testing.T) {
	router, repo := newRouter()
	repo.On("FindByEmail", mock.Anything, "alice@example.com").Return(nil, errors.New("not found"))
	repo.On("Create", mock.Anything, mock.AnythingOfType("*domain.User")).Return(nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/register",
		jsonBody(map[string]string{"name": "Alice", "email": "alice@example.com", "password": "secret123"}))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestRegister_InvalidBody(t *testing.T) {
	router, _ := newRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/register",
		bytes.NewBufferString("not-json"))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRegister_ValidationFails(t *testing.T) {
	router, _ := newRouter()
	// Missing name and password — validator should reject with 422.
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/register",
		jsonBody(map[string]string{"email": "missing@example.com"}))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

func TestRegister_DuplicateEmail(t *testing.T) {
	router, repo := newRouter()
	existing := &domain.User{ID: "1", Email: "dup@example.com"}
	repo.On("FindByEmail", mock.Anything, "dup@example.com").Return(existing, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/register",
		jsonBody(map[string]string{"name": "Dup", "email": "dup@example.com", "password": "secret123"}))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusConflict, w.Code)
}

// ── Login ─────────────────────────────────────────────────────────────────────

func TestLogin_Success(t *testing.T) {
	router, repo := newRouter()
	hashed, _ := hash.Password("pass123")
	user := &domain.User{ID: "u1", Email: "alice@example.com", Password: hashed, Role: domain.RoleUser}
	repo.On("FindByEmail", mock.Anything, "alice@example.com").Return(user, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/login",
		jsonBody(map[string]string{"email": "alice@example.com", "password": "pass123"}))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp) //nolint:errcheck
	assert.NotEmpty(t, resp["token"])
	assert.Equal(t, domain.RoleUser, resp["role"])
}

func TestLogin_InvalidBody(t *testing.T) {
	router, _ := newRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/login",
		bytes.NewBufferString("bad"))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestLogin_WrongPassword(t *testing.T) {
	router, repo := newRouter()
	hashed, _ := hash.Password("correct")
	user := &domain.User{ID: "u1", Email: "alice@example.com", Password: hashed}
	repo.On("FindByEmail", mock.Anything, "alice@example.com").Return(user, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/login",
		jsonBody(map[string]string{"email": "alice@example.com", "password": "wrong"}))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestLogin_UserNotFound(t *testing.T) {
	router, repo := newRouter()
	repo.On("FindByEmail", mock.Anything, "ghost@example.com").Return(nil, errors.New("not found"))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/login",
		jsonBody(map[string]string{"email": "ghost@example.com", "password": "pass"}))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// ── Auth middleware ───────────────────────────────────────────────────────────

func TestAuth_MissingHeader(t *testing.T) {
	router, _ := newRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/users", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuth_NotBearerPrefix(t *testing.T) {
	router, _ := newRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/users", nil)
	req.Header.Set("Authorization", "Token something")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuth_InvalidToken(t *testing.T) {
	router, _ := newRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/users", nil)
	req.Header.Set("Authorization", "Bearer bad.token.here")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuth_WrongSecret(t *testing.T) {
	router, _ := newRouter()
	// Token signed with a different secret — should be rejected.
	token, _ := auth.GenerateToken("u1", domain.RoleUser, "wrong-secret")
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/users", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// ── ListUsers ────────────────────────────────────────────────────────────────

func TestListUsers_Success(t *testing.T) {
	router, repo := newRouter()
	users := []*domain.User{{ID: "1", Name: "Alice", Email: "a@b.com"}}
	repo.On("FindAll", mock.Anything, mock.AnythingOfType("int64"), mock.AnythingOfType("int64")).Return(users, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/users", nil)
	req.Header.Set("Authorization", authHeader("u1"))
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestListUsers_ServiceError(t *testing.T) {
	router, repo := newRouter()
	repo.On("FindAll", mock.Anything, mock.AnythingOfType("int64"), mock.AnythingOfType("int64")).Return([]*domain.User{}, errors.New("db error"))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/users", nil)
	req.Header.Set("Authorization", authHeader("u1"))
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// ── GetUser ──────────────────────────────────────────────────────────────────

func TestGetUser_Success(t *testing.T) {
	router, repo := newRouter()
	user := &domain.User{ID: "abc123", Name: "Bob"}
	repo.On("FindByID", mock.Anything, "abc123").Return(user, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/users/abc123", nil)
	req.Header.Set("Authorization", authHeader("u1"))
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetUser_NotFound(t *testing.T) {
	router, repo := newRouter()
	repo.On("FindByID", mock.Anything, "unknown").Return(nil, errors.New("not found"))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/users/unknown", nil)
	req.Header.Set("Authorization", authHeader("u1"))
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ── CreateUser ───────────────────────────────────────────────────────────────

func TestCreateUser_Success(t *testing.T) {
	router, repo := newRouter()
	repo.On("FindByEmail", mock.Anything, "new@example.com").Return(nil, errors.New("not found"))
	repo.On("Create", mock.Anything, mock.AnythingOfType("*domain.User")).Return(nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/users",
		jsonBody(map[string]string{"name": "New", "email": "new@example.com", "password": "pass123"}))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", authHeader("u1"))
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestCreateUser_DuplicateEmail(t *testing.T) {
	router, repo := newRouter()
	existing := &domain.User{ID: "1", Email: "dup@example.com"}
	repo.On("FindByEmail", mock.Anything, "dup@example.com").Return(existing, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/users",
		jsonBody(map[string]string{"name": "Dup", "email": "dup@example.com", "password": "pass123"}))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", authHeader("u1"))
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestCreateUser_InvalidBody(t *testing.T) {
	router, _ := newRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/users",
		bytes.NewBufferString("bad"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", authHeader("u1"))
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ── UpdateUser ───────────────────────────────────────────────────────────────

// User role: can update their own account.
func TestUpdateUser_Success(t *testing.T) {
	router, repo := newRouter()
	name := "Bob Updated"
	updated := &domain.User{ID: "u1", Name: name, Role: domain.RoleUser}
	repo.On("Update", mock.Anything, "u1", mock.Anything).Return(updated, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPut, "/api/v1/users/u1",
		jsonBody(map[string]string{"name": name}))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", authHeader("u1"))
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

// User role: cannot update another user → 403.
func TestUpdateUser_Forbidden(t *testing.T) {
	router, _ := newRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPut, "/api/v1/users/other-user",
		jsonBody(map[string]string{"name": "Hacker"}))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", authHeader("u1")) // u1 (user) trying to modify other-user
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

// User role: own account not found → 404.
func TestUpdateUser_NotFound(t *testing.T) {
	router, repo := newRouter()
	repo.On("Update", mock.Anything, "notexist", mock.Anything).Return(nil, errors.New("not found"))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPut, "/api/v1/users/notexist",
		jsonBody(map[string]string{"name": "XY"}))
	req.Header.Set("Content-Type", "application/json")
	// Token user must match the param so the user-role ownership check passes.
	req.Header.Set("Authorization", authHeader("notexist"))
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// Admin role: can update their own account (self-edit always allowed).
func TestUpdateUser_AdminCanEditSelf(t *testing.T) {
	router, repo := newRouter()
	name := "Self Updated"
	updated := &domain.User{ID: "admin-u1", Name: name, Role: domain.RoleAdmin}
	// No FindByID call — self-edit short-circuits the role check.
	repo.On("Update", mock.Anything, "admin-u1", mock.Anything).Return(updated, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPut, "/api/v1/users/admin-u1",
		jsonBody(map[string]string{"name": name}))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", authHeaderAdmin("admin-u1"))
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

// Admin role: can update a user-role account.
func TestUpdateUser_AdminCanEditUser(t *testing.T) {
	router, repo := newRouter()
	name := "Updated by Admin"
	target := &domain.User{ID: "target-user", Name: "Target", Role: domain.RoleUser}
	updated := &domain.User{ID: "target-user", Name: name, Role: domain.RoleUser}
	repo.On("FindByID", mock.Anything, "target-user").Return(target, nil)
	repo.On("Update", mock.Anything, "target-user", mock.Anything).Return(updated, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPut, "/api/v1/users/target-user",
		jsonBody(map[string]string{"name": name}))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", authHeaderAdmin("admin-u1"))
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

// Admin role: cannot update another admin → 403.
func TestUpdateUser_AdminCannotEditAdmin(t *testing.T) {
	router, repo := newRouter()
	target := &domain.User{ID: "other-admin", Name: "Other Admin", Role: domain.RoleAdmin}
	repo.On("FindByID", mock.Anything, "other-admin").Return(target, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPut, "/api/v1/users/other-admin",
		jsonBody(map[string]string{"name": "Hacked"}))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", authHeaderAdmin("admin-u1"))
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestUpdateUser_InvalidBody(t *testing.T) {
	router, _ := newRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPut, "/api/v1/users/u1",
		bytes.NewBufferString("bad"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", authHeader("u1"))
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ── DeleteUser ───────────────────────────────────────────────────────────────

// User role: can delete their own account.
func TestDeleteUser_Success(t *testing.T) {
	router, repo := newRouter()
	repo.On("Delete", mock.Anything, "u1").Return(nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodDelete, "/api/v1/users/u1", nil)
	req.Header.Set("Authorization", authHeader("u1"))
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

// User role: cannot delete another user → 403.
func TestDeleteUser_Forbidden(t *testing.T) {
	router, _ := newRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodDelete, "/api/v1/users/other-user", nil)
	req.Header.Set("Authorization", authHeader("u1")) // u1 trying to delete other-user
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

// User role: own account not found → 404.
func TestDeleteUser_NotFound(t *testing.T) {
	router, repo := newRouter()
	repo.On("Delete", mock.Anything, "notexist").Return(errors.New("not found"))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodDelete, "/api/v1/users/notexist", nil)
	// Token user must match the param so the user-role ownership check passes.
	req.Header.Set("Authorization", authHeader("notexist"))
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// Admin role: can delete a user-role account.
func TestDeleteUser_AdminCanDeleteUser(t *testing.T) {
	router, repo := newRouter()
	target := &domain.User{ID: "target-user", Role: domain.RoleUser}
	repo.On("FindByID", mock.Anything, "target-user").Return(target, nil)
	repo.On("Delete", mock.Anything, "target-user").Return(nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodDelete, "/api/v1/users/target-user", nil)
	req.Header.Set("Authorization", authHeaderAdmin("admin-u1"))
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

// Admin role: cannot delete another admin → 403.
func TestDeleteUser_AdminCannotDeleteAdmin(t *testing.T) {
	router, repo := newRouter()
	target := &domain.User{ID: "other-admin", Role: domain.RoleAdmin}
	repo.On("FindByID", mock.Anything, "other-admin").Return(target, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodDelete, "/api/v1/users/other-admin", nil)
	req.Header.Set("Authorization", authHeaderAdmin("admin-u1"))
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)
}
