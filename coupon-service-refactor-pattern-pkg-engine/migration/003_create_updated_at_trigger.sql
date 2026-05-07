-- =====================================================
-- 003_create_updated_at_trigger.sql
-- Auto update updated_at column
-- =====================================================

CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_set_updated_at ON coupons;

CREATE TRIGGER trg_set_updated_at
BEFORE UPDATE ON coupons
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();