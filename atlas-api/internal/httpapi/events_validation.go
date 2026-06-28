package httpapi

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

const (
	eventOnboardingStarted           = "onboarding_started"
	eventOnboardingGoalSelected      = "onboarding_goal_selected"
	eventOnboardingEquipmentSelected = "onboarding_equipment_selected"
	eventOnboardingScheduleSelected  = "onboarding_schedule_selected"
	eventOnboardingCompleted         = "onboarding_completed"
	eventWorkoutCompleted            = "workout_completed"
)

var (
	eventAllowlist = map[string]map[string]struct{}{
		eventOnboardingStarted: {
			"entry_point": {},
			"platform":    {},
			"app_version": {},
		},
		eventOnboardingGoalSelected: {
			"goal":        {},
			"source":      {},
			"platform":    {},
			"app_version": {},
		},
		eventOnboardingEquipmentSelected: {
			"equipment_count": {},
			"source":          {},
			"platform":        {},
			"app_version":     {},
		},
		eventOnboardingScheduleSelected: {
			"days_per_week": {},
			"source":        {},
			"platform":      {},
			"app_version":   {},
		},
		eventOnboardingCompleted: {
			"goal":            {},
			"days_per_week":   {},
			"equipment_count": {},
			"source":          {},
			"platform":        {},
			"app_version":     {},
		},
		eventWorkoutCompleted: {
			"workout_id":        {},
			"duration_minutes":  {},
			"exercise_count":    {},
			"set_count":         {},
			"completion_source": {},
			"platform":          {},
			"app_version":       {},
		},
	}

	sensitiveKeyMatchers = []string{
		"email",
		"phone",
		"name",
		"address",
		"password",
		"token",
		"ssn",
		"birth",
		"dob",
	}

	emailPattern = regexp.MustCompile(`(?i)[A-Z0-9._%+\-]+@[A-Z0-9.\-]+\.[A-Z]{2,}`)
)

func validateAndSanitizeEventProperties(eventName string, properties map[string]interface{}) (map[string]interface{}, error) {
	allowedProperties, eventAllowed := eventAllowlist[eventName]
	if !eventAllowed {
		return nil, fmt.Errorf("event_name is not supported")
	}
	if len(properties) == 0 {
		return map[string]interface{}{}, nil
	}

	sanitized := make(map[string]interface{}, len(properties))
	for key, value := range properties {
		normalizedKey := strings.TrimSpace(key)
		if normalizedKey == "" {
			return nil, errors.New("properties keys must be non-empty")
		}
		if isSensitivePropertyKey(normalizedKey) {
			return nil, fmt.Errorf("property %q is sensitive and not allowed", normalizedKey)
		}
		if _, ok := allowedProperties[normalizedKey]; !ok {
			return nil, fmt.Errorf("property %q is not allowed for event %q (allowed: %s)", normalizedKey, eventName, strings.Join(sortedMapKeys(allowedProperties), ", "))
		}

		sanitizedValue, err := sanitizePropertyValue(value)
		if err != nil {
			return nil, fmt.Errorf("invalid value for property %q: %w", normalizedKey, err)
		}
		sanitized[normalizedKey] = sanitizedValue
	}

	return sanitized, nil
}

func sanitizePropertyValue(value interface{}) (interface{}, error) {
	switch typed := value.(type) {
	case nil:
		return nil, nil
	case bool:
		return typed, nil
	case float64:
		return typed, nil
	case float32:
		return float64(typed), nil
	case int:
		return typed, nil
	case int8:
		return int64(typed), nil
	case int16:
		return int64(typed), nil
	case int32:
		return int64(typed), nil
	case int64:
		return typed, nil
	case uint:
		return typed, nil
	case uint8:
		return uint64(typed), nil
	case uint16:
		return uint64(typed), nil
	case uint32:
		return uint64(typed), nil
	case uint64:
		return typed, nil
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return "", nil
		}
		if emailPattern.MatchString(trimmed) {
			return "[REDACTED_EMAIL]", nil
		}
		if len(trimmed) > 256 {
			return trimmed[:256], nil
		}
		return trimmed, nil
	case []interface{}:
		normalizedList := make([]interface{}, 0, len(typed))
		for _, item := range typed {
			sanitized, err := sanitizePropertyValue(item)
			if err != nil {
				return nil, err
			}
			normalizedList = append(normalizedList, sanitized)
		}
		return normalizedList, nil
	case []string:
		normalizedList := make([]interface{}, 0, len(typed))
		for _, item := range typed {
			normalizedList = append(normalizedList, strings.TrimSpace(item))
		}
		return normalizedList, nil
	default:
		return nil, fmt.Errorf("unsupported property type %T", value)
	}
}

func isSensitivePropertyKey(key string) bool {
	normalized := strings.ToLower(strings.TrimSpace(key))
	normalized = strings.NewReplacer("_", "", "-", "", " ", "", ".", "").Replace(normalized)

	for _, matcher := range sensitiveKeyMatchers {
		if strings.Contains(normalized, matcher) {
			return true
		}
	}

	return false
}

func sortedMapKeys(input map[string]struct{}) []string {
	keys := make([]string, 0, len(input))
	for key := range input {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
