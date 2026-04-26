-- Migration: 001_initial_schema.sql
-- PayRoute Cross-Border Payment System
-- Run order: this file must run first

-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- ============================================================
-- businesses
-- Represents companies using the PayRoute platform
-- ============================================================
CREATE TABLE IF NOT EXISTS businesses (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name       TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ============================================================
-- accounts
-- Each business has one account per currency.
-- balance is stored as NUMERIC to avoid floating-point drift.
-- A CHECK constraint prevents negative balances at DB level
-- as a last line of defence (application enforces this first
-- via SELECT FOR UPDATE + balance check).
-- ============================================================
CREATE TABLE IF NOT EXISTS accounts (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    business_id UUID NOT NULL REFERENCES businesses(id),
    currency    TEXT NOT NULL,
    balance     NUMERIC(20, 8) NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT accounts_balance_non_negative CHECK (balance >= 0),
    CONSTRAINT accounts_business_currency_unique UNIQUE (business_id, currency)
);

CREATE INDEX IF NOT EXISTS idx_accounts_business_id ON accounts(business_id);

-- ============================================================
-- fx_quotes
-- Stores point-in-time FX rates with expiry.
-- Quotes are locked at payment initiation and stored for audit.
-- ============================================================
CREATE TABLE IF NOT EXISTS fx_quotes (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    from_currency TEXT NOT NULL,
    to_currency   TEXT NOT NULL,
    rate          NUMERIC(20, 10) NOT NULL,
    expires_at    TIMESTAMPTZ NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_fx_quotes_expires_at ON fx_quotes(expires_at);

-- ============================================================
-- transactions
-- Core payment record. Status transitions are:
--   initiated → processing → completed
--                          → failed
--                          → reversed
-- source_amount is always NGN (sender side).
-- destination_amount is in destination_currency.
-- ============================================================
CREATE TYPE transaction_status AS ENUM (
    'initiated',
    'processing',
    'completed',
    'failed',
    'reversed'
);

CREATE TABLE IF NOT EXISTS transactions (
    id                   UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    reference            TEXT NOT NULL UNIQUE,
    sender_account_id    UUID NOT NULL REFERENCES accounts(id),
    recipient_name       TEXT NOT NULL,
    recipient_country    TEXT NOT NULL,
    destination_currency TEXT NOT NULL,
    source_amount        NUMERIC(20, 8) NOT NULL,
    destination_amount   NUMERIC(20, 8) NOT NULL,
    fx_rate              NUMERIC(20, 10) NOT NULL,
    fx_quote_id          UUID REFERENCES fx_quotes(id),
    status               transaction_status NOT NULL DEFAULT 'initiated',
    provider_reference   TEXT,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at         TIMESTAMPTZ,

    CONSTRAINT transactions_source_amount_positive  CHECK (source_amount > 0),
    CONSTRAINT transactions_dest_amount_positive    CHECK (destination_amount > 0),
    CONSTRAINT transactions_fx_rate_positive        CHECK (fx_rate > 0)
);

CREATE INDEX IF NOT EXISTS idx_transactions_status      ON transactions(status);
CREATE INDEX IF NOT EXISTS idx_transactions_created_at  ON transactions(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_transactions_provider_ref ON transactions(provider_reference);
CREATE INDEX IF NOT EXISTS idx_transactions_sender      ON transactions(sender_account_id);

-- ============================================================
-- transaction_status_history
-- Immutable audit log of every status transition.
-- Never update or delete rows here.
-- ============================================================
CREATE TABLE IF NOT EXISTS transaction_status_history (
    id             UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    transaction_id UUID NOT NULL REFERENCES transactions(id),
    from_status    transaction_status,
    to_status      transaction_status NOT NULL,
    reason         TEXT,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_status_history_tx_id ON transaction_status_history(transaction_id);

-- ============================================================
-- ledger_entries
-- Double-entry bookkeeping. Every balance change produces
-- at least two entries that sum to zero across a transaction.
-- entry_type: 'debit' reduces balance, 'credit' increases it.
-- ============================================================
CREATE TYPE ledger_entry_type AS ENUM ('debit', 'credit');

CREATE TABLE IF NOT EXISTS ledger_entries (
    id             UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    transaction_id UUID NOT NULL REFERENCES transactions(id),
    account_id     UUID NOT NULL REFERENCES accounts(id),
    amount         NUMERIC(20, 8) NOT NULL,
    currency       TEXT NOT NULL,
    entry_type     ledger_entry_type NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT ledger_amount_positive CHECK (amount > 0)
);

CREATE INDEX IF NOT EXISTS idx_ledger_transaction_id ON ledger_entries(transaction_id);
CREATE INDEX IF NOT EXISTS idx_ledger_account_id     ON ledger_entries(account_id);

-- ============================================================
-- webhook_events
-- Raw log of every inbound webhook BEFORE processing.
-- Stored regardless of whether processing succeeded.
-- This is the source of truth for provider communications.
-- ============================================================
CREATE TABLE IF NOT EXISTS webhook_events (
    id                 UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    provider_reference TEXT NOT NULL,
    payload            JSONB NOT NULL,
    headers            JSONB NOT NULL DEFAULT '{}',
    received_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed          BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE INDEX IF NOT EXISTS idx_webhook_provider_ref ON webhook_events(provider_reference);
CREATE INDEX IF NOT EXISTS idx_webhook_processed    ON webhook_events(processed) WHERE processed = FALSE;

-- ============================================================
-- idempotency_keys
-- Prevents duplicate payment creation.
-- Keyed on (key, endpoint) — same key on different endpoints
-- is allowed (though discouraged by clients).
-- ============================================================
CREATE TABLE IF NOT EXISTS idempotency_keys (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    key        TEXT NOT NULL,
    endpoint   TEXT NOT NULL,
    response   JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT idempotency_keys_unique UNIQUE (key, endpoint)
);

CREATE INDEX IF NOT EXISTS idx_idempotency_key ON idempotency_keys(key);
