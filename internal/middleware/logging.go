package middleware

import (
	"bytes"
	"io"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// RequestLogger provides structured logging with request tracing.
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		traceID := c.GetHeader("X-Trace-ID")
		if traceID == "" {
			traceID = uuid.New().String()
		}
		c.Set("trace_id", traceID)
		c.Header("X-Trace-ID", traceID)

		// Read and cache body for signature verification.
		if c.Request.Body != nil {
			body, _ := io.ReadAll(c.Request.Body)
			c.Set("rawBody", body)
			c.Request.Body = io.NopCloser(bytes.NewReader(body))
		}

		start := time.Now()
		c.Next()
		latency := time.Since(start)

		log.Info().
			Str("trace_id", traceID).
			Str("method", c.Request.Method).
			Str("path", c.Request.URL.Path).
			Int("status", c.Writer.Status()).
			Dur("latency", latency).
			Str("client_ip", c.ClientIP()).
			Msg("request")
	}
}

// GetAllowedOrigins returns the list of allowed CORS origins from environment.
// Defaults to localhost origins in development.
func GetAllowedOrigins() []string {
	origins := os.Getenv("ALLOWED_ORIGINS")
	if origins == "" {
		// Default to common development origins.
		return []string{
			"http://localhost:3000",
			"http://localhost:8080",
			"http://127.0.0.1:3000",
			"http://127.0.0.1:8080",
		}
	}
	return strings.Split(origins, ",")
}

// isOriginAllowed checks if the origin is in the allowed list.
func isOriginAllowed(origin string, allowedOrigins []string) bool {
	for _, allowed := range allowedOrigins {
		if strings.TrimSpace(allowed) == origin {
			return true
		}
	}
	return false
}

// CORS adds CORS headers with configurable allowed origins.
// Set ALLOWED_ORIGINS environment variable to comma-separated list of origins.
func CORS() gin.HandlerFunc {
	allowedOrigins := GetAllowedOrigins()

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")

		// Only set CORS headers if origin is allowed.
		if origin != "" && isOriginAllowed(origin, allowedOrigins) {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
		}

		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization, X-API-Key, X-Signature, X-Trace-ID, X-Idempotency-Key")
		c.Header("Access-Control-Expose-Headers", "X-Trace-ID")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}
