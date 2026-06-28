-- name: ListRecipes :many
SELECT
    id,
    slug,
    name,
    meal_type,
    description,
    servings,
    calories_kcal,
    protein_g,
    carbs_g,
    fat_g,
    ingredients_json,
    instructions_json,
    created_at,
    updated_at
FROM recipes
ORDER BY meal_type ASC, name ASC;

-- name: GetLatestMealPlanByUserID :one
SELECT id, user_id, week_start, target_json, created_at, updated_at
FROM meal_plans
WHERE user_id = $1
ORDER BY week_start DESC, updated_at DESC
LIMIT 1;

-- name: GetMealPlanByUserIDAndWeekStart :one
SELECT id, user_id, week_start, target_json, created_at, updated_at
FROM meal_plans
WHERE user_id = $1
  AND week_start = $2
LIMIT 1;

-- name: UpsertMealPlan :one
INSERT INTO meal_plans (
    user_id,
    week_start,
    target_json
) VALUES (
    $1,
    $2,
    $3
)
ON CONFLICT (user_id, week_start)
DO UPDATE SET
    target_json = EXCLUDED.target_json,
    updated_at = NOW()
RETURNING id, user_id, week_start, target_json, created_at, updated_at;

-- name: DeleteMealPlanItemsByMealPlanID :exec
DELETE FROM meal_plan_items
WHERE meal_plan_id = $1;

-- name: DeleteGroceryItemsByMealPlanID :exec
DELETE FROM grocery_items
WHERE meal_plan_id = $1;

-- name: CreateMealPlanItem :one
INSERT INTO meal_plan_items (
    meal_plan_id,
    day_of_week,
    meal_slot,
    recipe_id,
    servings
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5
)
RETURNING id, meal_plan_id, day_of_week, meal_slot, recipe_id, servings, created_at;

-- name: CreateGroceryItem :one
INSERT INTO grocery_items (
    meal_plan_id,
    name,
    quantity,
    unit,
    category
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5
)
RETURNING id, meal_plan_id, name, quantity, unit, category, created_at;

-- name: ListMealPlanItemsByMealPlanID :many
SELECT
    mpi.id,
    mpi.meal_plan_id,
    mpi.day_of_week,
    mpi.meal_slot,
    mpi.recipe_id,
    mpi.servings,
    mpi.created_at,
    r.slug AS recipe_slug,
    r.name AS recipe_name,
    r.meal_type AS recipe_meal_type,
    r.description AS recipe_description,
    r.servings AS recipe_servings,
    r.calories_kcal AS recipe_calories_kcal,
    r.protein_g AS recipe_protein_g,
    r.carbs_g AS recipe_carbs_g,
    r.fat_g AS recipe_fat_g,
    r.ingredients_json AS recipe_ingredients_json,
    r.instructions_json AS recipe_instructions_json
FROM meal_plan_items mpi
JOIN recipes r ON r.id = mpi.recipe_id
WHERE mpi.meal_plan_id = $1
ORDER BY mpi.day_of_week ASC, mpi.meal_slot ASC, mpi.created_at ASC;

-- name: ListGroceryItemsByMealPlanID :many
SELECT id, meal_plan_id, name, quantity, unit, category, created_at
FROM grocery_items
WHERE meal_plan_id = $1
ORDER BY category ASC, name ASC;
