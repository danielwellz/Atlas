-- +goose Up
CREATE TABLE IF NOT EXISTS readiness_checkins (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    date DATE NOT NULL,
    energy_level INTEGER NOT NULL,
    sleep_quality INTEGER NOT NULL,
    stress_level INTEGER NOT NULL,
    readiness_score DOUBLE PRECISION NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT readiness_checkins_energy_level_range_check CHECK (energy_level BETWEEN 1 AND 3),
    CONSTRAINT readiness_checkins_sleep_quality_range_check CHECK (sleep_quality BETWEEN 1 AND 3),
    CONSTRAINT readiness_checkins_stress_level_range_check CHECK (stress_level BETWEEN 1 AND 3),
    CONSTRAINT readiness_checkins_readiness_score_range_check CHECK (readiness_score >= 0),
    PRIMARY KEY (user_id, date)
);

CREATE INDEX IF NOT EXISTS idx_readiness_checkins_user_id_date
    ON readiness_checkins (user_id, date DESC);

-- +goose Down
DROP TABLE IF EXISTS readiness_checkins;
