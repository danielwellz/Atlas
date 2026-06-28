-- name: CreateFormCheckUpload :one
INSERT INTO form_check_uploads (
    user_id,
    movement_type,
    recording_started_at,
    recording_ended_at,
    summary_json,
    metadata_json,
    storage_key,
    created_at
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7,
    NOW()
)
RETURNING
    id,
    user_id,
    movement_type,
    recording_started_at,
    recording_ended_at,
    summary_json,
    metadata_json,
    storage_key,
    created_at;
