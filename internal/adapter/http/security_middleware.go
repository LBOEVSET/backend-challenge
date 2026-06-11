package http

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// ── CORS ──────────────────────────────────────────────────────────────────────

// CORS restricts cross-origin requests to the listed origins.
func CORS(allowedOrigins ...string) gin.HandlerFunc {
	allowed := make(map[string]bool, len(allowedOrigins))
	for _, o := range allowedOrigins {
		allowed[o] = true
	}
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if allowed[origin] {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
		}
		c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Authorization,Content-Type")
		c.Header("Access-Control-Max-Age", "86400")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// ── Security headers ──────────────────────────────────────────────────────────

// SecureHeaders adds defensive HTTP response headers to every response.
func SecureHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("Permissions-Policy", "geolocation=(), camera=(), microphone=()")
		c.Next()
	}
}

// ── Body size limit ───────────────────────────────────────────────────────────

// MaxBodySize rejects request bodies larger than the given byte count.
func MaxBodySize(bytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, bytes)
		c.Next()
	}
}

// ── Rate limiter ──────────────────────────────────────────────────────────────

type rateBucket struct {
	mu          sync.Mutex
	count       int
	windowStart time.Time
}

var (
	rateLimiters sync.Map
	cleanupOnce  sync.Once
)

func startRateLimitCleanup() {
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			cutoff := time.Now().Add(-5 * time.Minute)
			rateLimiters.Range(func(k, v any) bool {
				b := v.(*rateBucket)
				b.mu.Lock()
				stale := b.windowStart.Before(cutoff)
				b.mu.Unlock()
				if stale {
					rateLimiters.Delete(k)
				}
				return true
			})
		}
	}()
}

// RateLimit limits each IP to maxReq requests per window on the route it is
// applied to. Buckets are cleaned up every 5 minutes to prevent memory growth.
func RateLimit(maxReq int, window time.Duration) gin.HandlerFunc {
	cleanupOnce.Do(startRateLimitCleanup)
	return func(c *gin.Context) {
		ip := extractIP(c)
		key := c.FullPath() + "|" + ip

		val, _ := rateLimiters.LoadOrStore(key, &rateBucket{windowStart: time.Now()})
		b := val.(*rateBucket)

		b.mu.Lock()
		if time.Since(b.windowStart) > window {
			b.count = 0
			b.windowStart = time.Now()
		}
		b.count++
		over := b.count > maxReq
		b.mu.Unlock()

		if over {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "too many requests"})
			return
		}
		c.Next()
	}
}
