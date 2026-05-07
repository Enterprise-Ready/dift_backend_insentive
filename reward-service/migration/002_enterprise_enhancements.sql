-- ============================================================
-- Migration: Enterprise Reward Service Enhancements
-- ============================================================

-- ── Outbox table (transactional messaging) ───────────────────
CREATE TABLE IF NOT EXISTS reward_outbox (
    id           BIGSERIAL    PRIMARY KEY,
    subject      TEXT         NOT NULL,
    payload      JSONB        NOT NULL,
    status       TEXT         NOT NULL DEFAULT 'pending',  -- pending | sent | dead
    attempts     INT          NOT NULL DEFAULT 0,
    error        TEXT,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    processed_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_reward_outbox_pending
    ON reward_outbox(status, attempts, created_at)
    WHERE status = 'pending';

-- ── Earn transactions: add missing columns + index ────────────
ALTER TABLE reward_earn_transactions
    ADD COLUMN IF NOT EXISTS idempotency_status TEXT DEFAULT 'processed';

CREATE UNIQUE INDEX IF NOT EXISTS idx_earn_ref_id
    ON reward_earn_transactions(ref_id);

CREATE INDEX IF NOT EXISTS idx_earn_user_id_created
    ON reward_earn_transactions(user_id, created_at DESC);

-- ── Redeem requests: add result columns ──────────────────────
ALTER TABLE reward_redeem_requests
    ADD COLUMN IF NOT EXISTS success      BOOLEAN,
    ADD COLUMN IF NOT EXISTS reason       TEXT,
    ADD COLUMN IF NOT EXISTS processed_at BIGINT;

CREATE INDEX IF NOT EXISTS idx_redeem_user_id
    ON reward_redeem_requests(user_id);

CREATE INDEX IF NOT EXISTS idx_redeem_status
    ON reward_redeem_requests(success)
    WHERE success IS NOT NULL;

-- ── Dead letter queue log ─────────────────────────────────────
CREATE TABLE IF NOT EXISTS reward_dlq_log (
    id               BIGSERIAL    PRIMARY KEY,
    original_subject TEXT         NOT NULL,
    payload          TEXT         NOT NULL,
    error_message    TEXT         NOT NULL,
    failed_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_dlq_failed_at
    ON reward_dlq_log(failed_at DESC);

-- ── Row-level security (RLS) example ─────────────────────────
-- Uncomment if using per-tenant isolation
-- ALTER TABLE reward_earn_transactions ENABLE ROW LEVEL SECURITY;
-- CREATE POLICY earn_user_isolation ON reward_earn_transactions
--     USING (user_id = current_setting('app.current_user_id'));
