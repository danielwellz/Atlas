-- +goose Up
CREATE TABLE IF NOT EXISTS foods (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    external_id TEXT NOT NULL,
    provider TEXT NOT NULL,
    label TEXT NOT NULL,
    brand TEXT NOT NULL DEFAULT '',
    nutrients_json JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT foods_provider_external_id_unique UNIQUE (provider, external_id)
);

CREATE TABLE IF NOT EXISTS food_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    datetime TIMESTAMPTZ NOT NULL,
    food_id UUID NOT NULL REFERENCES foods(id) ON DELETE RESTRICT,
    quantity DOUBLE PRECISION NOT NULL,
    unit TEXT NOT NULL,
    nutrients_snapshot_json JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT food_logs_quantity_check CHECK (quantity > 0)
);

CREATE INDEX IF NOT EXISTS idx_food_logs_user_datetime ON food_logs (user_id, datetime DESC);
CREATE INDEX IF NOT EXISTS idx_food_logs_food_id ON food_logs (food_id);

-- +goose Down
DROP TABLE IF EXISTS food_logs;
DROP TABLE IF EXISTS foods;
