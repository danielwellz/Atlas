-- +goose Up
ALTER TABLE user_goals
    ADD COLUMN IF NOT EXISTS injuries_limitations_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN IF NOT EXISTS modality_preferences_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN IF NOT EXISTS prior_training_history_json JSONB,
    ADD COLUMN IF NOT EXISTS readiness_signals_json JSONB;

ALTER TABLE user_goals
    DROP CONSTRAINT IF EXISTS user_goals_injuries_limitations_array_check,
    DROP CONSTRAINT IF EXISTS user_goals_modality_preferences_array_check,
    DROP CONSTRAINT IF EXISTS user_goals_prior_training_history_object_check,
    DROP CONSTRAINT IF EXISTS user_goals_readiness_signals_object_check;

ALTER TABLE user_goals
    ADD CONSTRAINT user_goals_injuries_limitations_array_check
        CHECK (jsonb_typeof(injuries_limitations_json) = 'array'),
    ADD CONSTRAINT user_goals_modality_preferences_array_check
        CHECK (jsonb_typeof(modality_preferences_json) = 'array'),
    ADD CONSTRAINT user_goals_prior_training_history_object_check
        CHECK (prior_training_history_json IS NULL OR jsonb_typeof(prior_training_history_json) = 'object'),
    ADD CONSTRAINT user_goals_readiness_signals_object_check
        CHECK (readiness_signals_json IS NULL OR jsonb_typeof(readiness_signals_json) = 'object');

-- +goose Down
ALTER TABLE user_goals
    DROP CONSTRAINT IF EXISTS user_goals_injuries_limitations_array_check,
    DROP CONSTRAINT IF EXISTS user_goals_modality_preferences_array_check,
    DROP CONSTRAINT IF EXISTS user_goals_prior_training_history_object_check,
    DROP CONSTRAINT IF EXISTS user_goals_readiness_signals_object_check;

ALTER TABLE user_goals
    DROP COLUMN IF EXISTS readiness_signals_json,
    DROP COLUMN IF EXISTS prior_training_history_json,
    DROP COLUMN IF EXISTS modality_preferences_json,
    DROP COLUMN IF EXISTS injuries_limitations_json;
