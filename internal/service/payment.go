package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/ponti/instant-payment-gateway/internal/model"
	"github.com/ponti/instant-payment-gateway/internal/repository"
)

// PaymentService encapsulates payment business logic.
type PaymentService struct {
	repo    *repository.PostgresRepo
	webhook *WebhookService
}

func NewPaymentService(repo *repository.PostgresRepo, wh *WebhookService) *PaymentService {
	return &PaymentService{repo: repo, webhook: wh}
}

// InitiatePayment creates a new pending payment with idempotency protection.
func (s *PaymentService) InitiatePayment(ctx context.Context, req model.InitiatePaymentRequest) (*model.Transaction, error) {
	senderID, err := uuid.Parse(req.SenderID)
	if err != nil {
		return nil, fmt.Errorf("invalid sender_id: %w", err)
	}
	receiverID, err := uuid.Parse(req.ReceiverID)
	if err != nil {
		return nil, fmt.Errorf("invalid receiver_id: %w", err)
	}

	// Verify both accounts exist before starting transaction.
	sender, err := s.repo.GetAccount(ctx, senderID)
	if err != nil {
		return nil, fmt.Errorf("sender account not found: %w", err)
	}
	receiver, err := s.repo.GetAccount(ctx, receiverID)
	if err != nil {
		return nil, fmt.Errorf("receiver account not found: %w", err)
	}

	if sender.Currency != req.Currency || receiver.Currency != req.Currency {
		return nil, fmt.Errorf("currency mismatch: accounts use %s/%s, request uses %s", sender.Currency, receiver.Currency, req.Currency)
	}

	// Begin DB transaction for atomic idempotent insert.
	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	txn := &model.Transaction{
		IdempotencyKey: req.IdempotencyKey,
		SenderID:       senderID,
		ReceiverID:     receiverID,
		Amount:         req.Amount,
		Currency:       req.Currency,
		Status:         model.StatusPending,
		Description:    req.Description,
		Metadata:       "{}",
	}

	// Use atomic INSERT ... ON CONFLICT to handle race conditions.
	// If idempotency_key already exists, returns the existing transaction.
	result, created, err := s.repo.CreateTransactionIdempotent(ctx, tx, txn)
	if err != nil {
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}

	if !created {
		// Idempotent request - return existing transaction without committing new one.
		log.Info().Str("idempotency_key", req.IdempotencyKey).Msg("duplicate payment request, returning existing")
		tx.Rollback(ctx)
		return result, nil
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit: %w", err)
	}
	txn = result

	s.repo.CreateAuditLog(ctx, &model.AuditLog{
		Actor:    senderID.String(),
		Action:   "payment.initiated",
		Resource: txn.ID.String(),
		Details:  fmt.Sprintf("amount=%d currency=%s receiver=%s", req.Amount, req.Currency, req.ReceiverID),
	})

	s.webhook.Dispatch(model.WebhookPayload{
		Event:         "payment.initiated",
		TransactionID: txn.ID.String(),
		Status:        string(txn.Status),
		Timestamp:     time.Now().UTC(),
		Data:          txn,
	})

	return txn, nil
}

// AuthorizePayment moves a transaction from pending to authorized after validating funds.
// The callerAccountID is used to verify the requester has permission to authorize this transaction.
func (s *PaymentService) AuthorizePayment(ctx context.Context, txnID uuid.UUID, callerAccountID string) (*model.Transaction, error) {
	// Start transaction FIRST to acquire locks and prevent race conditions.
	dbTx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer dbTx.Rollback(ctx)

	// Get transaction with lock to prevent concurrent status changes.
	txn, err := s.repo.GetTransactionForUpdate(ctx, dbTx, txnID)
	if err != nil {
		return nil, fmt.Errorf("transaction not found: %w", err)
	}

	// Authorization check: only the sender can authorize their own payment.
	if callerAccountID != "" && txn.SenderID.String() != callerAccountID {
		return nil, fmt.Errorf("unauthorized: only the sender can authorize this transaction")
	}

	if txn.Status != model.StatusPending {
		return nil, fmt.Errorf("transaction is not pending, current status: %s", txn.Status)
	}

	// Get sender account with lock to prevent concurrent balance modifications.
	sender, err := s.repo.GetAccountForUpdate(ctx, dbTx, txn.SenderID)
	if err != nil {
		return nil, fmt.Errorf("sender account not found: %w", err)
	}
	if sender.Balance < txn.Amount {
		return nil, fmt.Errorf("insufficient funds: available %d, required %d", sender.Balance, txn.Amount)
	}

	if err := s.repo.UpdateTransactionStatus(ctx, dbTx, txnID, model.StatusAuthorized, ""); err != nil {
		return nil, err
	}

	if err := dbTx.Commit(ctx); err != nil {
		return nil, err
	}

	txn.Status = model.StatusAuthorized
	s.webhook.Dispatch(model.WebhookPayload{
		Event:         "payment.authorized",
		TransactionID: txn.ID.String(),
		Status:        string(txn.Status),
		Timestamp:     time.Now().UTC(),
		Data:          txn,
	})

	return txn, nil
}

// SettlePayment moves funds and creates double-entry ledger records.
// The callerAccountID is used to verify the requester has permission to settle this transaction.
func (s *PaymentService) SettlePayment(ctx context.Context, txnID uuid.UUID, callerAccountID string) (*model.Transaction, error) {
	// Start transaction FIRST to acquire all locks atomically.
	dbTx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer dbTx.Rollback(ctx)

	// Get transaction with lock to prevent concurrent status changes.
	txn, err := s.repo.GetTransactionForUpdate(ctx, dbTx, txnID)
	if err != nil {
		return nil, fmt.Errorf("transaction not found: %w", err)
	}

	// Authorization check: only the sender or receiver can settle the transaction.
	if callerAccountID != "" {
		if txn.SenderID.String() != callerAccountID && txn.ReceiverID.String() != callerAccountID {
			return nil, fmt.Errorf("unauthorized: only the sender or receiver can settle this transaction")
		}
	}

	if txn.Status != model.StatusAuthorized {
		return nil, fmt.Errorf("transaction must be authorized before settlement, current status: %s", txn.Status)
	}

	// Lock both accounts to prevent concurrent balance modifications.
	// Order by ID to prevent deadlocks.
	var senderBalance, receiverBalance int64
	if txn.SenderID.String() < txn.ReceiverID.String() {
		_, err = s.repo.GetAccountForUpdate(ctx, dbTx, txn.SenderID)
		if err != nil {
			return nil, fmt.Errorf("sender account not found: %w", err)
		}
		_, err = s.repo.GetAccountForUpdate(ctx, dbTx, txn.ReceiverID)
		if err != nil {
			return nil, fmt.Errorf("receiver account not found: %w", err)
		}
	} else {
		_, err = s.repo.GetAccountForUpdate(ctx, dbTx, txn.ReceiverID)
		if err != nil {
			return nil, fmt.Errorf("receiver account not found: %w", err)
		}
		_, err = s.repo.GetAccountForUpdate(ctx, dbTx, txn.SenderID)
		if err != nil {
			return nil, fmt.Errorf("sender account not found: %w", err)
		}
	}

	// Debit sender.
	senderBalance, err = s.repo.UpdateBalance(ctx, dbTx, txn.SenderID, -txn.Amount)
	if err != nil {
		return nil, fmt.Errorf("failed to debit sender: %w", err)
	}
	if senderBalance < 0 {
		return nil, fmt.Errorf("insufficient funds during settlement")
	}

	// Credit receiver.
	receiverBalance, err = s.repo.UpdateBalance(ctx, dbTx, txn.ReceiverID, txn.Amount)
	if err != nil {
		return nil, fmt.Errorf("failed to credit receiver: %w", err)
	}

	// Double-entry ledger: debit entry (money leaving sender).
	if err := s.repo.CreateLedgerEntry(ctx, dbTx, &model.LedgerEntry{
		TransactionID: txnID,
		AccountID:     txn.SenderID,
		Type:          model.EntryDebit,
		Amount:        txn.Amount,
		Currency:      txn.Currency,
		BalanceAfter:  senderBalance,
	}); err != nil {
		return nil, err
	}

	// Double-entry ledger: credit entry (money entering receiver).
	if err := s.repo.CreateLedgerEntry(ctx, dbTx, &model.LedgerEntry{
		TransactionID: txnID,
		AccountID:     txn.ReceiverID,
		Type:          model.EntryCredit,
		Amount:        txn.Amount,
		Currency:      txn.Currency,
		BalanceAfter:  receiverBalance,
	}); err != nil {
		return nil, err
	}

	providerRef := fmt.Sprintf("SIM-%s", uuid.New().String()[:8])
	if err := s.repo.UpdateTransactionStatus(ctx, dbTx, txnID, model.StatusSettled, providerRef); err != nil {
		return nil, err
	}

	if err := dbTx.Commit(ctx); err != nil {
		return nil, err
	}

	txn.Status = model.StatusSettled
	txn.ProviderRef = providerRef

	s.repo.CreateAuditLog(ctx, &model.AuditLog{
		Actor:    callerAccountID,
		Action:   "payment.settled",
		Resource: txn.ID.String(),
		Details:  fmt.Sprintf("amount=%d provider_ref=%s", txn.Amount, providerRef),
	})

	s.webhook.Dispatch(model.WebhookPayload{
		Event:         "payment.settled",
		TransactionID: txn.ID.String(),
		Status:        string(txn.Status),
		Timestamp:     time.Now().UTC(),
		Data:          txn,
	})

	return txn, nil
}

// GetTransaction returns a single transaction.
func (s *PaymentService) GetTransaction(ctx context.Context, id uuid.UUID) (*model.Transaction, error) {
	return s.repo.GetTransaction(ctx, id)
}

// ListTransactions returns filtered transactions.
func (s *PaymentService) ListTransactions(ctx context.Context, filter model.TransactionFilter) ([]model.Transaction, error) {
	return s.repo.ListTransactions(ctx, filter)
}

// GetLedgerEntries returns ledger entries for a transaction.
func (s *PaymentService) GetLedgerEntries(ctx context.Context, txnID uuid.UUID) ([]model.LedgerEntry, error) {
	return s.repo.GetLedgerEntries(ctx, txnID)
}

// GetDailyAnalytics returns aggregated transaction analytics for a date.
func (s *PaymentService) GetDailyAnalytics(ctx context.Context, date string) (*model.AnalyticsResponse, error) {
	return s.repo.GetDailyAnalytics(ctx, date)
}

// marshalJSON is a helper to convert a value to JSON string.
func marshalJSON(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}
