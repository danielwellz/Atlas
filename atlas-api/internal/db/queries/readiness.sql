-- name: UpsertReadinessCheckin :one
INSERT INTO readiness_checkins (
    user_id,
    date,
    energy_level,
    sleep_quality,
    stress_level,
    readiness_score
) VALUES (
    sqlc.arg(user_id),
    sqlc.arg(date),
    sqlc.arg(energy_level),
    sqlc.arg(sleep_quality),
    sqlc.arg(stress_level),
    (
        (
            sqlc.arg(energy_level)::integer
            + sqlc.arg(sleep_quality)::integer
            + (4 - sqlc.arg(stress_level)::integer)
        )::double precision / 3.0
    )::double precision
)
ON CONFLICT (user_id, date)
DO UPDATE SET
    energy_level = EXCLUDED.energy_level,
    sleep_quality = EXCLUDED.sleep_quality,
    stress_level = EXCLUDED.stress_level,
    readiness_score = EXCLUDED.readiness_score,
    updated_at = NOW()
RETURNING
    user_id,
    date,
    energy_level,
    sleep_quality,
    stress_level,
    readiness_score,
    created_at,
    updated_at;
