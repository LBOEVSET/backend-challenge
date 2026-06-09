package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/lboevset/backend-challenge/internal/application"
)

var validate = validator.New()

// Handler holds the application service used by all HTTP handlers.
type Handler struct {
	svc *application.UserService
}

// NewHandler constructs a Handler.
func NewHandler(svc *application.UserService) *Handler {
	return &Handler{svc: svc}
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func bindAndValidate(c *gin.Context, dst any) bool {
	if err := c.ShouldBindJSON(dst); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return false
	}
	if err := validate.Struct(dst); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return false
	}
	return true
}

func respondErr(c *gin.Context, status int, err error) {
	c.JSON(status, gin.H{"error": err.Error()})
}

// ── Auth ──────────────────────────────────────────────────────────────────────

// Register godoc
// POST /api/v1/auth/register
func (h *Handler) Register(c *gin.Context) {
	var in application.RegisterInput
	if !bindAndValidate(c, &in) {
		return
	}
	user, err := h.svc.Register(c.Request.Context(), in)
	if err != nil {
		respondErr(c, http.StatusConflict, err)
		return
	}
	c.JSON(http.StatusCreated, user)
}

// Login godoc
// POST /api/v1/auth/login
func (h *Handler) Login(c *gin.Context) {
	var in application.LoginInput
	if !bindAndValidate(c, &in) {
		return
	}
	token, err := h.svc.Login(c.Request.Context(), in)
	if err != nil {
		respondErr(c, http.StatusUnauthorized, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": token})
}

// ── User CRUD ─────────────────────────────────────────────────────────────────

// CreateUser godoc
// POST /api/v1/users   (protected)
func (h *Handler) CreateUser(c *gin.Context) {
	var in application.CreateInput
	if !bindAndValidate(c, &in) {
		return
	}
	user, err := h.svc.CreateUser(c.Request.Context(), in)
	if err != nil {
		respondErr(c, http.StatusConflict, err)
		return
	}
	c.JSON(http.StatusCreated, user)
}

// GetUser godoc
// GET /api/v1/users/:id   (protected)
func (h *Handler) GetUser(c *gin.Context) {
	user, err := h.svc.GetUser(c.Request.Context(), c.Param("id"))
	if err != nil || user == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	c.JSON(http.StatusOK, user)
}

// ListUsers godoc
// GET /api/v1/users   (protected)
func (h *Handler) ListUsers(c *gin.Context) {
	users, err := h.svc.ListUsers(c.Request.Context())
	if err != nil {
		respondErr(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, users)
}

// UpdateUser godoc
// PUT /api/v1/users/:id   (protected)
func (h *Handler) UpdateUser(c *gin.Context) {
	var in application.UpdateInput
	if !bindAndValidate(c, &in) {
		return
	}
	user, err := h.svc.UpdateUser(c.Request.Context(), c.Param("id"), in)
	if err != nil {
		respondErr(c, http.StatusNotFound, err)
		return
	}
	c.JSON(http.StatusOK, user)
}

// DeleteUser godoc
// DELETE /api/v1/users/:id   (protected)
func (h *Handler) DeleteUser(c *gin.Context) {
	if err := h.svc.DeleteUser(c.Request.Context(), c.Param("id")); err != nil {
		respondErr(c, http.StatusNotFound, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "user deleted"})
}

// Health godoc
// GET /api/v1/health
func (h *Handler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
