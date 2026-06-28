package habit

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCalculateSprintProgress_ComputesStreaksAndCompletion(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, time.February, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 0, 13)
	now := time.Date(2026, time.February, 5, 12, 0, 0, 0, time.UTC)

	progress := CalculateSprintProgress(
		[]SprintDailyChecklist{
			{Date: date(2026, time.February, 1), TotalEntries: 3, CompletedEntries: 3},
			{Date: date(2026, time.February, 2), TotalEntries: 3, CompletedEntries: 3},
			{Date: date(2026, time.February, 3), TotalEntries: 3, CompletedEntries: 2},
			{Date: date(2026, time.February, 4), TotalEntries: 3, CompletedEntries: 3},
			{Date: date(2026, time.February, 5), TotalEntries: 3, CompletedEntries: 3},
		},
		start,
		end,
		now,
	)

	require.Equal(t, int32(14), progress.TotalDays)
	require.Equal(t, int32(4), progress.CompletedDays)
	require.Equal(t, int32(5), progress.CurrentDay)
	require.Equal(t, int32(10), progress.DaysRemaining)
	require.InDelta(t, 28.57, progress.CompletionPct, 0.01)
	require.Equal(t, int32(2), progress.CurrentStreak)
	require.Equal(t, int32(2), progress.LongestStreak)
	require.True(t, progress.CompletedToday)
}

func TestCalculateSprintProgress_IgnoresFutureDaysForStreaks(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, time.February, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 0, 13)
	now := time.Date(2026, time.February, 3, 9, 0, 0, 0, time.UTC)

	progress := CalculateSprintProgress(
		[]SprintDailyChecklist{
			{Date: date(2026, time.February, 1), TotalEntries: 3, CompletedEntries: 3},
			{Date: date(2026, time.February, 2), TotalEntries: 3, CompletedEntries: 3},
			{Date: date(2026, time.February, 3), TotalEntries: 3, CompletedEntries: 0},
			{Date: date(2026, time.February, 4), TotalEntries: 3, CompletedEntries: 3},
		},
		start,
		end,
		now,
	)

	require.Equal(t, int32(2), progress.CompletedDays)
	require.Equal(t, int32(0), progress.CurrentStreak)
	require.Equal(t, int32(2), progress.LongestStreak)
	require.False(t, progress.CompletedToday)
}
