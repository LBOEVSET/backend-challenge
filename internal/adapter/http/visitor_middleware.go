package http

import (
	"context"
	"log"
	"net"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lboevset/backend-challenge/internal/domain"
	"github.com/lboevset/backend-challenge/internal/port"
)

// VisitorTracker returns a middleware that records each unique visitor
// (identified by IP + User-Agent) to MongoDB asynchronously.
// The goroutine runs after the response is sent — zero latency impact.
func VisitorTracker(repo port.VisitorRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next() // handle the request first

		// Capture values before the goroutine runs (gin.Context is reused)
		ip := extractIP(c)
		ua := c.Request.UserAgent()

		// Skip internal health-check bots — they're not real visitors.
		if isInternalBot(ua) {
			return
		}

		now := time.Now().UTC()

		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[visitor] panic: %v", r)
				}
			}()

			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()

			log.Printf("[visitor] tracking ip=%s ua=%s", ip, ua)

			record := &domain.VisitorRecord{
				IP:         ip,
				UserAgent:  ua,
				Device:     parseDevice(ua),
				OS:         parseOS(ua),
				Browser:    parseBrowser(ua),
				FirstVisit: now,
				LastVisit:  now,
			}

			if err := repo.Upsert(ctx, record); err != nil {
				log.Printf("[visitor] upsert error: %v", err)
			}
		}()
	}
}

// isInternalBot returns true for GKE/Kubernetes automated health-check agents.
func isInternalBot(ua string) bool {
	return strings.HasPrefix(ua, "GoogleHC/") || strings.HasPrefix(ua, "kube-probe/")
}

// extractIP returns the real client IP, respecting common proxy headers.
func extractIP(c *gin.Context) string {
	// X-Forwarded-For may contain a chain: "client, proxy1, proxy2"
	if xff := c.GetHeader("X-Forwarded-For"); xff != "" {
		if ip := strings.TrimSpace(strings.SplitN(xff, ",", 2)[0]); ip != "" {
			return ip
		}
	}
	if xri := c.GetHeader("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}
	// Fallback: strip port from RemoteAddr
	host, _, err := net.SplitHostPort(c.Request.RemoteAddr)
	if err != nil {
		return c.Request.RemoteAddr
	}
	return host
}

// parseDevice returns "mobile", "tablet", or "desktop".
func parseDevice(ua string) string {
	ua = strings.ToLower(ua)
	if strings.Contains(ua, "ipad") || strings.Contains(ua, "tablet") {
		return "tablet"
	}
	if strings.Contains(ua, "mobile") || strings.Contains(ua, "iphone") ||
		strings.Contains(ua, "android") && strings.Contains(ua, "mobile") {
		return "mobile"
	}
	return "desktop"
}

// parseOS returns the operating system family.
func parseOS(ua string) string {
	switch {
	case strings.Contains(ua, "Windows"):
		return "Windows"
	case strings.Contains(ua, "iPhone"), strings.Contains(ua, "iPad"):
		return "iOS"
	case strings.Contains(ua, "Android"):
		return "Android"
	case strings.Contains(ua, "Mac OS X"), strings.Contains(ua, "Macintosh"):
		return "macOS"
	case strings.Contains(ua, "Linux"):
		return "Linux"
	default:
		return "Other"
	}
}

// parseBrowser returns the browser name.
// Order matters — check Edge/OPR before Chrome, Chrome before Safari.
func parseBrowser(ua string) string {
	switch {
	case strings.Contains(ua, "Edg/"), strings.Contains(ua, "Edge"):
		return "Edge"
	case strings.Contains(ua, "OPR/"), strings.Contains(ua, "Opera"):
		return "Opera"
	case strings.Contains(ua, "Chrome"):
		return "Chrome"
	case strings.Contains(ua, "Firefox"):
		return "Firefox"
	case strings.Contains(ua, "Safari"):
		return "Safari"
	default:
		return "Other"
	}
}
