-- name: UpsertMomentumSprintEnrollment :one
INSERT INTO momentum_sprint_enrollments (
    user_id,
    goal,
    start_date,
    end_date,
    completed_at
) VALUES (
    $1,
    $2,
    $3,
    $4,
    NULL
)
ON CONFLICT (user_id) DO UPDATE
SET goal = EXCLUDED.goal,
    start_date = EXCLUDED.start_date,
    end_date = EXCLUDED.end_date,
    completed_at = NULL,
    created_at = NOW()
RETURNING id, user_id, goal, start_date, end_date, completed_at, created_at;

-- name: GetMomentumSprintEnrollmentByUserID :one
SELECT id, user_id, goal, start_date, end_date, completed_at, created_at
FROM momentum_sprint_enrollments
WHERE user_id = $1
LIMIT 1;

-- name: UpdateMomentumSprintEnrollmentCompletedAt :one
UPDATE momentum_sprint_enrollments
SET completed_at = $2
WHERE id = $1
RETURNING id, user_id, goal, start_date, end_date, completed_at, created_at;

-- name: CreateMomentumSprintChecklistEntry :one
INSERT INTO momentum_sprint_daily_checklist_entries (
    enrollment_id,
    date,
    habit_key,
    habit_label,
    display_order,
    completed,
    completed_at
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7
)
RETURNING id, enrollment_id, date, habit_key, habit_label, display_order, completed, completed_at, created_at;

-- name: DeleteMomentumSprintChecklistEntriesByEnrollmentID :exec
DELETE FROM momentum_sprint_daily_checklist_entries
WHERE enrollment_id = $1;

-- name: ListMomentumSprintChecklistEntriesByEnrollmentIDAndDate :many
SELECT id, enrollment_id, date, habit_key, habit_label, display_order, completed, completed_at, created_at
FROM momentum_sprint_daily_checklist_entries
WHERE enrollment_id = $1
  AND date = $2
ORDER BY display_order ASC, habit_key ASC;

-- name: UpdateMomentumSprintChecklistEntryCompletion :one
UPDATE momentum_sprint_daily_checklist_entries
SET completed = $4,
    completed_at = $5
WHERE enrollment_id = $1
  AND date = $2
  AND habit_key = $3
RETURNING id, enrollment_id, date, habit_key, habit_label, display_order, completed, completed_at, created_at;

-- name: ListMomentumSprintDailySummaryByEnrollmentID :many
SELECT
    date,
    COUNT(*)::int AS total_entries,
    COUNT(*) FILTER (WHERE completed)::int AS completed_entries
FROM momentum_sprint_daily_checklist_entries
WHERE enrollment_id = $1
GROUP BY date
ORDER BY date ASC;

-- name: CreateMomentumSprintRewardMilestone :one
INSERT INTO momentum_sprint_reward_milestones (
    enrollment_id,
    milestone_day,
    reward_label,
    unlocked_at
) VALUES (
    $1,
    $2,
    $3,
    $4
)
RETURNING id, enrollment_id, milestone_day, reward_label, unlocked_at, created_at;

-- name: DeleteMomentumSprintRewardMilestonesByEnrollmentID :exec
DELETE FROM momentum_sprint_reward_milestones
WHERE enrollment_id = $1;

-- name: ListMomentumSprintRewardMilestonesByEnrollmentID :many
SELECT id, enrollment_id, milestone_day, reward_label, unlocked_at, created_at
FROM momentum_sprint_reward_milestones
WHERE enrollment_id = $1
ORDER BY milestone_day ASC;

-- name: UnlockMomentumSprintRewardMilestonesByEnrollmentID :exec
UPDATE momentum_sprint_reward_milestones
SET unlocked_at = COALESCE(unlocked_at, $2)
WHERE enrollment_id = $1
  AND milestone_day <= $3;
