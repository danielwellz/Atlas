-- name: UpsertFood :one
INSERT INTO foods (
    external_id,
    provider,
    label,
    brand,
    nutrients_json
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5
)
ON CONFLICT (provider, external_id)
DO UPDATE SET
    label = EXCLUDED.label,
    brand = EXCLUDED.brand,
    nutrients_json = EXCLUDED.nutrients_json,
    updated_at = NOW()
RETURNING id, external_id, provider, label, brand, nutrients_json, created_at, updated_at;

-- name: GetFoodByID :one
SELECT id, external_id, provider, label, brand, nutrients_json, created_at, updated_at
FROM foods
WHERE id = $1
LIMIT 1;

-- name: CreateFoodLog :one
INSERT INTO food_logs (
    user_id,
    datetime,
    food_id,
    quantity,
    unit,
    nutrients_snapshot_json
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6
)
RETURNING id, user_id, datetime, food_id, quantity, unit, nutrients_snapshot_json, created_at;

-- name: ListFoodLogsByUserIDAndDate :many
SELECT
    fl.id,
    fl.user_id,
    fl.datetime,
    fl.food_id,
    fl.quantity,
    fl.unit,
    fl.nutrients_snapshot_json,
    fl.created_at,
    f.external_id,
    f.provider,
    f.label,
    f.brand,
    f.nutrients_json,
    f.created_at,
    f.updated_at
FROM food_logs fl
JOIN foods f ON f.id = fl.food_id
WHERE fl.user_id = $1
  AND (fl.datetime AT TIME ZONE 'UTC')::date = sqlc.arg(date)
ORDER BY fl.datetime DESC, fl.id DESC;

-- name: GetFoodLogDailyTotalsByUserIDAndDate :one
SELECT
    COALESCE(SUM(COALESCE(NULLIF(fl.nutrients_snapshot_json->>'calories_kcal', '')::double precision, 0.0)), 0.0)::double precision AS calories_kcal,
    COALESCE(SUM(COALESCE(NULLIF(fl.nutrients_snapshot_json->>'protein_g', '')::double precision, 0.0)), 0.0)::double precision AS protein_g,
    COALESCE(SUM(COALESCE(NULLIF(fl.nutrients_snapshot_json->>'carbs_g', '')::double precision, 0.0)), 0.0)::double precision AS carbs_g,
    COALESCE(SUM(COALESCE(NULLIF(fl.nutrients_snapshot_json->>'fat_g', '')::double precision, 0.0)), 0.0)::double precision AS fat_g
FROM food_logs fl
WHERE fl.user_id = $1
  AND (fl.datetime AT TIME ZONE 'UTC')::date = sqlc.arg(date);
