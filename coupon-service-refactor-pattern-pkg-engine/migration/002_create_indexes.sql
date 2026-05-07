-- =====================================================
-- 002_create_indexes.sql
-- Performance Indexes
-- =====================================================

-- Claim lookup (critical path)
CREATE INDEX IF NOT EXISTS idx_coupons_code
ON coupons (code);

-- Personal coupon lookup
CREATE INDEX IF NOT EXISTS idx_coupons_user_code
ON coupons (user_id, code);

-- Active filtering
CREATE INDEX IF NOT EXISTS idx_coupons_active
ON coupons (active);

-- Valid time filtering
CREATE INDEX IF NOT EXISTS idx_coupons_validity
ON coupons (valid_from, valid_to);

-- Admin dashboard
CREATE INDEX IF NOT EXISTS idx_coupons_created_at
ON coupons (created_at DESC);