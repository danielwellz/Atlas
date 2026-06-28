package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	db "github.com/atlas/atlas-api/internal/db/sqlc"
	"github.com/atlas/atlas-api/internal/httpapi/generated"
	"github.com/atlas/atlas-api/internal/progression"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

var (
	errNoActiveProgramEnrollment = errors.New("no active program enrollment")
	errActiveProgramNotFound     = errors.New("active program not found")
	errProgramScheduleNotFound   = errors.New("program schedule not found")
)

type currentProgramData struct {
	enrollment      db.UserProgramEnrollment
	program         db.Program
	goals           *db.UserGoal
	programState    *db.UserProgramState
	blocks          []generated.ProgramBlock
	blockByWeek     map[int32]generated.ProgramBlock
	templateByWeek  map[int32]db.ListProgramBlocksByProgramIDRow
	weeklyFrequency int32
}

type schedulePreferences struct {
	preferredDays []int32
	equipment     []string
}

type sessionAssignment struct {
	row       db.ProgramSession
	dayOfWeek int32
}

func (s *Server) loadCurrentProgramData(ctx context.Context, userID uuid.UUID) (currentProgramData, error) {
	enrollmentRow, err := s.queries.GetUserProgramEnrollmentByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return currentProgramData{}, errNoActiveProgramEnrollment
		}
		return currentProgramData{}, err
	}

	programRow, err := s.queries.GetProgramByID(ctx, enrollmentRow.ProgramID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return currentProgramData{}, errActiveProgramNotFound
		}
		return currentProgramData{}, err
	}

	blockRows, err := s.queries.ListProgramBlocksByProgramID(ctx, programRow.ID)
	if err != nil {
		return currentProgramData{}, err
	}
	if len(blockRows) == 0 {
		return currentProgramData{}, errProgramScheduleNotFound
	}

	blocks, blockByWeek, templateByWeek, weeklyFrequency := buildProgramBlocks(
		programRow.ID,
		programRow.WeeksLength,
		blockRows,
	)

	var goals *db.UserGoal
	goalsRow, err := s.queries.GetUserGoalsByUserID(ctx, userID)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return currentProgramData{}, err
		}
	} else {
		goals = &goalsRow
	}

	var programState *db.UserProgramState
	programStateRow, err := s.queries.GetUserProgramStateByUserIDAndProgramID(ctx, db.GetUserProgramStateByUserIDAndProgramIDParams{
		UserID:    userID,
		ProgramID: programRow.ID,
	})
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return currentProgramData{}, err
		}
	} else {
		programState = &programStateRow
	}

	return currentProgramData{
		enrollment:      enrollmentRow,
		program:         programRow,
		goals:           goals,
		programState:    programState,
		blocks:          blocks,
		blockByWeek:     blockByWeek,
		templateByWeek:  templateByWeek,
		weeklyFrequency: weeklyFrequency,
	}, nil
}

func buildProgramBlocks(
	programID uuid.UUID,
	weeksLength int32,
	rows []db.ListProgramBlocksByProgramIDRow,
) (
	[]generated.ProgramBlock,
	map[int32]generated.ProgramBlock,
	map[int32]db.ListProgramBlocksByProgramIDRow,
	int32,
) {
	templateByWeek := make(map[int32]db.ListProgramBlocksByProgramIDRow, len(rows))
	for _, row := range rows {
		templateByWeek[row.WeekIndex] = row
	}

	if weeksLength < 1 {
		weeksLength = int32(len(rows))
	}
	if weeksLength < 1 {
		weeksLength = 1
	}

	blocks := make([]generated.ProgramBlock, 0, weeksLength)
	blockByWeek := make(map[int32]generated.ProgramBlock, weeksLength)
	weeklyFrequency := int32(0)

	for weekIndex := int32(1); weekIndex <= weeksLength; weekIndex++ {
		source, sourceExists := templateByWeek[weekIndex]
		if !sourceExists {
			templateWeekIndex := resolveTemplateWeekIndex(weekIndex, templateByWeek)
			source = templateByWeek[templateWeekIndex]
		}

		sessionDays := append([]int32(nil), source.SessionDays...)
		sort.Slice(sessionDays, func(i, j int) bool {
			return sessionDays[i] < sessionDays[j]
		})

		sessionCount := source.SessionCount
		if sessionCount < 0 {
			sessionCount = 0
		}
		if sessionCount == 0 && len(sessionDays) > 0 {
			sessionCount = int32(len(sessionDays))
		}

		blockID := source.ID
		if !sourceExists {
			blockID = uuid.NewSHA1(programID, []byte(fmt.Sprintf("block-week-%d", weekIndex)))
		}

		if sessionCount > weeklyFrequency {
			weeklyFrequency = sessionCount
		}

		block := generated.ProgramBlock{
			Id:           openapi_types.UUID(blockID),
			WeekIndex:    weekIndex,
			SessionCount: sessionCount,
			SessionDays:  sessionDays,
		}

		blocks = append(blocks, block)
		blockByWeek[weekIndex] = block
	}

	if weeklyFrequency == 0 && len(blocks) > 0 {
		weeklyFrequency = int32(len(blocks[0].SessionDays))
	}

	return blocks, blockByWeek, templateByWeek, weeklyFrequency
}

func resolveTemplateWeekIndex(
	blockWeekIndex int32,
	templateByWeek map[int32]db.ListProgramBlocksByProgramIDRow,
) int32 {
	if _, exists := templateByWeek[blockWeekIndex]; exists {
		return blockWeekIndex
	}

	keys := make([]int32, 0, len(templateByWeek))
	for weekIndex := range templateByWeek {
		keys = append(keys, weekIndex)
	}
	if len(keys) == 0 {
		return 1
	}
	sort.Slice(keys, func(i, j int) bool {
		return keys[i] < keys[j]
	})

	candidate := keys[0]
	for _, weekIndex := range keys {
		if weekIndex > blockWeekIndex {
			break
		}
		candidate = weekIndex
	}
	return candidate
}

func resolveBlockWeekIndex(startDate time.Time, targetDate time.Time, totalWeeks int32) int32 {
	if totalWeeks < 1 {
		totalWeeks = 1
	}

	start := normalizeDateUTC(startDate)
	target := normalizeDateUTC(targetDate)
	if target.Before(start) {
		return 1
	}

	elapsedDays := int(target.Sub(start).Hours() / 24)
	weekIndex := int32(elapsedDays/7 + 1)
	if weekIndex < 1 {
		weekIndex = 1
	}
	if weekIndex > totalWeeks {
		weekIndex = totalWeeks
	}

	return weekIndex
}

func resolveBlockDateRange(startDate time.Time, blockWeekIndex int32) (time.Time, time.Time) {
	if blockWeekIndex < 1 {
		blockWeekIndex = 1
	}

	start := normalizeDateUTC(startDate).AddDate(0, 0, int((blockWeekIndex-1)*7))
	end := start.AddDate(0, 0, 6)
	return start, end
}

func (d currentProgramData) buildScheduleContext(targetDate time.Time) generated.ProgramScheduleContext {
	blockWeekIndex := resolveBlockWeekIndex(d.enrollment.StartDate, targetDate, d.program.WeeksLength)
	templateWeekIndex := resolveTemplateWeekIndex(blockWeekIndex, d.templateByWeek)
	blockStart, blockEnd := resolveBlockDateRange(d.enrollment.StartDate, blockWeekIndex)

	return generated.ProgramScheduleContext{
		BlockWeekIndex:    blockWeekIndex,
		TemplateWeekIndex: templateWeekIndex,
		TotalWeeks:        maxInt32(d.program.WeeksLength, 1),
		BlockStartDate:    openapi_types.Date{Time: blockStart},
		BlockEndDate:      openapi_types.Date{Time: blockEnd},
	}
}

func (d currentProgramData) toAPIProgram() (generated.Program, error) {
	program, err := toAPIProgram(d.program)
	if err != nil {
		return generated.Program{}, err
	}
	program.Blocks = append([]generated.ProgramBlock(nil), d.blocks...)
	program.WeeklyFrequency = d.weeklyFrequency
	return program, nil
}

func (d currentProgramData) schedulePreferences() (schedulePreferences, error) {
	preferences := schedulePreferences{
		preferredDays: []int32{},
		equipment:     []string{},
	}

	if d.goals == nil {
		return preferences, nil
	}

	equipment, err := parseStringSliceJSON(d.goals.EquipmentAccessJson)
	if err != nil {
		return schedulePreferences{}, err
	}
	preferences.equipment = normalizeTokens(equipment)

	constraints := map[string]interface{}{}
	if len(d.goals.ConstraintsJson) > 0 {
		if err := json.Unmarshal(d.goals.ConstraintsJson, &constraints); err != nil {
			return schedulePreferences{}, err
		}
	}

	dayNames := extractScheduleDays(constraints, d.goals.DaysPerWeek)
	preferredDays := make([]int32, 0, len(dayNames))
	for _, dayName := range dayNames {
		if dayIndex, ok := dayNameToISO(dayName); ok {
			preferredDays = append(preferredDays, dayIndex)
		}
	}
	sort.Slice(preferredDays, func(i, j int) bool {
		return preferredDays[i] < preferredDays[j]
	})
	preferences.preferredDays = dedupeSortedInt32(preferredDays)

	return preferences, nil
}

func assignSessionDays(
	sessionRows []db.ProgramSession,
	preferredDays []int32,
	weeklyFrequency int32,
) []sessionAssignment {
	if len(sessionRows) == 0 {
		return []sessionAssignment{}
	}

	sortedRows := append([]db.ProgramSession(nil), sessionRows...)
	sort.Slice(sortedRows, func(i, j int) bool {
		return sortedRows[i].DayOfWeek < sortedRows[j].DayOfWeek
	})

	targetCount := len(sortedRows)
	if weeklyFrequency > 0 && int(weeklyFrequency) < targetCount {
		targetCount = int(weeklyFrequency)
	}
	if len(preferredDays) > 0 && len(preferredDays) < targetCount {
		targetCount = len(preferredDays)
	}

	if targetCount < 1 {
		return []sessionAssignment{}
	}

	assignments := make([]sessionAssignment, 0, targetCount)
	for index := 0; index < targetCount; index++ {
		dayOfWeek := sortedRows[index].DayOfWeek
		if index < len(preferredDays) {
			dayOfWeek = preferredDays[index]
		}

		assignments = append(assignments, sessionAssignment{
			row:       sortedRows[index],
			dayOfWeek: dayOfWeek,
		})
	}

	sort.Slice(assignments, func(i, j int) bool {
		return assignments[i].dayOfWeek < assignments[j].dayOfWeek
	})

	return assignments
}

func (s *Server) buildProgramSessionsForTemplateWeek(
	ctx context.Context,
	userID uuid.UUID,
	templateWeekID uuid.UUID,
	preferences schedulePreferences,
	weeklyFrequency int32,
	programState *db.UserProgramState,
) ([]generated.ProgramSession, error) {
	sessionRows, err := s.queries.ListProgramSessionsByProgramWeekID(ctx, templateWeekID)
	if err != nil {
		return nil, err
	}

	assignments := assignSessionDays(sessionRows, preferences.preferredDays, weeklyFrequency)
	sessions := make([]generated.ProgramSession, 0, len(assignments))
	for _, assignment := range assignments {
		exercises, err := s.buildProgramSessionExercises(
			ctx,
			userID,
			assignment.row.ID,
			preferences.equipment,
			programState,
		)
		if err != nil {
			return nil, err
		}

		sessions = append(sessions, generated.ProgramSession{
			Id:        openapi_types.UUID(assignment.row.ID),
			DayOfWeek: assignment.dayOfWeek,
			Name:      assignment.row.Name,
			Exercises: exercises,
		})
	}

	return sessions, nil
}

func (s *Server) buildProgramSessionExercises(
	ctx context.Context,
	userID uuid.UUID,
	programSessionID uuid.UUID,
	equipmentFilter []string,
	programState *db.UserProgramState,
) ([]generated.ProgramSessionExercise, error) {
	exerciseRows, err := s.queries.ListProgramSessionExercisesByProgramSessionID(ctx, programSessionID)
	if err != nil {
		return nil, err
	}

	exercises := make([]generated.ProgramSessionExercise, 0, len(exerciseRows))
	for _, exerciseRow := range exerciseRows {
		prescription := generated.ProgramPrescription{}
		if err := json.Unmarshal(exerciseRow.PrescriptionJson, &prescription); err != nil {
			return nil, err
		}

		substitutionRows, err := s.queries.ListProgramSubstitutionCandidates(ctx, db.ListProgramSubstitutionCandidatesParams{
			Equipment:  equipmentFilter,
			MaxCount:   4,
			ExerciseID: exerciseRow.ExerciseID,
		})
		if err != nil {
			return nil, err
		}

		substitutionCandidates := make([]generated.ProgramSubstitutionCandidate, 0, len(substitutionRows))
		for _, substitutionRow := range substitutionRows {
			equipment, err := parseStringSliceJSON(substitutionRow.EquipmentJson)
			if err != nil {
				return nil, err
			}

			substitutionCandidates = append(substitutionCandidates, generated.ProgramSubstitutionCandidate{
				ExerciseId:      openapi_types.UUID(substitutionRow.ID),
				ExerciseSlug:    substitutionRow.Slug,
				ExerciseName:    substitutionRow.Name,
				MovementPattern: substitutionRow.MovementPattern,
				Equipment:       normalizeTokens(equipment),
			})
		}

		var recommendedLoadKg *float32
		exerciseAdjustmentReasons := make([]string, 0, 4)
		if programState != nil {
			exerciseAdjustmentReasons = appendUniqueReasons(exerciseAdjustmentReasons, parseAdjustmentReasons(programState.AdjustmentReasonsJson)...)
		}

		if exerciseRow.OrderIndex == 1 {
			progressRow, err := s.queries.GetUserExerciseProgressByUserIDAndExerciseID(ctx, db.GetUserExerciseProgressByUserIDAndExerciseIDParams{
				UserID:     userID,
				ExerciseID: exerciseRow.ExerciseID,
			})
			if err != nil {
				if !errors.Is(err, sql.ErrNoRows) {
					return nil, err
				}
			} else {
				targetRepsMin, targetRepsMax := progression.ParseRepsRange(prescription.RepsRange)

				var targetRPE *float64
				if prescription.RpeTarget != nil {
					target := float64(*prescription.RpeTarget)
					targetRPE = &target
				}

				var lastRPE *float64
				if progressRow.LastRpe.Valid {
					lastRPEValue := progressRow.LastRpe.Float64
					lastRPE = &lastRPEValue
				}

				deloadFlag := programState != nil && programState.DeloadFlag
				lastWeekAdherence := 1.0
				fatigueScore := 0.0
				recovered := true
				if programState != nil {
					lastWeekAdherence = programState.LastWeekAdherence
					fatigueScore = programState.FatigueScore
					recovered = programState.LastWeekHighRpeRate < 0.25 || programState.LastWeekAdherence >= 0.85
				}

				recommendation := progression.RecommendLoad(progression.LoadRecommendationInput{
					LastLoadKg:        progressRow.LastLoad,
					LastReps:          progressRow.LastReps,
					LastRPE:           lastRPE,
					TargetRepsMin:     targetRepsMin,
					TargetRepsMax:     targetRepsMax,
					TargetRPE:         targetRPE,
					LastWeekAdherence: lastWeekAdherence,
					FatigueScore:      fatigueScore,
					Recovered:         recovered,
					AdjustmentReasons: exerciseAdjustmentReasons,
					DeloadFlag:        deloadFlag,
				})

				load := float32(recommendation.LoadKg)
				recommendedLoadKg = &load
				exerciseAdjustmentReasons = appendUniqueReasons(exerciseAdjustmentReasons, recommendation.Why)
			}
		}

		if programState != nil && programState.DeloadFlag {
			adjustedSets := progression.AdjustVolumeForCatchUp(prescription.Sets, true)
			if adjustedSets != prescription.Sets {
				prescription.Sets = adjustedSets
				if exerciseRow.OrderIndex == 1 {
					exerciseAdjustmentReasons = appendUniqueReasons(
						exerciseAdjustmentReasons,
						"Volume reduced by one set for this catch-up deload week.",
					)
				}
			}
		}

		var progressionWhy *string
		if len(exerciseAdjustmentReasons) > 0 && exerciseRow.OrderIndex == 1 {
			why := strings.Join(exerciseAdjustmentReasons, " ")
			progressionWhy = &why
		}

		exercises = append(exercises, generated.ProgramSessionExercise{
			Id:                     openapi_types.UUID(exerciseRow.ID),
			OrderIndex:             exerciseRow.OrderIndex,
			ExerciseId:             openapi_types.UUID(exerciseRow.ExerciseID),
			ExerciseSlug:           exerciseRow.ExerciseSlug,
			ExerciseName:           exerciseRow.ExerciseName,
			Prescription:           prescription,
			RecommendedLoadKg:      recommendedLoadKg,
			AdjustmentReasons:      nilIfEmptyReasons(exerciseAdjustmentReasons),
			ProgressionWhy:         progressionWhy,
			SubstitutionCandidates: substitutionCandidates,
		})
	}

	return exercises, nil
}

func (s *Server) generateScheduledSessionsInRange(
	ctx context.Context,
	userID uuid.UUID,
	programData currentProgramData,
	fromDate time.Time,
	toDate time.Time,
) ([]generated.ProgramScheduledSession, error) {
	preferences, err := programData.schedulePreferences()
	if err != nil {
		return nil, err
	}

	sessionCache := make(map[int32][]generated.ProgramSession, len(programData.templateByWeek))
	scheduled := make([]generated.ProgramScheduledSession, 0)

	startDate := normalizeDateUTC(programData.enrollment.StartDate)
	for currentDate := normalizeDateUTC(fromDate); !currentDate.After(normalizeDateUTC(toDate)); currentDate = currentDate.AddDate(0, 0, 1) {
		if currentDate.Before(startDate) {
			continue
		}

		blockWeekIndex := resolveBlockWeekIndex(startDate, currentDate, programData.program.WeeksLength)
		templateWeekIndex := resolveTemplateWeekIndex(blockWeekIndex, programData.templateByWeek)

		templateBlock, exists := programData.templateByWeek[templateWeekIndex]
		if !exists {
			return nil, errProgramScheduleNotFound
		}

		sessions, exists := sessionCache[templateWeekIndex]
		if !exists {
			weekSessions, err := s.buildProgramSessionsForTemplateWeek(
				ctx,
				userID,
				templateBlock.ID,
				preferences,
				programData.weeklyFrequency,
				programData.programState,
			)
			if err != nil {
				return nil, err
			}
			sessions = weekSessions
			sessionCache[templateWeekIndex] = weekSessions
		}

		block, exists := programData.blockByWeek[blockWeekIndex]
		if !exists {
			return nil, errProgramScheduleNotFound
		}

		dayOfWeek := isoDayOfWeek(currentDate)
		for _, session := range sessions {
			if session.DayOfWeek != dayOfWeek {
				continue
			}

			scheduled = append(scheduled, generated.ProgramScheduledSession{
				ProgramSessionId: session.Id,
				BlockId:          block.Id,
				BlockWeekIndex:   blockWeekIndex,
				ScheduledDate:    openapi_types.Date{Time: currentDate},
				DayOfWeek:        session.DayOfWeek,
				Name:             session.Name,
				Exercises:        session.Exercises,
			})
		}
	}

	return scheduled, nil
}

func dayNameToISO(value string) (int32, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "monday":
		return 1, true
	case "tuesday":
		return 2, true
	case "wednesday":
		return 3, true
	case "thursday":
		return 4, true
	case "friday":
		return 5, true
	case "saturday":
		return 6, true
	case "sunday":
		return 7, true
	default:
		return 0, false
	}
}

func isoDayOfWeek(value time.Time) int32 {
	return int32((int(value.Weekday())+6)%7 + 1)
}

func normalizeTokens(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}

	normalized := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		token := strings.ToLower(strings.TrimSpace(value))
		if token == "" {
			continue
		}
		if _, exists := seen[token]; exists {
			continue
		}
		seen[token] = struct{}{}
		normalized = append(normalized, token)
	}

	return normalized
}

func parseStringSliceJSON(raw json.RawMessage) ([]string, error) {
	if len(raw) == 0 {
		return []string{}, nil
	}

	values := make([]string, 0)
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, err
	}
	return values, nil
}

func dedupeSortedInt32(values []int32) []int32 {
	if len(values) == 0 {
		return []int32{}
	}

	deduped := make([]int32, 0, len(values))
	var previous int32
	for index, value := range values {
		if index == 0 || value != previous {
			deduped = append(deduped, value)
			previous = value
		}
	}

	return deduped
}

func parseAdjustmentReasons(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return []string{}
	}

	values := make([]string, 0)
	if err := json.Unmarshal(raw, &values); err != nil {
		return []string{}
	}

	return appendUniqueReasons(nil, values...)
}

func appendUniqueReasons(values []string, additions ...string) []string {
	if len(additions) == 0 {
		return values
	}

	next := append([]string(nil), values...)
	seen := make(map[string]struct{}, len(next))
	for _, value := range next {
		seen[strings.ToLower(strings.TrimSpace(value))] = struct{}{}
	}

	for _, addition := range additions {
		trimmed := strings.TrimSpace(addition)
		if trimmed == "" {
			continue
		}

		key := strings.ToLower(trimmed)
		if _, exists := seen[key]; exists {
			continue
		}

		seen[key] = struct{}{}
		next = append(next, trimmed)
	}

	return next
}

func nilIfEmptyReasons(values []string) *[]string {
	if len(values) == 0 {
		return nil
	}
	cloned := append([]string(nil), values...)
	return &cloned
}

func maxInt32(left int32, right int32) int32 {
	if left > right {
		return left
	}
	return right
}
