-- name: CreateWorkout :one
INSERT INTO workouts (
    user_id,
    program_session_id,
    started_at,
    notes
) VALUES (
    $1,
    $2,
    $3,
    $4
)
RETURNING id, user_id, program_session_id, started_at, completed_at, notes, created_at;

-- name: GetWorkoutByIDAndUserID :one
SELECT id, user_id, program_session_id, started_at, completed_at, notes, created_at
FROM workouts
WHERE id = $1
  AND user_id = $2
LIMIT 1;

-- name: CompleteWorkout :one
UPDATE workouts
SET completed_at = $3,
    notes = $4
WHERE id = $1
  AND user_id = $2
  AND completed_at IS NULL
RETURNING id, user_id, program_session_id, started_at, completed_at, notes, created_at;

-- name: GetProgramSessionByID :one
SELECT id, program_week_id, day_of_week, name, created_at
FROM program_sessions
WHERE id = $1
LIMIT 1;

-- name: GetProgramIDByProgramSessionID :one
SELECT pw.program_id
FROM program_sessions ps
JOIN program_weeks pw ON pw.id = ps.program_week_id
WHERE ps.id = $1
LIMIT 1;

-- name: CountProgramSessionsForWeek :one
SELECT COUNT(*)::int
FROM program_sessions ps
JOIN program_weeks pw ON pw.id = ps.program_week_id
WHERE pw.program_id = $1
  AND pw.week_index = $2;

-- name: CountCompletedProgramWorkoutsBetween :one
SELECT COUNT(*)::int
FROM workouts w
JOIN program_sessions ps ON ps.id = w.program_session_id
JOIN program_weeks pw ON pw.id = ps.program_week_id
WHERE w.user_id = sqlc.arg(user_id)
  AND pw.program_id = sqlc.arg(program_id)
  AND pw.week_index = sqlc.arg(week_index)
  AND w.completed_at IS NOT NULL
  AND w.completed_at >= sqlc.arg(window_start)
  AND w.completed_at < sqlc.arg(window_end);

-- name: CountCompletedProgramWorkoutSetsBetween :one
SELECT COUNT(ws.id)::int
FROM workouts w
JOIN program_sessions ps ON ps.id = w.program_session_id
JOIN program_weeks pw ON pw.id = ps.program_week_id
JOIN workout_exercises we ON we.workout_id = w.id
JOIN workout_sets ws ON ws.workout_exercise_id = we.id
WHERE w.user_id = sqlc.arg(user_id)
  AND pw.program_id = sqlc.arg(program_id)
  AND pw.week_index = sqlc.arg(week_index)
  AND w.completed_at IS NOT NULL
  AND w.completed_at >= sqlc.arg(window_start)
  AND w.completed_at < sqlc.arg(window_end);

-- name: CountHighRpeProgramWorkoutSetsBetween :one
SELECT COUNT(ws.id)::int
FROM workouts w
JOIN program_sessions ps ON ps.id = w.program_session_id
JOIN program_weeks pw ON pw.id = ps.program_week_id
JOIN workout_exercises we ON we.workout_id = w.id
JOIN workout_sets ws ON ws.workout_exercise_id = we.id
WHERE w.user_id = sqlc.arg(user_id)
  AND pw.program_id = sqlc.arg(program_id)
  AND pw.week_index = sqlc.arg(week_index)
  AND w.completed_at IS NOT NULL
  AND w.completed_at >= sqlc.arg(window_start)
  AND w.completed_at < sqlc.arg(window_end)
  AND ws.rpe IS NOT NULL
  AND ws.rpe >= sqlc.arg(rpe_threshold);

-- name: CountCompletedProgramWorkoutsBefore :one
SELECT COUNT(*)::int
FROM workouts w
JOIN program_sessions ps ON ps.id = w.program_session_id
JOIN program_weeks pw ON pw.id = ps.program_week_id
WHERE w.user_id = sqlc.arg(user_id)
  AND pw.program_id = sqlc.arg(program_id)
  AND w.completed_at IS NOT NULL
  AND w.completed_at < sqlc.arg(before_time);

-- name: CreateWorkoutExercisesFromProgramSession :many
INSERT INTO workout_exercises (
    workout_id,
    exercise_id,
    order_index,
    planned_json,
    actual_json
)
SELECT
    $1 AS workout_id,
    pse.exercise_id,
    pse.order_index,
    pse.prescription_json,
    '{}'::jsonb
FROM program_session_exercises pse
WHERE pse.program_session_id = $2
ORDER BY pse.order_index ASC
RETURNING id, workout_id, exercise_id, order_index, planned_json, actual_json, created_at;

-- name: CreateWorkoutSetAutoIndexed :one
WITH target_workout AS (
    SELECT w.id, w.completed_at
    FROM workouts w
    WHERE w.id = sqlc.arg(workout_id)
      AND w.user_id = sqlc.arg(user_id)
    FOR UPDATE OF w
),
target_exercise AS (
    SELECT we.id
    FROM workout_exercises we
    JOIN target_workout tw ON tw.id = we.workout_id
    WHERE we.id = sqlc.arg(workout_exercise_id)
    FOR UPDATE OF we
),
existing_set AS (
    SELECT
        ws.id,
        ws.workout_exercise_id,
        ws.set_index,
        ws.reps,
        ws.weight_kg,
        ws.rpe,
        ws.completed_at,
        ws.created_at,
        ws.idempotency_key
    FROM workout_sets ws
    JOIN target_exercise te ON te.id = ws.workout_exercise_id
    WHERE ws.idempotency_key = sqlc.arg(idempotency_key)
    LIMIT 1
),
next_set_index AS (
    SELECT COALESCE(MAX(ws.set_index) + 1, 1) AS set_index
    FROM workout_sets ws
    JOIN target_exercise te ON te.id = ws.workout_exercise_id
),
inserted_set AS (
    INSERT INTO workout_sets (
        workout_exercise_id,
        set_index,
        reps,
        weight_kg,
        rpe,
        completed_at,
        idempotency_key
    )
    SELECT
        te.id,
        nsi.set_index,
        sqlc.arg(reps),
        sqlc.arg(weight_kg),
        sqlc.narg(rpe),
        sqlc.arg(completed_at),
        sqlc.arg(idempotency_key)
    FROM target_exercise te
    CROSS JOIN next_set_index nsi
    WHERE NOT EXISTS (SELECT 1 FROM existing_set)
      AND EXISTS (
          SELECT 1
          FROM target_workout tw
          WHERE tw.completed_at IS NULL
      )
    RETURNING id, workout_exercise_id, set_index, reps, weight_kg, rpe, completed_at, created_at, idempotency_key
)
SELECT id, workout_exercise_id, set_index, reps, weight_kg, rpe, completed_at, created_at, idempotency_key
FROM inserted_set
UNION ALL
SELECT id, workout_exercise_id, set_index, reps, weight_kg, rpe, completed_at, created_at, idempotency_key
FROM existing_set;

-- name: ListWorkoutHistory :many
SELECT id, user_id, program_session_id, started_at, completed_at, notes, created_at
FROM workouts
WHERE user_id = sqlc.arg(user_id)
  AND (
    NOT sqlc.arg(has_cursor)::boolean
    OR (started_at, id) < (sqlc.arg(cursor_started_at)::timestamptz, sqlc.arg(cursor_id)::uuid)
  )
ORDER BY started_at DESC, id DESC
LIMIT sqlc.arg(limit_count);

-- name: ListWorkoutExercisesByWorkoutID :many
SELECT
    we.id,
    we.workout_id,
    we.exercise_id,
    we.order_index,
    we.planned_json,
    we.actual_json,
    we.created_at,
    e.slug AS exercise_slug,
    e.name AS exercise_name
FROM workout_exercises we
JOIN exercises e ON e.id = we.exercise_id
WHERE we.workout_id = $1
ORDER BY we.order_index ASC;

-- name: ListWorkoutSetsByWorkoutID :many
SELECT
    ws.id,
    ws.workout_exercise_id,
    ws.set_index,
    ws.reps,
    ws.weight_kg,
    ws.rpe,
    ws.completed_at,
    ws.created_at,
    ws.idempotency_key
FROM workout_sets ws
JOIN workout_exercises we ON we.id = ws.workout_exercise_id
WHERE we.workout_id = $1
ORDER BY we.order_index ASC, ws.set_index ASC;

-- name: ListPreviousWorkoutSetsByWorkoutID :many
WITH current_workout AS (
    SELECT w.id, w.user_id, w.started_at
    FROM workouts w
    WHERE w.id = sqlc.arg(workout_id)
    LIMIT 1
),
current_exercises AS (
    SELECT
        we.id AS target_workout_exercise_id,
        we.exercise_id
    FROM workout_exercises we
    WHERE we.workout_id = sqlc.arg(workout_id)
),
latest_previous_workout AS (
    SELECT
        ce.target_workout_exercise_id,
        pw.id AS previous_workout_id,
        pw.completed_at AS previous_workout_completed_at,
        ROW_NUMBER() OVER (
            PARTITION BY ce.target_workout_exercise_id
            ORDER BY pw.completed_at DESC, pw.id DESC
        ) AS rank_index
    FROM current_exercises ce
    JOIN workout_exercises pwe ON pwe.exercise_id = ce.exercise_id
    JOIN workouts pw ON pw.id = pwe.workout_id
    JOIN current_workout cw ON cw.user_id = pw.user_id
    WHERE pw.completed_at IS NOT NULL
      AND pw.id <> cw.id
      AND (pw.started_at, pw.id) < (cw.started_at, cw.id)
),
selected_previous_workout AS (
    SELECT
        target_workout_exercise_id,
        previous_workout_id,
        previous_workout_completed_at
    FROM latest_previous_workout
    WHERE rank_index = 1
),
selected_previous_exercise AS (
    SELECT
        spw.target_workout_exercise_id,
        pwe.id AS previous_workout_exercise_id,
        spw.previous_workout_id,
        spw.previous_workout_completed_at
    FROM selected_previous_workout spw
    JOIN current_exercises ce
        ON ce.target_workout_exercise_id = spw.target_workout_exercise_id
    JOIN LATERAL (
        SELECT previous_we.id
        FROM workout_exercises previous_we
        WHERE previous_we.workout_id = spw.previous_workout_id
          AND previous_we.exercise_id = ce.exercise_id
        ORDER BY previous_we.order_index ASC
        LIMIT 1
    ) pwe ON TRUE
)
SELECT
    spe.target_workout_exercise_id,
    spe.previous_workout_id,
    spe.previous_workout_completed_at,
    ws.set_index,
    ws.reps,
    ws.weight_kg,
    ws.rpe
FROM selected_previous_exercise spe
JOIN workout_sets ws ON ws.workout_exercise_id = spe.previous_workout_exercise_id
ORDER BY spe.target_workout_exercise_id ASC, ws.set_index ASC;
