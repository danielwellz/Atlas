package httpapi

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	db "github.com/atlas/atlas-api/internal/db/sqlc"
	"github.com/atlas/atlas-api/internal/httpapi/generated"
	"github.com/atlas/atlas-api/internal/httpapi/middleware"
	"github.com/atlas/atlas-api/internal/progression"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"go.uber.org/zap"
)

const (
	defaultWorkoutHistoryLimit int32 = 20
	maxWorkoutHistoryLimit     int32 = 100
)

type workoutHistoryCursor struct {
	StartedAt time.Time
	WorkoutID uuid.UUID
}

func (s *Server) PostWorkoutsStart(ctx context.Context, request generated.PostWorkoutsStartRequestObject) (generated.PostWorkoutsStartResponseObject, error) {
	userID, ok := middleware.AuthenticatedUserID(ctx)
	if !ok {
		return generated.PostWorkoutsStart401JSONResponse{Message: "unauthorized"}, nil
	}

	programSessionID := uuid.NullUUID{}
	if request.Body != nil && request.Body.ProgramSessionId != nil {
		sessionID := uuid.UUID(*request.Body.ProgramSessionId)
		if _, err := s.queries.GetProgramSessionByID(ctx, sessionID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return generated.PostWorkoutsStart404JSONResponse{Message: "program session not found"}, nil
			}
			s.logger.Error("failed fetching program session", zap.Error(err), zap.String("program_session_id", sessionID.String()))
			return generated.PostWorkoutsStart400JSONResponse{Message: "could not start workout"}, nil
		}

		programSessionID = uuid.NullUUID{UUID: sessionID, Valid: true}
	}

	workoutRow, err := s.queries.CreateWorkout(ctx, db.CreateWorkoutParams{
		UserID:           userID,
		ProgramSessionID: programSessionID,
		StartedAt:        s.currentTime().UTC(),
		Notes:            "",
	})
	if err != nil {
		s.logger.Error("failed creating workout", zap.Error(err), zap.String("user_id", userID.String()))
		return generated.PostWorkoutsStart400JSONResponse{Message: "could not start workout"}, nil
	}

	if programSessionID.Valid {
		if _, err := s.queries.CreateWorkoutExercisesFromProgramSession(ctx, db.CreateWorkoutExercisesFromProgramSessionParams{
			WorkoutID:        workoutRow.ID,
			ProgramSessionID: programSessionID.UUID,
		}); err != nil {
			s.logger.Error("failed prefilling workout exercises", zap.Error(err), zap.String("workout_id", workoutRow.ID.String()), zap.String("program_session_id", programSessionID.UUID.String()))
			return generated.PostWorkoutsStart400JSONResponse{Message: "could not start workout"}, nil
		}
	}

	workout, err := s.buildWorkoutResponse(ctx, workoutRow)
	if err != nil {
		s.logger.Error("failed building workout response", zap.Error(err), zap.String("workout_id", workoutRow.ID.String()))
		return generated.PostWorkoutsStart400JSONResponse{Message: "could not start workout"}, nil
	}

	return generated.PostWorkoutsStart201JSONResponse{Workout: workout}, nil
}

func (s *Server) PostWorkoutsWorkoutIdAddSet(ctx context.Context, request generated.PostWorkoutsWorkoutIdAddSetRequestObject) (generated.PostWorkoutsWorkoutIdAddSetResponseObject, error) {
	if request.Body == nil {
		return generated.PostWorkoutsWorkoutIdAddSet400JSONResponse{Message: "request body is required"}, nil
	}

	userID, ok := middleware.AuthenticatedUserID(ctx)
	if !ok {
		return generated.PostWorkoutsWorkoutIdAddSet401JSONResponse{Message: "unauthorized"}, nil
	}

	if request.Body.Reps <= 0 {
		return generated.PostWorkoutsWorkoutIdAddSet400JSONResponse{Message: "reps must be greater than 0"}, nil
	}
	if request.Body.WeightKg < 0 {
		return generated.PostWorkoutsWorkoutIdAddSet400JSONResponse{Message: "weight_kg must be greater than or equal to 0"}, nil
	}
	idempotencyKey := strings.TrimSpace(request.Body.IdempotencyKey)
	if idempotencyKey == "" {
		return generated.PostWorkoutsWorkoutIdAddSet400JSONResponse{Message: "idempotency_key is required"}, nil
	}
	if request.Body.Rpe != nil && (*request.Body.Rpe < 0 || *request.Body.Rpe > 10) {
		return generated.PostWorkoutsWorkoutIdAddSet400JSONResponse{Message: "rpe must be between 0 and 10"}, nil
	}

	workoutID := uuid.UUID(request.WorkoutId)
	workoutExerciseID := uuid.UUID(request.Body.WorkoutExerciseId)

	rpe := sql.NullFloat64{}
	if request.Body.Rpe != nil {
		rpe = sql.NullFloat64{Float64: float64(*request.Body.Rpe), Valid: true}
	}

	setRow, err := s.queries.CreateWorkoutSetAutoIndexed(ctx, db.CreateWorkoutSetAutoIndexedParams{
		Reps:              request.Body.Reps,
		WeightKg:          float64(request.Body.WeightKg),
		Rpe:               rpe,
		CompletedAt:       s.currentTime().UTC(),
		WorkoutID:         workoutID,
		UserID:            userID,
		WorkoutExerciseID: workoutExerciseID,
		IdempotencyKey:    sql.NullString{String: idempotencyKey, Valid: true},
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			workoutRow, workoutErr := s.queries.GetWorkoutByIDAndUserID(ctx, db.GetWorkoutByIDAndUserIDParams{ID: workoutID, UserID: userID})
			if workoutErr != nil {
				if errors.Is(workoutErr, sql.ErrNoRows) {
					return generated.PostWorkoutsWorkoutIdAddSet404JSONResponse{Message: "workout not found"}, nil
				}
				s.logger.Error("failed fetching workout for add_set error mapping", zap.Error(workoutErr), zap.String("workout_id", workoutID.String()), zap.String("user_id", userID.String()))
				return generated.PostWorkoutsWorkoutIdAddSet400JSONResponse{Message: "could not add workout set"}, nil
			}

			if workoutRow.CompletedAt.Valid {
				return generated.PostWorkoutsWorkoutIdAddSet409JSONResponse{Message: "workout already completed"}, nil
			}

			return generated.PostWorkoutsWorkoutIdAddSet404JSONResponse{Message: "workout exercise not found"}, nil
		}

		s.logger.Error("failed creating workout set", zap.Error(err), zap.String("workout_id", workoutID.String()), zap.String("workout_exercise_id", workoutExerciseID.String()), zap.String("user_id", userID.String()))
		return generated.PostWorkoutsWorkoutIdAddSet400JSONResponse{Message: "could not add workout set"}, nil
	}

	return generated.PostWorkoutsWorkoutIdAddSet201JSONResponse{
		Set: toAPIWorkoutSet(db.WorkoutSet{
			ID:                setRow.ID,
			WorkoutExerciseID: setRow.WorkoutExerciseID,
			SetIndex:          setRow.SetIndex,
			Reps:              setRow.Reps,
			WeightKg:          setRow.WeightKg,
			Rpe:               setRow.Rpe,
			CompletedAt:       setRow.CompletedAt,
			CreatedAt:         setRow.CreatedAt,
			IdempotencyKey:    setRow.IdempotencyKey,
		}),
	}, nil
}

func (s *Server) PostWorkoutsWorkoutIdComplete(ctx context.Context, request generated.PostWorkoutsWorkoutIdCompleteRequestObject) (generated.PostWorkoutsWorkoutIdCompleteResponseObject, error) {
	userID, ok := middleware.AuthenticatedUserID(ctx)
	if !ok {
		return generated.PostWorkoutsWorkoutIdComplete401JSONResponse{Message: "unauthorized"}, nil
	}

	workoutID := uuid.UUID(request.WorkoutId)
	currentWorkout, err := s.queries.GetWorkoutByIDAndUserID(ctx, db.GetWorkoutByIDAndUserIDParams{
		ID:     workoutID,
		UserID: userID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return generated.PostWorkoutsWorkoutIdComplete404JSONResponse{Message: "workout not found"}, nil
		}
		s.logger.Error("failed fetching workout before complete", zap.Error(err), zap.String("workout_id", workoutID.String()), zap.String("user_id", userID.String()))
		return generated.PostWorkoutsWorkoutIdComplete400JSONResponse{Message: "could not complete workout"}, nil
	}

	if currentWorkout.CompletedAt.Valid {
		return generated.PostWorkoutsWorkoutIdComplete409JSONResponse{Message: "workout already completed"}, nil
	}

	notes := currentWorkout.Notes
	if request.Body != nil && request.Body.Notes != nil {
		notes = strings.TrimSpace(*request.Body.Notes)
	}

	updatedWorkout, err := s.queries.CompleteWorkout(ctx, db.CompleteWorkoutParams{
		ID:          workoutID,
		UserID:      userID,
		CompletedAt: sql.NullTime{Time: s.currentTime().UTC(), Valid: true},
		Notes:       notes,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			latestWorkout, latestErr := s.queries.GetWorkoutByIDAndUserID(ctx, db.GetWorkoutByIDAndUserIDParams{ID: workoutID, UserID: userID})
			if latestErr != nil {
				if errors.Is(latestErr, sql.ErrNoRows) {
					return generated.PostWorkoutsWorkoutIdComplete404JSONResponse{Message: "workout not found"}, nil
				}
				s.logger.Error("failed fetching workout after complete race", zap.Error(latestErr), zap.String("workout_id", workoutID.String()), zap.String("user_id", userID.String()))
				return generated.PostWorkoutsWorkoutIdComplete400JSONResponse{Message: "could not complete workout"}, nil
			}

			if latestWorkout.CompletedAt.Valid {
				return generated.PostWorkoutsWorkoutIdComplete409JSONResponse{Message: "workout already completed"}, nil
			}
		}

		s.logger.Error("failed completing workout", zap.Error(err), zap.String("workout_id", workoutID.String()), zap.String("user_id", userID.String()))
		return generated.PostWorkoutsWorkoutIdComplete400JSONResponse{Message: "could not complete workout"}, nil
	}

	if err := s.updateProgressionStateAfterWorkoutComplete(ctx, userID, updatedWorkout); err != nil {
		s.logger.Error("failed updating progression state", zap.Error(err), zap.String("workout_id", workoutID.String()), zap.String("user_id", userID.String()))
		return generated.PostWorkoutsWorkoutIdComplete400JSONResponse{Message: "could not complete workout"}, nil
	}

	workout, err := s.buildWorkoutResponse(ctx, updatedWorkout)
	if err != nil {
		s.logger.Error("failed building completed workout response", zap.Error(err), zap.String("workout_id", workoutID.String()))
		return generated.PostWorkoutsWorkoutIdComplete400JSONResponse{Message: "could not complete workout"}, nil
	}

	return generated.PostWorkoutsWorkoutIdComplete200JSONResponse{Workout: workout}, nil
}

func (s *Server) GetWorkoutsHistory(ctx context.Context, request generated.GetWorkoutsHistoryRequestObject) (generated.GetWorkoutsHistoryResponseObject, error) {
	userID, ok := middleware.AuthenticatedUserID(ctx)
	if !ok {
		return generated.GetWorkoutsHistory401JSONResponse{Message: "unauthorized"}, nil
	}

	limit := defaultWorkoutHistoryLimit
	if request.Params.Limit != nil {
		if *request.Params.Limit < 1 || *request.Params.Limit > maxWorkoutHistoryLimit {
			return generated.GetWorkoutsHistory400JSONResponse{Message: "limit must be between 1 and 100"}, nil
		}
		limit = *request.Params.Limit
	}

	cursor := workoutHistoryCursor{}
	hasCursor := false
	if request.Params.Cursor != nil && strings.TrimSpace(*request.Params.Cursor) != "" {
		parsedCursor, err := decodeWorkoutHistoryCursor(*request.Params.Cursor)
		if err != nil {
			return generated.GetWorkoutsHistory400JSONResponse{Message: "invalid cursor"}, nil
		}
		cursor = parsedCursor
		hasCursor = true
	}

	rows, err := s.queries.ListWorkoutHistory(ctx, db.ListWorkoutHistoryParams{
		UserID:          userID,
		HasCursor:       hasCursor,
		CursorStartedAt: cursor.StartedAt,
		CursorID:        cursor.WorkoutID,
		LimitCount:      limit + 1,
	})
	if err != nil {
		s.logger.Error("failed listing workout history", zap.Error(err), zap.String("user_id", userID.String()))
		return generated.GetWorkoutsHistory400JSONResponse{Message: "could not list workout history"}, nil
	}

	hasMore := len(rows) > int(limit)
	if hasMore {
		rows = rows[:limit]
	}

	history := make([]generated.WorkoutSummary, 0, len(rows))
	for _, row := range rows {
		history = append(history, toAPIWorkoutSummary(row))
	}

	var nextCursor *string
	if hasMore && len(rows) > 0 {
		cursorValue := encodeWorkoutHistoryCursor(workoutHistoryCursor{StartedAt: rows[len(rows)-1].StartedAt, WorkoutID: rows[len(rows)-1].ID})
		nextCursor = &cursorValue
	}

	return generated.GetWorkoutsHistory200JSONResponse{
		Workouts:   history,
		NextCursor: nextCursor,
	}, nil
}

func (s *Server) GetWorkoutsId(ctx context.Context, request generated.GetWorkoutsIdRequestObject) (generated.GetWorkoutsIdResponseObject, error) {
	userID, ok := middleware.AuthenticatedUserID(ctx)
	if !ok {
		return generated.GetWorkoutsId401JSONResponse{Message: "unauthorized"}, nil
	}

	workoutID := uuid.UUID(request.Id)
	workoutRow, err := s.queries.GetWorkoutByIDAndUserID(ctx, db.GetWorkoutByIDAndUserIDParams{
		ID:     workoutID,
		UserID: userID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return generated.GetWorkoutsId404JSONResponse{Message: "workout not found"}, nil
		}
		s.logger.Error("failed fetching workout", zap.Error(err), zap.String("workout_id", workoutID.String()), zap.String("user_id", userID.String()))
		return generated.GetWorkoutsId404JSONResponse{Message: "workout not found"}, nil
	}

	workout, err := s.buildWorkoutResponse(ctx, workoutRow)
	if err != nil {
		s.logger.Error("failed building workout detail response", zap.Error(err), zap.String("workout_id", workoutID.String()))
		return generated.GetWorkoutsId404JSONResponse{Message: "workout not found"}, nil
	}

	return generated.GetWorkoutsId200JSONResponse{Workout: workout}, nil
}

func (s *Server) buildWorkoutResponse(ctx context.Context, workoutRow db.Workout) (generated.Workout, error) {
	exerciseRows, err := s.queries.ListWorkoutExercisesByWorkoutID(ctx, workoutRow.ID)
	if err != nil {
		return generated.Workout{}, err
	}

	setRows, err := s.queries.ListWorkoutSetsByWorkoutID(ctx, workoutRow.ID)
	if err != nil {
		return generated.Workout{}, err
	}

	previousSetRows, err := s.queries.ListPreviousWorkoutSetsByWorkoutID(ctx, workoutRow.ID)
	if err != nil {
		return generated.Workout{}, err
	}

	setsByExerciseID := make(map[uuid.UUID][]generated.WorkoutSet, len(exerciseRows))
	for _, setRow := range setRows {
		setsByExerciseID[setRow.WorkoutExerciseID] = append(setsByExerciseID[setRow.WorkoutExerciseID], toAPIWorkoutSet(setRow))
	}

	type previousPerformancePayload struct {
		WorkoutID   uuid.UUID
		CompletedAt time.Time
		Sets        []map[string]interface{}
	}

	previousPerformanceByExerciseID := make(map[uuid.UUID]previousPerformancePayload, len(previousSetRows))
	for _, previousSetRow := range previousSetRows {
		if !previousSetRow.PreviousWorkoutCompletedAt.Valid {
			continue
		}

		payload, exists := previousPerformanceByExerciseID[previousSetRow.TargetWorkoutExerciseID]
		if !exists {
			payload = previousPerformancePayload{
				WorkoutID:   previousSetRow.PreviousWorkoutID,
				CompletedAt: previousSetRow.PreviousWorkoutCompletedAt.Time,
				Sets:        make([]map[string]interface{}, 0, 4),
			}
		}

		payload.Sets = append(payload.Sets, map[string]interface{}{
			"set_index": previousSetRow.SetIndex,
			"reps":      previousSetRow.Reps,
			"weight_kg": previousSetRow.WeightKg,
			"rpe":       nullableFloat64(previousSetRow.Rpe),
		})

		previousPerformanceByExerciseID[previousSetRow.TargetWorkoutExerciseID] = payload
	}

	exercises := make([]generated.WorkoutExercise, 0, len(exerciseRows))
	for _, row := range exerciseRows {
		plannedJSON, err := decodeJSONObject(row.PlannedJson)
		if err != nil {
			return generated.Workout{}, err
		}
		if previousPerformance, ok := previousPerformanceByExerciseID[row.ID]; ok {
			plannedJSON["previous_performance"] = map[string]interface{}{
				"workout_id":   previousPerformance.WorkoutID.String(),
				"completed_at": previousPerformance.CompletedAt,
				"sets":         previousPerformance.Sets,
			}
		}

		actualJSON, err := decodeJSONObject(row.ActualJson)
		if err != nil {
			return generated.Workout{}, err
		}

		sets := setsByExerciseID[row.ID]
		if sets == nil {
			sets = []generated.WorkoutSet{}
		}

		exercises = append(exercises, generated.WorkoutExercise{
			Id:           openapi_types.UUID(row.ID),
			WorkoutId:    openapi_types.UUID(row.WorkoutID),
			ExerciseId:   openapi_types.UUID(row.ExerciseID),
			OrderIndex:   row.OrderIndex,
			PlannedJson:  plannedJSON,
			ActualJson:   actualJSON,
			CreatedAt:    row.CreatedAt,
			ExerciseSlug: row.ExerciseSlug,
			ExerciseName: row.ExerciseName,
			Sets:         sets,
		})
	}

	return toAPIWorkout(workoutRow, exercises), nil
}

func toAPIWorkout(workoutRow db.Workout, exercises []generated.WorkoutExercise) generated.Workout {
	if exercises == nil {
		exercises = []generated.WorkoutExercise{}
	}

	return generated.Workout{
		Id:               openapi_types.UUID(workoutRow.ID),
		UserId:           openapi_types.UUID(workoutRow.UserID),
		ProgramSessionId: nullableUUIDPointer(workoutRow.ProgramSessionID),
		StartedAt:        workoutRow.StartedAt,
		CompletedAt:      nullableTimePointer(workoutRow.CompletedAt),
		Notes:            workoutRow.Notes,
		CreatedAt:        workoutRow.CreatedAt,
		Exercises:        exercises,
	}
}

func toAPIWorkoutSummary(workoutRow db.Workout) generated.WorkoutSummary {
	return generated.WorkoutSummary{
		Id:               openapi_types.UUID(workoutRow.ID),
		UserId:           openapi_types.UUID(workoutRow.UserID),
		ProgramSessionId: nullableUUIDPointer(workoutRow.ProgramSessionID),
		StartedAt:        workoutRow.StartedAt,
		CompletedAt:      nullableTimePointer(workoutRow.CompletedAt),
		Notes:            workoutRow.Notes,
		CreatedAt:        workoutRow.CreatedAt,
	}
}

func toAPIWorkoutSet(setRow db.WorkoutSet) generated.WorkoutSet {
	var rpe *float32
	if setRow.Rpe.Valid {
		value := float32(setRow.Rpe.Float64)
		rpe = &value
	}

	return generated.WorkoutSet{
		Id:                openapi_types.UUID(setRow.ID),
		WorkoutExerciseId: openapi_types.UUID(setRow.WorkoutExerciseID),
		SetIndex:          setRow.SetIndex,
		Reps:              setRow.Reps,
		WeightKg:          float32(setRow.WeightKg),
		Rpe:               rpe,
		CompletedAt:       setRow.CompletedAt,
		CreatedAt:         setRow.CreatedAt,
	}
}

func decodeJSONObject(raw json.RawMessage) (map[string]interface{}, error) {
	if len(raw) == 0 {
		return map[string]interface{}{}, nil
	}

	var value map[string]interface{}
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	if value == nil {
		value = map[string]interface{}{}
	}

	return value, nil
}

func nullableUUIDPointer(value uuid.NullUUID) *openapi_types.UUID {
	if !value.Valid {
		return nil
	}

	mapped := openapi_types.UUID(value.UUID)
	return &mapped
}

func nullableTimePointer(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}

	timestamp := value.Time
	return &timestamp
}

func nullableFloat64(value sql.NullFloat64) interface{} {
	if !value.Valid {
		return nil
	}

	return value.Float64
}

func encodeWorkoutHistoryCursor(cursor workoutHistoryCursor) string {
	payload := fmt.Sprintf("%s|%s", cursor.StartedAt.UTC().Format(time.RFC3339Nano), cursor.WorkoutID.String())
	return base64.RawURLEncoding.EncodeToString([]byte(payload))
}

func decodeWorkoutHistoryCursor(raw string) (workoutHistoryCursor, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(raw))
	if err != nil {
		return workoutHistoryCursor{}, err
	}

	parts := strings.Split(string(decoded), "|")
	if len(parts) != 2 {
		return workoutHistoryCursor{}, fmt.Errorf("invalid cursor payload")
	}

	startedAt, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return workoutHistoryCursor{}, err
	}

	workoutID, err := uuid.Parse(parts[1])
	if err != nil {
		return workoutHistoryCursor{}, err
	}

	return workoutHistoryCursor{
		StartedAt: startedAt.UTC(),
		WorkoutID: workoutID,
	}, nil
}

func (s *Server) updateProgressionStateAfterWorkoutComplete(ctx context.Context, userID uuid.UUID, workoutRow db.Workout) error {
	exerciseRows, err := s.queries.ListWorkoutExercisesByWorkoutID(ctx, workoutRow.ID)
	if err != nil {
		return err
	}

	setRows, err := s.queries.ListWorkoutSetsByWorkoutID(ctx, workoutRow.ID)
	if err != nil {
		return err
	}

	bestSetByWorkoutExerciseID := make(map[uuid.UUID]db.WorkoutSet, len(exerciseRows))
	for _, setRow := range setRows {
		current, exists := bestSetByWorkoutExerciseID[setRow.WorkoutExerciseID]
		if !exists ||
			setRow.WeightKg > current.WeightKg ||
			(setRow.WeightKg == current.WeightKg && setRow.Reps > current.Reps) ||
			(setRow.WeightKg == current.WeightKg && setRow.Reps == current.Reps && setRow.SetIndex > current.SetIndex) {
			bestSetByWorkoutExerciseID[setRow.WorkoutExerciseID] = setRow
		}
	}

	for _, exerciseRow := range exerciseRows {
		bestSet, ok := bestSetByWorkoutExerciseID[exerciseRow.ID]
		if !ok {
			continue
		}

		if err := s.queries.UpsertUserExerciseProgress(ctx, db.UpsertUserExerciseProgressParams{
			UserID:     userID,
			ExerciseID: exerciseRow.ExerciseID,
			LastLoad:   bestSet.WeightKg,
			LastReps:   bestSet.Reps,
			LastRpe:    bestSet.Rpe,
		}); err != nil {
			return err
		}
	}

	if !workoutRow.ProgramSessionID.Valid {
		return nil
	}

	programID, err := s.queries.GetProgramIDByProgramSessionID(ctx, workoutRow.ProgramSessionID.UUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}

	now := s.currentTime().UTC()
	programRow, err := s.queries.GetProgramByID(ctx, programID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	currentWeek := int32(1)
	currentWeekStart := normalizeDateUTC(now)
	enrollmentRow, err := s.queries.GetUserProgramEnrollmentByUserID(ctx, userID)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
	} else {
		weeksLength := programRow.WeeksLength
		if weeksLength < 1 {
			weeksLength = maxInt32(enrollmentRow.CurrentWeek, 1)
		}

		currentWeek = resolveBlockWeekIndex(enrollmentRow.StartDate, now, weeksLength)
		currentWeek = maxInt32(currentWeek, 1)

		start, _ := resolveBlockDateRange(enrollmentRow.StartDate, currentWeek)
		currentWeekStart = start
	}

	previousWeekIndex := currentWeek - 1
	if previousWeekIndex < 1 {
		previousWeekIndex = 1
	}

	scheduledSessions, err := s.queries.CountProgramSessionsForWeek(ctx, db.CountProgramSessionsForWeekParams{
		ProgramID: programID,
		WeekIndex: previousWeekIndex,
	})
	if err != nil {
		return err
	}

	previousWeekStart := currentWeekStart.AddDate(0, 0, -7)
	previousWeekEnd := currentWeekStart
	completedSessionsLastWeek, err := s.queries.CountCompletedProgramWorkoutsBetween(ctx, db.CountCompletedProgramWorkoutsBetweenParams{
		UserID:      userID,
		ProgramID:   programID,
		WeekIndex:   previousWeekIndex,
		WindowStart: sql.NullTime{Time: previousWeekStart, Valid: true},
		WindowEnd:   sql.NullTime{Time: previousWeekEnd, Valid: true},
	})
	if err != nil {
		return err
	}

	completedSetsLastWeek, err := s.queries.CountCompletedProgramWorkoutSetsBetween(ctx, db.CountCompletedProgramWorkoutSetsBetweenParams{
		UserID:      userID,
		ProgramID:   programID,
		WeekIndex:   previousWeekIndex,
		WindowStart: sql.NullTime{Time: previousWeekStart, Valid: true},
		WindowEnd:   sql.NullTime{Time: previousWeekEnd, Valid: true},
	})
	if err != nil {
		return err
	}

	highRpeSetsLastWeek, err := s.queries.CountHighRpeProgramWorkoutSetsBetween(ctx, db.CountHighRpeProgramWorkoutSetsBetweenParams{
		UserID:       userID,
		ProgramID:    programID,
		WeekIndex:    previousWeekIndex,
		WindowStart:  sql.NullTime{Time: previousWeekStart, Valid: true},
		WindowEnd:    sql.NullTime{Time: previousWeekEnd, Valid: true},
		RpeThreshold: sql.NullFloat64{Float64: 9, Valid: true},
	})
	if err != nil {
		return err
	}

	beforeTime := now
	if workoutRow.CompletedAt.Valid {
		beforeTime = workoutRow.CompletedAt.Time.UTC()
	}
	priorCompletedWorkouts, err := s.queries.CountCompletedProgramWorkoutsBefore(ctx, db.CountCompletedProgramWorkoutsBeforeParams{
		UserID:     userID,
		ProgramID:  programID,
		BeforeTime: sql.NullTime{Time: beforeTime, Valid: true},
	})
	if err != nil {
		return err
	}

	previousWeekAdherence := 1.0
	previousWeekDensity := 0.0
	previousConsecutiveLowAdherence := int32(0)

	stateRow, err := s.queries.GetUserProgramStateByUserIDAndProgramID(ctx, db.GetUserProgramStateByUserIDAndProgramIDParams{
		UserID:    userID,
		ProgramID: programID,
	})
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
	} else {
		previousWeekAdherence = stateRow.LastWeekAdherence
		previousWeekDensity = stateRow.LastWeekDensity
		previousConsecutiveLowAdherence = stateRow.ConsecutiveLowAdherenceWeeks
	}

	recoveredPerformance := hasRecoveredPerformance(exerciseRows, bestSetByWorkoutExerciseID)

	weeklyAdjustment := progression.ComputeWeeklyAdjustment(progression.WeeklyAdjustmentInput{
		ScheduledSessions:               scheduledSessions,
		CompletedSessions:               completedSessionsLastWeek,
		CompletedSets:                   completedSetsLastWeek,
		HighRpeSets:                     highRpeSetsLastWeek,
		PreviousWeekAdherence:           previousWeekAdherence,
		PreviousWeekDensity:             previousWeekDensity,
		PreviousConsecutiveLowAdherence: previousConsecutiveLowAdherence,
		RecoveredPerformance:            recoveredPerformance,
		HasProgramHistory:               priorCompletedWorkouts > 0,
	})

	reasonsJSON, err := json.Marshal(weeklyAdjustment.Reasons)
	if err != nil {
		return err
	}

	return s.queries.UpsertUserProgramState(ctx, db.UpsertUserProgramStateParams{
		UserID:                       userID,
		ProgramID:                    programID,
		CurrentWeek:                  currentWeek,
		DeloadFlag:                   weeklyAdjustment.DeloadFlag,
		LastWeekAdherence:            weeklyAdjustment.LastWeekAdherence,
		LastWeekScheduledSessions:    weeklyAdjustment.LastWeekScheduledSessions,
		LastWeekCompletedSessions:    weeklyAdjustment.LastWeekCompletedSessions,
		LastWeekDensity:              weeklyAdjustment.LastWeekDensity,
		LastWeekHighRpeRate:          weeklyAdjustment.LastWeekHighRpeRate,
		FatigueScore:                 weeklyAdjustment.FatigueScore,
		ConsecutiveLowAdherenceWeeks: weeklyAdjustment.ConsecutiveLowAdherenceWeeks,
		AdjustmentReasonsJson:        reasonsJSON,
	})
}

type progressionPlannedPrescription struct {
	RepsRange string   `json:"reps_range"`
	RpeTarget *float64 `json:"rpe_target"`
}

func hasRecoveredPerformance(
	exerciseRows []db.ListWorkoutExercisesByWorkoutIDRow,
	bestSetByWorkoutExerciseID map[uuid.UUID]db.WorkoutSet,
) bool {
	for _, exerciseRow := range exerciseRows {
		bestSet, ok := bestSetByWorkoutExerciseID[exerciseRow.ID]
		if !ok {
			continue
		}

		prescription := progressionPlannedPrescription{}
		if len(exerciseRow.PlannedJson) > 0 {
			_ = json.Unmarshal(exerciseRow.PlannedJson, &prescription)
		}

		targetRepsMin, _ := progression.ParseRepsRange(prescription.RepsRange)
		if targetRepsMin == 0 {
			targetRepsMin = 1
		}

		targetRPE := 8.0
		if prescription.RpeTarget != nil && *prescription.RpeTarget > 0 {
			targetRPE = *prescription.RpeTarget
		}

		repsRecovered := bestSet.Reps >= targetRepsMin
		rpeRecovered := !bestSet.Rpe.Valid || bestSet.Rpe.Float64 <= targetRPE
		return repsRecovered && rpeRecovered
	}

	return false
}
