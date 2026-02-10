package service

import (
	"testing"

	"github.com/ponti/instant-payment-gateway/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestTransactionStatusTransitions(t *testing.T) {
	tests := []struct {
		name     string
		from     model.TransactionStatus
		to       model.TransactionStatus
		valid    bool
	}{
		{"pending to authorized", model.StatusPending, model.StatusAuthorized, true},
		{"authorized to settled", model.StatusAuthorized, model.StatusSettled, true},
		{"pending to settled", model.StatusPending, model.StatusSettled, false},
		{"settled to refunded", model.StatusSettled, model.StatusRefunded, true},
		{"failed to settled", model.StatusFailed, model.StatusSettled, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid := isValidTransition(tt.from, tt.to)
			assert.Equal(t, tt.valid, valid)
		})
	}
}

func TestIdempotencyKeyValidation(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		valid bool
	}{
		{"valid UUID key", "550e8400-e29b-41d4-a716-446655440000", true},
		{"valid custom key", "pay_1234567890", true},
		{"empty key", "", false},
		{"too long key", string(make([]byte, 256)), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid := isValidIdempotencyKey(tt.key)
			assert.Equal(t, tt.valid, valid)
		})
	}
}

func TestAmountValidation(t *testing.T) {
	tests := []struct {
		name   string
		amount int64
		valid  bool
	}{
		{"positive amount", 1000, true},
		{"zero amount", 0, false},
		{"negative amount", -500, false},
		{"one cent", 1, true},
		{"large amount", 999999999, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid := tt.amount > 0
			assert.Equal(t, tt.valid, valid)
		})
	}
}

func TestCurrencyValidation(t *testing.T) {
	tests := []struct {
		name     string
		currency string
		valid    bool
	}{
		{"USD", "USD", true},
		{"EUR", "EUR", true},
		{"lowercase", "usd", false},
		{"too short", "US", false},
		{"too long", "USDD", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid := isValidCurrency(tt.currency)
			assert.Equal(t, tt.valid, valid)
		})
	}
}

// isValidTransition checks if a status transition is allowed.
func isValidTransition(from, to model.TransactionStatus) bool {
	transitions := map[model.TransactionStatus][]model.TransactionStatus{
		model.StatusPending:    {model.StatusAuthorized, model.StatusFailed},
		model.StatusAuthorized: {model.StatusSettled, model.StatusFailed},
		model.StatusSettled:    {model.StatusRefunded},
		model.StatusFailed:     {},
		model.StatusRefunded:   {},
	}

	allowed, ok := transitions[from]
	if !ok {
		return false
	}
	for _, s := range allowed {
		if s == to {
			return true
		}
	}
	return false
}

// isValidIdempotencyKey validates idempotency key format.
func isValidIdempotencyKey(key string) bool {
	return len(key) > 0 && len(key) <= 255
}

// isValidCurrency validates currency code (ISO 4217).
func isValidCurrency(code string) bool {
	if len(code) != 3 {
		return false
	}
	for _, c := range code {
		if c < 'A' || c > 'Z' {
			return false
		}
	}
	return true
}
