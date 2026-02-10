package model

import (
	"time"

	"github.com/google/uuid"
)

// TransactionStatus represents the state of a transaction.
type TransactionStatus string

const (
	StatusPending    TransactionStatus = "pending"
	StatusAuthorized TransactionStatus = "authorized"
	StatusSettled    TransactionStatus = "settled"
	StatusFailed     TransactionStatus = "failed"
	StatusRefunded   TransactionStatus = "refunded"
)

// LedgerEntryType represents the type of a ledger entry.
type LedgerEntryType string

const (
	EntryDebit  LedgerEntryType = "debit"
	EntryCredit LedgerEntryType = "credit"
)

// Account represents a user or merchant account.
type Account struct {
	ID        uuid.UUID `json:"id" db:"id"`
	Name      string    `json:"name" db:"name"`
	Email     string    `json:"email" db:"email"`
	Balance   int64     `json:"balance" db:"balance"` // stored in smallest currency unit (cents)
	Currency  string    `json:"currency" db:"currency"`
	Type      string    `json:"type" db:"type"` // "user" or "merchant"
	APIKey    string    `json:"-" db:"api_key"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// Transaction represents a payment record.
type Transaction struct {
	ID             uuid.UUID         `json:"id" db:"id"`
	IdempotencyKey string            `json:"idempotency_key" db:"idempotency_key"`
	SenderID       uuid.UUID         `json:"sender_id" db:"sender_id"`
	ReceiverID     uuid.UUID         `json:"receiver_id" db:"receiver_id"`
	Amount         int64             `json:"amount" db:"amount"`
	Currency       string            `json:"currency" db:"currency"`
	Status         TransactionStatus `json:"status" db:"status"`
	Description    string            `json:"description" db:"description"`
	Metadata       string            `json:"metadata" db:"metadata"`
	ProviderRef    string            `json:"provider_ref" db:"provider_ref"`
	CreatedAt      time.Time         `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at" db:"updated_at"`
}

// LedgerEntry represents a double-entry bookkeeping record.
type LedgerEntry struct {
	ID            uuid.UUID       `json:"id" db:"id"`
	TransactionID uuid.UUID       `json:"transaction_id" db:"transaction_id"`
	AccountID     uuid.UUID       `json:"account_id" db:"account_id"`
	Type          LedgerEntryType `json:"type" db:"type"`
	Amount        int64           `json:"amount" db:"amount"`
	Currency      string          `json:"currency" db:"currency"`
	BalanceAfter  int64           `json:"balance_after" db:"balance_after"`
	CreatedAt     time.Time       `json:"created_at" db:"created_at"`
}

// AuditLog represents an immutable event log entry.
type AuditLog struct {
	ID        uuid.UUID `json:"id" db:"id"`
	Actor     string    `json:"actor" db:"actor"`
	Action    string    `json:"action" db:"action"`
	Resource  string    `json:"resource" db:"resource"`
	Details   string    `json:"details" db:"details"`
	IP        string    `json:"ip" db:"ip"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// --- Request / Response DTOs ---

type InitiatePaymentRequest struct {
	IdempotencyKey string `json:"idempotency_key" binding:"required"`
	SenderID       string `json:"sender_id" binding:"required,uuid"`
	ReceiverID     string `json:"receiver_id" binding:"required,uuid"`
	Amount         int64  `json:"amount" binding:"required,gt=0"`
	Currency       string `json:"currency" binding:"required,len=3"`
	Description    string `json:"description"`
}

type AuthorizePaymentRequest struct {
	TransactionID string `json:"transaction_id" binding:"required,uuid"`
}

type TransactionFilter struct {
	AccountID string            `form:"account_id"`
	Status    TransactionStatus `form:"status"`
	DateFrom  string            `form:"date_from"`
	DateTo    string            `form:"date_to"`
	Limit     int               `form:"limit"`
	Offset    int               `form:"offset"`
}

type AnalyticsResponse struct {
	DailyVolume    int64   `json:"daily_volume"`
	TotalCount     int64   `json:"total_count"`
	SuccessCount   int64   `json:"success_count"`
	FailureCount   int64   `json:"failure_count"`
	SuccessRate    float64 `json:"success_rate"`
	AverageAmount  float64 `json:"average_amount"`
	Date           string  `json:"date"`
}

type WebhookPayload struct {
	Event         string      `json:"event"`
	TransactionID string      `json:"transaction_id"`
	Status        string      `json:"status"`
	Timestamp     time.Time   `json:"timestamp"`
	Data          interface{} `json:"data"`
}
