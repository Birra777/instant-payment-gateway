package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/ponti/instant-payment-gateway/internal/repository"
)

// hashAPIKey creates a SHA-256 hash of the API key for lookup.
func hashAPIKey(key string) string {
	hash := sha256.Sum256([]byte(key))
	return hex.EncodeToString(hash[:])
}

// APIKeyAuth validates requests using the X-API-Key header.
// API keys are hashed before lookup since we store hashed keys.
func APIKeyAuth(repo *repository.PostgresRepo) gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey := c.GetHeader("X-API-Key")
		if apiKey == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing API key"})
			return
		}

		// Hash the incoming API key to match against stored hash.
		hashedKey := hashAPIKey(apiKey)

		account, err := repo.GetAccountByAPIKey(c.Request.Context(), hashedKey)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid API key"})
			return
		}

		c.Set("account_id", account.ID.String())
		c.Set("account", account)
		c.Next()
	}
}

// VerifySignature validates HMAC-SHA256 request signatures.
func VerifySignature(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		signature := c.GetHeader("X-Signature")
		if signature == "" {
			c.Next() // Signature is optional for non-critical endpoints.
			return
		}

		body, exists := c.Get("rawBody")
		if !exists {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "unable to read request body"})
			return
		}

		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(body.([]byte))
		expected := hex.EncodeToString(mac.Sum(nil))

		if !hmac.Equal([]byte(signature), []byte(expected)) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid signature"})
			return
		}
		c.Next()
	}
}
