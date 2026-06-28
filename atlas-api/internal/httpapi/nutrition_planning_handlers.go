package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"path"
	"sort"
	"strings"
	"time"

	db "github.com/atlas/atlas-api/internal/db/sqlc"
	"github.com/atlas/atlas-api/internal/httpapi/generated"
	"github.com/atlas/atlas-api/internal/httpapi/middleware"
	nutritionrules "github.com/atlas/atlas-api/internal/nutrition"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"go.uber.org/zap"
)

type weeklyTargetsPayload struct {
	CaloriesTarget int32 `json:"calories_target"`
	ProteinGTarget int32 `json:"protein_g_target"`
	CarbsGTarget   int32 `json:"carbs_g_target"`
	FatGTarget     int32 `json:"fat_g_target"`
}

type weeklyCheckinPayload struct {
	PreviousTargets   weeklyTargetsPayload `json:"previous_targets"`
	NewTargets        weeklyTargetsPayload `json:"new_targets"`
	Goal              string               `json:"goal,omitempty"`
	TrainingPhase     string               `json:"training_phase,omitempty"`
	CalorieDelta      int32                `json:"calorie_delta,omitempty"`
	GoalPaceKgPerWeek float64              `json:"goal_pace_kg_per_week,omitempty"`
}

type recipeIngredientLine struct {
	Name     string  `json:"name"`
	Quantity float64 `json:"quantity"`
	Unit     string  `json:"unit"`
	Category string  `json:"category"`
}

type recipeDefinition struct {
	Row         db.Recipe
	Ingredients []recipeIngredientLine
}

type aggregatedGroceryItem struct {
	Name     string
	Quantity float64
	Unit     string
	Category string
}

func (s *Server) PostNutritionWeeklyCheckin(
	ctx context.Context,
	request generated.PostNutritionWeeklyCheckinRequestObject,
) (generated.PostNutritionWeeklyCheckinResponseObject, error) {
	userID, ok := middleware.AuthenticatedUserID(ctx)
	if !ok {
		return generated.PostNutritionWeeklyCheckin401JSONResponse{Message: "unauthorized"}, nil
	}

	weekStart := startOfUTCWeek(normalizeDateUTC(s.currentTime()))
	if request.Body != nil && request.Body.WeekStart != nil {
		weekStart = startOfUTCWeek(normalizeDateUTC(request.Body.WeekStart.Time))
	}

	targetsRow, err := s.queries.GetNutritionTargetsByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return generated.PostNutritionWeeklyCheckin400JSONResponse{Message: "nutrition targets must be configured before weekly check-in"}, nil
		}
		s.logger.Error("failed fetching nutrition targets for weekly checkin", zap.Error(err), zap.String("user_id", userID.String()))
		return generated.PostNutritionWeeklyCheckin500JSONResponse{Message: "could not run weekly nutrition check-in"}, nil
	}

	adherence, err := s.calculateWeeklyAdherence(ctx, userID, weekStart)
	if err != nil {
		s.logger.Error("failed calculating weekly adherence", zap.Error(err), zap.String("user_id", userID.String()))
		return generated.PostNutritionWeeklyCheckin500JSONResponse{Message: "could not run weekly nutrition check-in"}, nil
	}

	weightChangeKg, bodyWeightKg, err := s.calculateWeeklyWeightChange(ctx, userID, weekStart)
	if err != nil {
		s.logger.Error("failed calculating weekly weight change", zap.Error(err), zap.String("user_id", userID.String()))
		return generated.PostNutritionWeeklyCheckin500JSONResponse{Message: "could not run weekly nutrition check-in"}, nil
	}
	if bodyWeightKg <= 0 {
		bodyWeightKg, err = s.lookupBodyWeightKg(ctx, userID)
		if err != nil {
			s.logger.Error("failed resolving bodyweight for weekly checkin", zap.Error(err), zap.String("user_id", userID.String()))
			return generated.PostNutritionWeeklyCheckin500JSONResponse{Message: "could not run weekly nutrition check-in"}, nil
		}
	}

	goal, trainingPhase, err := s.resolveGoalAndTrainingPhase(ctx, userID)
	if err != nil {
		s.logger.Error("failed resolving goal and phase for weekly checkin", zap.Error(err), zap.String("user_id", userID.String()))
		return generated.PostNutritionWeeklyCheckin500JSONResponse{Message: "could not run weekly nutrition check-in"}, nil
	}

	adjustment := nutritionrules.ComputeWeeklyAdjustment(nutritionrules.WeeklyAdjustmentInput{
		CurrentTargets: nutritionrules.MacroTargets{
			CaloriesTarget: targetsRow.CaloriesTarget,
			ProteinGTarget: targetsRow.ProteinGTarget,
		},
		Goal:           goal,
		TrainingPhase:  trainingPhase,
		Adherence:      adherence,
		WeightChangeKg: weightChangeKg,
		BodyWeightKg:   bodyWeightKg,
	})

	previousMacroTargets := nutritionrules.RecomputeMacroTargets(
		targetsRow.CaloriesTarget,
		bodyWeightKg,
		goal,
		trainingPhase,
	)
	previousMacroTargets.CaloriesTarget = targetsRow.CaloriesTarget
	previousMacroTargets.ProteinGTarget = targetsRow.ProteinGTarget

	if _, err := s.queries.UpsertNutritionTargets(ctx, db.UpsertNutritionTargetsParams{
		UserID:         userID,
		CaloriesTarget: adjustment.NewTargets.CaloriesTarget,
		ProteinGTarget: adjustment.NewTargets.ProteinGTarget,
	}); err != nil {
		s.logger.Error("failed persisting adjusted nutrition targets", zap.Error(err), zap.String("user_id", userID.String()))
		return generated.PostNutritionWeeklyCheckin500JSONResponse{Message: "could not run weekly nutrition check-in"}, nil
	}

	payload := weeklyCheckinPayload{
		PreviousTargets:   toWeeklyTargetsPayload(previousMacroTargets),
		NewTargets:        toWeeklyTargetsPayload(adjustment.NewTargets),
		Goal:              goal,
		TrainingPhase:     trainingPhase,
		CalorieDelta:      adjustment.CalorieDelta,
		GoalPaceKgPerWeek: adjustment.GoalPaceKgPerWeek,
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		s.logger.Error("failed marshaling weekly targets payload", zap.Error(err), zap.String("user_id", userID.String()))
		return generated.PostNutritionWeeklyCheckin500JSONResponse{Message: "could not run weekly nutrition check-in"}, nil
	}

	checkinRow, err := s.queries.UpsertWeeklyCheckin(ctx, db.UpsertWeeklyCheckinParams{
		UserID:         userID,
		WeekStart:      weekStart,
		Adherence:      adherence,
		WeightChange:   weightChangeKg,
		NewTargetsJson: payloadJSON,
		Explanation:    adjustment.Explanation,
	})
	if err != nil {
		s.logger.Error("failed upserting weekly checkin", zap.Error(err), zap.String("user_id", userID.String()), zap.Time("week_start", weekStart))
		return generated.PostNutritionWeeklyCheckin500JSONResponse{Message: "could not run weekly nutrition check-in"}, nil
	}

	apiCheckin, err := toAPINutritionWeeklyCheckin(checkinRow)
	if err != nil {
		s.logger.Error("failed mapping weekly checkin payload", zap.Error(err), zap.String("user_id", userID.String()))
		return generated.PostNutritionWeeklyCheckin500JSONResponse{Message: "could not run weekly nutrition check-in"}, nil
	}

	return generated.PostNutritionWeeklyCheckin200JSONResponse{Checkin: apiCheckin}, nil
}

func (s *Server) GetNutritionWeeklyCheckinLatest(
	ctx context.Context,
	_ generated.GetNutritionWeeklyCheckinLatestRequestObject,
) (generated.GetNutritionWeeklyCheckinLatestResponseObject, error) {
	userID, ok := middleware.AuthenticatedUserID(ctx)
	if !ok {
		return generated.GetNutritionWeeklyCheckinLatest401JSONResponse{Message: "unauthorized"}, nil
	}

	row, err := s.queries.GetLatestWeeklyCheckinByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return generated.GetNutritionWeeklyCheckinLatest404JSONResponse{Message: "no weekly check-in found"}, nil
		}
		s.logger.Error("failed fetching latest weekly checkin", zap.Error(err), zap.String("user_id", userID.String()))
		return generated.GetNutritionWeeklyCheckinLatest500JSONResponse{Message: "could not fetch weekly nutrition check-in"}, nil
	}

	checkin, err := toAPINutritionWeeklyCheckin(row)
	if err != nil {
		s.logger.Error("failed mapping latest weekly checkin", zap.Error(err), zap.String("user_id", userID.String()))
		return generated.GetNutritionWeeklyCheckinLatest500JSONResponse{Message: "could not fetch weekly nutrition check-in"}, nil
	}

	return generated.GetNutritionWeeklyCheckinLatest200JSONResponse{Checkin: checkin}, nil
}

func (s *Server) PostNutritionMealPlanGenerate(
	ctx context.Context,
	request generated.PostNutritionMealPlanGenerateRequestObject,
) (generated.PostNutritionMealPlanGenerateResponseObject, error) {
	userID, ok := middleware.AuthenticatedUserID(ctx)
	if !ok {
		return generated.PostNutritionMealPlanGenerate401JSONResponse{Message: "unauthorized"}, nil
	}

	weekStart := startOfUTCWeek(normalizeDateUTC(s.currentTime()))
	if request.Body != nil && request.Body.WeekStart != nil {
		weekStart = startOfUTCWeek(normalizeDateUTC(request.Body.WeekStart.Time))
	}

	targets, err := s.resolveTargetsForMealPlan(ctx, userID, weekStart)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return generated.PostNutritionMealPlanGenerate400JSONResponse{Message: "nutrition targets must be configured before meal plan generation"}, nil
		}
		s.logger.Error("failed resolving meal plan targets", zap.Error(err), zap.String("user_id", userID.String()))
		return generated.PostNutritionMealPlanGenerate500JSONResponse{Message: "could not generate meal plan"}, nil
	}

	recipeRows, err := s.queries.ListRecipes(ctx)
	if err != nil {
		s.logger.Error("failed listing recipes", zap.Error(err), zap.String("user_id", userID.String()))
		return generated.PostNutritionMealPlanGenerate500JSONResponse{Message: "could not generate meal plan"}, nil
	}
	if len(recipeRows) == 0 {
		return generated.PostNutritionMealPlanGenerate400JSONResponse{Message: "no recipes are available to generate a meal plan"}, nil
	}

	slotRecipes, fallbackRecipes, err := parseRecipeCatalog(recipeRows)
	if err != nil {
		s.logger.Error("failed parsing recipe catalog", zap.Error(err), zap.String("user_id", userID.String()))
		return generated.PostNutritionMealPlanGenerate500JSONResponse{Message: "could not generate meal plan"}, nil
	}

	targetJSON, err := json.Marshal(targets)
	if err != nil {
		s.logger.Error("failed marshaling meal plan targets", zap.Error(err), zap.String("user_id", userID.String()))
		return generated.PostNutritionMealPlanGenerate500JSONResponse{Message: "could not generate meal plan"}, nil
	}

	mealPlanRow, err := s.queries.UpsertMealPlan(ctx, db.UpsertMealPlanParams{
		UserID:     userID,
		WeekStart:  weekStart,
		TargetJson: targetJSON,
	})
	if err != nil {
		s.logger.Error("failed upserting meal plan", zap.Error(err), zap.String("user_id", userID.String()), zap.Time("week_start", weekStart))
		return generated.PostNutritionMealPlanGenerate500JSONResponse{Message: "could not generate meal plan"}, nil
	}

	if err := s.queries.DeleteMealPlanItemsByMealPlanID(ctx, mealPlanRow.ID); err != nil {
		s.logger.Error("failed clearing meal plan items", zap.Error(err), zap.String("meal_plan_id", mealPlanRow.ID.String()))
		return generated.PostNutritionMealPlanGenerate500JSONResponse{Message: "could not generate meal plan"}, nil
	}
	if err := s.queries.DeleteGroceryItemsByMealPlanID(ctx, mealPlanRow.ID); err != nil {
		s.logger.Error("failed clearing grocery items", zap.Error(err), zap.String("meal_plan_id", mealPlanRow.ID.String()))
		return generated.PostNutritionMealPlanGenerate500JSONResponse{Message: "could not generate meal plan"}, nil
	}

	mealSlots := []string{"breakfast", "lunch", "dinner"}
	groceryByKey := map[string]aggregatedGroceryItem{}

	for dayIndex := 0; dayIndex < 7; dayIndex++ {
		selected := make([]recipeDefinition, 0, len(mealSlots))
		dayCalories := int32(0)
		for slotIndex, slot := range mealSlots {
			recipe := pickRecipeForSlot(slotRecipes, fallbackRecipes, slot, dayIndex+slotIndex)
			selected = append(selected, recipe)
			dayCalories += recipe.Row.CaloriesKcal
		}

		servingScale := calculateServingScale(targets.CaloriesTarget, dayCalories)

		for slotIndex, recipe := range selected {
			mealSlot := mealSlots[slotIndex]
			if _, err := s.queries.CreateMealPlanItem(ctx, db.CreateMealPlanItemParams{
				MealPlanID: mealPlanRow.ID,
				DayOfWeek:  int32(dayIndex + 1),
				MealSlot:   mealSlot,
				RecipeID:   recipe.Row.ID,
				Servings:   servingScale,
			}); err != nil {
				s.logger.Error("failed creating meal plan item", zap.Error(err), zap.String("meal_plan_id", mealPlanRow.ID.String()), zap.Int("day_of_week", dayIndex+1), zap.String("meal_slot", mealSlot))
				return generated.PostNutritionMealPlanGenerate500JSONResponse{Message: "could not generate meal plan"}, nil
			}

			for _, ingredient := range recipe.Ingredients {
				name := strings.TrimSpace(ingredient.Name)
				if name == "" {
					continue
				}
				quantity := ingredient.Quantity * servingScale
				if quantity <= 0 {
					continue
				}

				unit := strings.TrimSpace(ingredient.Unit)
				if unit == "" {
					unit = "item"
				}
				category := strings.TrimSpace(ingredient.Category)
				if category == "" {
					category = "general"
				}

				key := groceryItemKey(name, unit, category)
				existing := groceryByKey[key]
				if existing.Name == "" {
					existing = aggregatedGroceryItem{Name: name, Unit: unit, Category: category}
				}
				existing.Quantity += quantity
				groceryByKey[key] = existing
			}
		}
	}

	if err := s.persistGroceryItems(ctx, mealPlanRow.ID, groceryByKey); err != nil {
		s.logger.Error("failed creating grocery items", zap.Error(err), zap.String("meal_plan_id", mealPlanRow.ID.String()))
		return generated.PostNutritionMealPlanGenerate500JSONResponse{Message: "could not generate meal plan"}, nil
	}

	mealPlan, err := s.toAPINutritionMealPlan(ctx, mealPlanRow)
	if err != nil {
		s.logger.Error("failed mapping generated meal plan", zap.Error(err), zap.String("meal_plan_id", mealPlanRow.ID.String()))
		return generated.PostNutritionMealPlanGenerate500JSONResponse{Message: "could not generate meal plan"}, nil
	}

	return generated.PostNutritionMealPlanGenerate200JSONResponse{MealPlan: mealPlan}, nil
}

func (s *Server) GetNutritionMealPlanLatest(
	ctx context.Context,
	_ generated.GetNutritionMealPlanLatestRequestObject,
) (generated.GetNutritionMealPlanLatestResponseObject, error) {
	userID, ok := middleware.AuthenticatedUserID(ctx)
	if !ok {
		return generated.GetNutritionMealPlanLatest401JSONResponse{Message: "unauthorized"}, nil
	}

	row, err := s.queries.GetLatestMealPlanByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return generated.GetNutritionMealPlanLatest404JSONResponse{Message: "no meal plan found"}, nil
		}
		s.logger.Error("failed fetching latest meal plan", zap.Error(err), zap.String("user_id", userID.String()))
		return generated.GetNutritionMealPlanLatest500JSONResponse{Message: "could not fetch meal plan"}, nil
	}

	mealPlan, err := s.toAPINutritionMealPlan(ctx, row)
	if err != nil {
		s.logger.Error("failed mapping latest meal plan", zap.Error(err), zap.String("meal_plan_id", row.ID.String()))
		return generated.GetNutritionMealPlanLatest500JSONResponse{Message: "could not fetch meal plan"}, nil
	}

	return generated.GetNutritionMealPlanLatest200JSONResponse{MealPlan: mealPlan}, nil
}

func (s *Server) GetNutritionMealPlanByWeek(
	ctx context.Context,
	request generated.GetNutritionMealPlanByWeekRequestObject,
) (generated.GetNutritionMealPlanByWeekResponseObject, error) {
	userID, ok := middleware.AuthenticatedUserID(ctx)
	if !ok {
		return generated.GetNutritionMealPlanByWeek401JSONResponse{Message: "unauthorized"}, nil
	}

	weekStart := startOfUTCWeek(normalizeDateUTC(request.Params.WeekStart.Time))
	row, err := s.queries.GetMealPlanByUserIDAndWeekStart(ctx, db.GetMealPlanByUserIDAndWeekStartParams{
		UserID:    userID,
		WeekStart: weekStart,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return generated.GetNutritionMealPlanByWeek404JSONResponse{Message: "meal plan not found"}, nil
		}
		s.logger.Error("failed fetching meal plan by week", zap.Error(err), zap.String("user_id", userID.String()), zap.Time("week_start", weekStart))
		return generated.GetNutritionMealPlanByWeek500JSONResponse{Message: "could not fetch meal plan"}, nil
	}

	mealPlan, err := s.toAPINutritionMealPlan(ctx, row)
	if err != nil {
		s.logger.Error("failed mapping meal plan by week", zap.Error(err), zap.String("meal_plan_id", row.ID.String()))
		return generated.GetNutritionMealPlanByWeek500JSONResponse{Message: "could not fetch meal plan"}, nil
	}

	return generated.GetNutritionMealPlanByWeek200JSONResponse{MealPlan: mealPlan}, nil
}

func (s *Server) PutNutritionMealPlan(
	ctx context.Context,
	request generated.PutNutritionMealPlanRequestObject,
) (generated.PutNutritionMealPlanResponseObject, error) {
	if request.Body == nil {
		return generated.PutNutritionMealPlan400JSONResponse{Message: "request body is required"}, nil
	}

	userID, ok := middleware.AuthenticatedUserID(ctx)
	if !ok {
		return generated.PutNutritionMealPlan401JSONResponse{Message: "unauthorized"}, nil
	}

	if len(request.Body.Items) == 0 {
		return generated.PutNutritionMealPlan400JSONResponse{Message: "items must include at least one meal entry"}, nil
	}

	weekStart := startOfUTCWeek(normalizeDateUTC(request.Body.WeekStart.Time))
	targets, err := s.resolveTargetsForMealPlan(ctx, userID, weekStart)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return generated.PutNutritionMealPlan400JSONResponse{Message: "nutrition targets must be configured before meal plan updates"}, nil
		}
		s.logger.Error("failed resolving meal plan targets for upsert", zap.Error(err), zap.String("user_id", userID.String()))
		return generated.PutNutritionMealPlan500JSONResponse{Message: "could not save meal plan"}, nil
	}

	recipeRows, err := s.queries.ListRecipes(ctx)
	if err != nil {
		s.logger.Error("failed listing recipes for meal plan update", zap.Error(err), zap.String("user_id", userID.String()))
		return generated.PutNutritionMealPlan500JSONResponse{Message: "could not save meal plan"}, nil
	}
	if len(recipeRows) == 0 {
		return generated.PutNutritionMealPlan400JSONResponse{Message: "no recipes are available to save a meal plan"}, nil
	}

	_, fallbackRecipes, err := parseRecipeCatalog(recipeRows)
	if err != nil {
		s.logger.Error("failed parsing recipe catalog for meal plan update", zap.Error(err), zap.String("user_id", userID.String()))
		return generated.PutNutritionMealPlan500JSONResponse{Message: "could not save meal plan"}, nil
	}

	recipeByID := make(map[uuid.UUID]recipeDefinition, len(fallbackRecipes))
	for _, recipe := range fallbackRecipes {
		recipeByID[recipe.Row.ID] = recipe
	}

	type mealPlanInput struct {
		DayOfWeek int32
		MealSlot  string
		Servings  float64
		Recipe    recipeDefinition
	}
	inputs := make([]mealPlanInput, 0, len(request.Body.Items))
	slotDedup := make(map[string]struct{}, len(request.Body.Items))

	for _, item := range request.Body.Items {
		if item.DayOfWeek < 1 || item.DayOfWeek > 7 {
			return generated.PutNutritionMealPlan400JSONResponse{Message: "day_of_week must be between 1 and 7"}, nil
		}

		mealSlot, validSlot := normalizeMealSlotStrict(item.MealSlot)
		if !validSlot {
			return generated.PutNutritionMealPlan400JSONResponse{Message: "meal_slot must be one of breakfast, lunch, dinner, or snack"}, nil
		}

		servings := float64(item.Servings)
		if servings <= 0 {
			return generated.PutNutritionMealPlan400JSONResponse{Message: "servings must be greater than 0"}, nil
		}

		recipeID := uuid.UUID(item.RecipeId)
		recipe, ok := recipeByID[recipeID]
		if !ok {
			return generated.PutNutritionMealPlan400JSONResponse{Message: fmt.Sprintf("recipe %s is not available", recipeID.String())}, nil
		}

		slotKey := fmt.Sprintf("%d|%s", item.DayOfWeek, mealSlot)
		if _, exists := slotDedup[slotKey]; exists {
			return generated.PutNutritionMealPlan400JSONResponse{Message: "duplicate day_of_week and meal_slot entries are not allowed"}, nil
		}
		slotDedup[slotKey] = struct{}{}

		inputs = append(inputs, mealPlanInput{
			DayOfWeek: item.DayOfWeek,
			MealSlot:  mealSlot,
			Servings:  servings,
			Recipe:    recipe,
		})
	}

	targetJSON, err := json.Marshal(targets)
	if err != nil {
		s.logger.Error("failed marshaling meal plan targets for upsert", zap.Error(err), zap.String("user_id", userID.String()))
		return generated.PutNutritionMealPlan500JSONResponse{Message: "could not save meal plan"}, nil
	}

	mealPlanRow, err := s.queries.UpsertMealPlan(ctx, db.UpsertMealPlanParams{
		UserID:     userID,
		WeekStart:  weekStart,
		TargetJson: targetJSON,
	})
	if err != nil {
		s.logger.Error("failed upserting meal plan", zap.Error(err), zap.String("user_id", userID.String()), zap.Time("week_start", weekStart))
		return generated.PutNutritionMealPlan500JSONResponse{Message: "could not save meal plan"}, nil
	}

	if err := s.queries.DeleteMealPlanItemsByMealPlanID(ctx, mealPlanRow.ID); err != nil {
		s.logger.Error("failed clearing meal plan items before upsert", zap.Error(err), zap.String("meal_plan_id", mealPlanRow.ID.String()))
		return generated.PutNutritionMealPlan500JSONResponse{Message: "could not save meal plan"}, nil
	}
	if err := s.queries.DeleteGroceryItemsByMealPlanID(ctx, mealPlanRow.ID); err != nil {
		s.logger.Error("failed clearing grocery items before upsert", zap.Error(err), zap.String("meal_plan_id", mealPlanRow.ID.String()))
		return generated.PutNutritionMealPlan500JSONResponse{Message: "could not save meal plan"}, nil
	}

	groceryByKey := map[string]aggregatedGroceryItem{}
	for _, input := range inputs {
		if _, err := s.queries.CreateMealPlanItem(ctx, db.CreateMealPlanItemParams{
			MealPlanID: mealPlanRow.ID,
			DayOfWeek:  input.DayOfWeek,
			MealSlot:   input.MealSlot,
			RecipeID:   input.Recipe.Row.ID,
			Servings:   input.Servings,
		}); err != nil {
			s.logger.Error("failed creating meal plan item during upsert", zap.Error(err), zap.String("meal_plan_id", mealPlanRow.ID.String()), zap.Int32("day_of_week", input.DayOfWeek), zap.String("meal_slot", input.MealSlot))
			return generated.PutNutritionMealPlan500JSONResponse{Message: "could not save meal plan"}, nil
		}

		for _, ingredient := range input.Recipe.Ingredients {
			quantity := ingredient.Quantity * input.Servings
			if quantity <= 0 {
				continue
			}
			key := groceryItemKey(ingredient.Name, ingredient.Unit, ingredient.Category)
			existing := groceryByKey[key]
			if existing.Name == "" {
				existing = aggregatedGroceryItem{
					Name:     ingredient.Name,
					Unit:     ingredient.Unit,
					Category: ingredient.Category,
				}
			}
			existing.Quantity += quantity
			groceryByKey[key] = existing
		}
	}

	if err := s.persistGroceryItems(ctx, mealPlanRow.ID, groceryByKey); err != nil {
		s.logger.Error("failed creating grocery items during meal plan upsert", zap.Error(err), zap.String("meal_plan_id", mealPlanRow.ID.String()))
		return generated.PutNutritionMealPlan500JSONResponse{Message: "could not save meal plan"}, nil
	}

	mealPlan, err := s.toAPINutritionMealPlan(ctx, mealPlanRow)
	if err != nil {
		s.logger.Error("failed mapping upserted meal plan", zap.Error(err), zap.String("meal_plan_id", mealPlanRow.ID.String()))
		return generated.PutNutritionMealPlan500JSONResponse{Message: "could not save meal plan"}, nil
	}

	return generated.PutNutritionMealPlan200JSONResponse{MealPlan: mealPlan}, nil
}

func (s *Server) DeleteNutritionMealPlanByWeek(
	ctx context.Context,
	request generated.DeleteNutritionMealPlanByWeekRequestObject,
) (generated.DeleteNutritionMealPlanByWeekResponseObject, error) {
	userID, ok := middleware.AuthenticatedUserID(ctx)
	if !ok {
		return generated.DeleteNutritionMealPlanByWeek401JSONResponse{Message: "unauthorized"}, nil
	}

	weekStart := startOfUTCWeek(normalizeDateUTC(request.Params.WeekStart.Time))
	if s.queryDB == nil {
		s.logger.Error("query db is not configured for meal plan deletion", zap.String("user_id", userID.String()), zap.Time("week_start", weekStart))
		return generated.DeleteNutritionMealPlanByWeek500JSONResponse{Message: "could not delete meal plan"}, nil
	}

	result, err := s.queryDB.ExecContext(ctx, `
		DELETE FROM meal_plans
		WHERE user_id = $1
		  AND week_start = $2
	`, userID, weekStart)
	if err != nil {
		s.logger.Error("failed deleting meal plan by week", zap.Error(err), zap.String("user_id", userID.String()), zap.Time("week_start", weekStart))
		return generated.DeleteNutritionMealPlanByWeek500JSONResponse{Message: "could not delete meal plan"}, nil
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		s.logger.Error("failed resolving deleted meal plan rows", zap.Error(err), zap.String("user_id", userID.String()), zap.Time("week_start", weekStart))
		return generated.DeleteNutritionMealPlanByWeek500JSONResponse{Message: "could not delete meal plan"}, nil
	}
	if rowsAffected == 0 {
		return generated.DeleteNutritionMealPlanByWeek404JSONResponse{Message: "meal plan not found"}, nil
	}

	return generated.DeleteNutritionMealPlanByWeek204Response{}, nil
}

func (s *Server) PostNutritionRecipeImport(
	ctx context.Context,
	request generated.PostNutritionRecipeImportRequestObject,
) (generated.PostNutritionRecipeImportResponseObject, error) {
	if request.Body == nil {
		return generated.PostNutritionRecipeImport400JSONResponse{Message: "request body is required"}, nil
	}

	if _, ok := middleware.AuthenticatedUserID(ctx); !ok {
		return generated.PostNutritionRecipeImport401JSONResponse{Message: "unauthorized"}, nil
	}

	sourceURL := strings.TrimSpace(request.Body.SourceUrl)
	if sourceURL == "" {
		return generated.PostNutritionRecipeImport400JSONResponse{Message: "source_url is required"}, nil
	}
	if _, err := url.ParseRequestURI(sourceURL); err != nil {
		return generated.PostNutritionRecipeImport400JSONResponse{Message: "source_url must be a valid absolute URL"}, nil
	}
	if s.queryDB == nil {
		s.logger.Error("query db is not configured for recipe import", zap.String("source_url", sourceURL))
		return generated.PostNutritionRecipeImport500JSONResponse{Message: "could not import recipe"}, nil
	}

	draft := buildRecipeImportDraft(sourceURL, request.Body.Draft)
	confirm := request.Body.Confirm != nil && *request.Body.Confirm
	if !confirm {
		return generated.PostNutritionRecipeImport200JSONResponse{
			SourceUrl: sourceURL,
			Confirmed: false,
			Draft:     draft,
			Recipe:    nil,
		}, nil
	}

	ingredientsJSON, err := json.Marshal(toRecipeIngredientLines(draft.Ingredients))
	if err != nil {
		s.logger.Error("failed marshaling imported recipe ingredients", zap.Error(err), zap.String("source_url", sourceURL))
		return generated.PostNutritionRecipeImport500JSONResponse{Message: "could not import recipe"}, nil
	}

	instructionsJSON, err := json.Marshal(normalizeInstructionList(draft.Instructions))
	if err != nil {
		s.logger.Error("failed marshaling imported recipe instructions", zap.Error(err), zap.String("source_url", sourceURL))
		return generated.PostNutritionRecipeImport500JSONResponse{Message: "could not import recipe"}, nil
	}

	persisted := db.Recipe{}
	err = s.queryDB.QueryRowContext(ctx, `
		INSERT INTO recipes (
			id,
			slug,
			name,
			meal_type,
			description,
			servings,
			calories_kcal,
			protein_g,
			carbs_g,
			fat_g,
			ingredients_json,
			instructions_json,
			created_at,
			updated_at
		) VALUES (
			$1,
			$2,
			$3,
			$4,
			$5,
			$6,
			$7,
			$8,
			$9,
			$10,
			$11::jsonb,
			$12::jsonb,
			NOW(),
			NOW()
		)
		ON CONFLICT (slug)
		DO UPDATE SET
			name = EXCLUDED.name,
			meal_type = EXCLUDED.meal_type,
			description = EXCLUDED.description,
			servings = EXCLUDED.servings,
			calories_kcal = EXCLUDED.calories_kcal,
			protein_g = EXCLUDED.protein_g,
			carbs_g = EXCLUDED.carbs_g,
			fat_g = EXCLUDED.fat_g,
			ingredients_json = EXCLUDED.ingredients_json,
			instructions_json = EXCLUDED.instructions_json,
			updated_at = NOW()
		RETURNING id, slug, name, meal_type, description, servings, calories_kcal, protein_g, carbs_g, fat_g, ingredients_json, instructions_json, created_at, updated_at
	`,
		uuid.New(),
		draft.Slug,
		draft.Name,
		draft.MealType,
		draft.Description,
		float64(draft.Servings),
		draft.CaloriesKcal,
		draft.ProteinG,
		draft.CarbsG,
		draft.FatG,
		ingredientsJSON,
		instructionsJSON,
	).Scan(
		&persisted.ID,
		&persisted.Slug,
		&persisted.Name,
		&persisted.MealType,
		&persisted.Description,
		&persisted.Servings,
		&persisted.CaloriesKcal,
		&persisted.ProteinG,
		&persisted.CarbsG,
		&persisted.FatG,
		&persisted.IngredientsJson,
		&persisted.InstructionsJson,
		&persisted.CreatedAt,
		&persisted.UpdatedAt,
	)
	if err != nil {
		s.logger.Error("failed importing recipe", zap.Error(err), zap.String("source_url", sourceURL), zap.String("slug", draft.Slug))
		return generated.PostNutritionRecipeImport500JSONResponse{Message: "could not import recipe"}, nil
	}

	apiRecipe, err := toAPINutritionRecipe(persisted)
	if err != nil {
		s.logger.Error("failed mapping imported recipe", zap.Error(err), zap.String("source_url", sourceURL), zap.String("slug", draft.Slug))
		return generated.PostNutritionRecipeImport500JSONResponse{Message: "could not import recipe"}, nil
	}

	return generated.PostNutritionRecipeImport200JSONResponse{
		SourceUrl: sourceURL,
		Confirmed: true,
		Draft:     draft,
		Recipe:    &apiRecipe,
	}, nil
}

func (s *Server) calculateWeeklyAdherence(ctx context.Context, userID uuid.UUID, weekStart time.Time) (float64, error) {
	rows, err := s.queries.ListNutritionDailyCheckinsByDateRange(ctx, db.ListNutritionDailyCheckinsByDateRangeParams{
		UserID:    userID,
		StartDate: weekStart,
		EndDate:   weekStart.AddDate(0, 0, 7),
	})
	if err != nil {
		return 0, err
	}

	trackedDates := map[string]struct{}{}
	for _, row := range rows {
		if row.CaloriesEstimate.Valid || row.ProteinGEstimate.Valid {
			trackedDates[row.Date.Format("2006-01-02")] = struct{}{}
		}
	}

	adherence := float64(len(trackedDates)) / 7.0
	if adherence < 0 {
		return 0, nil
	}
	if adherence > 1 {
		return 1, nil
	}
	return adherence, nil
}

func (s *Server) calculateWeeklyWeightChange(ctx context.Context, userID uuid.UUID, weekStart time.Time) (float64, float64, error) {
	currentWeek, currentErr := s.queries.GetLatestUserWeightEntryByDateRange(ctx, db.GetLatestUserWeightEntryByDateRangeParams{
		UserID:    userID,
		StartDate: weekStart,
		EndDate:   weekStart.AddDate(0, 0, 7),
	})
	if currentErr != nil && !errors.Is(currentErr, sql.ErrNoRows) {
		return 0, 0, currentErr
	}

	previousWeek, previousErr := s.queries.GetLatestUserWeightEntryByDateRange(ctx, db.GetLatestUserWeightEntryByDateRangeParams{
		UserID:    userID,
		StartDate: weekStart.AddDate(0, 0, -7),
		EndDate:   weekStart,
	})
	if previousErr != nil && !errors.Is(previousErr, sql.ErrNoRows) {
		return 0, 0, previousErr
	}

	bodyWeightKg := 0.0
	if currentErr == nil {
		bodyWeightKg = currentWeek.WeightKg
	} else if previousErr == nil {
		bodyWeightKg = previousWeek.WeightKg
	}

	if currentErr == nil && previousErr == nil {
		return currentWeek.WeightKg - previousWeek.WeightKg, bodyWeightKg, nil
	}

	return 0, bodyWeightKg, nil
}

func (s *Server) lookupBodyWeightKg(ctx context.Context, userID uuid.UUID) (float64, error) {
	today := normalizeDateUTC(s.currentTime())
	latestWeight, err := s.queries.GetLatestUserWeightEntryByDateRange(ctx, db.GetLatestUserWeightEntryByDateRangeParams{
		UserID:    userID,
		StartDate: today.AddDate(-5, 0, 0),
		EndDate:   today.AddDate(0, 0, 1),
	})
	if err == nil {
		return latestWeight.WeightKg, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}

	profileRow, err := s.queries.GetUserProfileByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, nil
		}
		return 0, err
	}

	return profileRow.WeightKg, nil
}

func (s *Server) resolveGoalAndTrainingPhase(ctx context.Context, userID uuid.UUID) (string, string, error) {
	goal := "maintenance"
	goalsRow, err := s.queries.GetUserGoalsByUserID(ctx, userID)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return "", "", err
		}
	} else if strings.TrimSpace(goalsRow.PrimaryGoal) != "" {
		goal = goalsRow.PrimaryGoal
	}

	trainingPhase := "general"
	enrollmentRow, err := s.queries.GetUserProgramEnrollmentByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return goal, trainingPhase, nil
		}
		return "", "", err
	}

	programRow, err := s.queries.GetProgramByID(ctx, enrollmentRow.ProgramID)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return "", "", err
		}
	} else {
		trainingPhase = inferTrainingPhase(programRow)
	}

	programStateRow, err := s.queries.GetUserProgramStateByUserIDAndProgramID(ctx, db.GetUserProgramStateByUserIDAndProgramIDParams{
		UserID:    userID,
		ProgramID: enrollmentRow.ProgramID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return goal, trainingPhase, nil
		}
		return "", "", err
	}
	if programStateRow.DeloadFlag {
		trainingPhase = "deload"
	}

	return goal, trainingPhase, nil
}

func inferTrainingPhase(programRow db.Program) string {
	tags := []string{}
	if err := json.Unmarshal(programRow.GoalTagsJson, &tags); err == nil {
		for _, tag := range tags {
			normalized := strings.ToLower(strings.TrimSpace(tag))
			switch {
			case strings.Contains(normalized, "hypertrophy"), strings.Contains(normalized, "muscle"):
				return "hypertrophy"
			case strings.Contains(normalized, "strength"):
				return "strength"
			case strings.Contains(normalized, "endurance"):
				return "endurance"
			}
		}
	}

	fallback := strings.ToLower(strings.TrimSpace(programRow.Slug + " " + programRow.Name))
	switch {
	case strings.Contains(fallback, "hypertrophy"), strings.Contains(fallback, "muscle"):
		return "hypertrophy"
	case strings.Contains(fallback, "strength"):
		return "strength"
	case strings.Contains(fallback, "endurance"):
		return "endurance"
	default:
		return "general"
	}
}

func (s *Server) resolveTargetsForMealPlan(ctx context.Context, userID uuid.UUID, weekStart time.Time) (weeklyTargetsPayload, error) {
	checkinRow, err := s.queries.GetWeeklyCheckinByUserIDAndWeekStart(ctx, db.GetWeeklyCheckinByUserIDAndWeekStartParams{
		UserID:    userID,
		WeekStart: weekStart,
	})
	if err == nil {
		checkinPayload, parseErr := parseWeeklyCheckinPayload(checkinRow.NewTargetsJson)
		if parseErr != nil {
			return weeklyTargetsPayload{}, parseErr
		}
		return checkinPayload.NewTargets, nil
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return weeklyTargetsPayload{}, err
	}

	targetsRow, err := s.queries.GetNutritionTargetsByUserID(ctx, userID)
	if err != nil {
		return weeklyTargetsPayload{}, err
	}

	goal, trainingPhase, err := s.resolveGoalAndTrainingPhase(ctx, userID)
	if err != nil {
		return weeklyTargetsPayload{}, err
	}
	bodyWeightKg, err := s.lookupBodyWeightKg(ctx, userID)
	if err != nil {
		return weeklyTargetsPayload{}, err
	}

	recomputed := nutritionrules.RecomputeMacroTargets(
		targetsRow.CaloriesTarget,
		bodyWeightKg,
		goal,
		trainingPhase,
	)

	return weeklyTargetsPayload{
		CaloriesTarget: recomputed.CaloriesTarget,
		ProteinGTarget: recomputed.ProteinGTarget,
		CarbsGTarget:   recomputed.CarbsGTarget,
		FatGTarget:     recomputed.FatGTarget,
	}, nil
}

func toAPINutritionWeeklyCheckin(row db.WeeklyCheckin) (generated.NutritionWeeklyCheckin, error) {
	payload, err := parseWeeklyCheckinPayload(row.NewTargetsJson)
	if err != nil {
		return generated.NutritionWeeklyCheckin{}, err
	}

	return generated.NutritionWeeklyCheckin{
		UserId:            openapi_types.UUID(row.UserID),
		WeekStart:         openapi_types.Date{Time: row.WeekStart},
		Adherence:         float32(row.Adherence),
		WeightChange:      float32(row.WeightChange),
		CalorieDelta:      payload.CalorieDelta,
		GoalPaceKgPerWeek: float32(payload.GoalPaceKgPerWeek),
		PreviousTargets:   toAPIWeeklyMacroTargets(payload.PreviousTargets),
		NewTargets: generated.WeeklyMacroTargets{
			CaloriesTarget: payload.NewTargets.CaloriesTarget,
			ProteinGTarget: payload.NewTargets.ProteinGTarget,
			CarbsGTarget:   payload.NewTargets.CarbsGTarget,
			FatGTarget:     payload.NewTargets.FatGTarget,
		},
		Explanation: row.Explanation,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}, nil
}

func parseWeeklyCheckinPayload(raw json.RawMessage) (weeklyCheckinPayload, error) {
	payload := weeklyCheckinPayload{}
	if len(raw) == 0 {
		return payload, fmt.Errorf("empty weekly targets payload")
	}

	if err := json.Unmarshal(raw, &payload); err == nil && payload.NewTargets.CaloriesTarget > 0 {
		if payload.PreviousTargets.CaloriesTarget <= 0 {
			payload.PreviousTargets = payload.NewTargets
		}
		return payload, nil
	}

	legacyTargets := weeklyTargetsPayload{}
	if err := json.Unmarshal(raw, &legacyTargets); err != nil {
		return payload, err
	}
	if legacyTargets.CaloriesTarget <= 0 {
		return payload, fmt.Errorf("invalid calories target in weekly payload")
	}

	payload.PreviousTargets = legacyTargets
	payload.NewTargets = legacyTargets
	return payload, nil
}

func parseWeeklyTargetsPayload(raw json.RawMessage) (weeklyTargetsPayload, error) {
	payload := weeklyTargetsPayload{}
	if len(raw) == 0 {
		return payload, fmt.Errorf("empty weekly targets payload")
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return payload, err
	}
	if payload.CaloriesTarget <= 0 {
		return payload, fmt.Errorf("invalid calories target in weekly payload")
	}
	return payload, nil
}

func (s *Server) toAPINutritionMealPlan(ctx context.Context, row db.MealPlan) (generated.NutritionMealPlan, error) {
	itemRows, err := s.queries.ListMealPlanItemsByMealPlanID(ctx, row.ID)
	if err != nil {
		return generated.NutritionMealPlan{}, err
	}
	groceryRows, err := s.queries.ListGroceryItemsByMealPlanID(ctx, row.ID)
	if err != nil {
		return generated.NutritionMealPlan{}, err
	}
	targets, err := parseWeeklyTargetsPayload(row.TargetJson)
	if err != nil {
		return generated.NutritionMealPlan{}, err
	}

	items := make([]generated.NutritionMealPlanItem, 0, len(itemRows))
	for _, itemRow := range itemRows {
		ingredients, err := parseRecipeIngredients(itemRow.RecipeIngredientsJson)
		if err != nil {
			return generated.NutritionMealPlan{}, err
		}

		var description *string
		if trimmed := strings.TrimSpace(itemRow.RecipeDescription); trimmed != "" {
			description = &trimmed
		}

		items = append(items, generated.NutritionMealPlanItem{
			DayOfWeek: itemRow.DayOfWeek,
			MealSlot:  itemRow.MealSlot,
			Servings:  float32(itemRow.Servings),
			Recipe: generated.NutritionRecipe{
				Id:           openapi_types.UUID(itemRow.RecipeID),
				Slug:         itemRow.RecipeSlug,
				Name:         itemRow.RecipeName,
				MealType:     itemRow.RecipeMealType,
				Description:  description,
				Servings:     float32(itemRow.RecipeServings),
				CaloriesKcal: itemRow.RecipeCaloriesKcal,
				ProteinG:     itemRow.RecipeProteinG,
				CarbsG:       itemRow.RecipeCarbsG,
				FatG:         itemRow.RecipeFatG,
				Ingredients:  ingredients,
			},
		})
	}

	groceries := make([]generated.NutritionGroceryItem, 0, len(groceryRows))
	for _, groceryRow := range groceryRows {
		groceries = append(groceries, generated.NutritionGroceryItem{
			Name:     groceryRow.Name,
			Quantity: float32(groceryRow.Quantity),
			Unit:     groceryRow.Unit,
			Category: groceryRow.Category,
		})
	}

	return generated.NutritionMealPlan{
		Id:        openapi_types.UUID(row.ID),
		UserId:    openapi_types.UUID(row.UserID),
		WeekStart: openapi_types.Date{Time: row.WeekStart},
		Targets: generated.WeeklyMacroTargets{
			CaloriesTarget: targets.CaloriesTarget,
			ProteinGTarget: targets.ProteinGTarget,
			CarbsGTarget:   targets.CarbsGTarget,
			FatGTarget:     targets.FatGTarget,
		},
		Items:        items,
		GroceryItems: groceries,
		CreatedAt:    row.CreatedAt,
		UpdatedAt:    row.UpdatedAt,
	}, nil
}

func toWeeklyTargetsPayload(targets nutritionrules.MacroTargets) weeklyTargetsPayload {
	return weeklyTargetsPayload{
		CaloriesTarget: targets.CaloriesTarget,
		ProteinGTarget: targets.ProteinGTarget,
		CarbsGTarget:   targets.CarbsGTarget,
		FatGTarget:     targets.FatGTarget,
	}
}

func toAPIWeeklyMacroTargets(targets weeklyTargetsPayload) generated.WeeklyMacroTargets {
	return generated.WeeklyMacroTargets{
		CaloriesTarget: targets.CaloriesTarget,
		ProteinGTarget: targets.ProteinGTarget,
		CarbsGTarget:   targets.CarbsGTarget,
		FatGTarget:     targets.FatGTarget,
	}
}

func (s *Server) persistGroceryItems(ctx context.Context, mealPlanID uuid.UUID, groceryByKey map[string]aggregatedGroceryItem) error {
	keys := make([]string, 0, len(groceryByKey))
	for key := range groceryByKey {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		item := groceryByKey[key]
		if _, err := s.queries.CreateGroceryItem(ctx, db.CreateGroceryItemParams{
			MealPlanID: mealPlanID,
			Name:       item.Name,
			Quantity:   roundToThreeDecimals(item.Quantity),
			Unit:       item.Unit,
			Category:   item.Category,
		}); err != nil {
			return err
		}
	}

	return nil
}

func toAPINutritionRecipe(row db.Recipe) (generated.NutritionRecipe, error) {
	ingredients, err := parseRecipeIngredients(row.IngredientsJson)
	if err != nil {
		return generated.NutritionRecipe{}, err
	}

	var description *string
	if trimmed := strings.TrimSpace(row.Description); trimmed != "" {
		description = &trimmed
	}

	return generated.NutritionRecipe{
		Id:           openapi_types.UUID(row.ID),
		Slug:         row.Slug,
		Name:         row.Name,
		MealType:     row.MealType,
		Description:  description,
		Servings:     float32(row.Servings),
		CaloriesKcal: row.CaloriesKcal,
		ProteinG:     row.ProteinG,
		CarbsG:       row.CarbsG,
		FatG:         row.FatG,
		Ingredients:  ingredients,
	}, nil
}

func buildRecipeImportDraft(sourceURL string, provided *generated.NutritionRecipeImportDraft) generated.NutritionRecipeImportDraft {
	defaultSlug := inferRecipeSlugFromURL(sourceURL)
	if defaultSlug == "" {
		defaultSlug = fmt.Sprintf("imported-recipe-%d", time.Now().UTC().Unix())
	}
	defaultName := inferRecipeNameFromSlug(defaultSlug)
	if defaultName == "" {
		defaultName = "Imported Recipe"
	}

	description := "Imported recipe."
	if parsedURL, err := url.Parse(sourceURL); err == nil {
		host := strings.TrimSpace(parsedURL.Hostname())
		if host != "" {
			description = fmt.Sprintf("Imported from %s.", host)
		}
	}

	draft := generated.NutritionRecipeImportDraft{
		Slug:         defaultSlug,
		Name:         defaultName,
		MealType:     "lunch",
		Description:  description,
		Servings:     1,
		CaloriesKcal: 0,
		ProteinG:     0,
		CarbsG:       0,
		FatG:         0,
		Ingredients:  []generated.NutritionRecipeIngredient{},
		Instructions: []string{},
	}

	if provided == nil {
		return draft
	}

	if slug := slugifyRecipe(provided.Slug); slug != "" {
		draft.Slug = slug
	}

	if name := strings.TrimSpace(provided.Name); name != "" {
		draft.Name = name
	} else if inferred := inferRecipeNameFromSlug(draft.Slug); inferred != "" {
		draft.Name = inferred
	}

	if mealType, ok := normalizeMealSlotStrict(provided.MealType); ok {
		draft.MealType = mealType
	}

	if desc := strings.TrimSpace(provided.Description); desc != "" {
		draft.Description = desc
	}

	if provided.Servings > 0 {
		draft.Servings = provided.Servings
	}

	draft.CaloriesKcal = clampMacroInt32(provided.CaloriesKcal)
	draft.ProteinG = clampMacroInt32(provided.ProteinG)
	draft.CarbsG = clampMacroInt32(provided.CarbsG)
	draft.FatG = clampMacroInt32(provided.FatG)

	lines := toRecipeIngredientLines(provided.Ingredients)
	draft.Ingredients = make([]generated.NutritionRecipeIngredient, 0, len(lines))
	for _, line := range lines {
		draft.Ingredients = append(draft.Ingredients, generated.NutritionRecipeIngredient{
			Name:     line.Name,
			Quantity: float32(line.Quantity),
			Unit:     line.Unit,
			Category: line.Category,
		})
	}

	draft.Instructions = normalizeInstructionList(provided.Instructions)
	return draft
}

func inferRecipeSlugFromURL(sourceURL string) string {
	parsed, err := url.Parse(sourceURL)
	if err != nil {
		return ""
	}

	base := strings.TrimSpace(path.Base(strings.Trim(parsed.Path, "/")))
	if base != "" && base != "." && base != "/" {
		if slug := slugifyRecipe(base); slug != "" {
			return slug
		}
	}

	return slugifyRecipe(parsed.Hostname())
}

func inferRecipeNameFromSlug(slug string) string {
	tokens := strings.Split(strings.ReplaceAll(strings.TrimSpace(slug), "_", "-"), "-")
	parts := make([]string, 0, len(tokens))
	for _, token := range tokens {
		trimmed := strings.TrimSpace(token)
		if trimmed == "" {
			continue
		}
		parts = append(parts, strings.ToUpper(trimmed[:1])+trimmed[1:])
	}
	return strings.Join(parts, " ")
}

func slugifyRecipe(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" {
		return ""
	}

	builder := strings.Builder{}
	prevDash := false
	for _, char := range normalized {
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') {
			builder.WriteRune(char)
			prevDash = false
			continue
		}

		if !prevDash {
			builder.WriteRune('-')
			prevDash = true
		}
	}

	return strings.Trim(builder.String(), "-")
}

func toRecipeIngredientLines(ingredients []generated.NutritionRecipeIngredient) []recipeIngredientLine {
	lines := make([]recipeIngredientLine, 0, len(ingredients))
	for _, ingredient := range ingredients {
		name := strings.TrimSpace(ingredient.Name)
		if name == "" {
			continue
		}
		quantity := float64(ingredient.Quantity)
		if quantity <= 0 {
			continue
		}

		unit := strings.TrimSpace(ingredient.Unit)
		if unit == "" {
			unit = "item"
		}
		category := strings.TrimSpace(ingredient.Category)
		if category == "" {
			category = "general"
		}

		lines = append(lines, recipeIngredientLine{
			Name:     name,
			Quantity: roundToThreeDecimals(quantity),
			Unit:     unit,
			Category: category,
		})
	}

	return lines
}

func normalizeInstructionList(instructions []string) []string {
	if len(instructions) == 0 {
		return []string{}
	}

	normalized := make([]string, 0, len(instructions))
	for _, instruction := range instructions {
		trimmed := strings.TrimSpace(instruction)
		if trimmed == "" {
			continue
		}
		normalized = append(normalized, trimmed)
	}

	return normalized
}

func clampMacroInt32(value int32) int32 {
	if value < 0 {
		return 0
	}
	return value
}

func parseRecipeCatalog(rows []db.Recipe) (map[string][]recipeDefinition, []recipeDefinition, error) {
	slotRecipes := map[string][]recipeDefinition{}
	fallback := make([]recipeDefinition, 0, len(rows))

	for _, row := range rows {
		ingredients, err := parseRecipeIngredientLines(row.IngredientsJson)
		if err != nil {
			return nil, nil, err
		}

		definition := recipeDefinition{Row: row, Ingredients: ingredients}
		fallback = append(fallback, definition)

		slot := normalizeMealSlot(row.MealType)
		slotRecipes[slot] = append(slotRecipes[slot], definition)
	}

	for slot := range slotRecipes {
		sort.Slice(slotRecipes[slot], func(i, j int) bool {
			return slotRecipes[slot][i].Row.Name < slotRecipes[slot][j].Row.Name
		})
	}

	sort.Slice(fallback, func(i, j int) bool {
		return fallback[i].Row.Name < fallback[j].Row.Name
	})

	return slotRecipes, fallback, nil
}

func parseRecipeIngredients(raw json.RawMessage) ([]generated.NutritionRecipeIngredient, error) {
	lines, err := parseRecipeIngredientLines(raw)
	if err != nil {
		return nil, err
	}

	ingredients := make([]generated.NutritionRecipeIngredient, 0, len(lines))
	for _, ingredient := range lines {
		ingredients = append(ingredients, generated.NutritionRecipeIngredient{
			Name:     ingredient.Name,
			Quantity: float32(ingredient.Quantity),
			Unit:     ingredient.Unit,
			Category: ingredient.Category,
		})
	}

	return ingredients, nil
}

func parseRecipeIngredientLines(raw json.RawMessage) ([]recipeIngredientLine, error) {
	parsed := []recipeIngredientLine{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &parsed); err != nil {
			return nil, err
		}
	}

	ingredients := make([]recipeIngredientLine, 0, len(parsed))
	for _, ingredient := range parsed {
		name := strings.TrimSpace(ingredient.Name)
		if name == "" {
			continue
		}
		quantity := ingredient.Quantity
		if quantity <= 0 {
			continue
		}

		unit := strings.TrimSpace(ingredient.Unit)
		if unit == "" {
			unit = "item"
		}
		category := strings.TrimSpace(ingredient.Category)
		if category == "" {
			category = "general"
		}

		ingredients = append(ingredients, recipeIngredientLine{
			Name:     name,
			Quantity: quantity,
			Unit:     unit,
			Category: category,
		})
	}

	return ingredients, nil
}

func pickRecipeForSlot(
	slotRecipes map[string][]recipeDefinition,
	fallbackRecipes []recipeDefinition,
	slot string,
	index int,
) recipeDefinition {
	candidateSlot := normalizeMealSlot(slot)
	recipes := slotRecipes[candidateSlot]
	if len(recipes) == 0 {
		recipes = fallbackRecipes
	}
	if len(recipes) == 0 {
		return recipeDefinition{}
	}

	position := index % len(recipes)
	if position < 0 {
		position += len(recipes)
	}
	return recipes[position]
}

func normalizeMealSlot(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch normalized {
	case "breakfast", "lunch", "dinner", "snack":
		return normalized
	default:
		return "lunch"
	}
}

func normalizeMealSlotStrict(value string) (string, bool) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch normalized {
	case "breakfast", "lunch", "dinner", "snack":
		return normalized, true
	default:
		return "", false
	}
}

func calculateServingScale(targetCalories int32, dayCalories int32) float64 {
	if targetCalories <= 0 || dayCalories <= 0 {
		return 1
	}
	rawScale := float64(targetCalories) / float64(dayCalories)
	if rawScale < 0.75 {
		rawScale = 0.75
	}
	if rawScale > 1.35 {
		rawScale = 1.35
	}
	return roundToThreeDecimals(rawScale)
}

func groceryItemKey(name string, unit string, category string) string {
	return strings.ToLower(strings.TrimSpace(name)) + "|" + strings.ToLower(strings.TrimSpace(unit)) + "|" + strings.ToLower(strings.TrimSpace(category))
}

func roundToThreeDecimals(value float64) float64 {
	return float64(int(value*1000+0.5)) / 1000
}

func startOfUTCWeek(value time.Time) time.Time {
	utcDay := normalizeDateUTC(value)
	weekday := int(utcDay.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	return utcDay.AddDate(0, 0, -(weekday - 1))
}
