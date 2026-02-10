package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/ponti/instant-payment-gateway/internal/model"
	"github.com/ponti/instant-payment-gateway/internal/service"
	ws "github.com/ponti/instant-payment-gateway/internal/websocket"
)

// PaymentHandler handles payment HTTP endpoints.
type PaymentHandler struct {
	svc *service.PaymentService
	hub *ws.Hub
}

func NewPaymentHandler(svc *service.PaymentService, hub *ws.Hub) *PaymentHandler {
	return &PaymentHandler{svc: svc, hub: hub}
}

// InitiatePayment handles POST /api/v1/payments
func (h *PaymentHandler) InitiatePayment(c *gin.Context) {
	var req model.InitiatePaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	txn, err := h.svc.InitiatePayment(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	h.hub.Broadcast("payment.initiated", txn)
	c.JSON(http.StatusCreated, txn)
}

// AuthorizePayment handles POST /api/v1/payments/:id/authorize
func (h *PaymentHandler) AuthorizePayment(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid transaction ID"})
		return
	}

	// Get the authenticated account ID from context (set by auth middleware).
	callerAccountID, _ := c.Get("account_id")
	callerID := ""
	if callerAccountID != nil {
		callerID = callerAccountID.(string)
	}

	txn, err := h.svc.AuthorizePayment(c.Request.Context(), id, callerID)
	if err != nil {
		if err.Error() == "unauthorized: only the sender can authorize this transaction" {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	h.hub.Broadcast("payment.authorized", txn)
	c.JSON(http.StatusOK, txn)
}

// SettlePayment handles POST /api/v1/payments/:id/settle
func (h *PaymentHandler) SettlePayment(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid transaction ID"})
		return
	}

	// Get the authenticated account ID from context (set by auth middleware).
	callerAccountID, _ := c.Get("account_id")
	callerID := ""
	if callerAccountID != nil {
		callerID = callerAccountID.(string)
	}

	txn, err := h.svc.SettlePayment(c.Request.Context(), id, callerID)
	if err != nil {
		if err.Error() == "unauthorized: only the sender or receiver can settle this transaction" {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	h.hub.Broadcast("payment.settled", txn)
	c.JSON(http.StatusOK, txn)
}

// GetTransaction handles GET /api/v1/payments/:id
func (h *PaymentHandler) GetTransaction(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid transaction ID"})
		return
	}

	txn, err := h.svc.GetTransaction(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "transaction not found"})
		return
	}

	c.JSON(http.StatusOK, txn)
}

// ListTransactions handles GET /api/v1/payments
func (h *PaymentHandler) ListTransactions(c *gin.Context) {
	var filter model.TransactionFilter
	if err := c.ShouldBindQuery(&filter); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	txns, err := h.svc.ListTransactions(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list transactions"})
		return
	}

	c.JSON(http.StatusOK, txns)
}

// GetLedgerEntries handles GET /api/v1/payments/:id/ledger
func (h *PaymentHandler) GetLedgerEntries(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid transaction ID"})
		return
	}

	entries, err := h.svc.GetLedgerEntries(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get ledger entries"})
		return
	}

	c.JSON(http.StatusOK, entries)
}

// GetAnalytics handles GET /api/v1/analytics
func (h *PaymentHandler) GetAnalytics(c *gin.Context) {
	date := c.DefaultQuery("date", time.Now().UTC().Format("2006-01-02"))

	analytics, err := h.svc.GetDailyAnalytics(c.Request.Context(), date)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get analytics"})
		return
	}

	c.JSON(http.StatusOK, analytics)
}
