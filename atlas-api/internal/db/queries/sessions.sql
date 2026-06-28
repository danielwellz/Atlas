-- name: CreateSession :one
INSERT INTO sessions (
    id,
    user_id,
    refresh_token_hash,
    expires_at
) VALUES (
    $1,
    $2,
    $3,
    $4
)
RETURNING id, user_id, refresh_token_hash, expires_at, created_at, revoked_at;

-- name: GetActiveSessionByRefreshTokenHash :one
SELECT id, user_id, refresh_token_hash, expires_at, created_at, revoked_at
FROM sessions
WHERE refresh_token_hash = $1
  AND revoked_at IS NULL
  AND expires_at > NOW()
LIMIT 1;

-- name: RevokeSessionByID :execrows
UPDATE sessions
SET revoked_at = COALESCE(revoked_at, NOW())
WHERE id = $1;

-- name: RevokeSessionByRefreshTokenHash :execrows
UPDATE sessions
SET revoked_at = COALESCE(revoked_at, NOW())
WHERE refresh_token_hash = $1;
