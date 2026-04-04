package middlewares

import (
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// visitor records the rate limiter and last seen time for an IP
type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// RateLimiter struct holds the visitors map, mutex, and rate params
type RateLimiter struct {
	visitors map[string]*visitor
	mu       sync.Mutex
	rate     rate.Limit
	burst    int
}

// NewRateLimiter creates a new simple IP rate limiter
func NewRateLimiter(r rate.Limit, b int) *RateLimiter {
	rl := &RateLimiter{
		visitors: make(map[string]*visitor),
		rate:     r,
		burst:    b,
	}

	// Clean up old visitors every minute to prevent memory leaks
	go rl.cleanupVisitors()

	return rl
}

// getLimiter returns the rate limiter for a specific IP
func (rl *RateLimiter) getLimiter(ip string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	v, exists := rl.visitors[ip]
	if !exists {
		limiter := rate.NewLimiter(rl.rate, rl.burst)
		rl.visitors[ip] = &visitor{limiter, time.Now()}
		return limiter
	}

	v.lastSeen = time.Now()
	return v.limiter
}

// cleanupVisitors removes IPs that haven't been seen in 3 minutes
func (rl *RateLimiter) cleanupVisitors() {
	for {
		time.Sleep(time.Minute)
		rl.mu.Lock()
		for ip, v := range rl.visitors {
			if time.Since(v.lastSeen) > 3*time.Minute {
				delete(rl.visitors, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// IPRateLimitMiddleware is the Gin middleware function
func (rl *RateLimiter) IPRateLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get client IP
		ip, _, err := net.SplitHostPort(c.Request.RemoteAddr)
		if err != nil {
			// If we can't get IP, use fallback (X-Forwarded-For is usually handled by Gin ClientIP)
			ip = c.ClientIP()
		}

		limiter := rl.getLimiter(ip)
		if !limiter.Allow() {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "Terlalu banyak permintaan (Too Many Requests). Silakan coba beberapa saat lagi.",
			})
			return
		}

		c.Next()
	}
}
