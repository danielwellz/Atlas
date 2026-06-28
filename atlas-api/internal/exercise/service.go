package exercise

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	db "github.com/atlas/atlas-api/internal/db/sqlc"
	"github.com/atlas/atlas-api/internal/storage"
	"github.com/google/uuid"
)

type Service struct {
	queries db.Querier
	storage storage.Storage
}

func NewService(queries db.Querier, mediaStorage storage.Storage) *Service {
	return &Service{
		queries: queries,
		storage: mediaStorage,
	}
}

func (s *Service) List(ctx context.Context, filter ListFilter) ([]CatalogExercise, error) {
	rows, err := s.queries.ListExercises(ctx, db.ListExercisesParams{
		Query:     strings.TrimSpace(filter.Query),
		Equipment: normalizeFilterToken(filter.Equipment),
		Pattern:   normalizeFilterToken(filter.Pattern),
	})
	if err != nil {
		return nil, err
	}

	result := make([]CatalogExercise, 0, len(rows))
	for _, row := range rows {
		mediaRows, err := s.queries.ListExerciseMediaByExerciseID(ctx, row.ID)
		if err != nil {
			return nil, err
		}

		exercise, err := s.mapCatalogExercise(row, mediaRows)
		if err != nil {
			return nil, err
		}

		result = append(result, exercise)
	}

	return result, nil
}

func (s *Service) GetByID(ctx context.Context, exerciseID uuid.UUID) (CatalogExercise, error) {
	row, err := s.queries.GetExerciseByID(ctx, exerciseID)
	if err != nil {
		return CatalogExercise{}, err
	}

	mediaRows, err := s.queries.ListExerciseMediaByExerciseID(ctx, row.ID)
	if err != nil {
		return CatalogExercise{}, err
	}

	return s.mapCatalogExercise(row, mediaRows)
}

func (s *Service) ListSubstitutes(
	ctx context.Context,
	exerciseID uuid.UUID,
	filter SubstituteFilter,
) ([]RankedSubstitute, error) {
	prescribedRow, err := s.queries.GetExerciseByID(ctx, exerciseID)
	if err != nil {
		return nil, err
	}

	prescribedExercise, err := s.mapCatalogExercise(prescribedRow, nil)
	if err != nil {
		return nil, err
	}

	candidateRows, err := s.queries.ListExerciseCandidatesForSubstitution(ctx, exerciseID)
	if err != nil {
		return nil, err
	}

	candidates := make([]CatalogExercise, 0, len(candidateRows))
	for _, row := range candidateRows {
		candidate, mapErr := s.mapCatalogExercise(row, nil)
		if mapErr != nil {
			return nil, mapErr
		}
		candidates = append(candidates, candidate)
	}

	ranked := rankSubstitutes(prescribedExercise, candidates, filter)
	for index := range ranked {
		mediaRows, mediaErr := s.queries.ListExerciseMediaByExerciseID(ctx, ranked[index].Exercise.ID)
		if mediaErr != nil {
			return nil, mediaErr
		}

		media, mediaMapErr := s.mapMediaRows(mediaRows)
		if mediaMapErr != nil {
			return nil, mediaMapErr
		}
		ranked[index].Exercise.Media = media
	}

	return ranked, nil
}

func (s *Service) mapCatalogExercise(row db.Exercise, mediaRows []db.ExerciseMedium) (CatalogExercise, error) {
	primaryMuscles, err := parseStringArrayJSON(row.PrimaryMuscles)
	if err != nil {
		return CatalogExercise{}, fmt.Errorf("invalid primary_muscles for exercise %s: %w", row.ID, err)
	}
	if len(primaryMuscles) == 0 && strings.TrimSpace(row.PrimaryMuscleGroup) != "" {
		primaryMuscles = []string{row.PrimaryMuscleGroup}
	}

	secondaryMuscles, err := parseStringArrayJSON(row.SecondaryMuscles)
	if err != nil {
		return CatalogExercise{}, fmt.Errorf("invalid secondary_muscles for exercise %s: %w", row.ID, err)
	}
	if len(secondaryMuscles) == 0 && len(row.SecondaryMusclesJson) > 0 {
		secondaryMuscles, err = parseStringArrayJSON(row.SecondaryMusclesJson)
		if err != nil {
			return CatalogExercise{}, fmt.Errorf("invalid secondary_muscles_json for exercise %s: %w", row.ID, err)
		}
	}

	contraindications, err := parseStringArrayJSON(row.Contraindications)
	if err != nil {
		return CatalogExercise{}, fmt.Errorf("invalid contraindications for exercise %s: %w", row.ID, err)
	}

	contraindicationTags, err := parseStringArrayJSON(row.ContraindicationTags)
	if err != nil {
		return CatalogExercise{}, fmt.Errorf("invalid contraindication_tags for exercise %s: %w", row.ID, err)
	}
	if len(contraindicationTags) == 0 {
		contraindicationTags = contraindications
	}

	equipment, err := parseStringArrayJSON(row.EquipmentJson)
	if err != nil {
		return CatalogExercise{}, fmt.Errorf("invalid equipment_json for exercise %s: %w", row.ID, err)
	}

	equipmentRequirements, err := parseStringArrayJSON(row.EquipmentRequirements)
	if err != nil {
		return CatalogExercise{}, fmt.Errorf("invalid equipment_requirements for exercise %s: %w", row.ID, err)
	}
	if len(equipmentRequirements) == 0 {
		equipmentRequirements = equipment
	}

	movementTaxonomy, err := parseStringArrayJSON(row.MovementPatternTaxonomy)
	if err != nil {
		return CatalogExercise{}, fmt.Errorf("invalid movement_pattern_taxonomy for exercise %s: %w", row.ID, err)
	}
	if len(movementTaxonomy) == 0 && strings.TrimSpace(row.MovementPattern) != "" {
		movementTaxonomy = []string{row.MovementPattern}
	}

	media, err := s.mapMediaRows(mediaRows)
	if err != nil {
		return CatalogExercise{}, err
	}

	return CatalogExercise{
		ID:                    row.ID,
		Slug:                  row.Slug,
		Name:                  row.Name,
		PrimaryMuscleGroup:    row.PrimaryMuscleGroup,
		PrimaryMuscles:        primaryMuscles,
		SecondaryMuscles:      secondaryMuscles,
		MovementPattern:       row.MovementPattern,
		MovementTaxonomy:      movementTaxonomy,
		Contraindications:     contraindications,
		ContraindicationTags:  contraindicationTags,
		Equipment:             equipment,
		EquipmentRequirements: equipmentRequirements,
		Difficulty:            row.Difficulty,
		Description:           row.Description,
		CreatedAt:             row.CreatedAt,
		Media:                 media,
	}, nil
}

func (s *Service) mapMediaRows(mediaRows []db.ExerciseMedium) ([]Media, error) {
	media := make([]Media, 0, len(mediaRows))
	for _, mediaRow := range mediaRows {
		normalizedURI, err := s.normalizeURI(mediaRow.Uri)
		if err != nil {
			return nil, fmt.Errorf("invalid uri for exercise media %s: %w", mediaRow.ID, err)
		}

		thumbnailURI, err := s.normalizeNullableURI(mediaRow.ThumbnailUri)
		if err != nil {
			return nil, fmt.Errorf("invalid thumbnail_uri for exercise media %s: %w", mediaRow.ID, err)
		}

		var durationSeconds *int32
		if mediaRow.DurationSeconds.Valid {
			duration := mediaRow.DurationSeconds.Int32
			durationSeconds = &duration
		}

		media = append(media, Media{
			ID:              mediaRow.ID,
			ExerciseID:      mediaRow.ExerciseID,
			MediaType:       mediaRow.MediaType,
			URI:             normalizedURI,
			ThumbnailURI:    thumbnailURI,
			DurationSeconds: durationSeconds,
			CreatedAt:       mediaRow.CreatedAt,
		})
	}

	return media, nil
}

func parseStringArrayJSON(raw json.RawMessage) ([]string, error) {
	if len(raw) == 0 {
		return []string{}, nil
	}

	values := make([]string, 0)
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, err
	}
	return values, nil
}

func normalizeFilterToken(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func (s *Service) normalizeURI(uri string) (string, error) {
	trimmed := strings.TrimSpace(uri)
	if trimmed == "" {
		return "", storage.ErrEmptyURI
	}

	if s.storage == nil {
		return trimmed, nil
	}

	return s.storage.NormalizeURI(trimmed)
}

func (s *Service) normalizeNullableURI(uriValue sql.NullString) (*string, error) {
	if !uriValue.Valid || strings.TrimSpace(uriValue.String) == "" {
		return nil, nil
	}

	normalized, err := s.normalizeURI(uriValue.String)
	if err != nil {
		return nil, err
	}

	return &normalized, nil
}
