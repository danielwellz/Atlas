-- +goose Up
CREATE TABLE IF NOT EXISTS user_weight_entries (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    date DATE NOT NULL,
    weight_kg DOUBLE PRECISION NOT NULL,
    unit TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT user_weight_entries_primary_key PRIMARY KEY (user_id, date),
    CONSTRAINT user_weight_entries_weight_kg_check CHECK (weight_kg > 0),
    CONSTRAINT user_weight_entries_unit_check CHECK (unit IN ('kg', 'lb'))
);

CREATE INDEX IF NOT EXISTS idx_user_weight_entries_user_id_date_desc
    ON user_weight_entries (user_id, date DESC);

-- +goose Down
DROP TABLE IF EXISTS user_weight_entries;
