package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type biomechanicsResponse struct {
	Biomechanics interface{} `json:"biomechanics"`
}

type biomechanicsErrorResponse struct {
	Message string `json:"message"`
}

func (s *Server) handleGetExerciseBiomechanics(w http.ResponseWriter, r *http.Request) {
	exerciseIDText := chi.URLParam(r, "id")
	exerciseID, err := uuid.Parse(exerciseIDText)
	if err != nil {
		writeBiomechanicsJSON(w, http.StatusBadRequest, biomechanicsErrorResponse{Message: "invalid exercise id"})
		return
	}

	biomechanicsPayload, err := s.biomechSvc.GetByExerciseID(r.Context(), exerciseID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeBiomechanicsJSON(w, http.StatusNotFound, biomechanicsErrorResponse{Message: "exercise biomechanics not found"})
			return
		}

		s.logger.Error(
			"failed fetching exercise biomechanics",
			zap.Error(err),
			zap.String("exercise_id", exerciseID.String()),
		)
		writeBiomechanicsJSON(w, http.StatusInternalServerError, biomechanicsErrorResponse{Message: "could not fetch exercise biomechanics"})
		return
	}

	writeBiomechanicsJSON(w, http.StatusOK, biomechanicsResponse{
		Biomechanics: biomechanicsPayload,
	})
}

func writeBiomechanicsJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
