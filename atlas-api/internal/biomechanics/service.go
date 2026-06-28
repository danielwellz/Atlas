package biomechanics

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	db "github.com/atlas/atlas-api/internal/db/sqlc"
	"github.com/atlas/atlas-api/internal/storage"
	"github.com/google/uuid"
)

const getExerciseBiomechAssetSQL = `
SELECT id, animation_asset_key, rig_version, metadata_json
FROM exercise_biomech_assets
WHERE exercise_id = $1
`

const listBiomechMuscleGroupsSQL = `
SELECT muscle_group_slug, activation_level, role
FROM exercise_biomech_asset_muscle_groups
WHERE biomech_asset_id = $1
ORDER BY activation_level DESC, muscle_group_slug ASC
`

type Service struct {
	queries      db.Querier
	db           db.DBTX
	assetStorage storage.Storage
}

func NewService(queries db.Querier, dbtx db.DBTX, assetStorage storage.Storage) *Service {
	return &Service{
		queries:      queries,
		db:           dbtx,
		assetStorage: assetStorage,
	}
}

func (s *Service) GetByExerciseID(ctx context.Context, exerciseID uuid.UUID) (ExerciseBiomechanics, error) {
	exerciseRow, err := s.queries.GetExerciseByID(ctx, exerciseID)
	if err != nil {
		return ExerciseBiomechanics{}, err
	}

	if s.db == nil {
		return ExerciseBiomechanics{}, errors.New("biomechanics database handle is not configured")
	}

	var (
		biomechAssetID    uuid.UUID
		animationAssetKey string
		rigVersion        string
		metadataJSON      json.RawMessage
	)

	err = s.db.QueryRowContext(ctx, getExerciseBiomechAssetSQL, exerciseID).Scan(
		&biomechAssetID,
		&animationAssetKey,
		&rigVersion,
		&metadataJSON,
	)
	if err != nil {
		return ExerciseBiomechanics{}, err
	}

	metadata := map[string]interface{}{}
	if len(metadataJSON) > 0 {
		if unmarshalErr := json.Unmarshal(metadataJSON, &metadata); unmarshalErr != nil {
			return ExerciseBiomechanics{}, fmt.Errorf("invalid biomechanics metadata_json for exercise %s: %w", exerciseID, unmarshalErr)
		}
	}

	muscleHighlights, err := s.listMuscleHighlights(ctx, biomechAssetID, metadata)
	if err != nil {
		return ExerciseBiomechanics{}, err
	}

	jointAngles := parseJointAngles(metadata)

	animationAssetURI := animationAssetKey
	if s.assetStorage != nil {
		if normalized, normalizeErr := s.assetStorage.NormalizeURI(animationAssetKey); normalizeErr == nil {
			if strings.HasPrefix(normalized, "s3://") ||
				strings.HasPrefix(normalized, "http://") ||
				strings.HasPrefix(normalized, "https://") {
				animationAssetURI = normalized
			}
		}
	}

	return ExerciseBiomechanics{
		ExerciseID:        exerciseRow.ID,
		ExerciseSlug:      exerciseRow.Slug,
		ExerciseName:      exerciseRow.Name,
		AnimationAssetKey: animationAssetKey,
		AnimationAssetURI: animationAssetURI,
		RigVersion:        rigVersion,
		MuscleHighlights:  muscleHighlights,
		JointAngles:       jointAngles,
		Metadata:          metadata,
		MetadataJSON:      metadataJSON,
	}, nil
}

func (s *Service) listMuscleHighlights(
	ctx context.Context,
	biomechAssetID uuid.UUID,
	metadata map[string]interface{},
) ([]MuscleHighlight, error) {
	metadataHighlights := parseMuscleHighlights(metadata)
	metadataHighlightByGroup := make(map[string]MuscleHighlight, len(metadataHighlights))
	for _, highlight := range metadataHighlights {
		group := strings.ToLower(strings.TrimSpace(highlight.MuscleGroup))
		if group == "" {
			continue
		}
		metadataHighlightByGroup[group] = highlight
	}

	rows, err := s.db.QueryContext(ctx, listBiomechMuscleGroupsSQL, biomechAssetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]MuscleHighlight, 0)
	for rows.Next() {
		var highlight MuscleHighlight
		if scanErr := rows.Scan(&highlight.MuscleGroup, &highlight.ActivationLevel, &highlight.Role); scanErr != nil {
			return nil, scanErr
		}

		if metadataHighlight, ok := metadataHighlightByGroup[strings.ToLower(strings.TrimSpace(highlight.MuscleGroup))]; ok {
			if strings.TrimSpace(highlight.ColorHex) == "" {
				highlight.ColorHex = metadataHighlight.ColorHex
			}
			if strings.TrimSpace(highlight.Role) == "" {
				highlight.Role = metadataHighlight.Role
			}
		}

		result = append(result, highlight)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(result) > 0 {
		return result, nil
	}

	return metadataHighlights, nil
}

func parseMuscleHighlights(metadata map[string]interface{}) []MuscleHighlight {
	items, ok := metadata["muscleHighlights"].([]interface{})
	if !ok || len(items) == 0 {
		return []MuscleHighlight{}
	}

	result := make([]MuscleHighlight, 0, len(items))
	for _, item := range items {
		record, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		muscleGroup := strings.TrimSpace(asString(record["muscleGroup"]))
		if muscleGroup == "" {
			continue
		}

		result = append(result, MuscleHighlight{
			MuscleGroup:     muscleGroup,
			ActivationLevel: asFloat(record["activationLevel"]),
			Role:            strings.TrimSpace(asString(record["role"])),
			ColorHex:        strings.TrimSpace(asString(record["colorHex"])),
		})
	}

	return result
}

func parseJointAngles(metadata map[string]interface{}) []JointAngle {
	items, ok := metadata["jointAngles"].([]interface{})
	if !ok || len(items) == 0 {
		return []JointAngle{}
	}

	result := make([]JointAngle, 0, len(items))
	for _, item := range items {
		record, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		joint := strings.TrimSpace(asString(record["joint"]))
		if joint == "" {
			continue
		}

		result = append(result, JointAngle{
			Joint:         joint,
			MinDegrees:    asFloat(record["minDegrees"]),
			MaxDegrees:    asFloat(record["maxDegrees"]),
			TargetDegrees: asFloat(record["targetDegrees"]),
			Unit:          strings.TrimSpace(asString(record["unit"])),
		})
	}

	return result
}

func asString(value interface{}) string {
	if value == nil {
		return ""
	}

	switch typed := value.(type) {
	case string:
		return typed
	default:
		return fmt.Sprintf("%v", typed)
	}
}

func asFloat(value interface{}) float64 {
	if value == nil {
		return 0
	}

	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int32:
		return float64(typed)
	case int64:
		return float64(typed)
	case json.Number:
		parsed, err := typed.Float64()
		if err == nil {
			return parsed
		}
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		if err == nil {
			return parsed
		}
	}

	return 0
}
