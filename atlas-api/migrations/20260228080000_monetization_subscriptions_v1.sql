-- +goose Up
CREATE TABLE IF NOT EXISTS subscriptions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    platform TEXT NOT NULL,
    product_id TEXT NOT NULL,
    status TEXT NOT NULL,
    expires_at TIMESTAMPTZ,
    raw_receipt JSONB NOT NULL DEFAULT '{}'::jsonb,
    transaction_id TEXT NOT NULL,
    original_transaction_id TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT subscriptions_platform_check CHECK (platform IN ('ios', 'android')),
    CONSTRAINT subscriptions_status_check CHECK (status IN ('active', 'expired', 'canceled', 'grace_period', 'refunded')),
    CONSTRAINT subscriptions_user_platform_transaction_unique UNIQUE (user_id, platform, transaction_id)
);

CREATE INDEX IF NOT EXISTS idx_subscriptions_user_id
    ON subscriptions (user_id);

CREATE INDEX IF NOT EXISTS idx_subscriptions_user_status_expires
    ON subscriptions (user_id, status, expires_at);

CREATE TABLE IF NOT EXISTS subscription_product_entitlements (
    product_id TEXT NOT NULL,
    entitlement TEXT NOT NULL,
    PRIMARY KEY (product_id, entitlement),
    CONSTRAINT subscription_product_entitlements_entitlement_check CHECK (
        entitlement IN (
            'barcode_scan',
            'deep_nutrition',
            'biomechanics_overlays',
            'form_check_upload',
            'coach_tier_pro',
            'coach_tier_elite'
        )
    )
);

INSERT INTO subscription_product_entitlements (product_id, entitlement) VALUES
    ('atlas.pro.monthly', 'barcode_scan'),
    ('atlas.pro.monthly', 'deep_nutrition'),
    ('atlas.pro.monthly', 'biomechanics_overlays'),
    ('atlas.pro.monthly', 'form_check_upload'),
    ('atlas.pro.monthly', 'coach_tier_pro'),
    ('atlas.pro.yearly', 'barcode_scan'),
    ('atlas.pro.yearly', 'deep_nutrition'),
    ('atlas.pro.yearly', 'biomechanics_overlays'),
    ('atlas.pro.yearly', 'form_check_upload'),
    ('atlas.pro.yearly', 'coach_tier_pro'),
    ('atlas.elite.monthly', 'barcode_scan'),
    ('atlas.elite.monthly', 'deep_nutrition'),
    ('atlas.elite.monthly', 'biomechanics_overlays'),
    ('atlas.elite.monthly', 'form_check_upload'),
    ('atlas.elite.monthly', 'coach_tier_pro'),
    ('atlas.elite.monthly', 'coach_tier_elite'),
    ('atlas.elite.yearly', 'barcode_scan'),
    ('atlas.elite.yearly', 'deep_nutrition'),
    ('atlas.elite.yearly', 'biomechanics_overlays'),
    ('atlas.elite.yearly', 'form_check_upload'),
    ('atlas.elite.yearly', 'coach_tier_pro'),
    ('atlas.elite.yearly', 'coach_tier_elite')
ON CONFLICT DO NOTHING;

ALTER TABLE coach_sessions
    ADD COLUMN IF NOT EXISTS required_tier TEXT NOT NULL DEFAULT 'free';

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'coach_sessions_required_tier_check'
    ) THEN
        ALTER TABLE coach_sessions
            ADD CONSTRAINT coach_sessions_required_tier_check
            CHECK (required_tier IN ('free', 'pro', 'elite'));
    END IF;
END
$$;

CREATE OR REPLACE VIEW user_entitlements AS
SELECT
    s.user_id,
    spe.entitlement,
    MAX(s.expires_at) AS expires_at
FROM subscriptions s
JOIN subscription_product_entitlements spe ON spe.product_id = s.product_id
WHERE s.status IN ('active', 'grace_period')
  AND (s.expires_at IS NULL OR s.expires_at > NOW())
GROUP BY s.user_id, spe.entitlement;

-- +goose Down
DROP VIEW IF EXISTS user_entitlements;

ALTER TABLE coach_sessions
    DROP CONSTRAINT IF EXISTS coach_sessions_required_tier_check;

ALTER TABLE coach_sessions
    DROP COLUMN IF EXISTS required_tier;

DROP TABLE IF EXISTS subscription_product_entitlements;
DROP TABLE IF EXISTS subscriptions;
