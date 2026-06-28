-- +goose Up
ALTER TABLE exercises
    ADD COLUMN IF NOT EXISTS movement_pattern TEXT,
    ADD COLUMN IF NOT EXISTS primary_muscles JSONB NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN IF NOT EXISTS secondary_muscles JSONB NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN IF NOT EXISTS contraindications JSONB NOT NULL DEFAULT '[]'::jsonb;

UPDATE exercises
SET movement_pattern = 'core'
WHERE movement_pattern IS NULL
   OR btrim(movement_pattern) = '';

ALTER TABLE exercises
    ALTER COLUMN movement_pattern SET NOT NULL;

UPDATE exercises
SET primary_muscles = jsonb_build_array(primary_muscle_group)
WHERE jsonb_array_length(primary_muscles) = 0
  AND primary_muscle_group IS NOT NULL
  AND btrim(primary_muscle_group) <> '';

UPDATE exercises
SET secondary_muscles = secondary_muscles_json
WHERE jsonb_array_length(secondary_muscles) = 0
  AND jsonb_typeof(secondary_muscles_json) = 'array';

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'exercises_primary_muscles_array_check'
          AND conrelid = 'exercises'::regclass
    ) THEN
        ALTER TABLE exercises
            ADD CONSTRAINT exercises_primary_muscles_array_check CHECK (jsonb_typeof(primary_muscles) = 'array');
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'exercises_secondary_muscles_array_v2_check'
          AND conrelid = 'exercises'::regclass
    ) THEN
        ALTER TABLE exercises
            ADD CONSTRAINT exercises_secondary_muscles_array_v2_check CHECK (jsonb_typeof(secondary_muscles) = 'array');
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'exercises_contraindications_array_check'
          AND conrelid = 'exercises'::regclass
    ) THEN
        ALTER TABLE exercises
            ADD CONSTRAINT exercises_contraindications_array_check CHECK (jsonb_typeof(contraindications) = 'array');
    END IF;
END $$;

-- +goose Down
ALTER TABLE exercises
    DROP CONSTRAINT IF EXISTS exercises_contraindications_array_check,
    DROP CONSTRAINT IF EXISTS exercises_secondary_muscles_array_v2_check,
    DROP CONSTRAINT IF EXISTS exercises_primary_muscles_array_check,
    DROP COLUMN IF EXISTS contraindications,
    DROP COLUMN IF EXISTS secondary_muscles,
    DROP COLUMN IF EXISTS primary_muscles;
