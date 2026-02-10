-- Migration: 001_initial
-- Creates the core schema for the Instant Payment Gateway.

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Accounts table: user and merchant accounts with balances.
CREATE TABLE accounts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    balance BIGINT NOT NULL DEFAULT 0,
    currency CHAR(3) NOT NULL DEFAULT 'USD',
    type VARCHAR(20) NOT NULL CHECK (type IN ('user', 'merchant')),
    api_key VARCHAR(255) UNIQUE NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_accounts_api_key ON accounts(api_key);
CREATE INDEX idx_accounts_email ON accounts(email);

-- Transactions table: payment records with status tracking.
CREATE TABLE transactions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    idempotency_key VARCHAR(255) UNIQUE NOT NULL,
    sender_id UUID NOT NULL REFERENCES accounts(id),
    receiver_id UUID NOT NULL REFERENCES accounts(id),
    amount BIGINT NOT NULL CHECK (amount > 0),
    currency CHAR(3) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'authorized', 'settled', 'failed', 'refunded')),
    description TEXT DEFAULT '',
    metadata JSONB DEFAULT '{}',
    provider_ref VARCHAR(255) DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_transactions_sender ON transactions(sender_id);
CREATE INDEX idx_transactions_receiver ON transactions(receiver_id);
CREATE INDEX idx_transactions_status ON transactions(status);
CREATE INDEX idx_transactions_idempotency ON transactions(idempotency_key);
CREATE INDEX idx_transactions_created ON transactions(created_at);

-- Ledger entries: double-entry bookkeeping.
CREATE TABLE ledger_entries (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    transaction_id UUID NOT NULL REFERENCES transactions(id),
    account_id UUID NOT NULL REFERENCES accounts(id),
    type VARCHAR(10) NOT NULL CHECK (type IN ('debit', 'credit')),
    amount BIGINT NOT NULL CHECK (amount > 0),
    currency CHAR(3) NOT NULL,
    balance_after BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ledger_transaction ON ledger_entries(transaction_id);
CREATE INDEX idx_ledger_account ON ledger_entries(account_id);

-- Audit log: immutable event log.
CREATE TABLE audit_log (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    actor VARCHAR(255) NOT NULL,
    action VARCHAR(100) NOT NULL,
    resource VARCHAR(255) NOT NULL,
    details TEXT DEFAULT '',
    ip VARCHAR(45) DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_audit_actor ON audit_log(actor);
CREATE INDEX idx_audit_action ON audit_log(action);
CREATE INDEX idx_audit_created ON audit_log(created_at);

-- Prevent balance from going negative (enforced at DB level).
ALTER TABLE accounts ADD CONSTRAINT chk_balance_non_negative CHECK (balance >= 0);
