-- name: ListCompletedWorkoutIDsSince :many
SELECT id
FROM workouts
WHERE user_id = $1
  AND completed_at IS NOT NULL
  AND completed_at >= $2
ORDER BY completed_at DESC, id DESC;

-- name: ListCompletedWorkoutDatesSince :many
SELECT DISTINCT DATE_TRUNC('day', completed_at AT TIME ZONE 'UTC')::date AS completed_date
FROM workouts
WHERE user_id = sqlc.arg(user_id)
  AND completed_at IS NOT NULL
  AND completed_at >= sqlc.arg(completed_at)
ORDER BY completed_date ASC;

-- name: ListWorkoutSetMetricsSince :many
SELECT
    e.movement_pattern,
    ws.reps,
    ws.weight_kg
FROM workout_sets ws
JOIN workout_exercises we ON we.id = ws.workout_exercise_id
JOIN workouts w ON w.id = we.workout_id
JOIN exercises e ON e.id = we.exercise_id
WHERE w.user_id = $1
  AND ws.completed_at >= $2
ORDER BY ws.completed_at DESC, ws.id DESC;

-- name: ListCoreLiftSetsForPR :many
SELECT
    e.slug,
    ws.reps,
    ws.weight_kg
FROM workout_sets ws
JOIN workout_exercises we ON we.id = ws.workout_exercise_id
JOIN workouts w ON w.id = we.workout_id
JOIN exercises e ON e.id = we.exercise_id
WHERE w.user_id = $1
  AND e.slug IN ('back-squat', 'bench-press', 'conventional-deadlift')
ORDER BY ws.completed_at DESC, ws.id DESC;

-- name: ListMainLiftEstimatedOneRM :many
-- Epley estimated 1RM formula: estimated_1RM = weight_kg * (1 + reps / 30)
WITH scored_sets AS (
    SELECT
        CASE
            WHEN e.slug = 'back-squat' THEN 'squat'
            WHEN e.slug = 'bench-press' THEN 'bench'
            WHEN e.slug = 'conventional-deadlift' THEN 'deadlift'
            ELSE NULL
        END::text AS lift,
        ws.completed_at,
        ws.reps,
        ws.weight_kg,
        (ws.weight_kg * (1.0 + ws.reps::double precision / 30.0))::double precision AS estimated_one_rm_kg,
        ws.id
    FROM workout_sets ws
    JOIN workout_exercises we ON we.id = ws.workout_exercise_id
    JOIN workouts w ON w.id = we.workout_id
    JOIN exercises e ON e.id = we.exercise_id
    WHERE w.user_id = $1
      AND ws.reps > 0
      AND ws.weight_kg >= 0
      AND e.slug IN ('back-squat', 'bench-press', 'conventional-deadlift')
),
ranked AS (
    SELECT
        lift,
        completed_at,
        reps,
        weight_kg,
        estimated_one_rm_kg,
        ROW_NUMBER() OVER (
            PARTITION BY lift
            ORDER BY estimated_one_rm_kg DESC, completed_at DESC, id DESC
        ) AS rank_index
    FROM scored_sets
    WHERE lift IS NOT NULL
)
SELECT
    lift,
    completed_at,
    reps,
    weight_kg,
    estimated_one_rm_kg
FROM ranked
WHERE rank_index = 1
ORDER BY lift ASC;

-- name: ListLiftPREventsSince :many
-- Epley estimated 1RM formula: estimated_1RM = weight_kg * (1 + reps / 30)
WITH scored_sets AS (
    SELECT
        CASE
            WHEN e.slug = 'back-squat' THEN 'squat'
            WHEN e.slug = 'bench-press' THEN 'bench'
            WHEN e.slug = 'conventional-deadlift' THEN 'deadlift'
            ELSE NULL
        END::text AS lift,
        ws.completed_at,
        ws.reps,
        ws.weight_kg,
        (ws.weight_kg * (1.0 + ws.reps::double precision / 30.0))::double precision AS estimated_one_rm_kg,
        ws.id
    FROM workout_sets ws
    JOIN workout_exercises we ON we.id = ws.workout_exercise_id
    JOIN workouts w ON w.id = we.workout_id
    JOIN exercises e ON e.id = we.exercise_id
    WHERE w.user_id = sqlc.arg(user_id)
      AND ws.completed_at >= sqlc.arg(completed_at)
      AND ws.reps > 0
      AND ws.weight_kg >= 0
      AND e.slug IN ('back-squat', 'bench-press', 'conventional-deadlift')
),
events AS (
    SELECT
        lift,
        completed_at,
        reps,
        weight_kg,
        estimated_one_rm_kg,
        MAX(estimated_one_rm_kg) OVER (
            PARTITION BY lift
            ORDER BY completed_at ASC, id ASC
            ROWS BETWEEN UNBOUNDED PRECEDING AND 1 PRECEDING
        ) AS previous_best_estimated_one_rm_kg
    FROM scored_sets
    WHERE lift IS NOT NULL
)
SELECT
    lift,
    completed_at,
    reps,
    weight_kg,
    estimated_one_rm_kg,
    COALESCE(previous_best_estimated_one_rm_kg, -1.0)::double precision AS previous_best_estimated_one_rm_kg
FROM events
WHERE previous_best_estimated_one_rm_kg IS NULL
   OR estimated_one_rm_kg > previous_best_estimated_one_rm_kg
ORDER BY completed_at DESC, lift ASC;

-- name: ListWeeklyMuscleGroupVolumeSince :many
-- Muscle group allocation uses normalized coefficients:
-- primary muscles = 1.0 each, secondary muscles = 0.5 each.
WITH set_volume AS (
    SELECT
        ws.id AS workout_set_id,
        DATE_TRUNC('week', ws.completed_at AT TIME ZONE 'UTC')::date AS week_start_date,
        (ws.reps::double precision * ws.weight_kg) AS set_volume_kg,
        e.primary_muscles,
        e.secondary_muscles
    FROM workout_sets ws
    JOIN workout_exercises we ON we.id = ws.workout_exercise_id
    JOIN workouts w ON w.id = we.workout_id
    JOIN exercises e ON e.id = we.exercise_id
    WHERE w.user_id = sqlc.arg(user_id)
      AND ws.completed_at >= sqlc.arg(completed_at)
      AND ws.reps > 0
      AND ws.weight_kg >= 0
),
expanded AS (
    SELECT
        sv.week_start_date,
        LOWER(TRIM(muscle_map.muscle_group)) AS muscle_group,
        sv.set_volume_kg,
        muscle_map.coefficient,
        (
            COALESCE(jsonb_array_length(sv.primary_muscles), 0)::double precision
            + COALESCE(jsonb_array_length(sv.secondary_muscles), 0)::double precision * 0.5
        ) AS coefficient_total
    FROM set_volume sv
    CROSS JOIN LATERAL (
        SELECT primary_muscle.value AS muscle_group, 1.0::double precision AS coefficient
        FROM jsonb_array_elements_text(sv.primary_muscles) AS primary_muscle(value)
        UNION ALL
        SELECT secondary_muscle.value AS muscle_group, 0.5::double precision AS coefficient
        FROM jsonb_array_elements_text(sv.secondary_muscles) AS secondary_muscle(value)
    ) AS muscle_map
)
SELECT
    week_start_date,
    muscle_group,
    SUM(
        CASE
            WHEN coefficient_total <= 0 THEN 0
            ELSE set_volume_kg * (coefficient / coefficient_total)
        END
    )::double precision AS volume_kg
FROM expanded
WHERE muscle_group <> ''
GROUP BY week_start_date, muscle_group
ORDER BY week_start_date DESC, volume_kg DESC, muscle_group ASC;

-- name: ListNutritionProteinCheckinsSinceDate :many
SELECT date, hit_protein
FROM nutrition_daily_checkins
WHERE user_id = $1
  AND date >= $2
ORDER BY date ASC;

-- name: ListProteinHitDatesSinceDate :many
SELECT date AS hit_date
FROM nutrition_daily_checkins
WHERE user_id = sqlc.arg(user_id)
  AND date >= sqlc.arg(date)
  AND hit_protein = TRUE
ORDER BY hit_date ASC;

-- name: ListWeightTrendPointsSinceDate :many
SELECT date, weight_kg
FROM user_weight_entries
WHERE user_id = sqlc.arg(user_id)
  AND date >= sqlc.arg(date)
ORDER BY date ASC;

-- name: ListReadinessCheckinsSinceDate :many
SELECT date, energy_level, sleep_quality, stress_level, readiness_score
FROM readiness_checkins
WHERE user_id = sqlc.arg(user_id)
  AND date >= sqlc.arg(date)
ORDER BY date ASC;
