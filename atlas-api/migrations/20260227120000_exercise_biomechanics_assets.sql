-- +goose Up
CREATE TABLE IF NOT EXISTS exercise_biomech_assets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    exercise_id UUID NOT NULL UNIQUE REFERENCES exercises(id) ON DELETE CASCADE,
    animation_asset_key TEXT NOT NULL,
    rig_version TEXT NOT NULL,
    metadata_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT exercise_biomech_assets_metadata_object_check CHECK (jsonb_typeof(metadata_json) = 'object')
);

CREATE INDEX IF NOT EXISTS idx_exercise_biomech_assets_rig_version
    ON exercise_biomech_assets (rig_version);

CREATE TABLE IF NOT EXISTS muscle_groups (
    slug TEXT PRIMARY KEY,
    display_name TEXT NOT NULL,
    region TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS exercise_biomech_asset_muscle_groups (
    biomech_asset_id UUID NOT NULL REFERENCES exercise_biomech_assets(id) ON DELETE CASCADE,
    muscle_group_slug TEXT NOT NULL REFERENCES muscle_groups(slug) ON DELETE CASCADE,
    activation_level DOUBLE PRECISION NOT NULL DEFAULT 1.0,
    role TEXT NOT NULL DEFAULT 'primary',
    PRIMARY KEY (biomech_asset_id, muscle_group_slug, role),
    CONSTRAINT exercise_biomech_asset_muscle_groups_activation_check CHECK (activation_level >= 0 AND activation_level <= 1),
    CONSTRAINT exercise_biomech_asset_muscle_groups_role_check CHECK (role IN ('primary', 'secondary', 'stabilizer'))
);

CREATE INDEX IF NOT EXISTS idx_exercise_biomech_asset_muscle_groups_asset
    ON exercise_biomech_asset_muscle_groups (biomech_asset_id);

INSERT INTO muscle_groups (slug, display_name, region)
VALUES
    ('quads', 'Quadriceps', 'lower_body'),
    ('glutes', 'Glutes', 'lower_body'),
    ('hamstrings', 'Hamstrings', 'lower_body'),
    ('chest', 'Chest', 'upper_body'),
    ('upper_back', 'Upper Back', 'upper_body'),
    ('lats', 'Lats', 'upper_body'),
    ('shoulders', 'Shoulders', 'upper_body'),
    ('biceps', 'Biceps', 'upper_body'),
    ('triceps', 'Triceps', 'upper_body'),
    ('core', 'Core', 'trunk')
ON CONFLICT (slug) DO UPDATE
SET
    display_name = EXCLUDED.display_name,
    region = EXCLUDED.region;

-- +goose Down
DROP TABLE IF EXISTS exercise_biomech_asset_muscle_groups;
DROP TABLE IF EXISTS muscle_groups;
DROP TABLE IF EXISTS exercise_biomech_assets;
