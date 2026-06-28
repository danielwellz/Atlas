-- name: UpsertExercise :one
INSERT INTO exercises (
    id,
    slug,
    name,
    primary_muscle_group,
    secondary_muscles_json,
    movement_pattern,
    movement_pattern_taxonomy,
    primary_muscles,
    secondary_muscles,
    contraindications,
    contraindication_tags,
    equipment_json,
    equipment_requirements,
    difficulty,
    description
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    jsonb_build_array($6::text),
    $7,
    $8,
    $9,
    $9,
    $10,
    $10,
    $11,
    $12
)
ON CONFLICT (slug)
DO UPDATE SET
    name = EXCLUDED.name,
    primary_muscle_group = EXCLUDED.primary_muscle_group,
    secondary_muscles_json = EXCLUDED.secondary_muscles_json,
    movement_pattern = EXCLUDED.movement_pattern,
    movement_pattern_taxonomy = EXCLUDED.movement_pattern_taxonomy,
    primary_muscles = EXCLUDED.primary_muscles,
    secondary_muscles = EXCLUDED.secondary_muscles,
    contraindications = EXCLUDED.contraindications,
    contraindication_tags = EXCLUDED.contraindication_tags,
    equipment_json = EXCLUDED.equipment_json,
    equipment_requirements = EXCLUDED.equipment_requirements,
    difficulty = EXCLUDED.difficulty,
    description = EXCLUDED.description
RETURNING id, slug, name, primary_muscle_group, secondary_muscles_json, movement_pattern, movement_pattern_taxonomy, primary_muscles, secondary_muscles, contraindications, contraindication_tags, equipment_json, equipment_requirements, difficulty, description, created_at;

-- name: ListExercises :many
SELECT id, slug, name, primary_muscle_group, secondary_muscles_json, movement_pattern, movement_pattern_taxonomy, primary_muscles, secondary_muscles, contraindications, contraindication_tags, equipment_json, equipment_requirements, difficulty, description, created_at
FROM exercises
WHERE (sqlc.arg(query)::text = '' OR name ILIKE '%' || sqlc.arg(query) || '%' OR slug ILIKE '%' || sqlc.arg(query) || '%' OR description ILIKE '%' || sqlc.arg(query) || '%')
  AND (sqlc.arg(equipment)::text = '' OR equipment_json ? sqlc.arg(equipment))
  AND (sqlc.arg(pattern)::text = '' OR movement_pattern = sqlc.arg(pattern))
ORDER BY name ASC;

-- name: GetExerciseByID :one
SELECT id, slug, name, primary_muscle_group, secondary_muscles_json, movement_pattern, movement_pattern_taxonomy, primary_muscles, secondary_muscles, contraindications, contraindication_tags, equipment_json, equipment_requirements, difficulty, description, created_at
FROM exercises
WHERE id = $1
LIMIT 1;

-- name: ListExerciseCandidatesForSubstitution :many
SELECT id, slug, name, primary_muscle_group, secondary_muscles_json, movement_pattern, movement_pattern_taxonomy, primary_muscles, secondary_muscles, contraindications, contraindication_tags, equipment_json, equipment_requirements, difficulty, description, created_at
FROM exercises
WHERE id <> sqlc.arg(exercise_id)
ORDER BY name ASC;

-- name: ListExerciseMediaByExerciseID :many
SELECT id, exercise_id, media_type, uri, thumbnail_uri, duration_seconds, created_at
FROM exercise_media
WHERE exercise_id = $1
ORDER BY created_at ASC;
