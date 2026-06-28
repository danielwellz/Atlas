-- +goose Up
CREATE TABLE IF NOT EXISTS programs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    description TEXT NOT NULL,
    goal_tags_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    level TEXT NOT NULL,
    weeks_length INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT programs_goal_tags_array_check CHECK (jsonb_typeof(goal_tags_json) = 'array'),
    CONSTRAINT programs_weeks_length_check CHECK (weeks_length > 0)
);

CREATE TABLE IF NOT EXISTS program_weeks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    program_id UUID NOT NULL REFERENCES programs(id) ON DELETE CASCADE,
    week_index INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT program_weeks_week_index_check CHECK (week_index > 0),
    CONSTRAINT program_weeks_program_id_week_index_unique UNIQUE (program_id, week_index)
);

CREATE TABLE IF NOT EXISTS program_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    program_week_id UUID NOT NULL REFERENCES program_weeks(id) ON DELETE CASCADE,
    day_of_week INTEGER NOT NULL,
    name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT program_sessions_day_of_week_check CHECK (day_of_week BETWEEN 1 AND 7),
    CONSTRAINT program_sessions_program_week_id_day_of_week_unique UNIQUE (program_week_id, day_of_week)
);

CREATE TABLE IF NOT EXISTS program_session_exercises (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    program_session_id UUID NOT NULL REFERENCES program_sessions(id) ON DELETE CASCADE,
    exercise_id UUID NOT NULL REFERENCES exercises(id),
    prescription_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    order_index INTEGER NOT NULL,
    CONSTRAINT program_session_exercises_order_index_check CHECK (order_index > 0),
    CONSTRAINT program_session_exercises_prescription_object_check CHECK (jsonb_typeof(prescription_json) = 'object'),
    CONSTRAINT program_session_exercises_prescription_required_keys_check CHECK (
        prescription_json ? 'sets' AND prescription_json ? 'reps_range' AND prescription_json ? 'rest_seconds'
    ),
    CONSTRAINT program_session_exercises_program_session_id_order_index_unique UNIQUE (program_session_id, order_index)
);

CREATE TABLE IF NOT EXISTS user_program_enrollments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    program_id UUID NOT NULL REFERENCES programs(id),
    start_date DATE NOT NULL,
    current_week INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT user_program_enrollments_current_week_check CHECK (current_week > 0),
    CONSTRAINT user_program_enrollments_user_id_unique UNIQUE (user_id)
);

CREATE INDEX IF NOT EXISTS idx_program_weeks_program_id ON program_weeks (program_id);
CREATE INDEX IF NOT EXISTS idx_program_sessions_program_week_id ON program_sessions (program_week_id);
CREATE INDEX IF NOT EXISTS idx_program_session_exercises_program_session_id ON program_session_exercises (program_session_id);
CREATE INDEX IF NOT EXISTS idx_user_program_enrollments_user_id ON user_program_enrollments (user_id);

-- +goose Down
DROP TABLE IF EXISTS user_program_enrollments;
DROP TABLE IF EXISTS program_session_exercises;
DROP TABLE IF EXISTS program_sessions;
DROP TABLE IF EXISTS program_weeks;
DROP TABLE IF EXISTS programs;
