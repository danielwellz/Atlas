-- +goose Up
CREATE TABLE IF NOT EXISTS nutrition_targets (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    calories_target INTEGER NOT NULL,
    protein_g_target INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT nutrition_targets_calories_target_check CHECK (calories_target > 0),
    CONSTRAINT nutrition_targets_protein_target_check CHECK (protein_g_target > 0)
);

CREATE TABLE IF NOT EXISTS nutrition_daily_checkins (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    date DATE NOT NULL,
    calories_estimate INTEGER,
    protein_g_estimate INTEGER,
    hit_calories BOOLEAN NOT NULL,
    hit_protein BOOLEAN NOT NULL,
    notes TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT nutrition_daily_checkins_user_id_date_unique UNIQUE (user_id, date),
    CONSTRAINT nutrition_daily_checkins_calories_estimate_check CHECK (calories_estimate IS NULL OR calories_estimate >= 0),
    CONSTRAINT nutrition_daily_checkins_protein_estimate_check CHECK (protein_g_estimate IS NULL OR protein_g_estimate >= 0)
);

CREATE INDEX IF NOT EXISTS idx_nutrition_daily_checkins_user_id_date ON nutrition_daily_checkins (user_id, date DESC);

-- +goose Down
DROP TABLE IF EXISTS nutrition_daily_checkins;
DROP TABLE IF EXISTS nutrition_targets;
