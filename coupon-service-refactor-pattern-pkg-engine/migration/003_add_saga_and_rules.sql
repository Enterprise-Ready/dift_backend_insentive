-- =============================================
-- Migration: Add Saga Orchestrator + Rule Engine Tables
-- =============================================

-- ─────────────────────────────────────────────
-- Saga instances
-- Persists the entire saga lifecycle.
-- Status transitions: STARTED → CLAIMING → RESERVING → CONFIRMING → COMPLETED
--                                                     ↘ COMPENSATING → COMPENSATED
-- ─────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS saga_instances (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id          TEXT        NOT NULL,
    coupon_code      TEXT        NOT NULL,
    order_id         TEXT,
    idempotency_key  TEXT        NOT NULL UNIQUE,  -- prevents duplicate saga starts
    status           TEXT        NOT NULL DEFAULT 'STARTED',
    current_step     TEXT,
    failure_reason   TEXT,
    payload          JSONB       NOT NULL DEFAULT '{}',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at     TIMESTAMPTZ
);

-- Index for recovery worker: finds stale in-progress sagas
CREATE INDEX IF NOT EXISTS idx_saga_status_updated
    ON saga_instances (status, updated_at)
    WHERE status IN ('CLAIMING', 'RESERVING', 'CONFIRMING', 'COMPENSATING');

-- Index for user + coupon lookups
CREATE INDEX IF NOT EXISTS idx_saga_user_coupon
    ON saga_instances (user_id, coupon_code);

-- ─────────────────────────────────────────────
-- Saga step logs
-- Append-only audit log of every step execution.
-- ─────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS saga_step_logs (
    id          BIGSERIAL   PRIMARY KEY,
    saga_id     UUID        NOT NULL REFERENCES saga_instances(id),
    step_name   TEXT        NOT NULL,
    status      TEXT        NOT NULL,   -- PENDING | SUCCEEDED | FAILED | COMPENSATED
    attempt     INT         NOT NULL DEFAULT 1,
    error       TEXT,
    executed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_saga_step_logs_saga_id
    ON saga_step_logs (saga_id);

-- ─────────────────────────────────────────────
-- Coupon rules (Intelligence Engine)
-- Stores the rule tree per coupon code.
-- condition_group is a JSON tree of ConditionGroup.
-- ─────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS coupon_rules (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    coupon_code      TEXT        NOT NULL REFERENCES coupons(code) ON DELETE CASCADE,
    priority         INT         NOT NULL DEFAULT 0,
    stack_group      TEXT        NOT NULL DEFAULT '',
    stack_behavior   TEXT        NOT NULL DEFAULT 'allow',  -- allow | restrict | exclusive
    condition_group  JSONB       NOT NULL DEFAULT '{"operator":"AND","conditions":[]}',
    active           BOOLEAN     NOT NULL DEFAULT TRUE,
    valid_from       TIMESTAMPTZ NOT NULL,
    valid_to         TIMESTAMPTZ NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Enforce one active rule per coupon (partial unique index)
CREATE UNIQUE INDEX IF NOT EXISTS uidx_coupon_rules_active_code
    ON coupon_rules (coupon_code)
    WHERE active = TRUE;

CREATE INDEX IF NOT EXISTS idx_coupon_rules_stack_group
    ON coupon_rules (stack_group)
    WHERE active = TRUE;

-- ─────────────────────────────────────────────
-- updated_at auto-update trigger (reusable)
-- ─────────────────────────────────────────────
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_saga_instances_updated_at
    BEFORE UPDATE ON saga_instances
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_coupon_rules_updated_at
    BEFORE UPDATE ON coupon_rules
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ─────────────────────────────────────────────
-- Example: Insert a rule for coupon "VIP20"
-- Applies only if: order_total >= 500 AND user_segment IN ["vip","gold"]
-- ─────────────────────────────────────────────
-- INSERT INTO coupon_rules (coupon_code, priority, stack_group, stack_behavior, condition_group, valid_from, valid_to)
-- VALUES (
--   'VIP20',
--   100,
--   'vip-group',
--   'restrict',
--   '{
--     "operator": "AND",
--     "conditions": [
--       {"field": "order_total",  "operator": "gte", "value": 500},
--       {"field": "user_segment", "operator": "in",  "value": ["vip","gold"]}
--     ]
--   }',
--   NOW(),
--   NOW() + INTERVAL '90 days'
-- );
