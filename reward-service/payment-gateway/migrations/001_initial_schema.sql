-- ============================================================
-- Migration: 001_initial_schema.sql
-- Enterprise Payment Gateway - Full Schema
-- ============================================================

-- Extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- ============================================================
-- MERCHANTS
-- ============================================================
CREATE TABLE merchants (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name            VARCHAR(255) NOT NULL,
    business_type   VARCHAR(100),
    tax_id          VARCHAR(50),
    api_key         VARCHAR(255) UNIQUE NOT NULL,
    api_secret      TEXT NOT NULL,
    webhook_url     TEXT,
    webhook_secret  TEXT,
    is_active       BOOLEAN NOT NULL DEFAULT TRUE,
    providers       JSONB NOT NULL DEFAULT '{}',
    fee_config      JSONB NOT NULL DEFAULT '{}',
    limit_config    JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_merchants_api_key ON merchants(api_key);
CREATE INDEX idx_merchants_is_active ON merchants(is_active);

-- ============================================================
-- PAYMENTS
-- ============================================================
CREATE TABLE payments (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    merchant_id     UUID NOT NULL REFERENCES merchants(id),
    order_id        VARCHAR(100) NOT NULL,
    amount          NUMERIC(20, 6) NOT NULL CHECK (amount > 0),
    currency        VARCHAR(3) NOT NULL DEFAULT 'THB',
    method          VARCHAR(50) NOT NULL,
    provider        VARCHAR(50) NOT NULL,
    status          VARCHAR(30) NOT NULL DEFAULT 'PENDING',
    reference_id    VARCHAR(255),
    provider_ref_id VARCHAR(255),
    description     TEXT,
    customer_id     VARCHAR(255),
    customer_email  VARCHAR(255),
    customer_phone  VARCHAR(50),
    customer_name   VARCHAR(255),
    ip_address      INET,
    user_agent      TEXT,
    metadata        JSONB NOT NULL DEFAULT '{}',
    callback_url    TEXT,
    return_url      TEXT,
    qr_code_url     TEXT,
    qr_code_data    TEXT,
    bank_account_no VARCHAR(50),
    expires_at      TIMESTAMPTZ,
    paid_at         TIMESTAMPTZ,
    fee             NUMERIC(20, 6) NOT NULL DEFAULT 0,
    net_amount      NUMERIC(20, 6) NOT NULL DEFAULT 0,
    exchange_rate   NUMERIC(20, 8) NOT NULL DEFAULT 1,
    failure_code    VARCHAR(100),
    failure_message TEXT,
    risk_score      INTEGER NOT NULL DEFAULT 0,
    risk_level      VARCHAR(20) NOT NULL DEFAULT 'LOW',
    idempotency_key VARCHAR(255),
    retry_count     INTEGER NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_merchant_order UNIQUE (merchant_id, order_id),
    CONSTRAINT chk_payment_status CHECK (status IN (
        'PENDING','PROCESSING','SUCCESS','FAILED',
        'CANCELLED','REFUNDED','PARTIALLY_REFUNDED',
        'EXPIRED','DISPUTED','CHARGEBACK'
    )),
    CONSTRAINT chk_risk_level CHECK (risk_level IN ('LOW','MEDIUM','HIGH','CRITICAL'))
);

-- Indexes for common query patterns
CREATE INDEX idx_payments_merchant_id      ON payments(merchant_id);
CREATE INDEX idx_payments_order_id         ON payments(merchant_id, order_id);
CREATE INDEX idx_payments_status           ON payments(status);
CREATE INDEX idx_payments_created_at       ON payments(created_at DESC);
CREATE INDEX idx_payments_merchant_status  ON payments(merchant_id, status);
CREATE INDEX idx_payments_customer_id      ON payments(customer_id) WHERE customer_id IS NOT NULL;
CREATE INDEX idx_payments_provider_ref     ON payments(provider_ref_id) WHERE provider_ref_id IS NOT NULL;
CREATE INDEX idx_payments_idempotency      ON payments(merchant_id, idempotency_key) WHERE idempotency_key IS NOT NULL;
CREATE INDEX idx_payments_paid_at          ON payments(paid_at DESC) WHERE paid_at IS NOT NULL;
CREATE INDEX idx_payments_expires_at       ON payments(expires_at) WHERE expires_at IS NOT NULL;
CREATE INDEX idx_payments_ip_address       ON payments USING HASH(ip_address) WHERE ip_address IS NOT NULL;
-- Partial index for pending payments (most frequently queried)
CREATE INDEX idx_payments_pending          ON payments(merchant_id, created_at DESC)
    WHERE status IN ('PENDING', 'PROCESSING');

-- ============================================================
-- TRANSACTIONS (detailed event log per payment)
-- ============================================================
CREATE TABLE transactions (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    payment_id      UUID NOT NULL REFERENCES payments(id),
    type            VARCHAR(20) NOT NULL,
    amount          NUMERIC(20, 6) NOT NULL,
    currency        VARCHAR(3) NOT NULL,
    status          VARCHAR(30) NOT NULL,
    provider        VARCHAR(50) NOT NULL,
    provider_ref_id VARCHAR(255),
    raw_request     JSONB NOT NULL DEFAULT '{}',
    raw_response    JSONB NOT NULL DEFAULT '{}',
    error_code      VARCHAR(100),
    error_message   TEXT,
    processed_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_tx_type CHECK (type IN ('CHARGE','REFUND','CAPTURE','VOID','DISPUTE'))
);

CREATE INDEX idx_transactions_payment_id ON transactions(payment_id);
CREATE INDEX idx_transactions_created_at ON transactions(created_at DESC);

-- ============================================================
-- REFUNDS
-- ============================================================
CREATE TABLE refunds (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    payment_id      UUID NOT NULL REFERENCES payments(id),
    amount          NUMERIC(20, 6) NOT NULL CHECK (amount > 0),
    currency        VARCHAR(3) NOT NULL,
    status          VARCHAR(20) NOT NULL DEFAULT 'PENDING',
    reason          TEXT NOT NULL,
    provider_ref_id VARCHAR(255),
    requested_by    VARCHAR(255) NOT NULL,
    processed_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_refund_status CHECK (status IN ('PENDING','COMPLETED','FAILED'))
);

CREATE INDEX idx_refunds_payment_id ON refunds(payment_id);
CREATE INDEX idx_refunds_status     ON refunds(status);

-- ============================================================
-- CARD TOKENS (PCI-DSS compliant - no raw card data)
-- ============================================================
CREATE TABLE card_tokens (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    merchant_id     UUID NOT NULL REFERENCES merchants(id),
    customer_id     VARCHAR(255) NOT NULL,
    token_hash      VARCHAR(255) NOT NULL UNIQUE,
    last4           VARCHAR(4) NOT NULL,
    brand           VARCHAR(50) NOT NULL,
    expiry_month    SMALLINT NOT NULL CHECK (expiry_month BETWEEN 1 AND 12),
    expiry_year     SMALLINT NOT NULL,
    provider_token  TEXT NOT NULL,
    provider        VARCHAR(50) NOT NULL,
    is_default      BOOLEAN NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_card_tokens_customer   ON card_tokens(merchant_id, customer_id);
CREATE INDEX idx_card_tokens_token_hash ON card_tokens(token_hash);

-- ============================================================
-- AUDIT LOGS (append-only, immutable)
-- ============================================================
CREATE TABLE audit_logs (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    entity_type VARCHAR(50) NOT NULL,
    entity_id   UUID NOT NULL,
    action      VARCHAR(100) NOT NULL,
    actor_id    VARCHAR(255) NOT NULL,
    actor_type  VARCHAR(50) NOT NULL,
    ip_address  INET,
    old_value   JSONB,
    new_value   JSONB,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_audit_entity     ON audit_logs(entity_type, entity_id);
CREATE INDEX idx_audit_created_at ON audit_logs(created_at DESC);
CREATE INDEX idx_audit_actor      ON audit_logs(actor_id);

-- Make audit table immutable (no UPDATE/DELETE)
CREATE RULE audit_no_update AS ON UPDATE TO audit_logs DO INSTEAD NOTHING;
CREATE RULE audit_no_delete AS ON DELETE TO audit_logs DO INSTEAD NOTHING;

-- ============================================================
-- WEBHOOK DELIVERIES
-- ============================================================
CREATE TABLE webhook_deliveries (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    merchant_id     UUID NOT NULL REFERENCES merchants(id),
    payment_id      UUID REFERENCES payments(id),
    event           VARCHAR(100) NOT NULL,
    url             TEXT NOT NULL,
    payload         JSONB NOT NULL,
    response_status INTEGER,
    response_body   TEXT,
    attempt_count   INTEGER NOT NULL DEFAULT 0,
    next_retry_at   TIMESTAMPTZ,
    is_delivered    BOOLEAN NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_webhooks_merchant    ON webhook_deliveries(merchant_id);
CREATE INDEX idx_webhooks_payment     ON webhook_deliveries(payment_id);
CREATE INDEX idx_webhooks_retry       ON webhook_deliveries(next_retry_at)
    WHERE is_delivered = FALSE AND next_retry_at IS NOT NULL;

-- ============================================================
-- RISK RULES
-- ============================================================
CREATE TABLE risk_rules (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name        VARCHAR(255) NOT NULL UNIQUE,
    description TEXT,
    rule_type   VARCHAR(50) NOT NULL,
    conditions  JSONB NOT NULL,
    action      VARCHAR(20) NOT NULL DEFAULT 'SCORE',
    score       INTEGER NOT NULL DEFAULT 0,
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_rule_action CHECK (action IN ('SCORE','BLOCK','REVIEW','ALLOW'))
);

-- ============================================================
-- BLACKLISTS
-- ============================================================
CREATE TABLE blacklist (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    type        VARCHAR(20) NOT NULL,  -- 'IP', 'CARD', 'EMAIL', 'PHONE', 'DEVICE'
    value_hash  VARCHAR(255) NOT NULL,
    reason      TEXT,
    expires_at  TIMESTAMPTZ,
    created_by  VARCHAR(255),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_blacklist UNIQUE(type, value_hash),
    CONSTRAINT chk_blacklist_type CHECK (type IN ('IP','CARD','EMAIL','PHONE','DEVICE'))
);

CREATE INDEX idx_blacklist_lookup ON blacklist(type, value_hash);
CREATE INDEX idx_blacklist_expiry ON blacklist(expires_at) WHERE expires_at IS NOT NULL;

-- ============================================================
-- UPDATE TRIGGER
-- ============================================================
CREATE OR REPLACE FUNCTION update_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER merchants_updated_at BEFORE UPDATE ON merchants
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();
CREATE TRIGGER payments_updated_at BEFORE UPDATE ON payments
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();
CREATE TRIGGER webhook_deliveries_updated_at BEFORE UPDATE ON webhook_deliveries
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

-- ============================================================
-- ROW LEVEL SECURITY (PCI-DSS compliance)
-- ============================================================
ALTER TABLE payments ENABLE ROW LEVEL SECURITY;
ALTER TABLE transactions ENABLE ROW LEVEL SECURITY;
ALTER TABLE refunds ENABLE ROW LEVEL SECURITY;
ALTER TABLE card_tokens ENABLE ROW LEVEL SECURITY;

-- App role policy (only sees its own merchant's data)
CREATE POLICY payment_merchant_isolation ON payments
    USING (merchant_id = current_setting('app.merchant_id', TRUE)::UUID);

-- Superuser bypass
CREATE POLICY payment_superuser ON payments TO postgres USING (TRUE);

-- ============================================================
-- SEED: Default risk rules
-- ============================================================
INSERT INTO risk_rules (name, description, rule_type, conditions, action, score) VALUES
('large_amount', 'Flag large transactions', 'AMOUNT', '{"threshold": 100000}', 'SCORE', 30),
('high_velocity_ip', 'Too many transactions from same IP', 'VELOCITY', '{"max_per_hour": 20, "field": "ip"}', 'SCORE', 40),
('high_velocity_customer', 'Too many transactions from same customer', 'VELOCITY', '{"max_per_hour": 10, "field": "customer_id"}', 'SCORE', 40),
('crypto_high_amount', 'Crypto payment with high amount', 'COMPOSITE', '{"method": "CRYPTO", "amount_threshold": 10000}', 'SCORE', 20),
('blacklisted_ip', 'IP on blacklist', 'BLACKLIST', '{"field": "ip"}', 'BLOCK', 100),
('blacklisted_card', 'Card on blacklist', 'BLACKLIST', '{"field": "card"}', 'BLOCK', 100),
('unusual_hour', 'Transaction outside business hours', 'TIME', '{"hours": [2,3,4,5]}', 'SCORE', 10);
