CREATE TABLE users (
    id UUID PRIMARY KEY,
    email TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE sessions (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    refresh_token_hash TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ
);

CREATE TABLE consents (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    consent_type TEXT NOT NULL,
    granted_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    metadata_json JSONB NOT NULL,
    CONSTRAINT consents_user_id_consent_type_unique UNIQUE (user_id, consent_type)
);

CREATE TABLE app_events (
    id UUID PRIMARY KEY,
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    event_name TEXT NOT NULL,
    event_time TIMESTAMPTZ NOT NULL,
    properties_json JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE exercises (
    id UUID PRIMARY KEY,
    slug TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    primary_muscle_group TEXT NOT NULL,
    secondary_muscles_json JSONB NOT NULL,
    movement_pattern TEXT NOT NULL,
    movement_pattern_taxonomy JSONB NOT NULL,
    primary_muscles JSONB NOT NULL,
    secondary_muscles JSONB NOT NULL,
    contraindications JSONB NOT NULL,
    contraindication_tags JSONB NOT NULL,
    equipment_json JSONB NOT NULL,
    equipment_requirements JSONB NOT NULL,
    difficulty TEXT NOT NULL,
    description TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE exercise_media (
    id UUID PRIMARY KEY,
    exercise_id UUID NOT NULL REFERENCES exercises(id) ON DELETE CASCADE,
    media_type TEXT NOT NULL,
    uri TEXT NOT NULL,
    thumbnail_uri TEXT,
    duration_seconds INTEGER,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE exercise_biomech_assets (
    id UUID PRIMARY KEY,
    exercise_id UUID NOT NULL UNIQUE REFERENCES exercises(id) ON DELETE CASCADE,
    animation_asset_key TEXT NOT NULL,
    rig_version TEXT NOT NULL,
    metadata_json JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE muscle_groups (
    slug TEXT PRIMARY KEY,
    display_name TEXT NOT NULL,
    region TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE exercise_biomech_asset_muscle_groups (
    biomech_asset_id UUID NOT NULL REFERENCES exercise_biomech_assets(id) ON DELETE CASCADE,
    muscle_group_slug TEXT NOT NULL REFERENCES muscle_groups(slug) ON DELETE CASCADE,
    activation_level DOUBLE PRECISION NOT NULL,
    role TEXT NOT NULL,
    PRIMARY KEY (biomech_asset_id, muscle_group_slug, role)
);

CREATE TABLE user_profiles (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    display_name TEXT NOT NULL,
    sex TEXT NOT NULL,
    height_cm INTEGER NOT NULL,
    weight_kg DOUBLE PRECISION NOT NULL,
    experience_level TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE user_goals (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    primary_goal TEXT NOT NULL,
    secondary_goal TEXT,
    days_per_week INTEGER NOT NULL,
    session_duration_minutes INTEGER NOT NULL,
    equipment_access_json JSONB NOT NULL,
    constraints_json JSONB NOT NULL,
    injuries_limitations_json JSONB NOT NULL,
    modality_preferences_json JSONB NOT NULL,
    prior_training_history_json JSONB,
    readiness_signals_json JSONB,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE programs (
    id UUID PRIMARY KEY,
    slug TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    description TEXT NOT NULL,
    goal_tags_json JSONB NOT NULL,
    level TEXT NOT NULL,
    weeks_length INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE program_weeks (
    id UUID PRIMARY KEY,
    program_id UUID NOT NULL REFERENCES programs(id) ON DELETE CASCADE,
    week_index INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE program_sessions (
    id UUID PRIMARY KEY,
    program_week_id UUID NOT NULL REFERENCES program_weeks(id) ON DELETE CASCADE,
    day_of_week INTEGER NOT NULL,
    name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE program_session_exercises (
    id UUID PRIMARY KEY,
    program_session_id UUID NOT NULL REFERENCES program_sessions(id) ON DELETE CASCADE,
    exercise_id UUID NOT NULL REFERENCES exercises(id),
    prescription_json JSONB NOT NULL,
    order_index INTEGER NOT NULL
);

CREATE TABLE user_program_enrollments (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    program_id UUID NOT NULL REFERENCES programs(id),
    start_date DATE NOT NULL,
    current_week INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE user_program_state (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    program_id UUID NOT NULL REFERENCES programs(id),
    current_week INTEGER NOT NULL,
    deload_flag BOOLEAN NOT NULL,
    last_week_adherence DOUBLE PRECISION NOT NULL,
    last_week_scheduled_sessions INTEGER NOT NULL,
    last_week_completed_sessions INTEGER NOT NULL,
    last_week_density DOUBLE PRECISION NOT NULL,
    last_week_high_rpe_rate DOUBLE PRECISION NOT NULL,
    fatigue_score DOUBLE PRECISION NOT NULL,
    consecutive_low_adherence_weeks INTEGER NOT NULL,
    adjustment_reasons_json JSONB NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE workouts (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    program_session_id UUID REFERENCES program_sessions(id) ON DELETE SET NULL,
    started_at TIMESTAMPTZ NOT NULL,
    completed_at TIMESTAMPTZ,
    notes TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE workout_exercises (
    id UUID PRIMARY KEY,
    workout_id UUID NOT NULL REFERENCES workouts(id) ON DELETE CASCADE,
    exercise_id UUID NOT NULL REFERENCES exercises(id),
    order_index INTEGER NOT NULL,
    planned_json JSONB NOT NULL,
    actual_json JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE user_exercise_progress (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    exercise_id UUID NOT NULL REFERENCES exercises(id),
    last_load DOUBLE PRECISION NOT NULL,
    last_reps INTEGER NOT NULL,
    last_rpe DOUBLE PRECISION,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE workout_sets (
    id UUID PRIMARY KEY,
    workout_exercise_id UUID NOT NULL REFERENCES workout_exercises(id) ON DELETE CASCADE,
    set_index INTEGER NOT NULL,
    reps INTEGER NOT NULL,
    weight_kg DOUBLE PRECISION NOT NULL,
    rpe DOUBLE PRECISION,
    completed_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    idempotency_key TEXT
);

CREATE TABLE habits (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    type TEXT NOT NULL,
    target_json JSONB NOT NULL,
    active BOOLEAN NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE habit_daily_logs (
    id UUID PRIMARY KEY,
    habit_id UUID NOT NULL REFERENCES habits(id) ON DELETE CASCADE,
    date DATE NOT NULL,
    completed BOOLEAN NOT NULL,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE momentum_sprint_enrollments (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    goal TEXT NOT NULL,
    start_date DATE NOT NULL,
    end_date DATE NOT NULL,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE momentum_sprint_daily_checklist_entries (
    id UUID PRIMARY KEY,
    enrollment_id UUID NOT NULL REFERENCES momentum_sprint_enrollments(id) ON DELETE CASCADE,
    date DATE NOT NULL,
    habit_key TEXT NOT NULL,
    habit_label TEXT NOT NULL,
    display_order INTEGER NOT NULL,
    completed BOOLEAN NOT NULL,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE momentum_sprint_reward_milestones (
    id UUID PRIMARY KEY,
    enrollment_id UUID NOT NULL REFERENCES momentum_sprint_enrollments(id) ON DELETE CASCADE,
    milestone_day INTEGER NOT NULL,
    reward_label TEXT NOT NULL,
    unlocked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE nutrition_targets (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    calories_target INTEGER NOT NULL,
    protein_g_target INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE nutrition_daily_checkins (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    date DATE NOT NULL,
    calories_estimate INTEGER,
    protein_g_estimate INTEGER,
    hit_calories BOOLEAN NOT NULL,
    hit_protein BOOLEAN NOT NULL,
    notes TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE foods (
    id UUID PRIMARY KEY,
    external_id TEXT NOT NULL,
    provider TEXT NOT NULL,
    label TEXT NOT NULL,
    brand TEXT NOT NULL,
    nutrients_json JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT foods_provider_external_id_unique UNIQUE (provider, external_id)
);

CREATE TABLE food_logs (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    datetime TIMESTAMPTZ NOT NULL,
    food_id UUID NOT NULL REFERENCES foods(id) ON DELETE RESTRICT,
    quantity DOUBLE PRECISION NOT NULL,
    unit TEXT NOT NULL,
    nutrients_snapshot_json JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT food_logs_quantity_check CHECK (quantity > 0)
);

CREATE TABLE user_weight_entries (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    date DATE NOT NULL,
    weight_kg DOUBLE PRECISION NOT NULL,
    unit TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (user_id, date)
);

CREATE TABLE readiness_checkins (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    date DATE NOT NULL,
    energy_level INTEGER NOT NULL,
    sleep_quality INTEGER NOT NULL,
    stress_level INTEGER NOT NULL,
    readiness_score DOUBLE PRECISION NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT readiness_checkins_energy_level_range_check CHECK (energy_level BETWEEN 1 AND 3),
    CONSTRAINT readiness_checkins_sleep_quality_range_check CHECK (sleep_quality BETWEEN 1 AND 3),
    CONSTRAINT readiness_checkins_stress_level_range_check CHECK (stress_level BETWEEN 1 AND 3),
    CONSTRAINT readiness_checkins_readiness_score_range_check CHECK (readiness_score >= 0),
    PRIMARY KEY (user_id, date)
);

CREATE TABLE weekly_checkins (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    week_start DATE NOT NULL,
    adherence DOUBLE PRECISION NOT NULL,
    weight_change DOUBLE PRECISION NOT NULL,
    new_targets_json JSONB NOT NULL,
    explanation TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (user_id, week_start)
);

CREATE TABLE recipes (
    id UUID PRIMARY KEY,
    slug TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    meal_type TEXT NOT NULL,
    description TEXT NOT NULL,
    servings DOUBLE PRECISION NOT NULL,
    calories_kcal INTEGER NOT NULL,
    protein_g INTEGER NOT NULL,
    carbs_g INTEGER NOT NULL,
    fat_g INTEGER NOT NULL,
    ingredients_json JSONB NOT NULL,
    instructions_json JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE meal_plans (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    week_start DATE NOT NULL,
    target_json JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT meal_plans_user_week_start_unique UNIQUE (user_id, week_start)
);

CREATE TABLE meal_plan_items (
    id UUID PRIMARY KEY,
    meal_plan_id UUID NOT NULL REFERENCES meal_plans(id) ON DELETE CASCADE,
    day_of_week INTEGER NOT NULL,
    meal_slot TEXT NOT NULL,
    recipe_id UUID NOT NULL REFERENCES recipes(id) ON DELETE RESTRICT,
    servings DOUBLE PRECISION NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT meal_plan_items_unique_slot UNIQUE (meal_plan_id, day_of_week, meal_slot)
);

CREATE TABLE grocery_items (
    id UUID PRIMARY KEY,
    meal_plan_id UUID NOT NULL REFERENCES meal_plans(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    quantity DOUBLE PRECISION NOT NULL,
    unit TEXT NOT NULL,
    category TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE crews (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT NOT NULL,
    created_by_user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    is_private BOOLEAN NOT NULL,
    shared_plan_url TEXT,
    shared_habits_url TEXT,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE crew_members (
    crew_id UUID NOT NULL REFERENCES crews(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role TEXT NOT NULL,
    joined_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (crew_id, user_id)
);

CREATE TABLE crew_invites (
    id UUID PRIMARY KEY,
    crew_id UUID NOT NULL REFERENCES crews(id) ON DELETE CASCADE,
    invite_code TEXT NOT NULL UNIQUE,
    invited_by_user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    max_uses INTEGER NOT NULL,
    uses_count INTEGER NOT NULL,
    expires_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE coach_sessions (
    id UUID PRIMARY KEY,
    crew_id UUID NOT NULL REFERENCES crews(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    description TEXT NOT NULL,
    coach_name TEXT NOT NULL,
    duration_seconds INTEGER NOT NULL,
    required_tier TEXT NOT NULL,
    published BOOLEAN NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE coach_session_assets (
    id UUID PRIMARY KEY,
    coach_session_id UUID NOT NULL REFERENCES coach_sessions(id) ON DELETE CASCADE,
    asset_type TEXT NOT NULL,
    storage_key TEXT NOT NULL,
    mime_type TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE form_check_uploads (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    movement_type TEXT NOT NULL,
    recording_started_at TIMESTAMPTZ NOT NULL,
    recording_ended_at TIMESTAMPTZ NOT NULL,
    summary_json JSONB NOT NULL,
    metadata_json JSONB NOT NULL,
    storage_key TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE cue_timeline (
    id UUID PRIMARY KEY,
    coach_session_id UUID NOT NULL REFERENCES coach_sessions(id) ON DELETE CASCADE,
    cue_index INTEGER NOT NULL,
    start_ms INTEGER NOT NULL,
    end_ms INTEGER NOT NULL,
    cue_text TEXT NOT NULL,
    biomechanics_exercise_id UUID REFERENCES exercises(id) ON DELETE SET NULL,
    biomechanics_definition_type TEXT NOT NULL,
    biomechanics_definition_key TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE subscriptions (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    platform TEXT NOT NULL,
    product_id TEXT NOT NULL,
    status TEXT NOT NULL,
    expires_at TIMESTAMPTZ,
    raw_receipt JSONB NOT NULL,
    transaction_id TEXT NOT NULL,
    original_transaction_id TEXT,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE subscription_product_entitlements (
    product_id TEXT NOT NULL,
    entitlement TEXT NOT NULL,
    PRIMARY KEY (product_id, entitlement)
);

CREATE VIEW user_entitlements AS
SELECT
    s.user_id,
    spe.entitlement,
    MAX(s.expires_at) AS expires_at
FROM subscriptions s
JOIN subscription_product_entitlements spe ON spe.product_id = s.product_id
WHERE s.status IN ('active', 'grace_period')
  AND (s.expires_at IS NULL OR s.expires_at > NOW())
GROUP BY s.user_id, spe.entitlement;
