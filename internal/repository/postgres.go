package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ponti/instant-payment-gateway/internal/model"
)

// PostgresRepo implements data access for all entities.
type PostgresRepo struct {
	pool *pgxpool.Pool
}

func NewPostgresRepo(pool *pgxpool.Pool) *PostgresRepo {
	return &PostgresRepo{pool: pool}
}

// --- Account ---

func (r *PostgresRepo) CreateAccount(ctx context.Context, a *model.Account) error {
	a.ID = uuid.New()
	a.CreatedAt = time.Now().UTC()
	a.UpdatedAt = a.CreatedAt
	_, err := r.pool.Exec(ctx,
		`INSERT INTO accounts (id, name, email, balance, currency, type, api_key, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		a.ID, a.Name, a.Email, a.Balance, a.Currency, a.Type, a.APIKey, a.CreatedAt, a.UpdatedAt,
	)
	return err
}

func (r *PostgresRepo) GetAccount(ctx context.Context, id uuid.UUID) (*model.Account, error) {
	a := &model.Account{}
	err := r.pool.QueryRow(ctx,
		`SELECT id, name, email, balance, currency, type, api_key, created_at, updated_at
		 FROM accounts WHERE id = $1`, id,
	).Scan(&a.ID, &a.Name, &a.Email, &a.Balance, &a.Currency, &a.Type, &a.APIKey, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return a, nil
}

// GetAccountForUpdate retrieves an account with a row lock for safe balance operations.
// This must be called within a transaction to prevent race conditions.
func (r *PostgresRepo) GetAccountForUpdate(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*model.Account, error) {
	a := &model.Account{}
	err := tx.QueryRow(ctx,
		`SELECT id, name, email, balance, currency, type, api_key, created_at, updated_at
		 FROM accounts WHERE id = $1 FOR UPDATE`, id,
	).Scan(&a.ID, &a.Name, &a.Email, &a.Balance, &a.Currency, &a.Type, &a.APIKey, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return a, nil
}

func (r *PostgresRepo) GetAccountByAPIKey(ctx context.Context, key string) (*model.Account, error) {
	a := &model.Account{}
	err := r.pool.QueryRow(ctx,
		`SELECT id, name, email, balance, currency, type, api_key, created_at, updated_at
		 FROM accounts WHERE api_key = $1`, key,
	).Scan(&a.ID, &a.Name, &a.Email, &a.Balance, &a.Currency, &a.Type, &a.APIKey, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return a, nil
}

func (r *PostgresRepo) UpdateBalance(ctx context.Context, tx pgx.Tx, accountID uuid.UUID, delta int64) (int64, error) {
	var newBalance int64
	err := tx.QueryRow(ctx,
		`UPDATE accounts SET balance = balance + $1, updated_at = NOW()
		 WHERE id = $2 RETURNING balance`, delta, accountID,
	).Scan(&newBalance)
	return newBalance, err
}

func (r *PostgresRepo) ListAccounts(ctx context.Context) ([]model.Account, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, name, email, balance, currency, type, api_key, created_at, updated_at FROM accounts ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accounts []model.Account
	for rows.Next() {
		var a model.Account
		if err := rows.Scan(&a.ID, &a.Name, &a.Email, &a.Balance, &a.Currency, &a.Type, &a.APIKey, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		accounts = append(accounts, a)
	}
	return accounts, nil
}

// --- Transaction ---

// CreateTransaction inserts a new transaction. Returns an error if idempotency_key already exists.
func (r *PostgresRepo) CreateTransaction(ctx context.Context, tx pgx.Tx, t *model.Transaction) error {
	t.ID = uuid.New()
	t.CreatedAt = time.Now().UTC()
	t.UpdatedAt = t.CreatedAt
	_, err := tx.Exec(ctx,
		`INSERT INTO transactions (id, idempotency_key, sender_id, receiver_id, amount, currency, status, description, metadata, provider_ref, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		t.ID, t.IdempotencyKey, t.SenderID, t.ReceiverID, t.Amount, t.Currency,
		t.Status, t.Description, t.Metadata, t.ProviderRef, t.CreatedAt, t.UpdatedAt,
	)
	return err
}

// CreateTransactionIdempotent inserts a new transaction or returns the existing one if idempotency_key exists.
// This uses INSERT ... ON CONFLICT to handle race conditions atomically.
// Returns (transaction, created) where created is true if a new transaction was inserted.
func (r *PostgresRepo) CreateTransactionIdempotent(ctx context.Context, tx pgx.Tx, t *model.Transaction) (*model.Transaction, bool, error) {
	t.ID = uuid.New()
	t.CreatedAt = time.Now().UTC()
	t.UpdatedAt = t.CreatedAt

	var resultID uuid.UUID
	var wasInserted bool

	// Use a CTE to attempt insert and detect if it was actually inserted
	err := tx.QueryRow(ctx,
		`WITH ins AS (
			INSERT INTO transactions (id, idempotency_key, sender_id, receiver_id, amount, currency, status, description, metadata, provider_ref, created_at, updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
			ON CONFLICT (idempotency_key) DO NOTHING
			RETURNING id, true as inserted
		)
		SELECT COALESCE(ins.id, existing.id), COALESCE(ins.inserted, false)
		FROM (SELECT $2::varchar as key) k
		LEFT JOIN ins ON true
		LEFT JOIN transactions existing ON existing.idempotency_key = k.key AND ins.id IS NULL`,
		t.ID, t.IdempotencyKey, t.SenderID, t.ReceiverID, t.Amount, t.Currency,
		t.Status, t.Description, t.Metadata, t.ProviderRef, t.CreatedAt, t.UpdatedAt,
	).Scan(&resultID, &wasInserted)

	if err != nil {
		return nil, false, err
	}

	if wasInserted {
		t.ID = resultID
		return t, true, nil
	}

	// Fetch the existing transaction
	existing, err := r.GetTransactionByIdempotencyKeyForUpdate(ctx, tx, t.IdempotencyKey)
	if err != nil {
		return nil, false, err
	}
	return existing, false, nil
}

func (r *PostgresRepo) GetTransactionByIdempotencyKey(ctx context.Context, key string) (*model.Transaction, error) {
	t := &model.Transaction{}
	err := r.pool.QueryRow(ctx,
		`SELECT id, idempotency_key, sender_id, receiver_id, amount, currency, status, description, metadata, provider_ref, created_at, updated_at
		 FROM transactions WHERE idempotency_key = $1`, key,
	).Scan(&t.ID, &t.IdempotencyKey, &t.SenderID, &t.ReceiverID, &t.Amount, &t.Currency,
		&t.Status, &t.Description, &t.Metadata, &t.ProviderRef, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return t, nil
}

// GetTransactionByIdempotencyKeyForUpdate retrieves a transaction by idempotency key with a row lock.
// This must be called within a transaction to prevent race conditions.
func (r *PostgresRepo) GetTransactionByIdempotencyKeyForUpdate(ctx context.Context, tx pgx.Tx, key string) (*model.Transaction, error) {
	t := &model.Transaction{}
	err := tx.QueryRow(ctx,
		`SELECT id, idempotency_key, sender_id, receiver_id, amount, currency, status, description, metadata, provider_ref, created_at, updated_at
		 FROM transactions WHERE idempotency_key = $1 FOR UPDATE`, key,
	).Scan(&t.ID, &t.IdempotencyKey, &t.SenderID, &t.ReceiverID, &t.Amount, &t.Currency,
		&t.Status, &t.Description, &t.Metadata, &t.ProviderRef, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return t, nil
}

func (r *PostgresRepo) GetTransaction(ctx context.Context, id uuid.UUID) (*model.Transaction, error) {
	t := &model.Transaction{}
	err := r.pool.QueryRow(ctx,
		`SELECT id, idempotency_key, sender_id, receiver_id, amount, currency, status, description, metadata, provider_ref, created_at, updated_at
		 FROM transactions WHERE id = $1`, id,
	).Scan(&t.ID, &t.IdempotencyKey, &t.SenderID, &t.ReceiverID, &t.Amount, &t.Currency,
		&t.Status, &t.Description, &t.Metadata, &t.ProviderRef, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return t, nil
}

// GetTransactionForUpdate retrieves a transaction with a row lock for safe status transitions.
// This must be called within a transaction to prevent race conditions.
func (r *PostgresRepo) GetTransactionForUpdate(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*model.Transaction, error) {
	t := &model.Transaction{}
	err := tx.QueryRow(ctx,
		`SELECT id, idempotency_key, sender_id, receiver_id, amount, currency, status, description, metadata, provider_ref, created_at, updated_at
		 FROM transactions WHERE id = $1 FOR UPDATE`, id,
	).Scan(&t.ID, &t.IdempotencyKey, &t.SenderID, &t.ReceiverID, &t.Amount, &t.Currency,
		&t.Status, &t.Description, &t.Metadata, &t.ProviderRef, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return t, nil
}

func (r *PostgresRepo) UpdateTransactionStatus(ctx context.Context, tx pgx.Tx, id uuid.UUID, status model.TransactionStatus, providerRef string) error {
	_, err := tx.Exec(ctx,
		`UPDATE transactions SET status = $1, provider_ref = $2, updated_at = NOW() WHERE id = $3`,
		status, providerRef, id,
	)
	return err
}

func (r *PostgresRepo) ListTransactions(ctx context.Context, f model.TransactionFilter) ([]model.Transaction, error) {
	query := `SELECT id, idempotency_key, sender_id, receiver_id, amount, currency, status, description, metadata, provider_ref, created_at, updated_at FROM transactions WHERE 1=1`
	args := []interface{}{}
	argIdx := 1

	if f.AccountID != "" {
		query += fmt.Sprintf(` AND (sender_id = $%d OR receiver_id = $%d)`, argIdx, argIdx)
		args = append(args, f.AccountID)
		argIdx++
	}
	if f.Status != "" {
		query += fmt.Sprintf(` AND status = $%d`, argIdx)
		args = append(args, f.Status)
		argIdx++
	}
	if f.DateFrom != "" {
		query += fmt.Sprintf(` AND created_at >= $%d`, argIdx)
		args = append(args, f.DateFrom)
		argIdx++
	}
	if f.DateTo != "" {
		query += fmt.Sprintf(` AND created_at <= $%d`, argIdx)
		args = append(args, f.DateTo)
		argIdx++
	}

	query += ` ORDER BY created_at DESC`

	limit := f.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	query += fmt.Sprintf(` LIMIT $%d`, argIdx)
	args = append(args, limit)
	argIdx++

	if f.Offset > 0 {
		query += fmt.Sprintf(` OFFSET $%d`, argIdx)
		args = append(args, f.Offset)
	}

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var txns []model.Transaction
	for rows.Next() {
		var t model.Transaction
		if err := rows.Scan(&t.ID, &t.IdempotencyKey, &t.SenderID, &t.ReceiverID, &t.Amount, &t.Currency,
			&t.Status, &t.Description, &t.Metadata, &t.ProviderRef, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		txns = append(txns, t)
	}
	return txns, nil
}

// --- Ledger ---

func (r *PostgresRepo) CreateLedgerEntry(ctx context.Context, tx pgx.Tx, entry *model.LedgerEntry) error {
	entry.ID = uuid.New()
	entry.CreatedAt = time.Now().UTC()
	_, err := tx.Exec(ctx,
		`INSERT INTO ledger_entries (id, transaction_id, account_id, type, amount, currency, balance_after, created_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		entry.ID, entry.TransactionID, entry.AccountID, entry.Type, entry.Amount, entry.Currency, entry.BalanceAfter, entry.CreatedAt,
	)
	return err
}

func (r *PostgresRepo) GetLedgerEntries(ctx context.Context, txnID uuid.UUID) ([]model.LedgerEntry, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, transaction_id, account_id, type, amount, currency, balance_after, created_at
		 FROM ledger_entries WHERE transaction_id = $1 ORDER BY created_at`, txnID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []model.LedgerEntry
	for rows.Next() {
		var e model.LedgerEntry
		if err := rows.Scan(&e.ID, &e.TransactionID, &e.AccountID, &e.Type, &e.Amount, &e.Currency, &e.BalanceAfter, &e.CreatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, nil
}

// --- Audit Log ---

func (r *PostgresRepo) CreateAuditLog(ctx context.Context, log *model.AuditLog) error {
	log.ID = uuid.New()
	log.CreatedAt = time.Now().UTC()
	_, err := r.pool.Exec(ctx,
		`INSERT INTO audit_log (id, actor, action, resource, details, ip, created_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		log.ID, log.Actor, log.Action, log.Resource, log.Details, log.IP, log.CreatedAt,
	)
	return err
}

// --- Analytics ---

func (r *PostgresRepo) GetDailyAnalytics(ctx context.Context, date string) (*model.AnalyticsResponse, error) {
	a := &model.AnalyticsResponse{Date: date}
	err := r.pool.QueryRow(ctx,
		`SELECT
			COALESCE(SUM(amount), 0) as daily_volume,
			COUNT(*) as total_count,
			COUNT(*) FILTER (WHERE status = 'settled') as success_count,
			COUNT(*) FILTER (WHERE status = 'failed') as failure_count,
			COALESCE(AVG(amount), 0) as average_amount
		 FROM transactions
		 WHERE created_at::date = $1`, date,
	).Scan(&a.DailyVolume, &a.TotalCount, &a.SuccessCount, &a.FailureCount, &a.AverageAmount)
	if err != nil {
		return nil, err
	}
	if a.TotalCount > 0 {
		a.SuccessRate = float64(a.SuccessCount) / float64(a.TotalCount) * 100
	}
	return a, nil
}

// BeginTx starts a new database transaction.
func (r *PostgresRepo) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return r.pool.Begin(ctx)
}

// Pool returns the underlying connection pool.
func (r *PostgresRepo) Pool() *pgxpool.Pool {
	return r.pool
}

// --- Helpers ---

func buildPlaceholders(count int) string {
	ph := make([]string, count)
	for i := range ph {
		ph[i] = fmt.Sprintf("$%d", i+1)
	}
	return strings.Join(ph, ", ")
}
