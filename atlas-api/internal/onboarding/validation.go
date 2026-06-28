package onboarding

import (
	"fmt"
	"strings"
)

const (
	MinDaysPerWeek = 1
	MaxDaysPerWeek = 6

	MinSessionDurationMinutes = 15
	MaxSessionDurationMinutes = 120
)

type GoalsInput struct {
	DaysPerWeek            int32
	SessionDurationMinutes int32
}

type ProfileInput struct {
	DisplayName     string
	Sex             string
	ExperienceLevel string
}

type TrainingProfileInput struct {
	PrimaryGoal            string
	DaysPerWeek            int32
	SessionDurationMinutes int32
	EquipmentAccess        []string
	Constraints            map[string]interface{}
	InjuriesLimitations    []string
	ModalityPreferences    []string
	PriorTrainingHistory   map[string]interface{}
	ReadinessSignals       map[string]interface{}
}

func ValidateGoalsInput(input GoalsInput) error {
	if input.DaysPerWeek < MinDaysPerWeek || input.DaysPerWeek > MaxDaysPerWeek {
		return fmt.Errorf("daysPerWeek must be between %d and %d", MinDaysPerWeek, MaxDaysPerWeek)
	}

	if input.SessionDurationMinutes < MinSessionDurationMinutes || input.SessionDurationMinutes > MaxSessionDurationMinutes {
		return fmt.Errorf("sessionDurationMinutes must be between %d and %d", MinSessionDurationMinutes, MaxSessionDurationMinutes)
	}

	return nil
}

func ValidateProfileInput(input ProfileInput) error {
	if strings.TrimSpace(input.DisplayName) == "" {
		return fmt.Errorf("displayName is required")
	}
	if strings.TrimSpace(input.Sex) == "" {
		return fmt.Errorf("sex is required")
	}
	if strings.TrimSpace(input.ExperienceLevel) == "" {
		return fmt.Errorf("experienceLevel is required")
	}
	return nil
}

func ValidateTrainingProfileInput(input TrainingProfileInput) error {
	if strings.TrimSpace(input.PrimaryGoal) == "" {
		return fmt.Errorf("primaryGoal is required")
	}

	if err := ValidateGoalsInput(GoalsInput{
		DaysPerWeek:            input.DaysPerWeek,
		SessionDurationMinutes: input.SessionDurationMinutes,
	}); err != nil {
		return err
	}

	if len(input.EquipmentAccess) == 0 {
		return fmt.Errorf("equipmentAccessJson must include at least one item")
	}
	if len(extractStringTokens(input.EquipmentAccess)) == 0 {
		return fmt.Errorf("equipmentAccessJson must include at least one non-empty item")
	}

	if len(input.InjuriesLimitations) == 0 {
		return fmt.Errorf("injuriesLimitationsFlags must include at least one item")
	}
	if len(extractStringTokens(input.InjuriesLimitations)) == 0 {
		return fmt.Errorf("injuriesLimitationsFlags must include at least one non-empty item")
	}

	if len(input.ModalityPreferences) == 0 {
		return fmt.Errorf("modalityPreferences must include at least one item")
	}
	if len(extractStringTokens(input.ModalityPreferences)) == 0 {
		return fmt.Errorf("modalityPreferences must include at least one non-empty item")
	}

	scheduleDays := extractScheduleDays(input.Constraints)
	if len(scheduleDays) == 0 {
		return fmt.Errorf("constraintsJson.scheduleDays must include at least one valid day")
	}

	if err := validateOptionalObject("priorTrainingHistory", input.PriorTrainingHistory); err != nil {
		return err
	}

	if err := validateOptionalObject("readinessSignals", input.ReadinessSignals); err != nil {
		return err
	}

	return nil
}

func extractStringTokens(values []string) []string {
	tokens := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		tokens = append(tokens, trimmed)
	}
	return tokens
}

func extractScheduleDays(constraints map[string]interface{}) []string {
	if constraints == nil {
		return []string{}
	}

	rawDays, ok := constraints["scheduleDays"]
	if !ok {
		return []string{}
	}

	dayOrder := map[string]struct{}{
		"monday":    {},
		"tuesday":   {},
		"wednesday": {},
		"thursday":  {},
		"friday":    {},
		"saturday":  {},
		"sunday":    {},
	}

	values, ok := rawDays.([]interface{})
	if !ok {
		return []string{}
	}

	days := make([]string, 0, len(values))
	for _, item := range values {
		dayText, ok := item.(string)
		if !ok {
			continue
		}
		normalized := strings.ToLower(strings.TrimSpace(dayText))
		if normalized == "" {
			continue
		}
		if _, valid := dayOrder[normalized]; !valid {
			continue
		}
		days = append(days, normalized)
	}

	return days
}

func validateOptionalObject(name string, values map[string]interface{}) error {
	if values == nil {
		return nil
	}

	for key := range values {
		if strings.TrimSpace(key) == "" {
			return fmt.Errorf("%s contains an empty key", name)
		}
	}

	return nil
}
