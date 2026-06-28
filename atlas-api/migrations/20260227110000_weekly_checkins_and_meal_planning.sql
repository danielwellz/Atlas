-- +goose Up
CREATE TABLE IF NOT EXISTS weekly_checkins (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    week_start DATE NOT NULL,
    adherence DOUBLE PRECISION NOT NULL,
    weight_change DOUBLE PRECISION NOT NULL,
    new_targets_json JSONB NOT NULL,
    explanation TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT weekly_checkins_primary_key PRIMARY KEY (user_id, week_start),
    CONSTRAINT weekly_checkins_adherence_check CHECK (adherence >= 0 AND adherence <= 1)
);

CREATE INDEX IF NOT EXISTS idx_weekly_checkins_user_week_start_desc
    ON weekly_checkins (user_id, week_start DESC);

CREATE TABLE IF NOT EXISTS recipes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug TEXT NOT NULL,
    name TEXT NOT NULL,
    meal_type TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    servings DOUBLE PRECISION NOT NULL DEFAULT 1,
    calories_kcal INTEGER NOT NULL,
    protein_g INTEGER NOT NULL,
    carbs_g INTEGER NOT NULL,
    fat_g INTEGER NOT NULL,
    ingredients_json JSONB NOT NULL,
    instructions_json JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT recipes_slug_unique UNIQUE (slug),
    CONSTRAINT recipes_servings_check CHECK (servings > 0),
    CONSTRAINT recipes_calories_check CHECK (calories_kcal > 0),
    CONSTRAINT recipes_protein_check CHECK (protein_g >= 0),
    CONSTRAINT recipes_carbs_check CHECK (carbs_g >= 0),
    CONSTRAINT recipes_fat_check CHECK (fat_g >= 0)
);

CREATE TABLE IF NOT EXISTS meal_plans (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    week_start DATE NOT NULL,
    target_json JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT meal_plans_user_week_start_unique UNIQUE (user_id, week_start)
);

CREATE INDEX IF NOT EXISTS idx_meal_plans_user_week_start_desc
    ON meal_plans (user_id, week_start DESC);

CREATE TABLE IF NOT EXISTS meal_plan_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    meal_plan_id UUID NOT NULL REFERENCES meal_plans(id) ON DELETE CASCADE,
    day_of_week INTEGER NOT NULL,
    meal_slot TEXT NOT NULL,
    recipe_id UUID NOT NULL REFERENCES recipes(id) ON DELETE RESTRICT,
    servings DOUBLE PRECISION NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT meal_plan_items_day_of_week_check CHECK (day_of_week BETWEEN 1 AND 7),
    CONSTRAINT meal_plan_items_servings_check CHECK (servings > 0),
    CONSTRAINT meal_plan_items_unique_slot UNIQUE (meal_plan_id, day_of_week, meal_slot)
);

CREATE INDEX IF NOT EXISTS idx_meal_plan_items_meal_plan_id
    ON meal_plan_items (meal_plan_id);

CREATE TABLE IF NOT EXISTS grocery_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    meal_plan_id UUID NOT NULL REFERENCES meal_plans(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    quantity DOUBLE PRECISION NOT NULL,
    unit TEXT NOT NULL,
    category TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT grocery_items_quantity_check CHECK (quantity > 0)
);

CREATE INDEX IF NOT EXISTS idx_grocery_items_meal_plan_id
    ON grocery_items (meal_plan_id);

-- +goose Down
DROP TABLE IF EXISTS grocery_items;
DROP TABLE IF EXISTS meal_plan_items;
DROP TABLE IF EXISTS meal_plans;
DROP TABLE IF EXISTS recipes;
DROP TABLE IF EXISTS weekly_checkins;
