-- +goose Up
CREATE TABLE IF NOT EXISTS app_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    event_name TEXT NOT NULL,
    event_time TIMESTAMPTZ NOT NULL,
    properties_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT app_events_event_name_check CHECK (
        event_name IN (
            'onboarding_started',
            'onboarding_goal_selected',
            'onboarding_equipment_selected',
            'onboarding_schedule_selected',
            'onboarding_completed',
            'workout_completed'
        )
    ),
    CONSTRAINT app_events_properties_json_object_check CHECK (jsonb_typeof(properties_json) = 'object')
);

CREATE INDEX IF NOT EXISTS idx_app_events_event_time
    ON app_events (event_time DESC);

CREATE INDEX IF NOT EXISTS idx_app_events_event_name_event_time
    ON app_events (event_name, event_time DESC);

CREATE INDEX IF NOT EXISTS idx_app_events_user_id_event_time
    ON app_events (user_id, event_time DESC)
    WHERE user_id IS NOT NULL;

-- +goose Down
DROP TABLE IF EXISTS app_events;
