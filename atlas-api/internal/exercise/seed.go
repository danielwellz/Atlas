package exercise

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	db "github.com/atlas/atlas-api/internal/db/sqlc"
	"github.com/google/uuid"
)

const (
	csvColumnID                = "id"
	csvColumnSlug              = "slug"
	csvColumnName              = "name"
	csvColumnMovementPattern   = "movement_pattern"
	csvColumnPrimaryMuscles    = "primary_muscles"
	csvColumnSecondaryMuscles  = "secondary_muscles"
	csvColumnContraindications = "contraindications"
	csvColumnEquipment         = "equipment_json"
	csvColumnDifficulty        = "difficulty"
	csvColumnDescription       = "description"
)

var requiredExerciseCSVColumns = []string{
	csvColumnID,
	csvColumnSlug,
	csvColumnName,
	csvColumnMovementPattern,
	csvColumnPrimaryMuscles,
	csvColumnSecondaryMuscles,
	csvColumnContraindications,
	csvColumnEquipment,
	csvColumnDifficulty,
	csvColumnDescription,
}

var allowedDifficultyValues = map[string]struct{}{
	"beginner":     {},
	"intermediate": {},
	"advanced":     {},
}

var allowedMovementPatternValues = map[string]struct{}{
	"squat": {},
	"hinge": {},
	"push":  {},
	"pull":  {},
	"carry": {},
	"core":  {},
}

type SeedExerciseRow struct {
	ID                *uuid.UUID
	Slug              string
	Name              string
	MovementPattern   string
	PrimaryMuscles    []string
	SecondaryMuscles  []string
	Contraindications []string
	Equipment         []string
	Difficulty        string
	Description       string
}

type ImportResult struct {
	RowsProcessed int
}

func ParseExercisesCSV(reader io.Reader) ([]SeedExerciseRow, error) {
	csvReader := csv.NewReader(reader)
	csvReader.TrimLeadingSpace = true
	csvReader.FieldsPerRecord = -1

	header, err := csvReader.Read()
	if err != nil {
		if err == io.EOF {
			return nil, fmt.Errorf("csv file is empty")
		}
		return nil, fmt.Errorf("failed reading csv header: %w", err)
	}

	columnIndexes := mapCSVHeaderIndexes(header)
	for _, requiredColumn := range requiredExerciseCSVColumns {
		if _, ok := columnIndexes[requiredColumn]; !ok {
			return nil, fmt.Errorf("missing required csv column %q", requiredColumn)
		}
	}

	rows := make([]SeedExerciseRow, 0)
	lineNumber := 1
	for {
		record, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		lineNumber++
		if err != nil {
			return nil, fmt.Errorf("failed reading csv line %d: %w", lineNumber, err)
		}
		if isBlankRecord(record) {
			continue
		}

		row, err := parseSeedExerciseRecord(columnIndexes, record)
		if err != nil {
			return nil, fmt.Errorf("invalid exercises csv at line %d: %w", lineNumber, err)
		}

		rows = append(rows, row)
	}

	return rows, nil
}

func ParseExercisesCSVFile(filePath string) ([]SeedExerciseRow, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return ParseExercisesCSV(file)
}

func ImportExercisesCSV(ctx context.Context, queries db.Querier, filePath string) (ImportResult, error) {
	rows, err := ParseExercisesCSVFile(filePath)
	if err != nil {
		return ImportResult{}, err
	}

	for _, row := range rows {
		exerciseID := uuid.New()
		if row.ID != nil {
			exerciseID = *row.ID
		}

		primaryMusclesJSON, err := json.Marshal(row.PrimaryMuscles)
		if err != nil {
			return ImportResult{}, fmt.Errorf("failed marshaling primary muscles for %s: %w", row.Slug, err)
		}

		secondaryMusclesJSON, err := json.Marshal(row.SecondaryMuscles)
		if err != nil {
			return ImportResult{}, fmt.Errorf("failed marshaling secondary muscles for %s: %w", row.Slug, err)
		}

		contraindicationsJSON, err := json.Marshal(row.Contraindications)
		if err != nil {
			return ImportResult{}, fmt.Errorf("failed marshaling contraindications for %s: %w", row.Slug, err)
		}

		equipmentJSON, err := json.Marshal(row.Equipment)
		if err != nil {
			return ImportResult{}, fmt.Errorf("failed marshaling equipment for %s: %w", row.Slug, err)
		}

		_, err = queries.UpsertExercise(ctx, db.UpsertExerciseParams{
			ID:                   exerciseID,
			Slug:                 row.Slug,
			Name:                 row.Name,
			PrimaryMuscleGroup:   row.PrimaryMuscles[0],
			SecondaryMusclesJson: secondaryMusclesJSON,
			MovementPattern:      row.MovementPattern,
			PrimaryMuscles:       primaryMusclesJSON,
			SecondaryMuscles:     secondaryMusclesJSON,
			Contraindications:    contraindicationsJSON,
			EquipmentJson:        equipmentJSON,
			Difficulty:           row.Difficulty,
			Description:          row.Description,
		})
		if err != nil {
			return ImportResult{}, fmt.Errorf("failed upserting exercise %q: %w", row.Slug, err)
		}
	}

	return ImportResult{RowsProcessed: len(rows)}, nil
}

func parseSeedExerciseRecord(columnIndexes map[string]int, record []string) (SeedExerciseRow, error) {
	var id *uuid.UUID
	rawID := getCSVValue(record, columnIndexes[csvColumnID])
	if rawID != "" {
		parsedID, err := uuid.Parse(rawID)
		if err != nil {
			return SeedExerciseRow{}, fmt.Errorf("invalid id: %w", err)
		}
		id = &parsedID
	}

	slug := strings.ToLower(strings.TrimSpace(getCSVValue(record, columnIndexes[csvColumnSlug])))
	name := strings.TrimSpace(getCSVValue(record, columnIndexes[csvColumnName]))
	movementPattern := strings.ToLower(strings.TrimSpace(getCSVValue(record, columnIndexes[csvColumnMovementPattern])))
	difficulty := strings.ToLower(strings.TrimSpace(getCSVValue(record, columnIndexes[csvColumnDifficulty])))
	description := strings.TrimSpace(getCSVValue(record, columnIndexes[csvColumnDescription]))

	if slug == "" || name == "" || movementPattern == "" || difficulty == "" || description == "" {
		return SeedExerciseRow{}, fmt.Errorf("slug, name, movement_pattern, difficulty, and description are required")
	}

	if _, ok := allowedMovementPatternValues[movementPattern]; !ok {
		return SeedExerciseRow{}, fmt.Errorf("invalid movement_pattern %q", movementPattern)
	}

	if _, ok := allowedDifficultyValues[difficulty]; !ok {
		return SeedExerciseRow{}, fmt.Errorf("invalid difficulty %q", difficulty)
	}

	primaryMuscles, err := parseStringArrayField(getCSVValue(record, columnIndexes[csvColumnPrimaryMuscles]))
	if err != nil {
		return SeedExerciseRow{}, fmt.Errorf("invalid primary_muscles: %w", err)
	}
	if len(primaryMuscles) == 0 {
		return SeedExerciseRow{}, fmt.Errorf("primary_muscles must contain at least one value")
	}

	secondaryMuscles, err := parseStringArrayField(getCSVValue(record, columnIndexes[csvColumnSecondaryMuscles]))
	if err != nil {
		return SeedExerciseRow{}, fmt.Errorf("invalid secondary_muscles: %w", err)
	}

	contraindications, err := parseStringArrayField(getCSVValue(record, columnIndexes[csvColumnContraindications]))
	if err != nil {
		return SeedExerciseRow{}, fmt.Errorf("invalid contraindications: %w", err)
	}

	equipment, err := parseStringArrayField(getCSVValue(record, columnIndexes[csvColumnEquipment]))
	if err != nil {
		return SeedExerciseRow{}, fmt.Errorf("invalid equipment_json: %w", err)
	}

	return SeedExerciseRow{
		ID:                id,
		Slug:              slug,
		Name:              name,
		MovementPattern:   movementPattern,
		PrimaryMuscles:    primaryMuscles,
		SecondaryMuscles:  secondaryMuscles,
		Contraindications: contraindications,
		Equipment:         equipment,
		Difficulty:        difficulty,
		Description:       description,
	}, nil
}

func parseStringArrayField(raw string) ([]string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return []string{}, nil
	}

	values := make([]string, 0)
	if err := json.Unmarshal([]byte(trimmed), &values); err != nil {
		return nil, err
	}

	normalized := make([]string, 0, len(values))
	for _, value := range values {
		cleaned := strings.ToLower(strings.TrimSpace(value))
		if cleaned == "" {
			continue
		}
		normalized = append(normalized, cleaned)
	}

	return normalized, nil
}

func mapCSVHeaderIndexes(header []string) map[string]int {
	indexes := make(map[string]int, len(header))
	for idx, value := range header {
		indexes[strings.TrimSpace(value)] = idx
	}
	return indexes
}

func getCSVValue(record []string, index int) string {
	if index < 0 || index >= len(record) {
		return ""
	}
	return record[index]
}

func isBlankRecord(record []string) bool {
	for _, value := range record {
		if strings.TrimSpace(value) != "" {
			return false
		}
	}
	return true
}
