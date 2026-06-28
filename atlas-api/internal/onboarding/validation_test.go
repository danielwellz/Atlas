package onboarding

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateGoalsInput(t *testing.T) {
	err := ValidateGoalsInput(GoalsInput{DaysPerWeek: 3, SessionDurationMinutes: 45})
	require.NoError(t, err)
}

func TestValidateGoalsInputDaysPerWeekOutOfRange(t *testing.T) {
	err := ValidateGoalsInput(GoalsInput{DaysPerWeek: 0, SessionDurationMinutes: 45})
	require.Error(t, err)
	require.ErrorContains(t, err, "daysPerWeek")
}

func TestValidateGoalsInputDurationOutOfRange(t *testing.T) {
	err := ValidateGoalsInput(GoalsInput{DaysPerWeek: 3, SessionDurationMinutes: 130})
	require.Error(t, err)
	require.ErrorContains(t, err, "sessionDurationMinutes")
}

func TestValidateTrainingProfileInput(t *testing.T) {
	err := ValidateTrainingProfileInput(TrainingProfileInput{
		PrimaryGoal:            "build_strength",
		DaysPerWeek:            4,
		SessionDurationMinutes: 60,
		EquipmentAccess:        []string{"barbell", "dumbbell"},
		Constraints: map[string]interface{}{
			"scheduleDays": []interface{}{"Monday", "Wednesday"},
		},
		InjuriesLimitations: []string{"none"},
		ModalityPreferences: []string{"strength"},
		PriorTrainingHistory: map[string]interface{}{
			"yearsConsistent": 2,
		},
		ReadinessSignals: map[string]interface{}{
			"energy": "high",
		},
	})
	require.NoError(t, err)
}

func TestValidateTrainingProfileInputRequiresInjuriesFlags(t *testing.T) {
	err := ValidateTrainingProfileInput(TrainingProfileInput{
		PrimaryGoal:            "build_strength",
		DaysPerWeek:            4,
		SessionDurationMinutes: 60,
		EquipmentAccess:        []string{"barbell"},
		Constraints: map[string]interface{}{
			"scheduleDays": []interface{}{"Monday"},
		},
		InjuriesLimitations: []string{},
		ModalityPreferences: []string{"strength"},
	})
	require.Error(t, err)
	require.ErrorContains(t, err, "injuriesLimitationsFlags")
}

func TestValidateTrainingProfileInputRequiresPreferences(t *testing.T) {
	err := ValidateTrainingProfileInput(TrainingProfileInput{
		PrimaryGoal:            "build_strength",
		DaysPerWeek:            4,
		SessionDurationMinutes: 60,
		EquipmentAccess:        []string{"barbell"},
		Constraints: map[string]interface{}{
			"scheduleDays": []interface{}{"Monday"},
		},
		InjuriesLimitations: []string{"none"},
		ModalityPreferences: []string{},
	})
	require.Error(t, err)
	require.ErrorContains(t, err, "modalityPreferences")
}

func TestValidateTrainingProfileInputRequiresScheduleDay(t *testing.T) {
	err := ValidateTrainingProfileInput(TrainingProfileInput{
		PrimaryGoal:            "build_strength",
		DaysPerWeek:            4,
		SessionDurationMinutes: 60,
		EquipmentAccess:        []string{"barbell"},
		Constraints: map[string]interface{}{
			"timeCap": 45,
		},
		InjuriesLimitations: []string{"none"},
		ModalityPreferences: []string{"strength"},
	})
	require.Error(t, err)
	require.ErrorContains(t, err, "constraintsJson.scheduleDays")
}

func TestValidateTrainingProfileInputRejectsEmptyOptionalObjectKey(t *testing.T) {
	err := ValidateTrainingProfileInput(TrainingProfileInput{
		PrimaryGoal:            "build_strength",
		DaysPerWeek:            4,
		SessionDurationMinutes: 60,
		EquipmentAccess:        []string{"barbell"},
		Constraints: map[string]interface{}{
			"scheduleDays": []interface{}{"Monday"},
		},
		InjuriesLimitations: []string{"none"},
		ModalityPreferences: []string{"strength"},
		ReadinessSignals: map[string]interface{}{
			"": "invalid",
		},
	})
	require.Error(t, err)
	require.ErrorContains(t, err, "readinessSignals")
}
