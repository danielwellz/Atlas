-- name: ListPrograms :many
SELECT id, slug, name, description, goal_tags_json, level, weeks_length, created_at
FROM programs
ORDER BY name ASC;

-- name: GetProgramByID :one
SELECT id, slug, name, description, goal_tags_json, level, weeks_length, created_at
FROM programs
WHERE id = $1
LIMIT 1;

-- name: ListProgramBlocksByProgramID :many
SELECT
    pw.id,
    pw.program_id,
    pw.week_index,
    COALESCE(
        array_agg(ps.day_of_week ORDER BY ps.day_of_week) FILTER (WHERE ps.id IS NOT NULL),
        ARRAY[]::int[]
    )::int[] AS session_days,
    COUNT(ps.id)::int AS session_count
FROM program_weeks pw
LEFT JOIN program_sessions ps ON ps.program_week_id = pw.id
WHERE pw.program_id = $1
GROUP BY pw.id, pw.program_id, pw.week_index
ORDER BY pw.week_index ASC;

-- name: UpsertUserProgramEnrollment :one
INSERT INTO user_program_enrollments (
    user_id,
    program_id,
    start_date,
    current_week
) VALUES (
    $1,
    $2,
    $3,
    1
)
ON CONFLICT (user_id)
DO UPDATE SET
    program_id = EXCLUDED.program_id,
    start_date = EXCLUDED.start_date,
    current_week = 1,
    created_at = NOW()
RETURNING id, user_id, program_id, start_date, current_week, created_at;

-- name: GetUserProgramEnrollmentByUserID :one
SELECT id, user_id, program_id, start_date, current_week, created_at
FROM user_program_enrollments
WHERE user_id = $1
LIMIT 1;

-- name: GetProgramWeekByProgramIDAndWeekIndex :one
SELECT id, program_id, week_index, created_at
FROM program_weeks
WHERE program_id = $1
  AND week_index = $2
LIMIT 1;

-- name: ListProgramSessionsByProgramWeekID :many
SELECT id, program_week_id, day_of_week, name, created_at
FROM program_sessions
WHERE program_week_id = $1
ORDER BY day_of_week ASC;

-- name: ListProgramSessionExercisesByProgramSessionID :many
SELECT
    pse.id,
    pse.program_session_id,
    pse.exercise_id,
    pse.prescription_json,
    pse.order_index,
    e.slug AS exercise_slug,
    e.name AS exercise_name
FROM program_session_exercises pse
JOIN exercises e ON e.id = pse.exercise_id
WHERE pse.program_session_id = $1
ORDER BY pse.order_index ASC;

-- name: ListProgramSubstitutionCandidates :many
WITH prescribed AS (
    SELECT id, movement_pattern
    FROM exercises
    WHERE exercises.id = sqlc.arg(exercise_id)
)
SELECT
    e.id,
    e.slug,
    e.name,
    e.movement_pattern,
    e.equipment_json
FROM prescribed p
JOIN exercises e ON e.movement_pattern = p.movement_pattern
WHERE e.id <> p.id
  AND (
    cardinality(sqlc.arg(equipment)::text[]) = 0
    OR e.equipment_json ?| sqlc.arg(equipment)::text[]
  )
ORDER BY e.name ASC
LIMIT sqlc.arg(max_count);
