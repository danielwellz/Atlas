package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	db "github.com/atlas/atlas-api/internal/db/sqlc"
	"github.com/atlas/atlas-api/internal/habit"
	"github.com/atlas/atlas-api/internal/httpapi/generated"
	"github.com/atlas/atlas-api/internal/httpapi/middleware"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"go.uber.org/zap"
)

func (s *Server) GetHabits(ctx context.Context, request generated.GetHabitsRequestObject) (generated.GetHabitsResponseObject, error) {
	userID, ok := middleware.AuthenticatedUserID(ctx)
	if !ok {
		return generated.GetHabits401JSONResponse{Message: "unauthorized"}, nil
	}

	targetDate := normalizeDateUTC(s.currentTime())
	if request.Params.Date != nil {
		targetDate = normalizeDateUTC(request.Params.Date.Time)
	}

	habitRows, err := s.queries.ListHabitsByUserID(ctx, userID)
	if err != nil {
		s.logger.Error("failed listing habits", zap.Error(err), zap.String("user_id", userID.String()))
		return generated.GetHabits500JSONResponse{Message: "could not list habits"}, nil
	}

	habits := make([]generated.Habit, 0, len(habitRows))
	for _, row := range habitRows {
		logRow, logErr := s.queries.GetHabitDailyLogByHabitIDAndDate(ctx, db.GetHabitDailyLogByHabitIDAndDateParams{
			HabitID: row.ID,
			Date:    targetDate,
		})
		completed := false
		if logErr != nil {
			if !errors.Is(logErr, sql.ErrNoRows) {
				s.logger.Error(
					"failed fetching habit daily log for list",
					zap.Error(logErr),
					zap.String("habit_id", row.ID.String()),
					zap.String("user_id", userID.String()),
					zap.Time("date", targetDate),
				)
				return generated.GetHabits500JSONResponse{Message: "could not list habits"}, nil
			}
		} else {
			completed = logRow.Completed
		}

		mappedHabit, err := toAPIHabit(row, completed)
		if err != nil {
			s.logger.Error("failed mapping habit", zap.Error(err), zap.String("habit_id", row.ID.String()))
			return generated.GetHabits500JSONResponse{Message: "could not list habits"}, nil
		}
		habits = append(habits, mappedHabit)
	}

	return generated.GetHabits200JSONResponse{Habits: habits}, nil
}

func (s *Server) PostHabits(ctx context.Context, request generated.PostHabitsRequestObject) (generated.PostHabitsResponseObject, error) {
	if request.Body == nil {
		return generated.PostHabits400JSONResponse{Message: "request body is required"}, nil
	}

	userID, ok := middleware.AuthenticatedUserID(ctx)
	if !ok {
		return generated.PostHabits401JSONResponse{Message: "unauthorized"}, nil
	}

	name := strings.TrimSpace(request.Body.Name)
	habitType := strings.TrimSpace(request.Body.Type)
	if name == "" {
		return generated.PostHabits400JSONResponse{Message: "name is required"}, nil
	}
	if habitType == "" {
		return generated.PostHabits400JSONResponse{Message: "type is required"}, nil
	}

	target := request.Body.TargetJson
	if target == nil {
		target = map[string]interface{}{}
	}
	targetJSON, err := json.Marshal(target)
	if err != nil {
		return generated.PostHabits400JSONResponse{Message: "invalid target_json"}, nil
	}

	active := true
	if request.Body.Active != nil {
		active = *request.Body.Active
	}

	if active {
		activeCount, err := s.queries.CountActiveHabitsByUserID(ctx, userID)
		if err != nil {
			s.logger.Error("failed counting active habits", zap.Error(err), zap.String("user_id", userID.String()))
			return generated.PostHabits500JSONResponse{Message: "could not create habit"}, nil
		}
		if activeCount >= 3 {
			return generated.PostHabits409JSONResponse{Message: "max 3 active habits allowed"}, nil
		}
	}

	habitRow, err := s.queries.CreateHabit(ctx, db.CreateHabitParams{
		UserID:     userID,
		Name:       name,
		Type:       habitType,
		TargetJson: targetJSON,
		Active:     active,
	})
	if err != nil {
		if isMaxActiveHabitsViolation(err) {
			return generated.PostHabits409JSONResponse{Message: "max 3 active habits allowed"}, nil
		}
		s.logger.Error("failed creating habit", zap.Error(err), zap.String("user_id", userID.String()))
		return generated.PostHabits500JSONResponse{Message: "could not create habit"}, nil
	}

	mappedHabit, err := toAPIHabit(habitRow, false)
	if err != nil {
		s.logger.Error("failed mapping created habit", zap.Error(err), zap.String("habit_id", habitRow.ID.String()))
		return generated.PostHabits500JSONResponse{Message: "could not create habit"}, nil
	}

	return generated.PostHabits201JSONResponse{Habit: mappedHabit}, nil
}

func (s *Server) PostHabitsIdToggleToday(ctx context.Context, request generated.PostHabitsIdToggleTodayRequestObject) (generated.PostHabitsIdToggleTodayResponseObject, error) {
	userID, ok := middleware.AuthenticatedUserID(ctx)
	if !ok {
		return generated.PostHabitsIdToggleToday401JSONResponse{Message: "unauthorized"}, nil
	}

	habitID := uuid.UUID(request.Id)
	if _, err := s.queries.GetHabitByIDAndUserID(ctx, db.GetHabitByIDAndUserIDParams{ID: habitID, UserID: userID}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return generated.PostHabitsIdToggleToday404JSONResponse{Message: "habit not found"}, nil
		}
		s.logger.Error("failed fetching habit for toggle", zap.Error(err), zap.String("habit_id", habitID.String()), zap.String("user_id", userID.String()))
		return generated.PostHabitsIdToggleToday500JSONResponse{Message: "could not toggle habit"}, nil
	}

	now := s.currentTime().UTC()
	todayDate := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	logRow, err := s.queries.GetHabitDailyLogByHabitIDAndDate(ctx, db.GetHabitDailyLogByHabitIDAndDateParams{
		HabitID: habitID,
		Date:    todayDate,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			createdLog, createErr := s.queries.CreateHabitDailyLog(ctx, db.CreateHabitDailyLogParams{
				HabitID:     habitID,
				Date:        todayDate,
				Completed:   true,
				CompletedAt: sql.NullTime{Time: now, Valid: true},
			})
			if createErr != nil {
				s.logger.Error("failed creating habit daily log", zap.Error(createErr), zap.String("habit_id", habitID.String()), zap.String("user_id", userID.String()))
				return generated.PostHabitsIdToggleToday500JSONResponse{Message: "could not toggle habit"}, nil
			}
			return generated.PostHabitsIdToggleToday200JSONResponse{Log: toAPIHabitDailyLog(createdLog)}, nil
		}

		s.logger.Error("failed fetching habit daily log", zap.Error(err), zap.String("habit_id", habitID.String()), zap.String("user_id", userID.String()))
		return generated.PostHabitsIdToggleToday500JSONResponse{Message: "could not toggle habit"}, nil
	}

	updatedCompleted := !logRow.Completed
	updatedCompletedAt := sql.NullTime{}
	if updatedCompleted {
		updatedCompletedAt = sql.NullTime{Time: now, Valid: true}
	}

	updatedLog, err := s.queries.UpdateHabitDailyLogCompletion(ctx, db.UpdateHabitDailyLogCompletionParams{
		ID:          logRow.ID,
		Completed:   updatedCompleted,
		CompletedAt: updatedCompletedAt,
	})
	if err != nil {
		s.logger.Error("failed updating habit daily log", zap.Error(err), zap.String("habit_daily_log_id", logRow.ID.String()), zap.String("habit_id", habitID.String()), zap.String("user_id", userID.String()))
		return generated.PostHabitsIdToggleToday500JSONResponse{Message: "could not toggle habit"}, nil
	}

	return generated.PostHabitsIdToggleToday200JSONResponse{Log: toAPIHabitDailyLog(updatedLog)}, nil
}

func (s *Server) GetHabitsStreaks(ctx context.Context, _ generated.GetHabitsStreaksRequestObject) (generated.GetHabitsStreaksResponseObject, error) {
	userID, ok := middleware.AuthenticatedUserID(ctx)
	if !ok {
		return generated.GetHabitsStreaks401JSONResponse{Message: "unauthorized"}, nil
	}

	habitRows, err := s.queries.ListHabitsByUserID(ctx, userID)
	if err != nil {
		s.logger.Error("failed listing habits for streaks", zap.Error(err), zap.String("user_id", userID.String()))
		return generated.GetHabitsStreaks500JSONResponse{Message: "could not fetch habit streaks"}, nil
	}

	logRows, err := s.queries.ListHabitDailyLogsByUserID(ctx, userID)
	if err != nil {
		s.logger.Error("failed listing habit logs for streaks", zap.Error(err), zap.String("user_id", userID.String()))
		return generated.GetHabitsStreaks500JSONResponse{Message: "could not fetch habit streaks"}, nil
	}

	logsByHabit := make(map[uuid.UUID][]habit.DailyLog, len(habitRows))
	for _, row := range logRows {
		logsByHabit[row.HabitID] = append(logsByHabit[row.HabitID], habit.DailyLog{
			Date:      row.Date,
			Completed: row.Completed,
		})
	}

	now := s.currentTime().UTC()
	streaks := make([]generated.HabitStreak, 0, len(habitRows))
	for _, row := range habitRows {
		currentStreak, longestStreak := habit.CalculateStreaks(logsByHabit[row.ID], now)
		streaks = append(streaks, generated.HabitStreak{
			HabitId:       openapi_types.UUID(row.ID),
			Name:          row.Name,
			Type:          row.Type,
			CurrentStreak: currentStreak,
			LongestStreak: longestStreak,
		})
	}

	return generated.GetHabitsStreaks200JSONResponse{Streaks: streaks}, nil
}

func toAPIHabit(row db.Habit, completed bool) (generated.Habit, error) {
	target := map[string]interface{}{}
	if len(row.TargetJson) > 0 {
		if err := json.Unmarshal(row.TargetJson, &target); err != nil {
			return generated.Habit{}, err
		}
	}

	return generated.Habit{
		Id:         openapi_types.UUID(row.ID),
		UserId:     openapi_types.UUID(row.UserID),
		Name:       row.Name,
		Type:       row.Type,
		TargetJson: target,
		Active:     row.Active,
		Completed:  completed,
		CreatedAt:  row.CreatedAt,
	}, nil
}

func toAPIHabitDailyLog(row db.HabitDailyLog) generated.HabitDailyLog {
	var completedAt *time.Time
	if row.CompletedAt.Valid {
		timestamp := row.CompletedAt.Time
		completedAt = &timestamp
	}

	return generated.HabitDailyLog{
		Id:          openapi_types.UUID(row.ID),
		HabitId:     openapi_types.UUID(row.HabitID),
		Date:        openapi_types.Date{Time: row.Date},
		Completed:   row.Completed,
		CompletedAt: completedAt,
		CreatedAt:   row.CreatedAt,
	}
}

func isMaxActiveHabitsViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23514"
	}
	return false
}
