package exercise

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestRankSubstitutesPrioritizesPatternAndMuscleOverlap(t *testing.T) {
	prescribed := CatalogExercise{
		ID:               mustUUID(t, "00000000-0000-0000-0000-000000000001"),
		Name:             "Barbell Bench Press",
		MovementPattern:  "push",
		MovementTaxonomy: []string{"push", "horizontal_press"},
		PrimaryMuscles:   []string{"chest", "triceps"},
		SecondaryMuscles: []string{"front_delts"},
	}

	candidates := []CatalogExercise{
		{
			ID:                    mustUUID(t, "00000000-0000-0000-0000-000000000010"),
			Name:                  "Dumbbell Bench Press",
			MovementPattern:       "push",
			MovementTaxonomy:      []string{"push", "horizontal_press"},
			PrimaryMuscles:        []string{"chest", "triceps"},
			SecondaryMuscles:      []string{"front_delts"},
			EquipmentRequirements: []string{"dumbbell", "bench"},
		},
		{
			ID:                    mustUUID(t, "00000000-0000-0000-0000-000000000011"),
			Name:                  "Machine Chest Press",
			MovementPattern:       "push",
			MovementTaxonomy:      []string{"push"},
			PrimaryMuscles:        []string{"chest"},
			SecondaryMuscles:      []string{"triceps"},
			EquipmentRequirements: []string{"machine"},
		},
		{
			ID:                    mustUUID(t, "00000000-0000-0000-0000-000000000012"),
			Name:                  "Barbell Row",
			MovementPattern:       "pull",
			MovementTaxonomy:      []string{"pull", "horizontal_pull"},
			PrimaryMuscles:        []string{"lats"},
			SecondaryMuscles:      []string{"biceps"},
			EquipmentRequirements: []string{"barbell"},
		},
	}

	ranked := rankSubstitutes(
		prescribed,
		candidates,
		SubstituteFilter{
			Equipment: []string{"dumbbell", "bench", "machine"},
			Limit:     5,
		},
	)

	require.Len(t, ranked, 2)
	require.Equal(t, "Dumbbell Bench Press", ranked[0].Exercise.Name)
	require.Equal(t, []string{"push", "horizontal_press"}, ranked[0].Why.MatchedPattern)
	require.Equal(t, []string{"chest", "triceps", "front_delts"}, ranked[0].Why.MatchedMuscles)
	require.Equal(t, EquipmentFitExact, ranked[0].Why.EquipmentFit)

	require.Equal(t, "Machine Chest Press", ranked[1].Exercise.Name)
	require.Equal(t, []string{"push"}, ranked[1].Why.MatchedPattern)
	require.Equal(t, EquipmentFitExact, ranked[1].Why.EquipmentFit)
}

func TestRankSubstitutesAppliesConstraintFilters(t *testing.T) {
	prescribed := CatalogExercise{
		ID:               mustUUID(t, "00000000-0000-0000-0000-000000000020"),
		Name:             "Back Squat",
		MovementPattern:  "squat",
		MovementTaxonomy: []string{"squat", "bilateral"},
		PrimaryMuscles:   []string{"quads"},
		SecondaryMuscles: []string{"glutes", "core"},
	}

	candidates := []CatalogExercise{
		{
			ID:                    mustUUID(t, "00000000-0000-0000-0000-000000000021"),
			Name:                  "Front Squat",
			MovementPattern:       "squat",
			MovementTaxonomy:      []string{"squat", "bilateral"},
			PrimaryMuscles:        []string{"quads"},
			SecondaryMuscles:      []string{"core"},
			ContraindicationTags:  []string{"acute_knee_injury"},
			EquipmentRequirements: []string{"barbell", "rack"},
		},
		{
			ID:                    mustUUID(t, "00000000-0000-0000-0000-000000000022"),
			Name:                  "Goblet Squat",
			MovementPattern:       "squat",
			MovementTaxonomy:      []string{"squat", "bilateral"},
			PrimaryMuscles:        []string{"quads"},
			SecondaryMuscles:      []string{"glutes"},
			EquipmentRequirements: []string{"dumbbell", "kettlebell"},
		},
		{
			ID:                    mustUUID(t, "00000000-0000-0000-0000-000000000023"),
			Name:                  "Bodyweight Split Squat",
			MovementPattern:       "squat",
			MovementTaxonomy:      []string{"squat", "unilateral"},
			PrimaryMuscles:        []string{"quads"},
			SecondaryMuscles:      []string{"glutes"},
			EquipmentRequirements: []string{"dumbbell", "bodyweight"},
		},
	}

	ranked := rankSubstitutes(
		prescribed,
		candidates,
		SubstituteFilter{
			Equipment:   []string{"dumbbell"},
			InjuryFlags: []string{"acute_knee_injury"},
			Limit:       5,
		},
	)

	require.Len(t, ranked, 2)
	require.Equal(t, "Goblet Squat", ranked[0].Exercise.Name)
	require.Equal(t, EquipmentFitPartial, ranked[0].Why.EquipmentFit)
	require.Equal(t, "Bodyweight Split Squat", ranked[1].Exercise.Name)
	require.Equal(t, EquipmentFitPartial, ranked[1].Why.EquipmentFit)
}

func TestRankSubstitutesIsDeterministicOnScoreTies(t *testing.T) {
	prescribed := CatalogExercise{
		ID:               mustUUID(t, "00000000-0000-0000-0000-000000000030"),
		MovementPattern:  "pull",
		MovementTaxonomy: []string{"pull", "horizontal_pull"},
		PrimaryMuscles:   []string{"lats"},
		SecondaryMuscles: []string{"biceps"},
	}

	candidates := []CatalogExercise{
		{
			ID:                    mustUUID(t, "00000000-0000-0000-0000-000000000032"),
			Name:                  "Z Row",
			MovementPattern:       "pull",
			MovementTaxonomy:      []string{"pull", "horizontal_pull"},
			PrimaryMuscles:        []string{"lats"},
			SecondaryMuscles:      []string{"biceps"},
			EquipmentRequirements: []string{"dumbbell"},
		},
		{
			ID:                    mustUUID(t, "00000000-0000-0000-0000-000000000031"),
			Name:                  "A Row",
			MovementPattern:       "pull",
			MovementTaxonomy:      []string{"pull", "horizontal_pull"},
			PrimaryMuscles:        []string{"lats"},
			SecondaryMuscles:      []string{"biceps"},
			EquipmentRequirements: []string{"dumbbell"},
		},
	}

	ranked := rankSubstitutes(
		prescribed,
		candidates,
		SubstituteFilter{
			Equipment: []string{"dumbbell"},
			Limit:     5,
		},
	)

	require.Len(t, ranked, 2)
	require.Equal(t, "A Row", ranked[0].Exercise.Name)
	require.Equal(t, "Z Row", ranked[1].Exercise.Name)
}

func mustUUID(t *testing.T, raw string) uuid.UUID {
	t.Helper()

	parsed, err := uuid.Parse(raw)
	require.NoError(t, err)
	return parsed
}
