-- name: ListConsentsByUserID :many
SELECT id, user_id, consent_type, granted_at, revoked_at, metadata_json
FROM consents
WHERE user_id = $1
ORDER BY consent_type ASC;

-- name: HasActiveConsent :one
SELECT EXISTS (
    SELECT 1
    FROM consents
    WHERE user_id = $1
      AND consent_type = $2
      AND revoked_at IS NULL
) AS has_active_consent;

-- name: UpsertConsent :one
INSERT INTO consents (
    user_id,
    consent_type,
    granted_at,
    revoked_at,
    metadata_json
) VALUES (
    $1,
    $2,
    NOW(),
    NULL,
    $3
)
ON CONFLICT (user_id, consent_type)
DO UPDATE SET
    granted_at = NOW(),
    revoked_at = NULL,
    metadata_json = EXCLUDED.metadata_json
RETURNING id, user_id, consent_type, granted_at, revoked_at, metadata_json;

-- name: RevokeConsent :one
UPDATE consents
SET revoked_at = NOW()
WHERE user_id = $1
  AND consent_type = $2
  AND revoked_at IS NULL
RETURNING id, user_id, consent_type, granted_at, revoked_at, metadata_json;
