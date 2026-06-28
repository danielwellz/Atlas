-- name: UpsertSubscription :one
INSERT INTO subscriptions (
    user_id,
    platform,
    product_id,
    status,
    expires_at,
    raw_receipt,
    transaction_id,
    original_transaction_id
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7,
    $8
)
ON CONFLICT (user_id, platform, transaction_id)
DO UPDATE SET
    product_id = EXCLUDED.product_id,
    status = EXCLUDED.status,
    expires_at = EXCLUDED.expires_at,
    raw_receipt = EXCLUDED.raw_receipt,
    original_transaction_id = COALESCE(EXCLUDED.original_transaction_id, subscriptions.original_transaction_id),
    updated_at = NOW()
RETURNING
    id,
    user_id,
    platform,
    product_id,
    status,
    expires_at,
    raw_receipt,
    transaction_id,
    original_transaction_id,
    created_at,
    updated_at;

-- name: ListSubscriptionsByUserID :many
SELECT
    id,
    user_id,
    platform,
    product_id,
    status,
    expires_at,
    raw_receipt,
    transaction_id,
    original_transaction_id,
    created_at,
    updated_at
FROM subscriptions
WHERE user_id = $1
ORDER BY created_at DESC;

-- name: ListUserEntitlements :many
SELECT entitlement
FROM user_entitlements
WHERE user_id = $1
ORDER BY entitlement;
