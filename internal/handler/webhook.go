package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

// WebhookHandler handles incoming webhook notifications from payment providers.
type WebhookHandler struct {
	stripeSecret string
	momoSecret   string
}

func NewWebhookHandler(stripeSecret, momoSecret string) *WebhookHandler {
	return &WebhookHandler{
		stripeSecret: stripeSecret,
		momoSecret:   momoSecret,
	}
}

// verifyHMACSignature validates an HMAC-SHA256 signature.
func verifyHMACSignature(payload []byte, signature, secret string) bool {
	if secret == "" {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(signature), []byte(expected))
}

// HandleStripeWebhook handles POST /api/v1/webhooks/stripe
func (h *WebhookHandler) HandleStripeWebhook(c *gin.Context) {
	// Read raw body for signature verification.
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
		return
	}

	// Verify webhook signature.
	signature := c.GetHeader("X-Stripe-Signature")
	if signature == "" {
		log.Warn().Str("remote", c.ClientIP()).Msg("stripe webhook missing signature")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing signature"})
		return
	}

	if !verifyHMACSignature(body, signature, h.stripeSecret) {
		log.Warn().Str("remote", c.ClientIP()).Msg("stripe webhook invalid signature")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid signature"})
		return
	}

	// Parse the verified payload.
	var payload map[string]interface{}
	if err := c.ShouldBindJSON(&payload); err != nil {
		// Re-bind from the body we already read - need to restore it first.
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	// Log only safe fields - never log full payload which may contain PII.
	eventType, _ := payload["type"].(string)
	eventID, _ := payload["id"].(string)
	log.Info().
		Str("event_type", eventType).
		Str("event_id", eventID).
		Msg("received verified stripe webhook")

	// In production, process the event (update transaction status, etc.).
	c.JSON(http.StatusOK, gin.H{"received": true})
}

// HandleMoMoWebhook handles POST /api/v1/webhooks/momo
func (h *WebhookHandler) HandleMoMoWebhook(c *gin.Context) {
	// Read raw body for signature verification.
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
		return
	}

	// Verify webhook signature.
	signature := c.GetHeader("X-MoMo-Signature")
	if signature == "" {
		log.Warn().Str("remote", c.ClientIP()).Msg("momo webhook missing signature")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing signature"})
		return
	}

	if !verifyHMACSignature(body, signature, h.momoSecret) {
		log.Warn().Str("remote", c.ClientIP()).Msg("momo webhook invalid signature")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid signature"})
		return
	}

	// Parse the verified payload.
	var payload map[string]interface{}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	// Log only safe fields - never log full payload which may contain PII.
	transactionID, _ := payload["transactionId"].(string)
	status, _ := payload["status"].(string)
	log.Info().
		Str("transaction_id", transactionID).
		Str("status", status).
		Msg("received verified momo webhook")

	c.JSON(http.StatusOK, gin.H{"received": true})
}
