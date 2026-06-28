package httpapi

import (
	"encoding/json"
	"testing"
	"time"

	db "github.com/atlas/atlas-api/internal/db/sqlc"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestToAPIProgramParsesGoalTags(t *testing.T) {
	row := db.Program{
		ID:           uuid.New(),
		Slug:         "hypertrophy-foundations-3-days",
		Name:         "Hypertrophy Foundations (3 days/week)",
		Description:  "Three-day hypertrophy template.",
		GoalTagsJson: json.RawMessage(`["hypertrophy","foundations"]`),
		Level:        "beginner",
		WeeksLength:  8,
		CreatedAt:    time.Now().UTC(),
	}

	program, err := toAPIProgram(row)
	require.NoError(t, err)
	require.Equal(t, row.Slug, program.Slug)
	require.Equal(t, []string{"hypertrophy", "foundations"}, program.GoalTags)
	require.Equal(t, int32(8), program.WeeksLength)
}

func TestToAPIProgramReturnsErrorOnInvalidGoalTags(t *testing.T) {
	row := db.Program{
		ID:           uuid.New(),
		Slug:         "invalid-goal-tags",
		Name:         "Invalid",
		Description:  "Invalid",
		GoalTagsJson: json.RawMessage(`{"not":"array"}`),
		Level:        "beginner",
		WeeksLength:  4,
		CreatedAt:    time.Now().UTC(),
	}

	_, err := toAPIProgram(row)
	require.Error(t, err)
}

func TestToAPIProgramEnrollmentMapsValues(t *testing.T) {
	startDate := time.Date(2026, time.February, 26, 0, 0, 0, 0, time.UTC)
	row := db.UserProgramEnrollment{
		ID:          uuid.New(),
		UserID:      uuid.New(),
		ProgramID:   uuid.New(),
		StartDate:   startDate,
		CurrentWeek: 1,
		CreatedAt:   time.Now().UTC(),
	}

	enrollment := toAPIProgramEnrollment(row)
	require.Equal(t, "2026-02-26", enrollment.StartDate.String())
	require.Equal(t, int32(1), enrollment.CurrentWeek)
}
