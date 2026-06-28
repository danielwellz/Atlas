-- +goose Up
CREATE TABLE IF NOT EXISTS user_exercise_progress (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    exercise_id UUID NOT NULL REFERENCES exercises(id) ON DELETE CASCADE,
    last_load DOUBLE PRECISION NOT NULL,
    last_reps INTEGER NOT NULL,
    last_rpe DOUBLE PRECISION,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT user_exercise_progress_pk PRIMARY KEY (user_id, exercise_id),
    CONSTRAINT user_exercise_progress_last_load_check CHECK (last_load >= 0),
    CONSTRAINT user_exercise_progress_last_reps_check CHECK (last_reps > 0),
    CONSTRAINT user_exercise_progress_last_rpe_check CHECK (last_rpe IS NULL OR (last_rpe >= 0 AND last_rpe <= 10))
);

CREATE INDEX IF NOT EXISTS idx_user_exercise_progress_user_id ON user_exercise_progress (user_id);

CREATE TABLE IF NOT EXISTS user_program_state (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    program_id UUID NOT NULL REFERENCES programs(id) ON DELETE CASCADE,
    current_week INTEGER NOT NULL DEFAULT 1,
    deload_flag BOOLEAN NOT NULL DEFAULT FALSE,
    last_week_adherence DOUBLE PRECISION NOT NULL DEFAULT 1,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT user_program_state_pk PRIMARY KEY (user_id, program_id),
    CONSTRAINT user_program_state_current_week_check CHECK (current_week > 0),
    CONSTRAINT user_program_state_adherence_check CHECK (last_week_adherence >= 0 AND last_week_adherence <= 1)
);

CREATE INDEX IF NOT EXISTS idx_user_program_state_user_id ON user_program_state (user_id);

-- +goose Down
DROP TABLE IF EXISTS user_program_state;
DROP TABLE IF EXISTS user_exercise_progress;
