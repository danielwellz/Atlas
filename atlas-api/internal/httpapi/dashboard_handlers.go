package httpapi

import (
	"context"
	"database/sql"
	"time"

	"github.com/atlas/atlas-api/internal/analytics"
	db "github.com/atlas/atlas-api/internal/db/sqlc"
	"github.com/atlas/atlas-api/internal/httpapi/generated"
	"github.com/atlas/atlas-api/internal/httpapi/middleware"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"go.uber.org/zap"
)

func (s *Server) GetDashboardSummary(ctx context.Context, _ generated.GetDashboardSummaryRequestObject) (generated.GetDashboardSummaryResponseObject, error) {
	userID, ok := middleware.AuthenticatedUserID(ctx)
	if !ok {
		return generated.GetDashboardSummary401JSONResponse{Message: "unauthorized"}, nil
	}

	summary, err := s.analyticsSvc.BuildSummaryForUser(ctx, userID, s.currentTime())
	if err != nil {
		if err == sql.ErrNoRows {
			return generated.GetDashboardSummary200JSONResponse{
				Summary: toAPIDashboardSummary(analytics.Summary{}),
			}, nil
		}
		s.logger.Error("failed building dashboard summary", zap.Error(err), zap.String("user_id", userID.String()))
		return generated.GetDashboardSummary500JSONResponse{Message: "could not build dashboard summary"}, nil
	}

	return generated.GetDashboardSummary200JSONResponse{
		Summary: toAPIDashboardSummary(summary),
	}, nil
}

func (s *Server) PostDashboardReadinessCheckin(
	ctx context.Context,
	request generated.PostDashboardReadinessCheckinRequestObject,
) (generated.PostDashboardReadinessCheckinResponseObject, error) {
	userID, ok := middleware.AuthenticatedUserID(ctx)
	if !ok {
		return generated.PostDashboardReadinessCheckin401JSONResponse{Message: "unauthorized"}, nil
	}
	if request.Body == nil {
		return generated.PostDashboardReadinessCheckin400JSONResponse{Message: "request body is required"}, nil
	}

	checkinDate := normalizeDateUTC(s.currentTime())
	if request.Body.Date != nil {
		checkinDate = normalizeDateUTC(request.Body.Date.Time)
	}
	if request.Body.EnergyLevel < 1 || request.Body.EnergyLevel > 3 {
		return generated.PostDashboardReadinessCheckin400JSONResponse{Message: "energyLevel must be between 1 and 3"}, nil
	}
	if request.Body.SleepQuality < 1 || request.Body.SleepQuality > 3 {
		return generated.PostDashboardReadinessCheckin400JSONResponse{Message: "sleepQuality must be between 1 and 3"}, nil
	}
	if request.Body.StressLevel < 1 || request.Body.StressLevel > 3 {
		return generated.PostDashboardReadinessCheckin400JSONResponse{Message: "stressLevel must be between 1 and 3"}, nil
	}

	row, err := s.queries.UpsertReadinessCheckin(ctx, db.UpsertReadinessCheckinParams{
		UserID:       userID,
		Date:         checkinDate,
		EnergyLevel:  request.Body.EnergyLevel,
		SleepQuality: request.Body.SleepQuality,
		StressLevel:  request.Body.StressLevel,
	})
	if err != nil {
		s.logger.Error(
			"failed upserting dashboard readiness checkin",
			zap.Error(err),
			zap.String("user_id", userID.String()),
			zap.Time("date", checkinDate),
		)
		return generated.PostDashboardReadinessCheckin500JSONResponse{
			Message: "could not save readiness check-in",
		}, nil
	}

	return generated.PostDashboardReadinessCheckin200JSONResponse{
		Checkin: toAPIDashboardReadinessCheckin(row),
	}, nil
}

func toAPIDashboardSummary(summary analytics.Summary) generated.DashboardSummary {
	volumeByPattern := make(map[string]float32, len(summary.VolumeByMovementPatternLast7Days))
	for pattern, volume := range summary.VolumeByMovementPatternLast7Days {
		volumeByPattern[pattern] = float32(volume)
	}

	return generated.DashboardSummary{
		WorkoutsCompletedLast7Days:       summary.WorkoutsCompletedLast7Days,
		TotalSetsLast7Days:               summary.TotalSetsLast7Days,
		VolumeByMovementPatternLast7Days: volumeByPattern,
		ProteinAdherenceLast7DaysPercent: float32(summary.ProteinAdherenceLast7DaysPercent),
		NutritionTotalsToday: generated.MacroNutrientTotals{
			CaloriesKcal: float32(summary.NutritionTotalsToday.CaloriesKcal),
			ProteinG:     float32(summary.NutritionTotalsToday.ProteinG),
			CarbsG:       float32(summary.NutritionTotalsToday.CarbsG),
			FatG:         float32(summary.NutritionTotalsToday.FatG),
		},
		EstimatedOneRmByLift:    toAPIMainLiftEstimatedOneRMSummary(summary.EstimatedOneRMByLift),
		PrEvents:                toAPIPREvents(summary.PREvents),
		WeeklyMuscleGroupVolume: toAPIWeeklyMuscleGroupVolume(summary.WeeklyMuscleGroupVolume),
		WeeklyVolumeTrend:       toAPIWeeklyVolumeTrend(summary.WeeklyVolumeTrend),
		AdherenceStreaks:        toAPIAdherenceStreaks(summary.AdherenceStreaks),
		WeightTrendPoints:       toAPIWeightTrendPoints(summary.WeightTrendPoints),
		ReadinessSelfReportHistory: toAPIReadinessSelfReportHistory(
			summary.ReadinessSelfReportHistory,
		),
		Prs: generated.CoreLiftPRSummary{
			Squat:    toAPICoreLiftPR(summary.CoreLiftPRs.Squat),
			Bench:    toAPICoreLiftPR(summary.CoreLiftPRs.Bench),
			Deadlift: toAPICoreLiftPR(summary.CoreLiftPRs.Deadlift),
		},
	}
}

func toAPICoreLiftPR(pr analytics.CoreLiftPR) generated.CoreLiftPR {
	return generated.CoreLiftPR{
		Best5RmEstimateKg: float64PointerToFloat32(pr.Best5RMEstimateKg),
		BestSetReps:       pr.BestSetReps,
		BestSetWeightKg:   float64PointerToFloat32(pr.BestSetWeightKg),
	}
}

func toAPIMainLiftEstimatedOneRMSummary(summary analytics.MainLiftEstimatedOneRMSummary) generated.MainLiftEstimatedOneRMSummary {
	return generated.MainLiftEstimatedOneRMSummary{
		Squat:    toAPIMainLiftEstimatedOneRM(summary.Squat),
		Bench:    toAPIMainLiftEstimatedOneRM(summary.Bench),
		Deadlift: toAPIMainLiftEstimatedOneRM(summary.Deadlift),
	}
}

func toAPIMainLiftEstimatedOneRM(value analytics.MainLiftEstimatedOneRM) generated.MainLiftEstimatedOneRM {
	return generated.MainLiftEstimatedOneRM{
		EstimatedOneRmKg: float64PointerToFloat32(value.EstimatedOneRMKg),
		BestSetReps:      value.BestSetReps,
		BestSetWeightKg:  float64PointerToFloat32(value.BestSetWeightKg),
		AchievedAt:       value.AchievedAt,
	}
}

func toAPIPREvents(events []analytics.LiftPREvent) []generated.DashboardPREvent {
	apiEvents := make([]generated.DashboardPREvent, 0, len(events))
	for _, event := range events {
		apiEvents = append(apiEvents, generated.DashboardPREvent{
			Lift:                     generated.DashboardPREventLift(event.Lift),
			CompletedAt:              event.CompletedAt,
			Reps:                     event.Reps,
			WeightKg:                 float32(event.WeightKg),
			EstimatedOneRmKg:         float32(event.EstimatedOneRMKg),
			PreviousEstimatedOneRmKg: float64PointerToFloat32(event.PreviousEstimatedOneRMKg),
			ImprovementKg:            float64PointerToFloat32(event.ImprovementKg),
		})
	}
	return apiEvents
}

func toAPIWeeklyMuscleGroupVolume(values []analytics.WeeklyMuscleGroupVolume) []generated.WeeklyMuscleGroupVolume {
	apiValues := make([]generated.WeeklyMuscleGroupVolume, 0, len(values))
	for _, value := range values {
		volumeByMuscleGroup := make(map[string]float32, len(value.VolumeByMuscleGroup))
		for muscleGroup, volume := range value.VolumeByMuscleGroup {
			volumeByMuscleGroup[muscleGroup] = float32(volume)
		}

		apiValues = append(apiValues, generated.WeeklyMuscleGroupVolume{
			WeekStartDate:       toAPIDate(value.WeekStartDate),
			VolumeByMuscleGroup: volumeByMuscleGroup,
		})
	}
	return apiValues
}

func toAPIWeeklyVolumeTrend(values []analytics.WeeklyVolumeTrendPoint) []generated.WeeklyVolumeTrendPoint {
	apiValues := make([]generated.WeeklyVolumeTrendPoint, 0, len(values))
	for _, value := range values {
		apiValues = append(apiValues, generated.WeeklyVolumeTrendPoint{
			WeekStartDate: toAPIDate(value.WeekStartDate),
			TotalVolumeKg: float32(value.TotalVolumeKg),
		})
	}
	return apiValues
}

func toAPIAdherenceStreaks(streaks analytics.AdherenceStreaks) generated.AdherenceStreaks {
	return generated.AdherenceStreaks{
		Training: toAPIAdherenceStreak(streaks.Training),
		Protein:  toAPIAdherenceStreak(streaks.Protein),
	}
}

func toAPIAdherenceStreak(streak analytics.AdherenceStreak) generated.AdherenceStreak {
	return generated.AdherenceStreak{
		CurrentDays: streak.CurrentDays,
		LongestDays: streak.LongestDays,
	}
}

func toAPIWeightTrendPoints(points []analytics.WeightTrendPoint) []generated.WeightTrendPoint {
	apiPoints := make([]generated.WeightTrendPoint, 0, len(points))
	for _, point := range points {
		apiPoints = append(apiPoints, generated.WeightTrendPoint{
			Date:     toAPIDate(point.Date),
			WeightKg: float32(point.WeightKg),
		})
	}
	return apiPoints
}

func toAPIReadinessSelfReportHistory(
	points []analytics.ReadinessSelfReportPoint,
) []generated.ReadinessSelfReportPoint {
	apiPoints := make([]generated.ReadinessSelfReportPoint, 0, len(points))
	for _, point := range points {
		apiPoints = append(apiPoints, generated.ReadinessSelfReportPoint{
			Date:           toAPIDate(point.Date),
			ReadinessScore: float32(point.ReadinessScore),
			EnergyLevel:    point.EnergyLevel,
			SleepQuality:   point.SleepQuality,
			StressLevel:    point.StressLevel,
		})
	}
	return apiPoints
}

func toAPIDashboardReadinessCheckin(row db.ReadinessCheckin) generated.DashboardReadinessCheckin {
	return generated.DashboardReadinessCheckin{
		UserId:         row.UserID,
		Date:           toAPIDate(row.Date),
		EnergyLevel:    row.EnergyLevel,
		SleepQuality:   row.SleepQuality,
		StressLevel:    row.StressLevel,
		ReadinessScore: float32(row.ReadinessScore),
		CreatedAt:      row.CreatedAt.UTC(),
		UpdatedAt:      row.UpdatedAt.UTC(),
	}
}

func toAPIDate(value time.Time) openapi_types.Date {
	return openapi_types.Date{Time: value.UTC()}
}

func float64PointerToFloat32(value *float64) *float32 {
	if value == nil {
		return nil
	}
	mapped := float32(*value)
	return &mapped
}
