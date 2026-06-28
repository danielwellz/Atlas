-- +goose Up
CREATE TABLE IF NOT EXISTS momentum_sprint_enrollments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    goal TEXT NOT NULL,
    start_date DATE NOT NULL,
    end_date DATE NOT NULL,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT momentum_sprint_goal_non_empty_check CHECK (length(trim(goal)) > 0),
    CONSTRAINT momentum_sprint_date_range_check CHECK (end_date >= start_date)
);

CREATE TABLE IF NOT EXISTS momentum_sprint_daily_checklist_entries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    enrollment_id UUID NOT NULL REFERENCES momentum_sprint_enrollments(id) ON DELETE CASCADE,
    date DATE NOT NULL,
    habit_key TEXT NOT NULL,
    habit_label TEXT NOT NULL,
    display_order INTEGER NOT NULL,
    completed BOOLEAN NOT NULL DEFAULT FALSE,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT momentum_sprint_daily_checklist_entries_unique UNIQUE (enrollment_id, date, habit_key),
    CONSTRAINT momentum_sprint_habit_key_non_empty_check CHECK (length(trim(habit_key)) > 0),
    CONSTRAINT momentum_sprint_habit_label_non_empty_check CHECK (length(trim(habit_label)) > 0),
    CONSTRAINT momentum_sprint_display_order_non_negative_check CHECK (display_order >= 0),
    CONSTRAINT momentum_sprint_daily_completion_consistency_check CHECK (
        (completed = TRUE AND completed_at IS NOT NULL) OR
        (completed = FALSE AND completed_at IS NULL)
    )
);

CREATE TABLE IF NOT EXISTS momentum_sprint_reward_milestones (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    enrollment_id UUID NOT NULL REFERENCES momentum_sprint_enrollments(id) ON DELETE CASCADE,
    milestone_day INTEGER NOT NULL,
    reward_label TEXT NOT NULL,
    unlocked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT momentum_sprint_reward_milestones_unique UNIQUE (enrollment_id, milestone_day),
    CONSTRAINT momentum_sprint_milestone_day_check CHECK (milestone_day BETWEEN 1 AND 14),
    CONSTRAINT momentum_sprint_reward_label_non_empty_check CHECK (length(trim(reward_label)) > 0)
);

CREATE INDEX IF NOT EXISTS idx_momentum_sprint_enrollments_user_id
    ON momentum_sprint_enrollments (user_id);
CREATE INDEX IF NOT EXISTS idx_momentum_sprint_daily_checklist_entries_enrollment_date
    ON momentum_sprint_daily_checklist_entries (enrollment_id, date);
CREATE INDEX IF NOT EXISTS idx_momentum_sprint_reward_milestones_enrollment_day
    ON momentum_sprint_reward_milestones (enrollment_id, milestone_day);

-- +goose Down
DROP TABLE IF EXISTS momentum_sprint_reward_milestones;
DROP TABLE IF EXISTS momentum_sprint_daily_checklist_entries;
DROP TABLE IF EXISTS momentum_sprint_enrollments;
