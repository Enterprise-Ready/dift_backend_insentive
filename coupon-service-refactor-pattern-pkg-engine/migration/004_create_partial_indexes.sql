-- =====================================================
-- 004_create_partial_indexes.sql
-- Advanced Performance (Enterprise Optimization)
-- =====================================================

-- Fast lookup only for active coupons
CREATE INDEX IF NOT EXISTS idx_coupons_active_only
ON coupons (code)
WHERE active = TRUE;

-- Fast lookup only valid time window
CREATE INDEX IF NOT EXISTS idx_coupons_valid_active
ON coupons (code, valid_from, valid_to)
WHERE active = TRUE;