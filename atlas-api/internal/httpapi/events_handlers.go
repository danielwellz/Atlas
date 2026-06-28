package httpapi

import (
	"context"
	"encoding/json"

	"github.com/atlas/atlas-api/internal/consent"
	db "github.com/atlas/atlas-api/internal/db/sqlc"
	"github.com/atlas/atlas-api/internal/httpapi/generated"
	"github.com/atlas/atlas-api/internal/httpapi/middleware"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

func (s *Server) PostEvents(ctx context.Context, request generated.PostEventsRequestObject) (generated.PostEventsResponseObject, error) {
	if request.Body == nil {
		return generated.PostEvents400JSONResponse{Message: "request body is required"}, nil
	}
	if !request.Body.ConsentGranted {
		return generated.PostEvents403JSONResponse{Message: "analytics consent is required"}, nil
	}

	eventName := string(request.Body.EventName)
	rawProperties := map[string]interface{}{}
	if request.Body.Properties != nil {
		rawProperties = *request.Body.Properties
	}

	sanitizedProperties, err := validateAndSanitizeEventProperties(eventName, rawProperties)
	if err != nil {
		return generated.PostEvents400JSONResponse{Message: err.Error()}, nil
	}
	if request.Body.EventTime.IsZero() {
		return generated.PostEvents400JSONResponse{Message: "eventTime is required"}, nil
	}

	authenticatedUserID, authenticated := middleware.AuthenticatedUserID(ctx)
	if authenticated {
		hasConsent, err := s.queries.HasActiveConsent(ctx, db.HasActiveConsentParams{
			UserID:      authenticatedUserID,
			ConsentType: consent.TypeProductAnalytics,
		})
		if err != nil {
			s.logger.Error(
				"failed checking analytics consent",
				zap.Error(err),
				zap.String("user_id", authenticatedUserID.String()),
			)
			return generated.PostEvents400JSONResponse{Message: "could not verify analytics consent"}, nil
		}
		if !hasConsent {
			return generated.PostEvents403JSONResponse{Message: "analytics consent is not granted"}, nil
		}
	}

	propertiesJSON, err := json.Marshal(sanitizedProperties)
	if err != nil {
		s.logger.Error("failed marshaling event properties", zap.Error(err), zap.String("event_name", eventName))
		return generated.PostEvents400JSONResponse{Message: "invalid event properties"}, nil
	}

	dbUserID := uuid.NullUUID{}
	if authenticated {
		dbUserID = uuid.NullUUID{UUID: authenticatedUserID, Valid: true}
	}

	createdEvent, err := s.queries.CreateAppEvent(ctx, db.CreateAppEventParams{
		UserID:         dbUserID,
		EventName:      eventName,
		EventTime:      request.Body.EventTime,
		PropertiesJson: propertiesJSON,
	})
	if err != nil {
		s.logger.Error("failed ingesting app event", zap.Error(err), zap.String("event_name", eventName))
		return generated.PostEvents400JSONResponse{Message: "could not ingest event"}, nil
	}

	response := generated.PostEvents202JSONResponse{Accepted: true}
	if createdEvent.ID != uuid.Nil {
		eventID := createdEvent.ID
		response.EventId = &eventID
	}

	return response, nil
}
