package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/atlas/atlas-api/internal/consent"
	db "github.com/atlas/atlas-api/internal/db/sqlc"
	"github.com/atlas/atlas-api/internal/httpapi/generated"
	"github.com/atlas/atlas-api/internal/httpapi/middleware"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"go.uber.org/zap"
)

func (s *Server) PostFormCheckUploads(
	ctx context.Context,
	request generated.PostFormCheckUploadsRequestObject,
) (generated.PostFormCheckUploadsResponseObject, error) {
	if request.Body == nil {
		return generated.PostFormCheckUploads400JSONResponse{Message: "request body is required"}, nil
	}

	userID, ok := middleware.AuthenticatedUserID(ctx)
	if !ok {
		return generated.PostFormCheckUploads401JSONResponse{Message: "unauthorized"}, nil
	}

	hasConsent, err := s.queries.HasActiveConsent(ctx, db.HasActiveConsentParams{
		UserID:      userID,
		ConsentType: consent.TypeFormCheckUpload,
	})
	if err != nil {
		s.logger.Error(
			"failed checking form-check upload consent",
			zap.Error(err),
			zap.String("user_id", userID.String()),
		)
		return generated.PostFormCheckUploads500JSONResponse{Message: "could not verify form-check upload consent"}, nil
	}
	if !hasConsent {
		return generated.PostFormCheckUploads403JSONResponse{Message: "form-check upload consent is not granted"}, nil
	}

	movementType := strings.TrimSpace(string(request.Body.MovementType))
	if movementType == "" {
		return generated.PostFormCheckUploads400JSONResponse{Message: "movementType is required"}, nil
	}

	if request.Body.RecordingEndedAt.Before(request.Body.RecordingStartedAt) {
		return generated.PostFormCheckUploads400JSONResponse{Message: "recordingEndedAt must be after recordingStartedAt"}, nil
	}

	if !isValidFormCheckScore(request.Body.Summary.OverallScore) ||
		!isValidFormCheckScore(request.Body.Summary.RangeOfMotionScore) ||
		!isValidFormCheckScore(request.Body.Summary.KneeTrackingScore) ||
		!isValidFormCheckScore(request.Body.Summary.SymmetryScore) {
		return generated.PostFormCheckUploads400JSONResponse{Message: "summary scores must be between 0 and 100"}, nil
	}

	if request.Body.Summary.RangeOfMotionDegrees < 0 {
		return generated.PostFormCheckUploads400JSONResponse{Message: "rangeOfMotionDegrees must be >= 0"}, nil
	}
	if request.Body.Summary.RepetitionCount < 0 {
		return generated.PostFormCheckUploads400JSONResponse{Message: "repetitionCount must be >= 0"}, nil
	}

	summaryJSON, err := json.Marshal(request.Body.Summary)
	if err != nil {
		return generated.PostFormCheckUploads400JSONResponse{Message: "invalid summary payload"}, nil
	}

	metadata := map[string]interface{}{}
	if request.Body.MetadataJson != nil {
		metadata = *request.Body.MetadataJson
	}
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return generated.PostFormCheckUploads400JSONResponse{Message: "invalid metadataJson"}, nil
	}

	storageKey := strings.TrimSpace(valueOrEmpty(request.Body.StorageKey))
	if storageKey == "" {
		storageKey = fmt.Sprintf("form-check-uploads/%s/%s.json", userID.String(), uuid.NewString())
	}

	if s.assetStorage != nil {
		normalizedStorageKey, normalizeErr := s.assetStorage.NormalizeURI(storageKey)
		if normalizeErr != nil {
			return generated.PostFormCheckUploads400JSONResponse{Message: "invalid storageKey"}, nil
		}
		storageKey = normalizedStorageKey
	}

	row, err := s.queries.CreateFormCheckUpload(ctx, db.CreateFormCheckUploadParams{
		UserID:             userID,
		MovementType:       movementType,
		RecordingStartedAt: request.Body.RecordingStartedAt,
		RecordingEndedAt:   request.Body.RecordingEndedAt,
		SummaryJson:        summaryJSON,
		MetadataJson:       metadataJSON,
		StorageKey:         storageKey,
	})
	if err != nil {
		s.logger.Error(
			"failed creating form-check upload",
			zap.Error(err),
			zap.String("user_id", userID.String()),
		)
		return generated.PostFormCheckUploads500JSONResponse{Message: "could not save form-check upload"}, nil
	}

	upload, err := toAPIFormCheckUpload(row)
	if err != nil {
		s.logger.Error(
			"failed mapping form-check upload response",
			zap.Error(err),
			zap.String("upload_id", row.ID.String()),
		)
		return generated.PostFormCheckUploads500JSONResponse{Message: "could not save form-check upload"}, nil
	}

	return generated.PostFormCheckUploads201JSONResponse{
		Upload: upload,
	}, nil
}

func isValidFormCheckScore(score int32) bool {
	return score >= 0 && score <= 100
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func toAPIFormCheckUpload(row db.FormCheckUpload) (generated.FormCheckUpload, error) {
	summary := generated.FormCheckResultSummary{}
	if len(row.SummaryJson) > 0 {
		if err := json.Unmarshal(row.SummaryJson, &summary); err != nil {
			return generated.FormCheckUpload{}, err
		}
	}

	metadata := map[string]interface{}{}
	if len(row.MetadataJson) > 0 {
		if err := json.Unmarshal(row.MetadataJson, &metadata); err != nil {
			return generated.FormCheckUpload{}, err
		}
	}

	return generated.FormCheckUpload{
		Id:                 openapi_types.UUID(row.ID),
		UserId:             openapi_types.UUID(row.UserID),
		MovementType:       generated.FormCheckMovementType(row.MovementType),
		RecordingStartedAt: row.RecordingStartedAt,
		RecordingEndedAt:   row.RecordingEndedAt,
		Summary:            summary,
		StorageKey:         row.StorageKey,
		MetadataJson:       metadata,
		CreatedAt:          row.CreatedAt,
	}, nil
}
