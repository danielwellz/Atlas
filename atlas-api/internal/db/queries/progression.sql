-- name: UpsertUserExerciseProgress :exec
INSERT INTO user_exercise_progress (
    user_id,
    exercise_id,
    last_load,
    last_reps,
    last_rpe
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5
)
ON CONFLICT (user_id, exercise_id)
DO UPDATE SET
    last_load = EXCLUDED.last_load,
    last_reps = EXCLUDED.last_reps,
    last_rpe = EXCLUDED.last_rpe,
    updated_at = NOW();

-- name: GetUserExerciseProgressByUserIDAndExerciseID :one
SELECT user_id, exercise_id, last_load, last_reps, last_rpe, updated_at
FROM user_exercise_progress
WHERE user_id = $1
  AND exercise_id = $2
LIMIT 1;

-- name: UpsertUserProgramState :exec
INSERT INTO user_program_state (
    user_id,
    program_id,
    current_week,
    deload_flag,
    last_week_adherence,
    last_week_scheduled_sessions,
    last_week_completed_sessions,
    last_week_density,
    last_week_high_rpe_rate,
    fatigue_score,
    consecutive_low_adherence_weeks,
    adjustment_reasons_json
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7,
    $8,
    $9,
    $10,
    $11,
    $12
)
ON CONFLICT (user_id, program_id)
DO UPDATE SET
    current_week = EXCLUDED.current_week,
    deload_flag = EXCLUDED.deload_flag,
    last_week_adherence = EXCLUDED.last_week_adherence,
    last_week_scheduled_sessions = EXCLUDED.last_week_scheduled_sessions,
    last_week_completed_sessions = EXCLUDED.last_week_completed_sessions,
    last_week_density = EXCLUDED.last_week_density,
    last_week_high_rpe_rate = EXCLUDED.last_week_high_rpe_rate,
    fatigue_score = EXCLUDED.fatigue_score,
    consecutive_low_adherence_weeks = EXCLUDED.consecutive_low_adherence_weeks,
    adjustment_reasons_json = EXCLUDED.adjustment_reasons_json,
    updated_at = NOW();

-- name: GetUserProgramStateByUserIDAndProgramID :one
SELECT
    user_id,
    program_id,
    current_week,
    deload_flag,
    last_week_adherence,
    last_week_scheduled_sessions,
    last_week_completed_sessions,
    last_week_density,
    last_week_high_rpe_rate,
    fatigue_score,
    consecutive_low_adherence_weeks,
    adjustment_reasons_json,
    updated_at
FROM user_program_state
WHERE user_id = $1
  AND program_id = $2
LIMIT 1;
