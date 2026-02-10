package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestRateLimiter_AllowsWithinLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rl := NewRateLimiter(10, 10) // 10 rps, burst 10.

	router := gin.New()
	router.Use(rl.Middleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	for i := 0; i < 10; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.1:1234"
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	}
}

func TestRateLimiter_BlocksOverLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rl := NewRateLimiter(1, 1) // 1 rps, burst 1.

	router := gin.New()
	router.Use(rl.Middleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	// First request should pass.
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:1234"
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Second immediate request should be rate-limited.
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:1234"
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
}

func TestRateLimiter_DifferentClients(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rl := NewRateLimiter(1, 1) // 1 rps, burst 1.

	router := gin.New()
	router.Use(rl.Middleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	// First client.
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:1234"
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Different client should not be rate-limited.
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.2:1234"
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRateLimiter_APIKeyBasedLimiting(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rl := NewRateLimiter(1, 1)

	router := gin.New()
	router.Use(rl.Middleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	// First request with API key.
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("X-API-Key", "test-key-1")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Second request with same API key should be limited.
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/test", nil)
	req.Header.Set("X-API-Key", "test-key-1")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)

	// Different API key should pass.
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/test", nil)
	req.Header.Set("X-API-Key", "test-key-2")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}
