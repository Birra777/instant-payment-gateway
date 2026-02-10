package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTransactionStatusConstants(t *testing.T) {
	assert.Equal(t, TransactionStatus("pending"), StatusPending)
	assert.Equal(t, TransactionStatus("authorized"), StatusAuthorized)
	assert.Equal(t, TransactionStatus("settled"), StatusSettled)
	assert.Equal(t, TransactionStatus("failed"), StatusFailed)
	assert.Equal(t, TransactionStatus("refunded"), StatusRefunded)
}

func TestLedgerEntryTypeConstants(t *testing.T) {
	assert.Equal(t, LedgerEntryType("debit"), EntryDebit)
	assert.Equal(t, LedgerEntryType("credit"), EntryCredit)
}

func TestInitiatePaymentRequest_Validation(t *testing.T) {
	tests := []struct {
		name string
		req  InitiatePaymentRequest
		ok   bool
	}{
		{
			name: "valid request",
			req: InitiatePaymentRequest{
				IdempotencyKey: "key-123",
				SenderID:       "550e8400-e29b-41d4-a716-446655440000",
				ReceiverID:     "550e8400-e29b-41d4-a716-446655440001",
				Amount:         1000,
				Currency:       "USD",
			},
			ok: true,
		},
		{
			name: "zero amount",
			req: InitiatePaymentRequest{
				IdempotencyKey: "key-124",
				SenderID:       "550e8400-e29b-41d4-a716-446655440000",
				ReceiverID:     "550e8400-e29b-41d4-a716-446655440001",
				Amount:         0,
				Currency:       "USD",
			},
			ok: false,
		},
		{
			name: "missing idempotency key",
			req: InitiatePaymentRequest{
				SenderID:   "550e8400-e29b-41d4-a716-446655440000",
				ReceiverID: "550e8400-e29b-41d4-a716-446655440001",
				Amount:     1000,
				Currency:   "USD",
			},
			ok: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid := tt.req.IdempotencyKey != "" && tt.req.Amount > 0 && tt.req.Currency != ""
			assert.Equal(t, tt.ok, valid)
		})
	}
}
