-- name: CreateCrew :one
INSERT INTO crews (
    name,
    description,
    created_by_user_id,
    is_private,
    shared_plan_url,
    shared_habits_url
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6
)
RETURNING
    id,
    name,
    description,
    created_by_user_id,
    is_private,
    shared_plan_url,
    shared_habits_url,
    created_at,
    updated_at;

-- name: GetCrewByID :one
SELECT
    id,
    name,
    description,
    created_by_user_id,
    is_private,
    shared_plan_url,
    shared_habits_url,
    created_at,
    updated_at
FROM crews
WHERE id = $1
LIMIT 1;

-- name: AddCrewMember :one
INSERT INTO crew_members (
    crew_id,
    user_id,
    role
) VALUES (
    $1,
    $2,
    $3
)
ON CONFLICT (crew_id, user_id)
DO UPDATE SET
    role = crew_members.role
RETURNING crew_id, user_id, role, joined_at;

-- name: GetCrewMemberByCrewIDAndUserID :one
SELECT crew_id, user_id, role, joined_at
FROM crew_members
WHERE crew_id = $1
  AND user_id = $2
LIMIT 1;

-- name: ListCrewsByUserID :many
SELECT
    c.id,
    c.name,
    c.description,
    c.created_by_user_id,
    c.is_private,
    c.shared_plan_url,
    c.shared_habits_url,
    c.created_at,
    c.updated_at,
    cm.role AS my_role,
    COUNT(cm_all.user_id)::int AS member_count
FROM crew_members cm
JOIN crews c ON c.id = cm.crew_id
JOIN crew_members cm_all ON cm_all.crew_id = c.id
WHERE cm.user_id = $1
GROUP BY
    c.id,
    c.name,
    c.description,
    c.created_by_user_id,
    c.is_private,
    c.shared_plan_url,
    c.shared_habits_url,
    c.created_at,
    c.updated_at,
    cm.role
ORDER BY c.created_at DESC;

-- name: GetCrewByIDForUser :one
SELECT
    c.id,
    c.name,
    c.description,
    c.created_by_user_id,
    c.is_private,
    c.shared_plan_url,
    c.shared_habits_url,
    c.created_at,
    c.updated_at,
    cm.role AS my_role,
    COUNT(cm_all.user_id)::int AS member_count
FROM crews c
JOIN crew_members cm ON cm.crew_id = c.id
JOIN crew_members cm_all ON cm_all.crew_id = c.id
WHERE c.id = $1
  AND cm.user_id = $2
GROUP BY
    c.id,
    c.name,
    c.description,
    c.created_by_user_id,
    c.is_private,
    c.shared_plan_url,
    c.shared_habits_url,
    c.created_at,
    c.updated_at,
    cm.role
LIMIT 1;

-- name: ListCrewMembersByCrewID :many
SELECT
    cm.crew_id,
    cm.user_id,
    cm.role,
    cm.joined_at,
    u.email,
    COALESCE(up.display_name, '') AS display_name
FROM crew_members cm
JOIN users u ON u.id = cm.user_id
LEFT JOIN user_profiles up ON up.user_id = cm.user_id
WHERE cm.crew_id = $1
ORDER BY
    CASE WHEN cm.role = 'owner' THEN 0 ELSE 1 END,
    cm.joined_at ASC;

-- name: CreateCrewInvite :one
INSERT INTO crew_invites (
    crew_id,
    invite_code,
    invited_by_user_id,
    max_uses,
    expires_at
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5
)
RETURNING
    id,
    crew_id,
    invite_code,
    invited_by_user_id,
    max_uses,
    uses_count,
    expires_at,
    revoked_at,
    created_at;

-- name: GetActiveCrewInviteByCode :one
SELECT
    id,
    crew_id,
    invite_code,
    invited_by_user_id,
    max_uses,
    uses_count,
    expires_at,
    revoked_at,
    created_at
FROM crew_invites
WHERE invite_code = $1
  AND revoked_at IS NULL
  AND (expires_at IS NULL OR expires_at > NOW())
  AND uses_count < max_uses
LIMIT 1;

-- name: IncrementCrewInviteUse :one
UPDATE crew_invites
SET uses_count = uses_count + 1
WHERE id = $1
  AND revoked_at IS NULL
  AND (expires_at IS NULL OR expires_at > NOW())
  AND uses_count < max_uses
RETURNING
    id,
    crew_id,
    invite_code,
    invited_by_user_id,
    max_uses,
    uses_count,
    expires_at,
    revoked_at,
    created_at;

-- name: ListCoachSessionsByUserID :many
SELECT
    cs.id,
    cs.crew_id,
    cs.title,
    cs.description,
    cs.coach_name,
    cs.duration_seconds,
    cs.required_tier,
    cs.published,
    cs.created_at,
    cs.updated_at,
    c.name AS crew_name
FROM coach_sessions cs
JOIN crews c ON c.id = cs.crew_id
JOIN crew_members cm ON cm.crew_id = cs.crew_id
WHERE cm.user_id = $1
  AND cs.published = TRUE
ORDER BY cs.created_at DESC;

-- name: GetCoachSessionByIDForUser :one
SELECT
    cs.id,
    cs.crew_id,
    cs.title,
    cs.description,
    cs.coach_name,
    cs.duration_seconds,
    cs.required_tier,
    cs.published,
    cs.created_at,
    cs.updated_at,
    c.name AS crew_name
FROM coach_sessions cs
JOIN crews c ON c.id = cs.crew_id
JOIN crew_members cm ON cm.crew_id = cs.crew_id
WHERE cs.id = $1
  AND cm.user_id = $2
  AND cs.published = TRUE
LIMIT 1;

-- name: GetCoachSessionByID :one
SELECT
    id,
    crew_id,
    title,
    description,
    coach_name,
    duration_seconds,
    required_tier,
    published,
    created_at,
    updated_at
FROM coach_sessions
WHERE id = $1
  AND published = TRUE
LIMIT 1;

-- name: ListCoachSessionAssetsBySessionID :many
SELECT
    id,
    coach_session_id,
    asset_type,
    storage_key,
    mime_type,
    created_at
FROM coach_session_assets
WHERE coach_session_id = $1
ORDER BY created_at ASC, id ASC;

-- name: ListCueTimelineBySessionID :many
SELECT
    id,
    coach_session_id,
    cue_index,
    start_ms,
    end_ms,
    cue_text,
    biomechanics_exercise_id,
    biomechanics_definition_type,
    biomechanics_definition_key,
    created_at
FROM cue_timeline
WHERE coach_session_id = $1
ORDER BY cue_index ASC, start_ms ASC, id ASC;
