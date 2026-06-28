-- name: UpsertNutritionTargets :one
INSERT INTO nutrition_targets (
    user_id,
    calories_target,
    protein_g_target
) VALUES (
    $1,
    $2,
    $3
)
ON CONFLICT (user_id)
DO UPDATE SET
    calories_target = EXCLUDED.calories_target,
    protein_g_target = EXCLUDED.protein_g_target,
    updated_at = NOW()
RETURNING user_id, calories_target, protein_g_target, created_at, updated_at;

-- name: GetNutritionTargetsByUserID :one
SELECT user_id, calories_target, protein_g_target, created_at, updated_at
FROM nutrition_targets
WHERE user_id = $1
LIMIT 1;

-- name: UpsertNutritionDailyCheckin :one
INSERT INTO nutrition_daily_checkins (
    user_id,
    date,
    calories_estimate,
    protein_g_estimate,
    hit_calories,
    hit_protein,
    notes
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7
)
ON CONFLICT (user_id, date)
DO UPDATE SET
    calories_estimate = EXCLUDED.calories_estimate,
    protein_g_estimate = EXCLUDED.protein_g_estimate,
    hit_calories = EXCLUDED.hit_calories,
    hit_protein = EXCLUDED.hit_protein,
    notes = EXCLUDED.notes
RETURNING id, user_id, date, calories_estimate, protein_g_estimate, hit_calories, hit_protein, notes, created_at;

-- name: GetNutritionDailyCheckinByUserIDAndDate :one
SELECT id, user_id, date, calories_estimate, protein_g_estimate, hit_calories, hit_protein, notes, created_at
FROM nutrition_daily_checkins
WHERE user_id = $1
  AND date = $2
LIMIT 1;

-- name: UpsertUserWeightEntry :one
INSERT INTO user_weight_entries (
    user_id,
    date,
    weight_kg,
    unit
) VALUES (
    $1,
    $2,
    $3,
    $4
)
ON CONFLICT (user_id, date)
DO UPDATE SET
    weight_kg = EXCLUDED.weight_kg,
    unit = EXCLUDED.unit,
    updated_at = NOW()
RETURNING user_id, date, weight_kg, unit, created_at, updated_at;

-- name: ListNutritionWeightTrend :many
WITH bounds AS (
    SELECT DATE_TRUNC('week', sqlc.arg(anchor_date)::timestamp)::date AS current_week_start
),
week_series AS (
    SELECT generate_series(
        (SELECT current_week_start FROM bounds) - INTERVAL '7 weeks',
        (SELECT current_week_start FROM bounds),
        INTERVAL '1 week'
    )::date AS week_start_date
),
weekly_latest AS (
    SELECT DISTINCT ON (DATE_TRUNC('week', uwe.date::timestamp)::date)
        DATE_TRUNC('week', uwe.date::timestamp)::date AS week_start_date,
        uwe.date AS entry_date,
        uwe.weight_kg,
        uwe.unit,
        uwe.updated_at
    FROM user_weight_entries uwe
    JOIN bounds b ON true
    WHERE uwe.user_id = sqlc.arg(user_id)
      AND uwe.date >= (b.current_week_start - INTERVAL '7 weeks')
      AND uwe.date < (b.current_week_start + INTERVAL '1 week')
    ORDER BY DATE_TRUNC('week', uwe.date::timestamp)::date ASC, uwe.date DESC, uwe.updated_at DESC
)
SELECT
    ws.week_start_date,
    COALESCE((wl.entry_date IS NOT NULL), FALSE)::boolean AS has_entry,
    COALESCE(wl.entry_date, DATE '0001-01-01')::date AS entry_date,
    COALESCE(wl.weight_kg, 0.0)::double precision AS weight_kg,
    COALESCE(wl.unit, 'kg')::text AS unit
FROM week_series ws
LEFT JOIN weekly_latest wl ON wl.week_start_date = ws.week_start_date
ORDER BY ws.week_start_date ASC;

-- name: ListNutritionDailyCheckinsByDateRange :many
SELECT id, user_id, date, calories_estimate, protein_g_estimate, hit_calories, hit_protein, notes, created_at
FROM nutrition_daily_checkins
WHERE user_id = $1
  AND date >= sqlc.arg(start_date)
  AND date < sqlc.arg(end_date)
ORDER BY date ASC;

-- name: GetLatestUserWeightEntryByDateRange :one
SELECT user_id, date, weight_kg, unit, created_at, updated_at
FROM user_weight_entries
WHERE user_id = $1
  AND date >= sqlc.arg(start_date)
  AND date < sqlc.arg(end_date)
ORDER BY date DESC, updated_at DESC
LIMIT 1;

-- name: UpsertWeeklyCheckin :one
INSERT INTO weekly_checkins (
    user_id,
    week_start,
    adherence,
    weight_change,
    new_targets_json,
    explanation
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6
)
ON CONFLICT (user_id, week_start)
DO UPDATE SET
    adherence = EXCLUDED.adherence,
    weight_change = EXCLUDED.weight_change,
    new_targets_json = EXCLUDED.new_targets_json,
    explanation = EXCLUDED.explanation,
    updated_at = NOW()
RETURNING user_id, week_start, adherence, weight_change, new_targets_json, explanation, created_at, updated_at;

-- name: GetLatestWeeklyCheckinByUserID :one
SELECT user_id, week_start, adherence, weight_change, new_targets_json, explanation, created_at, updated_at
FROM weekly_checkins
WHERE user_id = $1
ORDER BY week_start DESC
LIMIT 1;

-- name: GetWeeklyCheckinByUserIDAndWeekStart :one
SELECT user_id, week_start, adherence, weight_change, new_targets_json, explanation, created_at, updated_at
FROM weekly_checkins
WHERE user_id = $1
  AND week_start = $2
LIMIT 1;
