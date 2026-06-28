-- +goose Up
CREATE TABLE IF NOT EXISTS workouts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    program_session_id UUID REFERENCES program_sessions(id) ON DELETE SET NULL,
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    notes TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT workouts_completed_after_started_check CHECK (
        completed_at IS NULL OR completed_at >= started_at
    )
);

CREATE TABLE IF NOT EXISTS workout_exercises (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workout_id UUID NOT NULL REFERENCES workouts(id) ON DELETE CASCADE,
    exercise_id UUID NOT NULL REFERENCES exercises(id),
    order_index INTEGER NOT NULL,
    planned_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    actual_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT workout_exercises_order_index_check CHECK (order_index > 0),
    CONSTRAINT workout_exercises_planned_object_check CHECK (jsonb_typeof(planned_json) = 'object'),
    CONSTRAINT workout_exercises_actual_object_check CHECK (jsonb_typeof(actual_json) = 'object'),
    CONSTRAINT workout_exercises_workout_id_order_index_unique UNIQUE (workout_id, order_index)
);

CREATE TABLE IF NOT EXISTS workout_sets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workout_exercise_id UUID NOT NULL REFERENCES workout_exercises(id) ON DELETE CASCADE,
    set_index INTEGER NOT NULL,
    reps INTEGER NOT NULL,
    weight_kg DOUBLE PRECISION NOT NULL,
    rpe DOUBLE PRECISION,
    completed_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT workout_sets_set_index_check CHECK (set_index > 0),
    CONSTRAINT workout_sets_reps_check CHECK (reps > 0),
    CONSTRAINT workout_sets_weight_kg_check CHECK (weight_kg >= 0),
    CONSTRAINT workout_sets_rpe_check CHECK (rpe IS NULL OR (rpe >= 0 AND rpe <= 10)),
    CONSTRAINT workout_sets_workout_exercise_id_set_index_unique UNIQUE (workout_exercise_id, set_index)
);

CREATE INDEX IF NOT EXISTS idx_workouts_user_started_at ON workouts (user_id, started_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_workouts_program_session_id ON workouts (program_session_id);
CREATE INDEX IF NOT EXISTS idx_workout_exercises_workout_id ON workout_exercises (workout_id);
CREATE INDEX IF NOT EXISTS idx_workout_sets_workout_exercise_id ON workout_sets (workout_exercise_id);

-- +goose Down
DROP TABLE IF EXISTS workout_sets;
DROP TABLE IF EXISTS workout_exercises;
DROP TABLE IF EXISTS workouts;
