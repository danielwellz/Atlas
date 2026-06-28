-- name: CountActiveHabitsByUserID :one
SELECT COUNT(*)
FROM habits
WHERE user_id = $1
  AND active = TRUE;

-- name: ListHabitsByUserID :many
SELECT id, user_id, name, type, target_json, active, created_at
FROM habits
WHERE user_id = $1
ORDER BY created_at ASC;

-- name: CreateHabit :one
INSERT INTO habits (
    user_id,
    name,
    type,
    target_json,
    active
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5
)
RETURNING id, user_id, name, type, target_json, active, created_at;

-- name: GetHabitByIDAndUserID :one
SELECT id, user_id, name, type, target_json, active, created_at
FROM habits
WHERE id = $1
  AND user_id = $2
LIMIT 1;

-- name: ListHabitDailyLogsByUserID :many
SELECT
    hdl.id,
    hdl.habit_id,
    hdl.date,
    hdl.completed,
    hdl.completed_at,
    hdl.created_at
FROM habit_daily_logs hdl
JOIN habits h ON h.id = hdl.habit_id
WHERE h.user_id = $1
ORDER BY hdl.habit_id ASC, hdl.date ASC;

-- name: GetHabitDailyLogByHabitIDAndDate :one
SELECT id, habit_id, date, completed, completed_at, created_at
FROM habit_daily_logs
WHERE habit_id = $1
  AND date = $2
LIMIT 1;

-- name: CreateHabitDailyLog :one
INSERT INTO habit_daily_logs (
    habit_id,
    date,
    completed,
    completed_at
) VALUES (
    $1,
    $2,
    $3,
    $4
)
RETURNING id, habit_id, date, completed, completed_at, created_at;

-- name: UpdateHabitDailyLogCompletion :one
UPDATE habit_daily_logs
SET completed = $2,
    completed_at = $3
WHERE id = $1
RETURNING id, habit_id, date, completed, completed_at, created_at;
