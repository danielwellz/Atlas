package exercise

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseExercisesCSV(t *testing.T) {
	csvData := strings.Join([]string{
		"id,slug,name,movement_pattern,primary_muscles,secondary_muscles,contraindications,equipment_json,difficulty,description",
		",back-squat,Back Squat,squat,\"[\"\"quads\"\"]\",\"[\"\"glutes\"\", \"\"core\"\"]\",\"[\"\"acute_knee_injury\"\"]\",\"[\"\"barbell\"\", \"\"rack\"\"]\",intermediate,Barbell squat movement.",
	}, "\n")

	rows, err := ParseExercisesCSV(strings.NewReader(csvData))
	require.NoError(t, err)
	require.Len(t, rows, 1)

	row := rows[0]
	require.Nil(t, row.ID)
	require.Equal(t, "back-squat", row.Slug)
	require.Equal(t, "Back Squat", row.Name)
	require.Equal(t, "squat", row.MovementPattern)
	require.Equal(t, []string{"quads"}, row.PrimaryMuscles)
	require.Equal(t, []string{"glutes", "core"}, row.SecondaryMuscles)
	require.Equal(t, []string{"acute_knee_injury"}, row.Contraindications)
	require.Equal(t, []string{"barbell", "rack"}, row.Equipment)
	require.Equal(t, "intermediate", row.Difficulty)
	require.Equal(t, "Barbell squat movement.", row.Description)
}

func TestParseExercisesCSVInvalidJSON(t *testing.T) {
	csvData := strings.Join([]string{
		"id,slug,name,movement_pattern,primary_muscles,secondary_muscles,contraindications,equipment_json,difficulty,description",
		",back-squat,Back Squat,squat,not-json,\"[\"\"glutes\"\"]\",\"[]\",\"[\"\"barbell\"\"]\",intermediate,Barbell squat movement.",
	}, "\n")

	_, err := ParseExercisesCSV(strings.NewReader(csvData))
	require.Error(t, err)
	require.ErrorContains(t, err, "line 2")
	require.ErrorContains(t, err, "invalid primary_muscles")
}

func TestParseExercisesCSVInvalidMovementPattern(t *testing.T) {
	csvData := strings.Join([]string{
		"id,slug,name,movement_pattern,primary_muscles,secondary_muscles,contraindications,equipment_json,difficulty,description",
		",back-squat,Back Squat,invalid-pattern,\"[\"\"quads\"\"]\",\"[\"\"glutes\"\"]\",\"[]\",\"[\"\"barbell\"\"]\",intermediate,Barbell squat movement.",
	}, "\n")

	_, err := ParseExercisesCSV(strings.NewReader(csvData))
	require.Error(t, err)
	require.ErrorContains(t, err, "invalid movement_pattern")
}

func TestParseExercisesCSVInvalidDifficulty(t *testing.T) {
	csvData := strings.Join([]string{
		"id,slug,name,movement_pattern,primary_muscles,secondary_muscles,contraindications,equipment_json,difficulty,description",
		",back-squat,Back Squat,squat,\"[\"\"quads\"\"]\",\"[\"\"glutes\"\"]\",\"[]\",\"[\"\"barbell\"\"]\",expert,Barbell squat movement.",
	}, "\n")

	_, err := ParseExercisesCSV(strings.NewReader(csvData))
	require.Error(t, err)
	require.ErrorContains(t, err, "invalid difficulty")
}
