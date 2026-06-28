-- +goose Up
ALTER TABLE exercises
    ADD COLUMN IF NOT EXISTS movement_pattern_taxonomy JSONB NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN IF NOT EXISTS contraindication_tags JSONB NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN IF NOT EXISTS equipment_requirements JSONB NOT NULL DEFAULT '[]'::jsonb;

UPDATE exercises
SET movement_pattern_taxonomy = jsonb_build_array(movement_pattern)
WHERE jsonb_array_length(movement_pattern_taxonomy) = 0
  AND movement_pattern IS NOT NULL
  AND btrim(movement_pattern) <> '';

UPDATE exercises
SET contraindication_tags = contraindications
WHERE jsonb_array_length(contraindication_tags) = 0
  AND jsonb_typeof(contraindications) = 'array';

UPDATE exercises
SET equipment_requirements = equipment_json
WHERE jsonb_array_length(equipment_requirements) = 0
  AND jsonb_typeof(equipment_json) = 'array';

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'exercises_movement_pattern_taxonomy_array_check'
          AND conrelid = 'exercises'::regclass
    ) THEN
        ALTER TABLE exercises
            ADD CONSTRAINT exercises_movement_pattern_taxonomy_array_check
            CHECK (jsonb_typeof(movement_pattern_taxonomy) = 'array');
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'exercises_contraindication_tags_array_check'
          AND conrelid = 'exercises'::regclass
    ) THEN
        ALTER TABLE exercises
            ADD CONSTRAINT exercises_contraindication_tags_array_check
            CHECK (jsonb_typeof(contraindication_tags) = 'array');
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'exercises_equipment_requirements_array_check'
          AND conrelid = 'exercises'::regclass
    ) THEN
        ALTER TABLE exercises
            ADD CONSTRAINT exercises_equipment_requirements_array_check
            CHECK (jsonb_typeof(equipment_requirements) = 'array');
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_exercises_movement_pattern_taxonomy_gin ON exercises USING GIN (movement_pattern_taxonomy);
CREATE INDEX IF NOT EXISTS idx_exercises_contraindication_tags_gin ON exercises USING GIN (contraindication_tags);
CREATE INDEX IF NOT EXISTS idx_exercises_equipment_requirements_gin ON exercises USING GIN (equipment_requirements);

-- +goose Down
DROP INDEX IF EXISTS idx_exercises_equipment_requirements_gin;
DROP INDEX IF EXISTS idx_exercises_contraindication_tags_gin;
DROP INDEX IF EXISTS idx_exercises_movement_pattern_taxonomy_gin;

ALTER TABLE exercises
    DROP CONSTRAINT IF EXISTS exercises_equipment_requirements_array_check,
    DROP CONSTRAINT IF EXISTS exercises_contraindication_tags_array_check,
    DROP CONSTRAINT IF EXISTS exercises_movement_pattern_taxonomy_array_check,
    DROP COLUMN IF EXISTS equipment_requirements,
    DROP COLUMN IF EXISTS contraindication_tags,
    DROP COLUMN IF EXISTS movement_pattern_taxonomy;
