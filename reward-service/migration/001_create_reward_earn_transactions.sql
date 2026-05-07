-- ============================================
-- Table: reward_earn_transactions
-- Description: เก็บประวัติการได้แต้ม
-- ============================================

CREATE TABLE IF NOT EXISTS reward_earn_transactions (
    earn_id TEXT PRIMARY KEY,            -- idempotency key
    user_id TEXT NOT NULL,
    ref_id  TEXT NOT NULL,               -- trip_id / order_id
    point   BIGINT NOT NULL CHECK (point > 0),
    source  TEXT NOT NULL,               -- trip | order | campaign
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- ป้องกัน earn ซ้ำจาก ref เดิม
CREATE UNIQUE INDEX IF NOT EXISTS idx_reward_earn_ref_id
ON reward_earn_transactions(ref_id);

-- ใช้ query ตาม user บ่อย
CREATE INDEX IF NOT EXISTS idx_reward_earn_user_id
ON reward_earn_transactions(user_id);

-- เผื่อ query ตาม created_at
CREATE INDEX IF NOT EXISTS idx_reward_earn_created_at
ON reward_earn_transactions(created_at);
