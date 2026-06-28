-- +goose Up
CREATE TABLE IF NOT EXISTS habits (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    type TEXT NOT NULL,
    target_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT habits_name_non_empty_check CHECK (length(trim(name)) > 0),
    CONSTRAINT habits_type_non_empty_check CHECK (length(trim(type)) > 0),
    CONSTRAINT habits_target_object_check CHECK (jsonb_typeof(target_json) = 'object')
);

CREATE TABLE IF NOT EXISTS habit_daily_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    habit_id UUID NOT NULL REFERENCES habits(id) ON DELETE CASCADE,
    date DATE NOT NULL,
    completed BOOLEAN NOT NULL,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT habit_daily_logs_habit_id_date_unique UNIQUE (habit_id, date),
    CONSTRAINT habit_daily_logs_completion_consistency_check CHECK (
        (completed = TRUE AND completed_at IS NOT NULL) OR
        (completed = FALSE AND completed_at IS NULL)
    )
);

CREATE INDEX IF NOT EXISTS idx_habits_user_id ON habits (user_id);
CREATE INDEX IF NOT EXISTS idx_habits_user_id_active ON habits (user_id, active);
CREATE INDEX IF NOT EXISTS idx_habit_daily_logs_habit_id_date ON habit_daily_logs (habit_id, date DESC);

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION enforce_max_active_habits_per_user()
RETURNS TRIGGER AS $$
DECLARE
    active_count INTEGER;
BEGIN
    IF NEW.active IS DISTINCT FROM TRUE THEN
        RETURN NEW;
    END IF;

    PERFORM 1
    FROM habits
    WHERE user_id = NEW.user_id
    FOR UPDATE;

    SELECT COUNT(*) INTO active_count
    FROM habits
    WHERE user_id = NEW.user_id
      AND active = TRUE;

    IF TG_OP = 'UPDATE' AND OLD.active = TRUE THEN
        active_count := active_count - 1;
    END IF;

    IF active_count >= 3 THEN
        RAISE EXCEPTION 'max 3 active habits per user'
            USING ERRCODE = '23514';
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

DROP TRIGGER IF EXISTS trg_enforce_max_active_habits_per_user ON habits;
CREATE TRIGGER trg_enforce_max_active_habits_per_user
BEFORE INSERT OR UPDATE OF active ON habits
FOR EACH ROW
EXECUTE FUNCTION enforce_max_active_habits_per_user();

-- +goose Down
DROP TRIGGER IF EXISTS trg_enforce_max_active_habits_per_user ON habits;
DROP FUNCTION IF EXISTS enforce_max_active_habits_per_user();
DROP TABLE IF EXISTS habit_daily_logs;
DROP TABLE IF EXISTS habits;
