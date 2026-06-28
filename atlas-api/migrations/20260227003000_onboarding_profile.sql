-- +goose Up
CREATE TABLE IF NOT EXISTS user_profiles (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    display_name TEXT NOT NULL,
    sex TEXT NOT NULL,
    height_cm INTEGER NOT NULL,
    weight_kg DOUBLE PRECISION NOT NULL,
    experience_level TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS user_goals (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    primary_goal TEXT NOT NULL,
    secondary_goal TEXT,
    days_per_week INTEGER NOT NULL,
    session_duration_minutes INTEGER NOT NULL,
    equipment_access_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    constraints_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT user_goals_days_per_week_check CHECK (days_per_week BETWEEN 1 AND 6),
    CONSTRAINT user_goals_duration_check CHECK (session_duration_minutes BETWEEN 15 AND 120),
    CONSTRAINT user_goals_equipment_array_check CHECK (jsonb_typeof(equipment_access_json) = 'array'),
    CONSTRAINT user_goals_constraints_object_check CHECK (jsonb_typeof(constraints_json) = 'object')
);

-- +goose Down
DROP TABLE IF EXISTS user_goals;
DROP TABLE IF EXISTS user_profiles;
