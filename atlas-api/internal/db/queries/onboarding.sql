-- name: GetOnboardingStatus :one
SELECT
    EXISTS(SELECT 1 FROM user_profiles up WHERE up.user_id = $1) AS has_profile,
    EXISTS(SELECT 1 FROM user_goals ug WHERE ug.user_id = $1) AS has_goals;

-- name: GetUserProfileByUserID :one
SELECT user_id, display_name, sex, height_cm, weight_kg, experience_level, created_at, updated_at
FROM user_profiles
WHERE user_id = $1
LIMIT 1;

-- name: UpsertUserProfile :one
INSERT INTO user_profiles (
    user_id,
    display_name,
    sex,
    height_cm,
    weight_kg,
    experience_level
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6
)
ON CONFLICT (user_id)
DO UPDATE SET
    display_name = EXCLUDED.display_name,
    sex = EXCLUDED.sex,
    height_cm = EXCLUDED.height_cm,
    weight_kg = EXCLUDED.weight_kg,
    experience_level = EXCLUDED.experience_level,
    updated_at = NOW()
RETURNING user_id, display_name, sex, height_cm, weight_kg, experience_level, created_at, updated_at;

-- name: GetUserGoalsByUserID :one
SELECT user_id, primary_goal, secondary_goal, days_per_week, session_duration_minutes, equipment_access_json, constraints_json, injuries_limitations_json, modality_preferences_json, prior_training_history_json, readiness_signals_json, created_at, updated_at
FROM user_goals
WHERE user_id = $1
LIMIT 1;

-- name: UpsertUserGoals :one
INSERT INTO user_goals (
    user_id,
    primary_goal,
    secondary_goal,
    days_per_week,
    session_duration_minutes,
    equipment_access_json,
    constraints_json,
    injuries_limitations_json,
    modality_preferences_json,
    prior_training_history_json,
    readiness_signals_json
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
    $11
)
ON CONFLICT (user_id)
DO UPDATE SET
    primary_goal = EXCLUDED.primary_goal,
    secondary_goal = EXCLUDED.secondary_goal,
    days_per_week = EXCLUDED.days_per_week,
    session_duration_minutes = EXCLUDED.session_duration_minutes,
    equipment_access_json = EXCLUDED.equipment_access_json,
    constraints_json = EXCLUDED.constraints_json,
    injuries_limitations_json = EXCLUDED.injuries_limitations_json,
    modality_preferences_json = EXCLUDED.modality_preferences_json,
    prior_training_history_json = EXCLUDED.prior_training_history_json,
    readiness_signals_json = EXCLUDED.readiness_signals_json,
    updated_at = NOW()
RETURNING user_id, primary_goal, secondary_goal, days_per_week, session_duration_minutes, equipment_access_json, constraints_json, injuries_limitations_json, modality_preferences_json, prior_training_history_json, readiness_signals_json, created_at, updated_at;
