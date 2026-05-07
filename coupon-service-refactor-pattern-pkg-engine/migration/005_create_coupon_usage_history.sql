CREATE TABLE coupon_usage_history (
    id BIGSERIAL PRIMARY KEY,
    coupon_code VARCHAR(100) NOT NULL,
    user_id VARCHAR(100) NOT NULL,
    order_id VARCHAR(100),
    used_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_usage_coupon_code
ON coupon_usage_history (coupon_code);

CREATE INDEX idx_usage_user
ON coupon_usage_history (user_id);