-- ============================================
-- Table: reward_redeem_requests
-- Description: เก็บคำขอแลกแต้ม
-- ============================================

CREATE TABLE IF NOT EXISTS reward_redeem_requests (
    redeem_id TEXT PRIMARY KEY,          -- idempotency key
    user_id TEXT NOT NULL,
    point BIGINT NOT NULL CHECK (point > 0),

    requested_at TIMESTAMP NOT NULL DEFAULT NOW(),

    success BOOLEAN,
    reason TEXT,
    processed_at TIMESTAMP
);

-- ใช้ query ตาม user บ่อย
CREATE INDEX IF NOT EXISTS idx_reward_redeem_user_id
ON reward_redeem_requests(user_id);

-- ใช้ดูสถานะที่ยังไม่ processed
CREATE INDEX IF NOT EXISTS idx_reward_redeem_processed_at
ON reward_redeem_requests(processed_at);
