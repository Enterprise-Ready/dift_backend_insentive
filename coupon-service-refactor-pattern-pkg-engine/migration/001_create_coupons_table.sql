-- =====================================================
-- 001_create_coupons_table.sql
-- Base table (Enterprise Grade)
-- =====================================================

CREATE TABLE IF NOT EXISTS coupons (
    id BIGSERIAL PRIMARY KEY,

    -- Business Identity
    code VARCHAR(100) NOT NULL,
    user_id VARCHAR(100), -- NULL = global coupon

    -- Discount Definition
    discount_type VARCHAR(20) NOT NULL,
    discount_value NUMERIC(12,2) NOT NULL,

    min_order NUMERIC(12,2) NOT NULL DEFAULT 0,
    max_discount NUMERIC(12,2),

    -- Usage Control
    max_usage INT NOT NULL DEFAULT 1,
    used INT NOT NULL DEFAULT 0,

    -- Validity
    valid_from TIMESTAMPTZ NOT NULL,
    valid_to   TIMESTAMPTZ NOT NULL,

    active BOOLEAN NOT NULL DEFAULT TRUE,

    -- Audit
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- =====================================================
    -- Constraints
    -- =====================================================

    CONSTRAINT uq_coupon_code_user UNIQUE (code, user_id),

    CONSTRAINT chk_discount_type
        CHECK (discount_type IN ('PERCENT', 'FIXED')),

    CONSTRAINT chk_discount_value_positive
        CHECK (discount_value > 0),

    CONSTRAINT chk_min_order_positive
        CHECK (min_order >= 0),

    CONSTRAINT chk_max_usage_positive
        CHECK (max_usage > 0),

    CONSTRAINT chk_used_valid
        CHECK (used >= 0 AND used <= max_usage),

    CONSTRAINT chk_valid_range
        CHECK (valid_from < valid_to)
);