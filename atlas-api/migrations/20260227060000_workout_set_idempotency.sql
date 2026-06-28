-- +goose Up
ALTER TABLE workout_sets
ADD COLUMN IF NOT EXISTS idempotency_key TEXT;

CREATE UNIQUE INDEX IF NOT EXISTS idx_workout_sets_exercise_idempotency_key
ON workout_sets (workout_exercise_id, idempotency_key)
WHERE idempotency_key IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_workout_sets_exercise_idempotency_key;

ALTER TABLE workout_sets
DROP COLUMN IF EXISTS idempotency_key;
