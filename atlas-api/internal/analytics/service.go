package analytics

import (
	"context"
	"database/sql"
	"sort"
	"strings"
	"time"

	db "github.com/atlas/atlas-api/internal/db/sqlc"
	"github.com/google/uuid"
)

const (
	// Uses the Epley equation for estimated 1RM:
	// estimated_1RM = load_kg * (1 + reps / 30).
	defaultLiftRepBracket     = int32(5)
	defaultWeeklyTrendWeeks   = 6
	defaultPREventLookbackDay = 180
	defaultPREventLimit       = 6
	defaultAdherenceLookback  = 90
	defaultWeightTrendDays    = 84
	defaultReadinessTrendDays = 30
)

type Service struct {
	queries db.Querier
}

func NewService(queries db.Querier) *Service {
	return &Service{queries: queries}
}

type SetMetric struct {
	MovementPattern string
	Reps            int32
	WeightKg        float64
}

type CoreLiftSet struct {
	ExerciseSlug string
	Reps         int32
	WeightKg     float64
}

type ProteinCheckin struct {
	HitProtein bool
}

type CoreLiftPR struct {
	Best5RMEstimateKg *float64
	BestSetReps       *int32
	BestSetWeightKg   *float64
}

type CoreLiftPRSummary struct {
	Squat    CoreLiftPR
	Bench    CoreLiftPR
	Deadlift CoreLiftPR
}

type MainLiftEstimateRow struct {
	Lift             string
	CompletedAt      time.Time
	Reps             int32
	WeightKg         float64
	EstimatedOneRMKg float64
}

type MainLiftEstimatedOneRM struct {
	EstimatedOneRMKg *float64
	BestSetReps      *int32
	BestSetWeightKg  *float64
	AchievedAt       *time.Time
}

type MainLiftEstimatedOneRMSummary struct {
	Squat    MainLiftEstimatedOneRM
	Bench    MainLiftEstimatedOneRM
	Deadlift MainLiftEstimatedOneRM
}

type LiftPREventRow struct {
	Lift                         string
	CompletedAt                  time.Time
	Reps                         int32
	WeightKg                     float64
	EstimatedOneRMKg             float64
	PreviousBestEstimatedOneRMKg float64
}

type LiftPREvent struct {
	Lift                     string
	CompletedAt              time.Time
	Reps                     int32
	WeightKg                 float64
	EstimatedOneRMKg         float64
	PreviousEstimatedOneRMKg *float64
	ImprovementKg            *float64
}

type WeeklyMuscleGroupVolumeRow struct {
	WeekStartDate time.Time
	MuscleGroup   string
	VolumeKg      float64
}

type WeeklyMuscleGroupVolume struct {
	WeekStartDate       time.Time
	VolumeByMuscleGroup map[string]float64
}

type WeeklyVolumeTrendPoint struct {
	WeekStartDate time.Time
	TotalVolumeKg float64
}

type AdherenceStreak struct {
	CurrentDays int32
	LongestDays int32
}

type AdherenceStreaks struct {
	Training AdherenceStreak
	Protein  AdherenceStreak
}

type WeightTrendPointRow struct {
	Date     time.Time
	WeightKg float64
}

type WeightTrendPoint struct {
	Date     time.Time
	WeightKg float64
}

type ReadinessSelfReportRow struct {
	Date           time.Time
	EnergyLevel    int32
	SleepQuality   int32
	StressLevel    int32
	ReadinessScore float64
}

type ReadinessSelfReportPoint struct {
	Date           time.Time
	EnergyLevel    int32
	SleepQuality   int32
	StressLevel    int32
	ReadinessScore float64
}

type MacroNutrientTotals struct {
	CaloriesKcal float64
	ProteinG     float64
	CarbsG       float64
	FatG         float64
}

type Summary struct {
	WorkoutsCompletedLast7Days       int32
	TotalSetsLast7Days               int32
	VolumeByMovementPatternLast7Days map[string]float64
	CoreLiftPRs                      CoreLiftPRSummary
	EstimatedOneRMByLift             MainLiftEstimatedOneRMSummary
	PREvents                         []LiftPREvent
	WeeklyMuscleGroupVolume          []WeeklyMuscleGroupVolume
	WeeklyVolumeTrend                []WeeklyVolumeTrendPoint
	AdherenceStreaks                 AdherenceStreaks
	WeightTrendPoints                []WeightTrendPoint
	ReadinessSelfReportHistory       []ReadinessSelfReportPoint
	ProteinAdherenceLast7DaysPercent float64
	NutritionTotalsToday             MacroNutrientTotals
}

func (s *Service) BuildSummaryForUser(ctx context.Context, userID uuid.UUID, now time.Time) (Summary, error) {
	windowStart := now.UTC().AddDate(0, 0, -7)
	windowStartDate := time.Date(now.UTC().Year(), now.UTC().Month(), now.UTC().Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, -6)
	weekTrendWindowStart := startOfUTCWeek(now).AddDate(0, 0, -7*(defaultWeeklyTrendWeeks-1))
	prEventWindowStart := now.UTC().AddDate(0, 0, -defaultPREventLookbackDay)
	adherenceWindowStartDate := normalizeUTCDate(now).AddDate(0, 0, -defaultAdherenceLookback)
	weightTrendWindowStartDate := normalizeUTCDate(now).AddDate(0, 0, -defaultWeightTrendDays)
	readinessWindowStartDate := normalizeUTCDate(now).AddDate(0, 0, -defaultReadinessTrendDays)

	completedWorkoutIDs, err := s.queries.ListCompletedWorkoutIDsSince(ctx, db.ListCompletedWorkoutIDsSinceParams{
		UserID:      userID,
		CompletedAt: sql.NullTime{Time: windowStart, Valid: true},
	})
	if err != nil {
		return Summary{}, err
	}

	setMetricRows, err := s.queries.ListWorkoutSetMetricsSince(ctx, db.ListWorkoutSetMetricsSinceParams{
		UserID:      userID,
		CompletedAt: windowStart,
	})
	if err != nil {
		return Summary{}, err
	}

	setMetrics := make([]SetMetric, 0, len(setMetricRows))
	for _, row := range setMetricRows {
		setMetrics = append(setMetrics, SetMetric{
			MovementPattern: row.MovementPattern,
			Reps:            row.Reps,
			WeightKg:        row.WeightKg,
		})
	}

	coreLiftRows, err := s.queries.ListCoreLiftSetsForPR(ctx, userID)
	if err != nil {
		return Summary{}, err
	}

	coreLiftSets := make([]CoreLiftSet, 0, len(coreLiftRows))
	for _, row := range coreLiftRows {
		coreLiftSets = append(coreLiftSets, CoreLiftSet{
			ExerciseSlug: row.Slug,
			Reps:         row.Reps,
			WeightKg:     row.WeightKg,
		})
	}

	mainLiftEstimateRows, err := s.queries.ListMainLiftEstimatedOneRM(ctx, userID)
	if err != nil {
		return Summary{}, err
	}

	mainLiftEstimates := make([]MainLiftEstimateRow, 0, len(mainLiftEstimateRows))
	for _, row := range mainLiftEstimateRows {
		mainLiftEstimates = append(mainLiftEstimates, MainLiftEstimateRow{
			Lift:             row.Lift,
			CompletedAt:      row.CompletedAt,
			Reps:             row.Reps,
			WeightKg:         row.WeightKg,
			EstimatedOneRMKg: row.EstimatedOneRmKg,
		})
	}

	prEventRows, err := s.queries.ListLiftPREventsSince(ctx, db.ListLiftPREventsSinceParams{
		UserID:      userID,
		CompletedAt: prEventWindowStart,
	})
	if err != nil {
		return Summary{}, err
	}

	prEvents := make([]LiftPREventRow, 0, len(prEventRows))
	for _, row := range prEventRows {
		prEvents = append(prEvents, LiftPREventRow{
			Lift:                         row.Lift,
			CompletedAt:                  row.CompletedAt,
			Reps:                         row.Reps,
			WeightKg:                     row.WeightKg,
			EstimatedOneRMKg:             row.EstimatedOneRmKg,
			PreviousBestEstimatedOneRMKg: row.PreviousBestEstimatedOneRmKg,
		})
	}

	weeklyMuscleVolumeRows, err := s.queries.ListWeeklyMuscleGroupVolumeSince(ctx, db.ListWeeklyMuscleGroupVolumeSinceParams{
		UserID:      userID,
		CompletedAt: weekTrendWindowStart,
	})
	if err != nil {
		return Summary{}, err
	}

	weeklyMuscleVolume := make([]WeeklyMuscleGroupVolumeRow, 0, len(weeklyMuscleVolumeRows))
	for _, row := range weeklyMuscleVolumeRows {
		weeklyMuscleVolume = append(weeklyMuscleVolume, WeeklyMuscleGroupVolumeRow{
			WeekStartDate: row.WeekStartDate,
			MuscleGroup:   row.MuscleGroup,
			VolumeKg:      row.VolumeKg,
		})
	}

	proteinCheckinRows, err := s.queries.ListNutritionProteinCheckinsSinceDate(ctx, db.ListNutritionProteinCheckinsSinceDateParams{
		UserID: userID,
		Date:   windowStartDate,
	})
	if err != nil {
		return Summary{}, err
	}

	proteinCheckins := make([]ProteinCheckin, 0, len(proteinCheckinRows))
	for _, row := range proteinCheckinRows {
		proteinCheckins = append(proteinCheckins, ProteinCheckin{
			HitProtein: row.HitProtein,
		})
	}

	completedWorkoutDateRows, err := s.queries.ListCompletedWorkoutDatesSince(ctx, db.ListCompletedWorkoutDatesSinceParams{
		UserID:      userID,
		CompletedAt: sql.NullTime{Time: adherenceWindowStartDate, Valid: true},
	})
	if err != nil {
		return Summary{}, err
	}

	completedWorkoutDates := make([]time.Time, 0, len(completedWorkoutDateRows))
	for _, completedDate := range completedWorkoutDateRows {
		completedWorkoutDates = append(completedWorkoutDates, completedDate)
	}

	proteinHitDateRows, err := s.queries.ListProteinHitDatesSinceDate(ctx, db.ListProteinHitDatesSinceDateParams{
		UserID: userID,
		Date:   adherenceWindowStartDate,
	})
	if err != nil {
		return Summary{}, err
	}

	proteinHitDates := make([]time.Time, 0, len(proteinHitDateRows))
	for _, hitDate := range proteinHitDateRows {
		proteinHitDates = append(proteinHitDates, hitDate)
	}

	weightTrendRows, err := s.queries.ListWeightTrendPointsSinceDate(ctx, db.ListWeightTrendPointsSinceDateParams{
		UserID: userID,
		Date:   weightTrendWindowStartDate,
	})
	if err != nil {
		return Summary{}, err
	}

	weightTrendPoints := make([]WeightTrendPointRow, 0, len(weightTrendRows))
	for _, row := range weightTrendRows {
		weightTrendPoints = append(weightTrendPoints, WeightTrendPointRow{
			Date:     row.Date,
			WeightKg: row.WeightKg,
		})
	}

	readinessRows, err := s.queries.ListReadinessCheckinsSinceDate(ctx, db.ListReadinessCheckinsSinceDateParams{
		UserID: userID,
		Date:   readinessWindowStartDate,
	})
	if err != nil {
		return Summary{}, err
	}

	readinessHistoryRows := make([]ReadinessSelfReportRow, 0, len(readinessRows))
	for _, row := range readinessRows {
		readinessHistoryRows = append(readinessHistoryRows, ReadinessSelfReportRow{
			Date:           row.Date,
			EnergyLevel:    row.EnergyLevel,
			SleepQuality:   row.SleepQuality,
			StressLevel:    row.StressLevel,
			ReadinessScore: row.ReadinessScore,
		})
	}

	nutritionTotalsRow, err := s.queries.GetFoodLogDailyTotalsByUserIDAndDate(ctx, db.GetFoodLogDailyTotalsByUserIDAndDateParams{
		UserID: userID,
		Date:   time.Date(now.UTC().Year(), now.UTC().Month(), now.UTC().Day(), 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		return Summary{}, err
	}

	weeklyMuscleGroupVolume := ComputeWeeklyMuscleGroupVolume(weeklyMuscleVolume, now, defaultWeeklyTrendWeeks)

	return Summary{
		WorkoutsCompletedLast7Days:       ComputeWorkoutsCompletedLast7Days(completedWorkoutIDs),
		TotalSetsLast7Days:               ComputeTotalSetsLast7Days(setMetrics),
		VolumeByMovementPatternLast7Days: ComputeVolumeByMovementPatternLast7Days(setMetrics),
		CoreLiftPRs:                      ComputeCoreLiftPRs(coreLiftSets),
		EstimatedOneRMByLift:             ComputeMainLiftEstimatedOneRM(mainLiftEstimates),
		PREvents:                         ComputeLiftPREvents(prEvents, defaultPREventLimit),
		WeeklyMuscleGroupVolume:          weeklyMuscleGroupVolume,
		WeeklyVolumeTrend:                ComputeWeeklyVolumeTrend(weeklyMuscleGroupVolume),
		AdherenceStreaks:                 ComputeAdherenceStreaks(completedWorkoutDates, proteinHitDates),
		WeightTrendPoints:                ComputeWeightTrendPoints(weightTrendPoints),
		ReadinessSelfReportHistory:       ComputeReadinessSelfReportHistory(readinessHistoryRows),
		ProteinAdherenceLast7DaysPercent: ComputeProteinAdherenceLast7DaysPercent(proteinCheckins),
		NutritionTotalsToday: MacroNutrientTotals{
			CaloriesKcal: nutritionTotalsRow.CaloriesKcal,
			ProteinG:     nutritionTotalsRow.ProteinG,
			CarbsG:       nutritionTotalsRow.CarbsG,
			FatG:         nutritionTotalsRow.FatG,
		},
	}, nil
}

func ComputeWorkoutsCompletedLast7Days(workoutIDs []uuid.UUID) int32 {
	return int32(len(workoutIDs))
}

func ComputeTotalSetsLast7Days(setMetrics []SetMetric) int32 {
	return int32(len(setMetrics))
}

func ComputeVolumeByMovementPatternLast7Days(setMetrics []SetMetric) map[string]float64 {
	volumeByPattern := map[string]float64{}
	for _, setMetric := range setMetrics {
		pattern := strings.TrimSpace(setMetric.MovementPattern)
		if pattern == "" {
			continue
		}
		if setMetric.Reps <= 0 || setMetric.WeightKg < 0 {
			continue
		}
		volumeByPattern[pattern] += float64(setMetric.Reps) * setMetric.WeightKg
	}
	return volumeByPattern
}

func ComputeCoreLiftPRs(coreLiftSets []CoreLiftSet) CoreLiftPRSummary {
	bestByLift := map[string]CoreLiftPR{}
	for _, set := range coreLiftSets {
		lift, ok := coreLiftFromSlug(set.ExerciseSlug)
		if !ok {
			continue
		}

		estimate, ok := estimateFiveRMKg(set.WeightKg, set.Reps)
		if !ok {
			continue
		}

		current := bestByLift[lift]
		if current.Best5RMEstimateKg == nil || estimate > *current.Best5RMEstimateKg {
			estimateCopy := estimate
			repsCopy := set.Reps
			weightCopy := set.WeightKg
			bestByLift[lift] = CoreLiftPR{
				Best5RMEstimateKg: &estimateCopy,
				BestSetReps:       &repsCopy,
				BestSetWeightKg:   &weightCopy,
			}
		}
	}

	return CoreLiftPRSummary{
		Squat:    bestByLift["squat"],
		Bench:    bestByLift["bench"],
		Deadlift: bestByLift["deadlift"],
	}
}

func ComputeMainLiftEstimatedOneRM(rows []MainLiftEstimateRow) MainLiftEstimatedOneRMSummary {
	bestByLift := map[string]MainLiftEstimatedOneRM{}

	for _, row := range rows {
		lift := strings.TrimSpace(strings.ToLower(row.Lift))
		if !isMainLift(lift) {
			continue
		}
		if row.Reps <= 0 || row.WeightKg < 0 || row.EstimatedOneRMKg < 0 {
			continue
		}

		current := bestByLift[lift]
		if current.EstimatedOneRMKg == nil || row.EstimatedOneRMKg > *current.EstimatedOneRMKg {
			estimatedOneRMCopy := row.EstimatedOneRMKg
			repsCopy := row.Reps
			weightCopy := row.WeightKg
			completedAtCopy := row.CompletedAt.UTC()
			bestByLift[lift] = MainLiftEstimatedOneRM{
				EstimatedOneRMKg: &estimatedOneRMCopy,
				BestSetReps:      &repsCopy,
				BestSetWeightKg:  &weightCopy,
				AchievedAt:       &completedAtCopy,
			}
		}
	}

	return MainLiftEstimatedOneRMSummary{
		Squat:    bestByLift["squat"],
		Bench:    bestByLift["bench"],
		Deadlift: bestByLift["deadlift"],
	}
}

func ComputeLiftPREvents(rows []LiftPREventRow, maxEvents int) []LiftPREvent {
	events := make([]LiftPREvent, 0, len(rows))
	for _, row := range rows {
		lift := strings.TrimSpace(strings.ToLower(row.Lift))
		if !isMainLift(lift) {
			continue
		}
		if row.Reps <= 0 || row.WeightKg < 0 || row.EstimatedOneRMKg < 0 {
			continue
		}

		event := LiftPREvent{
			Lift:             lift,
			CompletedAt:      row.CompletedAt.UTC(),
			Reps:             row.Reps,
			WeightKg:         row.WeightKg,
			EstimatedOneRMKg: row.EstimatedOneRMKg,
		}

		if row.PreviousBestEstimatedOneRMKg >= 0 {
			previousCopy := row.PreviousBestEstimatedOneRMKg
			improvement := row.EstimatedOneRMKg - row.PreviousBestEstimatedOneRMKg
			if improvement < 0 {
				continue
			}
			event.PreviousEstimatedOneRMKg = &previousCopy
			event.ImprovementKg = &improvement
		}

		events = append(events, event)
		if maxEvents > 0 && len(events) >= maxEvents {
			break
		}
	}

	return events
}

func ComputeWeeklyMuscleGroupVolume(rows []WeeklyMuscleGroupVolumeRow, now time.Time, weekCount int) []WeeklyMuscleGroupVolume {
	if weekCount <= 0 {
		return nil
	}

	volumeByWeek := map[time.Time]map[string]float64{}
	for _, row := range rows {
		weekStartDate := normalizeUTCDate(row.WeekStartDate)
		muscleGroup := strings.TrimSpace(strings.ToLower(row.MuscleGroup))
		if muscleGroup == "" || row.VolumeKg <= 0 {
			continue
		}

		currentByMuscleGroup, ok := volumeByWeek[weekStartDate]
		if !ok {
			currentByMuscleGroup = map[string]float64{}
			volumeByWeek[weekStartDate] = currentByMuscleGroup
		}
		currentByMuscleGroup[muscleGroup] += row.VolumeKg
	}

	weekStartDate := startOfUTCWeek(now)
	weeklyMuscleGroupVolume := make([]WeeklyMuscleGroupVolume, 0, weekCount)
	for i := 0; i < weekCount; i++ {
		week := weekStartDate.AddDate(0, 0, -7*i)
		sourceVolumes := volumeByWeek[week]
		clonedVolumes := make(map[string]float64, len(sourceVolumes))
		for muscleGroup, volume := range sourceVolumes {
			clonedVolumes[muscleGroup] = volume
		}

		weeklyMuscleGroupVolume = append(weeklyMuscleGroupVolume, WeeklyMuscleGroupVolume{
			WeekStartDate:       week,
			VolumeByMuscleGroup: clonedVolumes,
		})
	}

	return weeklyMuscleGroupVolume
}

func ComputeWeeklyVolumeTrend(weeklyMuscleGroupVolume []WeeklyMuscleGroupVolume) []WeeklyVolumeTrendPoint {
	trend := make([]WeeklyVolumeTrendPoint, 0, len(weeklyMuscleGroupVolume))
	for _, weeklyVolume := range weeklyMuscleGroupVolume {
		totalVolumeKg := 0.0
		for _, volume := range weeklyVolume.VolumeByMuscleGroup {
			totalVolumeKg += volume
		}

		trend = append(trend, WeeklyVolumeTrendPoint{
			WeekStartDate: weeklyVolume.WeekStartDate,
			TotalVolumeKg: totalVolumeKg,
		})
	}

	return trend
}

func ComputeAdherenceStreaks(trainingDates []time.Time, proteinHitDates []time.Time) AdherenceStreaks {
	return AdherenceStreaks{
		Training: ComputeAdherenceStreak(trainingDates),
		Protein:  ComputeAdherenceStreak(proteinHitDates),
	}
}

func ComputeAdherenceStreak(dates []time.Time) AdherenceStreak {
	if len(dates) == 0 {
		return AdherenceStreak{}
	}

	seen := map[time.Time]struct{}{}
	orderedDates := make([]time.Time, 0, len(dates))
	for _, rawDate := range dates {
		normalizedDate := normalizeUTCDate(rawDate)
		if _, exists := seen[normalizedDate]; exists {
			continue
		}
		seen[normalizedDate] = struct{}{}
		orderedDates = append(orderedDates, normalizedDate)
	}
	if len(orderedDates) == 0 {
		return AdherenceStreak{}
	}

	sort.Slice(orderedDates, func(i, j int) bool {
		return orderedDates[i].Before(orderedDates[j])
	})

	longest := int32(1)
	running := int32(1)
	for index := 1; index < len(orderedDates); index++ {
		if orderedDates[index].Sub(orderedDates[index-1]) == 24*time.Hour {
			running++
		} else {
			running = 1
		}
		if running > longest {
			longest = running
		}
	}

	current := int32(1)
	for index := len(orderedDates) - 1; index > 0; index-- {
		if orderedDates[index].Sub(orderedDates[index-1]) != 24*time.Hour {
			break
		}
		current++
	}

	return AdherenceStreak{
		CurrentDays: current,
		LongestDays: longest,
	}
}

func ComputeWeightTrendPoints(rows []WeightTrendPointRow) []WeightTrendPoint {
	points := make([]WeightTrendPoint, 0, len(rows))
	for _, row := range rows {
		if row.WeightKg <= 0 {
			continue
		}

		points = append(points, WeightTrendPoint{
			Date:     normalizeUTCDate(row.Date),
			WeightKg: row.WeightKg,
		})
	}

	sort.Slice(points, func(i, j int) bool {
		return points[i].Date.Before(points[j].Date)
	})
	return points
}

func ComputeReadinessSelfReportHistory(rows []ReadinessSelfReportRow) []ReadinessSelfReportPoint {
	history := make([]ReadinessSelfReportPoint, 0, len(rows))
	for _, row := range rows {
		if row.EnergyLevel < 1 || row.EnergyLevel > 3 {
			continue
		}
		if row.SleepQuality < 1 || row.SleepQuality > 3 {
			continue
		}
		if row.StressLevel < 1 || row.StressLevel > 3 {
			continue
		}
		if row.ReadinessScore < 0 {
			continue
		}

		history = append(history, ReadinessSelfReportPoint{
			Date:           normalizeUTCDate(row.Date),
			EnergyLevel:    row.EnergyLevel,
			SleepQuality:   row.SleepQuality,
			StressLevel:    row.StressLevel,
			ReadinessScore: row.ReadinessScore,
		})
	}

	sort.Slice(history, func(i, j int) bool {
		return history[i].Date.Before(history[j].Date)
	})
	return history
}

func ComputeProteinAdherenceLast7DaysPercent(proteinCheckins []ProteinCheckin) float64 {
	if len(proteinCheckins) == 0 {
		return 0
	}

	var hitCount int
	for _, checkin := range proteinCheckins {
		if checkin.HitProtein {
			hitCount++
		}
	}

	return (float64(hitCount) / float64(len(proteinCheckins))) * 100.0
}

func coreLiftFromSlug(slug string) (string, bool) {
	switch slug {
	case "back-squat":
		return "squat", true
	case "bench-press":
		return "bench", true
	case "conventional-deadlift":
		return "deadlift", true
	default:
		return "", false
	}
}

func isMainLift(lift string) bool {
	switch lift {
	case "squat", "bench", "deadlift":
		return true
	default:
		return false
	}
}

func normalizeUTCDate(value time.Time) time.Time {
	utcDate := value.UTC()
	return time.Date(utcDate.Year(), utcDate.Month(), utcDate.Day(), 0, 0, 0, 0, time.UTC)
}

func startOfUTCWeek(now time.Time) time.Time {
	day := normalizeUTCDate(now)
	weekday := int(day.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	return day.AddDate(0, 0, -(weekday - 1))
}

func estimateOneRMKg(weightKg float64, reps int32) (float64, bool) {
	if reps <= 0 || weightKg < 0 {
		return 0, false
	}
	return weightKg * (1.0 + float64(reps)/30.0), true
}

func estimateFiveRMKg(weightKg float64, reps int32) (float64, bool) {
	oneRMEstimate, ok := estimateOneRMKg(weightKg, reps)
	if !ok {
		return 0, false
	}
	fiveRMEstimate := oneRMEstimate / (1.0 + float64(defaultLiftRepBracket)/30.0)
	return fiveRMEstimate, true
}
