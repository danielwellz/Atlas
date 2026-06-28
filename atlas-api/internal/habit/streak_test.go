package habit

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCalculateStreaks_CurrentAndLongest(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.February, 26, 12, 0, 0, 0, time.UTC)
	logs := []DailyLog{
		{Date: date(2026, 2, 20), Completed: true},
		{Date: date(2026, 2, 21), Completed: true},
		{Date: date(2026, 2, 22), Completed: true},
		{Date: date(2026, 2, 23), Completed: false},
		{Date: date(2026, 2, 24), Completed: true},
		{Date: date(2026, 2, 25), Completed: true},
	}

	current, longest := CalculateStreaks(logs, now)
	require.Equal(t, int32(2), current)
	require.Equal(t, int32(3), longest)
}

func TestCalculateStreaks_CurrentZeroWhenTodayExplicitlyIncomplete(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.February, 26, 12, 0, 0, 0, time.UTC)
	logs := []DailyLog{
		{Date: date(2026, 2, 24), Completed: true},
		{Date: date(2026, 2, 25), Completed: true},
		{Date: date(2026, 2, 26), Completed: false},
	}

	current, longest := CalculateStreaks(logs, now)
	require.Equal(t, int32(0), current)
	require.Equal(t, int32(2), longest)
}

func TestCalculateStreaks_CurrentFromTodayWhenCompleted(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.February, 26, 12, 0, 0, 0, time.UTC)
	logs := []DailyLog{
		{Date: date(2026, 2, 24), Completed: true},
		{Date: date(2026, 2, 25), Completed: true},
		{Date: date(2026, 2, 26), Completed: true},
	}

	current, longest := CalculateStreaks(logs, now)
	require.Equal(t, int32(3), current)
	require.Equal(t, int32(3), longest)
}

func TestCalculateStreaks_ZeroWhenNoRecentCompletion(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.February, 26, 12, 0, 0, 0, time.UTC)
	logs := []DailyLog{
		{Date: date(2026, 2, 20), Completed: true},
		{Date: date(2026, 2, 21), Completed: true},
	}

	current, longest := CalculateStreaks(logs, now)
	require.Equal(t, int32(0), current)
	require.Equal(t, int32(2), longest)
}

func date(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}
