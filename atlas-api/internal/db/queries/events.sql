-- name: CreateAppEvent :one
INSERT INTO app_events (
    user_id,
    event_name,
    event_time,
    properties_json,
    created_at
) VALUES (
    $1,
    $2,
    $3,
    $4,
    NOW()
)
RETURNING id, user_id, event_name, event_time, properties_json, created_at;
