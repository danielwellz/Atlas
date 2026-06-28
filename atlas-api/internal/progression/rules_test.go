package progression

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRecommendLoadIncreaseWhenTargetHit(t *testing.T) {
	rpe := 7.0
	recommendation := RecommendLoad(LoadRecommendationInput{
		LastLoadKg:    100,
		LastReps:      8,
		LastRPE:       &rpe,
		TargetRepsMin: 5,
		TargetRepsMax: 8,
	})

	require.Equal(t, 105.0, recommendation.LoadKg)
	require.Contains(t, recommendation.Why, "increasing load")
}

func TestRecommendLoadDeloadWhenMissedAndHighRPE(t *testing.T) {
	rpe := 9.5
	recommendation := RecommendLoad(LoadRecommendationInput{
		LastLoadKg:    100,
		LastReps:      3,
		LastRPE:       &rpe,
		TargetRepsMin: 5,
		TargetRepsMax: 8,
	})

	require.Equal(t, 95.0, recommendation.LoadKg)
	require.Contains(t, recommendation.Why, "slight deload")
}

func TestAdjustVolumeForCatchUp(t *testing.T) {
	require.Equal(t, int32(3), AdjustVolumeForCatchUp(4, true))
	require.Equal(t, int32(1), AdjustVolumeForCatchUp(1, true))
	require.Equal(t, int32(4), AdjustVolumeForCatchUp(4, false))
}

func TestComputeWeeklyAdjustmentMissedWeekTriggersDeload(t *testing.T) {
	adjustment := ComputeWeeklyAdjustment(WeeklyAdjustmentInput{
		ScheduledSessions:               3,
		CompletedSessions:               0,
		CompletedSets:                   0,
		HighRpeSets:                     0,
		PreviousWeekAdherence:           0.66,
		PreviousWeekDensity:             12,
		PreviousConsecutiveLowAdherence: 1,
		RecoveredPerformance:            false,
		HasProgramHistory:               true,
	})

	require.True(t, adjustment.DeloadFlag)
	require.InDelta(t, 0.0, adjustment.LastWeekAdherence, 0.001)
	require.Equal(t, int32(2), adjustment.ConsecutiveLowAdherenceWeeks)
	require.Contains(t, adjustment.Reasons[0], "Missed all 3 scheduled sessions")
}

func TestComputeWeeklyAdjustmentPartialAdherenceHoldsWithoutDeload(t *testing.T) {
	adjustment := ComputeWeeklyAdjustment(WeeklyAdjustmentInput{
		ScheduledSessions:               3,
		CompletedSessions:               2,
		CompletedSets:                   16,
		HighRpeSets:                     2,
		PreviousWeekAdherence:           1,
		PreviousWeekDensity:             9,
		PreviousConsecutiveLowAdherence: 0,
		RecoveredPerformance:            true,
		HasProgramHistory:               true,
	})

	require.False(t, adjustment.DeloadFlag)
	require.InDelta(t, 0.667, adjustment.LastWeekAdherence, 0.001)
	require.InDelta(t, 8.0, adjustment.LastWeekDensity, 0.001)
	require.InDelta(t, 0.125, adjustment.LastWeekHighRpeRate, 0.001)
	require.Greater(t, len(adjustment.Reasons), 0)
	require.Contains(t, strings.ToLower(strings.Join(adjustment.Reasons, " ")), "partial adherence")
}
