package httpapi

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	db "github.com/atlas/atlas-api/internal/db/sqlc"
	"github.com/atlas/atlas-api/internal/habit"
	"github.com/atlas/atlas-api/internal/httpapi/generated"
	"github.com/atlas/atlas-api/internal/httpapi/middleware"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"go.uber.org/zap"
)

const (
	momentumSprintDurationDays = int32(14)
	momentumSprintDefaultGoal  = "general_fitness"
)

type momentumSprintHabitTemplate struct {
	Key   string
	Label string
}

var momentumSprintHabitTemplatesByGoal = map[string][]momentumSprintHabitTemplate{
	"build_strength": {
		{Key: "training_session", Label: "Complete your strength session"},
		{Key: "protein_target", Label: "Hit your daily protein target"},
		{Key: "mobility_reset", Label: "Complete 10 minutes of mobility"},
	},
	"lose_fat": {
		{Key: "calorie_target", Label: "Stay within your calorie target"},
		{Key: "step_goal", Label: "Hit your daily step goal"},
		{Key: "protein_target", Label: "Hit your daily protein target"},
	},
	"improve_endurance": {
		{Key: "endurance_session", Label: "Complete your endurance session"},
		{Key: "hydration_target", Label: "Hit your hydration target"},
		{Key: "recovery_walk", Label: "Complete a recovery walk"},
	},
	"general_fitness": {
		{Key: "movement_session", Label: "Complete your planned movement session"},
		{Key: "nutrition_check", Label: "Log your nutrition check-in"},
		{Key: "sleep_hygiene", Label: "Protect your evening sleep routine"},
	},
}

var momentumSprintRewardMilestones = []struct {
	Day   int32
	Label string
}{
	{Day: 3, Label: "3-Day Ignition"},
	{Day: 7, Label: "Week-One Lock-In"},
	{Day: 14, Label: "Sprint Finisher"},
}

func (s *Server) PostMomentumSprintEnroll(ctx context.Context, request generated.PostMomentumSprintEnrollRequestObject) (generated.PostMomentumSprintEnrollResponseObject, error) {
	if request.Body == nil {
		return generated.PostMomentumSprintEnroll400JSONResponse{Message: "request body is required"}, nil
	}

	userID, ok := middleware.AuthenticatedUserID(ctx)
	if !ok {
		return generated.PostMomentumSprintEnroll401JSONResponse{Message: "unauthorized"}, nil
	}

	requestedGoal := strings.TrimSpace(request.Body.Goal)
	if requestedGoal == "" {
		return generated.PostMomentumSprintEnroll400JSONResponse{Message: "goal is required"}, nil
	}

	goal := s.resolveMomentumSprintGoal(ctx, userID, requestedGoal)
	now := s.currentTime().UTC()
	startDate := normalizeDateUTC(now)
	endDate := startDate.AddDate(0, 0, int(momentumSprintDurationDays-1))

	enrollment, err := s.queries.UpsertMomentumSprintEnrollment(ctx, db.UpsertMomentumSprintEnrollmentParams{
		UserID:    userID,
		Goal:      goal,
		StartDate: startDate,
		EndDate:   endDate,
	})
	if err != nil {
		s.logger.Error("failed upserting momentum sprint enrollment", zap.Error(err), zap.String("user_id", userID.String()))
		return generated.PostMomentumSprintEnroll500JSONResponse{Message: "could not enroll in momentum sprint"}, nil
	}

	if err := s.queries.DeleteMomentumSprintChecklistEntriesByEnrollmentID(ctx, enrollment.ID); err != nil {
		s.logger.Error("failed deleting momentum sprint checklist entries", zap.Error(err), zap.String("enrollment_id", enrollment.ID.String()), zap.String("user_id", userID.String()))
		return generated.PostMomentumSprintEnroll500JSONResponse{Message: "could not enroll in momentum sprint"}, nil
	}
	if err := s.queries.DeleteMomentumSprintRewardMilestonesByEnrollmentID(ctx, enrollment.ID); err != nil {
		s.logger.Error("failed deleting momentum sprint milestones", zap.Error(err), zap.String("enrollment_id", enrollment.ID.String()), zap.String("user_id", userID.String()))
		return generated.PostMomentumSprintEnroll500JSONResponse{Message: "could not enroll in momentum sprint"}, nil
	}

	templates := momentumSprintHabitTemplatesForGoal(goal)
	if err := s.seedMomentumSprintChecklist(ctx, enrollment.ID, startDate, templates); err != nil {
		s.logger.Error("failed seeding momentum sprint checklist", zap.Error(err), zap.String("enrollment_id", enrollment.ID.String()), zap.String("user_id", userID.String()))
		return generated.PostMomentumSprintEnroll500JSONResponse{Message: "could not enroll in momentum sprint"}, nil
	}

	if err := s.seedMomentumSprintMilestones(ctx, enrollment.ID); err != nil {
		s.logger.Error("failed seeding momentum sprint milestones", zap.Error(err), zap.String("enrollment_id", enrollment.ID.String()), zap.String("user_id", userID.String()))
		return generated.PostMomentumSprintEnroll500JSONResponse{Message: "could not enroll in momentum sprint"}, nil
	}

	status, err := s.buildMomentumSprintStatus(ctx, enrollment, now)
	if err != nil {
		s.logger.Error("failed building momentum sprint status after enrollment", zap.Error(err), zap.String("enrollment_id", enrollment.ID.String()), zap.String("user_id", userID.String()))
		return generated.PostMomentumSprintEnroll500JSONResponse{Message: "could not enroll in momentum sprint"}, nil
	}

	return generated.PostMomentumSprintEnroll200JSONResponse(status), nil
}

func (s *Server) GetMomentumSprintStatus(ctx context.Context, _ generated.GetMomentumSprintStatusRequestObject) (generated.GetMomentumSprintStatusResponseObject, error) {
	userID, ok := middleware.AuthenticatedUserID(ctx)
	if !ok {
		return generated.GetMomentumSprintStatus401JSONResponse{Message: "unauthorized"}, nil
	}

	enrollment, found, err := s.getMomentumSprintEnrollment(ctx, userID)
	if err != nil {
		s.logger.Error("failed fetching momentum sprint enrollment", zap.Error(err), zap.String("user_id", userID.String()))
		return generated.GetMomentumSprintStatus500JSONResponse{Message: "could not fetch momentum sprint status"}, nil
	}
	if !found {
		return generated.GetMomentumSprintStatus200JSONResponse(defaultMomentumSprintStatusResponse()), nil
	}

	status, err := s.buildMomentumSprintStatus(ctx, enrollment, s.currentTime().UTC())
	if err != nil {
		s.logger.Error("failed building momentum sprint status", zap.Error(err), zap.String("enrollment_id", enrollment.ID.String()), zap.String("user_id", userID.String()))
		return generated.GetMomentumSprintStatus500JSONResponse{Message: "could not fetch momentum sprint status"}, nil
	}

	return generated.GetMomentumSprintStatus200JSONResponse(status), nil
}

func (s *Server) PostMomentumSprintDayComplete(ctx context.Context, request generated.PostMomentumSprintDayCompleteRequestObject) (generated.PostMomentumSprintDayCompleteResponseObject, error) {
	if request.Body == nil {
		return generated.PostMomentumSprintDayComplete400JSONResponse{Message: "request body is required"}, nil
	}

	userID, ok := middleware.AuthenticatedUserID(ctx)
	if !ok {
		return generated.PostMomentumSprintDayComplete401JSONResponse{Message: "unauthorized"}, nil
	}

	habitKey := strings.TrimSpace(request.Body.HabitKey)
	if habitKey == "" {
		return generated.PostMomentumSprintDayComplete400JSONResponse{Message: "habitKey is required"}, nil
	}

	enrollment, found, err := s.getMomentumSprintEnrollment(ctx, userID)
	if err != nil {
		s.logger.Error("failed fetching momentum sprint enrollment for day completion", zap.Error(err), zap.String("user_id", userID.String()))
		return generated.PostMomentumSprintDayComplete500JSONResponse{Message: "could not update momentum sprint checklist"}, nil
	}
	if !found {
		return generated.PostMomentumSprintDayComplete404JSONResponse{Message: "momentum sprint not found"}, nil
	}

	targetDate := normalizeDateUTC(s.currentTime().UTC())
	if request.Body.Date != nil {
		targetDate = normalizeDateUTC(request.Body.Date.Time)
	}
	if targetDate.Before(normalizeDateUTC(enrollment.StartDate)) || targetDate.After(normalizeDateUTC(enrollment.EndDate)) {
		return generated.PostMomentumSprintDayComplete400JSONResponse{Message: "date must be within sprint range"}, nil
	}

	completedAt := sql.NullTime{}
	if request.Body.Completed {
		completedAt = sql.NullTime{Time: s.currentTime().UTC(), Valid: true}
	}

	_, err = s.queries.UpdateMomentumSprintChecklistEntryCompletion(ctx, db.UpdateMomentumSprintChecklistEntryCompletionParams{
		EnrollmentID: enrollment.ID,
		Date:         targetDate,
		HabitKey:     habitKey,
		Completed:    request.Body.Completed,
		CompletedAt:  completedAt,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return generated.PostMomentumSprintDayComplete404JSONResponse{Message: "checklist entry not found"}, nil
		}
		s.logger.Error("failed updating momentum sprint checklist entry", zap.Error(err), zap.String("enrollment_id", enrollment.ID.String()), zap.String("habit_key", habitKey), zap.Time("date", targetDate), zap.String("user_id", userID.String()))
		return generated.PostMomentumSprintDayComplete500JSONResponse{Message: "could not update momentum sprint checklist"}, nil
	}

	updatedEnrollment, err := s.syncMomentumSprintCompletionState(ctx, enrollment, s.currentTime().UTC())
	if err != nil {
		s.logger.Error("failed syncing momentum sprint completion state", zap.Error(err), zap.String("enrollment_id", enrollment.ID.String()), zap.String("user_id", userID.String()))
		return generated.PostMomentumSprintDayComplete500JSONResponse{Message: "could not update momentum sprint checklist"}, nil
	}

	status, err := s.buildMomentumSprintStatus(ctx, updatedEnrollment, s.currentTime().UTC())
	if err != nil {
		s.logger.Error("failed rebuilding momentum sprint status", zap.Error(err), zap.String("enrollment_id", updatedEnrollment.ID.String()), zap.String("user_id", userID.String()))
		return generated.PostMomentumSprintDayComplete500JSONResponse{Message: "could not update momentum sprint checklist"}, nil
	}

	return generated.PostMomentumSprintDayComplete200JSONResponse(status), nil
}

func (s *Server) getMomentumSprintEnrollment(ctx context.Context, userID uuid.UUID) (db.MomentumSprintEnrollment, bool, error) {
	enrollment, err := s.queries.GetMomentumSprintEnrollmentByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return db.MomentumSprintEnrollment{}, false, nil
		}
		return db.MomentumSprintEnrollment{}, false, err
	}
	return enrollment, true, nil
}

func (s *Server) resolveMomentumSprintGoal(ctx context.Context, userID uuid.UUID, requestedGoal string) string {
	if strings.TrimSpace(requestedGoal) != "" {
		return normalizeMomentumSprintGoalKey(requestedGoal)
	}

	userGoals, err := s.queries.GetUserGoalsByUserID(ctx, userID)
	if err == nil {
		return normalizeMomentumSprintGoalKey(userGoals.PrimaryGoal)
	}

	return momentumSprintDefaultGoal
}

func normalizeMomentumSprintGoalKey(goal string) string {
	normalized := strings.ToLower(strings.TrimSpace(goal))
	normalized = strings.ReplaceAll(normalized, "-", "_")
	normalized = strings.ReplaceAll(normalized, " ", "_")

	switch normalized {
	case "build_strength", "strength":
		return "build_strength"
	case "lose_fat", "fat_loss", "weight_loss":
		return "lose_fat"
	case "improve_endurance", "endurance":
		return "improve_endurance"
	case "general_fitness", "fitness", "general":
		return "general_fitness"
	default:
		return momentumSprintDefaultGoal
	}
}

func momentumSprintHabitTemplatesForGoal(goal string) []momentumSprintHabitTemplate {
	templates, ok := momentumSprintHabitTemplatesByGoal[goal]
	if ok && len(templates) > 0 {
		return templates
	}
	return momentumSprintHabitTemplatesByGoal[momentumSprintDefaultGoal]
}

func (s *Server) seedMomentumSprintChecklist(ctx context.Context, enrollmentID uuid.UUID, startDate time.Time, templates []momentumSprintHabitTemplate) error {
	for dayOffset := int32(0); dayOffset < momentumSprintDurationDays; dayOffset++ {
		targetDate := normalizeDateUTC(startDate).AddDate(0, 0, int(dayOffset))
		for index, template := range templates {
			_, err := s.queries.CreateMomentumSprintChecklistEntry(ctx, db.CreateMomentumSprintChecklistEntryParams{
				EnrollmentID: enrollmentID,
				Date:         targetDate,
				HabitKey:     template.Key,
				HabitLabel:   template.Label,
				DisplayOrder: int32(index),
				Completed:    false,
				CompletedAt:  sql.NullTime{},
			})
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Server) seedMomentumSprintMilestones(ctx context.Context, enrollmentID uuid.UUID) error {
	for _, milestone := range momentumSprintRewardMilestones {
		_, err := s.queries.CreateMomentumSprintRewardMilestone(ctx, db.CreateMomentumSprintRewardMilestoneParams{
			EnrollmentID: enrollmentID,
			MilestoneDay: milestone.Day,
			RewardLabel:  milestone.Label,
			UnlockedAt:   sql.NullTime{},
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) syncMomentumSprintCompletionState(ctx context.Context, enrollment db.MomentumSprintEnrollment, now time.Time) (db.MomentumSprintEnrollment, error) {
	dailySummaries, err := s.queries.ListMomentumSprintDailySummaryByEnrollmentID(ctx, enrollment.ID)
	if err != nil {
		return db.MomentumSprintEnrollment{}, err
	}

	progressInputs := make([]habit.SprintDailyChecklist, 0, len(dailySummaries))
	for _, row := range dailySummaries {
		progressInputs = append(progressInputs, habit.SprintDailyChecklist{
			Date:             row.Date,
			TotalEntries:     row.TotalEntries,
			CompletedEntries: row.CompletedEntries,
		})
	}

	progress := habit.CalculateSprintProgress(progressInputs, enrollment.StartDate, enrollment.EndDate, now)
	if err := s.queries.UnlockMomentumSprintRewardMilestonesByEnrollmentID(ctx, db.UnlockMomentumSprintRewardMilestonesByEnrollmentIDParams{
		EnrollmentID: enrollment.ID,
		UnlockedAt:   sql.NullTime{Time: now, Valid: true},
		MilestoneDay: progress.CompletedDays,
	}); err != nil {
		return db.MomentumSprintEnrollment{}, err
	}

	completedAt := sql.NullTime{}
	if progress.TotalDays > 0 && progress.CompletedDays >= progress.TotalDays {
		completedAt = sql.NullTime{Time: now, Valid: true}
	}

	return s.queries.UpdateMomentumSprintEnrollmentCompletedAt(ctx, db.UpdateMomentumSprintEnrollmentCompletedAtParams{
		ID:          enrollment.ID,
		CompletedAt: completedAt,
	})
}

func (s *Server) buildMomentumSprintStatus(ctx context.Context, enrollment db.MomentumSprintEnrollment, now time.Time) (generated.MomentumSprintStatusResponse, error) {
	today := normalizeDateUTC(now)

	todayChecklistRows, err := s.queries.ListMomentumSprintChecklistEntriesByEnrollmentIDAndDate(ctx, db.ListMomentumSprintChecklistEntriesByEnrollmentIDAndDateParams{
		EnrollmentID: enrollment.ID,
		Date:         today,
	})
	if err != nil {
		return generated.MomentumSprintStatusResponse{}, err
	}

	dailySummaryRows, err := s.queries.ListMomentumSprintDailySummaryByEnrollmentID(ctx, enrollment.ID)
	if err != nil {
		return generated.MomentumSprintStatusResponse{}, err
	}

	milestoneRows, err := s.queries.ListMomentumSprintRewardMilestonesByEnrollmentID(ctx, enrollment.ID)
	if err != nil {
		return generated.MomentumSprintStatusResponse{}, err
	}

	progressInputs := make([]habit.SprintDailyChecklist, 0, len(dailySummaryRows))
	for _, row := range dailySummaryRows {
		progressInputs = append(progressInputs, habit.SprintDailyChecklist{
			Date:             row.Date,
			TotalEntries:     row.TotalEntries,
			CompletedEntries: row.CompletedEntries,
		})
	}
	progressValue := habit.CalculateSprintProgress(progressInputs, enrollment.StartDate, enrollment.EndDate, today)

	var nextMilestoneDay *int32
	var nextMilestoneLabel *string
	for _, row := range milestoneRows {
		if row.UnlockedAt.Valid {
			continue
		}
		day := row.MilestoneDay
		label := row.RewardLabel
		nextMilestoneDay = &day
		nextMilestoneLabel = &label
		break
	}

	progress := generated.MomentumSprintProgress{
		TotalDays:          progressValue.TotalDays,
		CompletedDays:      progressValue.CompletedDays,
		CurrentDay:         progressValue.CurrentDay,
		DaysRemaining:      progressValue.DaysRemaining,
		CompletionPercent:  progressValue.CompletionPct,
		CurrentStreak:      progressValue.CurrentStreak,
		LongestStreak:      progressValue.LongestStreak,
		CompletedToday:     progressValue.CompletedToday,
		NextMilestoneDay:   nextMilestoneDay,
		NextMilestoneLabel: nextMilestoneLabel,
	}
	apiEnrollment := toAPIMomentumSprintEnrollment(enrollment)

	return generated.MomentumSprintStatusResponse{
		Enrolled:       true,
		Enrollment:     &apiEnrollment,
		Progress:       &progress,
		TodayChecklist: toAPIMomentumSprintChecklistEntries(todayChecklistRows),
		Milestones:     toAPIMomentumSprintRewardMilestones(milestoneRows),
	}, nil
}

func defaultMomentumSprintStatusResponse() generated.MomentumSprintStatusResponse {
	return generated.MomentumSprintStatusResponse{
		Enrolled:       false,
		TodayChecklist: []generated.MomentumSprintChecklistEntry{},
		Milestones:     []generated.MomentumSprintRewardMilestone{},
	}
}

func toAPIMomentumSprintEnrollment(row db.MomentumSprintEnrollment) generated.MomentumSprintEnrollment {
	var completedAt *time.Time
	if row.CompletedAt.Valid {
		value := row.CompletedAt.Time
		completedAt = &value
	}

	return generated.MomentumSprintEnrollment{
		Id:          openapi_types.UUID(row.ID),
		UserId:      openapi_types.UUID(row.UserID),
		Goal:        row.Goal,
		StartDate:   openapi_types.Date{Time: row.StartDate},
		EndDate:     openapi_types.Date{Time: row.EndDate},
		CompletedAt: completedAt,
		CreatedAt:   row.CreatedAt,
	}
}

func toAPIMomentumSprintChecklistEntries(rows []db.MomentumSprintDailyChecklistEntry) []generated.MomentumSprintChecklistEntry {
	entries := make([]generated.MomentumSprintChecklistEntry, 0, len(rows))
	for _, row := range rows {
		var completedAt *time.Time
		if row.CompletedAt.Valid {
			value := row.CompletedAt.Time
			completedAt = &value
		}

		entries = append(entries, generated.MomentumSprintChecklistEntry{
			Id:           openapi_types.UUID(row.ID),
			Date:         openapi_types.Date{Time: row.Date},
			HabitKey:     row.HabitKey,
			HabitLabel:   row.HabitLabel,
			DisplayOrder: row.DisplayOrder,
			Completed:    row.Completed,
			CompletedAt:  completedAt,
			CreatedAt:    row.CreatedAt,
		})
	}
	return entries
}

func toAPIMomentumSprintRewardMilestones(rows []db.MomentumSprintRewardMilestone) []generated.MomentumSprintRewardMilestone {
	milestones := make([]generated.MomentumSprintRewardMilestone, 0, len(rows))
	for _, row := range rows {
		var unlockedAt *time.Time
		if row.UnlockedAt.Valid {
			value := row.UnlockedAt.Time
			unlockedAt = &value
		}

		milestones = append(milestones, generated.MomentumSprintRewardMilestone{
			Id:           openapi_types.UUID(row.ID),
			MilestoneDay: row.MilestoneDay,
			RewardLabel:  row.RewardLabel,
			Unlocked:     row.UnlockedAt.Valid,
			UnlockedAt:   unlockedAt,
			CreatedAt:    row.CreatedAt,
		})
	}
	return milestones
}
