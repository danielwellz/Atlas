-- +goose Up
CREATE TABLE IF NOT EXISTS crews (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    created_by_user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    is_private BOOLEAN NOT NULL DEFAULT TRUE,
    shared_plan_url TEXT,
    shared_habits_url TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_crews_created_by_user_id
    ON crews (created_by_user_id);

CREATE TABLE IF NOT EXISTS crew_members (
    crew_id UUID NOT NULL REFERENCES crews(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role TEXT NOT NULL DEFAULT 'member',
    joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (crew_id, user_id),
    CONSTRAINT crew_members_role_check CHECK (role IN ('owner', 'member'))
);

CREATE INDEX IF NOT EXISTS idx_crew_members_user_id
    ON crew_members (user_id);

CREATE TABLE IF NOT EXISTS crew_invites (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    crew_id UUID NOT NULL REFERENCES crews(id) ON DELETE CASCADE,
    invite_code TEXT NOT NULL UNIQUE,
    invited_by_user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    max_uses INTEGER NOT NULL DEFAULT 1,
    uses_count INTEGER NOT NULL DEFAULT 0,
    expires_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT crew_invites_max_uses_check CHECK (max_uses > 0),
    CONSTRAINT crew_invites_uses_count_check CHECK (uses_count >= 0 AND uses_count <= max_uses)
);

CREATE INDEX IF NOT EXISTS idx_crew_invites_crew_id
    ON crew_invites (crew_id);

CREATE INDEX IF NOT EXISTS idx_crew_invites_invite_code
    ON crew_invites (invite_code);

CREATE TABLE IF NOT EXISTS coach_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    crew_id UUID NOT NULL REFERENCES crews(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    coach_name TEXT NOT NULL,
    duration_seconds INTEGER NOT NULL DEFAULT 0,
    published BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT coach_sessions_duration_seconds_check CHECK (duration_seconds >= 0)
);

CREATE INDEX IF NOT EXISTS idx_coach_sessions_crew_id
    ON coach_sessions (crew_id);

CREATE TABLE IF NOT EXISTS coach_session_assets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    coach_session_id UUID NOT NULL REFERENCES coach_sessions(id) ON DELETE CASCADE,
    asset_type TEXT NOT NULL,
    storage_key TEXT NOT NULL,
    mime_type TEXT NOT NULL DEFAULT 'video/mp4',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT coach_session_assets_asset_type_check CHECK (asset_type IN ('video', 'thumbnail', 'captions'))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_coach_session_assets_unique_type
    ON coach_session_assets (coach_session_id, asset_type);

CREATE TABLE IF NOT EXISTS cue_timeline (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    coach_session_id UUID NOT NULL REFERENCES coach_sessions(id) ON DELETE CASCADE,
    cue_index INTEGER NOT NULL,
    start_ms INTEGER NOT NULL,
    end_ms INTEGER NOT NULL,
    cue_text TEXT NOT NULL,
    biomechanics_exercise_id UUID REFERENCES exercises(id) ON DELETE SET NULL,
    biomechanics_definition_type TEXT NOT NULL DEFAULT 'muscle_group',
    biomechanics_definition_key TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT cue_timeline_time_range_check CHECK (start_ms >= 0 AND end_ms >= start_ms),
    CONSTRAINT cue_timeline_definition_type_check CHECK (
        biomechanics_definition_type IN ('muscle_group', 'joint_angle', 'movement_pattern')
    )
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_cue_timeline_session_cue_index
    ON cue_timeline (coach_session_id, cue_index);

CREATE INDEX IF NOT EXISTS idx_cue_timeline_session_start_ms
    ON cue_timeline (coach_session_id, start_ms);

-- +goose Down
DROP TABLE IF EXISTS cue_timeline;
DROP TABLE IF EXISTS coach_session_assets;
DROP TABLE IF EXISTS coach_sessions;
DROP TABLE IF EXISTS crew_invites;
DROP TABLE IF EXISTS crew_members;
DROP TABLE IF EXISTS crews;
