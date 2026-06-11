package handler_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	httpAdapter "github.com/lboevset/backend-challenge/internal/adapter/http"
	"github.com/lboevset/backend-challenge/internal/application"
	mockRepo "github.com/lboevset/backend-challenge/test/mock"
)

// newRouterWithVisitor returns a router wired to the supplied visitor mock.
func newRouterWithVisitor(vr *mockRepo.VisitorRepository) *gin.Engine {
	repo := new(mockRepo.UserRepository)
	svc := application.NewUserService(repo, testSecret)
	return httpAdapter.NewRouter(svc, testSecret, vr, "http://localhost:5173")
}

// ── VisitorTracker middleware ─────────────────────────────────────────────────

func TestVisitorMiddleware_UpsertCalledOnRequest(t *testing.T) {
	vr := new(mockRepo.VisitorRepository)
	vr.On("Upsert", mock.Anything, mock.Anything).Return(nil)

	router := newRouterWithVisitor(vr)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/health", nil)
	req.Header.Set("User-Agent", "TestBrowser/1.0")
	router.ServeHTTP(w, req)

	// Give the goroutine time to complete.
	time.Sleep(50 * time.Millisecond)

	vr.AssertCalled(t, "Upsert", mock.Anything, mock.Anything)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestVisitorMiddleware_UpsertError_DoesNotAffectResponse(t *testing.T) {
	vr := new(mockRepo.VisitorRepository)
	vr.On("Upsert", mock.Anything, mock.Anything).Return(assert.AnError)

	router := newRouterWithVisitor(vr)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/health", nil)
	router.ServeHTTP(w, req)

	time.Sleep(50 * time.Millisecond)

	// Even when upsert fails, the HTTP response must be 200.
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestVisitorMiddleware_XForwardedFor(t *testing.T) {
	vr := new(mockRepo.VisitorRepository)
	vr.On("Upsert", mock.Anything, mock.Anything).Return(nil)

	router := newRouterWithVisitor(vr)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/health", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.5, 10.0.0.1")
	router.ServeHTTP(w, req)

	time.Sleep(50 * time.Millisecond)
	vr.AssertCalled(t, "Upsert", mock.Anything, mock.Anything)
}

func TestVisitorMiddleware_XRealIP(t *testing.T) {
	vr := new(mockRepo.VisitorRepository)
	vr.On("Upsert", mock.Anything, mock.Anything).Return(nil)

	router := newRouterWithVisitor(vr)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/health", nil)
	req.Header.Set("X-Real-IP", "198.51.100.7")
	router.ServeHTTP(w, req)

	time.Sleep(50 * time.Millisecond)
	vr.AssertCalled(t, "Upsert", mock.Anything, mock.Anything)
}

// ── Bot filtering ────────────────────────────────────────────────────────────

func TestVisitorMiddleware_GoogleHC_Skipped(t *testing.T) {
	vr := new(mockRepo.VisitorRepository)
	// Upsert must NOT be called for health-check bots.

	router := newRouterWithVisitor(vr)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/health", nil)
	req.Header.Set("User-Agent", "GoogleHC/1.0")
	router.ServeHTTP(w, req)

	time.Sleep(50 * time.Millisecond)
	vr.AssertNotCalled(t, "Upsert", mock.Anything, mock.Anything)
}

func TestVisitorMiddleware_KubeProbe_Skipped(t *testing.T) {
	vr := new(mockRepo.VisitorRepository)

	router := newRouterWithVisitor(vr)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/health", nil)
	req.Header.Set("User-Agent", "kube-probe/1.35")
	router.ServeHTTP(w, req)

	time.Sleep(50 * time.Millisecond)
	vr.AssertNotCalled(t, "Upsert", mock.Anything, mock.Anything)
}

// ── Device / OS / Browser detection (via exported helpers or end-to-end) ──────
// These exercise the parse* helpers indirectly through the middleware goroutine.

var uaTable = []struct {
	name string
	ua   string
}{
	{"chrome-windows", "Mozilla/5.0 (Windows NT 10.0) AppleWebKit/537.36 Chrome/120.0 Safari/537.36"},
	{"firefox-linux", "Mozilla/5.0 (X11; Linux x86_64; rv:109.0) Gecko/20100101 Firefox/118.0"},
	{"safari-macos", "Mozilla/5.0 (Macintosh; Intel Mac OS X 13_0) AppleWebKit/605.1 Version/16 Safari/605.1"},
	{"edge-windows", "Mozilla/5.0 (Windows NT 10.0) AppleWebKit/537.36 Edg/120.0 Safari/537.36"},
	{"opera", "Mozilla/5.0 (Windows NT 10.0) AppleWebKit/537.36 OPR/105.0 Safari/537.36"},
	{"android-mobile", "Mozilla/5.0 (Linux; Android 13; Pixel 7) Mobile Safari/537.36"},
	{"iphone", "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) Mobile/15E148 Safari/604.1"},
	{"ipad", "Mozilla/5.0 (iPad; CPU OS 17_0 like Mac OS X) AppleWebKit/605.1"},
	{"unknown-ua", "CustomClient/1.0"},
}

func TestVisitorMiddleware_VariousUserAgents(t *testing.T) {
	for _, tt := range uaTable {
		t.Run(tt.name, func(t *testing.T) {
			vr := new(mockRepo.VisitorRepository)
			vr.On("Upsert", mock.Anything, mock.Anything).Return(nil)

			router := newRouterWithVisitor(vr)
			w := httptest.NewRecorder()
			req, _ := http.NewRequest(http.MethodGet, "/api/v1/health", nil)
			req.Header.Set("User-Agent", tt.ua)
			router.ServeHTTP(w, req)

			time.Sleep(50 * time.Millisecond)
			vr.AssertCalled(t, "Upsert", mock.Anything, mock.Anything)
			assert.Equal(t, http.StatusOK, w.Code)
		})
	}
}
