package handler

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/ponti/instant-payment-gateway/internal/model"
	"github.com/ponti/instant-payment-gateway/internal/repository"
)

// AccountHandler handles account HTTP endpoints.
type AccountHandler struct {
	repo *repository.PostgresRepo
}

func NewAccountHandler(repo *repository.PostgresRepo) *AccountHandler {
	return &AccountHandler{repo: repo}
}

// CreateAccount handles POST /api/v1/accounts
func (h *AccountHandler) CreateAccount(c *gin.Context) {
	var req struct {
		Name     string `json:"name" binding:"required"`
		Email    string `json:"email" binding:"required,email"`
		Currency string `json:"currency" binding:"required,len=3"`
		Type     string `json:"type" binding:"required,oneof=user merchant"`
		Balance  int64  `json:"balance"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	rawAPIKey, err := generateAPIKey()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate API key"})
		return
	}

	// Hash the API key before storing - the raw key is only shown once.
	hashedKey := hashAPIKey(rawAPIKey)

	account := &model.Account{
		Name:     req.Name,
		Email:    req.Email,
		Balance:  req.Balance,
		Currency: req.Currency,
		Type:     req.Type,
		APIKey:   hashedKey,
	}

	if err := h.repo.CreateAccount(c.Request.Context(), account); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create account"})
		return
	}

	// Return the raw API key ONCE - it cannot be retrieved again.
	c.JSON(http.StatusCreated, gin.H{
		"id":       account.ID,
		"name":     account.Name,
		"email":    account.Email,
		"balance":  account.Balance,
		"currency": account.Currency,
		"type":     account.Type,
		"api_key":  rawAPIKey,
		"warning":  "Store this API key securely. It cannot be retrieved again.",
	})
}

// GetAccount handles GET /api/v1/accounts/:id
func (h *AccountHandler) GetAccount(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid account ID"})
		return
	}

	account, err := h.repo.GetAccount(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "account not found"})
		return
	}

	c.JSON(http.StatusOK, account)
}

// ListAccounts handles GET /api/v1/accounts
func (h *AccountHandler) ListAccounts(c *gin.Context) {
	accounts, err := h.repo.ListAccounts(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list accounts"})
		return
	}

	c.JSON(http.StatusOK, accounts)
}

func generateAPIKey() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "pk_" + hex.EncodeToString(b), nil
}

// hashAPIKey creates a SHA-256 hash of the API key for secure storage.
func hashAPIKey(key string) string {
	hash := sha256.Sum256([]byte(key))
	return hex.EncodeToString(hash[:])
}
