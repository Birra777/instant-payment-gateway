package middleware

import (
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// RateLimiter provides per-client rate limiting.
type RateLimiter struct {
	mu       sync.RWMutex
	clients  map[string]*rate.Limiter
	rps      rate.Limit
	burst    int
}

func NewRateLimiter(rps float64, burst int) *RateLimiter {
	return &RateLimiter{
		clients: make(map[string]*rate.Limiter),
		rps:     rate.Limit(rps),
		burst:   burst,
	}
}

func (rl *RateLimiter) getLimiter(key string) *rate.Limiter {
	rl.mu.RLock()
	limiter, exists := rl.clients[key]
	rl.mu.RUnlock()
	if exists {
		return limiter
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()
	// Double-check after acquiring write lock.
	if limiter, exists = rl.clients[key]; exists {
		return limiter
	}
	limiter = rate.NewLimiter(rl.rps, rl.burst)
	rl.clients[key] = limiter
	return limiter
}

// Middleware returns a Gin middleware that rate-limits by client IP.
func (rl *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.ClientIP()
		if apiKey := c.GetHeader("X-API-Key"); apiKey != "" {
			key = apiKey
		}

		if !rl.getLimiter(key).Allow() {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "rate limit exceeded",
			})
			return
		}
		c.Next()
	}
}
