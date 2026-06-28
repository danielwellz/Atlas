-- +goose Up
ALTER TABLE user_program_state
    ADD COLUMN IF NOT EXISTS last_week_scheduled_sessions INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS last_week_completed_sessions INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS last_week_density DOUBLE PRECISION NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS last_week_high_rpe_rate DOUBLE PRECISION NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS fatigue_score DOUBLE PRECISION NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS consecutive_low_adherence_weeks INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS adjustment_reasons_json JSONB NOT NULL DEFAULT '[]'::jsonb;

ALTER TABLE user_program_state
    DROP CONSTRAINT IF EXISTS user_program_state_last_week_scheduled_sessions_check,
    DROP CONSTRAINT IF EXISTS user_program_state_last_week_completed_sessions_check,
    DROP CONSTRAINT IF EXISTS user_program_state_last_week_density_check,
    DROP CONSTRAINT IF EXISTS user_program_state_last_week_high_rpe_rate_check,
    DROP CONSTRAINT IF EXISTS user_program_state_fatigue_score_check,
    DROP CONSTRAINT IF EXISTS user_program_state_consecutive_low_adherence_weeks_check,
    ADD CONSTRAINT user_program_state_last_week_scheduled_sessions_check
        CHECK (last_week_scheduled_sessions >= 0),
    ADD CONSTRAINT user_program_state_last_week_completed_sessions_check
        CHECK (last_week_completed_sessions >= 0),
    ADD CONSTRAINT user_program_state_last_week_density_check
        CHECK (last_week_density >= 0),
    ADD CONSTRAINT user_program_state_last_week_high_rpe_rate_check
        CHECK (last_week_high_rpe_rate >= 0 AND last_week_high_rpe_rate <= 1),
    ADD CONSTRAINT user_program_state_fatigue_score_check
        CHECK (fatigue_score >= 0 AND fatigue_score <= 1),
    ADD CONSTRAINT user_program_state_consecutive_low_adherence_weeks_check
        CHECK (consecutive_low_adherence_weeks >= 0);

-- +goose Down
ALTER TABLE user_program_state
    DROP CONSTRAINT IF EXISTS user_program_state_consecutive_low_adherence_weeks_check,
    DROP CONSTRAINT IF EXISTS user_program_state_fatigue_score_check,
    DROP CONSTRAINT IF EXISTS user_program_state_last_week_high_rpe_rate_check,
    DROP CONSTRAINT IF EXISTS user_program_state_last_week_density_check,
    DROP CONSTRAINT IF EXISTS user_program_state_last_week_completed_sessions_check,
    DROP CONSTRAINT IF EXISTS user_program_state_last_week_scheduled_sessions_check;

ALTER TABLE user_program_state
    DROP COLUMN IF EXISTS adjustment_reasons_json,
    DROP COLUMN IF EXISTS consecutive_low_adherence_weeks,
    DROP COLUMN IF EXISTS fatigue_score,
    DROP COLUMN IF EXISTS last_week_high_rpe_rate,
    DROP COLUMN IF EXISTS last_week_density,
    DROP COLUMN IF EXISTS last_week_completed_sessions,
    DROP COLUMN IF EXISTS last_week_scheduled_sessions;
