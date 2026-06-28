-- +goose Up
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS exercises (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    primary_muscle_group TEXT NOT NULL,
    secondary_muscles_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    movement_pattern TEXT NOT NULL,
    equipment_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    difficulty TEXT NOT NULL,
    description TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT exercises_difficulty_check CHECK (difficulty IN ('beginner', 'intermediate', 'advanced')),
    CONSTRAINT exercises_secondary_muscles_array_check CHECK (jsonb_typeof(secondary_muscles_json) = 'array'),
    CONSTRAINT exercises_equipment_array_check CHECK (jsonb_typeof(equipment_json) = 'array')
);

CREATE INDEX IF NOT EXISTS idx_exercises_movement_pattern ON exercises (movement_pattern);
CREATE INDEX IF NOT EXISTS idx_exercises_primary_muscle_group ON exercises (primary_muscle_group);
CREATE INDEX IF NOT EXISTS idx_exercises_equipment_json_gin ON exercises USING GIN (equipment_json);

CREATE TABLE IF NOT EXISTS exercise_media (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    exercise_id UUID NOT NULL REFERENCES exercises(id) ON DELETE CASCADE,
    media_type TEXT NOT NULL,
    uri TEXT NOT NULL,
    thumbnail_uri TEXT,
    duration_seconds INTEGER,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT exercise_media_media_type_check CHECK (media_type IN ('image', 'video')),
    CONSTRAINT exercise_media_duration_check CHECK (duration_seconds IS NULL OR duration_seconds >= 0)
);

CREATE INDEX IF NOT EXISTS idx_exercise_media_exercise_id ON exercise_media (exercise_id);
CREATE INDEX IF NOT EXISTS idx_exercise_media_media_type ON exercise_media (media_type);

-- +goose Down
DROP TABLE IF EXISTS exercise_media;
DROP TABLE IF EXISTS exercises;
