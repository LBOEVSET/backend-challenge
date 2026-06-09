package http

import (
	"github.com/gin-gonic/gin"
	"github.com/lboevset/backend-challenge/internal/application"
)

// NewRouter wires up all routes and returns the configured Gin engine.
func NewRouter(svc *application.UserService, jwtSecret string) *gin.Engine {
	r := gin.New()
	r.Use(Logger())
	r.Use(gin.Recovery())

	h := NewHandler(svc)

	v1 := r.Group("/api/v1")
	{
		v1.GET("/health", h.Health)

		// Public auth routes
		auth := v1.Group("/auth")
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
