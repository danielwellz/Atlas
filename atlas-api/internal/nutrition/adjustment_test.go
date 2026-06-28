package nutrition

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestComputeWeeklyAdjustmentHoldsWhenAdherenceIsLow(t *testing.T) {
	result := ComputeWeeklyAdjustment(WeeklyAdjustmentInput{
		CurrentTargets: MacroTargets{CaloriesTarget: 2200},
		Goal:           "fat_loss",
		TrainingPhase:  "hypertrophy",
		Adherence:      0.45,
		WeightChangeKg: 0.10,
		BodyWeightKg:   82,
	})

	require.Equal(t, int32(0), result.CalorieDelta)
	require.Equal(t, int32(2200), result.NewTargets.CaloriesTarget)
	require.Contains(t, result.Explanation, "Adherence is below 60%")
}

func TestComputeWeeklyAdjustmentReducesCaloriesWhenLosingTooSlowly(t *testing.T) {
	result := ComputeWeeklyAdjustment(WeeklyAdjustmentInput{
		CurrentTargets: MacroTargets{CaloriesTarget: 2200},
		Goal:           "fat loss",
		TrainingPhase:  "hypertrophy",
		Adherence:      0.9,
		WeightChangeKg: 0.10,
		BodyWeightKg:   82,
	})

	require.Equal(t, int32(-150), result.CalorieDelta)
	require.Equal(t, int32(2050), result.NewTargets.CaloriesTarget)
	require.Contains(t, result.Explanation, "reducing calories")
}

func TestComputeWeeklyAdjustmentIncreasesCaloriesWhenLosingTooFast(t *testing.T) {
	result := ComputeWeeklyAdjustment(WeeklyAdjustmentInput{
		CurrentTargets: MacroTargets{CaloriesTarget: 2100},
		Goal:           "fat_loss",
		TrainingPhase:  "strength",
		Adherence:      0.85,
		WeightChangeKg: -0.90,
		BodyWeightKg:   78,
	})

	require.Equal(t, int32(150), result.CalorieDelta)
	require.Equal(t, int32(2250), result.NewTargets.CaloriesTarget)
	require.Contains(t, result.Explanation, "increasing calories")
}

func TestComputeWeeklyAdjustmentRecomputesMacrosByPhaseAndGoal(t *testing.T) {
	result := ComputeWeeklyAdjustment(WeeklyAdjustmentInput{
		CurrentTargets: MacroTargets{CaloriesTarget: 2400},
		Goal:           "maintenance",
		TrainingPhase:  "deload",
		Adherence:      0.95,
		WeightChangeKg: 0,
		BodyWeightKg:   80,
	})

	require.Equal(t, int32(2400), result.NewTargets.CaloriesTarget)
	require.Equal(t, int32(160), result.NewTargets.ProteinGTarget)
	require.Equal(t, int32(85), result.NewTargets.FatGTarget)
	require.Equal(t, int32(250), result.NewTargets.CarbsGTarget)
}
