package httpapi

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateAndSanitizeEventProperties_AllowsConfiguredEventProperties(t *testing.T) {
	t.Parallel()

	properties, err := validateAndSanitizeEventProperties(eventOnboardingCompleted, map[string]interface{}{
		"goal":            "get stronger",
		"days_per_week":   4,
		"equipment_count": 3,
		"source":          "schedule_screen",
		"platform":        "ios",
		"app_version":     "0.0.1",
	})

	require.NoError(t, err)
	require.Equal(t, "get stronger", properties["goal"])
	require.Equal(t, 4, properties["days_per_week"])
}

func TestValidateAndSanitizeEventProperties_RejectsUnknownPropertyKey(t *testing.T) {
	t.Parallel()

	_, err := validateAndSanitizeEventProperties(eventWorkoutCompleted, map[string]interface{}{
		"unknown": true,
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "not allowed")
}

func TestValidateAndSanitizeEventProperties_RejectsSensitivePropertyKey(t *testing.T) {
	t.Parallel()

	_, err := validateAndSanitizeEventProperties(eventWorkoutCompleted, map[string]interface{}{
		"email": "athlete@atlas.local",
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "sensitive")
}

func TestValidateAndSanitizeEventProperties_RedactsEmailAddressValues(t *testing.T) {
	t.Parallel()

	properties, err := validateAndSanitizeEventProperties(eventOnboardingGoalSelected, map[string]interface{}{
		"goal":        "mail me at athlete@atlas.local",
		"source":      "goals_screen",
		"platform":    "ios",
		"app_version": "0.0.1",
	})

	require.NoError(t, err)
	require.Equal(t, "[REDACTED_EMAIL]", properties["goal"])
}

func TestValidateAndSanitizeEventProperties_RejectsUnsupportedPropertyValueType(t *testing.T) {
	t.Parallel()

	_, err := validateAndSanitizeEventProperties(eventOnboardingGoalSelected, map[string]interface{}{
		"goal":        map[string]interface{}{"nested": true},
		"source":      "goals_screen",
		"platform":    "ios",
		"app_version": "0.0.1",
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported")
}
