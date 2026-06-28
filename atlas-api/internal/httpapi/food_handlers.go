package httpapi

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	db "github.com/atlas/atlas-api/internal/db/sqlc"
	foodsvc "github.com/atlas/atlas-api/internal/food"
	"github.com/atlas/atlas-api/internal/httpapi/generated"
	"github.com/atlas/atlas-api/internal/httpapi/middleware"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"go.uber.org/zap"
)

func (s *Server) GetFoodsSearch(ctx context.Context, request generated.GetFoodsSearchRequestObject) (generated.GetFoodsSearchResponseObject, error) {
	userID, ok := middleware.AuthenticatedUserID(ctx)
	if !ok {
		return generated.GetFoodsSearch401JSONResponse{Message: "unauthorized"}, nil
	}

	query := strings.TrimSpace(request.Params.Q)
	if query == "" {
		return generated.GetFoodsSearch400JSONResponse{Message: "query parameter q is required"}, nil
	}

	limit := 0
	if request.Params.Limit != nil {
		limit = int(*request.Params.Limit)
	}

	rows, err := s.foodSvc.SearchFoods(ctx, query, limit)
	if err != nil {
		s.logger.Error("failed searching foods", zap.Error(err), zap.String("user_id", userID.String()), zap.String("query", query))
		return generated.GetFoodsSearch500JSONResponse{Message: "could not search foods"}, nil
	}

	foods := make([]generated.Food, 0, len(rows))
	for _, row := range rows {
		mappedFood, mapErr := toAPIFood(row)
		if mapErr != nil {
			s.logger.Error("failed mapping searched food", zap.Error(mapErr), zap.String("food_id", row.ID.String()))
			return generated.GetFoodsSearch500JSONResponse{Message: "could not search foods"}, nil
		}
		foods = append(foods, mappedFood)
	}

	return generated.GetFoodsSearch200JSONResponse{Foods: foods}, nil
}

func (s *Server) GetFoodsUpcCode(ctx context.Context, request generated.GetFoodsUpcCodeRequestObject) (generated.GetFoodsUpcCodeResponseObject, error) {
	userID, ok := middleware.AuthenticatedUserID(ctx)
	if !ok {
		return generated.GetFoodsUpcCode401JSONResponse{Message: "unauthorized"}, nil
	}

	normalizedCode, err := foodsvc.NormalizeUPC(request.Code)
	if err != nil {
		return generated.GetFoodsUpcCode400JSONResponse{Message: "invalid upc code"}, nil
	}

	row, err := s.foodSvc.LookupFoodByUPC(ctx, normalizedCode)
	if err != nil {
		if errors.Is(err, foodsvc.ErrFoodNotFound) {
			return generated.GetFoodsUpcCode404JSONResponse{Message: "food not found"}, nil
		}
		s.logger.Error(
			"failed getting food by upc",
			zap.Error(err),
			zap.String("user_id", userID.String()),
			zap.String("upc", normalizedCode),
		)
		return generated.GetFoodsUpcCode500JSONResponse{Message: "could not fetch food by upc"}, nil
	}

	mappedFood, err := toAPIFood(row)
	if err != nil {
		s.logger.Error("failed mapping food by upc", zap.Error(err), zap.String("food_id", row.ID.String()))
		return generated.GetFoodsUpcCode500JSONResponse{Message: "could not fetch food by upc"}, nil
	}

	return generated.GetFoodsUpcCode200JSONResponse{Food: mappedFood}, nil
}

func (s *Server) GetFoodById(ctx context.Context, request generated.GetFoodByIdRequestObject) (generated.GetFoodByIdResponseObject, error) {
	userID, ok := middleware.AuthenticatedUserID(ctx)
	if !ok {
		return generated.GetFoodById401JSONResponse{Message: "unauthorized"}, nil
	}

	foodID := uuid.UUID(request.Id)
	row, err := s.foodSvc.GetFood(ctx, foodID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return generated.GetFoodById404JSONResponse{Message: "food not found"}, nil
		}
		s.logger.Error("failed getting food detail", zap.Error(err), zap.String("user_id", userID.String()), zap.String("food_id", foodID.String()))
		return generated.GetFoodById500JSONResponse{Message: "could not fetch food detail"}, nil
	}

	mappedFood, err := toAPIFood(row)
	if err != nil {
		s.logger.Error("failed mapping food detail", zap.Error(err), zap.String("food_id", row.ID.String()))
		return generated.GetFoodById500JSONResponse{Message: "could not fetch food detail"}, nil
	}

	return generated.GetFoodById200JSONResponse{Food: mappedFood}, nil
}

func (s *Server) PostFoodLogs(ctx context.Context, request generated.PostFoodLogsRequestObject) (generated.PostFoodLogsResponseObject, error) {
	if request.Body == nil {
		return generated.PostFoodLogs400JSONResponse{Message: "request body is required"}, nil
	}

	userID, ok := middleware.AuthenticatedUserID(ctx)
	if !ok {
		return generated.PostFoodLogs401JSONResponse{Message: "unauthorized"}, nil
	}

	if request.Body.Quantity <= 0 {
		return generated.PostFoodLogs400JSONResponse{Message: "quantity must be greater than 0"}, nil
	}

	loggedAt := s.currentTime().UTC()
	if request.Body.Datetime != nil {
		loggedAt = request.Body.Datetime.UTC()
	}

	unit := ""
	if request.Body.Unit != nil {
		unit = strings.TrimSpace(*request.Body.Unit)
	}

	logRow, foodRow, snapshot, err := s.foodSvc.LogFood(ctx, userID, foodsvc.LogInput{
		FoodID:   uuid.UUID(request.Body.FoodId),
		Datetime: loggedAt,
		Quantity: float64(request.Body.Quantity),
		Unit:     unit,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return generated.PostFoodLogs404JSONResponse{Message: "food not found"}, nil
		}
		s.logger.Error("failed creating food log", zap.Error(err), zap.String("user_id", userID.String()))
		return generated.PostFoodLogs500JSONResponse{Message: "could not create food log"}, nil
	}

	mappedFood, err := toAPIFood(foodRow)
	if err != nil {
		s.logger.Error("failed mapping food row for log", zap.Error(err), zap.String("food_id", foodRow.ID.String()))
		return generated.PostFoodLogs500JSONResponse{Message: "could not create food log"}, nil
	}

	return generated.PostFoodLogs201JSONResponse{
		Log: generated.FoodLog{
			Id:                openapi_types.UUID(logRow.ID),
			UserId:            openapi_types.UUID(logRow.UserID),
			Datetime:          logRow.Datetime,
			FoodId:            openapi_types.UUID(logRow.FoodID),
			Quantity:          float32(logRow.Quantity),
			Unit:              logRow.Unit,
			NutrientsSnapshot: toAPINutrientValues(snapshot),
			CreatedAt:         logRow.CreatedAt,
			Food:              mappedFood,
		},
	}, nil
}

func (s *Server) GetFoodLogs(ctx context.Context, request generated.GetFoodLogsRequestObject) (generated.GetFoodLogsResponseObject, error) {
	userID, ok := middleware.AuthenticatedUserID(ctx)
	if !ok {
		return generated.GetFoodLogs401JSONResponse{Message: "unauthorized"}, nil
	}

	targetDate := normalizeDateUTC(s.currentTime())
	if request.Params.Date != nil {
		targetDate = normalizeDateUTC(request.Params.Date.Time)
	}

	logRows, totals, err := s.foodSvc.ListFoodLogsByDate(ctx, userID, targetDate)
	if err != nil {
		s.logger.Error("failed listing food logs", zap.Error(err), zap.String("user_id", userID.String()), zap.Time("date", targetDate))
		return generated.GetFoodLogs500JSONResponse{Message: "could not list food logs"}, nil
	}

	logs := make([]generated.FoodLog, 0, len(logRows))
	for _, item := range logRows {
		mappedFood, mapErr := toAPIFood(item.Food)
		if mapErr != nil {
			s.logger.Error("failed mapping food for food log list", zap.Error(mapErr), zap.String("food_id", item.Food.ID.String()))
			return generated.GetFoodLogs500JSONResponse{Message: "could not list food logs"}, nil
		}

		logs = append(logs, generated.FoodLog{
			Id:                openapi_types.UUID(item.Log.ID),
			UserId:            openapi_types.UUID(item.Log.UserID),
			Datetime:          item.Log.Datetime,
			FoodId:            openapi_types.UUID(item.Log.FoodID),
			Quantity:          float32(item.Log.Quantity),
			Unit:              item.Log.Unit,
			NutrientsSnapshot: toAPINutrientValues(item.NutrientValues),
			CreatedAt:         item.Log.CreatedAt,
			Food:              mappedFood,
		})
	}

	return generated.GetFoodLogs200JSONResponse{
		Date: openapi_types.Date{Time: targetDate},
		Logs: logs,
		Totals: generated.MacroNutrientTotals{
			CaloriesKcal: float32(totals.CaloriesKcal),
			ProteinG:     float32(totals.ProteinG),
			CarbsG:       float32(totals.CarbsG),
			FatG:         float32(totals.FatG),
		},
	}, nil
}

func toAPIFood(row db.Food) (generated.Food, error) {
	nutrients, err := foodsvc.ParseNutrientsJSON(row.NutrientsJson)
	if err != nil {
		return generated.Food{}, err
	}

	var brand *string
	if strings.TrimSpace(row.Brand) != "" {
		value := row.Brand
		brand = &value
	}

	return generated.Food{
		Id:         openapi_types.UUID(row.ID),
		ExternalId: row.ExternalID,
		Provider:   row.Provider,
		Label:      row.Label,
		Brand:      brand,
		Nutrients:  toAPINutrientValues(nutrients),
		CreatedAt:  row.CreatedAt,
		UpdatedAt:  row.UpdatedAt,
	}, nil
}

func toAPINutrientValues(value foodsvc.Nutrients) generated.NutrientValues {
	return generated.NutrientValues{
		CaloriesKcal: float64ToFloat32Pointer(value.CaloriesKcal),
		ProteinG:     float64ToFloat32Pointer(value.ProteinG),
		CarbsG:       float64ToFloat32Pointer(value.CarbsG),
		FatG:         float64ToFloat32Pointer(value.FatG),
	}
}

func float64ToFloat32Pointer(value *float64) *float32 {
	if value == nil {
		return nil
	}
	converted := float32(*value)
	return &converted
}
