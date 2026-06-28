package analytics

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestComputationsFromSeededWorkoutData(t *testing.T) {
	t.Parallel()

	seededCompletedWorkouts := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}
	seededSetMetrics := []SetMetric{
		{MovementPattern: "squat", Reps: 5, WeightKg: 120},
		{MovementPattern: "squat", Reps: 3, WeightKg: 130},
		{MovementPattern: "push", Reps: 5, WeightKg: 90},
		{MovementPattern: "hinge", Reps: 5, WeightKg: 160},
	}
	seededCoreLiftSets := []CoreLiftSet{
		{ExerciseSlug: "back-squat", Reps: 5, WeightKg: 120},
		{ExerciseSlug: "back-squat", Reps: 3, WeightKg: 130},
		{ExerciseSlug: "bench-press", Reps: 5, WeightKg: 90},
		{ExerciseSlug: "conventional-deadlift", Reps: 5, WeightKg: 160},
		{ExerciseSlug: "conventional-deadlift", Reps: 2, WeightKg: 180},
	}
	seededProteinCheckins := []ProteinCheckin{
		{HitProtein: true},
		{HitProtein: false},
		{HitProtein: true},
		{HitProtein: true},
	}

	workoutsCompleted := ComputeWorkoutsCompletedLast7Days(seededCompletedWorkouts)
	require.Equal(t, int32(3), workoutsCompleted)

	totalSets := ComputeTotalSetsLast7Days(seededSetMetrics)
	require.Equal(t, int32(4), totalSets)

	volumeByPattern := ComputeVolumeByMovementPatternLast7Days(seededSetMetrics)
	require.InDelta(t, 990.0, volumeByPattern["squat"], 0.001)
	require.InDelta(t, 450.0, volumeByPattern["push"], 0.001)
	require.InDelta(t, 800.0, volumeByPattern["hinge"], 0.001)

	prs := ComputeCoreLiftPRs(seededCoreLiftSets)
	require.NotNil(t, prs.Squat.Best5RMEstimateKg)
	require.NotNil(t, prs.Bench.Best5RMEstimateKg)
	require.NotNil(t, prs.Deadlift.Best5RMEstimateKg)

	require.InDelta(t, 122.5714, *prs.Squat.Best5RMEstimateKg, 0.001)
	require.Equal(t, int32(3), *prs.Squat.BestSetReps)
	require.InDelta(t, 130.0, *prs.Squat.BestSetWeightKg, 0.001)

	require.InDelta(t, 90.0, *prs.Bench.Best5RMEstimateKg, 0.001)
	require.Equal(t, int32(5), *prs.Bench.BestSetReps)
	require.InDelta(t, 90.0, *prs.Bench.BestSetWeightKg, 0.001)

	require.InDelta(t, 164.5714, *prs.Deadlift.Best5RMEstimateKg, 0.001)
	require.Equal(t, int32(2), *prs.Deadlift.BestSetReps)
	require.InDelta(t, 180.0, *prs.Deadlift.BestSetWeightKg, 0.001)

	proteinAdherence := ComputeProteinAdherenceLast7DaysPercent(seededProteinCheckins)
	require.InDelta(t, 75.0, proteinAdherence, 0.001)
}

func TestComputeCoreLiftPRsIgnoresInvalidAndUnknownSets(t *testing.T) {
	t.Parallel()

	prs := ComputeCoreLiftPRs([]CoreLiftSet{
		{ExerciseSlug: "back-squat", Reps: 0, WeightKg: 100},
		{ExerciseSlug: "bench-press", Reps: 5, WeightKg: -10},
		{ExerciseSlug: "unknown-lift", Reps: 5, WeightKg: 100},
	})

	require.Nil(t, prs.Squat.Best5RMEstimateKg)
	require.Nil(t, prs.Bench.Best5RMEstimateKg)
	require.Nil(t, prs.Deadlift.Best5RMEstimateKg)
}

func TestComputeMainLiftEstimatedOneRM(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.February, 27, 12, 0, 0, 0, time.UTC)
	summary := ComputeMainLiftEstimatedOneRM([]MainLiftEstimateRow{
		{Lift: "squat", CompletedAt: now.AddDate(0, 0, -3), Reps: 5, WeightKg: 100, EstimatedOneRMKg: 116.6667},
		{Lift: "squat", CompletedAt: now.AddDate(0, 0, -1), Reps: 3, WeightKg: 120, EstimatedOneRMKg: 132.0},
		{Lift: "bench", CompletedAt: now.AddDate(0, 0, -2), Reps: 5, WeightKg: 80, EstimatedOneRMKg: 93.3333},
		{Lift: "deadlift", CompletedAt: now.AddDate(0, 0, -2), Reps: 2, WeightKg: 170, EstimatedOneRMKg: 181.3333},
		{Lift: "unknown", CompletedAt: now, Reps: 5, WeightKg: 100, EstimatedOneRMKg: 116.6667},
	})

	require.NotNil(t, summary.Squat.EstimatedOneRMKg)
	require.InDelta(t, 132.0, *summary.Squat.EstimatedOneRMKg, 0.001)
	require.Equal(t, int32(3), *summary.Squat.BestSetReps)
	require.InDelta(t, 120.0, *summary.Squat.BestSetWeightKg, 0.001)
	require.NotNil(t, summary.Squat.AchievedAt)
	require.Equal(t, now.AddDate(0, 0, -1), *summary.Squat.AchievedAt)

	require.NotNil(t, summary.Bench.EstimatedOneRMKg)
	require.InDelta(t, 93.3333, *summary.Bench.EstimatedOneRMKg, 0.001)

	require.NotNil(t, summary.Deadlift.EstimatedOneRMKg)
	require.InDelta(t, 181.3333, *summary.Deadlift.EstimatedOneRMKg, 0.001)
}

func TestComputeLiftPREvents(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.February, 27, 12, 0, 0, 0, time.UTC)
	events := ComputeLiftPREvents([]LiftPREventRow{
		{
			Lift:                         "squat",
			CompletedAt:                  now,
			Reps:                         3,
			WeightKg:                     120,
			EstimatedOneRMKg:             132.0,
			PreviousBestEstimatedOneRMKg: 116.6667,
		},
		{
			Lift:                         "bench",
			CompletedAt:                  now.Add(-time.Hour),
			Reps:                         5,
			WeightKg:                     80,
			EstimatedOneRMKg:             93.3333,
			PreviousBestEstimatedOneRMKg: -1.0,
		},
		{
			Lift:                         "deadlift",
			CompletedAt:                  now.Add(-2 * time.Hour),
			Reps:                         2,
			WeightKg:                     170,
			EstimatedOneRMKg:             181.3333,
			PreviousBestEstimatedOneRMKg: 190.0,
		},
	}, 0)

	require.Len(t, events, 2)
	require.Equal(t, "squat", events[0].Lift)
	require.NotNil(t, events[0].PreviousEstimatedOneRMKg)
	require.NotNil(t, events[0].ImprovementKg)
	require.InDelta(t, 15.3333, *events[0].ImprovementKg, 0.001)

	require.Equal(t, "bench", events[1].Lift)
	require.Nil(t, events[1].PreviousEstimatedOneRMKg)
	require.Nil(t, events[1].ImprovementKg)
}

func TestComputeWeeklyMuscleGroupVolumeAndTrend(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.February, 27, 12, 0, 0, 0, time.UTC)
	currentWeekStart := startOfUTCWeek(now)
	previousWeekStart := currentWeekStart.AddDate(0, 0, -7)

	weeklyVolume := ComputeWeeklyMuscleGroupVolume([]WeeklyMuscleGroupVolumeRow{
		{WeekStartDate: currentWeekStart, MuscleGroup: "quads", VolumeKg: 430},
		{WeekStartDate: currentWeekStart, MuscleGroup: "glutes", VolumeKg: 215},
		{WeekStartDate: previousWeekStart, MuscleGroup: "quads", VolumeKg: 300},
		{WeekStartDate: previousWeekStart, MuscleGroup: "quads", VolumeKg: 20},
		{WeekStartDate: previousWeekStart, MuscleGroup: "chest", VolumeKg: 200},
		{WeekStartDate: previousWeekStart, MuscleGroup: "", VolumeKg: 100},
	}, now, 3)

	require.Len(t, weeklyVolume, 3)
	require.Equal(t, currentWeekStart, weeklyVolume[0].WeekStartDate)
	require.InDelta(t, 430.0, weeklyVolume[0].VolumeByMuscleGroup["quads"], 0.001)
	require.InDelta(t, 215.0, weeklyVolume[0].VolumeByMuscleGroup["glutes"], 0.001)

	require.Equal(t, previousWeekStart, weeklyVolume[1].WeekStartDate)
	require.InDelta(t, 320.0, weeklyVolume[1].VolumeByMuscleGroup["quads"], 0.001)
	require.InDelta(t, 200.0, weeklyVolume[1].VolumeByMuscleGroup["chest"], 0.001)
	require.Empty(t, weeklyVolume[2].VolumeByMuscleGroup)

	trend := ComputeWeeklyVolumeTrend(weeklyVolume)
	require.Len(t, trend, 3)
	require.InDelta(t, 645.0, trend[0].TotalVolumeKg, 0.001)
	require.InDelta(t, 520.0, trend[1].TotalVolumeKg, 0.001)
	require.InDelta(t, 0.0, trend[2].TotalVolumeKg, 0.001)
}

func TestComputeAdherenceStreaksFromFixtureDates(t *testing.T) {
	t.Parallel()

	dates := []time.Time{
		time.Date(2026, time.February, 18, 15, 0, 0, 0, time.UTC),
		time.Date(2026, time.February, 19, 7, 0, 0, 0, time.UTC),
		time.Date(2026, time.February, 21, 7, 0, 0, 0, time.UTC),
		time.Date(2026, time.February, 22, 18, 0, 0, 0, time.UTC),
		time.Date(2026, time.February, 23, 18, 0, 0, 0, time.UTC),
	}

	streak := ComputeAdherenceStreak(dates)
	require.Equal(t, int32(3), streak.CurrentDays)
	require.Equal(t, int32(3), streak.LongestDays)

	streaks := ComputeAdherenceStreaks(
		dates,
		[]time.Time{
			time.Date(2026, time.February, 10, 0, 0, 0, 0, time.UTC),
			time.Date(2026, time.February, 12, 0, 0, 0, 0, time.UTC),
		},
	)
	require.Equal(t, int32(3), streaks.Training.CurrentDays)
	require.Equal(t, int32(3), streaks.Training.LongestDays)
	require.Equal(t, int32(1), streaks.Protein.CurrentDays)
	require.Equal(t, int32(1), streaks.Protein.LongestDays)
}

func TestComputeWeightTrendPointsAndReadinessHistory(t *testing.T) {
	t.Parallel()

	weightTrend := ComputeWeightTrendPoints([]WeightTrendPointRow{
		{Date: time.Date(2026, time.February, 1, 0, 0, 0, 0, time.UTC), WeightKg: 84.2},
		{Date: time.Date(2026, time.February, 8, 0, 0, 0, 0, time.UTC), WeightKg: 83.6},
		{Date: time.Date(2026, time.February, 15, 0, 0, 0, 0, time.UTC), WeightKg: 0},
	})
	require.Len(t, weightTrend, 2)
	require.Equal(t, time.Date(2026, time.February, 1, 0, 0, 0, 0, time.UTC), weightTrend[0].Date)
	require.InDelta(t, 84.2, weightTrend[0].WeightKg, 0.001)

	readinessHistory := ComputeReadinessSelfReportHistory([]ReadinessSelfReportRow{
		{
			Date:           time.Date(2026, time.February, 25, 0, 0, 0, 0, time.UTC),
			EnergyLevel:    3,
			SleepQuality:   2,
			StressLevel:    1,
			ReadinessScore: 2.6667,
		},
		{
			Date:           time.Date(2026, time.February, 26, 0, 0, 0, 0, time.UTC),
			EnergyLevel:    1,
			SleepQuality:   2,
			StressLevel:    4,
			ReadinessScore: 0.5,
		},
	})
	require.Len(t, readinessHistory, 1)
	require.Equal(t, int32(3), readinessHistory[0].EnergyLevel)
	require.InDelta(t, 2.6667, readinessHistory[0].ReadinessScore, 0.001)
}

func TestEstimateOneRMKgUsesEpleyFormula(t *testing.T) {
	t.Parallel()

	estimate, ok := estimateOneRMKg(120, 5)
	require.True(t, ok)
	require.InDelta(t, 140.0, estimate, 0.001)
}

func TestComputeProteinAdherenceLast7DaysPercentReturnsZeroWithoutCheckins(t *testing.T) {
	t.Parallel()

	adherence := ComputeProteinAdherenceLast7DaysPercent(nil)
	require.InDelta(t, 0.0, adherence, 0.001)
}
