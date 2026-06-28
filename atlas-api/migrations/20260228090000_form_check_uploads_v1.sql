-- +goose Up
CREATE TABLE IF NOT EXISTS form_check_uploads (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    movement_type TEXT NOT NULL,
    recording_started_at TIMESTAMPTZ NOT NULL,
    recording_ended_at TIMESTAMPTZ NOT NULL,
    summary_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    metadata_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    storage_key TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT form_check_uploads_movement_type_check CHECK (
        movement_type IN ('squat', 'hinge', 'lunge', 'push', 'pull')
    ),
    CONSTRAINT form_check_uploads_recording_window_check CHECK (
        recording_ended_at >= recording_started_at
    ),
    CONSTRAINT form_check_uploads_storage_key_not_empty_check CHECK (
        length(trim(storage_key)) > 0
    )
);

CREATE INDEX IF NOT EXISTS idx_form_check_uploads_user_created_at
    ON form_check_uploads (user_id, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS form_check_uploads;
