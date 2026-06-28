package httpapi

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	db "github.com/atlas/atlas-api/internal/db/sqlc"
	"github.com/atlas/atlas-api/internal/httpapi/generated"
	"github.com/atlas/atlas-api/internal/httpapi/middleware"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"go.uber.org/zap"
)

const (
	kgPerPound  = 0.45359237
	poundsPerKg = 2.2046226218487757
)

func (s *Server) PutNutritionTargets(ctx context.Context, request generated.PutNutritionTargetsRequestObject) (generated.PutNutritionTargetsResponseObject, error) {
	if request.Body == nil {
		return generated.PutNutritionTargets400JSONResponse{Message: "request body is required"}, nil
	}

	userID, ok := middleware.AuthenticatedUserID(ctx)
	if !ok {
		return generated.PutNutritionTargets401JSONResponse{Message: "unauthorized"}, nil
	}

	if request.Body.CaloriesTarget <= 0 {
		return generated.PutNutritionTargets400JSONResponse{Message: "calories_target must be greater than 0"}, nil
	}
	if request.Body.ProteinGTarget <= 0 {
		return generated.PutNutritionTargets400JSONResponse{Message: "protein_g_target must be greater than 0"}, nil
	}

	row, err := s.queries.UpsertNutritionTargets(ctx, db.UpsertNutritionTargetsParams{
		UserID:         userID,
		CaloriesTarget: request.Body.CaloriesTarget,
		ProteinGTarget: request.Body.ProteinGTarget,
	})
	if err != nil {
		s.logger.Error("failed upserting nutrition targets", zap.Error(err), zap.String("user_id", userID.String()))
		return generated.PutNutritionTargets500JSONResponse{Message: "could not upsert nutrition targets"}, nil
	}

	return generated.PutNutritionTargets200JSONResponse{
		Targets: toAPINutritionTargets(row),
	}, nil
}

func (s *Server) GetNutritionToday(ctx context.Context, _ generated.GetNutritionTodayRequestObject) (generated.GetNutritionTodayResponseObject, error) {
	userID, ok := middleware.AuthenticatedUserID(ctx)
	if !ok {
		return generated.GetNutritionToday401JSONResponse{Message: "unauthorized"}, nil
	}

	today := normalizeDateUTC(s.currentTime())
	response := generated.NutritionTodayResponse{
		Date:              openapi_types.Date{Time: today},
		TargetsConfigured: false,
	}

	targetsRow, err := s.queries.GetNutritionTargetsByUserID(ctx, userID)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			s.logger.Error("failed fetching nutrition targets", zap.Error(err), zap.String("user_id", userID.String()))
			return generated.GetNutritionToday500JSONResponse{Message: "could not fetch nutrition today"}, nil
		}
	} else {
		targets := toAPINutritionTargets(targetsRow)
		response.Targets = &targets
		response.TargetsConfigured = true
	}

	checkinRow, err := s.queries.GetNutritionDailyCheckinByUserIDAndDate(ctx, db.GetNutritionDailyCheckinByUserIDAndDateParams{
		UserID: userID,
		Date:   today,
	})
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			s.logger.Error("failed fetching nutrition checkin", zap.Error(err), zap.String("user_id", userID.String()), zap.Time("date", today))
			return generated.GetNutritionToday500JSONResponse{Message: "could not fetch nutrition today"}, nil
		}
	} else {
		checkin := toAPINutritionDailyCheckin(checkinRow)
		response.Checkin = &checkin
	}

	return generated.GetNutritionToday200JSONResponse(response), nil
}

func (s *Server) PostNutritionCheckin(ctx context.Context, request generated.PostNutritionCheckinRequestObject) (generated.PostNutritionCheckinResponseObject, error) {
	if request.Body == nil {
		return generated.PostNutritionCheckin400JSONResponse{Message: "request body is required"}, nil
	}

	userID, ok := middleware.AuthenticatedUserID(ctx)
	if !ok {
		return generated.PostNutritionCheckin401JSONResponse{Message: "unauthorized"}, nil
	}

	checkinDate := normalizeDateUTC(s.currentTime())
	if request.Body.Date != nil {
		checkinDate = normalizeDateUTC(request.Body.Date.Time)
	}

	caloriesEstimate := sql.NullInt32{}
	if request.Body.CaloriesEstimate != nil {
		if *request.Body.CaloriesEstimate < 0 {
			return generated.PostNutritionCheckin400JSONResponse{Message: "calories_estimate must be greater than or equal to 0"}, nil
		}
		caloriesEstimate = sql.NullInt32{Int32: *request.Body.CaloriesEstimate, Valid: true}
	}

	proteinEstimate := sql.NullInt32{}
	if request.Body.ProteinGEstimate != nil {
		if *request.Body.ProteinGEstimate < 0 {
			return generated.PostNutritionCheckin400JSONResponse{Message: "protein_g_estimate must be greater than or equal to 0"}, nil
		}
		proteinEstimate = sql.NullInt32{Int32: *request.Body.ProteinGEstimate, Valid: true}
	}

	notes := ""
	if request.Body.Notes != nil {
		notes = strings.TrimSpace(*request.Body.Notes)
	}

	targetsRow, err := s.queries.GetNutritionTargetsByUserID(ctx, userID)
	hasTargets := true
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			hasTargets = false
		} else {
			s.logger.Error("failed fetching nutrition targets for checkin", zap.Error(err), zap.String("user_id", userID.String()))
			return generated.PostNutritionCheckin500JSONResponse{Message: "could not upsert nutrition checkin"}, nil
		}
	}

	hitCalories := false
	hitProtein := false
	if hasTargets && caloriesEstimate.Valid {
		hitCalories = caloriesEstimate.Int32 <= targetsRow.CaloriesTarget
	}
	if hasTargets && proteinEstimate.Valid {
		hitProtein = proteinEstimate.Int32 >= targetsRow.ProteinGTarget
	}

	checkinRow, err := s.queries.UpsertNutritionDailyCheckin(ctx, db.UpsertNutritionDailyCheckinParams{
		UserID:           userID,
		Date:             checkinDate,
		CaloriesEstimate: caloriesEstimate,
		ProteinGEstimate: proteinEstimate,
		HitCalories:      hitCalories,
		HitProtein:       hitProtein,
		Notes:            notes,
	})
	if err != nil {
		s.logger.Error("failed upserting nutrition checkin", zap.Error(err), zap.String("user_id", userID.String()), zap.Time("date", checkinDate))
		return generated.PostNutritionCheckin500JSONResponse{Message: "could not upsert nutrition checkin"}, nil
	}

	return generated.PostNutritionCheckin200JSONResponse{
		Checkin: toAPINutritionDailyCheckin(checkinRow),
	}, nil
}

func (s *Server) PostNutritionWeightEntry(ctx context.Context, request generated.PostNutritionWeightEntryRequestObject) (generated.PostNutritionWeightEntryResponseObject, error) {
	if request.Body == nil {
		return generated.PostNutritionWeightEntry400JSONResponse{Message: "request body is required"}, nil
	}

	userID, ok := middleware.AuthenticatedUserID(ctx)
	if !ok {
		return generated.PostNutritionWeightEntry401JSONResponse{Message: "unauthorized"}, nil
	}

	entry, validationMessage, err := s.upsertNutritionWeightEntry(ctx, userID, request.Body)
	if validationMessage != "" {
		return generated.PostNutritionWeightEntry400JSONResponse{Message: validationMessage}, nil
	}
	if err != nil {
		s.logger.Error("failed upserting nutrition weight entry", zap.Error(err), zap.String("user_id", userID.String()))
		return generated.PostNutritionWeightEntry500JSONResponse{Message: "could not upsert nutrition weight entry"}, nil
	}

	return generated.PostNutritionWeightEntry200JSONResponse{Entry: entry}, nil
}

func (s *Server) PutNutritionWeightEntry(ctx context.Context, request generated.PutNutritionWeightEntryRequestObject) (generated.PutNutritionWeightEntryResponseObject, error) {
	if request.Body == nil {
		return generated.PutNutritionWeightEntry400JSONResponse{Message: "request body is required"}, nil
	}

	userID, ok := middleware.AuthenticatedUserID(ctx)
	if !ok {
		return generated.PutNutritionWeightEntry401JSONResponse{Message: "unauthorized"}, nil
	}

	entry, validationMessage, err := s.upsertNutritionWeightEntry(ctx, userID, request.Body)
	if validationMessage != "" {
		return generated.PutNutritionWeightEntry400JSONResponse{Message: validationMessage}, nil
	}
	if err != nil {
		s.logger.Error("failed upserting nutrition weight entry", zap.Error(err), zap.String("user_id", userID.String()))
		return generated.PutNutritionWeightEntry500JSONResponse{Message: "could not upsert nutrition weight entry"}, nil
	}

	return generated.PutNutritionWeightEntry200JSONResponse{Entry: entry}, nil
}

func (s *Server) GetNutritionWeightTrend(ctx context.Context, _ generated.GetNutritionWeightTrendRequestObject) (generated.GetNutritionWeightTrendResponseObject, error) {
	userID, ok := middleware.AuthenticatedUserID(ctx)
	if !ok {
		return generated.GetNutritionWeightTrend401JSONResponse{Message: "unauthorized"}, nil
	}

	anchorDate := normalizeDateUTC(s.currentTime())
	rows, err := s.queries.ListNutritionWeightTrend(ctx, db.ListNutritionWeightTrendParams{
		AnchorDate: anchorDate,
		UserID:     userID,
	})
	if err != nil {
		s.logger.Error("failed listing nutrition weight trend", zap.Error(err), zap.String("user_id", userID.String()))
		return generated.GetNutritionWeightTrend500JSONResponse{Message: "could not fetch nutrition weight trend"}, nil
	}

	points := make([]generated.NutritionWeightTrendPoint, 0, len(rows))
	for _, row := range rows {
		point := generated.NutritionWeightTrendPoint{
			WeekStartDate: openapi_types.Date{Time: row.WeekStartDate},
		}
		if row.HasEntry {
			entryDate := openapi_types.Date{Time: row.EntryDate}
			unit := generated.WeightUnit(row.Unit)
			weightKg := float32(row.WeightKg)
			weight := float32(fromWeightKg(row.WeightKg, row.Unit))
			point.EntryDate = &entryDate
			point.Unit = &unit
			point.Weight = &weight
			point.WeightKg = &weightKg
		}
		points = append(points, point)
	}

	return generated.GetNutritionWeightTrend200JSONResponse{
		Points: points,
	}, nil
}

func (s *Server) upsertNutritionWeightEntry(ctx context.Context, userID uuid.UUID, body *generated.UpsertWeightEntryRequest) (generated.NutritionWeightEntry, string, error) {
	date := normalizeDateUTC(s.currentTime())
	if body.Date != nil {
		date = normalizeDateUTC(body.Date.Time)
	}

	if body.Weight <= 0 {
		return generated.NutritionWeightEntry{}, "weight must be greater than 0", nil
	}

	unit, ok := normalizeWeightUnit(string(body.Unit))
	if !ok {
		return generated.NutritionWeightEntry{}, "unit must be either kg or lb", nil
	}

	weightKg := toWeightKg(float64(body.Weight), unit)
	row, err := s.queries.UpsertUserWeightEntry(ctx, db.UpsertUserWeightEntryParams{
		UserID:   userID,
		Date:     date,
		WeightKg: weightKg,
		Unit:     unit,
	})
	if err != nil {
		return generated.NutritionWeightEntry{}, "", err
	}

	return toAPINutritionWeightEntry(row), "", nil
}

func toAPINutritionTargets(row db.NutritionTarget) generated.NutritionTargets {
	return generated.NutritionTargets{
		UserId:         openapi_types.UUID(row.UserID),
		CaloriesTarget: row.CaloriesTarget,
		ProteinGTarget: row.ProteinGTarget,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
	}
}

func toAPINutritionDailyCheckin(row db.NutritionDailyCheckin) generated.NutritionDailyCheckin {
	var caloriesEstimate *int32
	if row.CaloriesEstimate.Valid {
		value := row.CaloriesEstimate.Int32
		caloriesEstimate = &value
	}

	var proteinEstimate *int32
	if row.ProteinGEstimate.Valid {
		value := row.ProteinGEstimate.Int32
		proteinEstimate = &value
	}

	return generated.NutritionDailyCheckin{
		Id:               openapi_types.UUID(row.ID),
		UserId:           openapi_types.UUID(row.UserID),
		Date:             openapi_types.Date{Time: row.Date},
		CaloriesEstimate: caloriesEstimate,
		ProteinGEstimate: proteinEstimate,
		HitCalories:      row.HitCalories,
		HitProtein:       row.HitProtein,
		Notes:            row.Notes,
		CreatedAt:        row.CreatedAt,
	}
}

func toAPINutritionWeightEntry(row db.UserWeightEntry) generated.NutritionWeightEntry {
	return generated.NutritionWeightEntry{
		UserId:    openapi_types.UUID(row.UserID),
		Date:      openapi_types.Date{Time: row.Date},
		Weight:    float32(fromWeightKg(row.WeightKg, row.Unit)),
		Unit:      generated.WeightUnit(row.Unit),
		WeightKg:  float32(row.WeightKg),
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}
}

func normalizeWeightUnit(unit string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(unit)) {
	case "kg":
		return "kg", true
	case "lb":
		return "lb", true
	default:
		return "", false
	}
}

func toWeightKg(weight float64, unit string) float64 {
	if unit == "lb" {
		return weight * kgPerPound
	}
	return weight
}

func fromWeightKg(weightKg float64, unit string) float64 {
	if unit == "lb" {
		return weightKg * poundsPerKg
	}
	return weightKg
}

func normalizeDateUTC(value time.Time) time.Time {
	utc := value.UTC()
	return time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC)
}
