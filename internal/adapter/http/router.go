package http

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lboevset/backend-challenge/internal/application"
	"github.com/lboevset/backend-challenge/internal/port"
)

// NewRouter wires up all routes and returns the configured Gin engine.
// allowedOrigin is the frontend base URL permitted for CORS (e.g. "https://assignment-web.lboevset.com").
func NewRouter(svc *application.UserService, jwtSecret string, visitorRepo port.VisitorRepository, allowedOrigin string) *gin.Engine {
	r := gin.New()
	r.Use(Logger())
	r.Use(gin.Recovery())
	r.Use(SecureHeaders())
	r.Use(CORS(allowedOrigin, "http://localhost:5173", "http://localhost:5174"))
	r.Use(MaxBodySize(1 * 1024 * 1024)) // 1 MB
	r.Use(VisitorTracker(visitorRepo))

	h := NewHandler(svc)

	v1 := r.Group("/api/v1")
	{
		v1.GET("/health", h.Health)

		// Public auth routes — rate limited: 10 requests / minute / IP
		auth := v1.Group("/auth")
		auth.Use(RateLimit(10, time.Minute))
		{
			auth.POST("/register", h.Register)
			auth.POST("/login", h.Login)
		}

		// Protected user routes
		users := v1.Group("/users")
		users.Use(Auth(jwtSecret))
		{
			users.POST("", h.CreateUser)
			users.GET("", h.ListUsers)
			users.GET("/:id", h.GetUser)
			users.PUT("/:id", h.UpdateUser)
			users.DELETE("/:id", h.DeleteUser)
		}
	}

	return r
}
