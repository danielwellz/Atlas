package nutrition

import (
	"fmt"
	"math"
	"strings"
)

const (
	minCaloriesTarget int32   = 1200
	maxCaloriesTarget int32   = 5000
	calorieStepSmall  int32   = 100
	calorieStepLarge  int32   = 150
	adherenceFloor    float64 = 0.60
	paceToleranceKg   float64 = 0.15
	defaultBodyWeight float64 = 80.0
)

type MacroTargets struct {
	CaloriesTarget int32 `json:"calories_target"`
	ProteinGTarget int32 `json:"protein_g_target"`
	CarbsGTarget   int32 `json:"carbs_g_target"`
	FatGTarget     int32 `json:"fat_g_target"`
}

type WeeklyAdjustmentInput struct {
	CurrentTargets MacroTargets
	Goal           string
	TrainingPhase  string
	Adherence      float64
	WeightChangeKg float64
	BodyWeightKg   float64
}

type WeeklyAdjustmentResult struct {
	NewTargets        MacroTargets
	GoalPaceKgPerWeek float64
	CalorieDelta      int32
	Explanation       string
}

func RecomputeMacroTargets(calories int32, bodyWeightKg float64, goal string, trainingPhase string) MacroTargets {
	adjustedCalories := clampCalories(calories)
	targets := recomputeMacros(adjustedCalories, bodyWeightKg, normalizeGoalCategory(goal), trainingPhase)
	targets.CaloriesTarget = adjustedCalories
	return targets
}

func ComputeWeeklyAdjustment(input WeeklyAdjustmentInput) WeeklyAdjustmentResult {
	goalCategory := normalizeGoalCategory(input.Goal)
	goalPaceKgPerWeek := inferGoalPace(goalCategory)
	calories := input.CurrentTargets.CaloriesTarget
	if calories <= 0 {
		calories = 2200
	}

	calorieDelta := int32(0)
	reason := "Weight trend is within expected range; keeping calories steady."

	if input.Adherence < adherenceFloor {
		reason = "Adherence is below 60%; keeping calories steady until logging consistency improves."
	} else {
		deltaToGoal := input.WeightChangeKg - goalPaceKgPerWeek
		absDelta := math.Abs(deltaToGoal)
		if absDelta > paceToleranceKg {
			step := calorieStepSmall
			if absDelta >= 0.35 {
				step = calorieStepLarge
			}
			if deltaToGoal > paceToleranceKg {
				calorieDelta = -step
				reason = fmt.Sprintf("Weight is moving %.2f kg/week above target pace; reducing calories by %d.", deltaToGoal, step)
			} else {
				calorieDelta = step
				reason = fmt.Sprintf("Weight is moving %.2f kg/week below target pace; increasing calories by %d.", -deltaToGoal, step)
			}
		}
	}

	adjustedCalories := clampCalories(calories + calorieDelta)
	newTargets := recomputeMacros(adjustedCalories, input.BodyWeightKg, goalCategory, input.TrainingPhase)
	newTargets.CaloriesTarget = adjustedCalories

	explanation := fmt.Sprintf(
		"%s Target pace %.2f kg/week, observed %.2f kg/week, adherence %.0f%%. New targets: %d kcal, %dg protein, %dg carbs, %dg fat.",
		reason,
		goalPaceKgPerWeek,
		input.WeightChangeKg,
		clampZeroToOne(input.Adherence)*100,
		newTargets.CaloriesTarget,
		newTargets.ProteinGTarget,
		newTargets.CarbsGTarget,
		newTargets.FatGTarget,
	)

	return WeeklyAdjustmentResult{
		NewTargets:        newTargets,
		GoalPaceKgPerWeek: goalPaceKgPerWeek,
		CalorieDelta:      calorieDelta,
		Explanation:       explanation,
	}
}

func recomputeMacros(calories int32, bodyWeightKg float64, goalCategory string, trainingPhase string) MacroTargets {
	weightKg := bodyWeightKg
	if !isFinite(weightKg) || weightKg <= 0 {
		weightKg = defaultBodyWeight
	}

	proteinPerKg := proteinMultiplier(goalCategory)
	if strings.EqualFold(strings.TrimSpace(trainingPhase), "deload") && proteinPerKg < 2.0 {
		proteinPerKg = 2.0
	}

	proteinG := roundToNearest5(int32(math.Round(weightKg * proteinPerKg)))
	if proteinG < 90 {
		proteinG = 90
	}

	fatRatio := fatRatioByPhase(trainingPhase)
	if goalCategory == "loss" && fatRatio < 0.30 {
		fatRatio = 0.30
	}
	if goalCategory == "gain" && fatRatio > 0.25 {
		fatRatio = 0.25
	}

	fatG := roundToNearest5(int32(math.Round(float64(calories) * fatRatio / 9.0)))
	if fatG < 35 {
		fatG = 35
	}

	remainingCalories := calories - (proteinG * 4) - (fatG * 9)
	if remainingCalories < 0 {
		fatG = maxInt32(35, roundToNearest5(int32(math.Round(float64(calories-proteinG*4)/9.0))))
		remainingCalories = calories - (proteinG * 4) - (fatG * 9)
	}
	if remainingCalories < 0 {
		proteinG = maxInt32(90, roundToNearest5(int32(math.Round(float64(calories-fatG*9)/4.0))))
		remainingCalories = calories - (proteinG * 4) - (fatG * 9)
	}
	if remainingCalories < 0 {
		remainingCalories = 0
	}

	carbsG := roundToNearest5(int32(math.Round(float64(remainingCalories) / 4.0)))
	if carbsG < 0 {
		carbsG = 0
	}

	return MacroTargets{
		CaloriesTarget: calories,
		ProteinGTarget: proteinG,
		CarbsGTarget:   carbsG,
		FatGTarget:     fatG,
	}
}

func normalizeGoalCategory(goal string) string {
	normalized := strings.ToLower(strings.TrimSpace(goal))
	switch {
	case strings.Contains(normalized, "fat"), strings.Contains(normalized, "lose"), strings.Contains(normalized, "cut"):
		return "loss"
	case strings.Contains(normalized, "gain"), strings.Contains(normalized, "bulk"), strings.Contains(normalized, "build"), strings.Contains(normalized, "muscle"):
		return "gain"
	default:
		return "maintain"
	}
}

func inferGoalPace(goalCategory string) float64 {
	switch goalCategory {
	case "loss":
		return -0.40
	case "gain":
		return 0.25
	default:
		return 0
	}
}

func proteinMultiplier(goalCategory string) float64 {
	switch goalCategory {
	case "loss":
		return 2.2
	case "gain":
		return 1.8
	default:
		return 2.0
	}
}

func fatRatioByPhase(trainingPhase string) float64 {
	normalized := strings.ToLower(strings.TrimSpace(trainingPhase))
	switch normalized {
	case "deload":
		return 0.32
	case "endurance":
		return 0.30
	case "strength":
		return 0.28
	case "hypertrophy":
		return 0.25
	default:
		return 0.27
	}
}

func clampCalories(value int32) int32 {
	if value < minCaloriesTarget {
		return minCaloriesTarget
	}
	if value > maxCaloriesTarget {
		return maxCaloriesTarget
	}
	return value
}

func clampZeroToOne(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

func roundToNearest5(value int32) int32 {
	if value == 0 {
		return 0
	}
	return int32(math.Round(float64(value)/5.0) * 5)
}

func maxInt32(a int32, b int32) int32 {
	if a > b {
		return a
	}
	return b
}

func isFinite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}
